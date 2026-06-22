package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"songloft/internal/database"
	"songloft/internal/httputil"
	"songloft/internal/models"
)

// SongRepository 是 SongService 依赖的歌曲仓储接口。
type SongRepository interface {
	GetByID(ctx context.Context, id int64) (*models.Song, error)
	Create(ctx context.Context, song *models.Song) error
	Update(ctx context.Context, song *models.Song) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter *database.SongFilter) ([]*models.Song, error)
	ListIDs(ctx context.Context, filter *database.SongFilter) ([]int64, error)
	Count(ctx context.Context, filter *database.SongFilter) (int64, error)
	BatchDelete(ctx context.Context, ids []int64) (int, error)
	BatchCreate(ctx context.Context, songs []*models.Song) error
	UpsertRemote(ctx context.Context, song *models.Song) error
	UpdateLyrics(ctx context.Context, id int64, lyric, lyricSource, lyricRemoteURL string) error
	UpdateDuration(ctx context.Context, id int64, duration float64) error
	UpdateSource(ctx context.Context, id int64, pluginEntryPath, sourceData string) error
	ListLocalPaths(ctx context.Context) (map[string]database.LocalPathInfo, error)
	ListTypesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
	CountCoverPathReferences(ctx context.Context, coverPath string) (int, error)
	UpdateFingerprint(ctx context.Context, id int64, fingerprint string, duration float64) error
	ClearAllFingerprints(ctx context.Context) error
	ListLocalWithoutFingerprint(ctx context.Context) ([]database.SongIDPath, error)
	CountLocalFingerprints(ctx context.Context) (total, computed int64, err error)
	ListDuplicateGroups(ctx context.Context) ([]database.DuplicateGroup, error)
}

// Transactor 提供 UnitOfWork 事务执行入口，
// 让批量扫描在单一事务里操作 SongRepository。
type Transactor interface {
	RunInTx(ctx context.Context, fn func(context.Context, *database.UnitOfWork) error) error
}

// PlaylistAutoCreator 由扫描完成后调用，重建 auto_created 歌单。
type PlaylistAutoCreator interface {
	AutoCreate(ctx context.Context, includeSubdirs bool, excludeDirs []string) (*models.AutoCreatePlaylistsResponse, error)
}

// SongService 歌曲服务
type SongService struct {
	songs               SongRepository
	tx                  Transactor
	metadataExtractor   *MetadataExtractor
	scanner             *Scanner
	scanProgressManager *ScanProgressManager
	configService       *ConfigService
	playlistAutoCreator PlaylistAutoCreator
	cacheService        *CacheService       // 可选;由 app.go 通过 SetCacheService 注入,Delete 时清理 cache 残留
	fingerprintService  *FingerprintService // 可选;扫描完成后自动计算指纹
}

// NewSongService 创建歌曲服务
func NewSongService(
	songs SongRepository,
	tx Transactor,
	metadataExtractor *MetadataExtractor,
	scanner *Scanner,
	configService *ConfigService,
	playlistAutoCreator PlaylistAutoCreator,
) *SongService {
	return &SongService{
		songs:               songs,
		tx:                  tx,
		metadataExtractor:   metadataExtractor,
		scanner:             scanner,
		scanProgressManager: NewScanProgressManager(),
		configService:       configService,
		playlistAutoCreator: playlistAutoCreator,
	}
}

// SetScanner 更新扫描器引用（配置变更时调用）
func (s *SongService) SetScanner(scanner *Scanner) {
	s.scanner = scanner
}

// SetCacheService 注入 cache 服务,使 Delete/BatchDelete 能联动清理 cache 文件。
// 避免歌曲被删后 cache 残留,在 DB 重置/ID 复用场景下被新 song 误命中。
func (s *SongService) SetCacheService(cs *CacheService) {
	s.cacheService = cs
}

// SetFingerprintService 注入指纹服务，扫描完成后自动计算缺失指纹。
func (s *SongService) SetFingerprintService(fs *FingerprintService) {
	s.fingerprintService = fs
}

// GetScanProgress 获取扫描进度
func (s *SongService) GetScanProgress() ScanProgress {
	return s.scanProgressManager.GetProgress()
}

// CancelScan 取消扫描
func (s *SongService) CancelScan() bool {
	return s.scanProgressManager.Cancel()
}

// CountLocalFingerprints 返回本地歌曲总数和已计算指纹数。
func (s *SongService) CountLocalFingerprints(ctx context.Context) (total, computed int64, err error) {
	return s.songs.CountLocalFingerprints(ctx)
}

// GetDuplicateGroups 查询所有指纹重复的本地歌曲组。
func (s *SongService) GetDuplicateGroups(ctx context.Context) ([]database.DuplicateGroup, error) {
	return s.songs.ListDuplicateGroups(ctx)
}

// GetByID 根据 ID 获取歌曲
func (s *SongService) GetByID(ctx context.Context, id int64) (*models.Song, error) {
	song, err := s.songs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get song: %w", err)
	}
	return song, nil
}

// Update 更新歌曲
func (s *SongService) Update(ctx context.Context, song *models.Song) error {
	if err := song.Validate(); err != nil {
		return fmt.Errorf("invalid song data: %w", err)
	}
	if err := s.songs.Update(ctx, song); err != nil {
		return fmt.Errorf("failed to update song: %w", err)
	}
	return nil
}

// Delete 删除歌曲
func (s *SongService) Delete(ctx context.Context, id int64) error {
	song, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get song: %w", err)
	}

	if err := s.songs.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete song: %w", err)
	}

	if song != nil && song.CoverPath != "" {
		removeCoverIfUnreferenced(ctx, s.songs, song.CoverPath)
	}
	if s.cacheService != nil {
		cachePath := ""
		if song != nil {
			cachePath = song.CachePath
		}
		if err := s.cacheService.EvictSong(id, cachePath); err != nil {
			slog.Warn("evict cache after song delete failed", "songId", id, "error", err)
		}
	}
	return nil
}

// BatchDelete 批量删除歌曲。deleteFiles 为 true 时同步删除本地音频文件。
func (s *SongService) BatchDelete(ctx context.Context, ids []int64, deleteFiles bool) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	coverPathSet := make(map[string]struct{})
	cachePaths := make(map[int64]string)
	var filePaths []string
	for _, id := range ids {
		song, err := s.GetByID(ctx, id)
		if err != nil || song == nil {
			continue
		}
		if song.CoverPath != "" {
			coverPathSet[song.CoverPath] = struct{}{}
		}
		if song.CachePath != "" {
			cachePaths[id] = song.CachePath
		}
		if deleteFiles && song.Type == models.TypeLocal && song.FilePath != "" {
			filePaths = append(filePaths, song.FilePath)
		}
	}

	deleted, err := s.songs.BatchDelete(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("failed to batch delete songs: %w", err)
	}

	for coverPath := range coverPathSet {
		removeCoverIfUnreferenced(ctx, s.songs, coverPath)
	}
	for _, fp := range filePaths {
		if err := os.Remove(fp); err != nil {
			slog.Warn("删除音频文件失败", "path", fp, "error", err)
		} else {
			slog.Info("已删除音频文件", "path", fp)
		}
	}
	if s.cacheService != nil {
		for _, id := range ids {
			if err := s.cacheService.EvictSong(id, cachePaths[id]); err != nil {
				slog.Warn("evict cache after batch delete failed", "songId", id, "error", err)
			}
		}
	}
	return deleted, nil
}

// coverReferenceCounter 让 removeCoverIfUnreferenced 既能被 SongRepository
// 又能被 playlist 侧的实现接入，避免直接耦合 *database.SongRepository。
type coverReferenceCounter interface {
	CountCoverPathReferences(ctx context.Context, coverPath string) (int, error)
}

// removeCoverIfUnreferenced 仅在封面不再被任何 song/playlist 引用时,从磁盘删除。
// 封面按内容哈希分层存盘,多处共享同一物理文件 —— 不做引用计数会误删他人封面。
// 任何查询失败都保守跳过删除(宁可残留也不要误删)。
// 设计为 package-level helper,song / playlist service 共用。
func removeCoverIfUnreferenced(ctx context.Context, counter coverReferenceCounter, coverPath string) {
	if coverPath == "" {
		return
	}
	refs, err := counter.CountCoverPathReferences(ctx, coverPath)
	if err != nil {
		slog.Warn("查询封面引用计数失败,跳过删除", "coverPath", coverPath, "error", err)
		return
	}
	if refs > 0 {
		slog.Debug("封面仍被引用,跳过删除", "coverPath", coverPath, "refs", refs)
		return
	}
	if err := os.Remove(coverPath); err != nil {
		slog.Warn("删除封面文件失败", "coverPath", coverPath, "error", err)
		return
	}
	slog.Info("封面文件已删除", "coverPath", coverPath)
}

// List 列出歌曲
func (s *SongService) List(ctx context.Context, filter *database.SongFilter) ([]*models.Song, error) {
	songs, err := s.songs.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list songs: %w", err)
	}
	return songs, nil
}

// Search 搜索歌曲
func (s *SongService) Search(ctx context.Context, keyword string, songType string, limit, offset int) ([]*models.Song, error) {
	filter := &database.SongFilter{
		Keyword: keyword,
		Type:    songType,
		Limit:   limit,
		Offset:  offset,
		OrderBy: "added_at",
		Order:   "DESC",
	}
	return s.List(ctx, filter)
}

// Count 统计歌曲数量
func (s *SongService) Count(ctx context.Context, filter *database.SongFilter) (int64, error) {
	count, err := s.songs.Count(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count songs: %w", err)
	}
	return count, nil
}

// ListIDs 仅返回匹配 filter 的歌曲 ID 列表，无分页。
// 用于前端「全选当前筛选范围」一次性拿到所有匹配 id，避免拉完整 song 对象。
func (s *SongService) ListIDs(ctx context.Context, filter *database.SongFilter) ([]int64, error) {
	ids, err := s.songs.ListIDs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list song ids: %w", err)
	}
	return ids, nil
}

// ScanAndImportAsync 异步扫描并导入本地音乐文件
func (s *SongService) ScanAndImportAsync(reimport bool) error {
	if !s.scanProgressManager.Start() {
		return fmt.Errorf("scan already in progress")
	}
	go func() {
		ctx := context.Background()
		s.doScanAndImport(ctx, reimport)
	}()
	return nil
}

// scanProcessItem 扫描处理项
type scanProcessItem struct {
	filePath       string
	existingSongID int64 // >0 表示重新导入时的已有歌曲ID
}

// scanExtractResult 元数据提取结果
type scanExtractResult struct {
	item     scanProcessItem
	metadata *Metadata
	fileSize int64
}

const (
	metadataWorkers        = 4                // 并发元数据提取worker数量
	dbBatchSize            = 50               // 数据库批量提交大小
	fileStabilityThreshold = 10 * time.Second // 文件修改时间距今小于此值视为正在写入
)

// doScanAndImport 执行实际的扫描和导入操作
// 优化策略：
//  1. 预过滤：快速跳过已存在的文件，减少不必要的处理
//  2. 并发提取：使用worker池并行提取元数据，充分利用多核CPU
//  3. 批量写入：通过事务批量提交数据库操作，减少磁盘IO和锁竞争
func (s *SongService) doScanAndImport(ctx context.Context, reimport bool) {
	cancelCh := s.scanProgressManager.GetCancelChannel()

	files, err := s.scanner.ScanFiles(ctx)
	if err != nil {
		s.scanProgressManager.Fail(fmt.Errorf("failed to scan files: %w", err))
		return
	}

	select {
	case <-cancelCh:
		s.scanProgressManager.SetCancelled()
		return
	default:
	}

	s.scanProgressManager.SetTotalFiles(len(files))

	existingPaths, _ := s.songs.ListLocalPaths(ctx)
	if existingPaths == nil {
		existingPaths = make(map[string]database.LocalPathInfo)
	}

	toProcess := make([]scanProcessItem, 0, len(files))
	for _, filePath := range files {
		select {
		case <-cancelCh:
			s.scanProgressManager.SetCancelled()
			return
		default:
		}

		info, exists := existingPaths[filePath]
		if exists && !reimport && info.Duration > 0 {
			s.scanProgressManager.UpdateProgress(filePath, ProgressUpdateSkipped)
			continue
		}

		// 文件稳定性检测：跳过修改时间在最近 10 秒内的文件，
		// 避免导入正在拷贝中的不完整文件。
		if stat, err := os.Stat(filePath); err == nil {
			if time.Since(stat.ModTime()) < fileStabilityThreshold {
				slog.Debug("跳过正在写入的文件", "filePath", filePath)
				s.scanProgressManager.UpdateProgress(filePath, ProgressUpdateSkipped)
				continue
			}
		}

		item := scanProcessItem{filePath: filePath}
		if exists {
			item.existingSongID = info.SongID
		}
		toProcess = append(toProcess, item)
	}

	cleanedCount := s.cleanStaleRecords(ctx, files, existingPaths)
	if cleanedCount > 0 {
		s.scanProgressManager.SetCleanedFiles(cleanedCount)
	}

	if len(toProcess) == 0 {
		if s.configService != nil && s.configService.GetBool("scan_auto_create_playlists", true) {
			s.runAutoCreatePlaylists(ctx)
		} else {
			slog.Info("auto-create playlists disabled, skipping")
		}
		s.scanProgressManager.Complete()
		s.runAutoFingerprint()
		return
	}

	inputCh := make(chan scanProcessItem, metadataWorkers*2)
	resultCh := make(chan scanExtractResult, metadataWorkers*2)

	var wg sync.WaitGroup
	for i := 0; i < metadataWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range inputCh {
				select {
				case <-cancelCh:
					return
				default:
				}

				metadata, err := s.safeExtractMetadata(ctx, item.filePath)
				if err != nil {
					metadata = &Metadata{
						Title:  filepath.Base(item.filePath),
						Format: filepath.Ext(item.filePath),
					}
					slog.Info("doScanAndImport metadata failed", "filePath", item.filePath, "err", err)
				}

				if metadata.HasCover && metadata.CoverData != nil {
					coverPath, err := s.metadataExtractor.SaveCover(0, metadata)
					if err != nil {
						slog.Warn("worker保存封面失败", "filePath", item.filePath, "error", err)
					} else if coverPath != "" {
						metadata.CoverPath = coverPath
					}
					metadata.CoverData = nil
				}

				var fileSize int64
				if fileInfo, err := s.scanner.GetFileInfo(item.filePath); err == nil {
					fileSize = fileInfo.Size
				}

				select {
				case resultCh <- scanExtractResult{
					item:     item,
					metadata: metadata,
					fileSize: fileSize,
				}:
				case <-cancelCh:
					return
				}
			}
		}()
	}

	go func() {
		defer close(inputCh)
		for _, item := range toProcess {
			select {
			case inputCh <- item:
			case <-cancelCh:
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	allResults := make([]scanExtractResult, 0, len(toProcess))
	cancelled := false
	for result := range resultCh {
		select {
		case <-cancelCh:
			cancelled = true
		default:
		}
		if !cancelled {
			allResults = append(allResults, result)
		}
	}
	if cancelled {
		s.scanProgressManager.SetCancelled()
		return
	}

	fixSpamTags(allResults)

	for i := 0; i < len(allResults); i += dbBatchSize {
		end := min(i+dbBatchSize, len(allResults))
		s.flushScanBatch(ctx, allResults[i:end], reimport)
	}

	select {
	case <-cancelCh:
		s.scanProgressManager.SetCancelled()
		return
	default:
	}

	if s.configService != nil && s.configService.GetBool("scan_auto_create_playlists", true) {
		s.runAutoCreatePlaylists(ctx)
	} else {
		slog.Info("auto-create playlists disabled, skipping")
	}
	s.scanProgressManager.Complete()
	s.runAutoFingerprint()
}

// runAutoCreatePlaylists 扫描完成后按当前 includeSubdirs 配置重建 auto_created 歌单。
// 失败仅记录日志，不影响扫描的「完成」状态——下次扫描会再次尝试。
func (s *SongService) runAutoCreatePlaylists(ctx context.Context) {
	if s.playlistAutoCreator == nil {
		return
	}
	s.scanProgressManager.BeginCreatingPlaylists()

	includeSubdirs := false
	var autoCreateExcludeDirs []string
	if s.configService != nil {
		includeSubdirs = s.configService.GetBool("scan_auto_create_include_subdirs", false)
		var cfg struct {
			AutoCreateExcludeDirs []string `json:"auto_create_exclude_dirs"`
		}
		_ = s.configService.GetJSON("music_path", &cfg)
		autoCreateExcludeDirs = cfg.AutoCreateExcludeDirs
	}

	if _, err := s.playlistAutoCreator.AutoCreate(ctx, includeSubdirs, autoCreateExcludeDirs); err != nil {
		slog.Warn("自动创建歌单失败", "include_subdirs", includeSubdirs, "error", err)
	}
}

// runAutoFingerprint 扫描完成后自动为缺失指纹的歌曲计算指纹。
func (s *SongService) runAutoFingerprint() {
	if s.fingerprintService == nil || !IsChromaprintAvailable() {
		return
	}
	if _, err := s.fingerprintService.ComputeMissing(); err != nil {
		slog.Info("auto fingerprint skipped", "reason", err)
	}
}

// fixSpamTags 检测同目录下大量文件拥有完全相同的 (title, artist) 的情况，
// 判定为垃圾 tag（如广告），回退到用文件名作为标题。
func fixSpamTags(results []scanExtractResult) {
	type tagKey struct{ title, artist string }

	dirGroups := make(map[string][]int)
	for i, r := range results {
		dir := filepath.Dir(r.item.filePath)
		dirGroups[dir] = append(dirGroups[dir], i)
	}

	for dir, indices := range dirGroups {
		total := len(indices)
		counts := make(map[tagKey]int)
		for _, i := range indices {
			m := results[i].metadata
			if m.Title == "" && m.Artist == "" {
				continue
			}
			counts[tagKey{m.Title, m.Artist}]++
		}

		var spamKey tagKey
		var spamCount int
		for k, c := range counts {
			if c > spamCount {
				spamKey = k
				spamCount = c
			}
		}

		if spamCount < 3 || float64(spamCount)/float64(total) <= 0.5 {
			continue
		}

		slog.Warn("检测到疑似垃圾 tag，回退到文件名",
			"dir", dir, "spamTitle", spamKey.title, "spamArtist", spamKey.artist, "count", spamCount, "total", total)

		for _, i := range indices {
			m := results[i].metadata
			if m.Title == spamKey.title && m.Artist == spamKey.artist {
				fileName := strings.TrimSuffix(filepath.Base(results[i].item.filePath), filepath.Ext(results[i].item.filePath))
				m.Title = fileName
				m.Artist = ""
			}
		}
	}
}

// flushScanBatch 批量处理扫描结果，使用事务提高写入效率
// 将多条数据库操作合并在同一事务中提交，减少磁盘fsync次数和WAL刷写开销
func (s *SongService) flushScanBatch(ctx context.Context, batch []scanExtractResult, reimport bool) {
	if s.tx == nil {
		slog.Error("flushScanBatch 缺少事务执行器,跳过批次")
		for _, r := range batch {
			s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateFailed)
		}
		return
	}
	err := s.tx.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		txRepo := uow.Songs
		for _, r := range batch {
			if r.item.existingSongID > 0 && reimport {
				song, err := txRepo.GetByID(ctx, r.item.existingSongID)
				if err != nil {
					slog.Error("获取已有歌曲失败", "err", err, "songId", r.item.existingSongID)
					s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateFailed)
					continue
				}
				song.Title = r.metadata.Title
				song.Artist = r.metadata.Artist
				song.Album = r.metadata.Album
				song.Duration = r.metadata.Duration
				song.Format = r.metadata.Format
				song.BitRate = r.metadata.BitRate
				song.SampleRate = r.metadata.SampleRate
				song.ISRC = r.metadata.ISRC
				// lyric_source=manual 表示用户手动调整过歌词，
				// 重扫时不再用文件内嵌/外挂 .lrc 覆盖，否则不支持回写音频文件的格式
				// （如 .wav）一旦 reimport 就丢调整。
				if song.LyricSource != models.LyricSourceManual {
					models.ApplyLyricToSong(song, r.metadata.Lyric, r.metadata.LyricSource)
				}
				song.FileSize = r.fileSize
				song.UpdatedAt = time.Now()

				if r.metadata.CoverPath != "" {
					song.CoverPath = r.metadata.CoverPath
				}

				if err := txRepo.Update(ctx, song); err != nil {
					slog.Error("更新歌曲失败", "err", err, "song", song)
					s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateFailed)
					continue
				}
				s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateImported)
			} else {
				song := &models.Song{
					Type:       models.TypeLocal,
					Title:      r.metadata.Title,
					Artist:     r.metadata.Artist,
					Album:      r.metadata.Album,
					Duration:   r.metadata.Duration,
					FilePath:   r.item.filePath,
					Format:     r.metadata.Format,
					BitRate:    r.metadata.BitRate,
					SampleRate: r.metadata.SampleRate,
					FileSize:   r.fileSize,
					ISRC:       r.metadata.ISRC,
					AddedAt:    time.Now(),
					UpdatedAt:  time.Now(),
				}
				models.ApplyLyricToSong(song, r.metadata.Lyric, r.metadata.LyricSource)

				if r.metadata.CoverPath != "" {
					song.CoverPath = r.metadata.CoverPath
				}

				if err := txRepo.Create(ctx, song); err != nil {
					slog.Error("创建歌曲失败", "err", err, "song", song)
					s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateFailed)
					continue
				}
				s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateImported)
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("批次事务执行失败", "error", err)
	}
}

// cleanStaleRecords 清理数据库中已不存在于磁盘的本地歌曲记录
// 对比扫描得到的文件列表和数据库记录，删除文件不存在的记录
func (s *SongService) cleanStaleRecords(ctx context.Context, scannedFiles []string, existingPaths map[string]database.LocalPathInfo) int {
	scannedPathSet := make(map[string]struct{}, len(scannedFiles))
	for _, f := range scannedFiles {
		scannedPathSet[f] = struct{}{}
	}

	var staleIDs []int64
	for path, info := range existingPaths {
		if _, found := scannedPathSet[path]; !found {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				staleIDs = append(staleIDs, info.SongID)
				slog.Info("发现过期歌曲记录", "songId", info.SongID, "filePath", path)
			}
		}
	}

	if len(staleIDs) == 0 {
		return 0
	}

	cleaned, err := s.BatchDelete(ctx, staleIDs, false)
	if err != nil {
		slog.Warn("批量清理过期歌曲失败", "error", err, "count", len(staleIDs))
		return 0
	}

	slog.Info("扫描后自动清理过期歌曲记录", "cleaned", cleaned)
	return cleaned
}

// RemoteSongInput 批量添加网络歌曲的单条输入
type RemoteSongInput struct {
	URL             string // 仅纯外链歌曲使用;插件来源歌曲应留空
	Title           string
	Artist          string
	Album           string
	CoverURL        string
	Duration        float64
	PluginEntryPath string // 插件 entryPath(如 "subsonic");纯外链留空
	SourceData      string // 插件音源元数据(JSON 字符串,opaque)
	DedupKey        string // 去重 key(由插件定义,典型形态 "<platform>:<platform_id>");空时不去重
	Lyric           string // 歌词内容或歌词获取 URL
	LyricSource     string // 歌词来源类型
}

// RadioInput 批量添加电台的单条输入
type RadioInput struct {
	URL      string
	Title    string
	Artist   string
	CoverURL string
}

// AddRemoteSongs 批量添加网络歌曲
func (s *SongService) AddRemoteSongs(ctx context.Context, inputs []RemoteSongInput) ([]*models.Song, error) {
	now := time.Now()
	songs := make([]*models.Song, len(inputs))

	for i, input := range inputs {
		songs[i] = &models.Song{
			Type:            models.TypeRemote,
			Title:           input.Title,
			Artist:          input.Artist,
			Album:           input.Album,
			URL:             input.URL,
			CoverURL:        input.CoverURL,
			Duration:        input.Duration,
			PluginEntryPath: input.PluginEntryPath,
			SourceData:      input.SourceData,
			DedupKey:        input.DedupKey,
			AddedAt:         now,
			UpdatedAt:       now,
		}
		models.ApplyLyricToSong(songs[i], input.Lyric, input.LyricSource)
	}

	for _, song := range songs {
		if err := s.songs.UpsertRemote(ctx, song); err != nil {
			return nil, fmt.Errorf("failed to upsert remote song: %w", err)
		}
	}

	return songs, nil
}

// AddRadios 批量添加电台/广播
func (s *SongService) AddRadios(ctx context.Context, inputs []RadioInput) ([]*models.Song, error) {
	now := time.Now()
	songs := make([]*models.Song, len(inputs))
	for i, input := range inputs {
		songs[i] = &models.Song{
			Type:      models.TypeRadio,
			Title:     input.Title,
			Artist:    input.Artist,
			URL:       input.URL,
			CoverURL:  input.CoverURL,
			IsLive:    true,
			AddedAt:   now,
			UpdatedAt: now,
		}
	}

	if err := s.songs.BatchCreate(ctx, songs); err != nil {
		return nil, fmt.Errorf("failed to batch add radios: %w", err)
	}
	return songs, nil
}

// safeExtractMetadata 安全提取元数据，捕获 panic 防止单个文件错误导致整个扫描崩溃
func (s *SongService) safeExtractMetadata(ctx context.Context, filePath string) (metadata *Metadata, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic during metadata extraction", "filePath", filePath, "panic", r)
			err = fmt.Errorf("panic during metadata extraction: %v", r)
			metadata = nil
		}
	}()
	return s.metadataExtractor.Extract(ctx, filePath)
}

// CleanResult 清理结果统计
type CleanResult struct {
	FileNotFound  int `json:"file_not_found"`  // 文件不存在的歌曲数
	InExcludedDir int `json:"in_excluded_dir"` // 位于排除目录中的歌曲数
	Total         int `json:"total"`           // 总清理数
}

// CleanInvalidSongs 清理无效的本地歌曲
// 清理条件：文件不存在 或 文件路径在排除目录/路径中
func (s *SongService) CleanInvalidSongs(ctx context.Context) (*CleanResult, error) {
	filter := &database.SongFilter{
		Type:  models.TypeLocal,
		Limit: 100000,
	}

	songs, err := s.songs.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list local songs: %w", err)
	}

	result := &CleanResult{}
	for _, song := range songs {
		shouldClean := false
		reason := ""

		if _, err := os.Stat(song.FilePath); os.IsNotExist(err) {
			shouldClean = true
			reason = "file_not_found"
			result.FileNotFound++
		} else if s.scanner != nil && s.scanner.IsFileInExcludedArea(song.FilePath) {
			shouldClean = true
			reason = "in_excluded_dir"
			result.InExcludedDir++
		}

		if shouldClean {
			if err := s.Delete(ctx, song.ID); err != nil {
				slog.Warn("删除无效歌曲失败", "songId", song.ID, "filePath", song.FilePath, "reason", reason, "error", err)
				if reason == "file_not_found" {
					result.FileNotFound--
				} else {
					result.InExcludedDir--
				}
				continue
			}
			slog.Info("清理无效歌曲", "songId", song.ID, "filePath", song.FilePath, "reason", reason)
		}
	}

	result.Total = result.FileNotFound + result.InExcludedDir
	return result, nil
}

// UpdateLyrics 更新歌曲歌词，并在条件满足时把主歌词回写到本地音频文件。
//
// 行为：
//  1. 把新歌词内容更新进 DB（lyric 列、lyric_source 列、lyric_remote_url 列）
//  2. 若 song.Type==local 且 song.FilePath 存在、且 lyricSource 不是 url，
//     重新读取 song 后调用 WriteSongTags 把元数据完整回写到音频文件。
//     回写传完整 song（pkg/tag.WriteTag 是重建模式，避免清空 Title/Artist 等其它字段）。
//
// 返回值：
//   - status 表示文件回写结果（written/unchanged/failed）；DB 已写入成功才会有值
//   - err 仅在 DB 操作失败时非 nil（文件回写失败不算失败，会通过 status 表达）
//
// lyric 应为 LyricPayload JSON 文本（或空）；lyricRemoteURL 仅在 lyricSource="url" 时使用。
func (s *SongService) UpdateLyrics(ctx context.Context, id int64, lyric, lyricSource, lyricRemoteURL string) (FileWriteStatus, error) {
	if err := s.songs.UpdateLyrics(ctx, id, lyric, lyricSource, lyricRemoteURL); err != nil {
		return "", err
	}

	// url 来源不需要回写：歌词在运行时拉取，不缓存到本地文件
	if lyricSource == models.LyricSourceURL {
		return FileWriteUnchanged, nil
	}

	// 只对 type=local 且有 file_path 的歌曲尝试回写
	song, err := s.songs.GetByID(ctx, id)
	if err != nil || song == nil {
		// DB 已写成功，但读不到 song（极小概率），跳过文件回写不报错
		slog.Warn("UpdateLyrics: refetch song after lyric update failed", "songId", id, "err", err)
		return FileWriteUnchanged, nil
	}
	if song.Type != models.TypeLocal || song.FilePath == "" {
		return FileWriteUnchanged, nil
	}

	return WriteSongTags(song.FilePath, song), nil
}

// UpdateSongDuration 按 song ID 回填时长（仅在原 duration 为 0 时生效）。
// 由 SourceOrchestrator 在 Fetch 成功后内联调用。
func (s *SongService) UpdateSongDuration(ctx context.Context, id int64, duration float64) error {
	return s.songs.UpdateDuration(ctx, id, duration)
}

// UpdateSongSource 按 song ID 更新音源字段(plugin_entry_path + source_data)。
// 用于 SourceResolver 切换音源后回写。
func (s *SongService) UpdateSongSource(ctx context.Context, id int64, pluginEntryPath, sourceData string) error {
	return s.songs.UpdateSource(ctx, id, pluginEntryPath, sourceData)
}

// SaveCoverFromData 从原始字节保存封面到本地，返回 coverPath。
func (s *SongService) SaveCoverFromData(data []byte, ext string) (string, error) {
	if s.metadataExtractor == nil {
		return "", fmt.Errorf("metadata extractor not configured")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty cover data")
	}
	if ext == "" {
		ext = "jpg"
	}
	return s.metadataExtractor.SaveCoverData(data, ext)
}

// DownloadCover 从 URL 下载封面并保存到本地，返回 coverPath。
func (s *SongService) DownloadCover(ctx context.Context, coverURL string) (string, error) {
	if s.metadataExtractor == nil {
		return "", fmt.Errorf("metadata extractor not configured")
	}

	client := httputil.NewClient(30 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "image/*")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", fmt.Errorf("non-image Content-Type: %s", contentType)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty cover body")
	}

	ext := extFromContentType(contentType)
	return s.metadataExtractor.SaveCoverData(data, ext)
}

func extFromContentType(contentType string) string {
	ct := strings.ToLower(contentType)
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	switch ct {
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/bmp":
		return "bmp"
	default:
		return "jpg"
	}
}

// OrganizeItem 批量整理的单项输入。
type OrganizeItem struct {
	ID         int64  `json:"id"`
	TargetPath string `json:"target_path"`
}

// OrganizeResult 批量整理的单项结果。
type OrganizeResult struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"`
	FilePath string `json:"file_path,omitempty"`
	Error    string `json:"error,omitempty"`
}

// OrganizeSongs 批量移动/重命名本地歌曲文件。
func (s *SongService) OrganizeSongs(ctx context.Context, musicPath string, items []OrganizeItem) []OrganizeResult {
	results := make([]OrganizeResult, 0, len(items))
	for _, item := range items {
		r := s.organizeOne(ctx, musicPath, item)
		results = append(results, r)
	}
	return results
}

func (s *SongService) organizeOne(ctx context.Context, musicPath string, item OrganizeItem) OrganizeResult {
	song, err := s.songs.GetByID(ctx, item.ID)
	if err != nil {
		return OrganizeResult{ID: item.ID, Status: "error", Error: "song not found"}
	}
	if song.Type != models.TypeLocal {
		return OrganizeResult{ID: item.ID, Status: "error", Error: "not a local song"}
	}
	if song.FilePath == "" {
		return OrganizeResult{ID: item.ID, Status: "error", Error: "song has no file path"}
	}

	targetPath := filepath.Clean(item.TargetPath)
	if strings.HasPrefix(targetPath, "..") {
		return OrganizeResult{ID: item.ID, Status: "error", Error: "path traversal not allowed"}
	}

	absSource := filepath.Join(musicPath, song.FilePath)
	absTarget := filepath.Join(musicPath, targetPath)

	if !strings.HasPrefix(absTarget, musicPath+string(filepath.Separator)) && absTarget != musicPath {
		return OrganizeResult{ID: item.ID, Status: "error", Error: "target path outside music directory"}
	}

	if filepath.Ext(absSource) != filepath.Ext(absTarget) {
		return OrganizeResult{ID: item.ID, Status: "error", Error: "file extension mismatch"}
	}

	if absSource == absTarget {
		return OrganizeResult{ID: item.ID, Status: "ok", FilePath: targetPath}
	}

	if err := os.MkdirAll(filepath.Dir(absTarget), 0755); err != nil {
		return OrganizeResult{ID: item.ID, Status: "error", Error: fmt.Sprintf("create directory: %v", err)}
	}

	if err := moveFile(absSource, absTarget); err != nil {
		return OrganizeResult{ID: item.ID, Status: "error", Error: fmt.Sprintf("move file: %v", err)}
	}

	song.FilePath = targetPath
	if err := s.songs.Update(ctx, song); err != nil {
		_ = moveFile(absTarget, absSource)
		return OrganizeResult{ID: item.ID, Status: "error", Error: fmt.Sprintf("update database: %v", err)}
	}

	_ = os.Remove(filepath.Dir(absSource))

	return OrganizeResult{ID: item.ID, Status: "ok", FilePath: targetPath}
}
