package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"songloft/internal/database/testutil"
	"songloft/internal/middleware"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestFLVRadioPlaybackAndCompatibilitySession(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	fixture := filepath.Join(t.TempDir(), "live.flv")
	command := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=96x64:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "9",
		"-c:v", "libx264", "-threads", "1", "-preset", "ultrafast", "-g", "20", "-c:a", "aac", fixture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create FLV fixture: %v: %s", err, output)
	}
	flv, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var pulls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/watch" {
			http.Redirect(w, r, "/live", http.StatusFound)
			return
		}
		pulls.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(flv)
	}))
	defer upstream.Close()

	db := testutil.OpenMemoryDB(t)
	auth, err := services.NewAuthService(db.ConfigRepository(), db.TokenRepository(), "admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer auth.Close()
	login, err := auth.Login(context.Background(), "admin", "admin", "flv-player")
	if err != nil {
		t.Fatal(err)
	}
	other, err := auth.Login(context.Background(), "admin", "admin", "other-client")
	if err != nil {
		t.Fatal(err)
	}
	cache := services.NewCacheService(t.TempDir(), services.NewConfigService(db.ConfigRepository()))
	cache.SetFFmpegPath("ffmpeg")
	defer cache.Shutdown()
	transcode, err := services.NewHLSTranscodeService(cache, func() []string { return []string{"127.0.0.1"} })
	if err != nil {
		t.Fatal(err)
	}
	defer transcode.Close()
	songs := services.NewSongService(db.SongRepository(), nil, nil, nil, nil, nil)
	handler := NewSongHandler(songs, cache, nil, nil, nil, nil)
	handler.SetHLSTranscodeService(transcode)
	hls := NewHLSTranscodeHandler(transcode, "/music")
	const sessions = "/api/v1/proxy/transcode/hls/sessions"
	router := chi.NewRouter()
	router.Use(middleware.AuthMiddleware(auth))
	router.Post("/api/v1/songs/radio", handler.AddRadios)
	router.Put("/api/v1/songs/{id}", handler.UpdateSong)
	router.Get("/api/v1/songs/{id}/play", handler.GetSongPlay)
	router.Get("/api/v1/songs/{id}/play.flv", handler.GetSongPlay)
	router.Head("/api/v1/songs/{id}/play.flv", handler.GetSongPlay)
	router.Post("/api/v1/songs/{id}/hls/sessions", hls.CreateRadio(songs))
	router.Get(sessions+"/{id}/playlist.m3u8", hls.Playlist)
	router.Get(sessions+"/{id}/segments/{name}", hls.Segment)
	router.Delete(sessions+"/{id}", hls.Delete)
	request := func(method, path, body, token string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, strings.TrimPrefix(path, "/music"), strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != status {
			t.Fatalf("%s %s got %d, want %d: %.300s", method, req.URL.Path, recorder.Code, status, recorder.Body)
		}
		return recorder
	}
	token := login.AccessToken
	source := upstream.URL + "/watch"
	created := request(http.MethodPost, "/api/v1/songs/radio", fmt.Sprintf(`[{"title":"FLV","url":%q,"is_video":true,"radio_mode":"flv"}]`, source), token, 201)
	var result struct{ Songs []models.Song }
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil || len(result.Songs) != 1 {
		t.Fatalf("created radio: %v: %s", err, created.Body)
	}
	song := result.Songs[0]
	songPath := fmt.Sprintf("/api/v1/songs/%d", song.ID)
	if song.URL != songPath+"/play.flv" || !song.IsVideo || song.SourceURL != source {
		t.Fatalf("unexpected station: %+v", song)
	}
	head := request(http.MethodHead, song.URL, "", token, 200)
	if pulls.Load() != 0 || head.Header().Get("Content-Type") != "video/x-flv" {
		t.Fatal("HEAD must identify the stream without pulling it")
	}
	raw := request(http.MethodGet, song.URL, "", token, 200)
	if pulls.Load() != 1 || raw.Header().Get("Content-Type") != "video/x-flv" || !bytes.Equal(raw.Body.Bytes(), flv) {
		t.Fatal("FLV playback must forward the bytes from a single redirected source")
	}
	created = request(http.MethodPost, songPath+"/hls/sessions", "", token, 201)
	var session hlsTranscodeSessionResponse
	if err := json.Unmarshal(created.Body.Bytes(), &session); err != nil || session.PlaylistURL != "/music"+sessions+"/"+session.ID+"/playlist.m3u8" {
		t.Fatalf("invalid session response: %v: %s", err, created.Body)
	}
	request(http.MethodGet, session.PlaylistURL, "", other.AccessToken, 404)
	playlistURL := session.PlaylistURL + "?access_token=" + url.QueryEscape(token)
	playlist := request(http.MethodGet, playlistURL, "", "", 200)
	var segmentPath string
	for line := range strings.SplitSeq(playlist.Body.String(), "\n") {
		if strings.HasPrefix(line, "segments/") {
			segmentPath = "/music" + sessions + "/" + session.ID + "/" + line
			break
		}
	}
	if segmentPath == "" {
		t.Fatalf("empty playlist: %s", playlist.Body)
	}
	segment := request(http.MethodGet, segmentPath, "", "", 200)
	probe := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", "-i", "pipe:0")
	probe.Stdin = bytes.NewReader(segment.Body.Bytes())
	if output, err := probe.CombinedOutput(); err != nil || !strings.Contains(string(output), "h264") || !strings.Contains(string(output), "aac") {
		t.Fatalf("HLS segment audio/video: %v: %s", err, output)
	}
	request(http.MethodDelete, sessions+"/"+session.ID, "", token, 204)
	request(http.MethodGet, session.PlaylistURL, "", token, 404)

	request(http.MethodPut, songPath, `{"title":"FLV","radio_mode":"audio_only"}`, token, 200)
	request(http.MethodPost, songPath+"/hls/sessions", "", token, 422)
	audio := request(http.MethodGet, songPath+"/play", "", token, 200)
	probe = exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", "-i", "pipe:0")
	probe.Stdin = bytes.NewReader(audio.Body.Bytes())
	if output, err := probe.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "mp3" {
		t.Fatalf("expected a single MP3 track: %v: %s", err, output)
	}
}

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
