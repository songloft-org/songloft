package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func TestFavoriteChangesWaitAndReconnect(t *testing.T) {
	c := newFavoriteChanges()
	ctx := context.Background()
	initial, err := c.wait(ctx, "", time.Second)
	if err != nil || !initial.Changed || initial.Cursor == "" {
		t.Fatalf("initial: %+v %v", initial, err)
	}
	timeout, err := c.wait(ctx, initial.Cursor, time.Millisecond)
	if err != nil || timeout.Changed || timeout.Cursor != initial.Cursor {
		t.Fatalf("timeout: %+v %v", timeout, err)
	}
	// Multiple waiters and a write racing subscription must all observe the change.
	results := make(chan FavoriteChangesResponse, 16)
	for range 16 {
		go func() { got, _ := c.wait(ctx, initial.Cursor, time.Second); results <- got }()
	}
	c.notify()
	for range 16 {
		select {
		case got := <-results:
			if !got.Changed || got.Cursor == initial.Cursor {
				t.Fatalf("missed change: %+v", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("waiter did not wake")
		}
	}
	restarted, err := newFavoriteChanges().wait(ctx, initial.Cursor, time.Hour)
	if err != nil || !restarted.Changed {
		t.Fatalf("restart: %+v %v", restarted, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.wait(canceled, c.cursor(), time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestFavoriteChangesOnlyCommittedBuiltIns(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	s := NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), nil)
	ordinary := &models.Playlist{Name: "ordinary", Type: models.PlaylistTypeNormal}
	if err := s.Create(ctx, ordinary); err != nil {
		t.Fatal(err)
	}
	cursor, _ := s.FavoriteChanges(ctx, "")
	for _, tc := range []struct {
		id      int64
		typ     string
		changed bool
	}{{ordinary.ID, models.TypeRemote, false}, {1, models.TypeRemote, true}, {2, models.TypeRadio, true}} {
		song := &models.Song{Title: "notify", Type: tc.typ, URL: "https://example.com/audio"}
		if err := db.SongRepository().Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		if err := s.AddSong(ctx, tc.id, song.ID); err != nil {
			t.Fatal(err)
		}
		next, err := s.favoriteChanges.wait(ctx, cursor.Cursor, time.Millisecond)
		if err != nil || next.Changed != tc.changed {
			t.Fatalf("playlist %d: %+v %v", tc.id, next, err)
		}
		cursor = next
		if err := s.AddSong(ctx, tc.id, song.ID); err == nil {
			t.Fatal("duplicate accepted")
		}
		next, _ = s.favoriteChanges.wait(ctx, cursor.Cursor, time.Millisecond)
		if next.Changed {
			t.Fatal("failed write notified")
		}
	}
}

func TestFavoriteChangesCloseReleasesWaiters(t *testing.T) {
	broker := newFavoriteChanges()
	service := &PlaylistService{favoriteChanges: broker}
	first, _ := broker.wait(context.Background(), "", time.Second)
	done := make(chan error, 1)
	go func() { _, err := broker.wait(context.Background(), first.Cursor, time.Hour); done <- err }()
	service.CloseFavoriteChanges()
	service.CloseFavoriteChanges()
	broker.notify() // A concurrent committed write after shutdown is harmless.
	select {
	case err := <-done:
		if !errors.Is(err, ErrFavoriteChangesClosed) {
			t.Fatalf("unexpected close result: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close left a hanging request")
	}
}

func TestPlaylistReplacementPreservesMembershipTime(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	s := NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), nil)
	s.SetTransactor(db)
	p := &models.Playlist{Name: "timestamps", Type: models.PlaylistTypeNormal}
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for range 2 {
		song := &models.Song{Title: "time", Type: models.TypeRemote, URL: "https://example.com/audio"}
		if err := db.SongRepository().Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, song.ID)
	}
	if err := s.ReplaceSongs(ctx, p.ID, ids); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := db.DB().ExecContext(ctx, "UPDATE playlist_songs SET added_at = ? WHERE playlist_id = ?", when, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSongs(ctx, p.ID, []int64{ids[1], ids[0]}); err != nil {
		t.Fatal(err)
	}
	got, err := db.PlaylistSongRepository().GetSongs(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, song := range got {
		if song.PlaylistAddedAt == nil || !song.PlaylistAddedAt.Equal(when) {
			t.Fatalf("retained member time reset: %+v", song.PlaylistAddedAt)
		}
	}
}
