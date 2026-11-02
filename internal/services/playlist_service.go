package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"time"

	"songloft/internal/database"
	"songloft/internal/models"
)

// PlaylistRepository 是 PlaylistService 依赖的歌单仓储接口。
type PlaylistRepository interface {
	Create(ctx context.Context, playlist *models.Playlist) error
	GetByID(ctx context.Context, id int64) (*models.Playlist, error)
	Update(ctx context.Context, playlist *models.Playlist) error
	UpdateSort(ctx context.Context, id int64, sortBy, sortOrder string) error
	Touch(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter *database.PlaylistFilter) ([]*models.Playlist, error)
	Count(ctx context.Context, filter *database.PlaylistFilter) (int64, error)
	BatchDelete(ctx context.Context, ids []int64) (int, error)
	BatchUpdatePositions(ctx context.Context, playlistIDs []int64) error
	SetPinned(ctx context.Context, id int64, pinnedAt *time.Time) error
}

// PlaylistSongRepository 是 PlaylistService 依赖的歌单-歌曲关联仓储接口。
type PlaylistSongRepository interface {
	AddSong(ctx context.Context, playlistID, songID int64, position int) error
	RemoveSong(ctx context.Context, playlistID, songID int64) error
	GetSongs(ctx context.Context, playlistID int64) ([]*models.Song, error)
	GetSongsPaginated(ctx context.Context, playlistID int64, limit, offset int) ([]*models.Song, error)
	GetSongsFiltered(ctx context.Context, playlistID int64, filter database.PlaylistSongFilter) ([]*models.Song, error)
	CountSongs(ctx context.Context, playlistID int64) (int, error)
	CountSongsFiltered(ctx context.Context, playlistID int64, keyword string) (int, error)
	BatchUpdatePositions(ctx context.Context, playlistID int64, songIDs []int64) error
	MaxPosition(ctx context.Context, playlistID int64) (int, error)
	AddSongsBatch(ctx context.Context, playlistID int64, startPos int, songIDs []int64) (added int, skipped int, err error)
	ListSongIDsOrdered(ctx context.Context, playlistID int64, sort, order string) ([]int64, error)
}

// PlayHistoryCleaner 在删除歌单时清理其播放历史。
// play_history.context_key 是 TEXT，无法对 playlists 建外键做级联，故必须显式清理。
// 可选依赖，nil 安全（未注入时只是残留少量垃圾行，歌单 ID 不会被 AUTOINCREMENT 复用，无正确性影响）。
type PlayHistoryCleaner interface {
	ClearByPlaylist(ctx context.Context, playlistID int64) (int, error)
}

// PlaylistService 歌单服务
type PlaylistService struct {
	playlists         PlaylistRepository
	playlistSongs     PlaylistSongRepository
	songs             SongRepository
	metadataExtractor *MetadataExtractor
	playHistory       PlayHistoryCleaner // 可选，nil 安全
}

// SetPlayHistoryCleaner 注入播放历史清理器，删除歌单时顺带清理其播放历史。
func (s *PlaylistService) SetPlayHistoryCleaner(c PlayHistoryCleaner) {
	s.playHistory = c
}

// clearPlayHistory 清理歌单的播放历史，失败只记日志：
// 残留行不影响任何正确性（歌单 ID 由 AUTOINCREMENT 保证不复用），不值得让删除操作失败。
func (s *PlaylistService) clearPlayHistory(ctx context.Context, playlistID int64) {
	if s.playHistory == nil {
		return
	}
	if _, err := s.playHistory.ClearByPlaylist(ctx, playlistID); err != nil {
		slog.Warn("清理歌单播放历史失败", "playlist_id", playlistID, "error", err)
	}
}

// SongIDsOrdered 返回歌单内全部歌曲 ID，顺序与分页查询歌单歌曲一致（同一套 sort/order 逻辑）。
// 供客户端定位「某首歌在歌单里排第几」用，避免为此拉取完整歌曲对象；sort/order 为空时默认 position asc。
func (s *PlaylistService) SongIDsOrdered(ctx context.Context, playlistID int64, sort, order string) ([]int64, error) {
	ids, err := s.playlistSongs.ListSongIDsOrdered(ctx, playlistID, sort, order)
	if err != nil {
		return nil, fmt.Errorf("failed to list playlist song ids: %w", err)
	}
	return ids, nil
}

// NewPlaylistService 创建歌单服务
func NewPlaylistService(playlists PlaylistRepository, playlistSongs PlaylistSongRepository, songs SongRepository, metadataExtractor *MetadataExtractor) *PlaylistService {
	return &PlaylistService{
		playlists:         playlists,
		playlistSongs:     playlistSongs,
		songs:             songs,
		metadataExtractor: metadataExtractor,
	}
}

// Create 创建歌单
func (s *PlaylistService) Create(ctx context.Context, playlist *models.Playlist) error {
	if err := playlist.Validate(); err != nil {
		return fmt.Errorf("invalid playlist data: %w", err)
	}
	if err := s.playlists.Create(ctx, playlist); err != nil {
		return fmt.Errorf("failed to create playlist: %w", err)
	}
	return nil
}

// GetByID 根据 ID 获取歌单
func (s *PlaylistService) GetByID(ctx context.Context, id int64) (*models.Playlist, error) {
	playlist, err := s.playlists.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist: %w", err)
	}
	return playlist, nil
}

// Update 更新歌单
func (s *PlaylistService) Update(ctx context.Context, playlist *models.Playlist) error {
	existing, err := s.playlists.GetByID(ctx, playlist.ID)
	if err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}

	isBuiltIn := false
	for _, label := range existing.Labels {
		if label == models.PlaylistLabelBuiltIn {
			isBuiltIn = true
			break
		}
	}

	if isBuiltIn {
		// 内置歌单：只允许更新 cover_path 和 cover_url，其他字段保持不变。
		playlist.Name = existing.Name
		playlist.Description = existing.Description
		playlist.Labels = existing.Labels
		playlist.Type = existing.Type
	} else {
		// 非内置歌单：验证歌单数据（不校验 type，type 不允许修改）。
		if err := playlist.ValidateForUpdate(); err != nil {
			return fmt.Errorf("invalid playlist data: %w", err)
		}
	}

	if err := s.playlists.Update(ctx, playlist); err != nil {
		return fmt.Errorf("failed to update playlist: %w", err)
	}
	return nil
}

// SetPinned 设置歌单置顶状态。与 Update 不同，不做内置歌单保护——置顶对内置歌单同样生效。
func (s *PlaylistService) SetPinned(ctx context.Context, id int64, pinned bool) (*models.Playlist, error) {
	var pinnedAt *time.Time
	if pinned {
		now := time.Now()
		pinnedAt = &now
	}
	if err := s.playlists.SetPinned(ctx, id, pinnedAt); err != nil {
		return nil, fmt.Errorf("failed to set playlist pinned: %w", err)
	}
	return s.GetByID(ctx, id)
}

// Touch 更新歌单的最后播放时间（updated_at）
func (s *PlaylistService) Touch(ctx context.Context, id int64) error {
	if err := s.playlists.Touch(ctx, id); err != nil {
		return fmt.Errorf("failed to touch playlist: %w", err)
	}
	return nil
}

// UpdateSort 更新歌单的视图排序偏好
func (s *PlaylistService) UpdateSort(ctx context.Context, id int64, sortBy, sortOrder string) error {
	validSortFields := map[string]bool{
		"position": true, "added_at": true, "file_modified_at": true,
		"title": true, "artist": true, "album": true, "duration": true, "updated_at": true,
		"file_size": true,
	}
	if !validSortFields[sortBy] {
		sortBy = "position"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}
	if err := s.playlists.UpdateSort(ctx, id, sortBy, sortOrder); err != nil {
		return fmt.Errorf("failed to update playlist sort: %w", err)
	}
	return nil
}

// Delete 删除歌单
func (s *PlaylistService) Delete(ctx context.Context, id int64) error {
	playlist, err := s.playlists.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}

	for _, label := range playlist.Labels {
		if label == models.PlaylistLabelBuiltIn {
			return fmt.Errorf("cannot delete built-in playlist")
		}
	}

	if err := s.playlists.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete playlist: %w", err)
	}
	s.clearPlayHistory(ctx, id)

	if playlist.CoverPath != "" {
		removeCoverIfUnreferenced(ctx, s.songs, playlist.CoverPath)
	}
	return nil
}

// BatchDelete 批量删除歌单（跳过 built_in 歌单）
func (s *PlaylistService) BatchDelete(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	// 收集候选 cover_path（去重）。仓储层会跳过 built_in 歌单，
	// 未被删除的歌单其 cover_path 行仍在表里，helper 的引用计数会自然保护它。
	// 同时记下哪些 id 真会被删掉：播放历史没有外键级联，只能显式清理，
	// 而 built_in 歌单（收藏 / 电台收藏）不会被删除，绝不能连带清空它们的历史。
	coverPathSet := make(map[string]struct{})
	deletableIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		playlist, err := s.playlists.GetByID(ctx, id)
		if err != nil {
			// 不存在是正常情况（调用方可能传了脏 id）；其余错误说明这一行读失败，
			// 它仍会被下面的 BatchDelete 删掉，但我们无从判断该不该清它的播放历史，
			// 于是留下一条残留行（无正确性影响，歌单 id 由 AUTOINCREMENT 保证不复用）。
			if !errors.Is(err, database.ErrNotFound) {
				slog.Warn("批量删除歌单：读取歌单失败，其播放历史可能残留",
					"playlist_id", id, "error", err)
			}
			continue
		}
		if playlist.CoverPath != "" {
			coverPathSet[playlist.CoverPath] = struct{}{}
		}
		if !slices.Contains(playlist.Labels, models.PlaylistLabelBuiltIn) {
			deletableIDs = append(deletableIDs, id)
		}
	}

	deleted, err := s.playlists.BatchDelete(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("failed to batch delete playlists: %w", err)
	}

	for _, id := range deletableIDs {
		s.clearPlayHistory(ctx, id)
	}

	for coverPath := range coverPathSet {
		removeCoverIfUnreferenced(ctx, s.songs, coverPath)
	}
	return deleted, nil
}

// SongIDsInPlaylist 返回歌单内全部歌曲 ID（含本地）。
// 供删除歌单时先行收集候选歌曲、删后交由 SongService.DeleteOrphanSongs 清理孤儿用。
func (s *PlaylistService) SongIDsInPlaylist(ctx context.Context, playlistID int64) ([]int64, error) {
	songs, err := s.playlistSongs.GetSongs(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist songs: %w", err)
	}
	ids := make([]int64, 0, len(songs))
	for _, song := range songs {
		ids = append(ids, song.ID)
	}
	return ids, nil
}

// List 列出歌单
func (s *PlaylistService) List(ctx context.Context, filter *database.PlaylistFilter) ([]*models.Playlist, error) {
	playlists, err := s.playlists.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list playlists: %w", err)
	}
	return playlists, nil
}

// Count 统计歌单数量
func (s *PlaylistService) Count(ctx context.Context, filter *database.PlaylistFilter) (int64, error) {
	return s.playlists.Count(ctx, filter)
}

// AddSong 添加歌曲到歌单
func (s *PlaylistService) AddSong(ctx context.Context, playlistID, songID int64) error {
	playlist, err := s.playlists.GetByID(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}

	song, err := s.songs.GetByID(ctx, songID)
	if err != nil {
		return fmt.Errorf("failed to get song: %w", err)
	}

	if !playlist.CanAddSong(song.Type) {
		return fmt.Errorf("cannot add %s to %s playlist", song.Type, playlist.Type)
	}

	maxPos, err := s.playlistSongs.MaxPosition(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("failed to get max position: %w", err)
	}
	position := maxPos + 1

	if err := s.playlistSongs.AddSong(ctx, playlistID, songID, position); err != nil {
		return fmt.Errorf("failed to add song to playlist: %w", err)
	}
	return nil
}

// AddSongs 批量添加歌曲到歌单：单事务批量插入，position 一次性算定，
// 类型不兼容或不存在的歌曲计入 skipped。已存在的歌曲由 INSERT OR IGNORE 自然跳过。
func (s *PlaylistService) AddSongs(ctx context.Context, playlistID int64, songIDs []int64) (added int, skipped int, err error) {
	if len(songIDs) == 0 {
		return 0, 0, nil
	}

	playlist, err := s.playlists.GetByID(ctx, playlistID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get playlist: %w", err)
	}

	// 一次性查所有候选歌曲的 type，过滤掉不兼容或不存在的（避免 O(N) 单首查询）。
	types, err := s.songs.ListTypesByIDs(ctx, songIDs)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list song types: %w", err)
	}
	eligible := make([]int64, 0, len(songIDs))
	for _, id := range songIDs {
		typ, ok := types[id]
		if !ok || !playlist.CanAddSong(typ) {
			skipped++
			continue
		}
		eligible = append(eligible, id)
	}

	if len(eligible) == 0 {
		return 0, skipped, nil
	}

	startPos, err := s.playlistSongs.MaxPosition(ctx, playlistID)
	if err != nil {
		return 0, skipped, fmt.Errorf("failed to get max position: %w", err)
	}

	addedN, skippedN, err := s.playlistSongs.AddSongsBatch(ctx, playlistID, startPos, eligible)
	if err != nil {
		return addedN, skipped + skippedN, fmt.Errorf("failed to add songs to playlist: %w", err)
	}
	return addedN, skipped + skippedN, nil
}

// RemoveSong 从歌单移除歌曲
func (s *PlaylistService) RemoveSong(ctx context.Context, playlistID, songID int64) error {
	if err := s.playlistSongs.RemoveSong(ctx, playlistID, songID); err != nil {
		return fmt.Errorf("failed to remove song from playlist: %w", err)
	}
	return nil
}

// GetSongs 获取歌单中的歌曲（支持分页、排序、搜索）
func (s *PlaylistService) GetSongs(ctx context.Context, playlistID int64, filter database.PlaylistSongFilter) ([]*models.Song, error) {
	songs, err := s.playlistSongs.GetSongsFiltered(ctx, playlistID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist songs: %w", err)
	}
	return songs, nil
}

// CountSongs 统计歌单中满足过滤条件的歌曲总数
func (s *PlaylistService) CountSongs(ctx context.Context, playlistID int64, keyword string) (int, error) {
	if keyword != "" {
		count, err := s.playlistSongs.CountSongsFiltered(ctx, playlistID, keyword)
		if err != nil {
			return 0, fmt.Errorf("failed to count playlist songs: %w", err)
		}
		return count, nil
	}
	count, err := s.playlistSongs.CountSongs(ctx, playlistID)
	if err != nil {
		return 0, fmt.Errorf("failed to count playlist songs: %w", err)
	}
	return count, nil
}

// ReorderSongs 重新排序歌单中的歌曲，内置收藏歌单同样允许调整成员顺序。
func (s *PlaylistService) ReorderSongs(ctx context.Context, playlistID int64, songIDs []int64) error {
	if _, err := s.playlists.GetByID(ctx, playlistID); err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}

	existingSongs, err := s.playlistSongs.GetSongs(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("failed to get playlist songs: %w", err)
	}
	if len(songIDs) != len(existingSongs) {
		return fmt.Errorf("song count mismatch")
	}
	if err := s.playlistSongs.BatchUpdatePositions(ctx, playlistID, songIDs); err != nil {
		return fmt.Errorf("failed to batch update song positions: %w", err)
	}
	return nil
}

// validSortActions 是 SortSongs 接受的 action 值白名单。
var validSortActions = map[string]bool{
	"name_asc":      true,
	"name_desc":     true,
	"number_prefix": true,
	"shuffle":       true,
}

// numPrefixRe 提取标题中第一个出现的数字。
var numPrefixRe = regexp.MustCompile(`(\d+)`)

// extractLeadingNumber 从文本中提取第一个出现的数字，返回 (数值, 是否成功)。
// 与前端 PlaylistSort.extractLeadingNumber 语义完全一致。
func extractLeadingNumber(s string) (int, bool) {
	match := numPrefixRe.FindStringSubmatch(s)
	if match == nil {
		return 0, false
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// SortSongs 服务端排序歌单内歌曲（永久排序），直接更新 position。
// action 必须是 name_asc / name_desc / number_prefix / shuffle 之一；允许内置收藏歌单。
func (s *PlaylistService) SortSongs(ctx context.Context, playlistID int64, action string) error {
	if !validSortActions[action] {
		return fmt.Errorf("invalid sort action: %s", action)
	}

	if _, err := s.playlists.GetByID(ctx, playlistID); err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}

	var songIDs []int64
	var err error

	switch action {
	case "name_asc":
		songIDs, err = s.playlistSongs.ListSongIDsOrdered(ctx, playlistID, "title", "asc")
	case "name_desc":
		songIDs, err = s.playlistSongs.ListSongIDsOrdered(ctx, playlistID, "title", "desc")
	case "number_prefix":
		songIDs, err = s.sortByNumberPrefix(ctx, playlistID)
	case "shuffle":
		songIDs, err = s.shuffleSongIDs(ctx, playlistID)
	}
	if err != nil {
		return err
	}

	if err := s.playlistSongs.BatchUpdatePositions(ctx, playlistID, songIDs); err != nil {
		return fmt.Errorf("failed to batch update song positions: %w", err)
	}
	return nil
}

// sortByNumberPrefix 按标题中第一个数字前缀排序，无数字的排在最后再按标题字母序。
func (s *PlaylistService) sortByNumberPrefix(ctx context.Context, playlistID int64) ([]int64, error) {
	songs, err := s.playlistSongs.GetSongs(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist songs: %w", err)
	}

	type songWithNum struct {
		id  int64
		num int
		has bool
	}
	items := make([]songWithNum, len(songs))
	for i, song := range songs {
		n, ok := extractLeadingNumber(song.Title)
		items[i] = songWithNum{id: song.ID, num: n, has: ok}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.has != b.has {
			return a.has // 有数字的优先
		}
		if a.has {
			return a.num < b.num
		}
		return false // 无数字的保持原顺序
	})

	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.id
	}
	return ids, nil
}

// shuffleSongIDs 随机打乱歌单内歌曲 ID 顺序。
func (s *PlaylistService) shuffleSongIDs(ctx context.Context, playlistID int64) ([]int64, error) {
	songs, err := s.playlistSongs.GetSongs(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist songs: %w", err)
	}
	ids := make([]int64, len(songs))
	for i, song := range songs {
		ids[i] = song.ID
	}
	rand.Shuffle(len(ids), func(i, j int) {
		ids[i], ids[j] = ids[j], ids[i]
	})
	return ids, nil
}

// MoveSong 移动歌单中单首歌曲到 afterSongID 之后（nil 表示移到最前面）。
// 内置收藏歌单与普通歌单使用相同的成员排序规则。
func (s *PlaylistService) MoveSong(ctx context.Context, playlistID int64, songID int64, afterSongID *int64) error {
	if _, err := s.playlists.GetByID(ctx, playlistID); err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}

	currentOrder, err := s.playlistSongs.ListSongIDsOrdered(ctx, playlistID, "", "")
	if err != nil {
		return fmt.Errorf("failed to get song order: %w", err)
	}

	songIdx := -1
	for i, id := range currentOrder {
		if id == songID {
			songIdx = i
			break
		}
	}
	if songIdx == -1 {
		return fmt.Errorf("song %d not found in playlist", songID)
	}

	if afterSongID != nil {
		found := false
		for _, id := range currentOrder {
			if id == *afterSongID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("after_song_id %d not found in playlist", *afterSongID)
		}
		if songID == *afterSongID {
			return nil
		}
	}

	newOrder := make([]int64, 0, len(currentOrder))
	for _, id := range currentOrder {
		if id != songID {
			newOrder = append(newOrder, id)
		}
	}

	if afterSongID == nil {
		newOrder = slices.Insert(newOrder, 0, songID)
	} else {
		for i, id := range newOrder {
			if id == *afterSongID {
				newOrder = slices.Insert(newOrder, i+1, songID)
				break
			}
		}
	}

	if err := s.playlistSongs.BatchUpdatePositions(ctx, playlistID, newOrder); err != nil {
		return fmt.Errorf("failed to update song positions: %w", err)
	}
	return nil
}

// ReorderPlaylists 重新排序歌单列表
func (s *PlaylistService) ReorderPlaylists(ctx context.Context, playlistIDs []int64) error {
	// allPlaylists 按 position 升序返回（含隐藏歌单）。
	allPlaylists, err := s.playlists.List(ctx, &database.PlaylistFilter{Limit: 0})
	if err != nil {
		return fmt.Errorf("failed to list playlists: %w", err)
	}

	// 校验请求的 ID 均存在且不重复。
	existing := make(map[int64]bool, len(allPlaylists))
	for _, p := range allPlaylists {
		existing[p.ID] = true
	}
	reorderSet := make(map[int64]bool, len(playlistIDs))
	for _, id := range playlistIDs {
		if reorderSet[id] {
			return fmt.Errorf("duplicate playlist id: %d", id)
		}
		if !existing[id] {
			return fmt.Errorf("playlist %d not found", id)
		}
		reorderSet[id] = true
	}

	// 前端列表默认排除隐藏歌单，因此这里可能只收到可见歌单的子集。
	// 部分重排时：请求中的歌单按新顺序依次填充它们原先占据的槽位，
	// 未纳入排序的歌单（如隐藏的总目录歌单）保持原位，避免 position 错乱。
	finalOrder := make([]int64, 0, len(allPlaylists))
	cursor := 0
	for _, p := range allPlaylists {
		if reorderSet[p.ID] {
			finalOrder = append(finalOrder, playlistIDs[cursor])
			cursor++
		} else {
			finalOrder = append(finalOrder, p.ID)
		}
	}

	if err := s.playlists.BatchUpdatePositions(ctx, finalOrder); err != nil {
		return fmt.Errorf("failed to batch update playlist positions: %w", err)
	}
	return nil
}

// UploadCover 上传歌单封面图片
func (s *PlaylistService) UploadCover(ctx context.Context, playlistID int64, coverData []byte, coverExt string) (*models.Playlist, error) {
	playlist, err := s.playlists.GetByID(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist: %w", err)
	}

	metadata := &Metadata{
		HasCover:  true,
		CoverData: coverData,
		CoverExt:  coverExt,
	}
	coverPath, err := s.metadataExtractor.SaveCover(playlistID, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to save cover: %w", err)
	}

	playlist.CoverPath = coverPath
	playlist.CoverURL = "" // 清空 CoverURL，使用本地路径
	if err := s.playlists.Update(ctx, playlist); err != nil {
		return nil, fmt.Errorf("failed to update playlist: %w", err)
	}
	return playlist, nil
}
