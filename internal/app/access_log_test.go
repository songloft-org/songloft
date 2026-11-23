package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chi_middleware "github.com/go-chi/chi/v5/middleware"
	"songloft/internal/config"
	"songloft/internal/database/testutil"
	"songloft/internal/handlers"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestRadioStartAndAccessLogsShareRequestID(t *testing.T) {
	repo := testutil.OpenMemoryDB(t).SongRepository()
	song := &models.Song{Title: "radio", Type: models.TypeRadio, RadioMode: models.RadioModeHLS, URL: "https://example.com/live.m3u8"}
	if err := repo.Create(t.Context(), song); err != nil {
		t.Fatal(err)
	}
	app := &App{router: chi.NewRouter(), config: &config.AppConfig{}}
	app.setupBaseRouter()
	h := handlers.NewSongHandler(services.NewSongService(repo, nil, nil, nil, nil, nil), nil, nil, nil, nil, nil)
	app.router.Get("/api/v1/songs/{id}/play.m3u8", h.GetSongPlay)
	app.router.Head("/api/v1/songs/{id}/play.m3u8", h.GetSongPlay)

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		app.router.ServeHTTP(w, httptest.NewRequest(method, song.PlaybackURL()+"?access_token=fixture-secret", nil))
		if w.Code != http.StatusFound {
			t.Fatalf("%s status = %d", method, w.Code)
		}
	}
	if strings.Contains(logs.String(), "fixture-secret") {
		t.Fatal("radio logging includes the query token")
	}
	var entries []map[string]any
	decoder := json.NewDecoder(&logs)
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 3 || entries[0]["msg"] != "radio stream requested" || entries[1]["msg"] != "access" || entries[2]["method"] != "HEAD" {
		t.Fatalf("unexpected start and end logs: %+v", entries)
	}
	if id, ok := entries[0]["request_id"].(string); !ok || id == "" || id != entries[1]["request_id"] {
		t.Fatalf("GET start and finish need the same request ID: %+v", entries)
	}
}

func TestAccessLogRangeResponse(t *testing.T) {
	for _, tt := range []struct {
		name         string
		rangeHeader  string
		shortBody    bool
		status       int
		writtenBytes int
		contentRange string
	}{
		{name: "full file", status: http.StatusOK, writtenBytes: 10},
		{name: "partial file", rangeHeader: "bytes=2-5", status: http.StatusPartialContent, writtenBytes: 4, contentRange: "bytes 2-5/10"},
		{name: "unsatisfiable range", rangeHeader: "bytes=10-", status: http.StatusRequestedRangeNotSatisfiable, writtenBytes: len("invalid range: failed to overlap\n"), contentRange: "bytes */10"},
		{name: "short body", shortBody: true, status: http.StatusOK, writtenBytes: 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1/play?access_token=private-test-token", nil)
			if tt.rangeHeader != "" {
				req.Header.Set("Range", tt.rangeHeader)
			}
			w := httptest.NewRecorder()
			chi_middleware.RequestLogger(slogLogFormatter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.shortBody {
					w.Header().Set("Content-Length", "10")
					_, _ = w.Write([]byte("0123"))
					return
				}
				http.ServeContent(w, r, "song.mp3", time.Time{}, strings.NewReader("0123456789"))
			})).ServeHTTP(w, req)

			raw := logs.String()
			var entry map[string]any
			decoder := json.NewDecoder(strings.NewReader(raw))
			for entry["msg"] != "access" {
				if err := decoder.Decode(&entry); err != nil {
					t.Fatalf("decode access log: %v", err)
				}
			}
			for field, want := range map[string]any{
				"msg": "access", "path": "/api/v1/songs/1/play", "method": "GET",
				"status": float64(tt.status), "bytes": float64(tt.writtenBytes),
			} {
				if got := entry[field]; got != want {
					t.Errorf("%s = %v, want %v", field, got, want)
				}
			}
			for field, want := range map[string]string{
				"range": tt.rangeHeader, "content_range": tt.contentRange,
				"content_length": w.Header().Get("Content-Length"),
			} {
				got, present := entry[field]
				if want == "" {
					if present {
						t.Errorf("empty %s must be omitted, got %v", field, got)
					}
				} else if got != want {
					t.Errorf("%s = %v, want %v", field, got, want)
				}
			}
			if strings.Contains(raw, "private-test-token") || strings.Contains(raw, "access_token") {
				t.Fatal("access log exposes query credentials")
			}
		})
	}
}
