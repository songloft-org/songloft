package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"songloft/internal/database"
	"songloft/internal/models"
	"songloft/internal/services/source"

	"github.com/hanxi/tag"
)

const (
	// autoConvertConfigKey 自动转换开关配置 key
	autoConvertConfigKey = "auto_convert_remote"
	// downloadInterval 手动批量转换时,触发新下载后的基础间隔
	defaultDownloadInterval = 3 * time.Second
	// downloadJitter 限速抖动上限
	defaultDownloadJitter = 2 * time.Second
	// maxFileNameBytes 文件名最大字节数(留余量给扩展名和冲突后缀)
	maxFileNameBytes = 180
	// maxConflictSuffix 冲突后缀上限
	maxConflictSuffix = 100
)

// 自动模式并发去重控制
const autoConvertWorkers = 2

var (
	// ErrAlreadyRunning 歌单已有转换任务在运行
	ErrAlreadyRunning = errors.New("convert task already running for this playlist")
	// ErrNotRemote 歌曲不是 remote 类型
	ErrNotRemote = errors.New("song is not a remote song")
	// ErrAutoDisabled 自动转换未开启
	ErrAutoDisabled = errors.New("auto convert is disabled")
	// ErrNoRemoteSongs 歌单中没有可转换的网络歌曲
	ErrNoRemoteSongs = errors.New("no remote songs to convert")
)

// AutoConvertConfig 自动转换配置
type AutoConvertConfig struct {
	Enabled bool `json:"enabled"`
}

// ConvertService 网络歌曲转本地歌曲服务
type ConvertService struct {
	db                *database.DB
	songService       *SongService
	playlistService   *PlaylistService
	cacheService      *CacheService
	configService     *ConfigService
	metadataExtractor *MetadataExtractor // 复用其 SaveCoverData,把网络封面落地到 cover_storage_path
	progressMgr       *ConvertProgressManager
	orchestrator      CacheSongFetcher // 注入后用于 fallback 模式下载;未注入时降级为纯外链路径

	// 自动模式并发控制
	autoSem      chan struct{} // worker 信号量
	autoInflight sync.Map      // map[int64]struct{},正在自动转换的 songID
	rng          *rand.Rand    // 限速 jitter 随机源
	rngMu        sync.Mutex
	musicPathFn  func() string // 获取最新 music_path 配置(响应配置变更)

	// 内部 HTTP 调用解析相对路径用
	urlResolver  *InternalURLResolver
	httpClient   *http.Client  // 自动跟随重定向,用于下载 JS 插件的 music/url 端点
	lyricFetcher *LyricFetcher // 解包插件 JSON 拿到 LRC 文本

	downloadInterval time.Duration
	downloadJitter   time.Duration
}

// NewConvertService 创建转换服务
func NewConvertService(
	db database.DB,
	songService *SongService,
	playlistService *PlaylistService,
	cacheService *CacheService,
	configService *ConfigService,
	metadataExtractor *MetadataExtractor,
	musicPathFn func() string,
	urlResolver *InternalURLResolver,
	lyricFetcher *LyricFetcher,
) *ConvertService {
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
		// 自动跟随重定向(默认 10 跳),用于完整走完 JS 插件→cache endpoint→真实 CDN 的链路
	}
	return &ConvertService{
		db:                &db,
		songService:       songService,
		playlistService:   playlistService,
		cacheService:      cacheService,
		configService:     configService,
		metadataExtractor: metadataExtractor,
		progressMgr:       NewConvertProgressManager(),
		autoSem:           make(chan struct{}, autoConvertWorkers),
		rng:               rand.New(rand.NewSource(time.Now().UnixNano())),
		musicPathFn:       musicPathFn,
		urlResolver:       urlResolver,
		httpClient:        httpClient,
		lyricFetcher:      lyricFetcher,
		downloadInterval:  defaultDownloadInterval,
		downloadJitter:    defaultDownloadJitter,
	}
}

// SetOrchestrator 注入 SourceOrchestrator,使 convertOne 可以走 L1/L2 fallback。
// 未注入时降级到旧的 fetchToTemp 路径(仅靠 song.URL 单源下载)。
func (c *ConvertService) SetOrchestrator(o CacheSongFetcher) {
	c.orchestrator = o
}

// resolveDownloadURL 解析下载用的绝对 URL
// 网络歌曲 URL 可能是 JS 插件的相对路径(如 /api/v1/jsplugin/...),
// 需要拼接本机 server 地址 + 内部 access_token
func (c *ConvertService) resolveDownloadURL(rawURL string) string {
	return c.urlResolver.Resolve(rawURL)
}

// IsAutoConvertEnabled 返回自动转换开关是否打开
func (c *ConvertService) IsAutoConvertEnabled() bool {
	var cfg AutoConvertConfig
	if err := c.configService.GetJSON(autoConvertConfigKey, &cfg); err != nil {
		return true
	}
	return cfg.Enabled
}

// SetAutoConvertEnabled 设置自动转换开关
func (c *ConvertService) SetAutoConvertEnabled(enabled bool) error {
	return c.configService.SetJSON(autoConvertConfigKey, AutoConvertConfig{Enabled: enabled})
}

// GetProgress 获取指定歌单的转换进度
func (c *ConvertService) GetProgress(playlistID int64) ConvertProgress {
	return c.progressMgr.GetProgress(playlistID)
}

// CancelConvert 取消指定歌单的转换
func (c *ConvertService) CancelConvert(playlistID int64) bool {
	return c.progressMgr.Cancel(playlistID)
}

// ConvertPlaylistToLocal 异步启动整歌单的转换任务
func (c *ConvertService) ConvertPlaylistToLocal(ctx context.Context, playlistID int64) error {
	playlist, err := (*c.db).PlaylistRepository().GetByID(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}
	if playlist.Type != models.PlaylistTypeNormal {
		return fmt.Errorf("playlist %d is not a normal playlist", playlistID)
	}

	songs, err := (*c.db).PlaylistSongRepository().GetSongs(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("failed to get playlist songs: %w", err)
	}

	remoteSongs := make([]*models.Song, 0, len(songs))
	for _, s := range songs {
		if s.Type == models.TypeRemote {
			remoteSongs = append(remoteSongs, s)
		}
	}
	if len(remoteSongs) == 0 {
		return ErrNoRemoteSongs
	}

	if !c.progressMgr.Start(playlistID, len(remoteSongs)) {
		return ErrAlreadyRunning
	}

	go c.runPlaylistConvert(playlistID, playlist.Name, remoteSongs)
	return nil
}

// runPlaylistConvert 手动批量转换的后台执行
func (c *ConvertService) runPlaylistConvert(playlistID int64, playlistName string, songs []*models.Song) {
	cancelCh := c.progressMgr.GetCancelChannel(playlistID)
	ctx := context.Background()

	for i, song := range songs {
		select {
		case <-cancelCh:
			c.progressMgr.SetCancelled(playlistID)
			return
		default:
		}

		c.progressMgr.UpdateCurrent(playlistID, song.Title)

		triggeredDownload, err := c.convertOne(ctx, playlistID, playlistName, song)
		if err != nil {
			if errors.Is(err, errSkipAlreadyLocal) {
				c.progressMgr.UpdateProgress(playlistID, song.Title, ConvertUpdateSkipped, "")
			} else {
				slog.Warn("convert song failed",
					"playlistId", playlistID,
					"songId", song.ID,
					"title", song.Title,
					"error", err)
				c.progressMgr.UpdateProgress(playlistID, song.Title,
					ConvertUpdateFailed,
					fmt.Sprintf("%s: %v", song.Title, err))
			}
		} else {
			c.progressMgr.UpdateProgress(playlistID, song.Title, ConvertUpdateConverted, "")
		}

		// 触发了新下载时,限速防风控(最后一首不需要等)
		if triggeredDownload && i < len(songs)-1 {
			c.progressMgr.SetWaiting(playlistID, true)
			if !c.sleepInterruptible(c.nextInterval(), cancelCh) {
				c.progressMgr.SetWaiting(playlistID, false)
				c.progressMgr.SetCancelled(playlistID)
				return
			}
			c.progressMgr.SetWaiting(playlistID, false)
		}
	}

	c.progressMgr.Complete(playlistID)
}

// nextInterval 计算下次下载间隔(基础值 + 随机抖动)
func (c *ConvertService) nextInterval() time.Duration {
	c.rngMu.Lock()
	defer c.rngMu.Unlock()
	if c.downloadJitter <= 0 {
		return c.downloadInterval
	}
	return c.downloadInterval + time.Duration(c.rng.Int63n(int64(c.downloadJitter)))
}

// sleepInterruptible 可中断的 sleep,返回 false 表示被取消
func (c *ConvertService) sleepInterruptible(d time.Duration, cancelCh <-chan struct{}) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-cancelCh:
		return false
	}
}

// errSkipAlreadyLocal 标记跳过非 remote 的歌曲
var errSkipAlreadyLocal = errors.New("skip non-remote song")

// convertOne 转换单首歌曲到本地,返回是否触发了新下载
func (c *ConvertService) convertOne(ctx context.Context, playlistID int64, playlistName string, song *models.Song) (bool, error) {
	if song.Type != models.TypeRemote {
		return false, errSkipAlreadyLocal
	}
	if song.URL == "" && !song.IsPluginSourced() {
		return false, fmt.Errorf("song has neither url nor plugin source")
	}

	musicPath := c.musicPathFn()
	if musicPath == "" {
		return false, fmt.Errorf("music_path is empty")
	}
	// 不要在这里 Abs:scanner.ScanFiles 直接以 MusicPath 配置值为起点拼接路径,
	// 现有 local 歌曲的 file_path 与 MusicPath 配置共用同一种格式(相对或绝对)。
	// 如果转换时存绝对路径而 scanner 给的是相对路径,重新扫描会因路径字符串不一致
	// 导致 ListLocalSongPaths 去重失败,重复 INSERT 同一首歌。

	// 1. 确定文件来源
	// 优先复用 cache_service 已缓存的文件;cache 未命中时,直接 HTTP GET
	// 走"插件 URL → 跟随 302 到 cache endpoint → 真实 CDN"的完整链路,
	// 把流落到临时文件供后续 copy。
	// 不能复用 cache_service.DownloadToCache:它会先设 inflight,
	// 然后 lxmusic 类插件检查 cache 时看到 inflight=200,误以为已缓存而
	// 返回 302 到 cache endpoint,downloadClient 跟随回来再等 inflight,
	// 形成自循环死锁。
	var (
		srcPath           string
		tmpDownloadPath   string
		fetchedExt        string
		triggeredDownload bool
	)
	if p, ok := c.cacheService.FindCachedFileBySong(song); ok {
		srcPath = p
	}
	if srcPath == "" {
		// 优先用 SourceOrchestrator(支持插件 L1 自搜 + 跨插件 L2 fallback);
		// 未注入或纯外链 song 时降级到旧的简化下载路径。
		if c.orchestrator != nil && song.IsPluginSourced() {
			res, err := c.orchestrator.Fetch(ctx, songInfoOf(song), source.ModeFallback)
			if err != nil {
				return true, fmt.Errorf("orchestrator fetch failed: %w", err)
			}
			tmpDownloadPath = res.TempPath
			fetchedExt = ""
			srcPath = res.TempPath
			triggeredDownload = true
		} else if song.URL != "" {
			downloadURL := c.resolveDownloadURL(song.URL)
			tmp, ext, err := c.fetchToTemp(ctx, downloadURL)
			if err != nil {
				return true, fmt.Errorf("download failed: %w", err)
			}
			tmpDownloadPath = tmp
			fetchedExt = ext
			srcPath = tmp
			triggeredDownload = true
		} else {
			return false, fmt.Errorf("song has no playable source")
		}
	}
	defer func() {
		if tmpDownloadPath != "" {
			_ = os.Remove(tmpDownloadPath)
		}
	}()

	// 2. 决定文件扩展名(优先级:fetched Content-Type > srcPath 已有 ext > URL > .mp3)
	ext := fetchedExt
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(srcPath))
	}
	if ext == "" {
		ext = extFromURL(song.URL)
	}
	if ext == "" {
		ext = ".mp3"
	}

	// 3. 生成目标路径(歌单名 / 艺术家 - 标题.ext)
	dstPath, err := c.resolveTargetPath(musicPath, playlistName, song.Artist, song.Title, ext)
	if err != nil {
		return triggeredDownload, fmt.Errorf("resolve target path: %w", err)
	}

	// 4. 创建目录并拷贝文件
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return triggeredDownload, fmt.Errorf("mkdir: %w", err)
	}
	written, err := copyFile(srcPath, dstPath)
	if err != nil {
		return triggeredDownload, fmt.Errorf("copy file: %w", err)
	}

	// 5. 解析网络歌词:lyric_source=url 时,GET URL 拿到 payload 后回填
	//    新 song 的 lyric_source 改为 cached,后续播放无需再去网络拉。
	//    存库的是 LyricPayload JSON 文本(保留 tlyric/rlyric/lxlyric 供未来渲染)。
	lyricStored := song.Lyric
	lyricSource := song.LyricSource
	if lyricSource == models.LyricSourceURL {
		if song.LyricRemoteURL == "" {
			lyricStored = ""
			lyricSource = ""
		} else if payload, err := c.lyricFetcher.Fetch(ctx, song.LyricRemoteURL); err != nil {
			slog.Warn("fetch lyric url failed,skip lyric for converted song",
				"url", song.LyricRemoteURL, "songId", song.ID, "error", err)
			lyricStored = ""
			lyricSource = ""
		} else if payload.IsEmpty() {
			lyricStored = ""
			lyricSource = ""
		} else {
			lyricStored = payload.MarshalString()
			lyricSource = models.LyricSourceCached
		}
	}

	// 5.5. 持久化封面:原 song 仅有 CoverURL 时,下载并落地到 cover_storage_path
	//      成功后 writeFileTags 会把它一起嵌入音频文件 tag,链接失效也不受影响
	//      失败仅 warn,新 song 退回到 CoverURL,前端仍可走 CDN
	coverPath := song.CoverPath
	if coverPath == "" && song.CoverURL != "" {
		if p, err := c.fetchAndSaveCover(ctx, song.CoverURL); err != nil {
			slog.Warn("fetch cover url failed,keep CoverURL",
				"url", song.CoverURL, "songId", song.ID, "error", err)
		} else if p != "" {
			coverPath = p
		}
	}

	// 6. 构造新的 local song(基于原 remote song 复制元数据)
	newSong := &models.Song{
		Type:        models.TypeLocal,
		Title:       song.Title,
		Artist:      song.Artist,
		Album:       song.Album,
		Duration:    song.Duration,
		FilePath:    dstPath,
		URL:         "",
		CoverPath:   coverPath,
		CoverURL:    song.CoverURL,
		Lyric:       lyricStored,
		LyricSource: lyricSource,
		FileSize:    written,
		Format:      strings.TrimPrefix(ext, "."),
		BitRate:     song.BitRate,
		SampleRate:  song.SampleRate,
		IsLive:      false,
		// 新本地 song 不再带 plugin_entry_path / source_data,纯本地化
	}

	// 6. 事务:CreateSong + ReplaceSongInPlaylist
	// 必须复用同一个事务,避免内层另开事务导致 SQLITE_BUSY
	if err := (*c.db).RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		if err := uow.Songs.Create(ctx, newSong); err != nil {
			return fmt.Errorf("create local song: %w", err)
		}
		if err := uow.PlaylistSongs.ReplaceSong(ctx, playlistID, song.ID, newSong.ID); err != nil {
			return fmt.Errorf("replace song in playlist: %w", err)
		}
		return nil
	}); err != nil {
		_ = os.Remove(dstPath)
		return triggeredDownload, err
	}

	slog.Info("convert remote song to local",
		"playlistId", playlistID,
		"oldSongId", song.ID,
		"newSongId", newSong.ID,
		"dstPath", dstPath)

	// 写入元数据 tag 到本地文件(失败不影响主流程,只记 warning)
	// MP3 / FLAC 已支持;M4A / OGG 返回 ErrUnsupportedWrite,会被静默忽略
	// 传 newSong:它的 Lyric 已经是解析后的文本(原 URL 来源也已转 cached)
	c.writeFileTags(dstPath, newSong)

	// 孤儿清理:若原 remote song 已不再被任何歌单引用,删除它
	// 注意直接调用 DB.DeleteSong(而非 SongService.Delete),避免 cover_path 文件被清理
	// —— 新 local song 共享同一份 cover_path
	songsRepo := (*c.db).SongRepository()
	if cnt, err := songsRepo.CountPlaylistsContaining(ctx, song.ID); err == nil && cnt == 0 {
		if err := songsRepo.Delete(ctx, song.ID); err != nil {
			slog.Warn("delete orphan remote song failed",
				"songId", song.ID,
				"error", err)
		} else {
			slog.Info("orphan remote song deleted",
				"songId", song.ID,
				"title", song.Title)
			// 同步清掉该 songID 名下的所有 cache 残留(可能含历史多个 key)。
			// 不清的话,如果未来 DB 重置导致 ID 复用,旧 cache 会被新 song 误命中
			// —— 这正是本次 bug 的根因。
			if err := c.cacheService.EvictSong(song.ID); err != nil {
				slog.Warn("evict orphan remote song cache failed",
					"songId", song.ID, "error", err)
			}
		}
	}

	return triggeredDownload, nil
}

// OnCacheDownloaded 缓存下载完成回调(自动模式入口)。
// 新签名按 song.ID 触发,由 SourceOrchestrator 在 Fetch 成功后调用。
func (c *ConvertService) OnCacheDownloaded(songID int64, filePath string) {
	if !c.IsAutoConvertEnabled() {
		return
	}
	ctx := context.Background()
	song, err := (*c.db).SongRepository().GetByID(ctx, songID)
	if err != nil || song == nil || song.Type != models.TypeRemote {
		return
	}

	// 跨调用去重:同一首歌正在自动转换中则跳过
	if _, loaded := c.autoInflight.LoadOrStore(song.ID, struct{}{}); loaded {
		return
	}

	go func() {
		defer c.autoInflight.Delete(song.ID)
		// worker 限制
		c.autoSem <- struct{}{}
		defer func() { <-c.autoSem }()

		playlistIDs, err := (*c.db).PlaylistSongRepository().ListPlaylistsContainingSong(ctx, song.ID)
		if err != nil {
			slog.Warn("auto convert: list playlists failed", "songId", song.ID, "error", err)
			return
		}
		for _, pid := range playlistIDs {
			// 手动转换正在跑该歌单时,跳过自动避免冲突
			if c.progressMgr.IsRunning(pid) {
				continue
			}
			playlist, err := (*c.db).PlaylistRepository().GetByID(ctx, pid)
			if err != nil {
				continue
			}
			// 重新读取 song,可能已被其他歌单的转换替换
			fresh, err := (*c.db).SongRepository().GetByID(ctx, song.ID)
			if err != nil || fresh.Type != models.TypeRemote {
				continue
			}
			if _, err := c.convertOne(ctx, pid, playlist.Name, fresh); err != nil && !errors.Is(err, errSkipAlreadyLocal) {
				slog.Warn("auto convert one failed",
					"playlistId", pid,
					"songId", song.ID,
					"error", err)
			}
		}
	}()
}

// resolveTargetPath 生成目标文件路径,处理同名冲突
func (c *ConvertService) resolveTargetPath(musicPath, playlistName, artist, title, ext string) (string, error) {
	dirName := sanitizeFileName(playlistName)
	if dirName == "" {
		dirName = "untitled_playlist"
	}

	baseName := buildFileBase(artist, title)
	if baseName == "" {
		baseName = "untitled"
	}

	dir := filepath.Join(musicPath, dirName)

	primary := filepath.Join(dir, baseName+ext)
	if _, err := os.Stat(primary); os.IsNotExist(err) {
		return primary, nil
	} else if err != nil && !os.IsNotExist(err) {
		// 其他错误(权限等),仍尝试主路径
	}

	for i := 2; i <= maxConflictSuffix; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", baseName, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("too many file name conflicts for %s", baseName)
}

// buildFileBase 拼接 "艺术家 - 标题",空艺术家时只用标题
func buildFileBase(artist, title string) string {
	a := sanitizeFileName(artist)
	t := sanitizeFileName(title)
	if a == "" {
		return t
	}
	if t == "" {
		return a
	}
	combined := a + " - " + t
	if utf8.RuneCountInString(combined) > 0 && len(combined) > maxFileNameBytes {
		return truncateUTF8(combined, maxFileNameBytes)
	}
	return combined
}

// sanitizeFileName 清理跨平台不安全的字符
func sanitizeFileName(s string) string {
	if s == "" {
		return ""
	}
	// 替换非法字符为下划线
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			b.WriteRune('_')
		default:
			if unicode.IsControl(r) {
				b.WriteRune('_')
			} else {
				b.WriteRune(r)
			}
		}
	}
	out := b.String()
	// 去除首尾空格和点(Windows 不允许文件名以点或空格结尾)
	out = strings.TrimSpace(out)
	out = strings.Trim(out, ".")
	out = strings.TrimSpace(out)
	// Windows 保留名替换
	upper := strings.ToUpper(out)
	if isWindowsReservedName(upper) {
		out = "_" + out
	}
	// 限长
	if len(out) > maxFileNameBytes {
		out = truncateUTF8(out, maxFileNameBytes)
	}
	return out
}

// isWindowsReservedName 检查是否为 Windows 保留名(CON/PRN/AUX/NUL/COM1-9/LPT1-9)
func isWindowsReservedName(upper string) bool {
	switch upper {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) {
		c := upper[3]
		if c >= '1' && c <= '9' {
			return true
		}
	}
	return false
}

// truncateUTF8 按字节数截断 UTF-8 字符串,确保不切到字符中间
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i]
		}
	}
	return s[:maxBytes]
}

// writeFileTags 把 song 的元数据写入到本地文件(MP3 / FLAC 等)。
// 失败只记 warning,不影响主转换流程。
func (c *ConvertService) writeFileTags(filePath string, song *models.Song) {
	// song.Lyric 是 LyricPayload JSON;tag 只能写纯 LRC 文本,取主歌词字段
	mainLyric := models.UnmarshalLyric(song.Lyric).Lyric

	opts := tag.WriteOptions{
		Title:       song.Title,
		Artist:      song.Artist,
		AlbumArtist: song.Artist, // 大多数情况下专辑艺术家与艺术家一致
		Album:       song.Album,
		Lyrics:      mainLyric,
	}

	// 解析 added_at 年份作为发行年(网络歌曲的 Song 模型没有专门的 year 字段,
	// 使用 added_at 是个保守的兜底;如果 song 有 raw metadata 含 year,后续可扩展)
	if !song.AddedAt.IsZero() {
		opts.Year = song.AddedAt.Year()
	}

	// 防御性处理:如果传入 song 的 LyricSource 还是 url(理论上不应该,
	// convertOne 已经把 URL 歌词转 cached),清空避免把 URL 当歌词写入
	if song.LyricSource == models.LyricSourceURL {
		opts.Lyrics = ""
	}

	// 读取封面(优先 cover_path 本地文件)
	if song.CoverPath != "" {
		if data, err := os.ReadFile(song.CoverPath); err == nil {
			opts.Picture = &tag.Picture{
				MIMEType:    tag.MIMETypeFromExt(filepath.Ext(song.CoverPath)),
				Data:        data,
				Description: "",
			}
		} else {
			slog.Debug("read cover failed,skip embedding",
				"coverPath", song.CoverPath, "error", err)
		}
	}

	if err := tag.WriteTag(filePath, opts); err != nil {
		if errors.Is(err, tag.ErrUnsupportedWrite) {
			slog.Debug("tag write skipped for unsupported format",
				"path", filePath, "error", err)
			return
		}
		slog.Warn("write tag failed",
			"path", filePath, "error", err)
		return
	}
	slog.Debug("tag written", "path", filePath,
		"title", opts.Title, "artist", opts.Artist,
		"hasPicture", opts.Picture != nil,
		"lyricsLen", len(opts.Lyrics))
}

// fetchToTemp 通过 HTTP GET 把 URL 内容下载到系统临时目录,返回临时路径和 Content-Type 推断的扩展名
//
// 自动跟随重定向,适用于"JS 插件 music/url 端点 → 302 to cache endpoint → 真实 CDN"
// 这类多跳链路。响应必须是音频类型(audio/* 或 application/octet-stream)。
func (c *ConvertService) fetchToTemp(ctx context.Context, downloadURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isAudioContentType(contentType) {
		return "", "", fmt.Errorf("非音频响应 Content-Type: %s", contentType)
	}

	ext := getExtFromContentType(contentType)

	tmpFile, err := os.CreateTemp("", "mimusic-convert-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	written, err := io.Copy(tmpFile, resp.Body)
	closeErr := tmpFile.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("write temp file: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("close temp file: %w", closeErr)
	}
	if written < MinAudioSize {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("下载内容过小: %d bytes", written)
	}

	return tmpPath, ext, nil
}

// fetchAndSaveCover 下载封面 URL 并持久化到 cover_storage_path,返回存储后的本地路径。
//
// 相对路径会自动拼成本机 server + access_token(同 resolveDownloadURL),
// 用于 JS 插件代理的封面 URL。响应必须是 image/* Content-Type,
// 大小限制 10 MB 防异常响应耗尽内存。
// 所有失败都返回 (空字符串, error),调用方应作为非致命错误处理(保留原 CoverURL)。
func (c *ConvertService) fetchAndSaveCover(ctx context.Context, coverURL string) (string, error) {
	if c.metadataExtractor == nil {
		return "", fmt.Errorf("metadata extractor not configured")
	}

	resolved := c.resolveDownloadURL(coverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "image/*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", fmt.Errorf("非图片响应 Content-Type: %s", contentType)
	}

	// 限制读取 10 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty cover body")
	}

	ext := extFromImageContentType(contentType)
	if ext == "" {
		ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(coverURL)), ".")
	}
	if ext == "" {
		ext = "jpg"
	}

	return c.metadataExtractor.SaveCoverData(data, ext)
}

// extFromImageContentType 从 image/* Content-Type 推断文件扩展名(不含点)。
// 无法识别时返回空字符串,由调用方做兜底。
func extFromImageContentType(contentType string) string {
	ct := strings.ToLower(contentType)
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	switch ct {
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/bmp":
		return "bmp"
	}
	return ""
}

// extFromURL 从 URL 推断文件扩展名
func extFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	switch ext {
	case ".mp3", ".flac", ".wav", ".ape", ".ogg", ".m4a", ".wma", ".aac":
		return ext
	}
	return ""
}

// copyFile 将 src 拷贝到 dst,返回写入字节数
func copyFile(src, dst string) (int64, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("open src: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("create dst: %w", err)
	}
	written, err := io.Copy(dstFile, srcFile)
	if cerr := dstFile.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(dst)
		return 0, err
	}
	return written, nil
}
