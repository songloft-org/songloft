package services

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/hanxi/tag"

	"songloft/internal/database"
	"songloft/internal/fileutil"
	"songloft/internal/models"
)

// SupportsMetadataOverrides identifies the local formats covered by the metadata editor.
func SupportsMetadataOverrides(song *models.Song) bool {
	if song == nil || song.Type != models.TypeLocal || song.CueSourcePath != "" {
		return false
	}
	switch strings.ToLower(filepath.Ext(song.FilePath)) {
	case ".dsf", ".wv", ".mov", ".wav", ".aac", ".dff", ".mka", ".mkv", ".webm",
		".wma", ".wmv", ".avi", ".flv", ".rm", ".rmvb", ".dts", ".ts", ".mpg", ".mpeg":
		return true
	}
	return false
}

var metadataFieldBits = map[string]models.SongMetadataMask{
	tag.FieldTitle:    models.MetadataOverrideTitle,
	tag.FieldArtist:   models.MetadataOverrideArtist,
	tag.FieldAlbum:    models.MetadataOverrideAlbum,
	tag.FieldYear:     models.MetadataOverrideYear,
	tag.FieldGenre:    models.MetadataOverrideGenre,
	tag.FieldLanguage: models.MetadataOverrideLanguage,
	tag.FieldStyle:    models.MetadataOverrideStyle,
	tag.FieldTrack:    models.MetadataOverrideTrack,
	tag.FieldDisc:     models.MetadataOverrideDisc,
	tag.FieldLyrics:   models.MetadataOverrideLyrics,
	tag.FieldPicture:  models.MetadataOverridePicture,
}

func IsMetadataField(field string) bool {
	return metadataFieldBits[field] != 0
}

func copyMetadataOverride(dst, src *models.Song, field string) {
	switch field {
	case tag.FieldTitle:
		dst.Title = src.Title
	case tag.FieldArtist:
		dst.Artist = src.Artist
	case tag.FieldAlbum:
		dst.Album = src.Album
	case tag.FieldYear:
		dst.Year = src.Year
	case tag.FieldGenre:
		dst.Genre = src.Genre
	case tag.FieldLanguage:
		dst.Language = src.Language
	case tag.FieldStyle:
		dst.Style = src.Style
	case tag.FieldTrack:
		dst.Track = src.Track
	case tag.FieldDisc:
		dst.Disc = src.Disc
	case tag.FieldLyrics:
		dst.Lyric, dst.LyricSource, dst.LyricRemoteURL = src.Lyric, src.LyricSource, src.LyricRemoteURL
	case tag.FieldPicture:
		dst.CoverPath, dst.CoverURL = src.CoverPath, src.CoverURL
	}
}

// EditLocalMetadata saves the edit and its markers together. The file write
// follows the commit and reports its result independently.
func (s *SongService) EditLocalMetadata(ctx context.Context, edits *models.Song, fields []string, rename bool) (*models.Song, FileWriteStatus, error) {
	lock := &s.metadataEditLocks[uint64(edits.ID)%uint64(len(s.metadataEditLocks))]
	lock.Lock()
	defer lock.Unlock()

	if !SupportsMetadataOverrides(edits) || s.tx == nil {
		if rename && edits.CueSourcePath == "" {
			if _, err := s.RenameLocalSongFile(ctx, edits, edits.Title); err != nil {
				return nil, "", err
			}
		} else if err := s.Update(ctx, edits); err != nil {
			return nil, "", err
		}
		if edits.CueSourcePath != "" {
			return edits, FileWriteUnchanged, nil
		}
		return edits, WriteSongTags(edits.FilePath, edits), nil
	}

	var song *models.Song
	var originalPath string
	err := s.tx.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		current, err := uow.Songs.GetByID(ctx, edits.ID)
		if err != nil {
			return err
		}
		originalPath = current.FilePath
		var mask models.SongMetadataMask
		for _, field := range fields {
			copyMetadataOverride(current, edits, field)
			mask |= metadataFieldBits[field]
		}
		if mask != 0 {
			if err := uow.Songs.MarkMetadataOverrides(ctx, current.ID, mask, time.Now().UnixNano()); err != nil {
				return err
			}
		}

		if slices.Contains(fields, tag.FieldArtist) {
			existing, err := uow.SongArtists.GetBySongID(ctx, current.ID)
			if err != nil {
				return err
			}
			var inputs []models.ArtistInput
			for _, a := range existing {
				if a.Role != models.ArtistRoleArtist {
					inputs = append(inputs, models.ArtistInput{Name: a.Artist.Name, Role: a.Role, Position: a.Position})
				}
			}
			for _, name := range strings.Split(current.Artist, " & ") {
				if name = strings.TrimSpace(name); name != "" {
					inputs = append(inputs, models.ArtistInput{Name: name, Role: models.ArtistRoleArtist, Position: len(inputs)})
				}
			}
			if err := uow.SongArtists.SetSongArtists(ctx, current.ID, inputs); err != nil {
				return err
			}
		}
		song = current
		if rename {
			_, err := s.renameLocalSongFile(ctx, current, current.Title, uow.Songs.Update)
			return err
		}
		return uow.Songs.Update(ctx, current)
	})
	if err != nil {
		if song != nil && song.FilePath != originalPath {
			if rollbackErr := fileutil.MoveFile(song.FilePath, originalPath); rollbackErr != nil {
				return nil, "", fmt.Errorf("save metadata: %w; restore filename: %v", err, rollbackErr)
			}
		}
		return nil, "", err
	}
	if len(fields) == 0 {
		fields = nonemptyLocalMetadataFields(song)
	}
	return song, writeEditedSongTags(song, fields), nil
}

func resetMetadataField(dst *models.Song, m *Metadata, field string) {
	switch field {
	case tag.FieldLanguage:
		dst.Language = m.Language
	case tag.FieldStyle:
		dst.Style = m.Style
	case tag.FieldLyrics:
		dst.Lyric, dst.LyricSource, dst.LyricRemoteURL = "", "", ""
		models.ApplyLyricToSong(dst, m.Lyric, m.LyricSource)
	case tag.FieldPicture:
		dst.CoverPath, dst.CoverURL = m.CoverPath, ""
	}
}

func nonemptyLocalMetadataFields(s *models.Song) []string {
	var fields []string
	for _, entry := range []struct {
		field   string
		present bool
	}{
		{tag.FieldTitle, s.Title != ""}, {tag.FieldArtist, s.Artist != ""}, {tag.FieldAlbum, s.Album != ""},
		{tag.FieldYear, s.Year > 0}, {tag.FieldGenre, s.Genre != ""}, {tag.FieldLanguage, s.Language != ""},
		{tag.FieldStyle, s.Style != ""}, {tag.FieldTrack, s.Track != ""}, {tag.FieldDisc, s.Disc != ""},
		{tag.FieldLyrics, s.Lyric != "" && s.LyricSource != models.LyricSourceURL},
		{tag.FieldPicture, s.CoverPath != ""},
	} {
		if entry.present {
			fields = append(fields, entry.field)
		}
	}
	return fields
}
