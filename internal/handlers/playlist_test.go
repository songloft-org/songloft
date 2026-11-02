package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"

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
	handler := NewPlaylistHandler(env.newService(), nil)

	if handler == nil {
		t.Error("NewPlaylistHandler() returned nil")
	}
}

func TestListPlaylists(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(env.newService(), nil)

	req := newRouteRequest("GET", "/api/v1/playlists/999", nil, map[string]string{"id": "999"})
	rr := httptest.NewRecorder()

	handler.GetPlaylist(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusNotFound)
	}
}

func TestCreatePlaylist(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService(), nil)

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
	handler := NewPlaylistHandler(env.newService(), nil)

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
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(env.newService(), nil)

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
	handler := NewPlaylistHandler(svc, nil)
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

// 内置歌单保护针对歌单身份，不能阻止收藏成员的顺序调整。
func TestBuiltInPlaylistSongOrdering(t *testing.T) {
	for _, playlistID := range []int64{1, 2} {
		for _, action := range []string{"reorder", "move", "name_asc", "name_desc", "number_prefix", "shuffle"} {
			t.Run(fmt.Sprintf("%d/%s", playlistID, action), func(t *testing.T) {
				env := newPlaylistHandlerEnv(t)
				svc := env.newService()
				handler := NewPlaylistHandler(svc, nil)
				ctx := context.Background()
				before, err := env.playlists.GetByID(ctx, playlistID)
				if err != nil {
					t.Fatal(err)
				}
				ids := make([]int64, 0, 3)
				for i, title := range []string{"03 C", "01 A", "02 B"} {
					typ := models.TypeLocal
					if playlistID == 2 {
						typ = models.TypeRadio
					}
					song := &models.Song{Type: typ, Title: title, FilePath: fmt.Sprintf("/music/%d.mp3", i), URL: fmt.Sprintf("https://example.com/%d", i)}
					if err := env.songs.Create(ctx, song); err != nil {
						t.Fatal(err)
					}
					if err := svc.AddSong(ctx, playlistID, song.ID); err != nil {
						t.Fatal(err)
					}
					ids = append(ids, song.ID)
				}
				want := []int64{ids[1], ids[2], ids[0]}
				method, endpoint := "POST", "sort"
				var body any = map[string]string{"action": action}
				handle := handler.SortPlaylistSongs
				switch action {
				case "reorder":
					method, endpoint, handle = "PUT", "reorder", handler.ReorderPlaylistSongs
					body = map[string][]int64{"song_ids": want}
				case "move":
					method, endpoint, handle = "PUT", "move", handler.MovePlaylistSong
					body = map[string]any{"song_id": ids[0], "after_song_id": ids[2]}
				case "name_desc":
					slices.Reverse(want)
				}
				payload, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				id := strconv.FormatInt(playlistID, 10)
				path := "/api/v1/playlists/" + id + "/songs/" + endpoint
				rr := httptest.NewRecorder()
				handle(rr, newRouteRequest(method, path, payload, map[string]string{"id": id}))
				if rr.Code != http.StatusOK {
					t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
				}
				// 从新的 service 读持久位置，不依赖请求内返回值或客户端视图。
				gotSongs, err := env.newService().GetSongs(ctx, playlistID, database.PlaylistSongFilter{OrderBy: "position", Order: "asc"})
				if err != nil {
					t.Fatal(err)
				}
				got := make([]int64, 0, len(gotSongs))
				for _, song := range gotSongs {
					got = append(got, song.ID)
				}
				if action == "shuffle" {
					slices.Sort(got)
					want = slices.Clone(ids)
					slices.Sort(want)
				}
				if !slices.Equal(got, want) {
					t.Fatalf("persisted order %v, want %v", got, want)
				}
				after, err := env.playlists.GetByID(ctx, playlistID)
				if err != nil {
					t.Fatal(err)
				}
				if before.ID != after.ID || before.Name != after.Name || before.Type != after.Type || !slices.Equal(before.Labels, after.Labels) {
					t.Fatalf("built-in identity changed: %+v", after)
				}
				if err := svc.Delete(ctx, playlistID); err == nil {
					t.Fatal("built-in deletion must remain forbidden")
				}
			})
		}
	}
}

func TestReorderPlaylistSongsInvalidID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService(), nil)

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
	handler := NewPlaylistHandler(env.newService(), nil)

	req := newRouteRequest("PUT", "/api/v1/playlists/1/songs/reorder", []byte("invalid json"), map[string]string{"id": "1"})
	rr := httptest.NewRecorder()

	handler.ReorderPlaylistSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestSortPlaylistSongs(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc, nil)
	ctx := context.Background()

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})

	tracks := []*models.Song{
		{Type: models.TypeLocal, Title: "歌曲C", FilePath: "/music/c.mp3"},
		{Type: models.TypeLocal, Title: "歌曲A", FilePath: "/music/a.mp3"},
		{Type: models.TypeLocal, Title: "歌曲B", FilePath: "/music/b.mp3"},
	}
	for _, song := range tracks {
		if err := env.songs.Create(ctx, song); err != nil {
			t.Fatalf("create song: %v", err)
		}
		if err := svc.AddSong(ctx, playlist.ID, song.ID); err != nil {
			t.Fatalf("AddSong() error = %v", err)
		}
	}

	tests := []struct {
		name   string
		action string
	}{
		{"name_asc", "name_asc"},
		{"name_desc", "name_desc"},
		{"shuffle", "shuffle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]string{"action": tt.action}
			body, _ := json.Marshal(reqBody)

			id := strconv.FormatInt(playlist.ID, 10)
			req := newRouteRequest("POST", "/api/v1/playlists/"+id+"/songs/sort", body, map[string]string{"id": id})
			rr := httptest.NewRecorder()

			handler.SortPlaylistSongs(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
			}
		})
	}
}

func TestSortPlaylistSongsInvalidID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService(), nil)

	reqBody := map[string]string{"action": "name_asc"}
	body, _ := json.Marshal(reqBody)

	req := newRouteRequest("POST", "/api/v1/playlists/invalid/songs/sort", body, map[string]string{"id": "invalid"})
	rr := httptest.NewRecorder()

	handler.SortPlaylistSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestSortPlaylistSongsInvalidAction(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc, nil)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	})

	reqBody := map[string]string{"action": "invalid_action"}
	body, _ := json.Marshal(reqBody)

	id := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("POST", "/api/v1/playlists/"+id+"/songs/sort", body, map[string]string{"id": id})
	rr := httptest.NewRecorder()

	handler.SortPlaylistSongs(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusInternalServerError)
	}
}

func TestSortPlaylistSongsInvalidJSON(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService(), nil)

	req := newRouteRequest("POST", "/api/v1/playlists/1/songs/sort", []byte("invalid json"), map[string]string{"id": "1"})
	rr := httptest.NewRecorder()

	handler.SortPlaylistSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestListPlaylistsWithFilters(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc, nil)

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
	handler := NewPlaylistHandler(env.newService(), nil)

	req := newRouteRequest("GET", "/api/v1/playlists/invalid/songs", nil, map[string]string{"id": "invalid"})
	rr := httptest.NewRecorder()

	handler.GetPlaylistSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestAddSongToPlaylistInvalidID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService(), nil)

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
	handler := NewPlaylistHandler(env.newService(), nil)

	req := newRouteRequest("POST", "/api/v1/playlists/1/songs", []byte("invalid json"), map[string]string{"id": "1"})
	rr := httptest.NewRecorder()

	handler.AddSongToPlaylist(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestRemoveSongFromPlaylistInvalidIDs(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	handler := NewPlaylistHandler(env.newService(), nil)

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
	handler := NewPlaylistHandler(env.newService(), nil)

	req := newRouteRequest("PUT", "/api/v1/playlists/1", []byte("invalid json"), map[string]string{"id": "1"})
	rr := httptest.NewRecorder()

	handler.UpdatePlaylist(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdatePlaylistWithCoverSongID(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	ctx := context.Background()

	// Create a song with CoverPath (simulating a local song with extracted cover)
	song := &models.Song{
		Type:      models.TypeLocal,
		Title:     "Song With Cover",
		FilePath:  "/music/test.mp3",
		CoverPath: "/data/covers/ab/cd/testhash.jpg",
	}
	if err := env.songs.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}
	t.Logf("Song created: ID=%d, CoverPath=%q, CoverURL=%q", song.ID, song.CoverPath, song.CoverURL)

	// Create a song service for the handler
	songSvc := services.NewSongService(env.songs, nil, nil, nil, nil, nil)
	handler := NewPlaylistHandler(svc, songSvc)

	// Create a playlist
	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "Test Playlist",
	})
	t.Logf("Playlist created: ID=%d, CoverPath=%q, CoverURL=%q", playlist.ID, playlist.CoverPath, playlist.CoverURL)

	// Update playlist with cover_song_id
	reqBody := map[string]interface{}{
		"name":          "Test Playlist",
		"cover_song_id": song.ID,
	}
	body, _ := json.Marshal(reqBody)

	idStr := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("PUT", "/api/v1/playlists/"+idStr, body, map[string]string{"id": idStr})
	rr := httptest.NewRecorder()

	handler.UpdatePlaylist(rr, req)

	t.Logf("Update response: Status=%d", rr.Code)
	t.Logf("Response body: %s", rr.Body.String())

	if rr.Code != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	// Parse the response
	var respPlaylist map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&respPlaylist); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Check cover_url in response
	coverURL, ok := respPlaylist["cover_url"]
	t.Logf("Response cover_url: %v (exists: %v)", coverURL, ok)

	if !ok || coverURL == "" || coverURL == nil {
		t.Errorf("expected cover_url to be set, got: %v", coverURL)
	}

	// Verify DB state directly
	updated, err := svc.GetByID(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	t.Logf("DB state: CoverPath=%q, CoverURL=%q", updated.CoverPath, updated.CoverURL)

	if updated.CoverPath != song.CoverPath {
		t.Errorf("expected CoverPath=%q, got %q", song.CoverPath, updated.CoverPath)
	}
}

func TestUpdatePlaylistWithCoverSongID_RemoteSong(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	ctx := context.Background()

	// Create a remote song with CoverURL
	song := &models.Song{
		Type:     models.TypeRemote,
		Title:    "Remote Song",
		URL:      "https://example.com/song.mp3",
		CoverURL: "https://cdn.example.com/cover.jpg",
	}
	if err := env.songs.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}
	t.Logf("Song created: ID=%d, CoverPath=%q, CoverURL=%q", song.ID, song.CoverPath, song.CoverURL)

	songSvc := services.NewSongService(env.songs, nil, nil, nil, nil, nil)
	handler := NewPlaylistHandler(svc, songSvc)

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "Test Playlist Remote",
	})

	reqBody := map[string]interface{}{
		"name":          "Test Playlist Remote",
		"cover_song_id": song.ID,
	}
	body, _ := json.Marshal(reqBody)

	idStr := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("PUT", "/api/v1/playlists/"+idStr, body, map[string]string{"id": idStr})
	rr := httptest.NewRecorder()

	handler.UpdatePlaylist(rr, req)

	t.Logf("Update response: Status=%d", rr.Code)
	t.Logf("Response body: %s", rr.Body.String())

	if rr.Code != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	// Verify DB state
	updated, err := svc.GetByID(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	t.Logf("DB state: CoverPath=%q, CoverURL=%q", updated.CoverPath, updated.CoverURL)

	if updated.CoverURL != song.CoverURL {
		t.Errorf("expected CoverURL=%q, got %q", song.CoverURL, updated.CoverURL)
	}

	// Verify JSON response has cover_url
	var respPlaylist map[string]interface{}
	json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&respPlaylist)
	coverURL := respPlaylist["cover_url"]
	t.Logf("Response cover_url: %v", coverURL)
	if coverURL == "" || coverURL == nil {
		t.Errorf("expected cover_url to be set")
	}
}

// TestGetPlaylistCoverStaleFileFallsBack 验证读路径防护：
// 歌单 cover_path 指向的文件已丢失时，不直接 404，而是回退到歌曲封面。
func TestGetPlaylistCoverStaleFileFallsBack(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc, nil)
	ctx := context.Background()

	// 真实存在的歌曲封面文件（回退目标）。
	dir := t.TempDir()
	songCover := filepath.Join(dir, "song.jpg")
	if err := os.WriteFile(songCover, []byte("SONG_COVER_BYTES"), 0o644); err != nil {
		t.Fatalf("write song cover: %v", err)
	}

	song := &models.Song{Type: models.TypeLocal, Title: "s1", FilePath: "/music/s1.mp3", CoverPath: songCover}
	if err := env.songs.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	// 歌单封面指向不存在的文件，且无 cover_url。
	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type:      models.PlaylistTypeNormal,
		Name:      "Stale Cover",
		CoverPath: filepath.Join(dir, "missing.jpg"),
	})
	if err := env.playlistSongs.AddSong(ctx, playlist.ID, song.ID, 1); err != nil {
		t.Fatalf("add song: %v", err)
	}

	idStr := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("GET", "/api/v1/playlists/"+idStr+"/cover", nil, map[string]string{"id": idStr})
	rr := httptest.NewRecorder()
	handler.GetPlaylistCover(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected fallback 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "SONG_COVER_BYTES" {
		t.Errorf("expected fallback to song cover bytes, got %q", rr.Body.String())
	}
}

// TestGetPlaylistCoverExistingFileServed 验证正常路径未被破坏：
// cover_path 文件存在时直接返回该文件。
func TestGetPlaylistCoverExistingFileServed(t *testing.T) {
	env := newPlaylistHandlerEnv(t)
	svc := env.newService()
	handler := NewPlaylistHandler(svc, nil)

	dir := t.TempDir()
	coverFile := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(coverFile, []byte("PLAYLIST_COVER_BYTES"), 0o644); err != nil {
		t.Fatalf("write cover: %v", err)
	}

	playlist := createTestPlaylist(t, svc, &models.Playlist{
		Type:      models.PlaylistTypeNormal,
		Name:      "Has Cover",
		CoverPath: coverFile,
	})

	idStr := strconv.FormatInt(playlist.ID, 10)
	req := newRouteRequest("GET", "/api/v1/playlists/"+idStr+"/cover", nil, map[string]string{"id": idStr})
	rr := httptest.NewRecorder()
	handler.GetPlaylistCover(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "PLAYLIST_COVER_BYTES" {
		t.Errorf("expected playlist cover bytes, got %q", rr.Body.String())
	}
}
