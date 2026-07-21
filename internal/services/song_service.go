package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hanxi/tag"

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
	ListCueSources(ctx context.Context) (map[string]bool, error)
	ListCueAudioPaths(ctx context.Context, cueSourcePath string) ([]string, error)
	DeleteByCueSource(ctx context.Context, cueSourcePath string) (int, error)
	UpdateFingerprint(ctx context.Context, id int64, fingerprint string, duration float64) error
	ClearAllFingerprints(ctx context.Context) error
	ListLocalWithoutFingerprint(ctx context.Context) ([]database.SongIDPath, error)
	CountLocalFingerprints(ctx context.Context) (total, computed int64, err error)
	ListDuplicateGroups(ctx context.Context) ([]database.DuplicateGroup, error)
	ListFacet(ctx context.Context, field string, f *database.FacetFilter) ([]database.Facet, error)
	CountFacet(ctx context.Context, field, keyword string) (int64, error)
}

// Transactor 提供 UnitOfWork 事务执行入口，
// 让批量扫描在单一事务里操作 SongRepository。
type Transactor interface {
	RunInTx(ctx context.Context, fn func(context.Context, *database.UnitOfWork) error) error
}

// PlaylistAutoCreator 由扫描完成后调用，重建 auto_created 歌单。
type PlaylistAutoCreator interface {
	AutoCreate(ctx context.Context, playlistMode string, excludeDirs []string) (*models.AutoCreatePlaylistsResponse, error)
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
func (s *SongService) Delete(ctx context.Context, id int64, deleteFiles bool) error {
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
	if deleteFiles && song != nil && song.Type == models.TypeLocal && song.FilePath != "" && song.CueSourcePath == "" {
		if err := os.Remove(song.FilePath); err != nil {
			slog.Warn("删除音频文件失败", "path", song.FilePath, "error", err)
		} else {
			slog.Info("已删除音频文件", "path", song.FilePath)
		}
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
		if deleteFiles && song.Type == models.TypeLocal && song.FilePath != "" && song.CueSourcePath == "" {
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

// ListFacet 按维度聚合曲库标签，返回该维度下的取值 + 计数 + 代表封面（支持搜索/排序/分页）。
// field 支持 genre/artist/album/language/style/year/decade；未知 field 返回 database.ErrNotFound。
func (s *SongService) ListFacet(ctx context.Context, field string, f *database.FacetFilter) ([]database.Facet, error) {
	facets, err := s.songs.ListFacet(ctx, field, f)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to list facet %q: %w", field, err)
	}
	return facets, nil
}

// CountFacet 返回某维度去重取值的总数（供分页判断），与 ListFacet 共享 keyword 过滤。
func (s *SongService) CountFacet(ctx context.Context, field, keyword string) (int64, error) {
	total, err := s.songs.CountFacet(ctx, field, keyword)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return 0, err
		}
		return 0, fmt.Errorf("failed to count facet %q: %w", field, err)
	}
	return total, nil
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

// ScanAndImportAsync 异步扫描并导入本地音乐文件。
// paths 为空时扫描整个音乐根目录；非空时只扫描给定目录（含子目录），
// 用于目录级定向扫描（Issue #262），此时过期记录清理也仅收敛到这些目录之内。
func (s *SongService) ScanAndImportAsync(reimport bool, paths []string) error {
	if !s.scanProgressManager.Start() {
		return fmt.Errorf("scan already in progress")
	}
	go func() {
		ctx := context.Background()
		s.doScanAndImport(ctx, reimport, paths)
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
	item             scanProcessItem
	metadata         *Metadata
	fileSize         int64
	fileModTime      time.Time
	extractionFailed bool
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
func (s *SongService) doScanAndImport(ctx context.Context, reimport bool, scopeRoots []string) {
	cancelCh := s.scanProgressManager.GetCancelChannel()

	// 派生可取消 ctx：用户取消时同步 cancel，使 CUE 切分阶段的 ffmpeg 子进程
	// （exec.CommandContext）被及时杀掉，而非等其自然结束。
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-cancelCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	scanResult, err := s.scanner.ScanFilesWithCueInDirs(ctx, scopeRoots, func(count int) {
		s.scanProgressManager.SetDiscoveredFiles(count)
	})
	if err != nil {
		s.scanProgressManager.Fail(fmt.Errorf("failed to scan files: %w", err))
		return
	}
	files := scanResult.AudioFiles

	select {
	case <-cancelCh:
		s.scanProgressManager.SetCancelled()
		return
	default:
	}

	s.scanProgressManager.SetTotalFiles(len(files))

	existingPaths, err := s.songs.ListLocalPaths(ctx)
	if err != nil {
		slog.Warn("ListLocalPaths 查询失败，重试一次", "error", err)
		time.Sleep(500 * time.Millisecond)
		existingPaths, err = s.songs.ListLocalPaths(ctx)
		if err != nil {
			slog.Error("ListLocalPaths 重试仍然失败，终止扫描", "error", err)
			s.scanProgressManager.Fail(fmt.Errorf("数据库查询失败: %w", err))
			return
		}
	}
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

	cleanedCount := s.cleanStaleRecords(ctx, files, existingPaths, scopeRoots)
	if cleanedCount > 0 {
		s.scanProgressManager.SetCleanedFiles(cleanedCount)
	}

	if len(toProcess) == 0 {
		s.runCueProcessing(ctx, scanResult, files, reimport, scopeRoots)
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
		s.setLocalSongCount(ctx)
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
				extractionFailed := err != nil
				if err != nil {
					rawName := strings.TrimSuffix(filepath.Base(item.filePath), filepath.Ext(item.filePath))
					metadata = &Metadata{
						Title:  tag.FixEncoding([]byte(rawName)),
						Format: NormalizeFormat(filepath.Ext(item.filePath)),
					}
					slog.Warn("doScanAndImport metadata failed", "filePath", item.filePath, "err", err)
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
				var fileModTime time.Time
				if fileInfo, err := s.scanner.GetFileInfo(item.filePath); err == nil {
					fileSize = fileInfo.Size
					fileModTime = fileInfo.ModTime
				}

				select {
				case resultCh <- scanExtractResult{
					item:             item,
					metadata:         metadata,
					fileSize:         fileSize,
					fileModTime:      fileModTime,
					extractionFailed: extractionFailed,
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
		s.flushScanBatch(ctx, allResults[i:end])
	}

	select {
	case <-cancelCh:
		s.scanProgressManager.SetCancelled()
		return
	default:
	}

	// CUE 整轨处理
	s.runCueProcessing(ctx, scanResult, files, reimport, scopeRoots)

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
	s.setLocalSongCount(ctx)
	s.scanProgressManager.Complete()
	s.runAutoFingerprint()
}

// setLocalSongCount 查询 DB 中本地歌曲总数并写入扫描进度，供前端展示。
func (s *SongService) setLocalSongCount(ctx context.Context) {
	localCount, err := s.songs.Count(ctx, &database.SongFilter{Type: "local"})
	if err != nil {
		slog.Warn("查询本地歌曲总数失败", "error", err)
		return
	}
	s.scanProgressManager.SetLocalSongCount(int(localCount))
}

// runCueProcessing 执行 CUE 处理阶段（外部 .cue + FLAC 内嵌 CUESHEET），
// 并清理失效的 CUE 记录。仅解析元数据和入库，播放时按需提取。
func (s *SongService) runCueProcessing(ctx context.Context, scanResult *ScanResult, files []string, reimport bool, scopeRoots []string) {
	s.scanProgressManager.BeginSplittingCue()
	s.processCueFiles(ctx, scanResult.CueFiles, reimport)
	s.processEmbeddedCueSheets(ctx, files, reimport)
	s.cleanStaleCueRecords(ctx, scopeRoots)
}

// runAutoCreatePlaylists 扫描完成后按当前 playlistMode 配置重建 auto_created 歌单。
// 失败仅记录日志，不影响扫描的「完成」状态——下次扫描会再次尝试。
func (s *SongService) runAutoCreatePlaylists(ctx context.Context) {
	if s.playlistAutoCreator == nil {
		return
	}
	s.scanProgressManager.BeginCreatingPlaylists()

	playlistMode := models.PlaylistModeDirectory
	var autoCreateExcludeDirs []string
	if s.configService != nil {
		playlistMode = s.configService.GetString("scan_playlist_mode", models.PlaylistModeDirectory)
		var cfg struct {
			AutoCreateExcludeDirs []string `json:"auto_create_exclude_dirs"`
		}
		_ = s.configService.GetJSON("music_path", &cfg)
		autoCreateExcludeDirs = cfg.AutoCreateExcludeDirs
	}

	if _, err := s.playlistAutoCreator.AutoCreate(ctx, playlistMode, autoCreateExcludeDirs); err != nil {
		slog.Warn("自动创建歌单失败", "playlist_mode", playlistMode, "error", err)
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
func (s *SongService) flushScanBatch(ctx context.Context, batch []scanExtractResult) {
	if s.tx == nil {
		slog.Error("flushScanBatch 缺少事务执行器,跳过批次")
		for _, r := range batch {
			s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateFailed)
		}
		return
	}
	// 每个 item 的进度结果，事务提交成功后才写入 progress manager
	itemResults := make([]ProgressUpdateType, len(batch))

	txFn := func(ctx context.Context, uow *database.UnitOfWork) error {
		txRepo := uow.Songs
		for i, r := range batch {
			if r.item.existingSongID > 0 {
				if r.extractionFailed {
					slog.Warn("元数据提取失败，保留已有记录", "filePath", r.item.filePath)
					itemResults[i] = ProgressUpdateFailed
					continue
				}
				song, err := txRepo.GetByID(ctx, r.item.existingSongID)
				if err != nil {
					slog.Error("获取已有歌曲失败", "err", err, "songId", r.item.existingSongID)
					itemResults[i] = ProgressUpdateFailed
					continue
				}
				song.Title = r.metadata.Title
				song.Artist = r.metadata.Artist
				song.Album = r.metadata.Album
				song.Duration = r.metadata.Duration
				song.Format = r.metadata.Format
				song.BitRate = r.metadata.BitRate
				song.SampleRate = r.metadata.SampleRate
				song.IsVideo = r.metadata.IsVideo
				song.ISRC = r.metadata.ISRC
				song.Year = r.metadata.Year
				song.Genre = r.metadata.Genre
				// 语种/风格多数格式无标准标签，读到才更新，避免重扫抹掉用户手填值
				if r.metadata.Language != "" {
					song.Language = r.metadata.Language
				}
				if r.metadata.Style != "" {
					song.Style = r.metadata.Style
				}
				if r.metadata.Track != "" {
					song.Track = r.metadata.Track
				}
				if song.LyricSource != models.LyricSourceManual {
					models.ApplyLyricToSong(song, r.metadata.Lyric, r.metadata.LyricSource)
				}
				song.FileSize = r.fileSize
				song.UpdatedAt = time.Now()
				if !r.fileModTime.IsZero() {
					mt := r.fileModTime
					song.FileModifiedAt = &mt
				}

				if r.metadata.CoverPath != "" {
					song.CoverPath = r.metadata.CoverPath
				}

				if err := txRepo.Update(ctx, song); err != nil {
					slog.Error("更新歌曲失败", "err", err, "song", song)
					itemResults[i] = ProgressUpdateFailed
					continue
				}
				itemResults[i] = ProgressUpdateImported
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
					IsVideo:    r.metadata.IsVideo,
					FileSize:   r.fileSize,
					ISRC:       r.metadata.ISRC,
					Track:      r.metadata.Track,
					Year:       r.metadata.Year,
					Genre:      r.metadata.Genre,
					Language:   r.metadata.Language,
					Style:      r.metadata.Style,
					AddedAt:    time.Now(),
					UpdatedAt:  time.Now(),
				}
				if !r.fileModTime.IsZero() {
					mt := r.fileModTime
					song.FileModifiedAt = &mt
				}
				models.ApplyLyricToSong(song, r.metadata.Lyric, r.metadata.LyricSource)

				if r.metadata.CoverPath != "" {
					song.CoverPath = r.metadata.CoverPath
				}

				if err := txRepo.Create(ctx, song); err != nil {
					slog.Error("创建歌曲失败", "err", err, "song", song)
					itemResults[i] = ProgressUpdateFailed
					continue
				}
				itemResults[i] = ProgressUpdateImported
			}
		}
		return nil
	}

	const maxRetries = 3
	retryDelays := [maxRetries]time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	var err error
	for attempt := range maxRetries {
		err = s.tx.RunInTx(ctx, txFn)
		if err == nil {
			for i, r := range batch {
				s.scanProgressManager.UpdateProgress(r.item.filePath, itemResults[i])
			}
			return
		}
		slog.Warn("批次事务执行失败，准备重试", "attempt", attempt+1, "error", err)
		time.Sleep(retryDelays[attempt])
	}

	slog.Error("批次事务执行失败，已耗尽重试", "error", err)
	for _, r := range batch {
		s.scanProgressManager.UpdateProgress(r.item.filePath, ProgressUpdateFailed)
	}
}

// cleanStaleRecords 清理数据库中已不存在于磁盘的本地歌曲记录
// 对比扫描得到的文件列表和数据库记录，删除文件不存在的记录
// isUnderScope 判断 path 是否落在 scopeRoots 中任一目录之下（含目录本身）。
// scopeRoots 为空表示全库作用域（不做限制），返回 true。
func isUnderScope(path string, scopeRoots []string) bool {
	if len(scopeRoots) == 0 {
		return true
	}
	for _, root := range scopeRoots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (s *SongService) cleanStaleRecords(ctx context.Context, scannedFiles []string, existingPaths map[string]database.LocalPathInfo, scopeRoots []string) int {
	scannedPathSet := make(map[string]struct{}, len(scannedFiles))
	for _, f := range scannedFiles {
		scannedPathSet[f] = struct{}{}
	}

	var staleIDs []int64
	for path, info := range existingPaths {
		// 定向扫描：只清理落在本次作用域内的记录，作用域外的一律不动，
		// 避免"只扫某目录"却误删其余曲库（Issue #262）。
		if !isUnderScope(path, scopeRoots) {
			continue
		}
		// CUE 来源的歌曲由 cleanStaleCueRecords 单独处理
		if info.CueSourcePath != "" {
			continue
		}
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
	LyricRemoteURL  string // 歌词远程 URL(直传,优先于 Lyric+LyricSource=url 间接方式)
	IsVideo         bool   // 是否含视频画面(网络歌曲不走扫描 ffprobe,由客户端开关声明)
}

// RadioInput 批量添加电台的单条输入
type RadioInput struct {
	URL      string
	Title    string
	Artist   string
	CoverURL string
	IsVideo  bool // 是否为视频电台(直播画面)
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
			IsVideo:         input.IsVideo,
			AddedAt:         now,
			UpdatedAt:       now,
		}
		models.ApplyLyricToSong(songs[i], input.Lyric, input.LyricSource)
		if input.LyricRemoteURL != "" {
			songs[i].LyricSource = models.LyricSourceURL
			songs[i].Lyric = ""
			songs[i].LyricRemoteURL = input.LyricRemoteURL
		}
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
			IsVideo:   input.IsVideo,
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
		// CUE track 由 cleanStaleCueRecords 统一管理，不在此逐条处理
		if song.CueSourcePath != "" {
			continue
		}

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
			if err := s.Delete(ctx, song.ID, false); err != nil {
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

// OrganizeResult 批量整理的单项结果。Status ∈ ok | skip | error。
type OrganizeResult struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"`
	FilePath string `json:"file_path,omitempty"`
	Error    string `json:"error,omitempty"`
}

// OrganizePreviewResult 整理预览（dry-run）的单项结果。Status ∈ ok | conflict | skip | error。
type OrganizePreviewResult struct {
	ID      int64  `json:"id"`
	OldPath string `json:"old_path,omitempty"`
	NewPath string `json:"new_path,omitempty"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

// organizePlan 是单项整理经校验后的计划。
// 说明：song.FilePath 由扫描器存储为「以 music_path 为根的完整路径」（可直接用于 os 操作，
// 见 BatchDelete 的 os.Remove(song.FilePath)）。因此 absSource 直接取 song.FilePath，
// 而 newPath = Join(musicPath, target_path) 保持同一种根格式，写回 DB 后与扫描格式一致。
type organizePlan struct {
	song      *models.Song
	newPath   string // 移动后写回 DB 的 file_path（Join(musicPath, target_path)，与扫描格式一致）
	absSource string // 源文件路径（= song.FilePath，已是完整路径）
	absTarget string // 目标文件路径（= newPath）
	noop      bool   // 源与目标相同，无需移动
}

// resolveMusicPath 返回当前 music_path；未配置时报错。
func (s *SongService) resolveMusicPath() (string, error) {
	if s.scanner == nil {
		return "", fmt.Errorf("music_path 未配置")
	}
	musicPath := s.scanner.GetMusicPath()
	if musicPath == "" {
		return "", fmt.Errorf("music_path 未设置")
	}
	return musicPath, nil
}

// resolveOrganizeTarget 校验单项并计算源/目标绝对路径，preview 与 execute 共用。
// 返回的 *OrganizeResult 非 nil 时表示该项已有终态（skip / error），调用方直接采用。
func (s *SongService) resolveOrganizeTarget(ctx context.Context, musicPath string, item OrganizeItem) (*organizePlan, *OrganizeResult) {
	song, err := s.songs.GetByID(ctx, item.ID)
	if err != nil {
		return nil, &OrganizeResult{ID: item.ID, Status: "error", Error: "song not found"}
	}
	if song.Type != models.TypeLocal {
		return nil, &OrganizeResult{ID: item.ID, Status: "error", Error: "not a local song"}
	}
	// CUE 拆分歌曲多条记录共享同一音频文件，搬动会导致其它轨路径失效，直接跳过。
	if song.CueSourcePath != "" {
		return nil, &OrganizeResult{ID: item.ID, Status: "skip", Error: "cue song skipped"}
	}
	if song.FilePath == "" {
		return nil, &OrganizeResult{ID: item.ID, Status: "error", Error: "song has no file path"}
	}

	targetPath := filepath.Clean(item.TargetPath)
	if strings.HasPrefix(targetPath, "..") {
		return nil, &OrganizeResult{ID: item.ID, Status: "error", Error: "path traversal not allowed"}
	}

	// newPath 与扫描器存储格式一致（以 music_path 为根）；absSource 直接取已存储的完整路径。
	newPath := filepath.Join(musicPath, targetPath)
	absSource := song.FilePath

	if !strings.HasPrefix(newPath, musicPath+string(filepath.Separator)) && newPath != musicPath {
		return nil, &OrganizeResult{ID: item.ID, Status: "error", Error: "target path outside music directory"}
	}

	if filepath.Ext(absSource) != filepath.Ext(newPath) {
		return nil, &OrganizeResult{ID: item.ID, Status: "error", Error: "file extension mismatch"}
	}

	return &organizePlan{
		song:      song,
		newPath:   newPath,
		absSource: absSource,
		absTarget: newPath,
		noop:      filepath.Clean(absSource) == filepath.Clean(newPath),
	}, nil
}

// PreviewOrganize 预览批量整理（dry-run，不落盘）。music_path 由 service 自取。
func (s *SongService) PreviewOrganize(ctx context.Context, items []OrganizeItem) []OrganizePreviewResult {
	results := make([]OrganizePreviewResult, 0, len(items))
	musicPath, err := s.resolveMusicPath()
	if err != nil {
		for _, item := range items {
			results = append(results, OrganizePreviewResult{ID: item.ID, Status: "error", Error: err.Error()})
		}
		return results
	}

	seen := make(map[string]int64) // absTarget → 首个占用该目标的 song id，检测批内撞名
	for _, item := range items {
		plan, res := s.resolveOrganizeTarget(ctx, musicPath, item)
		if res != nil {
			results = append(results, OrganizePreviewResult{ID: res.ID, Status: res.Status, Error: res.Error})
			continue
		}

		pr := OrganizePreviewResult{ID: item.ID, OldPath: plan.song.FilePath, NewPath: plan.newPath, Status: "ok"}
		if !plan.noop {
			if fileExists(plan.absTarget) {
				pr.Status = "conflict"
				pr.Error = "target already exists"
			} else if prevID, ok := seen[plan.absTarget]; ok {
				pr.Status = "conflict"
				pr.Error = fmt.Sprintf("target conflicts with song %d in this batch", prevID)
			} else {
				seen[plan.absTarget] = item.ID
			}
		}
		results = append(results, pr)
	}
	return results
}

// OrganizeSongs 批量移动/重命名本地歌曲文件。music_path 由 service 自取。
func (s *SongService) OrganizeSongs(ctx context.Context, items []OrganizeItem) []OrganizeResult {
	results := make([]OrganizeResult, 0, len(items))
	musicPath, err := s.resolveMusicPath()
	if err != nil {
		for _, item := range items {
			results = append(results, OrganizeResult{ID: item.ID, Status: "error", Error: err.Error()})
		}
		return results
	}
	for _, item := range items {
		results = append(results, s.organizeOne(ctx, musicPath, item))
	}
	return results
}

func (s *SongService) organizeOne(ctx context.Context, musicPath string, item OrganizeItem) OrganizeResult {
	plan, res := s.resolveOrganizeTarget(ctx, musicPath, item)
	if res != nil {
		return *res
	}
	if plan.noop {
		return OrganizeResult{ID: item.ID, Status: "ok", FilePath: plan.newPath}
	}

	// 拒绝覆盖已存在的目标文件（moveFile 底层 os.Rename 会静默覆盖）。
	if fileExists(plan.absTarget) {
		return OrganizeResult{ID: item.ID, Status: "error", Error: "target already exists"}
	}

	if err := os.MkdirAll(filepath.Dir(plan.absTarget), 0755); err != nil {
		return OrganizeResult{ID: item.ID, Status: "error", Error: fmt.Sprintf("create directory: %v", err)}
	}

	if err := moveFile(plan.absSource, plan.absTarget); err != nil {
		return OrganizeResult{ID: item.ID, Status: "error", Error: fmt.Sprintf("move file: %v", err)}
	}

	plan.song.FilePath = plan.newPath
	if err := s.songs.Update(ctx, plan.song); err != nil {
		_ = moveFile(plan.absTarget, plan.absSource)
		return OrganizeResult{ID: item.ID, Status: "error", Error: fmt.Sprintf("update database: %v", err)}
	}

	_ = os.Remove(filepath.Dir(plan.absSource))

	return OrganizeResult{ID: item.ID, Status: "ok", FilePath: plan.newPath}
}

// fileExists 判断路径是否存在。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// RenameLocalSongFile 按 newTitle 重命名本地歌曲文件（保留原目录与扩展名），
// 移动成功后连同 song 其余已修改字段一起写回 DB；DB 失败时回滚文件移动。
// changed=false 表示清理后的文件名与原文件同名（未移动，但仍写回 DB）。
// 仅适用于本地非 CUE 歌曲，其余情况返回 error 且不改动任何状态。
func (s *SongService) RenameLocalSongFile(ctx context.Context, song *models.Song, newTitle string) (changed bool, err error) {
	if song.Type != models.TypeLocal {
		return false, fmt.Errorf("仅支持本地歌曲")
	}
	// CUE 拆分歌曲多条记录共享同一音频文件，改名会导致其它轨路径失效。
	if song.CueSourcePath != "" {
		return false, fmt.Errorf("CUE 歌曲不支持重命名")
	}
	if song.FilePath == "" {
		return false, fmt.Errorf("歌曲无文件路径")
	}

	base := sanitizePathComponent(newTitle)
	if base == "" {
		return false, fmt.Errorf("标题不适合作为文件名")
	}

	oldPath := song.FilePath
	newPath := filepath.Join(filepath.Dir(oldPath), base+filepath.Ext(oldPath))

	// 同名：无需移动，仅写回 DB（承载 title/artist/album 等其它已改字段）。
	if filepath.Clean(newPath) == filepath.Clean(oldPath) {
		if err := s.songs.Update(ctx, song); err != nil {
			return false, fmt.Errorf("更新数据库失败: %w", err)
		}
		return false, nil
	}

	// 拒绝覆盖已存在的目标文件（moveFile 底层 os.Rename 会静默覆盖）。
	// 例外：大小写不敏感文件系统（macOS/Windows）上仅改标题大小写时，newPath 会命中原文件本身
	// （os.Stat 解析到同一 inode），此时并非冲突，应放行让 os.Rename 完成大小写改名。
	if oldInfo, statErr := os.Stat(oldPath); statErr == nil {
		if newInfo, err := os.Stat(newPath); err == nil && !os.SameFile(oldInfo, newInfo) {
			return false, fmt.Errorf("目标文件已存在: %s", filepath.Base(newPath))
		}
	} else if fileExists(newPath) {
		// 源文件 stat 失败（异常），保守地按原逻辑拒绝已存在的目标。
		return false, fmt.Errorf("目标文件已存在: %s", filepath.Base(newPath))
	}

	if err := moveFile(oldPath, newPath); err != nil {
		return false, fmt.Errorf("移动文件失败: %w", err)
	}

	song.FilePath = newPath
	if err := s.songs.Update(ctx, song); err != nil {
		// 回滚文件移动，保持磁盘与 DB 一致。
		_ = moveFile(newPath, oldPath)
		song.FilePath = oldPath
		return false, fmt.Errorf("更新数据库失败: %w", err)
	}

	return true, nil
}
