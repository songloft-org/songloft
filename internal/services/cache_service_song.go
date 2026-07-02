package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/models"
	"songloft/internal/services/source"
)

// CacheSongFetcher 抽象 SourceOrchestrator,由 app.go 注入。
// 解耦 cache_service 与 source 包的具体类型,便于后续替换或单测 mock。
type CacheSongFetcher interface {
	Fetch(ctx context.Context, song *source.SongInfo, mode source.FetchMode) (*source.FetchResult, error)
	ResolveURL(ctx context.Context, song *source.SongInfo) (string, error)
}

// 全局唯一(per process)的 song-id-indexed 状态。
// 与原 hash-indexed inflight 共存,不冲突。
type songCacheState struct {
	inflightMu          sync.Mutex
	inflight            map[int64]*inflightDownload // songID → 同步等待 channel
	transcodeInflightMu sync.Mutex
	transcodeInflight   map[string]*inflightDownload // "tc_{songID}_{format}" → 同步等待 channel
}

var (
	songStateOnce sync.Once
	songState     *songCacheState
)

func getSongState() *songCacheState {
	songStateOnce.Do(func() {
		songState = &songCacheState{
			inflight:          make(map[int64]*inflightDownload),
			transcodeInflight: make(map[string]*inflightDownload),
		}
	})
	return songState
}

// SetOrchestrator 注入下载编排器,使 CacheService 能在 cache miss 时拉取音频。
// 不强制要求 —— 未注入时 Get 会返回 ErrNoOrchestrator(用于一些纯本地化测试)。
func (c *CacheService) SetOrchestrator(o CacheSongFetcher) {
	c.orchestrator = o
}

// ErrNoOrchestrator orchestrator 未注入
var ErrNoOrchestrator = errors.New("source orchestrator not configured")

// getCachePath 计算 song 对应的缓存文件路径(不含扩展名,扩展名由命中 / 写入时确定)。
//
// 布局:cache_dir/<id/100%1000>/<id/10000%100>/<id>.<key>
//
//	例如 id=1234, key="subsonic_srv1_550109760" → cache_dir/12/0/1234.subsonic_srv1_550109760
//
// key 用于绑定"该文件属于哪首具体的歌"(取自 plugin_entry_path + dedup_key 或 URL md5),
// 防止跨 DB 重建后旧 cache 与新 song.ID 偶然碰撞被误命中。key 为空时退化为旧形态
// `<id>`,只用于 EvictSong 这类不需要严格匹配的场景。
func (c *CacheService) getCachePath(songID int64, key string) (dir string, base string) {
	first := songID / 100 % 1000   // 第一层目录
	second := songID / 10000 % 100 // 第二层目录,大致使每个目录文件数可控
	dir = filepath.Join(c.cacheDir, strconv.FormatInt(first, 10), strconv.FormatInt(second, 10))
	idStr := strconv.FormatInt(songID, 10)
	if key == "" {
		base = idStr
	} else {
		base = idStr + "." + key
	}
	return dir, base
}

// cacheKeyOf 计算 song 的 cache 指纹 key。
//   - 插件来源(有 plugin_entry_path + dedup_key):key = sanitize(plugin + "_" + dedup_key)
//   - 纯外链(仅 URL):key = "u" + md5(URL)[:12]
//   - 都没有:返回 "",调用方应避免走 cache 路径
func cacheKeyOf(song *models.Song) string {
	if song == nil {
		return ""
	}
	if song.PluginEntryPath != "" && song.DedupKey != "" {
		return sanitizeCacheKey(song.PluginEntryPath + "_" + song.DedupKey)
	}
	if song.URL != "" {
		sum := md5.Sum([]byte(song.URL))
		return "u" + hex.EncodeToString(sum[:])[:12]
	}
	return ""
}

// sanitizeCacheKey 过滤文件名不安全字符,把非 [a-zA-Z0-9._-] 一律替换为 _。
// 截断到 64 字节,避免极端长 dedup_key 撞 filesystem 上限。
func sanitizeCacheKey(s string) string {
	const maxLen = 64
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	return out
}

// FindCachedFileBySong 查找歌曲的缓存文件。
// 优先从 song.CachePath（DB 存储的路径）查找；fallback 到旧格式哈希分桶目录。
func (c *CacheService) FindCachedFileBySong(song *models.Song) (string, bool) {
	if song == nil {
		return "", false
	}

	// 优先：DB 中记录的结构化缓存路径
	if song.CachePath != "" {
		if _, err := os.Stat(song.CachePath); err == nil {
			return song.CachePath, true
		}
		// 文件不存在，惰性清理
		if c.clearCachePath != nil {
			_ = c.clearCachePath(context.Background(), song.ID)
		}
	}

	// fallback：旧格式哈希分桶（兼容升级期间的旧缓存文件）
	key := cacheKeyOf(song)
	if key == "" {
		return "", false
	}
	dir, base := c.getCachePath(song.ID, key)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, base) || strings.HasSuffix(name, ".tmp") {
			continue
		}
		if name == base || (len(name) > len(base) && name[len(base)] == '.') {
			p := filepath.Join(dir, name)
			return p, true
		}
	}
	return "", false
}

// EvictSong 删除指定歌曲的所有缓存文件。
// 先从 cachePath (DB 存储的结构化路径) 删除，再 fallback 到旧格式哈希分桶清理。
func (c *CacheService) EvictSong(songID int64, cachePath string) error {
	// 新格式：从 DB cache_path 删除
	if cachePath != "" {
		if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
			slog.Warn("cache: remove structured file failed", "path", cachePath, "error", err)
		}
		cleanEmptyParentDirs(filepath.Dir(cachePath), c.cacheDir)
		if c.clearCachePath != nil {
			_ = c.clearCachePath(context.Background(), songID)
		}
	}

	// 旧格式 fallback
	dir, _ := c.getCachePath(songID, "")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	idStr := strconv.FormatInt(songID, 10)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, idStr) {
			continue
		}
		if name == idStr || (len(name) > len(idStr) && name[len(idStr)] == '.') {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
				slog.Warn("cache: remove file failed", "path", name, "error", err)
			}
		}
	}
	return nil
}

// Get 是 cache handler 的统一入口:
//   - 命中本地缓存 → 直接返回路径
//   - 未命中且 song 为插件来源 → 调 Orchestrator(Strict) 拉到临时文件 → 移入缓存目录 → 返回
//   - 未命中且 song 为纯外链 → 简化下载路径(httpClient.Get)
//   - 同 song.ID 的并发请求通过 inflight 去重
//
// 严格模式(ModeStrict)保证 cache HTTP 路径不被长 fallback 阻塞;失败时调用方应触发
// Orchestrator.AsyncReassign 在后台静默切源。
func (c *CacheService) Get(ctx context.Context, song *models.Song) (string, error) {
	if song == nil {
		return "", errors.New("song is nil")
	}

	// 1. 命中本地缓存
	if p, ok := c.FindCachedFileBySong(song); ok {
		return p, nil
	}

	// 2. inflight 去重(同 song.ID 的并发请求只下载一次)
	// 注意：若首个请求被 ctx.Canceled（用户切歌触发 abort），不能把这个错误传染给
	// 后到的等待者——他们的 ctx 是新的、未取消，必须重新尝试下载。否则"切走又切回"
	// 同一首歌时会立刻 502（issue #79 的二次原因）。
	state := getSongState()
	for {
		state.inflightMu.Lock()
		dl, ok := state.inflight[song.ID]
		if !ok {
			// 没有 inflight，自己来下载
			dl = &inflightDownload{done: make(chan struct{})}
			state.inflight[song.ID] = dl
			state.inflightMu.Unlock()
			break
		}
		state.inflightMu.Unlock()

		// 等待已存在的 inflight 完成；同时监听本等待者自己的 ctx，
		// 防止首请求卡住时把第二、三个等待者也一起拖死（issue #79 残留点）。
		select {
		case <-dl.done:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if dl.err != nil {
			if errors.Is(dl.err, context.Canceled) || errors.Is(dl.err, context.DeadlineExceeded) {
				// 首请求被自己的 ctx 取消，不算"真错误"，本等待者重新尝试占位下载
				slog.Debug("cache inflight: prior request canceled, retrying as new downloader",
					"songId", song.ID, "err", dl.err)
				continue
			}
			return "", dl.err
		}
		// 首请求成功，去 cache 取
		if p, ok := c.FindCachedFileBySong(song); ok {
			return p, nil
		}
		return "", fmt.Errorf("cache file not found after wait")
	}
	dl := state.inflight[song.ID]
	defer func() {
		state.inflightMu.Lock()
		// 仅在 map 里的还是自己时删除，防止其它代码路径替换后误删
		if state.inflight[song.ID] == dl {
			delete(state.inflight, song.ID)
		}
		state.inflightMu.Unlock()
		close(dl.done)
	}()

	// 3. 实际拉取
	var (
		tmpPath  string
		ext      string
		fetchErr error
	)

	switch {
	case song.IsPluginSourced():
		if c.orchestrator == nil {
			dl.err = ErrNoOrchestrator
			return "", ErrNoOrchestrator
		}
		res, err := c.orchestrator.Fetch(ctx, songInfoOf(song), source.ModeStrict)
		if err != nil {
			dl.err = err
			return "", err
		}
		tmpPath = res.TempPath
		ext = extensionFromFormat(res.Info)
	case song.URL != "":
		// 纯外链:简化下载,无 fallback
		tmpPath, ext, fetchErr = c.downloadExternalToTemp(ctx, song.URL)
		if fetchErr != nil {
			dl.err = fetchErr
			return "", fetchErr
		}
	default:
		dl.err = errors.New("song has no playable source")
		return "", dl.err
	}

	// 4. 移入 cache 目录
	finalPath, err := c.moveToCache(song, tmpPath, ext)
	if err != nil {
		_ = os.Remove(tmpPath)
		dl.err = err
		return "", err
	}
	// 触发 LRU 淘汰
	go c.EvictLRU()
	return finalPath, nil
}

// moveToCache 把临时文件移入 song 对应的缓存路径,返回最终路径。
func (c *CacheService) moveToCache(song *models.Song, tmpPath, ext string) (string, error) {
	if song == nil {
		return "", errors.New("song is nil")
	}
	key := cacheKeyOf(song)
	dir, base := c.getCachePath(song.ID, key)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir cache dir: %w", err)
	}
	finalPath := filepath.Join(dir, base+ext)
	if _, err := os.Stat(finalPath); err == nil {
		_ = os.Remove(finalPath)
	}
	if err := moveFile(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("move to cache: %w", err)
	}
	return finalPath, nil
}

// FinalizeCache 将临时文件移入结构化缓存目录并写入元数据。
// 由流式代理完成回调在 goroutine 中调用。
func (c *CacheService) FinalizeCache(ctx context.Context, song *models.Song, tmpPath, ext string) {
	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	finalPath, err := c.moveToCache(song, tmpPath, ext)
	if err != nil {
		slog.Warn("finalize cache: move failed", "songId", song.ID, "error", err)
		return
	}

	if c.updateCachePath != nil {
		if err := c.updateCachePath(ctx, song.ID, finalPath); err != nil {
			slog.Warn("finalize cache: update cache_path failed", "songId", song.ID, "error", err)
		}
	}

	if c.onCacheComplete != nil {
		songCopy := *song
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			c.onCacheComplete(ctx, &songCopy, finalPath)
		}()
	}

	c.EvictLRU()
}

// AsyncDownloadAndCache 在后台全量下载远程歌曲并缓存。
// 用于 206 场景：客户端已在接收代理流，此方法独立发起全量 GET。
func (c *CacheService) AsyncDownloadAndCache(ctx context.Context, song *models.Song, url string) {
	tmpPath, ext, err := c.downloadExternalToTemp(ctx, url)
	if err != nil {
		slog.Warn("async cache download failed", "songId", song.ID, "error", err)
		return
	}
	c.FinalizeCache(ctx, song, tmpPath, ext)
}

// ResolveURL 解析插件歌曲的可下载音频 URL（不下载）。
func (c *CacheService) ResolveURL(ctx context.Context, song *models.Song) (string, error) {
	if c.orchestrator == nil {
		return "", ErrNoOrchestrator
	}
	return c.orchestrator.ResolveURL(ctx, songInfoOf(song))
}

// ClearStaleCachePath 清理过期的缓存路径记录（文件已不存在时调用）。
func (c *CacheService) ClearStaleCachePath(songID int64) {
	if c.clearCachePath != nil {
		_ = c.clearCachePath(context.Background(), songID)
	}
}

// downloadExternalToTemp 纯外链歌曲的简化下载:直接 HTTP GET,无 fallback、无元数据校验。
// 因为纯外链没有"插件源"概念,Orchestrator 也无能为力。
func (c *CacheService) downloadExternalToTemp(ctx context.Context, url string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	httputil.ApplyBasicAuthFromURL(req)
	resp, err := c.downloadClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	ext := GetExtFromContentType(contentType)

	tmp, err := os.CreateTemp("", "songloft-extdl-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	written, err := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("write temp: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("close temp: %w", closeErr)
	}
	if written < MinAudioSize {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("downloaded too small: %d", written)
	}
	return tmpPath, ext, nil
}

// songInfoOf 把 models.Song 投影成 source.SongInfo(避免 source 包依赖 models)
func songInfoOf(s *models.Song) *source.SongInfo {
	if s == nil {
		return nil
	}
	return &source.SongInfo{
		ID:              s.ID,
		Title:           s.Title,
		Artist:          s.Artist,
		Album:           s.Album,
		Duration:        s.Duration,
		PluginEntryPath: s.PluginEntryPath,
		SourceData:      s.SourceData,
	}
}

// extensionFromFormat 把 AudioInfoCopy 的格式字段映射回扩展名。
// 留空时由调用方根据 Content-Type 兜底。
func extensionFromFormat(info *source.AudioInfoCopy) string {
	if info != nil && info.Format != "" {
		return "." + info.Format
	}
	return ".mp3"
}

// 编译期断言
var _ = slog.Info
