package services

import (
	"context"
	"errors"
	"fmt"

	"songloft/internal/database"
	"songloft/internal/models"
)

// PlaylistRepository 是 PlaylistService 依赖的歌单仓储接口。
type PlaylistRepository interface {
	Create(ctx context.Context, playlist *models.Playlist) error
	GetByID(ctx context.Context, id int64) (*models.Playlist, error)
	Update(ctx context.Context, playlist *models.Playlist) error
	Touch(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter *database.PlaylistFilter) ([]*models.Playlist, error)
	Count(ctx context.Context, filter *database.PlaylistFilter) (int64, error)
	BatchDelete(ctx context.Context, ids []int64) (int, error)
	BatchUpdatePositions(ctx context.Context, playlistIDs []int64) error
}

// PlaylistSongRepository 是 PlaylistService 依赖的歌单-歌曲关联仓储接口。
type PlaylistSongRepository interface {
	AddSong(ctx context.Context, playlistID, songID int64, position int) error
	RemoveSong(ctx context.Context, playlistID, songID int64) error
	GetSongs(ctx context.Context, playlistID int64) ([]*models.Song, error)
	GetSongsPaginated(ctx context.Context, playlistID int64, limit, offset int) ([]*models.Song, error)
	CountSongs(ctx context.Context, playlistID int64) (int, error)
	BatchUpdatePositions(ctx context.Context, playlistID int64, songIDs []int64) error
}

// PlaylistService 歌单服务
type PlaylistService struct {
	playlists         PlaylistRepository
	playlistSongs     PlaylistSongRepository
	songs             SongRepository
	metadataExtractor *MetadataExtractor
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

// Touch 更新歌单的最后播放时间（updated_at）
func (s *PlaylistService) Touch(ctx context.Context, id int64) error {
	if err := s.playlists.Touch(ctx, id); err != nil {
		return fmt.Errorf("failed to touch playlist: %w", err)
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
	coverPathSet := make(map[string]struct{})
	for _, id := range ids {
		playlist, err := s.playlists.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				continue
			}
			continue
		}
		if playlist.CoverPath != "" {
			coverPathSet[playlist.CoverPath] = struct{}{}
		}
	}

	deleted, err := s.playlists.BatchDelete(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("failed to batch delete playlists: %w", err)
	}

	for coverPath := range coverPathSet {
		removeCoverIfUnreferenced(ctx, s.songs, coverPath)
	}
	return deleted, nil
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

	songs, err := s.playlistSongs.GetSongs(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("failed to get playlist songs: %w", err)
	}
	position := len(songs) + 1

	if err := s.playlistSongs.AddSong(ctx, playlistID, songID, position); err != nil {
		return fmt.Errorf("failed to add song to playlist: %w", err)
	}
	return nil
}

// AddSongs 批量添加歌曲到歌单，跳过已存在的歌曲
func (s *PlaylistService) AddSongs(ctx context.Context, playlistID int64, songIDs []int64) (added int, skipped int, err error) {
	for _, songID := range songIDs {
		if addErr := s.AddSong(ctx, playlistID, songID); addErr != nil {
			skipped++
			continue
		}
		added++
	}
	return added, skipped, nil
}

// RemoveSong 从歌单移除歌曲
func (s *PlaylistService) RemoveSong(ctx context.Context, playlistID, songID int64) error {
	if err := s.playlistSongs.RemoveSong(ctx, playlistID, songID); err != nil {
		return fmt.Errorf("failed to remove song from playlist: %w", err)
	}
	return nil
}

// GetSongs 获取歌单中的歌曲（支持分页）
func (s *PlaylistService) GetSongs(ctx context.Context, playlistID int64, limit, offset int) ([]*models.Song, error) {
	songs, err := s.playlistSongs.GetSongsPaginated(ctx, playlistID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist songs: %w", err)
	}
	return songs, nil
}

// CountSongs 统计歌单中的歌曲总数
func (s *PlaylistService) CountSongs(ctx context.Context, playlistID int64) (int, error) {
	count, err := s.playlistSongs.CountSongs(ctx, playlistID)
	if err != nil {
		return 0, fmt.Errorf("failed to count playlist songs: %w", err)
	}
	return count, nil
}

// ReorderSongs 重新排序歌单中的歌曲
func (s *PlaylistService) ReorderSongs(ctx context.Context, playlistID int64, songIDs []int64) error {
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

// ReorderPlaylists 重新排序歌单列表
func (s *PlaylistService) ReorderPlaylists(ctx context.Context, playlistIDs []int64) error {
	allPlaylists, err := s.playlists.List(ctx, &database.PlaylistFilter{Limit: 0})
	if err != nil {
		return fmt.Errorf("failed to list playlists: %w", err)
	}
	if len(playlistIDs) != len(allPlaylists) {
		return fmt.Errorf("playlist count mismatch: expected %d, got %d", len(allPlaylists), len(playlistIDs))
	}
	if err := s.playlists.BatchUpdatePositions(ctx, playlistIDs); err != nil {
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
