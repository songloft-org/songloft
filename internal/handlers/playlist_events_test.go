package handlers

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"songloft/internal/models"
)

func TestPlaylistMembershipEventsFromHTTP(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	h := NewPlaylistHandler(svc, nil)
	router := chi.NewRouter()
	router.Post("/playlists/{id}/songs", h.AddSongToPlaylist)
	router.Delete("/playlists/{id}/songs/{songId}", h.RemoveSongFromPlaylist)
	p := createTestPlaylist(t, svc, &models.Playlist{Name: "http events", Type: models.PlaylistTypeNormal})
	song := &models.Song{Title: "HTTP", Type: models.TypeRemote, URL: "https://example.com/audio"}
	if err := env.songs.Create(context.Background(), song); err != nil {
		t.Fatal(err)
	}
	var events []models.PlaylistSongsChangeEvent
	svc.SetOnSongsChange(func(changes []models.PlaylistSongsChangeEvent) { events = append(events, changes...) })
	for _, id := range []int64{1, p.ID} {
		for _, method := range []string{"POST", "DELETE"} {
			events = nil
			path := fmt.Sprintf("/playlists/%d/songs", id)
			body := fmt.Sprintf(`{"song_ids":[%d,%d]}`, song.ID, song.ID)
			kind := "add"
			if method == "DELETE" {
				path += fmt.Sprintf("/%d", song.ID)
				kind = "remove"
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
			if response.Code != 200 || len(events) != 1 {
				t.Fatalf("%s %s: HTTP %d, events %+v", method, path, response.Code, events)
			}
			if events[0].PlaylistID != id || events[0].SongID != song.ID || events[0].Type != kind {
				t.Fatalf("wrong event: %+v", events[0])
			}
		}
	}
}
