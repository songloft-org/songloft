package services

import (
	"context"
	"errors"
	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"testing"
)

// Clearing an absent checkpoint also fences off in-flight first writes.
func TestPlaylistProgressClearAbsent(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	s := NewPlaylistProgressService(db)
	ctx := context.Background()
	if err := s.Clear(ctx, 1); err != nil {
		t.Fatal(err)
	}
	mode := "order"
	_, err := s.Save(ctx, 1, models.UpdatePlaylistProgress{Version: 0, PlayMode: &mode})
	if !errors.Is(err, database.ErrProgressConflict) {
		t.Fatalf("clear allowed an old first write: %v", err)
	}
}

func TestPlaylistProgress(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	s := NewPlaylistProgressService(db)
	ctx := context.Background()
	p := &models.Playlist{Type: "normal", Name: "book"}
	if err := db.PlaylistRepository().Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	songs := seedPlayHistorySongs(t, NewPlayHistoryService(db), 2)
	if err := db.PlaylistSongRepository().AddSong(ctx, p.ID, songs[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	empty, err := s.Get(ctx, p.ID)
	if err != nil || empty.Version != 0 {
		t.Fatalf("empty: %+v %v", empty, err)
	}
	mode := "single"
	pos := int64(754000)
	sid := songs[0].ID
	saved, err := s.Save(ctx, p.ID, models.UpdatePlaylistProgress{SongID: &sid, PositionMS: &pos, PlayMode: &mode})
	if err != nil || saved.Song.ID != sid || saved.PositionMS != pos || saved.Version != 1 {
		t.Fatalf("save: %+v %v", saved, err)
	}
	_, err = s.Save(ctx, p.ID, models.UpdatePlaylistProgress{Version: 0, SongID: &sid})
	if !errors.Is(err, database.ErrProgressConflict) {
		t.Fatalf("stale overwrite: %v", err)
	}
	mode = "order"
	saved, err = s.Save(ctx, p.ID, models.UpdatePlaylistProgress{Version: 1, PlayMode: &mode})
	if err != nil || saved.PositionMS != pos || saved.PlayMode != mode {
		t.Fatalf("mode overwrote position: %+v %v", saved, err)
	}
	missing := songs[1].ID
	_, err = s.Save(ctx, p.ID, models.UpdatePlaylistProgress{Version: 2, SongID: &missing})
	if !errors.Is(err, ErrInvalidProgress) {
		t.Fatalf("foreign member: %v", err)
	}
	if err := db.PlaylistSongRepository().RemoveSong(ctx, p.ID, sid); err != nil {
		t.Fatal(err)
	}
	saved, err = s.Get(ctx, p.ID)
	if err != nil || !saved.Invalid || saved.Song != nil || saved.PlayMode != mode {
		t.Fatalf("invalid: %+v %v", saved, err)
	}
	if err := s.Clear(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	saved, err = s.Get(ctx, p.ID)
	if err != nil || saved.Version != 3 || saved.Invalid || saved.PlayMode != mode {
		t.Fatalf("clear: %+v %v", saved, err)
	}
	_, err = s.Save(ctx, p.ID, models.UpdatePlaylistProgress{Version: 2, PlayMode: &mode})
	if !errors.Is(err, database.ErrProgressConflict) {
		t.Fatalf("clear allowed stale write: %v", err)
	}
}
