package handlers

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestScanAndPlayDTS(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not available")
	}
	root := t.TempDir()
	src := filepath.Join(root, "Surround.dts")
	cmd := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi",
		"-i", "sine=frequency=800:sample_rate=48000:duration=4",
		"-af", "pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0",
		"-c:a", "dca", "-strict", "experimental", src)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate 5.1 DTS: %v: %s", err, output)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	uppercase := filepath.Join(root, "Uppercase.DTS")
	if err := os.WriteFile(uppercase, raw, 0600); err != nil {
		t.Fatal(err)
	}
	mtime := time.Now().Add(-time.Minute)
	for _, path := range []string{src, uppercase} {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	db := testutil.OpenMemoryDB(t)
	config := services.NewConfigService(db.ConfigRepository())
	var scanConfig struct {
		SupportedFormats []string `json:"supported_formats"`
	}
	if err := config.GetJSON("scan_config", &scanConfig); err != nil {
		t.Fatal(err)
	}
	scanner := services.NewScanner(&services.ScanConfig{MusicPath: root, SupportedFormats: scanConfig.SupportedFormats})
	extractor := services.NewMetadataExtractor(&services.MetadataConfig{FFProbePath: ffprobe})
	repo := db.SongRepository()
	service := services.NewSongService(repo, db, extractor, scanner, config, nil)
	if err := service.ScanAndImportAsync(false, nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for service.GetScanProgress().EndTime == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if progress := service.GetScanProgress(); progress.Status != services.ScanStatusCompleted || progress.ImportedFiles != 2 {
		t.Fatalf("expected both DTS files to be imported: %+v", progress)
	}
	songs, err := repo.List(context.Background(), &database.SongFilter{Type: models.TypeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 2 {
		t.Fatalf("found %d DTS songs, want 2", len(songs))
	}
	cache := services.NewCacheService(t.TempDir(), config)
	cache.SetFFmpegPath(ffmpeg)
	handler := NewSongHandler(service, cache, nil, nil, nil, nil)
	for _, song := range songs {
		title := strings.TrimSuffix(filepath.Base(song.FilePath), filepath.Ext(song.FilePath))
		if song.Title != title || song.Format != "dts" || song.IsVideo || song.Duration < 3.9 || song.Duration > 4.1 || song.SampleRate != 48000 {
			t.Fatalf("unexpected DTS metadata: %+v", song)
		}
		resp := playSeekRequest(t, handler, song.ID, "")
		if got := resp.Header().Get("Content-Type"); resp.Code != http.StatusOK || got != "audio/mpeg" {
			t.Fatalf("play %s: got status %d and type %q, want 200 audio/mpeg", title, resp.Code, got)
		}
		probe := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name,channels", "-of", "csv=p=0", "pipe:0")
		probe.Stdin = resp.Body
		if got, err := probe.CombinedOutput(); err != nil || strings.TrimSpace(string(got)) != "mp3,2" {
			t.Fatalf("play %s: expected stereo MP3: %v: %s", title, err, got)
		}
	}
}
