package services

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/pkg/cue"
)

func TestCueEncodingImportAndReimport(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	root := t.TempDir()
	audioPath := filepath.Join(root, "CDImage10.wav")
	// A sparse, valid 64-minute PCM WAV covers every INDEX in the supplied CUE.
	wav, err := os.Create(audioPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { wav.Close() })
	const dataSize = 64 * 60 * 8000
	header := make([]byte, 44)
	copy(header, "RIFF")
	binary.LittleEndian.PutUint32(header[4:], dataSize+36)
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], 8000)
	binary.LittleEndian.PutUint32(header[28:], 8000)
	binary.LittleEndian.PutUint16(header[32:], 1)
	binary.LittleEndian.PutUint16(header[34:], 8)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], dataSize)
	if _, err := wav.Write(header); err != nil {
		t.Fatal(err)
	}
	if err := wav.Truncate(dataSize + 44); err != nil {
		t.Fatal(err)
	}
	if err := wav.Close(); err != nil {
		t.Fatal(err)
	}

	fixtureDir := filepath.Join("..", "..", "pkg", "cue", "testdata")
	data, err := os.ReadFile(filepath.Join(fixtureDir, "issue554.cp950.cue"))
	if err != nil {
		t.Fatal(err)
	}
	cuePath := filepath.Join(root, "album.cue")
	writeCue := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(cuePath, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeCue(data)
	extractor := NewMetadataExtractor(&MetadataConfig{FFProbePath: "ffprobe", CoverStoragePath: t.TempDir()})
	service := NewSongService(db.SongRepository(), db, extractor, nil, nil, nil)
	readSongs := func() []*models.Song {
		t.Helper()
		songs, err := db.SongRepository().List(ctx, &database.SongFilter{Type: models.TypeLocal})
		if err != nil {
			t.Fatal(err)
		}
		return songs
	}
	want, err := cue.ParseFile(filepath.Join(fixtureDir, "issue554.utf8.cue"))
	if err != nil {
		t.Fatal(err)
	}
	assertMetadata := func() {
		t.Helper()
		songs := readSongs()
		if len(songs) != 16 {
			t.Fatalf("got %d songs, want 16", len(songs))
		}
		for _, song := range songs {
			index := song.CueTrackIndex - 1
			if index < 0 || index >= len(want.Files[0].Tracks) {
				t.Fatalf("unexpected track index: %d", song.CueTrackIndex)
			}
			track := want.Files[0].Tracks[index]
			if song.Title != track.Title || song.Artist != track.Performer || song.Album != want.Title || song.FilePath != audioPath || song.CueSourcePath != cuePath || song.CueStartSeconds != track.Start.ToSeconds() || song.Duration <= 0 {
				t.Errorf("incorrect persisted metadata: %+v", song)
			}
		}
	}
	// First import persists correctly decoded metadata through the real UoW/SQLite.
	service.processCueFiles(ctx, []string{cuePath}, false)
	assertMetadata()
	// Incremental scanning still skips existing sources; explicit reimport repairs them.
	old := readSongs()[0]
	old.Title, old.Artist, old.Album = "old title", "old artist", "old album"
	if err := db.SongRepository().Update(ctx, old); err != nil {
		t.Fatal(err)
	}
	service.processCueFiles(ctx, []string{cuePath}, false)
	got, err := db.SongRepository().GetByID(ctx, old.ID)
	if err != nil || got.Title != old.Title {
		t.Fatalf("incremental scan did not preserve existing source: %+v, %v", got, err)
	}
	service.processCueFiles(ctx, []string{cuePath}, true)
	assertMetadata()
	// An undecidable replacement must not delete or overwrite existing records.
	before := readSongs()
	writeCue([]byte("TITLE \"\xa4\xa4\xa4\xe5\"\nFILE \"CDImage10.wav\" WAVE\n TRACK 01 AUDIO\n INDEX 01 00:00:00\n"))
	service.processCueFiles(ctx, []string{cuePath}, true)
	if after := readSongs(); !reflect.DeepEqual(after, before) {
		t.Fatal("failed encoding detection changed previously imported songs")
	}
}
