package httputil

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// HEAD 支持的服务器：走 HEAD 路径拿大小，GET 不应被触发（行为与旧版完全一致）。
func TestFetchContentLengthHeadSupported(t *testing.T) {
	getCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Content-Length", "1234")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			getCalled = true
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer srv.Close()

	size, err := fetchContentLength(srv.Client(), srv.URL, nil)
	if err != nil {
		t.Fatalf("fetchContentLength() error = %v", err)
	}
	if size != 1234 {
		t.Fatalf("size = %d, want 1234", size)
	}
	if getCalled {
		t.Fatal("GET should not be called when HEAD works")
	}
}

// HEAD 404（仅注册 GET/POST）→ 回退 Range 0-0 →
// 206 + Content-Range 解析总大小。
func TestFetchContentLengthHead404FallsBackToRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.Header.Get("Range") == "bytes=0-0":
			w.Header().Set("Content-Range", "bytes 0-0/61520341")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("x"))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer srv.Close()

	size, err := fetchContentLength(srv.Client(), srv.URL, nil)
	if err != nil {
		t.Fatalf("fetchContentLength() error = %v", err)
	}
	if size != 61520341 {
		t.Fatalf("size = %d, want 61520341", size)
	}
}

// HEAD 405 + 416（Content-Range 以「bytes */total」回告总大小）。
func TestFetchContentLengthFallsBackTo416Total(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Range", "bytes */777")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		}
	}))
	defer srv.Close()

	size, err := fetchContentLength(srv.Client(), srv.URL, nil)
	if err != nil {
		t.Fatalf("fetchContentLength() error = %v", err)
	}
	if size != 777 {
		t.Fatalf("size = %d, want 777", size)
	}
}

// 服务器忽略 Range 返回 200 全量 → 直接取 Content-Length。
func TestFetchContentLengthFallsBackWhenRangeIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodGet:
			w.Header().Set("Content-Length", "999")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	size, err := fetchContentLength(srv.Client(), srv.URL, nil)
	if err != nil {
		t.Fatalf("fetchContentLength() error = %v", err)
	}
	if size != 999 {
		t.Fatalf("size = %d, want 999", size)
	}
}

// 两条探测路径都失败：错误应上抛（与旧行为一致）。
func TestFetchContentLengthBothProbesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := fetchContentLength(srv.Client(), srv.URL, nil); err == nil {
		t.Fatal("expected error when both HEAD and range probe fail")
	}
}

// 端到端：对「无 HEAD 路由、支持 Range」的服务器，HTTPReadSeeker
// 应回退成功并正确读出头部/尾部数据。
func TestHTTPReadSeekerWorksWithoutHead(t *testing.T) {
	payload := bytes.Repeat([]byte{0}, 300*1024)
	payload[0], payload[1], payload[2] = 'A', 'B', 'C'
	payload[len(payload)-1] = 'Z'

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound) // 模拟 无 HEAD 路由
			return
		}
		http.ServeContent(w, r, "audio.mp3", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()

	rs, err := NewHTTPReadSeekerWithHeaders(srv.Client(), srv.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPReadSeekerWithHeaders() error = %v", err)
	}
	if got := rs.Size(); got != int64(len(payload)) {
		t.Fatalf("Size() = %d, want %d", got, len(payload))
	}

	head := make([]byte, 3)
	if _, err := io.ReadFull(rs, head); err != nil {
		t.Fatalf("read head: %v", err)
	}
	if string(head) != "ABC" {
		t.Fatalf("head = %q, want %q", head, "ABC")
	}

	if _, err := rs.Seek(-1, io.SeekEnd); err != nil {
		t.Fatalf("seek: %v", err)
	}
	tail := make([]byte, 1)
	if _, err := io.ReadFull(rs, tail); err != nil {
		t.Fatalf("read tail: %v", err)
	}
	if tail[0] != 'Z' {
		t.Fatalf("tail = %q, want %q", tail, "Z")
	}
}