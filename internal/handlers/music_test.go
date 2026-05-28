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

// newTestSongRepo 启动 :memory: SQLite，返回 SongRepository(供 song handler 测试共享)。
func newTestSongRepo(t *testing.T) *database.SongRepository {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	return mdb.SongRepository()
}

// seedSong 创建一条歌曲并返回其 ID。
func seedSong(t *testing.T, repo *database.SongRepository, song *models.Song) int64 {
	t.Helper()
	if err := repo.Create(context.Background(), song); err != nil {
		t.Fatalf("seed song: %v", err)
	}
	return song.ID
}

// TestNewSongHandler 测试创建歌曲处理器
func TestNewSongHandler(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	if handler == nil {
		t.Error("NewSongHandler() returned nil")
	}
}

// TestListSongs 测试获取歌曲列表
func TestListSongs(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"})
	seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "歌曲2", FilePath: "/music/2.mp3"})
	seedSong(t, repo, &models.Song{Type: models.TypeRemote, Title: "歌曲3", URL: "https://example.com/3.mp3"})

	req := httptest.NewRequest("GET", "/api/v1/songs", nil)
	rr := httptest.NewRecorder()

	handler.ListSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["total"].(float64) != 3 {
		t.Errorf("total = %v, want 3", resp["total"])
	}
}

// TestListSongsWithFilter 测试带过滤条件的歌曲列表
func TestListSongsWithFilter(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"})
	seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "歌曲2", FilePath: "/music/2.mp3"})
	seedSong(t, repo, &models.Song{Type: models.TypeRemote, Title: "歌曲3", URL: "https://example.com/3.mp3"})

	req := httptest.NewRequest("GET", "/api/v1/songs?type=local", nil)
	rr := httptest.NewRecorder()

	handler.ListSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", resp["total"])
	}
}

// TestGetSong 测试获取单个歌曲
func TestGetSong(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "测试歌曲", FilePath: "/music/test.mp3"})

	req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(id, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.GetSong(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var respSong models.Song
	json.NewDecoder(rr.Body).Decode(&respSong)

	if respSong.Title != "测试歌曲" {
		t.Errorf("song title = %v, want 测试歌曲", respSong.Title)
	}
}

// TestGetSongNotFound 测试获取不存在的歌曲
func TestGetSongNotFound(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/songs/999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.GetSong(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusNotFound)
	}
}

// TestGetSongInvalidID 测试无效的歌曲ID
func TestGetSongInvalidID(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/songs/invalid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.GetSong(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

// TestDeleteSong 测试删除歌曲
func TestDeleteSong(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "测试歌曲", FilePath: "/music/test.mp3"})
	ctx := context.Background()

	idStr := strconv.FormatInt(id, 10)
	req := httptest.NewRequest("DELETE", "/api/v1/songs/"+idStr, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", idStr)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.DeleteSong(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	if _, err := songService.GetByID(ctx, id); err == nil {
		t.Error("song should be deleted")
	}
}

// TestAddRemoteSongs 测试批量添加网络歌曲
func TestAddRemoteSongs(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	reqBody := []map[string]interface{}{
		{
			"url":       "https://example.com/song.mp3",
			"title":     "网络歌曲",
			"artist":    "艺术家",
			"album":     "专辑",
			"cover_url": "https://example.com/cover.jpg",
			"duration":  253.5,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/songs/remote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.AddRemoteSongs(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusCreated)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	songs, ok := resp["songs"].([]interface{})
	if !ok || len(songs) != 1 {
		t.Errorf("expected 1 song in response, got %v", resp["songs"])
	}
	if count, _ := resp["count"].(float64); int(count) != 1 {
		t.Errorf("expected count=1, got %v", resp["count"])
	}
}

// TestAddRemoteSongsMissingFields 测试批量添加网络歌曲缺少必填字段
func TestAddRemoteSongsMissingFields(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	tests := []struct {
		name string
		body interface{}
	}{
		{"missing url", []map[string]string{{"title": "歌曲"}}},
		{"missing title", []map[string]string{{"url": "https://example.com/song.mp3"}}},
		{"empty array", []map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/v1/songs/remote", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.AddRemoteSongs(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

// TestAddRadios 测试批量添加电台/广播
func TestAddRadios(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	reqBody := []map[string]string{
		{
			"url":       "https://example.com/radio.m3u8",
			"title":     "测试电台",
			"cover_url": "https://example.com/cover.jpg",
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/songs/radio", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.AddRadios(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusCreated)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if count, _ := resp["count"].(float64); int(count) != 1 {
		t.Errorf("expected count=1, got %v", resp["count"])
	}
}

// TestDeleteSongInvalidID 测试删除歌曲无效ID
func TestDeleteSongInvalidID(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	req := httptest.NewRequest("DELETE", "/api/v1/songs/invalid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.DeleteSong(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

// TestDeleteSongNotFound 测试删除不存在的歌曲
func TestDeleteSongNotFound(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	req := httptest.NewRequest("DELETE", "/api/v1/songs/999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.DeleteSong(rr, req)

	// 注意：当前实现将所有删除错误都作为 500 处理
	// 这是合理的，因为 Service 层会先验证歌曲是否存在
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusInternalServerError)
	}
}

// TestAddRadiosMissingFields 测试批量添加电台/广播缺少必填字段
func TestAddRadiosMissingFields(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	tests := []struct {
		name string
		body interface{}
	}{
		{"missing url", []map[string]string{{"title": "电台"}}},
		{"missing title", []map[string]string{{"url": "https://example.com/radio.m3u8"}}},
		{"empty array", []map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/v1/songs/radio", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.AddRadios(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

// TestAddRadiosInvalidJSON 测试批量添加电台/广播无效JSON
func TestAddRadiosInvalidJSON(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/api/v1/songs/radio", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.AddRadios(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

// TestAddRemoteSongsInvalidJSON 测试批量添加网络歌曲无效JSON
func TestAddRemoteSongsInvalidJSON(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/api/v1/songs/remote", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.AddRemoteSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

// TestListSongsWithPagination 测试带分页的歌曲列表
func TestListSongsWithPagination(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"})
	seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "歌曲2", FilePath: "/music/2.mp3"})

	req := httptest.NewRequest("GET", "/api/v1/songs?limit=2&offset=1", nil)
	rr := httptest.NewRecorder()

	handler.ListSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["limit"].(float64) != 2 {
		t.Errorf("limit = %v, want 2", resp["limit"])
	}
	if resp["offset"].(float64) != 1 {
		t.Errorf("offset = %v, want 1", resp["offset"])
	}
}

// TestListSongsInvalidPagination 测试无效的分页参数
func TestListSongsInvalidPagination(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/songs?limit=invalid&offset=invalid", nil)
	rr := httptest.NewRecorder()

	handler.ListSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}
