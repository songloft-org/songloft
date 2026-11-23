package handlers

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"songloft/internal/models"
	"songloft/internal/services"
)

func TestSongCoverRejectsLocalCoverTargets(t *testing.T) {
	var coverCalls atomic.Int32
	// This origin returns fixture data. It never calls a Songloft handler recursively.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect.png" {
			http.Redirect(w, r, "/music/api/v1/playlists/7/cover", http.StatusFound)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/jsplugin/fixture/") && r.URL.Query().Get("access_token") == "fixture" {
			w.Header().Set("Content-Type", "image/png")
			fmt.Fprint(w, "plugin image")
			return
		}
		coverCalls.Add(1)
		fmt.Fprint(w, "unexpected fetch")
	}))
	defer upstream.Close()
	repo := newTestSongRepo(t)
	service := services.NewSongService(repo, nil, nil, nil, nil, nil)
	h := NewSongHandler(service, nil, nil, nil, nil, nil)
	resolver := services.NewInternalURLResolver(upstream.Listener.Addr().(*net.TCPAddr).Port, "fixture", "/music")
	h.SetURLResolver(resolver)

	for _, tc := range []struct {
		name, source string
		status       int
	}{
		{"relative", "/api/v1/songs/9/cover?v=1", http.StatusNotFound},
		{"local absolute with base path", upstream.URL + "/music/api/v1/songs/9/cover?v=1", http.StatusNotFound},
		{"incoming host", upstream.URL + "/api/v1/playlists/7/cover", http.StatusNotFound},
		{"redirect", upstream.URL + "/redirect.png", http.StatusNotFound},
		{"plugin image", "/api/v1/jsplugin/fixture/image?id=9", http.StatusOK},
		{"plugin nested path", "/api/v1/jsplugin/fixture/proxy/api/v1/songs/9/cover", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := seedSong(t, repo, &models.Song{Title: tc.name, URL: "https://example.com/audio", Type: models.TypeRadio, CoverURL: tc.source})
			requestURL := "https://songloft.test/api/v1/songs/cover"
			if tc.name == "incoming host" {
				requestURL = upstream.URL + "/api/v1/songs/cover"
				h.SetURLResolver(nil)
				t.Cleanup(func() { h.SetURLResolver(resolver) })
			}
			w := httptest.NewRecorder()
			h.GetSongCover(w, newRouteRequest(http.MethodGet, requestURL, nil, map[string]string{"id": strconv.FormatInt(id, 10)}))
			if w.Code != tc.status {
				t.Errorf("status=%d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == http.StatusOK && w.Body.String() != "plugin image" {
				t.Errorf("plugin image body = %q", w.Body.String())
			}
		})
	}
	if got := coverCalls.Load(); got != 0 {
		t.Errorf("cover endpoints received %d requests, want 0", got)
	}
}

func TestRemoteCoverCachesRepeatedFailureByURL(t *testing.T) {
	var requests atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Query().Get("variant") == "good" {
			w.Header().Set("Content-Type", "image/png")
			fmt.Fprint(w, "cover image")
			return
		}
		http.NotFound(w, r)
	}))
	defer origin.Close()
	coverURL := origin.URL + "/image.png?variant=missing"
	for range 2 {
		w := httptest.NewRecorder()
		proxyRemoteCover(w, httptest.NewRequest(http.MethodGet, "/", nil), nil, coverURL, false)
		if w.Code != http.StatusNotFound {
			t.Fatalf("missing status = %d", w.Code)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("upstream requests = %d, want 1", requests.Load())
	}
	w := httptest.NewRecorder()
	if proxyRemoteCover(w, httptest.NewRequest(http.MethodGet, "/", nil), nil, coverURL, true) || w.Body.Len() != 0 {
		t.Fatalf("cached failure should leave the fallback response unwritten: %s", w.Body.String())
	}
	proxyRemoteCover(w, httptest.NewRequest(http.MethodGet, "/", nil), nil, origin.URL+"/image.png?variant=good", false)
	if w.Code != http.StatusOK || w.Body.String() != "cover image" || requests.Load() != 2 {
		t.Fatalf("other URL variant: %d %q, requests=%d", w.Code, w.Body.String(), requests.Load())
	}
}

func TestRemoteCoverFailureCacheExpiresAndStaysBounded(t *testing.T) {
	cache := coverFailureCache{entries: make(map[[sha256.Size]byte]coverFailure)}
	start := time.Now()
	old := sha256.Sum256([]byte("0"))
	for i := range remoteCoverFailureLimit + 1 {
		cache.remember(sha256.Sum256([]byte(strconv.Itoa(i))), http.StatusNotFound, start.Add(time.Duration(i)*time.Nanosecond))
	}
	if len(cache.entries) != remoteCoverFailureLimit || cache.status(old, start) != 0 {
		t.Fatal("the oldest entry should be evicted at capacity")
	}
	key := sha256.Sum256([]byte(strconv.Itoa(remoteCoverFailureLimit)))
	if cache.status(key, start.Add(time.Second)) != http.StatusNotFound || cache.status(key, start.Add(remoteCoverFailureTTL+time.Second)) != 0 {
		t.Fatal("the saved status should disappear after its TTL")
	}
}

func TestUpdateSongRejectsLocalCoverReference(t *testing.T) {
	repo := newTestSongRepo(t)
	h := NewSongHandler(services.NewSongService(repo, nil, nil, nil, nil, nil), nil, nil, nil, nil, nil)
	const previousCover = "https://example.com/cover.jpg"
	id := seedSong(t, repo, &models.Song{Title: "radio", Type: models.TypeRadio, URL: "https://example.com/live", CoverURL: previousCover})
	body := fmt.Sprintf(`{"title":"radio","cover_url":"/api/v1/songs/%d/cover"}`, id)
	w := httptest.NewRecorder()
	h.UpdateSong(w, newRouteRequest(http.MethodPut, "/", []byte(body), map[string]string{"id": strconv.FormatInt(id, 10)}))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "封面") {
		t.Errorf("update = %d %s", w.Code, w.Body.String())
	}
	song, err := repo.GetByID(t.Context(), id)
	if err != nil || song.CoverURL != previousCover {
		t.Fatalf("cover update must leave previous data intact: %v, %v", song, err)
	}
}

func TestPlaylistCoverSkipsLocalCoverReference(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, "unexpected remote cover")
	}))
	defer upstream.Close()
	env := newPlaylistHandlerEnv(t)
	service := env.newService()
	playlist := createTestPlaylist(t, service, &models.Playlist{
		Type: models.PlaylistTypeNormal, Name: "fallback", CoverURL: "/api/v1/songs/9/cover",
	})
	coverPath := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(coverPath, []byte("local member image"), 0600); err != nil {
		t.Fatal(err)
	}
	id := seedSong(t, env.songs, &models.Song{Type: models.TypeRemote, Title: "member", URL: "https://example.com/music", CoverPath: coverPath})
	if err := env.playlistSongs.AddSong(t.Context(), playlist.ID, id, 1); err != nil {
		t.Fatal(err)
	}
	h := NewPlaylistHandler(service, nil)
	h.SetURLResolver(services.NewInternalURLResolver(upstream.Listener.Addr().(*net.TCPAddr).Port, "fixture", ""))
	w := httptest.NewRecorder()
	h.GetPlaylistCover(w, newRouteRequest(http.MethodGet, "/api/v1/playlists/cover", nil, map[string]string{"id": strconv.FormatInt(playlist.ID, 10)}))
	if w.Code != http.StatusOK || w.Body.String() != "local member image" || calls.Load() != 0 {
		t.Fatalf("fallback = %d %q, outbound requests = %d", w.Code, w.Body.String(), calls.Load())
	}
}

func TestRemoteCoverDoesNotWaitForAFreeSlot(t *testing.T) {
	for range cap(remoteCoverSlots) {
		remoteCoverSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for range cap(remoteCoverSlots) {
			<-remoteCoverSlots
		}
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1/cover", nil)
	const remote = "https://example.invalid/image.png"
	for _, allowFallback := range []bool{false, true} {
		w := httptest.NewRecorder()
		if proxyRemoteCover(w, request, nil, remote, allowFallback) {
			t.Fatal("proxy unexpectedly handled a request while busy")
		}
		if allowFallback {
			if w.Body.Len() != 0 || len(w.Header()) != 0 {
				t.Fatalf("fallback response was already written: %s", w.Body.String())
			}
		} else if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("busy status = %d", w.Code)
		}
	}
}
