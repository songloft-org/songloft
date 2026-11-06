package mobile

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartWithUnavailableSystemTemp(t *testing.T) {
	dataDir := t.TempDir()
	musicDir := t.TempDir()
	// Android's default /data/local/tmp is not writable by an ordinary app.
	// A file used as TMPDIR reproduces the unavailable directory even as root.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", blocked)
	t.Cleanup(Stop)
	client := &http.Client{Timeout: 5 * time.Second}
	for attempt := 0; attempt < 2; attempt++ {
		port, err := Start(dataDir, musicDir, 0)
		if err != nil {
			t.Fatalf("start %d: %v", attempt, err)
		}
		if !IsRunning() || GetPort() != port || port <= 0 {
			t.Fatalf("invalid running state: port=%d, GetPort=%d", port, GetPort())
		}
		if got := os.TempDir(); got != filepath.Join(dataDir, "tmp") {
			t.Fatalf("temporary directory = %q", got)
		}
		f, err := os.CreateTemp("", "mobile-test-")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		os.Remove(f.Name())
		roots, err := filepath.Glob(filepath.Join(dataDir, "tmp", "songloft-hls-*"))
		if err != nil || len(roots) != 1 {
			t.Fatalf("HLS roots = %v, error = %v", roots, err)
		}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", port))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("health status = %d", resp.StatusCode)
		}
		Stop()
		if IsRunning() || GetPort() != 0 {
			t.Fatal("backend still running after Stop")
		}
		if _, err := os.Stat(roots[0]); !os.IsNotExist(err) {
			t.Fatalf("HLS temporary root remains after Stop: %v", err)
		}
	}
}

func TestStartRejectsUnavailableSandboxTemp(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "tmp"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", os.TempDir())
	t.Cleanup(Stop)
	if port, err := Start(dataDir, "", 0); err == nil || port != -1 {
		t.Fatalf("Start = (%d, %v), want (-1, error)", port, err)
	}
	if IsRunning() {
		t.Fatal("backend running after failed temp initialization")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "songloft.db")); !os.IsNotExist(err) {
		t.Fatalf("database initialized before temp directory validation: %v", err)
	}
}
