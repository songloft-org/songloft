package database

import (
	"context"
	"testing"

	"songloft/internal/models"
)

func TestUpdateRadioModeBatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := db.SongRepository()
	ctx := context.Background()

	a := &models.Song{Type: models.TypeRadio, Title: "A", URL: "https://example.com/a", IsVideo: true}
	b := &models.Song{Type: models.TypeRadio, Title: "B", URL: "https://example.com/b", IsVideo: false}
	local := &models.Song{Type: models.TypeLocal, Title: "Local", FilePath: "/music/local.mp3"}
	for _, song := range []*models.Song{a, b, local} {
		if err := repo.Create(ctx, song); err != nil {
			t.Fatalf("create %s: %v", song.Title, err)
		}
	}

	video := true
	updated, err := repo.UpdateRadioModeBatch(ctx, []int64{a.ID, b.ID, local.ID, 99999}, models.RadioModeHLS, &video)
	if err != nil {
		t.Fatalf("UpdateRadioModeBatch: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	for _, id := range []int64{a.ID, b.ID} {
		got, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.RadioMode != models.RadioModeHLS || !got.IsVideo {
			t.Fatalf("song %d after hls = %+v", id, got)
		}
	}
	gotLocal, err := repo.GetByID(ctx, local.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotLocal.RadioMode != models.RadioModeAuto || gotLocal.IsVideo {
		t.Fatalf("local song changed = %+v", gotLocal)
	}

	updated, err = repo.UpdateRadioModeBatch(ctx, []int64{a.ID, b.ID}, models.RadioModeAudioOnly, nil)
	if err != nil {
		t.Fatalf("audio-only UpdateRadioModeBatch: %v", err)
	}
	if updated != 2 {
		t.Fatalf("audio-only updated = %d, want 2", updated)
	}
	for _, id := range []int64{a.ID, b.ID} {
		got, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.RadioMode != models.RadioModeAudioOnly || got.IsVideo {
			t.Fatalf("song %d after audio-only = %+v", id, got)
		}
	}
}
