package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"

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
