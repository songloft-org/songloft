package services

import (
	"container/heap"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/models"
)

const (
	// MinAudioSize 最小有效音频文件大小（1KB），低于此值认为是错误响应
	MinAudioSize = 1024
	// defaultMaxCacheSize 默认最大缓存大小（1GB）
	defaultMaxCacheSize int64 = 1 * 1024 * 1024 * 1024
	// cacheConfigKey 缓存配置在 configs 表中的 key
	cacheConfigKey = "music_cache_config"
)

// CacheStats 缓存统计信息
type CacheStats struct {
	TotalSize int64 `json:"total_size"` // 总大小（字节）
	FileCount int   `json:"file_count"` // 文件数量
	MaxSize   int64 `json:"max_size"`   // 最大缓存大小（字节），0 表示无限制
}

// CacheConfig 缓存配置（持久化到 configs 表）
type CacheConfig struct {
	MaxSize  int64  `json:"max_size"`  // 最大缓存大小（字节），0 表示无限制
	CacheDir string `json:"cache_dir"` // 自定义缓存目录，空字符串表示使用默认目录
}

// CacheConfigResponse 缓存配置 API 响应（含只读的默认目录信息）
type CacheConfigResponse struct {
	MaxSize         int64  `json:"max_size"`
	CacheDir        string `json:"cache_dir"`
	DefaultCacheDir string `json:"default_cache_dir"`
}

// inflightDownload 追踪正在进行的下载
type inflightDownload struct {
	done chan struct{} // 下载完成时关闭
	err  error         // 下载结果，在 close(done) 前设置
}

// CacheService 音乐缓存服务
type CacheService struct {
	cacheDir        string
	defaultCacheDir string
	configService   *ConfigService
	downloadClient  *http.Client // 用于纯外链 GET（cache_service_song.downloadExternalToTemp）
	lruIndex        map[string]time.Time
	lruMu           sync.RWMutex
	orchestrator    CacheSongFetcher // 下载编排器(按 song.ID),由 app.go 注入
	ffmpegPath      string           // ffmpeg 可执行文件路径,由 app.go 注入
	transcodeSem    chan struct{}    // 转码串行信号量（默认 size=1），防止并发 ffmpeg 争抢 CPU
	// 回调：由 app.go 注入,连接 SongRepository
	updateCachePath    func(ctx context.Context, songID int64, cachePath string) error
	clearCachePath     func(ctx context.Context, songID int64) error
	clearAllCachePaths func(ctx context.Context) error
	listSongsWithCache func(ctx context.Context) ([]*models.Song, error)
	// 缓存完成回调：完整元数据兜底提取
	onCacheComplete func(ctx context.Context, song *models.Song, filePath string)
}

// NewCacheService 创建缓存服务
func NewCacheService(defaultCacheDir string, configService *ConfigService) *CacheService {
	cs := &CacheService{
		cacheDir:        defaultCacheDir,
		defaultCacheDir: defaultCacheDir,
		configService:   configService,
		lruIndex:        make(map[string]time.Time),
		transcodeSem:    make(chan struct{}, 1),
		downloadClient:  httputil.NewClient(120 * time.Second),
	}
	var cfg CacheConfig
	if err := configService.GetJSON(cacheConfigKey, &cfg); err == nil && cfg.CacheDir != "" {
		cs.cacheDir = cfg.CacheDir
		slog.Info("使用自定义缓存目录", "path", cfg.CacheDir)
	}
	cs.loadLRUIndex()
	return cs
}

// SetCachePathCallbacks 注入缓存路径更新回调（由 app.go 调用）。
func (c *CacheService) SetCachePathCallbacks(
	update func(ctx context.Context, songID int64, cachePath string) error,
	clear func(ctx context.Context, songID int64) error,
	clearAll func(ctx context.Context) error,
	listWithCache func(ctx context.Context) ([]*models.Song, error),
) {
	c.updateCachePath = update
	c.clearCachePath = clear
	c.clearAllCachePaths = clearAll
	c.listSongsWithCache = listWithCache
}

// SetCacheCompleteCallback 注入缓存完成回调（由 app.go 调用）。
// 缓存完成后，对元数据缺失的歌曲从本地文件做完整提取兜底。
func (c *CacheService) SetCacheCompleteCallback(
	fn func(ctx context.Context, song *models.Song, filePath string),
) {
	c.onCacheComplete = fn
}

// isAudioContentType 检查 Content-Type 是否为音频类型
func isAudioContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "audio/") ||
		strings.Contains(ct, "video/mp4") ||
		strings.Contains(ct, "application/octet-stream")
}

// GetExtFromContentType 根据 Content-Type 获取文件扩展名
func GetExtFromContentType(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "audio/mpeg"):
		return ".mp3"
	case strings.Contains(ct, "audio/flac"):
		return ".flac"
	case strings.Contains(ct, "audio/ogg"):
		return ".ogg"
	case strings.Contains(ct, "audio/x-m4a"), strings.Contains(ct, "audio/mp4"), strings.Contains(ct, "video/mp4"):
		return ".m4a"
	case strings.Contains(ct, "video/quicktime"):
		return ".mov"
	case strings.Contains(ct, "audio/wav"):
		return ".wav"
	default:
		return ".mp3"
	}
}

// loadLRUIndex 启动时从文件系统 mtime 加载 LRU 索引
func (c *CacheService) loadLRUIndex() {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	count := 0
	err := filepath.Walk(c.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if info.IsDir() {
			return nil
		}
		// 跳过临时文件
		if strings.HasSuffix(info.Name(), ".tmp") {
			return nil
		}
		// 从文件名提取 hash（文件名格式：hash.ext）
		name := info.Name()
		ext := filepath.Ext(name)
		hash := strings.TrimSuffix(name, ext)
		if hash != "" {
			c.lruIndex[hash] = info.ModTime()
			count++
		}
		return nil
	})
	if err != nil {
		slog.Warn("加载 LRU 索引失败", "error", err)
	}
	slog.Info("LRU 索引加载完成", "count", count)
}

// GetCacheStats 统计缓存目录的总大小和文件数
func (c *CacheService) GetCacheStats() CacheStats {
	stats := CacheStats{
		MaxSize: c.getMaxCacheSize(),
	}

	err := filepath.Walk(c.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".tmp") {
			return nil
		}
		stats.TotalSize += info.Size()
		stats.FileCount++
		return nil
	})
	if err != nil {
		slog.Warn("统计缓存大小失败", "error", err)
	}

	return stats
}

// CleanCache 清理全部缓存文件
func (c *CacheService) CleanCache() error {
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	// 删除缓存目录下的所有内容
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取缓存目录失败: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(c.cacheDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			slog.Warn("删除缓存文件失败", "path", path, "error", err)
		}
	}

	// 清空内存索引
	c.lruIndex = make(map[string]time.Time)

	// 清空所有歌曲的 cache_path
	if c.clearAllCachePaths != nil {
		if err := c.clearAllCachePaths(context.Background()); err != nil {
			slog.Warn("清空 cache_path 失败", "error", err)
		}
	}

	slog.Info("缓存已全部清理")
	return nil
}

// lruEntry LRU 淘汰排序用的条目
type lruEntry struct {
	hash       string
	filePath   string
	size       int64
	lastAccess time.Time
}

// lruMaxHeap 最大堆（按 lastAccess 降序：最新访问的在堆顶）
// 用于在遍历过程中只保留最旧的 N 个文件，避免全量收集。
// 遍历时如果当前文件比堆顶更旧，替换堆顶，最终堆中即为最应淘汰的文件。
type lruMaxHeap []lruEntry

func (h lruMaxHeap) Len() int           { return len(h) }
func (h lruMaxHeap) Less(i, j int) bool { return h[i].lastAccess.After(h[j].lastAccess) }
func (h lruMaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *lruMaxHeap) Push(x any)        { *h = append(*h, x.(lruEntry)) }
func (h *lruMaxHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// lruHeapCap 计算 LRU 淘汰堆的容量上限
// 基于当前索引大小的 1/4，最小 128，最大不超过索引总数
const minLRUHeapCap = 128

// EvictLRU 按 LRU 策略淘汰缓存，直到总大小低于上限
// 使用 container/heap 实现最大堆，只保留最旧的 N 个文件作为淘汰候选，
// 避免全量收集所有文件信息后再排序，内存开销从 O(全部文件数) 降为 O(候选数)。
func (c *CacheService) EvictLRU() {
	maxSize := c.getMaxCacheSize()
	if maxSize <= 0 {
		return // 0 表示无限制
	}

	// 计算堆容量：取索引大小的 1/4 与最小值中的较大者，但不超过索引总数
	heapCap := len(c.lruIndex) / 4
	if heapCap < minLRUHeapCap {
		heapCap = minLRUHeapCap
	}
	if indexSize := len(c.lruIndex); heapCap > indexSize && indexSize > 0 {
		heapCap = indexSize
	}

	var totalSize int64
	h := &lruMaxHeap{}
	heap.Init(h)

	c.lruMu.RLock()
	err := filepath.Walk(c.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".tmp") {
			return nil
		}
		name := info.Name()
		ext := filepath.Ext(name)
		hash := strings.TrimSuffix(name, ext)

		lastAccess := info.ModTime()
		if t, ok := c.lruIndex[hash]; ok {
			lastAccess = t
		}

		entry := lruEntry{
			hash:       hash,
			filePath:   path,
			size:       info.Size(),
			lastAccess: lastAccess,
		}
		totalSize += info.Size()

		// 维护固定大小的最大堆（堆顶为最新访问），保留最旧的 heapCap 个文件
		if h.Len() < heapCap {
			heap.Push(h, entry)
		} else if h.Len() > 0 && entry.lastAccess.Before((*h)[0].lastAccess) {
			// 当前文件比堆顶更旧（更早访问），替换堆顶
			(*h)[0] = entry
			heap.Fix(h, 0)
		}

		return nil
	})
	c.lruMu.RUnlock()

	if err != nil {
		slog.Warn("遍历缓存目录失败", "error", err)
		return
	}

	if totalSize <= maxSize {
		return // 未超限，无需淘汰
	}

	// 将堆中的候选文件按访问时间升序排序（最旧的在前），依次淘汰
	candidates := []lruEntry(*h)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastAccess.Before(candidates[j].lastAccess)
	})

	// 从最旧的开始删除，直到总大小低于上限
	c.lruMu.Lock()
	defer c.lruMu.Unlock()

	evicted := 0
	for _, entry := range candidates {
		if totalSize <= maxSize {
			break
		}
		if err := os.Remove(entry.filePath); err != nil {
			slog.Warn("LRU 淘汰删除文件失败", "path", entry.filePath, "error", err)
			continue
		}
		totalSize -= entry.size
		delete(c.lruIndex, entry.hash)
		cleanEmptyParentDirs(filepath.Dir(entry.filePath), c.cacheDir)
		evicted++
		slog.Debug("LRU 淘汰缓存文件", "hash", entry.hash, "size", entry.size)
	}

	if evicted > 0 {
		slog.Info("LRU 淘汰完成", "evicted", evicted, "remainingSize", totalSize)
	}
	if totalSize > maxSize {
		slog.Warn("LRU 淘汰后仍超限，下次淘汰将继续清理",
			"remainingSize", totalSize, "maxSize", maxSize, "heapCap", heapCap)
	}
}

// getMaxCacheSize 从 configService 读取最大缓存大小配置
func (c *CacheService) getMaxCacheSize() int64 {
	var cfg CacheConfig
	if err := c.configService.GetJSON(cacheConfigKey, &cfg); err != nil {
		return defaultMaxCacheSize
	}
	return cfg.MaxSize
}

// GetCacheConfig 获取缓存配置
func (c *CacheService) GetCacheConfig() CacheConfig {
	var cfg CacheConfig
	if err := c.configService.GetJSON(cacheConfigKey, &cfg); err != nil {
		return CacheConfig{MaxSize: defaultMaxCacheSize}
	}
	return cfg
}

// GetCacheConfigResponse 获取含默认目录信息的完整缓存配置
func (c *CacheService) GetCacheConfigResponse() CacheConfigResponse {
	cfg := c.GetCacheConfig()
	return CacheConfigResponse{
		MaxSize:         cfg.MaxSize,
		CacheDir:        cfg.CacheDir,
		DefaultCacheDir: c.defaultCacheDir,
	}
}

// UpdateCacheConfig 更新缓存配置
func (c *CacheService) UpdateCacheConfig(cfg CacheConfig) error {
	oldDir := c.cacheDir
	if err := c.configService.SetJSON(cacheConfigKey, cfg); err != nil {
		return fmt.Errorf("更新缓存配置失败: %w", err)
	}

	newDir := c.defaultCacheDir
	if cfg.CacheDir != "" {
		newDir = cfg.CacheDir
	}
	if newDir != oldDir {
		c.setCacheDir(newDir)
	}

	go c.EvictLRU()
	return nil
}

// cleanEmptyParentDirs 向上递归删除空目录，直到 stopAt（不含 stopAt 本身）。
func cleanEmptyParentDirs(dir, stopAt string) {
	for dir != stopAt && strings.HasPrefix(dir, stopAt) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// setCacheDir 切换缓存目录并重建 LRU 索引
func (c *CacheService) setCacheDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("创建缓存目录失败", "path", dir, "error", err)
		return
	}
	c.cacheDir = dir
	c.lruMu.Lock()
	c.lruIndex = make(map[string]time.Time)
	c.lruMu.Unlock()
	c.loadLRUIndex()
	slog.Info("缓存目录已切换", "path", dir)
}
