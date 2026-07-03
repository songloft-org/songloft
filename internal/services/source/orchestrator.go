package source

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"os"
	"sync"
	"time"
)

// FetchMode 定义 Orchestrator.Fetch 的工作模式。
type FetchMode int

const (
	// ModeStrict 仅尝试主源 + 插件内 L1 自搜,失败立即返回。
	// 用于 cache HTTP handler 等同步路径,避免阻塞客户端等 fallback 完成。
	ModeStrict FetchMode = iota

	// ModeFallback 全链路 fallback:主源 → L1(插件内) → L2(跨插件 fan-out)。
	// 用于 convert 后台批量任务等可承受较长耗时的路径。
	ModeFallback
)

// SongUpdater 抽象 SongService 的"更新音源"操作。
// 通过接口注入,避免 source 包依赖 services 包。
type SongUpdater interface {
	UpdateSongSource(ctx context.Context, id int64, pluginEntryPath, sourceData string) error
	UpdateSongDuration(ctx context.Context, id int64, duration float64) error
}

// ReassignSessionKey 标识触发 AsyncReassign 的客户端会话。
// 与 playactivity.SessionKey 字段对应；source 包不直接依赖 playactivity，
// 通过这个最小映射类型把会话信息透到 ReassignTracker.Track。
type ReassignSessionKey struct {
	ClientID string
}

// ReassignTracker 把 AsyncReassign 的 60s ctx 注册进上层 cancel 表，
// 让用户切歌（同会话内 Activate(otherSongID)）时 reassign goroutine
// 立即让位，不再阻塞插件 worker。
//
// 由 app/wire 阶段注入；nil 时退化为不跟踪（保持向后兼容）。
type ReassignTracker interface {
	Track(parent context.Context, sk ReassignSessionKey, songID int64, cat string) (context.Context, func())
}

// OrchestratorOpts 编排器配置。
type OrchestratorOpts struct {
	Fetcher     *SourceFetcher
	Resolver    *SourceResolver
	SongUpdater SongUpdater
	MaxAttempts int // ModeFallback 时总尝试次数上限(含主源);默认 4
	// FallbackInterval 进入 L2 fallback 时,候选源之间的最小间隔(防风控)
	FallbackInterval time.Duration // 默认 3s
	// FallbackJitter 在 FallbackInterval 上随机加一个 [0, jitter) 抖动
	FallbackJitter time.Duration // 默认 2s
	// AsyncReassignDedupe 同一 songID 的 AsyncReassign 多次调用去重 TTL
	AsyncReassignDedupe time.Duration // 默认 5min
	// ActivityRegistry AsyncReassign 注入到上层 playActivity 的桥接接口；nil 时不跟踪。
	ActivityRegistry ReassignTracker
}

// SourceOrchestrator 编排 Fetcher + Resolver + SongUpdater。
// 是上层(CacheService、ConvertService)与音源逻辑的唯一入口。
type SourceOrchestrator struct {
	opts OrchestratorOpts

	mu             sync.Mutex
	reassignActive map[int64]time.Time // songID → 上次 reassign 时间(去重)
}

func NewSourceOrchestrator(opts OrchestratorOpts) *SourceOrchestrator {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 4
	}
	if opts.FallbackInterval <= 0 {
		opts.FallbackInterval = 3 * time.Second
	}
	if opts.FallbackJitter <= 0 {
		opts.FallbackJitter = 2 * time.Second
	}
	if opts.AsyncReassignDedupe <= 0 {
		opts.AsyncReassignDedupe = 5 * time.Minute
	}
	return &SourceOrchestrator{
		opts:           opts,
		reassignActive: make(map[int64]time.Time),
	}
}

// Fetch 编排整个下载链路。成功返回 FetchResult(包含临时文件路径),失败返回分类错误。
//
// 流程(ModeFallback):
//  1. 主源 + L1(插件内自搜):Fetcher.Fetch(主插件, 主 source_data, allowPluginFallback=true)
//     - 成功且 L1 触发了 source_data 变更 → persistIfChanged 回写 song
//     - 成功 → return
//     - 失败且 IsFallbackable → 继续 step 2;否则直接返回
//  2. L2(跨插件 fan-out):Resolver.Discover → 按 Score 降序逐个 Fetcher.Fetch(候选, allowPluginFallback=false)
//     - 总尝试次数受 MaxAttempts 限制
//     - 候选间 sleep [FallbackInterval, FallbackInterval+FallbackJitter)
//     - 任一成功 → persistIfChanged 回写 song,return
//     - 全部失败 → 返回 AllSourcesFailedError
//
// ModeStrict 仅执行 step 1,失败直接返回(不进入 L2)。
func (o *SourceOrchestrator) Fetch(ctx context.Context, song *SongInfo, mode FetchMode) (*FetchResult, error) {
	if song == nil {
		return nil, errors.New("song is nil")
	}

	// Step 1: 主源(允许 L1 插件内自搜)
	res, err := o.opts.Fetcher.Fetch(ctx, song.PluginEntryPath, song.SourceData, song, true)
	if err == nil {
		o.persistIfChanged(ctx, song, res)
		return res, nil
	}

	if mode == ModeStrict {
		return nil, err
	}

	if !IsFallbackable(err) {
		return nil, err
	}

	lastErr := err
	tried := 1

	// Step 2: 跨插件 fan-out
	alts, _ := o.opts.Resolver.Discover(ctx, song, []string{song.PluginEntryPath})
	if len(alts) == 0 {
		return nil, &AllSourcesFailedError{Tried: tried, LastErr: lastErr}
	}

	for _, cand := range alts {
		if tried >= o.opts.MaxAttempts {
			break
		}
		o.sleepFallback(ctx)
		tried++
		slog.Info("source: trying fallback candidate",
			"songId", song.ID, "plugin", cand.PluginEntryPath, "score", cand.Score, "attempt", tried)

		res, err := o.opts.Fetcher.Fetch(ctx, cand.PluginEntryPath, cand.SourceData, song, false)
		if err != nil {
			lastErr = err
			if !IsFallbackable(err) {
				return nil, err
			}
			continue
		}
		// 成功:把切换结果回写
		res.UpdatedSourceData = cand.SourceData // 标记必须 persist(主源已改)
		res.PluginEntryPath = cand.PluginEntryPath
		o.persistIfChanged(ctx, song, res)
		return res, nil
	}

	return nil, &AllSourcesFailedError{Tried: tried, LastErr: lastErr}
}

// ResolveURL 解析插件歌曲的可下载音频 URL（不下载）。
// 仅调用主源插件，不走 L2 fallback。用于流式代理场景。
func (o *SourceOrchestrator) ResolveURL(ctx context.Context, song *SongInfo) (*ResolvedURL, error) {
	if song == nil {
		return nil, errors.New("song is nil")
	}
	return o.opts.Fetcher.ResolveURL(ctx, song.PluginEntryPath, song.SourceData, song, true)
}

// AsyncReassign 在后台为指定 song 寻找新源并更新字段。
// 用于 cache HTTP handler 失败时:同步返回错误给客户端,后台静默切源。
// 5 分钟内同一 songID 的多次调用会被去重。
//
// sk 标识触发本次 reassign 的客户端会话；若注入了 ActivityRegistry，
// reassign goroutine 的 ctx 会注册到对应桶，用户切到其他歌时被一并 cancel。
//
// loader 由调用方提供(避免 source 包依赖 SongService 类型);它根据 ID 加载 SongInfo。
// 加载失败时直接放弃。
func (o *SourceOrchestrator) AsyncReassign(songID int64, sk ReassignSessionKey, loader func(context.Context, int64) (*SongInfo, error)) {
	if !o.tryAcquireReassign(songID) {
		return
	}
	go func() {
		defer o.releaseReassign(songID)
		base, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		ctx := base
		if o.opts.ActivityRegistry != nil {
			var release func()
			ctx, release = o.opts.ActivityRegistry.Track(base, sk, songID, "reassign")
			defer release()
		}

		song, err := loader(ctx, songID)
		if err != nil || song == nil {
			return
		}
		res, err := o.Fetch(ctx, song, ModeFallback)
		if err != nil {
			slog.Info("async reassign: all sources failed", "songId", songID, "error", err)
			return
		}
		// reassign 只需要更新 song.plugin_entry_path/source_data(已在 persistIfChanged 完成),
		// Fetch 产生的临时音频文件不需要保留,立即清理防止泄漏。
		if res.TempPath != "" {
			_ = os.Remove(res.TempPath)
		}
		slog.Info("async reassign succeeded", "songId", songID)
	}()
}

func (o *SourceOrchestrator) persistIfChanged(ctx context.Context, song *SongInfo, res *FetchResult) {
	if res == nil || o.opts.SongUpdater == nil {
		return
	}

	// 1. 时长回填(主源/fallback 都有意义)
	if res.Info != nil && res.Info.Duration > 0 && song.Duration == 0 {
		_ = o.opts.SongUpdater.UpdateSongDuration(ctx, song.ID, res.Info.Duration)
	}

	// 2. 音源切换回写(只在确实变化时)
	newPlugin := res.PluginEntryPath
	newSource := res.SourceData
	if res.UpdatedSourceData != "" {
		newSource = res.UpdatedSourceData
	}
	if newPlugin == song.PluginEntryPath && newSource == song.SourceData {
		return
	}
	if err := o.opts.SongUpdater.UpdateSongSource(ctx, song.ID, newPlugin, newSource); err != nil {
		slog.Warn("persist song source failed", "songId", song.ID, "plugin", newPlugin, "error", err)
		return
	}
	slog.Info("song source updated",
		"songId", song.ID,
		"from_plugin", song.PluginEntryPath, "to_plugin", newPlugin,
		"used_fallback", res.UsedFallback)
}

func (o *SourceOrchestrator) sleepFallback(ctx context.Context) {
	jitter := time.Duration(rand.Int63n(int64(o.opts.FallbackJitter)))
	d := o.opts.FallbackInterval + jitter
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

func (o *SourceOrchestrator) tryAcquireReassign(songID int64) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	// 顺带清理过期条目,防止 map 无限增长
	threshold := 2 * o.opts.AsyncReassignDedupe
	for id, last := range o.reassignActive {
		if time.Since(last) > threshold {
			delete(o.reassignActive, id)
		}
	}
	if last, ok := o.reassignActive[songID]; ok {
		if time.Since(last) < o.opts.AsyncReassignDedupe {
			return false
		}
	}
	o.reassignActive[songID] = time.Now()
	return true
}

func (o *SourceOrchestrator) releaseReassign(songID int64) {
	// 不立刻删除——保留 timestamp 作为去重窗口。
	// 过期条目在 tryAcquireReassign 中统一清理(2×AsyncReassignDedupe 阈值)。
	_ = songID
}
