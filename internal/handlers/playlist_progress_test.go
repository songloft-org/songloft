package handlers

import (
	"context"
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
	"strconv"
	"strings"
	"testing"
)

func TestPlaylistProgressHTTP(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	s := &models.Song{Title: "chapter", Type: models.TypeLocal, FilePath: "/music/chapter.wav"}
	if err := db.SongRepository().Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := db.PlaylistSongRepository().AddSong(ctx, 1, s.ID, 1); err != nil {
		t.Fatal(err)
	}
	h := NewPlaylistProgressHandler(services.NewPlaylistProgressService(db))
	router := chi.NewRouter()
	router.Get("/playlists/{id}/playback-progress", h.Handle)
	router.Put("/playlists/{id}/playback-progress", h.Handle)
	router.Delete("/playlists/{id}/playback-progress", h.Handle)
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/playlists/1/playback-progress", "", 200},
		{"PUT", "/playlists/1/playback-progress", `{"version":0,"song_id":` + strconv.FormatInt(s.ID, 10) + `,"position_ms":754000,"play_mode":"single"}`, 200},
		{"PUT", "/playlists/1/playback-progress", `{"version":0,"play_mode":"order"}`, 409},
		{"PUT", "/playlists/1/playback-progress", `{"version":1,"position_ms":-1}`, 400},
		{"PUT", "/playlists/1/playback-progress", `{"version":1,"play_mode":"unknown"}`, 400},
		{"DELETE", "/playlists/1/playback-progress", "", 204},
		{"PUT", "/playlists/1/playback-progress", `{"version":1,"play_mode":"order"}`, 409},
		{"GET", "/playlists/2/playback-progress", "", 400},
		{"GET", "/playlists/0/playback-progress", "", 400},
		{"GET", "/playlists/9999/playback-progress", "", 404},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Errorf("%s %s: got %d expected %d: %s", tc.method, tc.path, w.Code, tc.code, w.Body.String())
		}
	}
}
