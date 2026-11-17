package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"songloft/internal/services"
)

func TestScanRejectsExcludedRootsWithoutStarting(t *testing.T) {
	root := t.TempDir()
	for _, paths := range [][]string{nil, {filepath.Join(root, "album")}} {
		scanner := services.NewScanner(&services.ScanConfig{MusicPath: root, ExcludePaths: []string{root}})
		service := services.NewSongService(newTestSongRepo(t), nil, nil, scanner, nil, nil)
		handler := NewScanHandler(service, scanner, nil)
		body, _ := json.Marshal(ScanRequest{Paths: paths})
		rr := httptest.NewRecorder()
		handler.ScanAndImport(rr, httptest.NewRequest("POST", "/api/v1/scan", strings.NewReader(string(body))))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "排除") {
			t.Fatalf("response: %d %s", rr.Code, rr.Body.String())
		}
		if service.GetScanProgress().Status != services.ScanStatusIdle {
			t.Fatal("rejected scan started background work")
		}
	}
}

func TestValidateScanReportsMixedRootsAndRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	scanner := services.NewScanner(&services.ScanConfig{MusicPath: root, ExcludePaths: []string{filepath.Join(root, "yt-dlp")}})
	handler := NewScanHandler(nil, scanner, nil)
	paths := []string{filepath.Join(root, "yt-dlp", "album"), filepath.Join(root, "allowed")}
	body, _ := json.Marshal(ScanRequest{Paths: paths})
	rr := httptest.NewRecorder()
	handler.ValidateScan(rr, httptest.NewRequest("POST", "/scan/validate", strings.NewReader(string(body))))
	var validation services.ScanValidation
	if err := json.Unmarshal(rr.Body.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK || len(validation.Paths) != 1 || validation.Paths[0] != paths[1] || len(validation.ExcludedDirectories) != 1 || validation.ExcludedDirectories[0].Path != paths[0] {
		t.Fatalf("response: %d %s", rr.Code, rr.Body.String())
	}
	for _, body := range []string{`{"paths":["/etc"]}`, `{"paths":[" "]}`, `{`} {
		rr = httptest.NewRecorder()
		handler.ValidateScan(rr, httptest.NewRequest("POST", "/scan/validate", strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, rr.Code)
		}
	}
}
