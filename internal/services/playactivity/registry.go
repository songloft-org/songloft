// Package playactivity 维护进行中"和某首歌相关"的工作的 cancel 句柄，
// 让用户切歌时旧工作（HTTP play、prefetch、ffmpeg 转码、AsyncReassign）能被
// 一次性取消，不再因为客户端不 abort 旧 HTTP 而占用 plugin worker / 转码 sem。
//
// 见 issue #79：快速切歌仍会"转圈"——根因之一是后端无法从外部得知用户已经放弃旧请求。
package playactivity

import (
	"context"
	"sync"
	"sync/atomic"

	"songloft/internal/contextkeys"
)

// Category 标记一条 entry 的工作类型。Activate 在判断"是否取消"时按 cat 区分行为。
type Category string

const (
	CatPlay      Category = "play"      // GET /songs/{id}/play 主路径
	CatPrefetch  Category = "prefetch"  // GET /songs/{id}/play?prefetch=1
	CatTranscode Category = "transcode" // ffmpeg 转码（GetOrTranscode）
	CatReassign  Category = "reassign"  // SourceOrchestrator.AsyncReassign
)

// SessionKey 把 Registry 按客户端会话分桶，防止多客户端同时登录时相互 cancel。
//
// 当前来自 r.Context() 里的 client_id（见 internal/middleware/auth.go）。
// 未来加 UserID 多用户化时直接添加字段即可，调用点只需要更新 SessionFromContext。
type SessionKey struct {
	ClientID string
}

// ctxClientIDKey 保留给本包测试使用，并与认证中间件共用同一个 typed key。
const ctxClientIDKey = contextkeys.ClientIDKey

// SessionFromContext 从请求 ctx 里抽出 client_id 构造 SessionKey；
// 没有 client_id（系统任务、未走鉴权中间件的内部调用）时返回零值，落到独立"系统桶"。
func SessionFromContext(ctx context.Context) SessionKey {
	if ctx == nil {
		return SessionKey{}
	}
	if v, ok := ctx.Value(ctxClientIDKey).(string); ok {
		return SessionKey{ClientID: v}
	}
	return SessionKey{}
}

// entry 内部记录单条已注册工作。
type entry struct {
	id     uint64
	songID int64
	cat    Category
	cancel context.CancelFunc
}

// Registry 是按 (sessionKey, songID, category) 索引的 cancel 表。
type Registry struct {
	mu      sync.Mutex
	nextID  atomic.Uint64
	buckets map[SessionKey]map[uint64]*entry
}

// New 创建空 Registry。
func New() *Registry {
	return &Registry{
		buckets: make(map[SessionKey]map[uint64]*entry),
	}
}

// Track 把一条工作注册进 registry。
//
// 返回派生 ctx（context.WithCancel(parent)）和 release 闭包。release 必须用 defer 调用：
// 它会先 cancel ctx 再从 registry 移除该 entry，保证不泄漏 goroutine。
func (r *Registry) Track(parent context.Context, sk SessionKey, songID int64, cat Category) (context.Context, func()) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	id := r.nextID.Add(1)
	e := &entry{id: id, songID: songID, cat: cat, cancel: cancel}

	r.mu.Lock()
	bucket, ok := r.buckets[sk]
	if !ok {
		bucket = make(map[uint64]*entry)
		r.buckets[sk] = bucket
	}
	bucket[id] = e
	r.mu.Unlock()

	release := func() {
		cancel()
		r.mu.Lock()
		if b, ok := r.buckets[sk]; ok {
			delete(b, id)
			if len(b) == 0 {
				delete(r.buckets, sk)
			}
		}
		r.mu.Unlock()
	}
	return ctx, release
}

// Activate 标记 (sk, keepSongID) 为当前活跃。
//
// 仅在 sk 桶内 cancel，不影响其他 sessionKey：
//   - songID != keepSongID 的 play / transcode / reassign 工作
//
// **不取消任何 CatPrefetch 条目**（无论 songID 是否等于 keepSongID）。prefetch 天然是为
// 「下一首」预热的：顺序播放时，正在播放 song N（触发 Activate(N)）的同时，插件已经为
// song N+1 发起了 prefetch。若 Activate 把非当前歌的 prefetch 掐掉，就会在每次切歌时杀掉
// 刚发起的「下一首」预热转码，导致真正播放 N+1 时只能从零实时转码——prefetch 特性形同虚设
// （songloft-org/songloft#300：日志里预热 ffmpeg 反复 "signal: killed"，播放时 dur_ms 高达
// 29s~117s 的实时转码）。prefetch 无需 Activate 清理也不会失控：ffmpeg 处理完整个文件后自然
// 退出、prepareSongPlayback 返回即 release() 注销 entry；转码信号量串行（sem=1，不会 CPU 风暴）
// + inflight 去重，让孤儿转码有界且会把结果缓存下来（下次真正播放直接命中）。
//
// 同样不动同桶 keepSongID 的任何工作（play / transcode / reassign）——避免取消"自己"。
// keepSongID 的 prefetch 尤其重要：慢音源（如 B站，music/url 解析要 ~9s）的预热是让「真实
// 播放直接命中缓存」的关键，掐掉会让同步播放路从零重解析，libmpv 等客户端 ~5s 无数据即断连
// 直接 502（songloft-org/songloft#271）。
func (r *Registry) Activate(sk SessionKey, keepSongID int64) {
	r.mu.Lock()
	bucket, ok := r.buckets[sk]
	if !ok {
		r.mu.Unlock()
		return
	}
	// 收集要 cancel 的 entries，先释放锁再 cancel（cancel 可能会唤醒 select 触发 release，
	// release 又要拿同一把锁——避免重入）。
	toCancel := make([]*entry, 0)
	for id, e := range bucket {
		// 保留所有 prefetch（为下一首预热，见函数注释）与当前活跃歌曲的全部工作。
		if e.cat == CatPrefetch || e.songID == keepSongID {
			continue
		}
		toCancel = append(toCancel, e)
		delete(bucket, id)
	}
	if len(bucket) == 0 {
		delete(r.buckets, sk)
	}
	r.mu.Unlock()

	for _, e := range toCancel {
		e.cancel()
	}
}

// Size 返回 sk 桶内的 entry 数（用于测试与诊断）。
func (r *Registry) Size(sk SessionKey) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buckets[sk])
}

// TotalSize 返回所有桶里的 entry 总数（用于测试与诊断）。
func (r *Registry) TotalSize() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	total := 0
	for _, b := range r.buckets {
		total += len(b)
	}
	return total
}
