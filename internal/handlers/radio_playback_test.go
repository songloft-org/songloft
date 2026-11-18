package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestRadioPlaybackModes(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	dir := t.TempDir()
	args := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=96x64:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "2",
		"-c:v", "libx265", "-threads", "1", "-preset", "ultrafast", "-x265-params", "pools=none:frame-threads=1",
		"-c:a", "aac", "-f", "hls", "-hls_list_size", "0", filepath.Join(dir, "media.m3u8")}
	if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("create HEVC/AAC fixture: %v: %s", err, output)
	}
	var pulls atomic.Int32
	files := http.FileServer(http.Dir(dir))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pulls.Add(1)
		if r.URL.Path == "/578" {
			http.Redirect(w, r, "/media.m3u8", http.StatusFound)
			return
		}
		files.ServeHTTP(w, r)
	}))
	defer upstream.Close()
	db := testutil.OpenMemoryDB(t)
	config := services.NewConfigService(db.ConfigRepository())
	cache := services.NewCacheService(t.TempDir(), config)
	cache.SetFFmpegPath("ffmpeg")
	defer cache.Shutdown()
	transcode, err := services.NewHLSTranscodeService(cache, func() []string { return []string{"127.0.0.1"} })
	if err != nil {
		t.Fatal(err)
	}
	defer transcode.Close()
	h := NewSongHandler(services.NewSongService(db.SongRepository(), nil, nil, nil, nil, nil), cache, nil, nil, nil, nil)
	h.SetHLSTranscodeService(transcode)
	router := chi.NewRouter()
	router.Post("/api/v1/songs/radio", h.AddRadios)
	router.Get("/api/v1/songs/{id}", h.GetSong)
	router.Put("/api/v1/songs/{id}", h.UpdateSong)
	router.Get("/api/v1/songs/{id}/play.m3u8", h.GetSongPlay)
	router.Get("/api/v1/songs/{id}/play", h.GetSongPlay)
	router.Head("/api/v1/songs/{id}/play", h.GetSongPlay)

	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rr.Code != status {
			t.Fatalf("%s %s: status = %d, want %d, response = %.300q", method, path, rr.Code, status, rr.Body.String())
		}
		return rr
	}
	source := upstream.URL + "/578"
	created := request("POST", "/api/v1/songs/radio", fmt.Sprintf(`[{"title":"IPTV","url":%q,"is_video":true,"radio_mode":"hls"}]`, source), http.StatusCreated)
	var result struct{ Songs []models.Song }
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil || len(result.Songs) != 1 {
		t.Fatalf("created radio: %v, response = %s", err, created.Body.String())
	}
	songPath := fmt.Sprintf("/api/v1/songs/%d", result.Songs[0].ID)
	if result.Songs[0].URL != songPath+"/play.m3u8" || !result.Songs[0].IsVideo {
		t.Fatalf("HLS playback response: %s", created.Body.String())
	}
	if got := request("GET", songPath+"/play.m3u8", "", http.StatusFound).Header().Get("Location"); got != source {
		t.Fatalf("redirect = %q, want %q", got, source)
	}
	request("PUT", songPath, `{"title":"IPTV","radio_mode":"audio_only","is_video":true}`, http.StatusOK)
	detail := request("GET", songPath, "", http.StatusOK)
	var song models.Song
	if err := json.Unmarshal(detail.Body.Bytes(), &song); err != nil || song.IsVideo || song.RadioMode != models.RadioModeAudioOnly || song.URL != songPath+"/play" || song.SourceURL != source {
		t.Fatalf("persisted audio-only radio: %v, response = %s", err, detail.Body.String())
	}
	head := request("HEAD", song.URL, "", http.StatusOK)
	if pulls.Load() != 0 || head.Body.Len() != 0 || head.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatal("HEAD should report MP3 without opening the upstream")
	}
	stream := request("GET", song.URL, "", http.StatusOK)
	if stream.Header().Get("Content-Type") != "audio/mpeg" || stream.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("audio response headers: %v", stream.Header())
	}
	file := filepath.Join(t.TempDir(), "radio.mp3")
	if err := os.WriteFile(file, stream.Body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name,codec_type", "-of", "csv=p=0", file).CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "mp3,audio" {
		t.Fatalf("expected one MP3 audio stream: %v: %s", err, output)
	}
	request("PUT", songPath, `{"title":"IPTV","radio_mode":"invalid"}`, http.StatusBadRequest)
	request("PUT", songPath, `{"title":"IPTV","url":"file:///tmp/source.ts"}`, http.StatusBadRequest)
	cache.SetFFmpegPath("")
	request("GET", song.URL, "", http.StatusServiceUnavailable)
	reset := request("PUT", songPath, `{"title":"IPTV","radio_mode":""}`, http.StatusOK)
	song = models.Song{}
	if err := json.Unmarshal(reset.Body.Bytes(), &song); err != nil || song.RadioMode != models.RadioModeAuto || song.URL != songPath+"/play" {
		t.Fatalf("restore automatic mode: %v, response = %s", err, reset.Body.String())
	}
}
