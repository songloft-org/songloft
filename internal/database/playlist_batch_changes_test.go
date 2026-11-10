package database_test

import (
	"context"
	"slices"
	"testing"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func TestPlaylistBatchChangesOnlyCommittedRows(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	var ids []int64
	// The first 500 rows commit; row 502 fails a real SQLite foreign key.
	for range 501 {
		song := &models.Song{Title: "batch", Type: models.TypeRemote, URL: "https://example.com/audio"}
		if err := db.SongRepository().Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, song.ID)
	}
	input := append(slices.Clone(ids), 999999)
	got, skipped, err := db.PlaylistSongRepository().AddSongsBatchWithChanges(ctx, 1, 0, input)
	if err == nil || skipped != 0 || !slices.Equal(got, ids[:500]) {
		t.Fatalf("changes = %v, skipped %d, err %v", got, skipped, err)
	}
	persisted, err := db.PlaylistSongRepository().ListSongIDsOrdered(ctx, 1, "", "")
	if err != nil || !slices.Equal(persisted, got) {
		t.Fatalf("persisted %v, err %v", persisted, err)
	}
	if err := db.PlaylistSongRepository().Clear(ctx, 1); err != nil {
		t.Fatal(err)
	}
	added, skipped, err := db.PlaylistSongRepository().AddSongsBatch(ctx, 1, 0, input)
	if err == nil || added != 500 || skipped != 0 {
		t.Fatalf("count includes rolled-back rows: %d/%d/%v", added, skipped, err)
	}
}
