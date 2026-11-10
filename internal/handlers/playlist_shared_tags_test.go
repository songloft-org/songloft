package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestPlaylistTagsAPIAndFilter(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	ctx := context.Background()
	svc := services.NewSongTagService(db.SongTagRepository(), db)
	tag, err := svc.Create(ctx, "Navidrome", "")
	if err != nil {
		t.Fatal(err)
	}
	h := NewSongTagHandler(svc, nil, nil)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{}`, 400}, {`{"tag_ids":null}`, 400}, {`{"tag_ids":[-1]}`, 400},
		{`{"tag_ids":[99999]}`, 404}, {fmt.Sprintf(`{"tag_ids":[%d,%d]}`, tag.ID, tag.ID), 200},
	} {
		w := httptest.NewRecorder()
		h.SetPlaylistTags(w, newRouteRequest("PUT", "/", []byte(tc.body), map[string]string{"id": "1"}))
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body.String())
		}
	}
	ph := NewPlaylistHandler(services.NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), nil), nil)
	w := httptest.NewRecorder()
	ph.ListPlaylists(w, newRouteRequest("GET", fmt.Sprintf("/playlists?tag_id=%d", tag.ID), nil, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"total":1`) || strings.Contains(w.Body.String(), "电台收藏") {
		t.Fatalf("filtered list: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	ph.ListPlaylists(w, newRouteRequest("GET", "/playlists?tag_id=bad", nil, nil))
	if w.Code != 400 {
		t.Fatalf("invalid filter: %d", w.Code)
	}
}

func TestPlaylistCoverSkipsMissingAndFailedMembers(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	p := createTestPlaylist(t, svc, &models.Playlist{Type: models.PlaylistTypeNormal, Name: "fallback", CoverPath: "/missing/cover.png"})
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "missing", 404) }))
	defer upstream.Close()
	// Artwork candidates, rather than the first 20 members, bound fallback work.
	for i := 0; i < 21; i++ {
		s := &models.Song{Type: models.TypeRemote, Title: fmt.Sprintf("song %d", i), URL: fmt.Sprintf("https://example.com/%d", i)}
		if i == 20 {
			s.CoverURL = upstream.URL + "/missing.png"
		}
		if err := env.songs.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
		if err := env.playlistSongs.AddSong(ctx, p.ID, s.ID, i+1); err != nil {
			t.Fatal(err)
		}
	}
	cover := filepath.Join(t.TempDir(), "cover.png")
	content := []byte("test local artwork")
	if err := os.WriteFile(cover, content, 0600); err != nil {
		t.Fatal(err)
	}
	s := &models.Song{Type: models.TypeRemote, Title: "valid artwork", URL: "https://example.com/final", CoverPath: cover}
	if err := env.songs.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := env.playlistSongs.AddSong(ctx, p.ID, s.ID, 22); err != nil {
		t.Fatal(err)
	}
	p, err := svc.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.CoverURLPath() == "" {
		t.Fatal("nonempty playlist needs a fallback cover endpoint")
	}
	if (&models.Playlist{ID: p.ID, SongCount: 1}).CoverURLPath() == "" {
		t.Fatal("member-only artwork must expose the playlist cover endpoint")
	}
	h := NewPlaylistHandler(svc, nil)
	w := httptest.NewRecorder()
	h.GetPlaylistCover(w, newRouteRequest("GET", "/", nil, map[string]string{"id": strconv.FormatInt(p.ID, 10)}))
	if w.Code != 200 || w.Body.String() != string(content) {
		t.Fatalf("fallback: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("dynamic cover cache: %v", w.Header())
	}
}

func TestPlaylistCoverResolvesAuthenticatedPluginRelativeURL(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jsplugin/navidrome/cover" || r.URL.Query().Get("access_token") != "fixture" || r.URL.Query().Get("id") != "remote" {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("plugin artwork"))
	}))
	defer upstream.Close()
	port, err := strconv.Atoi(strings.TrimPrefix(upstream.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	svc := env.newService()
	p := createTestPlaylist(t, svc, &models.Playlist{Type: models.PlaylistTypeNormal, Name: "plugin cover"})
	s := &models.Song{Type: models.TypeRemote, Title: "plugin", URL: "https://example.com/music", CoverURL: "/api/v1/jsplugin/navidrome/cover?id=remote"}
	if err := env.songs.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := env.playlistSongs.AddSong(context.Background(), p.ID, s.ID, 1); err != nil {
		t.Fatal(err)
	}
	h := NewPlaylistHandler(svc, nil)
	h.SetURLResolver(services.NewInternalURLResolver(port, "fixture"))
	w := httptest.NewRecorder()
	h.GetPlaylistCover(w, newRouteRequest("GET", "/", nil, map[string]string{"id": strconv.FormatInt(p.ID, 10)}))
	if w.Code != 200 || w.Body.String() != "plugin artwork" {
		t.Fatalf("plugin fallback: %d %s", w.Code, w.Body.String())
	}
}

func TestPlaylistCoverRemoteTimeoutStillServesLocalMember(t *testing.T) {
	for _, ownCover := range []bool{true, false} {
		t.Run(fmt.Sprintf("own_cover_%t", ownCover), func(t *testing.T) {
			t.Parallel()
			env := newPlaylistHandlerEnv(t)
			svc := env.newService()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-r.Context().Done()
			}))
			defer upstream.Close()
			p := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "slow artwork"}
			if ownCover {
				p.CoverURL = upstream.URL
			}
			createTestPlaylist(t, svc, p)
			ctx := context.Background()
			if !ownCover {
				song := &models.Song{Type: models.TypeRemote, Title: "slow", URL: "https://example.com/slow", CoverURL: upstream.URL}
				if err := env.songs.Create(ctx, song); err != nil {
					t.Fatal(err)
				}
				if err := env.playlistSongs.AddSong(ctx, p.ID, song.ID, 1); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(t.TempDir(), "local.png")
			if err := os.WriteFile(path, []byte("LOCAL_AFTER_TIMEOUT"), 0600); err != nil {
				t.Fatal(err)
			}
			song := &models.Song{Type: models.TypeLocal, Title: "local", FilePath: "/music/local.mp3", CoverPath: path}
			if err := env.songs.Create(ctx, song); err != nil {
				t.Fatal(err)
			}
			if err := env.playlistSongs.AddSong(ctx, p.ID, song.ID, 2); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			NewPlaylistHandler(svc, nil).GetPlaylistCover(w, newRouteRequest("GET", "/", nil, map[string]string{"id": strconv.FormatInt(p.ID, 10)}))
			if w.Code != http.StatusOK || w.Body.String() != "LOCAL_AFTER_TIMEOUT" {
				t.Fatalf("local fallback after remote timeout: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestPlaylistCoverMemberChangeDoesNotReuseLastModified(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	p := createTestPlaylist(t, svc, &models.Playlist{Type: models.PlaylistTypeNormal, Name: "changing artwork"})
	ctx := context.Background()
	for i, content := range []string{"FIRST_ARTWORK", "SECOND_ARTWORK"} {
		path := filepath.Join(t.TempDir(), "cover.png")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		// A newly selected cover can have an older file modification time.
		modified := time.Date(2026, 1, 2-i, 12, 0, 0, 0, time.UTC)
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
		song := &models.Song{Type: models.TypeLocal, Title: content, FilePath: fmt.Sprintf("/music/%d.mp3", i), CoverPath: path}
		if err := env.songs.Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		if err := env.playlistSongs.AddSong(ctx, p.ID, song.ID, i+1); err != nil {
			t.Fatal(err)
		}
	}
	h := NewPlaylistHandler(svc, nil)
	id := map[string]string{"id": strconv.FormatInt(p.ID, 10)}
	first := httptest.NewRecorder()
	h.GetPlaylistCover(first, newRouteRequest("GET", "/", nil, id))
	if first.Code != http.StatusOK || first.Body.String() != "FIRST_ARTWORK" {
		t.Fatalf("first cover: %d %s", first.Code, first.Body.String())
	}
	songs, err := env.playlistSongs.GetSongs(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.playlistSongs.RemoveSong(ctx, p.ID, songs[0].ID); err != nil {
		t.Fatal(err)
	}
	req := newRouteRequest("GET", "/", nil, id)
	req.Header.Set("If-Modified-Since", first.Header().Get("Last-Modified"))
	second := httptest.NewRecorder()
	h.GetPlaylistCover(second, req)
	if second.Code != http.StatusOK || second.Body.String() != "SECOND_ARTWORK" {
		t.Fatalf("changed member must return current bytes: %d %s", second.Code, second.Body.String())
	}
}
