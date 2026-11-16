package jsplugin

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLynxBundleCompression(t *testing.T) {
	root := t.TempDir()
	body := bytes.Repeat([]byte("LYNX bundle payload with repeated snapshots\x00"), 4096)
	if err := os.WriteFile(filepath.Join(root, "main.web.bundle"), body, 0644); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{}
	for _, tc := range []struct {
		name, method, accept, byteRange string
		compressed                      bool
		status                          int
	}{
		{"gzip full bundle", http.MethodGet, "gzip", "", true, 200},
		{"identity full bundle", http.MethodGet, "", "", false, 200},
		{"range remains byte exact", http.MethodGet, "gzip", "bytes=0-15", false, 206},
		{"head keeps original length", http.MethodHead, "gzip", "", false, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/v1/jsplugin/miot/static/main.web.bundle", nil)
			req.Header.Set("Accept-Encoding", tc.accept)
			req.Header.Set("Range", tc.byteRange)
			rec := httptest.NewRecorder()
			if !manager.tryServeStaticFile(rec, req, root, "main.web.bundle", "miot") {
				t.Fatal("bundle not served")
			}
			if rec.Code != tc.status {
				t.Fatalf("status=%d", rec.Code)
			}
			if got := rec.Header().Get("Content-Encoding"); (got == "gzip") != tc.compressed {
				t.Fatalf("Content-Encoding=%q, want gzip=%v", got, tc.compressed)
			}
			got := rec.Body.Bytes()
			if tc.compressed {
				if len(got) >= len(body)/2 {
					t.Fatalf("gzip did not reduce the payload: %d", len(got))
				}
				reader, err := gzip.NewReader(bytes.NewReader(got))
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				got, err = io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
					t.Fatal("missing Vary")
				}
			}
			want := body
			if tc.byteRange != "" {
				want = body[:16]
				if rec.Header().Get("Content-Range") != "bytes 0-15/"+strconv.Itoa(len(body)) {
					t.Fatalf("unexpected Content-Range: %q", rec.Header().Get("Content-Range"))
				}
			}
			if tc.method == http.MethodHead {
				want = nil
				if rec.Header().Get("Content-Length") != strconv.Itoa(len(body)) {
					t.Fatalf("HEAD length=%q", rec.Header().Get("Content-Length"))
				}
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("payload differs: %d vs %d", len(got), len(want))
			}
			cached := httptest.NewRequest(http.MethodGet, req.URL.String(), nil)
			cached.Header.Set("If-Modified-Since", rec.Header().Get("Last-Modified"))
			cached.Header.Set("Accept-Encoding", "gzip")
			again := httptest.NewRecorder()
			manager.tryServeStaticFile(again, cached, root, "main.web.bundle", "miot")
			if again.Code != 304 || again.Body.Len() != 0 {
				t.Fatalf("conditional response=%d body=%d", again.Code, again.Body.Len())
			}
		})
	}
}
