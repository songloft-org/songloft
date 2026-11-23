package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestAutoRadioModeUsesProbedContainer(t *testing.T) {
	var hits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/station.php":
			http.Redirect(w, r, "/playlist", http.StatusFound)
		case "/playlist":
			w.Header().Set("Content-Type", "application/octet-stream")
			io.WriteString(w, "#EXTM3U\n#EXT-X-VERSION:3\n")
		case "/stream":
			io.WriteString(w, "FLV\x01"+strings.Repeat("x", 128))
		default:
			http.NotFound(w, r)
		}
	}))
	defer origin.Close()
	db := testutil.OpenMemoryDB(t)
	repo := db.SongRepository()
	service := services.NewSongService(repo, nil, nil, nil, nil, nil)
	service.SetRadioSourceProbe(services.NewRadioSourceProbe(func() []string { return []string{"127.0.0.1"} }))
	h := NewSongHandler(service, nil, nil, nil, nil, nil)
	router := chi.NewRouter()
	router.Post("/api/v1/songs/radio", h.AddRadios)
	router.Put("/api/v1/songs/{id}", h.UpdateSong)
	router.Post("/api/v1/songs/batch-radio-mode", h.BatchUpdateRadioMode)
	router.Get("/api/v1/songs/{id}/play.m3u8", h.GetSongPlay)
	router.Get("/api/v1/songs/{id}/play.flv", h.GetSongPlay)
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	added := request(http.MethodPost, "/api/v1/songs/radio", fmt.Sprintf(`[{"url":%q,"title":"HLS"},{"url":%q,"title":"FLV"}]`, origin.URL+"/station.php", origin.URL+"/stream"), http.StatusCreated)
	var result struct{ Songs []models.Song }
	if err := json.Unmarshal(added.Body.Bytes(), &result); err != nil || len(result.Songs) != 2 {
		t.Fatalf("add result: %s, %v", added.Body.String(), err)
	}
	for i, suffix := range []string{".m3u8", ".flv"} {
		song := result.Songs[i]
		if !strings.HasSuffix(song.URL, suffix) || song.RadioMode != models.RadioModeAuto {
			t.Fatalf("automatic song %d: %+v", i, song)
		}
	}
	before := hits.Load()
	hls := result.Songs[0]
	flv := result.Songs[1]
	listed, err := repo.ListByIDs(t.Context(), []int64{hls.ID, flv.ID})
	if err != nil || len(listed) != 2 || !listed[0].IsHLSStream() || !listed[1].IsFLVStream() || hits.Load() != before {
		t.Fatalf("list should use persisted detections: %v, %v", listed, err)
	}
	playlist := db.PlaylistSongRepository()
	if err := playlist.AddSong(t.Context(), 2, hls.ID, 1); err != nil {
		t.Fatal(err)
	}
	members, err := playlist.GetSongsFiltered(t.Context(), 2, database.PlaylistSongFilter{Limit: 10})
	if err != nil || len(members) != 1 || !members[0].IsHLSStream() || hits.Load() != before {
		t.Fatalf("playlist should use the persisted detection: %v, %v", members, err)
	}
	if got := request(http.MethodGet, hls.URL, "", http.StatusFound).Header().Get("Location"); got != origin.URL+"/station.php" {
		t.Errorf("HLS redirect location = %q", got)
	}
	if got := request(http.MethodGet, flv.URL, "", http.StatusOK).Header().Get("Content-Type"); got != "video/x-flv" {
		t.Errorf("FLV content type = %q", got)
	}
	songPath := fmt.Sprintf("/api/v1/songs/%d", hls.ID)
	before = hits.Load()
	request(http.MethodPut, songPath, `{"title":"Explicit","radio_mode":"flv"}`, http.StatusOK)
	if hits.Load() != before {
		t.Fatal("manual mode should skip the probe")
	}
	request(http.MethodPost, "/api/v1/songs/batch-radio-mode", fmt.Sprintf(`{"ids":[%d],"radio_mode":""}`, hls.ID), http.StatusOK)
	updated, err := repo.GetByID(t.Context(), hls.ID)
	if err != nil || !updated.IsHLSStream() || updated.RadioMode != models.RadioModeAuto {
		t.Fatalf("reset to automatic: %v, %v", updated, err)
	}
	req := request(http.MethodPut, songPath, fmt.Sprintf(`{"title":"Changed","url":%q}`, origin.URL+"/stream"), http.StatusOK)
	if !strings.Contains(req.Body.String(), `/play.flv"`) {
		t.Fatalf("source change response: %s", req.Body.String())
	}
	if saved, err := repo.UpdateRadioDetectedMode(t.Context(), hls.ID, origin.URL+"/station.php", models.RadioModeHLS); err != nil || saved {
		t.Fatalf("accepted stale detection for old source: %t, %v", saved, err)
	}
}

func TestAutoRadioProbeDeadline(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(stopped)
	}))
	defer origin.Close()
	repo := newTestSongRepo(t)
	service := services.NewSongService(repo, nil, nil, nil, nil, nil)
	service.SetRadioSourceProbe(services.NewRadioSourceProbe(func() []string { return []string{"127.0.0.1"} }))
	song := &models.Song{Title: "timeout", Type: models.TypeRadio, URL: origin.URL}
	seedSong(t, repo, song)
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	service.RefreshRadioModes(ctx, []*models.Song{song})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("radio probe did not reach upstream")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("radio probe did not cancel upstream")
	}
	if song.RadioDetectedMode != models.RadioModeAuto {
		t.Fatalf("incomplete response detected as %q", song.RadioDetectedMode)
	}
}

func TestAutoRadioProbeRevalidatesRedirect(t *testing.T) {
	var blocked atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blocked" {
			blocked.Add(1)
			io.WriteString(w, "FLV fixture")
			return
		}
		_, port, _ := net.SplitHostPort(r.Host)
		http.Redirect(w, r, "http://localhost:"+port+"/blocked", http.StatusFound)
	}))
	defer origin.Close()
	repo := newTestSongRepo(t)
	service := services.NewSongService(repo, nil, nil, nil, nil, nil)
	service.SetRadioSourceProbe(services.NewRadioSourceProbe(func() []string { return []string{"127.0.0.1"} }))
	song := &models.Song{Title: "redirected", Type: models.TypeRadio, URL: origin.URL}
	seedSong(t, repo, song)
	service.RefreshRadioModes(t.Context(), []*models.Song{song})
	if blocked.Load() != 0 || song.RadioDetectedMode != models.RadioModeAuto {
		t.Fatalf("blocked redirect was contacted or detected: requests=%d, mode=%s", blocked.Load(), song.RadioDetectedMode)
	}
}
