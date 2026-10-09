package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func TestScanAndImportDSFWithDefaultConfig(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	configService := NewConfigService(db.ConfigRepository())
	var scanConfig struct {
		SupportedFormats []string `json:"supported_formats"`
	}
	if err := configService.GetJSON("scan_config", &scanConfig); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	sample, err := os.ReadFile(filepath.Join("..", "..", "pkg", "tag", "testdata", "with_tags", "sample.dsf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sample.dsf", "sample.DSF"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, sample, 0644); err != nil {
			t.Fatal(err)
		}
		// 避开扫描器的文件写入稳定性窗口。
		mtime := time.Now().Add(-time.Minute)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	scanner := NewScanner(&ScanConfig{MusicPath: root, SupportedFormats: scanConfig.SupportedFormats})
	extractor := NewMetadataExtractor(&MetadataConfig{FFProbePath: "ffprobe", CoverStoragePath: t.TempDir()})
	service := NewSongService(db.SongRepository(), db, extractor, scanner, nil, nil)
	service.doScanAndImport(context.Background(), false, nil)
	songs, err := db.SongRepository().List(context.Background(), &database.SongFilter{Type: models.TypeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 2 {
		t.Fatalf("imported %d songs, want both .dsf and .DSF; progress: %+v", len(songs), service.GetScanProgress())
	}
	for _, song := range songs {
		if song.Title != "Test Title" || song.Artist != "Test Artist" || song.Album != "Test Album" || song.Duration <= 0 || song.Format != "dsf" {
			t.Errorf("DSF metadata not imported: %+v", song)
		}
	}
}
