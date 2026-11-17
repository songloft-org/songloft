package database_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func TestAutoCreatePreservesLegacyPlaylistOrder(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	repo := db.PlaylistRepository()
	// Older auto-created playlists share position=0. Their timestamps determine
	// the default order; a scan with unchanged metadata must not touch them.
	for _, seed := range []struct{ name, labels, updated string }{
		{"Jazz", `["auto_created"]`, "2020-01-01 00:00:00"},
		{"Manual", `[]`, "2021-01-01 00:00:00"},
		{"Rock", `["auto_created"]`, "2022-01-01 00:00:00"},
	} {
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO playlists
			(type, name, labels, position, updated_at) VALUES ('normal', ?, ?, 0, ?)`, seed.name, seed.labels, seed.updated); err != nil {
			t.Fatal(err)
		}
	}
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "Jazz 1", FilePath: "/music/Jazz/1.mp3", Duration: 120},
		{Type: models.TypeLocal, Title: "Rock 1", FilePath: "/music/Rock/1.mp3", Duration: 120},
	}
	if err := db.SongRepository().BatchCreate(ctx, songs); err != nil {
		t.Fatal(err)
	}
	list := func() []*models.Playlist {
		t.Helper()
		got, err := repo.List(ctx, &database.PlaylistFilter{Type: models.PlaylistTypeNormal})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	before := list()
	assertUnchanged := func() {
		t.Helper()
		after := list()
		if len(after) != len(before) {
			t.Fatalf("playlist count: %d, want %d", len(after), len(before))
		}
		for i, old := range before {
			got := after[i]
			if got.ID != old.ID || !got.UpdatedAt.Equal(old.UpdatedAt) {
				t.Errorf("playlist at %d changed: id=%d updated=%v; want id=%d updated=%v", i, got.ID, got.UpdatedAt, old.ID, old.UpdatedAt)
			}
		}
	}
	for pass := 0; pass < 2; pass++ {
		if _, err := repo.AutoCreate(ctx, models.PlaylistModeDirectory, nil, "", "/music"); err != nil {
			t.Fatal(err)
		}
		assertUnchanged()
		if pass == 0 {
			// Adding a member still refreshes the song list without moving playlists.
			if err := db.SongRepository().Create(ctx, &models.Song{Type: models.TypeLocal, Title: "Jazz 2", FilePath: "/music/Jazz/2.mp3", Duration: 120}); err != nil {
				t.Fatal(err)
			}
		}
	}
	jazz, err := repo.FindByName(ctx, "Jazz")
	if err != nil || jazz.SongCount != 2 {
		t.Fatalf("Jazz membership not refreshed: %+v, %v", jazz, err)
	}
	// Skipping redundant metadata writes must still allow a missing cover to be
	// filled when a later scan extracts one from a member.
	songs[0].CoverURL = "https://example.com/jazz.jpg"
	if err := db.SongRepository().Update(ctx, songs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AutoCreate(ctx, models.PlaylistModeDirectory, nil, "", "/music"); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetByID(ctx, jazz.ID)
	if err != nil || updated.CoverURL != songs[0].CoverURL || !updated.UpdatedAt.After(jazz.UpdatedAt) {
		t.Fatalf("changed metadata not refreshed: %+v, %v", updated, err)
	}
}

func TestAutoCreateAppendsNewPlaylists(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	repo := db.PlaylistRepository()
	manual := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "Manual"}
	if err := repo.Create(ctx, manual); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"Rock", "Jazz"} {
		if err := db.SongRepository().Create(ctx, &models.Song{Type: models.TypeLocal, Title: dir, FilePath: "/music/" + dir + "/1.mp3", Duration: 120}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.AutoCreate(ctx, models.PlaylistModeDirectory, nil, "", "/music"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.List(ctx, &database.PlaylistFilter{Type: models.PlaylistTypeNormal})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range got {
		if !p.IsBuiltIn() {
			names = append(names, p.Name)
		}
	}
	if want := []string{"Manual", "Rock", "Jazz"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("new scan playlists must append: got %v, want %v", names, want)
	}
	rock, err := repo.FindByName(ctx, "Rock")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateSort(ctx, rock.ID, "title", "desc"); err != nil {
		t.Fatal(err)
	}
	pinned := time.Now()
	if err := repo.SetPinned(ctx, rock.ID, &pinned); err != nil {
		t.Fatal(err)
	}
	if err := repo.BatchUpdatePositions(ctx, []int64{got[len(got)-1].ID, manual.ID, rock.ID}); err != nil {
		t.Fatal(err)
	}
	before, err := repo.GetByID(ctx, rock.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AutoCreate(ctx, models.PlaylistModeDirectory, nil, "", "/music"); err != nil {
		t.Fatal(err)
	}
	after, err := repo.GetByID(ctx, rock.ID)
	if err != nil {
		t.Fatal(err)
	}
	var position int
	if err := db.DB().QueryRowContext(ctx, "SELECT position FROM playlists WHERE id = ?", rock.ID).Scan(&position); err != nil {
		t.Fatal(err)
	}
	if position != 3 || after.SortBy != before.SortBy || after.SortOrder != before.SortOrder || !reflect.DeepEqual(after.PinnedAt, before.PinnedAt) {
		t.Fatalf("scan changed user preferences: before=%+v after=%+v", before, after)
	}
}
