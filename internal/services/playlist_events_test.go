package services

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func TestPlaylistMembershipChanges(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	s := NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), nil)
	s.SetTransactor(db)
	p := &models.Playlist{Name: "events", Type: models.PlaylistTypeNormal}
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	var songs []*models.Song
	for _, name := range []string{"A", "B", "C"} {
		song := &models.Song{Title: name, Artist: "Artist", Type: models.TypeRemote, URL: "https://example.com/" + name}
		if err := db.SongRepository().Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		songs = append(songs, song)
	}
	radio := &models.Song{Title: "Radio", Type: models.TypeRadio, URL: "https://example.com/radio"}
	if err := db.SongRepository().Create(ctx, radio); err != nil {
		t.Fatal(err)
	}
	var events []models.PlaylistSongsChangeEvent
	started := time.Now().UnixMilli()
	s.SetOnSongsChange(func(changes []models.PlaylistSongsChangeEvent) {
		// Reading with the ordinary repository proves the transaction is committed
		// and released (the memory DB has only one connection).
		for _, e := range changes {
			ids, err := s.SongIDsOrdered(ctx, e.PlaylistID, "", "")
			if err != nil {
				t.Fatal(err)
			}
			present := false
			for _, id := range ids {
				if id == e.SongID {
					present = true
				}
			}
			if present != (e.Type == "add") {
				t.Fatalf("event precedes commit: %+v, members %v", e, ids)
			}
			if e.Timestamp < started || e.Timestamp > time.Now().UnixMilli() {
				t.Fatalf("invalid timestamp: %+v", e)
			}
			if e.Song.ID != e.SongID || e.Song.Artist != "Artist" || e.Song.Title == "" {
				t.Fatalf("missing snapshot: %+v", e)
			}
		}
		events = append(events, changes...)
	})
	assertEvents := func(playlistID int64, kinds []string, ids []int64) {
		t.Helper()
		var gotKinds []string
		var gotIDs []int64
		for _, event := range events {
			if event.PlaylistID != playlistID {
				t.Fatalf("playlist ID: %+v", event)
			}
			gotKinds = append(gotKinds, event.Type)
			gotIDs = append(gotIDs, event.SongID)
		}
		if !reflect.DeepEqual(gotKinds, kinds) || !reflect.DeepEqual(gotIDs, ids) {
			t.Fatalf("events = %v/%v, want %v/%v", gotKinds, gotIDs, kinds, ids)
		}
		events = nil
	}
	a, b, c := songs[0].ID, songs[1].ID, songs[2].ID
	// Built-in favorites use the same service path as ordinary playlists.
	if err := s.AddSong(ctx, 1, a); err != nil {
		t.Fatal(err)
	}
	assertEvents(1, []string{"add"}, []int64{a})
	if err := s.AddSong(ctx, 1, a); err == nil {
		t.Fatal("duplicate accepted")
	}
	assertEvents(1, nil, nil)
	if err := s.RemoveSong(ctx, 1, a); err != nil {
		t.Fatal(err)
	}
	assertEvents(1, []string{"remove"}, []int64{a})
	if err := s.RemoveSong(ctx, 1, a); err == nil {
		t.Fatal("missing member removed")
	}
	assertEvents(1, nil, nil)
	added, skipped, err := s.AddSongs(ctx, p.ID, []int64{a, a, b, radio.ID, 999999})
	if err != nil || added != 2 || skipped != 3 {
		t.Fatalf("batch = %d/%d/%v", added, skipped, err)
	}
	assertEvents(p.ID, []string{"add", "add"}, []int64{a, b})
	if _, _, err := s.AddSongs(ctx, p.ID, []int64{a, b}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReorderSongs(ctx, p.ID, []int64{b, a}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSongs(ctx, p.ID, []int64{a, b}); err != nil {
		t.Fatal(err)
	}
	assertEvents(p.ID, nil, nil)
	s.SetTransactor(failingPlaylistTx{db})
	if err := s.ReplaceSongs(ctx, p.ID, []int64{c}); err == nil {
		t.Fatal("expected rollback")
	}
	assertEvents(p.ID, nil, nil)
	s.SetTransactor(db)
	if err := s.ReplaceSongs(ctx, p.ID, []int64{a, c}); err != nil {
		t.Fatal(err)
	}
	assertEvents(p.ID, []string{"remove", "add"}, []int64{b, c})
	if err := s.ReplaceSongs(ctx, p.ID, nil); err != nil {
		t.Fatal(err)
	}
	assertEvents(p.ID, []string{"remove", "remove"}, []int64{a, c})
}

func TestPlaylistMembershipPartialBatchFailure(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	s := NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), nil)
	var ids []int64
	for range 501 {
		song := &models.Song{Title: "batch", Type: models.TypeRemote, URL: "https://example.com/audio"}
		if err := db.SongRepository().Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, song.ID)
	}
	// Use a real database failure in the second chunk, after the first commits.
	if _, err := db.DB().ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER fail_late_playlist_add BEFORE INSERT ON playlist_songs
        WHEN NEW.song_id = %d BEGIN SELECT RAISE(ABORT, 'injected insertion failure'); END`, ids[500])); err != nil {
		t.Fatal(err)
	}
	var got []int64
	s.SetOnSongsChange(func(changes []models.PlaylistSongsChangeEvent) {
		for _, e := range changes {
			got = append(got, e.SongID)
		}
	})
	added, skipped, err := s.AddSongs(ctx, 1, ids)
	if err == nil || added != 500 || skipped != 0 || !reflect.DeepEqual(got, ids[:500]) {
		t.Fatalf("batch: %d/%d/%v; %d events", added, skipped, err, len(got))
	}
}
