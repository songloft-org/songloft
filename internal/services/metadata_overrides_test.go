package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hanxi/tag"

	"songloft/internal/database"
	"songloft/internal/database/sqlc"
	"songloft/internal/models"
)

func writeOverrideFixture(t *testing.T, name, title string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	wav := make([]byte, 44+8000)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 8000)
	binary.LittleEndian.PutUint32(wav[28:], 8000)
	binary.LittleEndian.PutUint16(wav[32:], 1)
	binary.LittleEndian.PutUint16(wav[34:], 8)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], 8000)
	if err := os.WriteFile(name, wav, 0600); err != nil {
		t.Fatal(err)
	}
	if err := tag.WriteTag(name, tag.WriteOptions{Title: title, Artist: "File artist", Album: "File album"}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(name, old, old); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataOverridesSurviveRestartAndScopedRescan(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	music := filepath.Join(base, "music")
	paths := []string{filepath.Join(music, "one", "song.wav"), filepath.Join(music, "two", "song.wav")}
	for _, path := range paths {
		writeOverrideFixture(t, path, "On disk")
	}
	dbPath := filepath.Join(base, "library.db")
	open := func() (*database.SQLiteDB, *SongService) {
		t.Helper()
		db, err := database.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		extractor := NewMetadataExtractor(&MetadataConfig{CoverStoragePath: filepath.Join(base, "covers")})
		scanner := NewScanner(&ScanConfig{MusicPath: music, SupportedFormats: []string{"wav"}})
		return db, NewSongService(db.SongRepository(), db, extractor, scanner, nil, nil)
	}
	db, service := open()
	t.Cleanup(func() { _ = db.Close() })
	service.scanAndImport(ctx, false, nil, 0)
	songs, err := db.SongRepository().List(ctx, &database.SongFilter{Type: models.TypeLocal})
	if err != nil || len(songs) != 2 {
		t.Fatalf("initial scan got %d songs: %v", len(songs), err)
	}

	for _, song := range songs {
		song.Title = "Manual"
		song.Artist = ""
		song.CoverPath = ""
		_, status, err := service.EditLocalMetadata(ctx, song, []string{"title", "artist", "picture"}, false)
		if err != nil || status != FileWriteWritten {
			t.Fatalf("edit status %q: %v", status, err)
		}
		writeOverrideFixture(t, song.FilePath, "External edit")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, service = open()

	const editedMask = models.MetadataOverrideTitle | models.MetadataOverrideArtist | models.MetadataOverridePicture
	assertSong := func(songID int64, title, artist string, mask models.SongMetadataMask) {
		t.Helper()
		repo := db.SongRepository()
		song, err := repo.GetByID(ctx, songID)
		if err != nil {
			t.Fatal(err)
		}
		if song.MetadataOverrideMask != mask || (mask == 0) != (song.MetadataEditedAt == 0) ||
			song.Title != title || song.Artist != artist || song.CoverPath != "" {
			t.Fatalf("title=%q artist=%q cover=%q mask=%d editedAt=%d", song.Title, song.Artist, song.CoverPath, song.MetadataOverrideMask, song.MetadataEditedAt)
		}
	}
	service.scanAndImport(ctx, false, nil, 0)
	service.scanAndImport(ctx, true, nil, 0)
	for _, song := range songs {
		if err := db.SongRepository().UpdateMetadata(ctx, sqlc.UpdateSongMetadataParams{
			ID: song.ID, Column11: "Automatic artist", Artist: "Automatic artist",
			Column15: "automatic-cover", CoverPath: "automatic-cover",
		}); err != nil {
			t.Fatal(err)
		}
		if err := db.SongRepository().UpdateTagFields(ctx, sqlc.UpdateSongTagFieldsParams{
			ID: song.ID, Column1: "Automatic title", Title: "Automatic title",
			Column3: "Automatic artist", Artist: "Automatic artist",
		}); err != nil {
			t.Fatal(err)
		}

		assertSong(song.ID, "Manual", "", editedMask)
	}

	service.scanAndImport(ctx, true, []string{filepath.Dir(paths[0])}, time.Now().UnixNano())
	var outside *models.Song
	for _, song := range songs {
		if song.FilePath == paths[0] {
			assertSong(song.ID, "External edit", "File artist", 0)
		} else {
			assertSong(song.ID, "Manual", "", editedMask)
			outside = song
		}
	}
	// Failed native reads preserve overrides even when there is a filename fallback.
	invalid := []byte("RIFF corrupted data")
	if err := os.WriteFile(outside.FilePath, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(outside.FilePath, old, old); err != nil {
		t.Fatal(err)
	}
	service.scanAndImport(ctx, true, []string{filepath.Dir(paths[1])}, time.Now().UnixNano())
	assertSong(outside.ID, "Manual", "", editedMask)

	// A failed file write saves the edit and leaves the original file byte-for-byte.
	outside.Title = "DB only"
	_, status, err := service.EditLocalMetadata(ctx, outside, []string{"title"}, false)
	if err != nil || status == FileWriteWritten {
		t.Fatalf("invalid WAVE write status %q: %v", status, err)
	}
	got, err := os.ReadFile(outside.FilePath)
	if err != nil || !bytes.Equal(got, invalid) {
		t.Fatalf("source changed on failed rewrite: %v", err)
	}
	assertSong(outside.ID, "DB only", "", editedMask)

	// A newer edit keeps every override on this song, including older fields.
	// Cancellation protects the same fields, even on a later scan.
	writeOverrideFixture(t, outside.FilePath, "On disk again")
	beforeEdit := time.Now().UnixNano()
	outside.Title = "Newer"
	_, _, err = service.EditLocalMetadata(ctx, outside, []string{"title"}, false)
	if err != nil {
		t.Fatal(err)
	}
	batch := []scanExtractResult{{
		item:     scanProcessItem{filePath: outside.FilePath, existingSongID: outside.ID},
		metadata: &Metadata{Title: "Older scan", FileMetadataRead: true},
	}}
	cancel := make(chan struct{})
	close(cancel)
	service.flushScanBatchWithReset(ctx, batch, time.Now().UnixNano(), cancel)
	assertSong(outside.ID, "Newer", "", editedMask)
	service.flushScanBatchWithReset(ctx, batch, beforeEdit, nil)
	assertSong(outside.ID, "Newer", "", editedMask)
	service.flushScanBatchWithReset(ctx, batch, time.Now().UnixNano(), nil)
	assertSong(outside.ID, "Older scan", "", 0)
}
