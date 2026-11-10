package services

import (
	"context"
	"fmt"
	"strings"

	"songloft/internal/database"
	"songloft/internal/models"
)

// SongTagRepository 是 SongTagService 依赖的标签仓储接口。
type SongTagRepository interface {
	Create(ctx context.Context, name, color string) (int64, error)
	GetByID(ctx context.Context, id int64) (*models.SongTag, error)
	GetByName(ctx context.Context, name string) (*models.SongTag, error)
	Update(ctx context.Context, id int64, name, color string) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, keyword, orderBy, order string, limit, offset int) ([]models.SongTag, error)
	Count(ctx context.Context, keyword string) (int64, error)
	GetBySongID(ctx context.Context, songID int64) ([]models.SongTag, error)
	SetSongTags(ctx context.Context, songID int64, tagIDs []int64) error
	LinkSongTag(ctx context.Context, songID, tagID int64) error
	UnlinkSongTag(ctx context.Context, songID, tagID int64) error
	ListSongIDs(ctx context.Context, tagID int64, limit, offset int) ([]int64, error)
	CountSongs(ctx context.Context, tagID int64) (int64, error)
}

// SongTagService 自定义标签服务
type SongTagService struct {
	db   database.DB
	tags SongTagRepository
}

func NewSongTagService(tags SongTagRepository, db database.DB) *SongTagService {
	return &SongTagService{tags: tags, db: db}
}

func (s *SongTagService) Create(ctx context.Context, name, color string) (*models.SongTag, error) {
	name = strings.TrimSpace(name)
	if err := validateTagName(name); err != nil {
		return nil, err
	}
	id, err := s.tags.Create(ctx, name, color)
	if err != nil {
		return nil, fmt.Errorf("song_tag_service: create: %w", err)
	}
	return s.tags.GetByID(ctx, id)
}

func (s *SongTagService) GetByID(ctx context.Context, id int64) (*models.SongTag, error) {
	return s.tags.GetByID(ctx, id)
}

func (s *SongTagService) Update(ctx context.Context, id int64, name, color string) error {
	name = strings.TrimSpace(name)
	if err := validateTagName(name); err != nil {
		return err
	}
	tag, err := s.tags.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if name == "" {
		name = tag.Name
	}
	if color == "" {
		color = tag.Color
	}
	return s.tags.Update(ctx, id, name, color)
}

func (s *SongTagService) Delete(ctx context.Context, id int64) error {
	return s.db.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		if err := uow.SongTags.ClearPlaylistReferences(ctx, id); err != nil {
			return err
		}
		if _, err := uow.PlayHistory.ClearByTag(ctx, id); err != nil {
			return err
		}
		return uow.SongTags.Delete(ctx, id)
	})
}

func (s *SongTagService) GetPlaylistTags(ctx context.Context, id int64) ([]models.SongTag, error) {
	if s.db == nil {
		return nil, fmt.Errorf("playlist tags require database")
	}
	return s.db.SongTagRepository().GetByPlaylistID(ctx, id)
}

func (s *SongTagService) SetPlaylistTags(ctx context.Context, id int64, tagIDs []int64) error {
	if s.db == nil {
		return fmt.Errorf("playlist tags require database")
	}
	return s.db.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		return uow.SongTags.SetPlaylistTags(ctx, id, tagIDs)
	})
}

func (s *SongTagService) BindPlaylists(ctx context.Context, tagID int64, ids []int64, unbind bool) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("playlist tags require database")
	}
	count := 0
	err := s.db.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		if _, err := uow.SongTags.GetByID(ctx, tagID); err != nil {
			return err
		}
		seen := make(map[int64]bool)
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			tags, err := uow.SongTags.GetByPlaylistID(ctx, id)
			if err != nil {
				return err
			}
			tagIDs := make([]int64, 0, len(tags)+1)
			for _, tag := range tags {
				if tag.ID != tagID {
					tagIDs = append(tagIDs, tag.ID)
				}
			}
			if !unbind {
				tagIDs = append(tagIDs, tagID)
			}
			if err := uow.SongTags.SetPlaylistTags(ctx, id, tagIDs); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SongTagService) List(ctx context.Context, keyword, orderBy, order string, limit, offset int) ([]models.SongTag, error) {
	return s.tags.List(ctx, keyword, orderBy, order, limit, offset)
}

func (s *SongTagService) Count(ctx context.Context, keyword string) (int64, error) {
	return s.tags.Count(ctx, keyword)
}

func (s *SongTagService) GetSongTags(ctx context.Context, songID int64) ([]models.SongTag, error) {
	return s.tags.GetBySongID(ctx, songID)
}

func (s *SongTagService) SetSongTags(ctx context.Context, songID int64, tagIDs []int64) error {
	return s.tags.SetSongTags(ctx, songID, tagIDs)
}

// BindByNames 按标签名列表为歌曲关联标签（不存在则自动创建）。
// 用于扫描时从文件 SONGLOFT_TAGS 字段导入。不会删除已有绑定。
func (s *SongTagService) BindByNames(ctx context.Context, songID int64, tagNames []string) error {
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 50 {
			continue
		}
		existing, err := s.tags.GetByName(ctx, name)
		var tagID int64
		if err != nil {
			tagID, err = s.tags.Create(ctx, name, "")
			if err != nil {
				return fmt.Errorf("song_tag_service: create tag %q: %w", name, err)
			}
		} else {
			tagID = existing.ID
		}
		_ = s.tags.LinkSongTag(ctx, songID, tagID)
	}
	return nil
}

func (s *SongTagService) BatchBind(ctx context.Context, tagID int64, songIDs []int64) (int, error) {
	bound := 0
	for _, songID := range songIDs {
		if err := s.tags.LinkSongTag(ctx, songID, tagID); err != nil {
			return bound, fmt.Errorf("song_tag_service: bind song %d: %w", songID, err)
		}
		bound++
	}
	return bound, nil
}

func (s *SongTagService) BatchUnbind(ctx context.Context, tagID int64, songIDs []int64) (int, error) {
	unbound := 0
	for _, songID := range songIDs {
		if err := s.tags.UnlinkSongTag(ctx, songID, tagID); err != nil {
			return unbound, fmt.Errorf("song_tag_service: unbind song %d: %w", songID, err)
		}
		unbound++
	}
	return unbound, nil
}

func (s *SongTagService) ListSongIDs(ctx context.Context, tagID int64, limit, offset int) ([]int64, error) {
	return s.tags.ListSongIDs(ctx, tagID, limit, offset)
}

func (s *SongTagService) CountSongs(ctx context.Context, tagID int64) (int64, error) {
	return s.tags.CountSongs(ctx, tagID)
}

func validateTagName(name string) error {
	if name == "" {
		return fmt.Errorf("标签名不能为空")
	}
	if len([]rune(name)) > 50 {
		return fmt.Errorf("标签名不能超过 50 个字符")
	}
	if strings.Contains(name, ",") {
		return fmt.Errorf("标签名不能包含逗号")
	}
	return nil
}
