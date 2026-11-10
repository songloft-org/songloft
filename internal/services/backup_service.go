package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"songloft/internal/database"
	"songloft/internal/models"
)

// BackupService 歌单备份还原服务
type BackupService struct {
	db *database.DB
}

// NewBackupService 创建备份服务
func NewBackupService(db database.DB) *BackupService {
	return &BackupService{db: &db}
}

// Export 导出所有歌单及其歌曲元数据
func (s *BackupService) Export(ctx context.Context) (*models.BackupData, error) {
	db := *s.db
	playlists, err := db.PlaylistRepository().List(ctx, &database.PlaylistFilter{
		Limit: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}

	backupPlaylists := make([]models.BackupPlaylist, 0, len(playlists))
	for _, p := range playlists {
		songs, err := db.PlaylistSongRepository().GetSongs(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("get songs for playlist %q: %w", p.Name, err)
		}

		backupSongs := make([]models.BackupSong, 0, len(songs))
		for _, song := range songs {
			bs := songToBackup(song)
			tags, err := db.SongTagRepository().GetBySongID(ctx, song.ID)
			if err != nil {
				return nil, err
			}
			for _, tag := range tags {
				bs.TagIDs = append(bs.TagIDs, tag.ID)
			}
			backupSongs = append(backupSongs, bs)
		}

		labels := []string{}
		tagIDs := []int64{}
		for _, label := range p.Labels {
			if id, ok := models.ParsePlaylistTagLabel(label); ok {
				tagIDs = append(tagIDs, id)
			} else if !strings.HasPrefix(label, "tag:") {
				labels = append(labels, label)
			}
		}

		backupPlaylists = append(backupPlaylists, models.BackupPlaylist{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Labels:      labels,
			Songs:       backupSongs,
			TagIDs:      tagIDs,
		})
	}

	tags, err := db.SongTagRepository().List(ctx, "", "id", "asc", 0, 0)
	if err != nil {
		return nil, err
	}
	definitions := make([]models.BackupTag, 0, len(tags))
	for _, tag := range tags {
		definitions = append(definitions, models.BackupTag{ID: tag.ID, Name: tag.Name, Color: tag.Color})
	}
	return &models.BackupData{
		Version:    models.BackupVersion,
		ExportedAt: time.Now(),
		Playlists:  backupPlaylists,
		Tags:       definitions,
	}, nil
}

// Import 从备份数据导入歌单，同名歌单合并歌曲
func (s *BackupService) Import(ctx context.Context, data *models.BackupData) (*models.ImportResult, error) {
	if data.Version != 1 && data.Version != models.BackupVersion {
		return nil, fmt.Errorf("不支持的备份版本: %d", data.Version)
	}

	result := &models.ImportResult{}

	err := (*s.db).RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		tagMap := make(map[int64]int64)
		if data.Version >= 2 {
			for _, tag := range data.Tags {
				if tag.ID <= 0 || tagMap[tag.ID] != 0 {
					return fmt.Errorf("invalid backup tag ID: %d", tag.ID)
				}
				if err := validateTagName(tag.Name); err != nil {
					return err
				}
				existing, err := uow.SongTags.GetByName(ctx, tag.Name)
				if err == nil {
					tagMap[tag.ID] = existing.ID
					continue
				}
				if !errors.Is(err, database.ErrNotFound) {
					return err
				}
				id, err := uow.SongTags.Create(ctx, tag.Name, tag.Color)
				if err != nil {
					return err
				}
				tagMap[tag.ID] = id
			}
		}
		localPaths, err := uow.Songs.ListLocalPaths(ctx)
		if err != nil {
			return fmt.Errorf("load local paths: %w", err)
		}

		for _, bp := range data.Playlists {
			if err := s.importPlaylist(ctx, uow, &bp, localPaths, result, tagMap); err != nil {
				return fmt.Errorf("import playlist %q: %w", bp.Name, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *BackupService) importPlaylist(
	ctx context.Context,
	uow *database.UnitOfWork,
	bp *models.BackupPlaylist,
	localPaths map[string]database.LocalPathInfo,
	result *models.ImportResult,
	tagMap map[int64]int64,
) error {
	songIDs := make([]int64, 0, len(bp.Songs))
	for _, bs := range bp.Songs {
		songID := s.matchOrCreateSong(ctx, uow, &bs, localPaths, result)
		if songID > 0 {
			songIDs = append(songIDs, songID)
			for _, oldID := range bs.TagIDs {
				if id := tagMap[oldID]; id > 0 {
					if err := uow.SongTags.LinkSongTag(ctx, songID, id); err != nil {
						return err
					}
				}
			}
		}
	}

	playlistID, err := s.findOrCreatePlaylist(ctx, uow, bp, result)
	if err != nil {
		return err
	}
	if len(bp.TagIDs) > 0 {
		existing, err := uow.SongTags.GetByPlaylistID(ctx, playlistID)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(existing)+len(bp.TagIDs))
		for _, tag := range existing {
			ids = append(ids, tag.ID)
		}
		for _, oldID := range bp.TagIDs {
			if id := tagMap[oldID]; id > 0 {
				ids = append(ids, id)
			}
		}
		if err := uow.SongTags.SetPlaylistTags(ctx, playlistID, ids); err != nil {
			return err
		}
	}

	existingCount, err := uow.PlaylistSongs.CountSongs(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("count existing songs: %w", err)
	}
	nextPos := existingCount + 1

	for _, songID := range songIDs {
		inserted, err := uow.PlaylistSongs.AddSongIgnore(ctx, playlistID, songID, nextPos)
		if err != nil {
			return fmt.Errorf("add song %d to playlist: %w", songID, err)
		}
		if inserted {
			nextPos++
		} else {
			result.SongsSkipped++
		}
	}

	return nil
}

func (s *BackupService) matchOrCreateSong(
	ctx context.Context,
	uow *database.UnitOfWork,
	bs *models.BackupSong,
	localPaths map[string]database.LocalPathInfo,
	result *models.ImportResult,
) int64 {
	switch bs.Type {
	case models.TypeLocal:
		if info, ok := localPaths[bs.FilePath]; ok {
			result.SongsMatched++
			return info.SongID
		}
		slog.Debug("跳过本地歌曲（文件不在库中）", "file_path", bs.FilePath)
		return 0

	case models.TypeRemote, models.TypeRadio:
		song := backupToSong(bs)
		// 先查找是否已存在
		existed := false
		if bs.PluginEntryPath != "" && bs.DedupKey != "" {
			if _, err := uow.Songs.FindByDedupKey(ctx, bs.PluginEntryPath, bs.DedupKey); err == nil {
				existed = true
			}
		}
		if err := uow.Songs.UpsertRemote(ctx, song); err != nil {
			slog.Warn("导入歌曲失败", "title", bs.Title, "error", err)
			return 0
		}
		if existed {
			result.SongsMatched++
		} else {
			result.SongsCreated++
		}
		return song.ID

	default:
		slog.Warn("跳过未知类型歌曲", "type", bs.Type, "title", bs.Title)
		return 0
	}
}

func (s *BackupService) findOrCreatePlaylist(
	ctx context.Context,
	uow *database.UnitOfWork,
	bp *models.BackupPlaylist,
	result *models.ImportResult,
) (int64, error) {
	existing, err := uow.Playlists.FindByName(ctx, bp.Name)
	if err == nil {
		result.PlaylistsMerged++
		return existing.ID, nil
	}
	if err != database.ErrNotFound {
		return 0, fmt.Errorf("find playlist by name: %w", err)
	}

	labels := bp.Labels
	if labels == nil {
		labels = []string{}
	}
	// 不创建新的 built_in 歌单
	filtered := make([]string, 0, len(labels))
	for _, l := range labels {
		if l != models.PlaylistLabelBuiltIn && !strings.HasPrefix(l, "tag:") {
			filtered = append(filtered, l)
		}
	}

	playlist := &models.Playlist{
		Name:        bp.Name,
		Type:        bp.Type,
		Description: bp.Description,
		Labels:      filtered,
	}
	if err := uow.Playlists.Create(ctx, playlist); err != nil {
		return 0, fmt.Errorf("create playlist: %w", err)
	}
	result.PlaylistsCreated++
	return playlist.ID, nil
}

func songToBackup(s *models.Song) models.BackupSong {
	return models.BackupSong{
		Type:            s.Type,
		Title:           s.Title,
		Artist:          s.Artist,
		Album:           s.Album,
		Duration:        s.Duration,
		FilePath:        s.FilePath,
		URL:             s.URL,
		CoverURL:        s.CoverURL,
		FileSize:        s.FileSize,
		Format:          s.Format,
		BitRate:         s.BitRate,
		SampleRate:      s.SampleRate,
		IsLive:          s.IsLive,
		PluginEntryPath: s.PluginEntryPath,
		SourceData:      s.SourceData,
		DedupKey:        s.DedupKey,
	}
}

func backupToSong(bs *models.BackupSong) *models.Song {
	return &models.Song{
		Type:            bs.Type,
		Title:           bs.Title,
		Artist:          bs.Artist,
		Album:           bs.Album,
		Duration:        bs.Duration,
		FilePath:        bs.FilePath,
		URL:             bs.URL,
		CoverURL:        bs.CoverURL,
		FileSize:        bs.FileSize,
		Format:          bs.Format,
		BitRate:         bs.BitRate,
		SampleRate:      bs.SampleRate,
		IsLive:          bs.IsLive,
		PluginEntryPath: bs.PluginEntryPath,
		SourceData:      bs.SourceData,
		DedupKey:        bs.DedupKey,
	}
}
