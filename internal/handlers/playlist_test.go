package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"mimusic/internal/database"
	"mimusic/internal/database/testutil"
	"mimusic/internal/models"
	"mimusic/internal/services"

	"github.com/go-chi/chi/v5"
)

// playlistHandlerEnv 把 :memory: SQLite 下 handler 测试需要的仓储打包好。
type playlistHandlerEnv struct {
	playlists     *database.PlaylistRepository
	playlistSongs *database.PlaylistSongRepository
	songs         *database.SongRepository
}

func newPlaylistHandlerEnv(t *testing.T) *playlistHandlerEnv {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	return &playlistHandlerEnv{
		playlists:     mdb.PlaylistRepository(),
		playlistSongs: mdb.PlaylistSongRepository(),
		songs:         mdb.SongRepository(),
	}
}

func (e *playlistHandlerEnv) newService() *services.PlaylistService {
	return services.NewPlaylistService(e.playlists, e.playlistSongs, e.songs, nil)
}

// createTestPlaylist 创建一条歌单并返回。
func createTestPlaylist(t *testing.T, svc *services.PlaylistService, p *models.Playlist) *models.Playlist {
	t.Helper()
	if err := svc.Create(context.Background(), p); err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	return p
}

func newRouteRequest(method, target string, body []byte, params map[string]string) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestNewPlaylistHandler(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	if handler == nil {
		t.Error("NewPlaylistHandler() returned nil")
	}
}

func TestListPlaylists(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	createTestPlaylist(t, svc, &models.Playlist{Type: models.PlaylistTypeNormal, Name: "歌单1"})
	createTestPlaylist(t, svc, &models.Playlist{Type: models.PlaylistTypeNormal, Name: "歌单2"})

	req := httptest.NewRequest("GET", "/api/v1/playlists", nil)
	rr := httptest.NewRecorder()

	handler.ListPlaylists(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestGetPlaylist(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})

	id := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("GET", "/api/v1/playlists/"+id, nil, map[string]string{"id": id})
	rr := httptest.NewRecorder()

	handler.GetPlaylist(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestGetPlaylistNotFound(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	req := newRouteRequest("GET", "/api/v1/playlists/999", nil, map[string]string{"id": "999"})
	rr := httptest.NewRecorder()

	handler.GetPlaylist(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusNotFound)
	}
}

func TestCreatePlaylist(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	playlist := models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "新歌单",
	}
	body, _ := json.Marshal(playlist)

	req := httptest.NewRequest("POST", "/api/v1/playlists", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.CreatePlaylist(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusCreated)
	}
}

func TestCreatePlaylistInvalidJSON(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	req := httptest.NewRequest("POST", "/api/v1/playlists", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.CreatePlaylist(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdatePlaylist(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "原名称",
	})

	updatedPlaylist := models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "新名称",
	}
	body, _ := json.Marshal(updatedPlaylist)

	id := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("PUT", "/api/v1/playlists/"+id, body, map[string]string{"id": id})
	rr := httptest.NewRecorder()

	handler.UpdatePlaylist(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestDeletePlaylist(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})

	id := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("DELETE", "/api/v1/playlists/"+id, nil, map[string]string{"id": id})
	rr := httptest.NewRecorder()

	handler.DeletePlaylist(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestGetPlaylistSongs(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})

	id := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("GET", "/api/v1/playlists/"+id+"/songs", nil, map[string]string{"id": id})
	rr := httptest.NewRecorder()

	handler.GetPlaylistSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestAddSongToPlaylist(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}
	if err := env.songs.Create(context.Background(), song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	reqBody := map[string]interface{}{"song_ids": []int64{song.ID}}
	body, _ := json.Marshal(reqBody)

	id := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("POST", "/api/v1/playlists/"+id+"/songs", body, map[string]string{"id": id})
	rr := httptest.NewRecorder()

	handler.AddSongToPlaylist(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestRemoveSongFromPlaylist(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})
	song := &models.Song{Type: models.TypeLocal, Title: "歌曲", FilePath: "/music/x.mp3"}
	if err := env.songs.Create(context.Background(), song); err != nil {
		t.Fatalf("create song: %v", err)
	}
	if err := svc.AddSong(context.Background(), playlist.ID, song.ID); err != nil {
		t.Fatalf("add song: %v", err)
	}

	pidStr := strconv.FormatInt(playlist.ID, 10)
	sidStr := strconv.FormatInt(song.ID, 10)
	req := newRouteRequest("DELETE", "/api/v1/playlists/"+pidStr+"/songs/"+sidStr, nil, map[string]string{"id": pidStr, "songId": sidStr})
	rr := httptest.NewRecorder()

	handler.RemoveSongFromPlaylist(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestInvalidPlaylistID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	tests := []struct {
		name    string
		handler func(w http.ResponseWriter, r *http.Request)
		method  string
		url     string
	}{
		{"GetPlaylist", handler.GetPlaylist, "GET", "/api/v1/playlists/invalid"},
		{"UpdatePlaylist", handler.UpdatePlaylist, "PUT", "/api/v1/playlists/invalid"},
		{"DeletePlaylist", handler.DeletePlaylist, "DELETE", "/api/v1/playlists/invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRouteRequest(tt.method, tt.url, nil, map[string]string{"id": "invalid"})
			rr := httptest.NewRecorder()

			tt.handler(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("%s returned wrong status code: got %v want %v", tt.name, rr.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestReorderPlaylistSongs(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)
	ctx := context.Background()

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})

	tracks := []*models.Song{
		{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"},
		{Type: models.TypeLocal, Title: "歌曲2", FilePath: "/music/2.mp3"},
		{Type: models.TypeLocal, Title: "歌曲3", FilePath: "/music/3.mp3"},
	}
	for _, song := range tracks {
		if err := env.songs.Create(ctx, song); err != nil {
			t.Fatalf("create song: %v", err)
		}
		if err := svc.AddSong(ctx, playlist.ID, song.ID); err != nil {
			t.Fatalf("AddSong() error = %v", err)
		}
	}

	reqBody := map[string][]int64{"song_ids": {tracks[2].ID, tracks[0].ID, tracks[1].ID}}
	body, _ := json.Marshal(reqBody)

	id := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("PUT", "/api/v1/playlists/"+id+"/songs/reorder", body, map[string]string{"id": id})
	rr := httptest.NewRecorder()

	handler.ReorderPlaylistSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["message"] != "歌单歌曲已重新排序" {
		t.Errorf("unexpected response message: got %v", resp["message"])
	}
}

func TestReorderPlaylistSongsInvalidID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	reqBody := map[string][]int64{"song_ids": {1, 2, 3}}
	body, _ := json.Marshal(reqBody)

	req := newRouteRequest("PUT", "/api/v1/playlists/invalid/songs/reorder", body, map[string]string{"id": "invalid"})
	rr := httptest.NewRecorder()

	handler.ReorderPlaylistSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestReorderPlaylistSongsInvalidJSON(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	req := newRouteRequest("PUT", "/api/v1/playlists/1/songs/reorder", []byte("invalid json"), map[string]string{"id": "1"})
	rr := httptest.NewRecorder()

	handler.ReorderPlaylistSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestListPlaylistsWithFilters(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc)

	createTestPlaylist(t, svc, &models.Playlist{Type: models.PlaylistTypeNormal, Name: "歌单1"})
	createTestPlaylist(t, svc, &models.Playlist{Type: models.PlaylistTypeNormal, Name: "歌单2"})

	req := httptest.NewRequest("GET", "/api/v1/playlists?type=normal&limit=10&offset=0", nil)
	rr := httptest.NewRecorder()

	handler.ListPlaylists(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestGetPlaylistSongsInvalidID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	req := newRouteRequest("GET", "/api/v1/playlists/invalid/songs", nil, map[string]string{"id": "invalid"})
	rr := httptest.NewRecorder()

	handler.GetPlaylistSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestAddSongToPlaylistInvalidID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	reqBody := map[string]int64{"song_id": 1}
	body, _ := json.Marshal(reqBody)

	req := newRouteRequest("POST", "/api/v1/playlists/invalid/songs", body, map[string]string{"id": "invalid"})
	rr := httptest.NewRecorder()

	handler.AddSongToPlaylist(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestAddSongToPlaylistInvalidJSON(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	req := newRouteRequest("POST", "/api/v1/playlists/1/songs", []byte("invalid json"), map[string]string{"id": "1"})
	rr := httptest.NewRecorder()

	handler.AddSongToPlaylist(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestRemoveSongFromPlaylistInvalidIDs(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	tests := []struct {
		name         string
		playlistID   string
		songID       string
		expectedCode int
	}{
		{"invalid playlist ID", "invalid", "1", http.StatusBadRequest},
		{"invalid song ID", "1", "invalid", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRouteRequest("DELETE", "/api/v1/playlists/"+tt.playlistID+"/songs/"+tt.songID, nil, map[string]string{"id": tt.playlistID, "songId": tt.songID})
			rr := httptest.NewRecorder()

			handler.RemoveSongFromPlaylist(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, tt.expectedCode)
			}
		})
	}
}

func TestUpdatePlaylistInvalidJSON(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService())

	req := newRouteRequest("PUT", "/api/v1/playlists/1", []byte("invalid json"), map[string]string{"id": "1"})
	rr := httptest.NewRecorder()

	handler.UpdatePlaylist(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}
