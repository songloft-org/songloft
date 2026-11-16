package services

import (
	"context"
	"os"
	"testing"

	"github.com/hanxi/tag"
	"songloft/internal/database"
	"songloft/internal/database/testutil"
)

func TestAlbumMetadataScanAndRescan(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	repo := db.SongRepository()
	extractor := NewMetadataExtractor(&MetadataConfig{})
	svc := NewSongService(repo, db, extractor, nil, nil, nil)
	path := copyTestFile(t, "../../pkg/tag/testdata/without_tags/sample.flac")
	writeAndExtract := func(track, disc string) *Metadata {
		t.Helper()
		if err := tag.WriteTag(path, tag.WriteOptions{Title: "Title", Album: "Album", Track: track, Disc: disc}); err != nil {
			t.Fatal(err)
		}
		meta, err := extractor.Extract(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Track != track || meta.Disc != disc {
			t.Fatalf("extracted track/disc: %q %q", meta.Track, meta.Disc)
		}
		return meta
	}
	meta := writeAndExtract("10/12", "2/2")
	svc.flushScanBatch(ctx, []scanExtractResult{{item: scanProcessItem{filePath: path}, metadata: meta}})
	songs, err := repo.List(ctx, &database.SongFilter{Album: "Album"})
	if err != nil || len(songs) != 1 {
		t.Fatalf("import: %+v %v", songs, err)
	}
	id := songs[0].ID
	if songs[0].Track != "10/12" || songs[0].Disc != "2/2" {
		t.Fatalf("imported: %+v", songs[0])
	}
	meta = writeAndExtract("1/12", "1/2")
	svc.flushScanBatch(ctx, []scanExtractResult{{item: scanProcessItem{filePath: path, existingSongID: id}, metadata: meta}})
	song, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if song.Track != "1/12" || song.Disc != "1/2" {
		t.Fatalf("rescanned: %+v", song)
	}
	// Other edits must retain the fields when rewriting the complete tag.
	if status := WriteSongTags(path, song); status != FileWriteWritten && status != FileWriteSkipped {
		t.Fatalf("write: %s", status)
	}
	got, err := extractor.Extract(ctx, path)
	if err != nil || got.Track != "1/12" || got.Disc != "1/2" {
		t.Fatalf("rewritten: %+v %v", got, err)
	}
	// Removing file tags externally must also clear stale database ordering.
	plain, err := os.ReadFile("../../pkg/tag/testdata/without_tags/sample.flac")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, plain, 0600); err != nil {
		t.Fatal(err)
	}
	got, err = extractor.Extract(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	svc.flushScanBatch(ctx, []scanExtractResult{{item: scanProcessItem{filePath: path, existingSongID: id}, metadata: got}})
	song, err = repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if song.Track != "" || song.Disc != "" {
		t.Fatalf("removed tags: track=%q disc=%q", song.Track, song.Disc)
	}
}
