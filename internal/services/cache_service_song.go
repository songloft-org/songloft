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

	"songloft/internal/models"
	"songloft/internal/services/source"
)

// CacheSongFetcher 抽象 SourceOrchestrator,由 app.go 注入。
// 解耦 cache_service 与 source 包的具体类型,便于后续替换或单测 mock。
type CacheSongFetcher interface {
	Fetch(ctx context.Context, song *source.SongInfo, mode source.FetchMode) (*source.FetchResult, error)
}

// 全局唯一(per process)的 song-id-indexed 状态。
// 与原 hash-indexed inflight 共存,不冲突。
type songCacheState struct {
	inflightMu sync.Mutex
	inflight   map[int64]*inflightDownload // songID → 同步等待 channel
	lruMu      sync.RWMutex
	lru        map[int64]time.Time // songID → 最后访问时间
}

var (
	songStateOnce sync.Once
	songState     *songCacheState
)

func getSongState() *songCacheState {
	songStateOnce.Do(func() {
		songState = &songCacheState{
			inflight: make(map[int64]*inflightDownload),
			lru:      make(map[int64]time.Time),
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
//	例如 id=1234, key="lxmusic_kg_550109760" → cache_dir/12/0/1234.lxmusic_kg_550109760
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

// FindCachedFileBySong 在 song.ID 对应的目录下查找属于该 song 的 cache 文件。
// 严格匹配 `<id>.<key>.<ext>` 形态,key 由 cacheKeyOf(song) 计算。
// 命中时返回路径并 touch LRU;cacheKeyOf 返回空(无法唯一识别歌曲)时直接 miss。
//
// 旧格式 `<id><ext>` (无 .<key> 段) 不会命中——这是设计意图,
// 旧 cache 残留(如跨 DB 重建)因 key 不一致会自然失效,触发重新下载。
func (c *CacheService) FindCachedFileBySong(song *models.Song) (string, bool) {
	if song == nil {
		return "", false
	}
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
		// 严格匹配 "base" 或 "base.ext" 形式,避免前缀误命中
		if name == base || (len(name) > len(base) && name[len(base)] == '.') {
			p := filepath.Join(dir, name)
			c.touchSongLRU(song.ID)
			return p, true
		}
	}
	return "", false
}

// EvictSong 删除指定 song.ID 的所有缓存文件(供 SongService.Delete 钩子调用)。
// 删除目录下所有以 `<id>` 开头(后跟 `.` 或文件结束)的文件——含历史多个 key 的残留与旧格式。
// 目录或文件不存在视为成功。
func (c *CacheService) EvictSong(songID int64) error {
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
		// 必须是 "<id>" 或 "<id>." 开头,避免 id=12 误删 1234.mp3
		if name == idStr || (len(name) > len(idStr) && name[len(idStr)] == '.') {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
				slog.Warn("cache: remove file failed", "path", name, "error", err)
			}
		}
	}
	state := getSongState()
	state.lruMu.Lock()
	delete(state.lru, songID)
	state.lruMu.Unlock()
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
	state := getSongState()
	state.inflightMu.Lock()
	if dl, ok := state.inflight[song.ID]; ok {
		state.inflightMu.Unlock()
		<-dl.done
		if dl.err != nil {
			return "", dl.err
		}
		if p, ok := c.FindCachedFileBySong(song); ok {
			return p, nil
		}
		return "", fmt.Errorf("cache file not found after wait")
	}
	dl := &inflightDownload{done: make(chan struct{})}
	state.inflight[song.ID] = dl
	state.inflightMu.Unlock()
	defer func() {
		state.inflightMu.Lock()
		delete(state.inflight, song.ID)
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
	c.touchSongLRU(song.ID)
	// 触发 LRU 淘汰
	go c.EvictLRU()
	return finalPath, nil
}

// moveToCache 把临时文件移入 song 对应的缓存路径,返回最终路径。
// 文件名形如 `<id>.<key><ext>`,key 由 cacheKeyOf(song) 计算。key 为空时(理论上不会到这,
// 上层 Get 在 song 没有任何可识别源时已直接报错)退化为 `<id><ext>`。
// Windows 上若目标已存在则先删除。
func (c *CacheService) moveToCache(song *models.Song, tmpPath, ext string) (string, error) {
	if song == nil {
		return "", errors.New("song is nil")
	}
	key := cacheKeyOf(song)
	dir, base := c.getCachePath(song.ID, key)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir cache dir: %w", err)
	}
	name := base
	if ext != "" {
		name = base + ext
	}
	finalPath := filepath.Join(dir, name)
	if _, err := os.Stat(finalPath); err == nil {
		_ = os.Remove(finalPath)
	}
	if err := moveFile(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("move to cache: %w", err)
	}
	return finalPath, nil
}

// downloadExternalToTemp 纯外链歌曲的简化下载:直接 HTTP GET,无 fallback、无元数据校验。
// 因为纯外链没有"插件源"概念,Orchestrator 也无能为力。
func (c *CacheService) downloadExternalToTemp(ctx context.Context, url string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := c.downloadClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	ext := getExtFromContentType(contentType)

	tmp, err := os.CreateTemp("", "mimusic-extdl-*"+ext)
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

// touchSongLRU 更新 song-id-indexed LRU 时间戳。
// 与原 hash-indexed LRU 并存,EvictLRU 会同时考虑两者(但实现上仍以原 hash 索引为主)。
func (c *CacheService) touchSongLRU(songID int64) {
	state := getSongState()
	state.lruMu.Lock()
	state.lru[songID] = time.Now()
	state.lruMu.Unlock()
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
	_ = info
	// FetchResult 的 Info 只有 Duration/Size 等校验关键字段,没有 Format
	// (Format 字段在 metadata.AudioInfo 里,但 source.AudioInfoCopy 投影时未带过来)
	// 暂时统一返回 .mp3,后续可在 FetchResult 加 Ext 字段
	// 注意:FetchResult.TempPath 由 os.CreateTemp 创建,后缀是随机的,不能直接保留
	// TODO(ext-detection): 在 SourceFetcher 中根据 Content-Type 设置 ext,通过 FetchResult 透传。
	return ".mp3"
}

// 编译期断言
var _ = slog.Info
