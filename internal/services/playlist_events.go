package services

import (
	"time"

	"songloft/internal/models"
)

// SetOnSongsChange installs a post-commit observer during application setup.
// The observer must enqueue work without waiting for plugin callbacks.
func (s *PlaylistService) SetOnSongsChange(fn func([]models.PlaylistSongsChangeEvent)) {
	s.onSongsChange = fn
}

func playlistSongChange(playlistID int64, song *models.Song, kind string) models.PlaylistSongsChangeEvent {
	return models.PlaylistSongsChangeEvent{
		PlaylistID: playlistID, SongID: song.ID, Type: kind,
		Song: models.PlaylistEventSong{ID: song.ID, Title: song.Title, Artist: song.Artist},
	}
}

func (s *PlaylistService) notifySongsChange(changes []models.PlaylistSongsChangeEvent) {
	if len(changes) == 0 {
		return
	}
	for _, change := range changes {
		if (change.PlaylistID == 1 || change.PlaylistID == 2) && s.favoriteChanges != nil {
			s.favoriteChanges.notify()
			break
		}
	}
	if s.onSongsChange == nil {
		return
	}
	now := time.Now().UnixMilli()
	for i := range changes {
		changes[i].Timestamp = now
	}
	s.onSongsChange(changes)
}
