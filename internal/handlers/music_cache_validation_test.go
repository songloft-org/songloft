package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
	"songloft/internal/services/source"
)

func cacheTestWAV(seconds int) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+seconds*8000))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(8000))
	binary.Write(&b, binary.LittleEndian, uint32(8000))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(8))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(seconds*8000))
	b.Write(bytes.Repeat([]byte{128}, seconds*8000))
	return b.Bytes()
}

func cacheTestMP3(t *testing.T, duration string) []byte {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg required for actual MP3 HTTP regression")
	}
	path := filepath.Join(t.TempDir(), "audio.mp3")
	cmd := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", duration, "-c:a", "libmp3lame", "-b:a", "128k", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v %s", err, out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCacheValidationProxyMP3(t *testing.T) {
	for _, duration := range []string{"10", "40"} {
		data := cacheTestMP3(t, duration)
		for _, partial := range []bool{false, true} {
			name := duration + map[bool]string{false: "s/200", true: "s/206"}[partial]
			t.Run(name, func(t *testing.T) {
				db := testutil.OpenMemoryDB(t)
				repo := db.SongRepository()
				c := services.NewCacheService(t.TempDir(), services.NewConfigService(db.ConfigRepository()))
				defer c.Shutdown()
				c.SetCachePathCallbacks(repo.UpdateCachePath, repo.ClearCachePath, repo.ClearAllCachePaths, repo.ListSongsWithCache)
				song := &models.Song{Type: models.TypeRemote, URL: "https://example.invalid/audio.mp3", Duration: 40}
				if err := repo.Create(context.Background(), song); err != nil {
					t.Fatal(err)
				}
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "audio/mpeg")
					http.ServeContent(w, r, "audio.mp3", time.Time{}, bytes.NewReader(data))
				}))
				defer upstream.Close()
				done := make(chan error, 1)
				req := httptest.NewRequest(http.MethodGet, "/proxy", nil)
				if partial {
					req.Header.Set("Range", "bytes=0-1023")
				}
				rr := httptest.NewRecorder()
				ServeRemoteResourceWithCache(rr, req, upstream.URL, nil,
					func(path, ct string) {
						done <- c.FinalizeCache(context.Background(), song, path, services.GetExtFromContentType(ct))
					},
					func() { c.AsyncDownloadAndCache(context.Background(), song, upstream.URL, nil); done <- nil },
				)
				wantCode, wantBody := http.StatusOK, data
				if partial {
					wantCode, wantBody = http.StatusPartialContent, data[:1024]
				}
				if rr.Code != wantCode || !bytes.Equal(rr.Body.Bytes(), wantBody) {
					t.Fatalf("stream modified: status=%d bytes=%d", rr.Code, rr.Body.Len())
				}
				var finalizeErr error
				select {
				case finalizeErr = <-done:
				case <-time.After(10 * time.Second):
					t.Fatal("background cache did not finish")
				}
				stored, err := repo.GetByID(context.Background(), song.ID)
				if err != nil {
					t.Fatal(err)
				}
				if duration == "10" {
					if !partial {
						var invalid *source.InvalidAudioError
						if !errors.As(finalizeErr, &invalid) || invalid.Reason != source.ReasonTooShort {
							t.Fatalf("expected typed too_short, got %v", finalizeErr)
						}
					}
					if stored.CachePath != "" {
						t.Fatalf("prompt persisted: %q", stored.CachePath)
					}
					if _, ok := c.FindCachedFileBySong(song); ok {
						t.Fatal("prompt retained in cache")
					}
				} else {
					if finalizeErr != nil {
						t.Fatal(finalizeErr)
					}
					cached, err := os.ReadFile(stored.CachePath)
					if err != nil || !bytes.Equal(cached, data) {
						t.Fatalf("valid mp3 not persisted intact: %v", err)
					}
				}
			})
		}
	}
}

func TestCacheValidationPlayRejectsOldPrompt(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	repo := db.SongRepository()
	config := services.NewConfigService(db.ConfigRepository())
	c := services.NewCacheService(t.TempDir(), config)
	defer c.Shutdown()
	c.SetCachePathCallbacks(repo.UpdateCachePath, repo.ClearCachePath, repo.ClearAllCachePaths, repo.ListSongsWithCache)
	s := services.NewSongService(repo, nil, nil, nil, nil, nil)
	h := NewSongHandler(s, c, nil, nil, nil, nil)
	good := cacheTestWAV(40)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(good)
	}))
	defer upstream.Close()
	old := filepath.Join(t.TempDir(), "old-prompt.wav")
	if err := os.WriteFile(old, cacheTestWAV(10), 0600); err != nil {
		t.Fatal(err)
	}
	song := &models.Song{Type: models.TypeRemote, URL: upstream.URL, Duration: 40, Format: "wav"}
	id := seedSong(t, repo, song)
	if err := repo.UpdateCachePath(context.Background(), id, old); err != nil {
		t.Fatal(err)
	}
	complete := make(chan struct{}, 1)
	c.SetCacheCompleteCallback(func(context.Context, *models.Song, string) { complete <- struct{}{} })
	rr := playSeekRequest(t, h, id, "")
	if rr.Code != http.StatusOK || !bytes.Equal(rr.Body.Bytes(), good) {
		t.Fatalf("play served old prompt: status=%d bytes=%d", rr.Code, rr.Body.Len())
	}
	select {
	case <-complete:
	case <-time.After(5 * time.Second):
		t.Fatal("replacement not cached")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old prompt file retained: %v", err)
	}
	stored, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := os.ReadFile(stored.CachePath)
	if err != nil || !bytes.Equal(cached, good) {
		t.Fatalf("replacement cache invalid: %v", err)
	}
}
