package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"songloft/internal/models"
	"songloft/internal/services"
	"testing"
	"time"
)

func TestFavoriteChangesHTTPWakesAfterWrite(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	h := NewPlaylistHandler(svc, nil)
	arrived := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") != "" {
			arrived <- struct{}{}
		}
		h.FavoriteChanges(w, r)
	}))
	defer server.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	get := func(path string) (services.FavoriteChangesResponse, error) {
		resp, err := client.Get(server.URL + path)
		if err != nil {
			return services.FavoriteChangesResponse{}, err
		}
		defer resp.Body.Close()
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Error("cacheable change response")
		}
		var data services.FavoriteChangesResponse
		err = json.NewDecoder(resp.Body).Decode(&data)
		return data, err
	}
	first, err := get("/")
	if err != nil || !first.Changed {
		t.Fatalf("initial %+v %v", first, err)
	}
	done := make(chan services.FavoriteChangesResponse, 1)
	errs := make(chan error, 1)
	go func() { data, err := get("/?cursor=" + first.Cursor); done <- data; errs <- err }()
	<-arrived
	song := &models.Song{Title: "HTTP", Type: models.TypeRemote, URL: "https://example.com/http"}
	if err := env.songs.Create(context.Background(), song); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddSong(context.Background(), 1, song.ID); err != nil {
		t.Fatal(err)
	}
	next := <-done
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if !next.Changed || next.Cursor == first.Cursor {
		t.Fatalf("did not invalidate: %+v", next)
	}
}
