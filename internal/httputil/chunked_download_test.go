package httputil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// rangeClient 模拟支持 Range 的 CDN(googlevideo 等):按请求的 Range 返回 206 分片。
func rangeClient(full []byte, requests *int) *http.Client {
	total := int64(len(full))
	return &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		*requests++
		var start, end int64
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
			return nil, fmt.Errorf("bad range: %v", err)
		}
		if end > total-1 {
			end = total - 1
		}
		chunk := full[start : end+1]
		h := make(http.Header)
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		return &http.Response{
			StatusCode:    http.StatusPartialContent,
			Body:          io.NopCloser(bytes.NewReader(chunk)),
			ContentLength: int64(len(chunk)),
			Header:        h,
		}, nil
	})}
}

// 200 客户端:忽略 Range,整段返回(模拟不支持 Range 的服务端)。
func okClient(body []byte, contentLength int64) *http.Client {
	return &http.Client{Transport: rtFunc(func(_ *http.Request) (*http.Response, error) {
		h := make(http.Header)
		h.Set("Content-Type", "audio/mpeg")
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: contentLength,
			Header:        h,
		}, nil
	})}
}

// 支持 Range → 分块顺序拉取,正确拼接完整文件,分块数符合预期。
func TestChunkedDownload_Ranged(t *testing.T) {
	full := bytes.Repeat([]byte("z"), int(DownloadChunkSize*2+DownloadChunkSize/2)) // 需 3 块
	requests := 0
	var buf bytes.Buffer

	n, hdr, err := ChunkedDownload(context.Background(), rangeClient(full, &requests), "http://x/a", nil, 30*time.Second, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != int64(len(full)) || buf.Len() != len(full) {
		t.Fatalf("size mismatch: n=%d bufLen=%d want=%d", n, buf.Len(), len(full))
	}
	if !bytes.Equal(buf.Bytes(), full) {
		t.Fatalf("content mismatch")
	}
	if requests != 3 {
		t.Fatalf("expected 3 range requests, got %d", requests)
	}
	if hdr == nil {
		t.Fatalf("expected first header to be captured")
	}
}

// 文件小于单块 → 一个请求即完成(末块短读终止)。
func TestChunkedDownload_SingleChunk(t *testing.T) {
	full := bytes.Repeat([]byte("z"), 1024)
	requests := 0
	var buf bytes.Buffer

	n, _, err := ChunkedDownload(context.Background(), rangeClient(full, &requests), "http://x/a", nil, 30*time.Second, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1024 || requests != 1 {
		t.Fatalf("expected 1024 bytes in 1 request, got n=%d requests=%d", n, requests)
	}
}

// 服务端忽略 Range(返回 200)→ 回退整段下载,首个响应头可取到 Content-Type。
func TestChunkedDownload_Status200Fallback(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 500)
	var buf bytes.Buffer

	n, hdr, err := ChunkedDownload(context.Background(), okClient(body, 500), "http://x/a", nil, 30*time.Second, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 500 {
		t.Fatalf("expected 500 bytes, got %d", n)
	}
	if hdr.Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("expected Content-Type from first response, got %q", hdr.Get("Content-Type"))
	}
}

// 200 且 Content-Length 声明长于实际 → 判为截断报错。
func TestChunkedDownload_Truncated(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 500)
	var buf bytes.Buffer

	_, _, err := ChunkedDownload(context.Background(), okClient(body, 1000), "http://x/a", nil, 30*time.Second, &buf)
	if err == nil {
		t.Fatalf("expected truncation error")
	}
}

func TestParseContentRangeTotal(t *testing.T) {
	cases := map[string]int64{
		"bytes 0-9999/12345": 12345,
		"bytes 0-9999/*":     0,
		"":                   0,
		"garbage":            0,
		"bytes 0-9999/-5":    0,
	}
	for in, want := range cases {
		if got := parseContentRangeTotal(in); got != want {
			t.Errorf("parseContentRangeTotal(%q) = %d, want %d", in, got, want)
		}
	}
}
