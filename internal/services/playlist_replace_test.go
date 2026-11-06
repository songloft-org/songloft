package services

import (
	"context"
	"errors"
	"reflect"
	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"testing"
)

// Fail after the transaction callback has mutated real repositories, to prove rollback.
type failingPlaylistTx struct{ database.DB }

func (f failingPlaylistTx) RunInTx(ctx context.Context, fn func(context.Context, *database.UnitOfWork) error) error {
	return f.DB.RunInTx(ctx, func(ctx context.Context, u *database.UnitOfWork) error {
		if err := fn(ctx, u); err != nil {
			return err
		}
		return errors.New("injected commit failure")
	})
}
func TestPlaylistReplaceSongsAtomic(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	s := NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), nil)
	s.SetTransactor(db)
	p := &models.Playlist{Name: "sync", Type: models.PlaylistTypeNormal}
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, name := range []string{"A", "B", "C"} {
		song := &models.Song{Title: name, Type: models.TypeRemote, URL: "https://example.com/" + name}
		if err := db.SongRepository().Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, song.ID)
	}
	assertIDs := func(want []int64) {
		t.Helper()
		got, err := s.SongIDsOrdered(ctx, p.ID, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) || len(want) > 0 && !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if err := s.ReplaceSongs(ctx, p.ID, ids[:2]); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]int64{{ids[0], 999999}, {ids[0], ids[0]}, {-1}} {
		if err := s.ReplaceSongs(ctx, p.ID, bad); err == nil {
			t.Fatal("accepted invalid IDs")
		}
		assertIDs(ids[:2])
	}
	s.SetTransactor(failingPlaylistTx{db})
	if err := s.ReplaceSongs(ctx, p.ID, ids[2:]); err == nil {
		t.Fatal("expected failure")
	}
	assertIDs(ids[:2])
	s.SetTransactor(db)
	if err := s.ReplaceSongs(ctx, p.ID, []int64{ids[2], ids[0]}); err != nil {
		t.Fatal(err)
	}
	assertIDs([]int64{ids[2], ids[0]})
	if err := s.ReplaceSongs(ctx, 1, ids); err == nil {
		t.Fatal("replaced built-in playlist")
	}
	if err := s.ReplaceSongs(ctx, p.ID, []int64{}); err != nil {
		t.Fatal(err)
	}
	assertIDs(nil)
}
