package services

import (
	"container/heap"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// MinAudioSize 最小有效音频文件大小（1KB），低于此值认为是错误响应
	MinAudioSize = 1024
	// maxRedirects 最大重定向层数
	maxRedirects = 5
	// defaultMaxCacheSize 默认最大缓存大小（1GB）
	defaultMaxCacheSize int64 = 1 * 1024 * 1024 * 1024
	// cacheConfigKey 缓存配置在 configs 表中的 key
	cacheConfigKey = "music_cache_config"
	// maxErrorBodySize 限制非音频响应读取大小，防止内存爆炸
	maxErrorBodySize = 1024 * 1024 // 1MB
)

// CacheStats 缓存统计信息
type CacheStats struct {
	TotalSize int64 `json:"total_size"` // 总大小（字节）
	FileCount int   `json:"file_count"` // 文件数量
	MaxSize   int64 `json:"max_size"`   // 最大缓存大小（字节），0 表示无限制
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxSize int64 `json:"max_size"` // 最大缓存大小（字节），0 表示无限制
}

// inflightDownload 追踪正在进行的下载
type inflightDownload struct {
	done chan struct{} // 下载完成时关闭
	err  error         // 下载结果，在 close(done) 前设置
}

// CacheService 音乐缓存服务
type CacheService struct {
	cacheDir           string
	configService      *ConfigService
	client             *http.Client // 手动处理重定向(用于 resolveRedirects)
	downloadClient     *http.Client // 自动跟随重定向(用于 doDownload 的 GET 阶段)
	mu                 sync.Mutex
	inflight           map[string]*inflightDownload
	onDownloadComplete func(hash string, filePath string) // 下载完成回调
	lruIndex           map[string]time.Time               // LRU 访问时间索引（hash -> lastAccess）
	lruMu              sync.RWMutex                       // LRU 索引的读写锁
	orchestrator       CacheSongFetcher                   // 下载编排器(按 song.ID),由 app.go 注入;未注入时旧 hash 路径不受影响
}

// SetOnDownloadComplete 注册下载完成回调
func (c *CacheService) SetOnDownloadComplete(fn func(hash, filePath string)) {
	c.onDownloadComplete = fn
}

// NewCacheService 创建缓存服务
func NewCacheService(cacheDir string, configService *ConfigService) *CacheService {
	cs := &CacheService{
		cacheDir:      cacheDir,
		configService: configService,
		inflight:      make(map[string]*inflightDownload),
		lruIndex:      make(map[string]time.Time),
		client: &http.Client{
			Timeout: 120 * time.Second,
			// 禁用自动重定向，手动处理(用于 resolveRedirects 探测每跳真实 URL)
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		// downloadClient 自动跟随重定向(默认最多 10 跳),
		// 用于 GET 实际拉取音频流。兼容 JS 插件 /api/v1/jsplugin/.../music/url/{hash}
		// 这类返回 302 重定向到真实 CDN URL 的端点。
		downloadClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
	// 启动时从文件系统加载 LRU 索引
	cs.loadLRUIndex()
	return cs
}

// getCacheDir 根据 hash 生成缓存目录路径
// 将 hash 分割为两级目录：前 2 位 / 第 3-4 位（与封面目录拆分方式一致）
// 例如：5d7677b2ca1b8b1f -> {cache_root}/5d/76/
func (c *CacheService) getCacheDir(hash string) string {
	if len(hash) < 4 {
		// hash 长度不足，直接使用 hash 作为目录
		return filepath.Join(c.cacheDir, hash)
	}
	first := hash[:2]
	second := hash[2:4]
	return filepath.Join(c.cacheDir, first, second)
}

// FindCachedFile 查找缓存文件
// 返回文件路径和是否存在
// 命中缓存时自动更新 LRU 访问时间
func (c *CacheService) FindCachedFile(hash string) (string, bool) {
	dir := c.getCacheDir(hash)

	// 检查目录是否存在
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	// 查找以 hash 为前缀的文件
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 检查文件名是否以 hash 开头（排除 .tmp 临时文件）
		if strings.HasPrefix(name, hash) && !strings.HasSuffix(name, ".tmp") {
			filePath := filepath.Join(dir, name)
			// 命中缓存时更新 LRU 访问时间
			c.TouchCache(hash, filePath)
			return filePath, true
		}
	}

	return "", false
}

// IsDownloading 检查指定 hash 是否正在下载中
func (c *CacheService) IsDownloading(hash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.inflight[hash]
	return ok
}

// WaitForDownload 等待指定 hash 的下载完成
// 返回下载错误（如果有），nil 表示下载成功或没有正在进行的下载
func (c *CacheService) WaitForDownload(hash string) error {
	c.mu.Lock()
	dl, ok := c.inflight[hash]
	c.mu.Unlock()
	if !ok {
		return nil
	}
	<-dl.done
	return dl.err
}

// DownloadToCache 下载音乐文件到本地缓存
// 使用 context.Background() 确保客户端断开后下载仍继续
// 内部处理并发：如果同一 hash 已有下载在进行，等待已有下载完成
func (c *CacheService) DownloadToCache(hash, rawURL string) error {
	slog.Info("开始下载到缓存", "hash", hash, "url", rawURL)

	// 检查是否已有下载在进行
	c.mu.Lock()
	if dl, ok := c.inflight[hash]; ok {
		c.mu.Unlock()
		slog.Info("等待已有下载完成", "hash", hash)
		<-dl.done
		return dl.err
	}

	// 注册 inflight
	dl := &inflightDownload{done: make(chan struct{})}
	c.inflight[hash] = dl
	c.mu.Unlock()

	// defer 清理
	defer func() {
		c.mu.Lock()
		close(dl.done)
		delete(c.inflight, hash)
		c.mu.Unlock()
	}()

	// 执行下载
	err := c.doDownload(hash, rawURL)
	dl.err = err
	return err
}

// doDownload 执行实际的下载操作
func (c *CacheService) doDownload(hash, rawURL string) error {
	// 1. 解析重定向获取真实 URL
	realURL, err := c.resolveRedirects(rawURL)
	if err != nil {
		slog.Warn("解析重定向失败", "url", rawURL, "error", err)
		return err
	}
	if realURL != rawURL {
		slog.Info("重定向解析完成", "originalUrl", rawURL, "realUrl", realURL)
	}

	// 2. 创建 HTTP 请求（使用 context.Background 确保客户端断开后继续下载）
	req, err := http.NewRequest(http.MethodGet, realURL, nil)
	if err != nil {
		slog.Error("创建 HTTP 请求失败", "error", err)
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// 3. 发起请求(使用 downloadClient 自动跟随重定向,兼容 JS 插件 302 跳转的端点)
	resp, err := c.downloadClient.Do(req)
	if err != nil {
		slog.Error("下载音乐文件失败", "error", err)
		return err
	}
	defer resp.Body.Close()

	// 4. 检查状态码
	if resp.StatusCode != http.StatusOK {
		slog.Error("下载音乐文件失败", "statusCode", resp.StatusCode)
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 5. 检查 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !isAudioContentType(contentType) {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		slog.Warn("下载返回非音频内容，可能是错误响应",
			"hash", hash,
			"contentType", contentType,
			"bodyLen", len(body),
			"body", string(body))
		return fmt.Errorf("下载返回非音频内容 (Content-Type: %s, body: %s)", contentType, string(body))
	}

	ext := getExtFromContentType(contentType)

	// 6. 创建缓存目录和临时文件
	cacheDir := c.getCacheDir(hash)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		slog.Warn("创建缓存目录失败", "dir", cacheDir, "error", err)
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}

	tempPath := filepath.Join(cacheDir, hash+".tmp")
	finalPath := filepath.Join(cacheDir, hash+ext)

	tmpFile, err := os.Create(tempPath)
	if err != nil {
		slog.Error("创建临时文件失败", "path", tempPath, "error", err)
		return fmt.Errorf("创建临时文件失败: %w", err)
	}

	// 7. 下载到临时文件
	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tempPath)
		slog.Error("下载写入失败", "hash", hash, "error", err)
		return fmt.Errorf("下载写入失败: %w", err)
	}

	// 必须在 Rename 前关闭文件句柄，Windows 不允许对打开的文件进行 rename
	if err := tmpFile.Close(); err != nil {
		os.Remove(tempPath)
		slog.Error("关闭临时文件失败", "path", tempPath, "error", err)
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	// 8. 验证文件大小
	if written < MinAudioSize {
		os.Remove(tempPath)
		slog.Warn("下载内容过小，删除临时文件", "hash", hash, "size", written)
		return fmt.Errorf("下载内容过小: %d bytes", written)
	}

	// 9. 移动为正式文件
	// Windows 上如果目标文件已存在，os.Rename 会失败，因此先删除可能存在的旧文件
	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Remove(finalPath); err != nil {
			slog.Warn("删除已存在的缓存文件失败", "path", finalPath, "error", err)
		}
	}
	if err := moveFile(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		slog.Error("移动缓存文件失败", "from", tempPath, "to", finalPath, "error", err)
		return fmt.Errorf("移动缓存文件失败: %w", err)
	}

	slog.Info("下载缓存完成", "path", finalPath, "size", written)

	// 更新 LRU 索引
	c.TouchCache(hash, finalPath)

	// 下载完成后检查是否需要 LRU 淘汰
	c.EvictLRU()

	// 下载成功，触发回调（异步执行，不阻塞返回）
	if c.onDownloadComplete != nil {
		go c.onDownloadComplete(hash, finalPath)
	}

	return nil
}

// resolveRedirects 使用 HEAD 请求解析 URL 重定向，返回最终真实 URL
// 最多跟随 5 层重定向，超限则返回最后一个 URL
//
// 容错策略：HEAD 请求失败或返回 4xx/5xx 时静默降级,返回当前 URL,
// 让后续 GET 请求直接尝试。这是为了兼容某些只支持 GET 的端点(例如
// 部分 JS 插件转发的内部 API 不响应 HEAD)。GET 阶段若真的失败,
// 会通过 Content-Type 校验和状态码检查报出真实错误。
func (c *CacheService) resolveRedirects(rawURL string) (string, error) {
	currentURL := rawURL

	for depth := 0; depth < maxRedirects; depth++ {
		req, err := http.NewRequest(http.MethodHead, currentURL, nil)
		if err != nil {
			return "", fmt.Errorf("create HEAD request: %w", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := c.client.Do(req)
		if err != nil {
			// HEAD 网络错误时降级:返回当前 URL,让 GET 直接尝试
			slog.Debug("resolveRedirects: HEAD 请求失败,降级到 GET",
				"url", currentURL, "error", err)
			return currentURL, nil
		}
		resp.Body.Close()

		slog.Debug("resolveRedirects: HEAD 响应",
			"depth", depth,
			"url", currentURL,
			"statusCode", resp.StatusCode,
			"contentType", resp.Header.Get("Content-Type"),
			"location", resp.Header.Get("Location"))

		// 检查重定向
		switch resp.StatusCode {
		case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
			http.StatusTemporaryRedirect, http.StatusPermanentRedirect:

			location := resp.Header.Get("Location")
			if location == "" {
				return "", fmt.Errorf("redirect without Location header")
			}
			// 处理相对路径
			if !strings.HasPrefix(location, "http") {
				idx := strings.Index(currentURL[8:], "/") // 跳过 https://
				if idx > 0 {
					location = currentURL[:8+idx] + location
				}
			}
			slog.Debug("resolveRedirects: 跟随重定向", "from", currentURL, "to", location)
			currentURL = location
			continue

		default:
			// 非重定向状态:4xx/5xx 时降级,让 GET 直接尝试
			// (部分端点不支持 HEAD 但 GET 可用,例如 JS 插件转发的内部 API)
			if resp.StatusCode >= 400 {
				slog.Debug("resolveRedirects: HEAD 返回错误状态,降级到 GET",
					"url", currentURL, "statusCode", resp.StatusCode)
				return currentURL, nil
			}
			// 200 或其他成功状态码，返回当前 URL
			return currentURL, nil
		}
	}

	// 重定向层数超限，返回最后的 URL
	return currentURL, nil
}

// isAudioContentType 检查 Content-Type 是否为音频类型
func isAudioContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "audio/") ||
		strings.Contains(ct, "video/mp4") ||
		strings.Contains(ct, "application/octet-stream")
}

// PrefetchToCache 在后台异步下载音乐文件到缓存，立即返回
// 如果文件已缓存或正在下载，则不做任何操作
func (c *CacheService) PrefetchToCache(hash, url string) {
	// 已缓存
	if _, found := c.FindCachedFile(hash); found {
		slog.Info("预加载：文件已缓存", "hash", hash)
		return
	}
	// 已在下载中
	if c.IsDownloading(hash) {
		slog.Info("预加载：文件正在下载中", "hash", hash)
		return
	}
	// 启动后台下载
	go func() {
		slog.Info("预加载：启动后台下载", "hash", hash)
		if err := c.DownloadToCache(hash, url); err != nil {
			slog.Warn("预加载下载失败", "hash", hash, "error", err)
		} else {
			slog.Info("预加载下载完成", "hash", hash)
		}
	}()
}

// getExtFromContentType 根据 Content-Type 获取文件扩展名
func getExtFromContentType(contentType string) string {
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

// TouchCache 更新缓存文件的最后访问时间（内存索引 + 文件 mtime）
func (c *CacheService) TouchCache(hash string, filePath string) {
	now := time.Now()

	// 更新内存索引
	c.lruMu.Lock()
	c.lruIndex[hash] = now
	c.lruMu.Unlock()

	// 更新文件 mtime
	if err := os.Chtimes(filePath, now, now); err != nil {
		slog.Debug("更新缓存文件 mtime 失败", "hash", hash, "error", err)
	}
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

func (h lruMaxHeap) Len() int            { return len(h) }
func (h lruMaxHeap) Less(i, j int) bool  { return h[i].lastAccess.After(h[j].lastAccess) }
func (h lruMaxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *lruMaxHeap) Push(x interface{}) { *h = append(*h, x.(lruEntry)) }
func (h *lruMaxHeap) Pop() interface{} {
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

// UpdateCacheConfig 更新缓存配置
func (c *CacheService) UpdateCacheConfig(cfg CacheConfig) error {
	if err := c.configService.SetJSON(cacheConfigKey, cfg); err != nil {
		return fmt.Errorf("更新缓存配置失败: %w", err)
	}
	// 配置更新后立即检查是否需要淘汰
	go c.EvictLRU()
	return nil
}
