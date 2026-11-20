package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"songloft/internal/contextkeys"
	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
	"songloft/internal/services/playactivity"

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	if handler == nil {
		t.Error("NewSongHandler() returned nil")
	}
}

// TestListSongs 测试获取歌曲列表
func TestListSongs(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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

func TestListSongsWithCommaSeparatedTypes(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地", FilePath: "/music/local.mp3"})
	seedSong(t, repo, &models.Song{Type: models.TypeRemote, Title: "网络", URL: "https://example.com/remote.mp3"})
	seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "电台", URL: "https://example.com/radio.m3u8"})

	req := httptest.NewRequest("GET", "/api/v1/songs?type=local,remote", nil)
	rr := httptest.NewRecorder()
	handler.ListSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("handler returned status %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := int(resp["total"].(float64)); got != 2 {
		t.Fatalf("total = %d, want 2", got)
	}
}

func TestListSongsRejectsUnsupportedType(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/songs?type=unsupported", nil)
	rr := httptest.NewRecorder()
	handler.ListSongs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("handler returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestGetSong 测试获取单个歌曲
func TestGetSong(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

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
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/songs?limit=invalid&offset=invalid", nil)
	rr := httptest.NewRecorder()

	handler.ListSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

// TestServeRadioICYPassthrough 验证电台代理对 ICY 元数据的透传:
//   - 客户端未请求 Icy-MetaData(浏览器 <audio>)→ 上游不应收到该头,响应也不应带 icy-metaint,
//     否则交织的元数据块会污染音频流,播放约 1 秒后中断(#275 回归)。
//   - 客户端显式请求 Icy-MetaData(原生播放器)→ 透传给上游,并回传 icy-metaint 以便定位元数据块。
func TestServeRadioICYPassthrough(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	var gotIcyReq string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIcyReq = r.Header.Get("Icy-MetaData")
		if gotIcyReq == "1" {
			w.Header().Set("icy-metaint", "16000")
		}
		w.Header().Set("Content-Type", "audio/aac")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("audio-bytes"))
	}))
	defer upstream.Close()

	id := seedSong(t, repo, &models.Song{Type: models.TypeRemote, Title: "电台", URL: upstream.URL + "/live"})
	song, err := songService.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get song: %v", err)
	}

	t.Run("浏览器不请求元数据", func(t *testing.T) {
		gotIcyReq = ""
		req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play", nil)
		rr := httptest.NewRecorder()
		handler.serveRadio(rr, req, song)

		if gotIcyReq != "" {
			t.Errorf("上游收到了 Icy-MetaData=%q,期望不发送", gotIcyReq)
		}
		if v := rr.Header().Get("icy-metaint"); v != "" {
			t.Errorf("响应带了 icy-metaint=%q,期望不透传", v)
		}
	})

	t.Run("原生播放器请求元数据", func(t *testing.T) {
		gotIcyReq = ""
		req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play", nil)
		req.Header.Set("Icy-MetaData", "1")
		rr := httptest.NewRecorder()
		handler.serveRadio(rr, req, song)

		if gotIcyReq != "1" {
			t.Errorf("上游 Icy-MetaData=%q,期望透传为 1", gotIcyReq)
		}
		if v := rr.Header().Get("icy-metaint"); v != "16000" {
			t.Errorf("响应 icy-metaint=%q,期望回传 16000", v)
		}
	})
}

// TestServeRadioICYDeinterleave 验证:当上游**无条件**交织 ICY 元数据(Shoutcast v1)时,
//   - 浏览器路径(未请求 Icy-MetaData)→ 代理去交织,body 为纯音频,且不转发 icy-metaint;
//   - 原生路径(请求 Icy-MetaData)→ body 原样透传交织字节,并回传 icy-metaint。(#275)
func TestServeRadioICYDeinterleave(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	const metaint = 16
	// 交织流: [16A][len=1][16B 元数据][16C]
	metaBlock := []byte("StreamTitle='x';") // 恰 16 字节
	interleaved := bytes.Join([][]byte{
		bytes.Repeat([]byte("A"), metaint),
		{byte(len(metaBlock) / 16)},
		metaBlock,
		bytes.Repeat([]byte("C"), metaint),
	}, nil)
	pureAudio := append(bytes.Repeat([]byte("A"), metaint), bytes.Repeat([]byte("C"), metaint)...)

	var gotIcyReq string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIcyReq = r.Header.Get("Icy-MetaData")
		// 无条件交织:不管客户端是否请求都设 icy-metaint 并写交织字节。
		w.Header().Set("icy-metaint", strconv.Itoa(metaint))
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(interleaved)
	}))
	defer upstream.Close()

	id := seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "电台", URL: upstream.URL + "/live"})
	song, err := songService.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get song: %v", err)
	}

	t.Run("浏览器去交织", func(t *testing.T) {
		gotIcyReq = ""
		req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play", nil)
		rr := httptest.NewRecorder()
		handler.serveRadio(rr, req, song)

		if gotIcyReq != "" {
			t.Errorf("上游收到 Icy-MetaData=%q,期望不发送", gotIcyReq)
		}
		if v := rr.Header().Get("icy-metaint"); v != "" {
			t.Errorf("响应带了 icy-metaint=%q,期望去交织后不透传", v)
		}
		if !bytes.Equal(rr.Body.Bytes(), pureAudio) {
			t.Errorf("body=%q,期望去交织后纯音频=%q", rr.Body.Bytes(), pureAudio)
		}
	})

	t.Run("原生透传交织流", func(t *testing.T) {
		gotIcyReq = ""
		req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play", nil)
		req.Header.Set("Icy-MetaData", "1")
		rr := httptest.NewRecorder()
		handler.serveRadio(rr, req, song)

		if gotIcyReq != "1" {
			t.Errorf("上游 Icy-MetaData=%q,期望透传为 1", gotIcyReq)
		}
		if v := rr.Header().Get("icy-metaint"); v != strconv.Itoa(metaint) {
			t.Errorf("响应 icy-metaint=%q,期望回传 %d", v, metaint)
		}
		if !bytes.Equal(rr.Body.Bytes(), interleaved) {
			t.Errorf("body=%q,期望原样透传交织字节=%q", rr.Body.Bytes(), interleaved)
		}
	})
}

// TestNormalizeAudioContentType 验证非标准音频 MIME 归一化(#275)。
// streamtheworld 类 HE-AAC 电台返回 audio/aacp,浏览器 <audio> 据此选不对解码器;
// 实际负载是标准 ADTS AAC,改标 audio/aac 更兼容。参数(如 charset)需保留,未命中原样透传。
func TestNormalizeAudioContentType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"audio/aacp", "audio/aac"},
		{"audio/aacp; charset=utf-8", "audio/aac; charset=utf-8"},
		{"AUDIO/AACP", "audio/aac"},
		{"audio/x-aac", "audio/aac"},
		{"audio/x-aacp", "audio/aac"},
		{"audio/aac", "audio/aac"},
		{"audio/mpeg", "audio/mpeg"},
		{"application/octet-stream", "application/octet-stream"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeAudioContentType(c.in); got != c.want {
			t.Errorf("normalizeAudioContentType(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

// TestServeRadioNormalizesAACPContentType 验证 serveRadio 把上游 audio/aacp 改标为 audio/aac(#275)。
func TestServeRadioNormalizesAACPContentType(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/aacp")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("aac-bytes"))
	}))
	defer upstream.Close()

	id := seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "电台", URL: upstream.URL + "/live.aac"})
	song, err := songService.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get song: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play", nil)
	rr := httptest.NewRecorder()
	handler.serveRadio(rr, req, song)

	if ct := rr.Header().Get("Content-Type"); ct != "audio/aac" {
		t.Errorf("Content-Type=%q,期望归一化为 audio/aac", ct)
	}
}

// TestServeRadioUsesNonBrowserUserAgent 验证 serveRadio 向上游发的 User-Agent 不是浏览器风格(songloft#275)。
// streamtheworld 等防盗链电台检测到浏览器 UA 会只回约 32KB 预览就断流(约 3 秒),
// 导致所有经本机代理播放的客户端(web/桌面/小爱音箱)约 3 秒后断开。
func TestServeRadioUsesNonBrowserUserAgent(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	var gotUA string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mp3-bytes"))
	}))
	defer upstream.Close()

	id := seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "电台", URL: upstream.URL + "/live.mp3"})
	song, err := songService.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get song: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play", nil)
	rr := httptest.NewRecorder()
	handler.serveRadio(rr, req, song)

	if gotUA == "" {
		t.Fatal("上游未收到 User-Agent")
	}
	if strings.Contains(gotUA, "Mozilla") || strings.Contains(gotUA, "Chrome") || strings.Contains(gotUA, "Safari") {
		t.Errorf("serveRadio 向上游发了浏览器风格 UA=%q,会触发防盗链电台断流", gotUA)
	}
}

// TestServeRadioHLSDirectBypassesProxy 验证 hls=direct 让原生端绕过 HLS 反代直接 302(#249)。
// 即使 /settings/hls-proxy 已开,带 hls=direct 的请求也应 302 直连源站,
// 避免直播切片经反代往返后过期。不带该参数时(浏览器)仍走反代。
func TestServeRadioHLSDirectBypassesProxy(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	repo := mdb.SongRepository()
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	configService := services.NewConfigService(mdb.ConfigRepository())

	// 上游 m3u8:被反代时会命中(返回 m3u8 文本);被 302 直连时不应命中。
	var upstreamHit bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.Header().Set("Content-Type", hlsContentType)
		w.Write([]byte("#EXTM3U\n#EXT-X-ENDLIST\n"))
	}))
	defer upstream.Close()

	songURL := upstream.URL + "/live/index.m3u8"
	hlsHandler := NewHLSHandler(songService, configService)
	hlsHandler.allowHost = func(string) bool { return true }
	if err := hlsHandler.SetEnabled(true); err != nil {
		t.Fatalf("enable hls proxy: %v", err)
	}
	handler := NewSongHandler(songService, nil, nil, nil, hlsHandler, nil)

	id := seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "HLS电台", URL: songURL})
	song, err := songService.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get song: %v", err)
	}

	t.Run("原生 hls=direct 绕过反代 302 直连", func(t *testing.T) {
		upstreamHit = false
		req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play.m3u8?hls=direct", nil)
		rr := httptest.NewRecorder()
		handler.serveRadio(rr, req, song)

		if rr.Code != http.StatusFound {
			t.Fatalf("status=%d,期望 302", rr.Code)
		}
		if loc := rr.Header().Get("Location"); loc != songURL {
			t.Errorf("Location=%q,期望直连源站 %q", loc, songURL)
		}
		if upstreamHit {
			t.Error("hls=direct 时上游 m3u8 被反代命中,期望 302 不拉取")
		}
	})

	t.Run("浏览器不带 hls=direct 走反代", func(t *testing.T) {
		upstreamHit = false
		req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play.m3u8", nil)
		rr := httptest.NewRecorder()
		handler.serveRadio(rr, req, song)

		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d,期望反代 200,body=%q", rr.Code, rr.Body.String())
		}
		if !upstreamHit {
			t.Error("反代路径未拉取上游 m3u8")
		}
	})
}

// TestParseSeekSeconds 验证 seek 参数解析与「尾部夹紧」。
// 夹紧是关键：seek 越过文件尾会让 ffmpeg 零输出，进而触发无损降级把整首歌从头重播一遍，
// 比忽略 seek 更糟（songloft-plugin-miot#60）。
func TestParseSeekSeconds(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		duration float64
		want     float64
	}{
		{"空值", "", 200, 0},
		{"零", "0", 200, 0},
		{"负数", "-5", 200, 0},
		{"非数字", "abc", 200, 0},
		{"无穷", "Inf", 200, 0},
		{"NaN", "NaN", 200, 0},
		{"正常值", "60", 200, 60},
		{"小数", "60.5", 200, 60.5},
		{"贴近结尾被夹紧", "199", 200, 0},
		{"超过时长被夹紧", "9999", 200, 0},
		{"时长未知不夹紧", "9999", 0, 9999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSeekSeconds(tc.raw, tc.duration); got != tc.want {
				t.Errorf("parseSeekSeconds(%q, %v) = %v, want %v", tc.raw, tc.duration, got, tc.want)
			}
		})
	}
}

// TestParseSpeed 验证倍速参数解析与 [0.5, 2.0] 夹紧。
// 超出该区间需要链式拼接多个 atempo（单个 atempo 只支持 0.5–2.0），目前不支持，越界直接夹紧。
// 非法值回退 1.0（不变速），让缺省/坏参数的客户端拿到原速播放而非报错。
func TestParseSpeed(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want float64
	}{
		{"空值", "", 1.0},
		{"零", "0", 1.0},
		{"负数", "-1.5", 1.0},
		{"非数字", "fast", 1.0},
		{"无穷", "Inf", 1.0},
		{"NaN", "NaN", 1.0},
		{"正常1倍", "1.0", 1.0},
		{"正常1.5倍", "1.5", 1.5},
		{"正常0.75倍", "0.75", 0.75},
		{"低于下界夹紧", "0.3", speedMin},
		{"高于上界夹紧", "3.0", speedMax},
		{"下界本身", "0.5", 0.5},
		{"上界本身", "2.0", 2.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSpeed(tc.raw); got != tc.want {
				t.Errorf("parseSpeed(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// newSeekTestHandler 造一个带假 ffmpeg 的 handler：ffmpeg 换成 /bin/echo，
// 于是响应 body 就是 ffmpeg 的参数列表，可直接断言参数契约。ffmpegPath 传空则模拟缺 ffmpeg。
func newSeekTestHandler(t *testing.T, ffmpegPath string) (*SongHandler, *database.SongRepository, *services.SongService) {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	repo := mdb.SongRepository()
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	cacheService := services.NewCacheService(t.TempDir(), services.NewConfigService(mdb.ConfigRepository()))
	cacheService.SetFFmpegPath(ffmpegPath)
	return NewSongHandler(songService, cacheService, nil, nil, nil, nil), repo, songService
}

// writeSeekTestFile 写一个测试音频文件并返回路径。
func writeSeekTestFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("audio-file-bytes"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return p
}

// playSeekRequest 发一次带 seek 的播放请求。
func playSeekRequest(t *testing.T, handler *SongHandler, id int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	return playSeekRequestMethod(t, handler, id, query, "GET")
}

func playSeekRequestMethod(t *testing.T, handler *SongHandler, id int64, query, method string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play?"+query, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(id, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	handler.GetSongPlay(rr, req)
	return rr
}

type blockingResponseWriter struct {
	header  http.Header
	started chan struct{}
	release chan struct{}
	once    sync.Once
	body    bytes.Buffer
	status  int
}

func newBlockingResponseWriter() *blockingResponseWriter {
	return &blockingResponseWriter{
		header:  make(http.Header),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (w *blockingResponseWriter) Header() http.Header {
	return w.header
}

func (w *blockingResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *blockingResponseWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.release
	return w.body.Write(p)
}

func TestGetSongPlayTracksAndCancelsLocalStreams(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	repo := db.SongRepository()
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	activity := playactivity.New()
	handler := NewSongHandler(songService, nil, nil, nil, nil, activity)

	firstPath := writeSeekTestFile(t, "first.mp3")
	secondPath := writeSeekTestFile(t, "second.mp3")
	firstID := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "第一首", FilePath: firstPath})
	secondID := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "第二首", FilePath: secondPath})
	request := func(id int64) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatInt(id, 10))
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, contextkeys.ClientIDKey, "speaker-client")
		return req.WithContext(ctx)
	}

	firstWriter := newBlockingResponseWriter()
	firstDone := make(chan struct{})
	go func() {
		handler.GetSongPlay(firstWriter, request(firstID))
		close(firstDone)
	}()
	select {
	case <-firstWriter.started:
	case <-time.After(time.Second):
		t.Fatal("first local stream did not start")
	}

	sk := playactivity.SessionKey{ClientID: "speaker-client"}
	if got := activity.Size(sk); got != 1 {
		t.Fatalf("active local stream count = %d, want 1", got)
	}

	secondWriter := httptest.NewRecorder()
	handler.GetSongPlay(secondWriter, request(secondID))
	if secondWriter.Code != http.StatusOK || secondWriter.Body.String() != "audio-file-bytes" {
		t.Fatalf("second local stream = %d %q", secondWriter.Code, secondWriter.Body.String())
	}

	close(firstWriter.release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("canceled first local stream did not return")
	}
	if got := activity.Size(sk); got != 0 {
		t.Fatalf("active local stream count after switch = %d, want 0", got)
	}
}

// TestGetSongPlaySeekStreamsMP3 验证本地歌曲带 seek 时走流式 MP3：
// chunked（无 Content-Length）、no-store，且 ffmpeg 参数契约完整。
func TestGetSongPlaySeekStreamsMP3(t *testing.T) {
	handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
	src := writeSeekTestFile(t, "song.mp3")
	id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})

	rr := playSeekRequest(t, handler, id, "seek=60")

	if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("Content-Type=%q, 期望 audio/mpeg", ct)
	}
	if cl := rr.Header().Get("Content-Length"); cl != "5600000" {
		t.Errorf("Content-Length=%q, 期望 5600000（140s × 320kbps/8）", cl)
	}
	if ar := rr.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("Accept-Ranges=%q, 期望 bytes", ar)
	}
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control=%q, 期望含 no-store", cc)
	}
	body := rr.Body.String()
	for _, want := range []string{"-ss 60.000", src, "-map 0:a:0", "-codec:a copy", "-write_xing 0", "-f mp3"} {
		if !strings.Contains(body, want) {
			t.Errorf("ffmpeg 参数缺 %q，实际: %s", want, body)
		}
	}
}

// TestGetSongPlaySeekFallsBackToWholeFile 验证 seek 流无法开始时无损降级为「从头完整提供文件」：
// body 必须等于原文件字节、带 Content-Length，且预设的 seek 响应头已被清掉。
func TestGetSongPlaySeekFallsBackToWholeFile(t *testing.T) {
	cases := []struct {
		name       string
		ffmpegPath string
	}{
		{"缺 ffmpeg", ""},
		{"ffmpeg 零输出", "/bin/true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, repo, _ := newSeekTestHandler(t, tc.ffmpegPath)
			src := writeSeekTestFile(t, "song.mp3")
			id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})

			rr := playSeekRequest(t, handler, id, "seek=60")

			if rr.Body.String() != "audio-file-bytes" {
				t.Errorf("body=%q, 期望原文件完整字节", rr.Body.String())
			}
			if rr.Header().Get("Content-Length") == "" {
				t.Error("降级后应由 http.ServeFile 提供 Content-Length")
			}
			if cc := rr.Header().Get("Cache-Control"); strings.Contains(cc, "no-store") {
				t.Errorf("Cache-Control=%q, 降级时应清掉 seek 的 no-store", cc)
			}
		})
	}
}

// TestGetSongPlayRepeatedRangesStayByteBased 防止把同设备的重复 Range 读取
// 当成播放进度：音箱可能预读整首，也可能重新播放，服务端必须遵守当前请求的字节区间。
func TestGetSongPlayRepeatedRangesStayByteBased(t *testing.T) {
	handler, repo, _ := newSeekTestHandler(t, "")
	src := writeSeekTestFile(t, "song.mp3")
	id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})
	for _, tc := range []struct {
		rangeHeader  string
		body         string
		contentRange string
	}{
		{"bytes=0-3", "audi", "bytes 0-3/16"},
		{"bytes=4-7", "o-fi", "bytes 4-7/16"},
		{"bytes=0-3", "audi", "bytes 0-3/16"},
		{"", "audio-file-bytes", ""},
		{"", "audio-file-bytes", ""},
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/songs/"+strconv.FormatInt(id, 10)+"/play?normalize=0", nil)
		r.RemoteAddr = "10.0.0.56:59000"
		r.Header.Set("Range", tc.rangeHeader)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatInt(id, 10))
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		handler.GetSongPlay(w, r)
		wantStatus := http.StatusOK
		if tc.rangeHeader != "" {
			wantStatus = http.StatusPartialContent
		}
		if w.Code != wantStatus || w.Body.String() != tc.body {
			t.Fatalf("Range %q: status=%d body=%q, want status=%d body=%q", tc.rangeHeader, w.Code, w.Body.String(), wantStatus, tc.body)
		}
		if got := w.Header().Get("Content-Range"); got != tc.contentRange {
			t.Errorf("Range %q: Content-Range=%q, want %q", tc.rangeHeader, got, tc.contentRange)
		}
		if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(tc.body)) {
			t.Errorf("Range %q: Content-Length=%q, want %d", tc.rangeHeader, got, len(tc.body))
		}
	}
}

// TestGetSongPlaySeekIgnored 验证不该起 seek 流的场景：视频画面、prefetch、HEAD、电台。
func TestGetSongPlaySeekIgnored(t *testing.T) {
	t.Run("media=video 直出原容器", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "clip.mp4")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "视频", FilePath: src, Format: "mp4", Duration: 200})

		rr := playSeekRequest(t, handler, id, "media=video&seek=30")

		if ct := rr.Header().Get("Content-Type"); ct != "video/mp4" {
			t.Errorf("Content-Type=%q, 期望 video/mp4（seek 被忽略）", ct)
		}
		if rr.Body.String() != "audio-file-bytes" {
			t.Errorf("body=%q, 期望原文件字节", rr.Body.String())
		}
	})

	t.Run("HEAD 不起 ffmpeg", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})

		rr := playSeekRequestMethod(t, handler, id, "seek=60", "HEAD")

		if strings.Contains(rr.Body.String(), "-ss") {
			t.Errorf("HEAD 起了 ffmpeg: %s", rr.Body.String())
		}
	})

	t.Run("prefetch 立即 202", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})

		rr := playSeekRequest(t, handler, id, "prefetch=1&seek=60")

		if rr.Code != http.StatusAccepted {
			t.Errorf("status=%d, 期望 202", rr.Code)
		}
		if strings.Contains(rr.Body.String(), "-ss") {
			t.Errorf("prefetch 起了 seek 流: %s", rr.Body.String())
		}
	})

	t.Run("电台直播忽略 seek", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write([]byte("live-bytes"))
		}))
		defer upstream.Close()
		id := seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "电台", URL: upstream.URL + "/live"})

		rr := playSeekRequest(t, handler, id, "seek=60")

		if rr.Body.String() != "live-bytes" {
			t.Errorf("body=%q, 期望原样代理直播流", rr.Body.String())
		}
	})
}

// TestGetSongPlaySeekRemote 验证网络歌曲：已缓存时走 seek 流，未缓存时忽略 seek 直接代理
// （未缓存下拿到本地文件要先同步下载整首，会让「续播」这一下按键卡住一整首下载时长）。
func TestGetSongPlaySeekRemote(t *testing.T) {
	t.Run("已缓存走 seek 流", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		cached := writeSeekTestFile(t, "cached.mp3")
		// Remote cache hits now require real audio; this test isolates seek arguments.
		if err := os.WriteFile(cached, cacheTestWAV(200), 0644); err != nil {
			t.Fatal(err)
		}
		id := seedSong(t, repo, &models.Song{
			Type: models.TypeRemote, Title: "网络歌", URL: "https://example.com/a.mp3",
			Format: "mp3", Duration: 200,
		})
		// cache_path 由专门的 UpdateCachePath 写入，Create 不落这一列
		if err := repo.UpdateCachePath(context.Background(), id, cached); err != nil {
			t.Fatalf("update cache path: %v", err)
		}

		rr := playSeekRequest(t, handler, id, "seek=60")

		if !strings.Contains(rr.Body.String(), "-ss 60.000") {
			t.Errorf("已缓存网络歌未走 seek 流: %s", rr.Body.String())
		}
	})

	t.Run("未缓存忽略 seek", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write([]byte("proxied-bytes"))
		}))
		defer upstream.Close()
		id := seedSong(t, repo, &models.Song{
			Type: models.TypeRemote, Title: "网络歌", URL: upstream.URL + "/a.mp3", Duration: 200,
		})

		rr := playSeekRequest(t, handler, id, "seek=60")

		if strings.Contains(rr.Body.String(), "-ss") {
			t.Errorf("未缓存网络歌起了 seek 流: %s", rr.Body.String())
		}
	})
}

// TestGetSongPlayNormalizeStreamsLive 验证「开了音量均衡但均衡产物还没转好」时，播放请求
// 立刻边转边发，而不是被整首 loudnorm 同步阻塞。
//
// 修复前实测 dur_ms=22392 / 24348 / 22381：音箱那端是「前 20 多秒空白」，而插件的自动切歌
// 定时器在推 URL 那一刻就起算，尾部又被砍掉同样长度（songloft-org/songloft-plugin-miot#61）。
func TestGetSongPlayNormalizeStreamsLive(t *testing.T) {
	t.Run("产物未就绪走实时流", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})

		rr := playSeekRequest(t, handler, id, "normalize=1&format=mp3")

		if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
			t.Errorf("Content-Type=%q, 期望 audio/mpeg", ct)
		}
		// 实时流带 CBR 估算的 Content-Length + Accept-Ranges（songloft-org/songloft#442）：
		// 少了这两个头，浏览器判定该流不可 seek，拖动进度条会静默卡死（实测 readyState 4→1、
		// buffered 零增长、无 error）。均衡流从 #61 起就有同样问题，一并修掉。
		// 200s × 320kbps / 8 = 8,000,000 字节。
		if cl := rr.Header().Get("Content-Length"); cl != "8000000" {
			t.Errorf("Content-Length=%q, 期望 8000000（200s × 320k CBR）", cl)
		}
		if ar := rr.Header().Get("Accept-Ranges"); ar != "bytes" {
			t.Errorf("Accept-Ranges=%q, 期望 bytes", ar)
		}
		body := rr.Body.String()
		for _, want := range []string{"-af loudnorm=", "-codec:a libmp3lame", "-b:a 320k", "-f mp3", "pipe:1"} {
			if !strings.Contains(body, want) {
				t.Errorf("实时均衡流参数缺 %q，实际: %s", want, body)
			}
		}
		if strings.Contains(body, "-ss") {
			t.Errorf("没请求 seek 却出现 -ss: %s", body)
		}
	})

	t.Run("均衡叠加 seek 由同一条 ffmpeg 完成", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})

		rr := playSeekRequest(t, handler, id, "normalize=1&format=mp3&seek=60")

		body := rr.Body.String()
		for _, want := range []string{"-ss 60.000", "-af loudnorm=", "-codec:a libmp3lame"} {
			if !strings.Contains(body, want) {
				t.Errorf("参数缺 %q，实际: %s", want, body)
			}
		}
		// 只该起一条 ffmpeg：出现两次 -f mp3 说明先转码再 seek 各跑了一遍
		if n := strings.Count(body, "pipe:1"); n != 1 {
			t.Errorf("pipe:1 出现 %d 次，期望 1（seek 与均衡应由同一条 ffmpeg 完成）: %s", n, body)
		}
	})

	// media=video 与 normalize 冲突：均衡链一律 -vn，无论走实时流还是阻塞转码都会把画面切掉。
	// 上游因此在 videoIntent 下强制关掉 normalize，直出原容器（原先 videoIntent 清空 targetFormat
	// 保画面，紧接着 normalize 又把它填回 mp3，画面照样丢——#315 起就存在的坑）。
	t.Run("media=video 直出原容器、不做均衡", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "clip.mp4")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "视频", FilePath: src, Format: "mp4", Duration: 200})

		rr := playSeekRequest(t, handler, id, "normalize=1&media=video")

		if strings.Contains(rr.Body.String(), "loudnorm") {
			t.Errorf("media=video 起了均衡（-vn 会丢画面）: %s", rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); ct != "video/mp4" {
			t.Errorf("Content-Type=%q, 期望 video/mp4（画面必须保住）", ct)
		}
		if rr.Body.String() != "audio-file-bytes" {
			t.Errorf("body=%q, 期望原容器字节（未经任何 ffmpeg）", rr.Body.String())
		}
	})

	t.Run("HEAD 不起实时流", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 200})

		rr := playSeekRequestMethod(t, handler, id, "normalize=1&format=mp3", "HEAD")

		if strings.Contains(rr.Body.String(), "loudnorm") {
			t.Errorf("HEAD 起了均衡流: %s", rr.Body.String())
		}
	})
}

// writeRealMP3TestFile 把仓库里的真实 MP3 样本复制到临时目录并返回路径。
//
// 需要「真」mp3 而不是 writeSeekTestFile 那种占位字节，是因为 NeedsTranscodeForServe 在
// 「声称格式 == 目标格式」时会用 magic bytes 复核（防伪装扩展名，songloft-org/songloft#300）：
// 占位内容会被判成伪 mp3 而强制转码，测不出「无需转码」这条短路。
func writeRealMP3TestFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../pkg/tag/testdata/with_tags/sample.id3v23.mp3")
	if err != nil {
		t.Fatalf("read sample mp3: %v", err)
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return p
}

// TestGetSongPlayTranscodeStreamsLive 验证普通 ?format= / ?quality= 转码播放也走边转边发
// （songloft-org/songloft#442）。
//
// 修复前只有 normalize 有这条快路径，普通转码一律阻塞到整首转完才发第一个字节——弱 CPU
// （报告环境是 N5095）上无损转 mp3 要等 5 秒以上才出声。
//
// 同时钉住「泛化后不能对不需要转码的请求误起 ffmpeg」这条不变量：守卫从原来的
// `!opts.normalize` 换成了自洽的 needsTranscode 判断，判错的代价是每次播放都白烧一个
// libmp3lame 进程。
func TestParseFirstByteRange(t *testing.T) {
	const total = 1000
	cases := []struct {
		header                    string
		wantStart, wantEnd        int64
		wantOK, wantUnsatisfiable bool
	}{
		{"", 0, 0, false, false},
		{"bytes=0-", 0, total - 1, true, false},
		{"bytes=500-", 500, total - 1, true, false},
		{"bytes=100-199", 100, 199, true, false},
		{"bytes=100-99999", 100, total - 1, true, false},
		{"bytes=-200", total - 200, total - 1, true, false},
		{"bytes=-99999", 0, total - 1, true, false},
		{"bytes=1000-", 0, 0, false, true},
		{"bytes=2000-3000", 0, 0, false, true},
		{"bytes=0-99,200-299", 0, 0, false, false},
		{"bytes=abc-", 0, 0, false, false},
		{"bytes=200-100", 0, 0, false, false},
		{"bytes=-0", 0, 0, false, false},
		{"items=0-100", 0, 0, false, false},
		{"bytes=-", 0, 0, false, false},
	}
	for _, c := range cases {
		t.Run(c.header, func(t *testing.T) {
			start, end, ok, unsat := parseFirstByteRange(c.header, total)
			if ok != c.wantOK || unsat != c.wantUnsatisfiable {
				t.Fatalf("ok=%v unsatisfiable=%v, want ok=%v unsatisfiable=%v", ok, unsat, c.wantOK, c.wantUnsatisfiable)
			}
			if ok && (start != c.wantStart || end != c.wantEnd) {
				t.Errorf("range = [%d,%d], want [%d,%d]", start, end, c.wantStart, c.wantEnd)
			}
		})
	}
}

func TestPlanCBRRange(t *testing.T) {
	song := &models.Song{ID: 1, Duration: 200}
	newReq := func(rangeHeader string) *http.Request {
		r := httptest.NewRequest("GET", "/play", nil)
		if rangeHeader != "" {
			r.Header.Set("Range", rangeHeader)
		}
		return r
	}

	t.Run("no range gives total length but not 206", func(t *testing.T) {
		p := planCBRRange(newReq(""), song, servePlayOptions{targetFormat: "mp3", speed: 1.0})
		if p == nil {
			t.Fatal("want range mode enabled")
		}
		if p.totalBytes != 8_000_000 || p.contentLength() != 8_000_000 {
			t.Errorf("totalBytes=%d contentLength=%d, want both 8000000", p.totalBytes, p.contentLength())
		}
		if p.partial {
			t.Error("no Range header must not be 206")
		}
	})

	t.Run("bytes=0- answered as full 200", func(t *testing.T) {
		p := planCBRRange(newReq("bytes=0-"), song, servePlayOptions{targetFormat: "mp3", speed: 1.0})
		if p == nil || p.partial {
			t.Fatalf("want full 200, got plan=%+v", p)
		}
		if p.startSecond != 0 {
			t.Errorf("startSecond=%v, want 0", p.startSecond)
		}
	})

	t.Run("byte offset converts back to seconds", func(t *testing.T) {
		p := planCBRRange(newReq("bytes=2400000-"), song, servePlayOptions{targetFormat: "mp3", speed: 1.0})
		if p == nil || !p.partial {
			t.Fatalf("want 206, got plan=%+v", p)
		}
		if p.startSecond != 60 {
			t.Errorf("startSecond=%v, want 60", p.startSecond)
		}
		if p.contentLength() != 8_000_000-2_400_000 {
			t.Errorf("contentLength=%d, want %d", p.contentLength(), 8_000_000-2_400_000)
		}
	})

	t.Run("quality changes the byte-to-time ratio", func(t *testing.T) {
		p := planCBRRange(newReq("bytes=960000-"), song, servePlayOptions{targetFormat: "mp3", bitrate: 128, speed: 1.0})
		if p == nil {
			t.Fatal("want range mode enabled")
		}
		if p.totalBytes != 3_200_000 {
			t.Errorf("totalBytes=%d, want 3200000", p.totalBytes)
		}
		if p.startSecond != 60 {
			t.Errorf("startSecond=%v, want 60", p.startSecond)
		}
	})

	t.Run("start past the end marks 416", func(t *testing.T) {
		p := planCBRRange(newReq("bytes=9000000-"), song, servePlayOptions{targetFormat: "mp3", speed: 1.0})
		if p == nil || !p.unsatisfiable {
			t.Fatalf("want unsatisfiable, got plan=%+v", p)
		}
	})

	t.Run("seek uses remaining duration", func(t *testing.T) {
		p := planCBRRange(newReq(""), song, servePlayOptions{targetFormat: "mp3", speed: 1.0, seekSeconds: 30})
		if p == nil {
			t.Fatal("want range mode enabled for seek")
		}
		if p.totalBytes != 6_800_000 {
			t.Errorf("totalBytes=%d, want 6800000 (170s × 40000 B/s)", p.totalBytes)
		}
		if p.partial {
			t.Error("no Range header must not be 206")
		}
	})

	t.Run("disabled when preconditions unmet", func(t *testing.T) {
		cases := []struct {
			name string
			song *models.Song
			opts servePlayOptions
		}{
			{"unknown duration", &models.Song{ID: 1, Duration: 0}, servePlayOptions{targetFormat: "mp3", speed: 1.0}},
			{"speed change", song, servePlayOptions{targetFormat: "mp3", speed: 1.5}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				if p := planCBRRange(newReq("bytes=100-"), c.song, c.opts); p != nil {
					t.Errorf("want range mode disabled, got plan=%+v", p)
				}
			})
		}
	})
}

func TestExactLengthWriter(t *testing.T) {
	t.Run("truncates overflow and stops upstream", func(t *testing.T) {
		var buf bytes.Buffer
		ew := &exactLengthWriter{w: &buf, want: 10}
		n, err := ew.Write([]byte("0123456789ABCDEF"))
		if n != 10 {
			t.Errorf("n=%d, want 10", n)
		}
		if !errors.Is(err, errExactLengthReached) {
			t.Errorf("err=%v, want errExactLengthReached", err)
		}
		if buf.String() != "0123456789" {
			t.Errorf("wrote %q, want %q", buf.String(), "0123456789")
		}
	})

	t.Run("further writes report the sentinel", func(t *testing.T) {
		var buf bytes.Buffer
		ew := &exactLengthWriter{w: &buf, want: 4}
		_, _ = ew.Write([]byte("abcd"))
		if _, err := ew.Write([]byte("efgh")); !errors.Is(err, errExactLengthReached) {
			t.Errorf("err=%v, want errExactLengthReached", err)
		}
		if buf.Len() != 4 {
			t.Errorf("wrote %d bytes, want 4", buf.Len())
		}
	})

	t.Run("pads with zeros when short", func(t *testing.T) {
		var buf bytes.Buffer
		ew := &exactLengthWriter{w: &buf, want: 100}
		_, _ = ew.Write([]byte("short"))
		ew.fill()
		if buf.Len() != 100 {
			t.Fatalf("padded to %d bytes, want 100", buf.Len())
		}
		if !bytes.Equal(buf.Bytes()[5:], make([]byte, 95)) {
			t.Error("padding must be zero bytes")
		}
	})

	t.Run("exact fit reports no error", func(t *testing.T) {
		var buf bytes.Buffer
		ew := &exactLengthWriter{w: &buf, want: 4}
		if n, err := ew.Write([]byte("abcd")); n != 4 || err != nil {
			t.Errorf("n=%d err=%v, want 4/nil", n, err)
		}
	})
}

func TestGetSongPlayTranscodeStreamsLive(t *testing.T) {
	t.Run("无损源 format=mp3 走实时流", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.flac")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "无损歌", FilePath: src, Format: "flac", Duration: 240})

		rr := playSeekRequest(t, handler, id, "format=mp3")

		if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
			t.Errorf("Content-Type=%q, 期望 audio/mpeg", ct)
		}
		// 240s × 320kbps / 8 = 9,600,000 字节。带上 Content-Length + Accept-Ranges 客户端才能
		// 把「第 N 秒」映射成字节偏移去发 Range 请求，否则拖动会静默卡死（见 cbrRangePlan）。
		if cl := rr.Header().Get("Content-Length"); cl != "9600000" {
			t.Errorf("Content-Length=%q, 期望 9600000（240s × 320k CBR）", cl)
		}
		if ar := rr.Header().Get("Accept-Ranges"); ar != "bytes" {
			t.Errorf("Accept-Ranges=%q, 期望 bytes", ar)
		}
		if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("Cache-Control=%q, 期望含 no-store", cc)
		}
		body := rr.Body.String()
		for _, want := range []string{"-codec:a libmp3lame", "-b:a 320k", "-map 0:a:0", "-vn", "-write_xing 0", "-f mp3", "pipe:1"} {
			if !strings.Contains(body, want) {
				t.Errorf("实时转码流参数缺 %q，实际: %s", want, body)
			}
		}
		if strings.Contains(body, "-ss") {
			t.Errorf("没请求 seek 却出现 -ss: %s", body)
		}
		if strings.Contains(body, "-af") {
			t.Errorf("没请求均衡/变速却出现滤镜: %s", body)
		}
	})

	// ?quality= 被上游翻成 bitrate 并把 targetFormat 自动填成 mp3，各档都要如实出现在 -b:a 上。
	for _, q := range []string{"128", "192", "320"} {
		t.Run("quality="+q+" 走实时流且码率如实", func(t *testing.T) {
			handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
			src := writeSeekTestFile(t, "song.flac")
			id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "无损歌", FilePath: src, Format: "flac", Duration: 240})

			rr := playSeekRequest(t, handler, id, "quality="+q)

			body := rr.Body.String()
			if !strings.Contains(body, "pipe:1") {
				t.Fatalf("quality=%s 未走实时流，实际: %s", q, body)
			}
			if want := "-b:a " + q + "k"; !strings.Contains(body, want) {
				t.Errorf("参数缺 %q，实际: %s", want, body)
			}
		})
	}

	// 源已是 mp3 但请求了 quality：必须重编码到请求码率，不能 copy（copy 出来是源码率）。
	t.Run("mp3 源指定 quality 仍重编码", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeRealMP3TestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "mp3 歌", FilePath: src, Format: "mp3", Duration: 240})

		rr := playSeekRequest(t, handler, id, "quality=128")

		body := rr.Body.String()
		if !strings.Contains(body, "-b:a 128k") || !strings.Contains(body, "-codec:a libmp3lame") {
			t.Errorf("期望按 128k 重编码，实际: %s", body)
		}
		if strings.Contains(body, "-codec:a copy") {
			t.Errorf("指定 quality 时走了 copy（码率会是源码率而非请求值）: %s", body)
		}
	})

	// 这是本次泛化最大的风险面：真 mp3 源 + format=mp3 根本不需要转码，
	// 必须直出原文件（支持 Range），一个 ffmpeg 都不能起。
	t.Run("无需转码时不起 ffmpeg", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeRealMP3TestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "mp3 歌", FilePath: src, Format: "mp3", Duration: 240})

		rr := playSeekRequest(t, handler, id, "format=mp3")

		body := rr.Body.String()
		if strings.Contains(body, "pipe:1") || strings.Contains(body, "-codec:a") {
			t.Errorf("无需转码却起了 ffmpeg: %s", body)
		}
		if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "public") {
			t.Errorf("Cache-Control=%q, 期望走 ServeFile 的 public 长缓存", cc)
		}
	})

	// pipe 固定输出 MP3，冒充不了 m4a/ogg/flac/wav —— 这些目标一律留给阻塞转码路径。
	t.Run("目标格式非 mp3 不走实时流", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeRealMP3TestFile(t, "song.mp3")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "mp3 歌", FilePath: src, Format: "mp3", Duration: 240})

		rr := playSeekRequest(t, handler, id, "format=flac")

		if body := rr.Body.String(); strings.Contains(body, "pipe:1") {
			t.Errorf("目标 flac 走了只会出 mp3 的实时流: %s", body)
		}
	})

	// 转码产物已落盘 → 交给 ServeFile（支持 Range 且更快），不该再起实时流。
	t.Run("转码产物已就绪走 ServeFile", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.flac")
		song := &models.Song{Type: models.TypeLocal, Title: "无损歌", FilePath: src, Format: "flac", Duration: 240}
		id := seedSong(t, repo, song)
		if _, err := handler.cacheService.GetOrTranscode(context.Background(), src, song, "mp3", 0, -1, false); err != nil {
			t.Fatalf("预置转码产物失败: %v", err)
		}

		rr := playSeekRequest(t, handler, id, "format=mp3")

		if body := rr.Body.String(); strings.Contains(body, "pipe:1") {
			t.Errorf("产物已就绪却仍走实时流: %s", body)
		}
		if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "public") {
			t.Errorf("Cache-Control=%q, 期望走 ServeFile 的 public 长缓存", cc)
		}
	})

	// 修复前这两件事要两条 ffmpeg：先整首转码落盘，再对产物起一条 seek 流。
	t.Run("转码叠加 seek 由同一条 ffmpeg 完成", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.flac")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "无损歌", FilePath: src, Format: "flac", Duration: 240})

		rr := playSeekRequest(t, handler, id, "format=mp3&seek=60")

		body := rr.Body.String()
		for _, want := range []string{"-ss 60.000", "-codec:a libmp3lame"} {
			if !strings.Contains(body, want) {
				t.Errorf("参数缺 %q，实际: %s", want, body)
			}
		}
		if n := strings.Count(body, "pipe:1"); n != 1 {
			t.Errorf("pipe:1 出现 %d 次，期望 1（转码与 seek 应由同一条 ffmpeg 完成）: %s", n, body)
		}
	})

	// media=video 下上游已清空 targetFormat 保画面，实时流的 -vn 会把画面切掉，必须不接管。
	t.Run("media=video 直出原容器", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "clip.mkv")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "视频", FilePath: src, Format: "mkv", Duration: 240})

		rr := playSeekRequest(t, handler, id, "format=mp3&media=video")

		if body := rr.Body.String(); body != "audio-file-bytes" {
			t.Errorf("body=%q, 期望原容器字节（未经任何 ffmpeg）", body)
		}
	})

	t.Run("HEAD 不起实时流", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, "/bin/echo")
		src := writeSeekTestFile(t, "song.flac")
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "无损歌", FilePath: src, Format: "flac", Duration: 240})

		rr := playSeekRequestMethod(t, handler, id, "format=mp3", "HEAD")

		if body := rr.Body.String(); strings.Contains(body, "pipe:1") {
			t.Errorf("HEAD 起了实时流: %s", body)
		}
	})
}

// TestGetSongPlayTranscodeStreamsLiveRealFFmpeg 用**真** ffmpeg 端到端验证实时转码流
// （songloft-org/songloft#442）。
//
// 上面那些用 /bin/echo 的测试只能钉住参数串，钉不住「ffmpeg 真的接受这个组合」和
// 「客户端拿到的是一段可解码、没被截断的 mp3」——参数写错的典型表现恰恰是 ffmpeg 零输出后
// 静默降级，参数断言全绿而用户听不到声。
func TestGetSongPlayTranscodeStreamsLiveRealFFmpeg(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not available")
	}

	// 合成一首 5 秒的无损源：够短所以测试快，够长所以能验证「没被截断」。
	src := filepath.Join(t.TempDir(), "in.flac")
	gen := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=44100",
		"-ac", "2", "-c:a", "flac", "-y", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("合成测试音频失败: %v: %s", err, out)
	}

	// probe 把响应体落盘后用 ffprobe 读一个字段（duration / bit_rate）。
	probe := func(t *testing.T, body []byte, entry string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "got.mp3")
		if err := os.WriteFile(p, body, 0644); err != nil {
			t.Fatalf("write response body: %v", err)
		}
		out, err := exec.Command(ffprobePath, "-v", "error",
			"-show_entries", "format="+entry,
			"-of", "default=nw=1:nokey=1", p).Output()
		if err != nil {
			t.Fatalf("ffprobe %s 失败（响应可能不是有效 mp3，%d 字节）: %v", entry, len(body), err)
		}
		return strings.TrimSpace(string(out))
	}

	t.Run("format=mp3 产出完整可解码的 mp3", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, ffmpegPath)
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "无损歌", FilePath: src, Format: "flac", Duration: 5})

		rr := playSeekRequest(t, handler, id, "format=mp3")

		if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
			t.Fatalf("Content-Type=%q, 期望 audio/mpeg（未走实时流？）", ct)
		}
		dur, err := strconv.ParseFloat(probe(t, rr.Body.Bytes(), "duration"), 64)
		if err != nil {
			t.Fatalf("解析 duration: %v", err)
		}
		// 允许 ±0.5s：mp3 帧对齐与无 Xing 头的估算都会有小偏差，但截断会差一个量级。
		if dur < 4.5 || dur > 5.5 {
			t.Errorf("输出时长 %.2fs，期望约 5s（偏差过大说明流被截断）", dur)
		}
	})

	t.Run("quality=128 输出的实际码率就是 128k", func(t *testing.T) {
		handler, repo, _ := newSeekTestHandler(t, ffmpegPath)
		id := seedSong(t, repo, &models.Song{Type: models.TypeLocal, Title: "无损歌", FilePath: src, Format: "flac", Duration: 5})

		rr := playSeekRequest(t, handler, id, "quality=128")

		br, err := strconv.Atoi(probe(t, rr.Body.Bytes(), "bit_rate"))
		if err != nil {
			t.Fatalf("解析 bit_rate: %v", err)
		}
		// CBR 128k，容差 ±15% 覆盖容器开销；修复前这里会是 320k（pipe 固定码率）。
		if br < 110_000 || br > 148_000 {
			t.Errorf("实际码率 %d bps，期望约 128000（?quality= 未生效？）", br)
		}
	})
}

// TestGetSongPlayPrefetchWarmsNormalized 验证 ?prefetch=1&normalize=1 预热出来的确实是
// **均衡产物**（文件名带 norm. 标记）。
//
// 修复前 prepareSongPlayback 收不到 normalize：一是给 GetOrTranscode 硬编码 false，
// 二是 mp3 源 + format=mp3 时 NeedsTranscodeForServe 为 false 直接 return，预热一行没干，
// 真实播放仍要冷启动整首 loudnorm（songloft-org/songloft-plugin-miot#61）。
func TestGetSongPlayPrefetchWarmsNormalized(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	handler, repo, _ := newSeekTestHandler(t, ffmpegPath)

	src := filepath.Join(t.TempDir(), "tone.mp3")
	gen := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-codec:a", "libmp3lame", "-y", src)
	if out, genErr := gen.CombinedOutput(); genErr != nil {
		t.Skipf("generate sample mp3 failed: %v (%s)", genErr, out)
	}
	song := &models.Song{Type: models.TypeLocal, Title: "本地歌", FilePath: src, Format: "mp3", Duration: 3}
	id := seedSong(t, repo, song)

	rr := playSeekRequest(t, handler, id, "prefetch=1&normalize=1&format=mp3")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status=%d, 期望 202", rr.Code)
	}

	// 预热在 goroutine 里跑，轮询等产物落地
	cacheService := handler.cacheService
	var found bool
	for range 100 {
		if _, ok := cacheService.FindTranscodedFile(song, "mp3", 0, -1, true); ok {
			found = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !found {
		t.Fatal("预热没有产出均衡产物：真实播放仍会冷启动整首 loudnorm 并阻塞首个 play 请求")
	}
}

// TestListRandomSongs 验证 GET /songs/random handler：
//   - 返回完整歌曲对象
//   - limit 参数控制数量
//   - 默认 limit=50
//   - limit 上限 500
//   - 过滤条件生效
func TestListRandomSongs(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewSongHandler(songService, nil, nil, nil, nil, nil)

	// 创建 5 首歌曲
	for i := range 5 {
		seedSong(t, repo, &models.Song{
			Type:     models.TypeLocal,
			Title:    fmt.Sprintf("歌曲%d", i+1),
			FilePath: fmt.Sprintf("/music/%d.mp3", i+1),
		})
	}
	seedSong(t, repo, &models.Song{
		Type:  models.TypeRemote,
		Title: "网络歌曲",
		URL:   "https://example.com/remote.mp3",
	})

	// 1) 基本请求：返回 200 + 完整歌曲对象
	req := httptest.NewRequest("GET", "/api/v1/songs/random", nil)
	rr := httptest.NewRecorder()
	handler.ListRandomSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	songs, ok := resp["songs"].([]interface{})
	if !ok {
		t.Fatalf("songs field missing or not array: %v", resp)
	}
	// 默认 limit=50，池子只有 6 首 → 返回全部 6 首
	if len(songs) != 6 {
		t.Errorf("songs len = %d, want 6", len(songs))
	}
	if total, ok := resp["total"].(float64); !ok || int(total) != 6 {
		t.Errorf("total = %v, want 6", resp["total"])
	}

	// 2) 指定 limit：返回指定数量
	req = httptest.NewRequest("GET", "/api/v1/songs/random?limit=2", nil)
	rr = httptest.NewRecorder()
	handler.ListRandomSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	songs = resp["songs"].([]interface{})
	if len(songs) != 2 {
		t.Errorf("songs len = %d, want 2", len(songs))
	}

	// 3) 类型过滤：只随机 local
	req = httptest.NewRequest("GET", "/api/v1/songs/random?type=local&limit=10", nil)
	rr = httptest.NewRecorder()
	handler.ListRandomSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	songs = resp["songs"].([]interface{})
	if len(songs) != 5 {
		t.Errorf("songs len = %d, want 5 (all local)", len(songs))
	}
	if int(resp["total"].(float64)) != 5 {
		t.Errorf("total = %v, want 5", resp["total"])
	}

	// 4) limit 上限 500
	req = httptest.NewRequest("GET", "/api/v1/songs/random?limit=9999", nil)
	rr = httptest.NewRecorder()
	handler.ListRandomSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	songs = resp["songs"].([]interface{})
	if len(songs) != 6 {
		t.Errorf("songs len = %d, want 6 (capped at 500, pool has 6)", len(songs))
	}

	// 5) 无效 limit 参数回退默认值
	req = httptest.NewRequest("GET", "/api/v1/songs/random?limit=invalid", nil)
	rr = httptest.NewRecorder()
	handler.ListRandomSongs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}
