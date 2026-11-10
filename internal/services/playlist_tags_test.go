package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func TestPlaylistTagsPreserveFlagsRollbackAndDeleteReferences(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	svc := NewSongTagService(db.SongTagRepository(), db)
	tag, err := svc.Create(ctx, "Navidrome", "")
	if err != nil {
		t.Fatal(err)
	}
	p := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "来源收藏", Labels: []string{"auto_created"}}
	if err := db.PlaylistRepository().Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPlaylistTags(ctx, p.ID, []int64{tag.ID, tag.ID}); err != nil {
		t.Fatal(err)
	}
	stale, err := db.PlaylistRepository().GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stale.Labels, []string{"auto_created", models.PlaylistTagLabel(tag.ID)}) {
		t.Fatalf("labels: %v", stale.Labels)
	}
	if err := svc.SetPlaylistTags(ctx, p.ID, []int64{tag.ID, 99999}); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("invalid reference: %v", err)
	}
	if tags, err := svc.GetPlaylistTags(ctx, p.ID); err != nil || len(tags) != 1 {
		t.Fatalf("rollback: %v %v", tags, err)
	}
	if _, err := svc.BindPlaylists(ctx, tag.ID, []int64{1, 99999}, false); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("invalid batch: %v", err)
	}
	if tags, _ := svc.GetPlaylistTags(ctx, 1); len(tags) != 0 {
		t.Fatalf("partial batch persisted: %v", tags)
	}
	for i := 0; i < 2; i++ {
		song := &models.Song{Type: models.TypeRemote, Title: "tagged", URL: fmt.Sprintf("https://example.com/%d", i)}
		if err := db.SongRepository().Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		if err := db.SongTagRepository().SetSongTags(ctx, song.ID, []int64{tag.ID}); err != nil {
			t.Fatal(err)
		}
	}
	hidden := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "隐藏来源", Labels: []string{"hidden"}}
	if err := db.PlaylistRepository().Create(ctx, hidden); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPlaylistTags(ctx, hidden.ID, []int64{tag.ID}); err != nil {
		t.Fatal(err)
	}
	counts, err := svc.GetByID(ctx, tag.ID)
	if err != nil || counts.PlaylistCount != 1 || counts.SongCount != 2 {
		t.Fatalf("counts: %+v %v", counts, err)
	}
	list, err := svc.List(ctx, "", "id", "asc", 20, 0)
	if err != nil || len(list) != 1 || list[0].SongCount != 2 || list[0].PlaylistCount != 1 {
		t.Fatalf("distinct visible counts: %+v %v", list, err)
	}
	if err := svc.Delete(ctx, tag.ID); err != nil {
		t.Fatal(err)
	}
	reused, err := svc.Create(ctx, "新的标签", "")
	if err != nil {
		t.Fatal(err)
	}
	if reused.ID != tag.ID {
		t.Fatalf("test requires reused INTEGER PRIMARY KEY: %d != %d", reused.ID, tag.ID)
	}
	stale.Name = "编辑旧数据"
	if err := db.PlaylistRepository().Update(ctx, stale); err != nil {
		t.Fatal(err)
	}
	got, err := db.PlaylistRepository().GetByID(ctx, p.ID)
	if err != nil || !slices.Equal(got.Labels, []string{"auto_created"}) {
		t.Fatalf("resurrected deleted reference: %+v %v", got, err)
	}
}

func TestBackupSharedTagsRemapsIDsAndAcceptsV1(t *testing.T) {
	ctx := context.Background()
	source := testutil.OpenMemoryDB(t)
	svc := NewSongTagService(source.SongTagRepository(), source)
	tag, err := svc.Create(ctx, "Navidrome", "#123456")
	if err != nil {
		t.Fatal(err)
	}
	song := &models.Song{Type: models.TypeRemote, Title: "remote", URL: "https://example.com/music.mp3"}
	if err := source.SongRepository().Create(ctx, song); err != nil {
		t.Fatal(err)
	}
	if err := source.PlaylistSongRepository().AddSong(ctx, 1, song.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPlaylistTags(ctx, 1, []int64{tag.ID}); err != nil {
		t.Fatal(err)
	}
	if err := source.SongTagRepository().SetSongTags(ctx, song.ID, []int64{tag.ID}); err != nil {
		t.Fatal(err)
	}
	backup, err := NewBackupService(source).Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target := testutil.OpenMemoryDB(t)
	if _, err := target.SongTagRepository().Create(ctx, "占位", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBackupService(target).Import(ctx, backup); err != nil {
		t.Fatal(err)
	}
	gotTags, err := target.SongTagRepository().GetByPlaylistID(ctx, 1)
	if err != nil || len(gotTags) != 1 || gotTags[0].Name != "Navidrome" || gotTags[0].ID == tag.ID {
		t.Fatalf("tag remap: %+v %v", gotTags, err)
	}
	ids, err := target.SongTagRepository().ListSongIDs(ctx, gotTags[0].ID, 0, 0)
	if err != nil || len(ids) != 1 {
		t.Fatalf("song tags: %v %v", ids, err)
	}
	backup.Version = 1
	backup.Tags = nil
	if _, err := NewBackupService(target).Import(ctx, backup); err != nil {
		t.Fatalf("v1 import: %v", err)
	}
	got, err := target.PlaylistRepository().GetByID(ctx, 1)
	if err != nil || !slices.Contains(got.Labels, "built_in") {
		t.Fatalf("built-in lost: %+v %v", got, err)
	}
}
