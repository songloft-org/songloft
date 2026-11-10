package database_test

import (
	"context"
	"encoding/json"
	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"testing"
	"time"
)

func TestPlaylistMemberTimeIsSeparateFromSongTime(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	song := &models.Song{Title: "time", Type: models.TypeRemote, URL: "https://example.com/time"}
	if err := db.SongRepository().Create(ctx, song); err != nil {
		t.Fatal(err)
	}
	library := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.DB().ExecContext(ctx, "UPDATE songs SET added_at = ? WHERE id = ?", library, song.ID); err != nil {
		t.Fatal(err)
	}
	member := time.Date(2021, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := db.PlaylistSongRepository().AddSong(ctx, 1, song.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(ctx, "UPDATE playlist_songs SET added_at = ? WHERE playlist_id = 1", member); err != nil {
		t.Fatal(err)
	}
	repo := db.PlaylistSongRepository()
	for _, get := range []func() ([]*models.Song, error){func() ([]*models.Song, error) { return repo.GetSongs(ctx, 1) }, func() ([]*models.Song, error) { return repo.GetSongsPaginated(ctx, 1, 10, 0) }, func() ([]*models.Song, error) {
		return repo.GetSongsFiltered(ctx, 1, database.PlaylistSongFilter{Keyword: "time"})
	}} {
		got, err := get()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !got[0].AddedAt.Equal(library) || got[0].PlaylistAddedAt == nil || !got[0].PlaylistAddedAt.Equal(member) {
			t.Fatalf("wrong times: %+v", got)
		}
		for _, value := range []any{got[0], got[0].Brief()} {
			data, _ := json.Marshal(value)
			var fields map[string]any
			_ = json.Unmarshal(data, &fields)
			if fields["playlist_added_at"] == nil {
				t.Fatalf("missing timestamp: %s", data)
			}
		}
	}
	detail, err := db.SongRepository().GetByID(ctx, song.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PlaylistAddedAt != nil || !detail.AddedAt.Equal(library) {
		t.Fatalf("membership leaked to song: %+v", detail)
	}
}
