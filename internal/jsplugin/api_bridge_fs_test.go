package jsplugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func newTestBridgeHandler(t *testing.T, permissions []string, entryPath string) *BridgeHandler {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	plugin := &models.JSPlugin{
		EntryPath:   entryPath,
		Permissions: permissions,
	}
	svc := &JSService{plugin: plugin}
	dataDir := t.TempDir()
	return NewBridgeHandler(svc, dataDir, db, nil, nil, nil, nil, nil, "", "", "")
}

func TestResolveFSPath_EmptyPath(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFS}, "test-plugin")
	_, err := h.resolveFSPath("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestResolveFSPath_PathTraversal(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFS}, "test-plugin")
	_, err := h.resolveFSPath("../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestResolveFSPath_RelativePath(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFS}, "test-plugin")
	got, err := h.resolveFSPath("data/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(h.pluginDataDir(), "data", "config.json")
	abs, _ := filepath.Abs(expected)
	if got != abs {
		t.Fatalf("got %q, want %q", got, abs)
	}
}

func TestResolveFSPath_RelativePath_NoPermission(t *testing.T) {
	h := newTestBridgeHandler(t, []string{}, "test-plugin")
	_, err := h.resolveFSPath("data/config.json")
	if err == nil {
		t.Fatal("expected error for missing fs permission")
	}
}

func TestResolveFSPath_MusicPath(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFSMusic}, "test-plugin")
	musicDir := t.TempDir()
	err := h.db.ConfigRepository().Set(context.Background(), &models.Config{
		Key:   "music_path",
		Value: `{"path":"` + musicDir + `"}`,
	})
	if err != nil {
		t.Fatalf("set music_path: %v", err)
	}

	got, err := h.resolveFSPath("music://artist/song.mp3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := filepath.Abs(filepath.Join(musicDir, "artist", "song.mp3"))
	if got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
}

func TestResolveFSPath_MusicPath_NoPermission(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFS}, "test-plugin")
	_, err := h.resolveFSPath("music://artist/song.mp3")
	if err == nil {
		t.Fatal("expected error for missing fs:music permission")
	}
}

func TestResolveFSPath_MusicPath_NotConfigured(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFSMusic}, "test-plugin")
	_ = h.db.ConfigRepository().Delete(context.Background(), "music_path")
	_, err := h.resolveFSPath("music://artist/song.mp3")
	if err == nil {
		t.Fatal("expected error for unconfigured music_path")
	}
}

func TestResolveFSPath_AbsolutePath(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFSExternal}, "test-plugin")
	extDir := t.TempDir()
	h.service.plugin.ExternalPaths = []string{extDir}

	target := filepath.Join(extDir, "subdir", "file.txt")
	got, err := h.resolveFSPath(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := filepath.Abs(target)
	if got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
}

func TestResolveFSPath_AbsolutePath_DirItself(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFSExternal}, "test-plugin")
	extDir := t.TempDir()
	h.service.plugin.ExternalPaths = []string{extDir}

	got, err := h.resolveFSPath(extDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := filepath.Abs(extDir)
	if got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
}

func TestResolveFSPath_AbsolutePath_NoPermission(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFS}, "test-plugin")
	_, err := h.resolveFSPath("/some/external/path")
	if err == nil {
		t.Fatal("expected error for missing fs:external permission")
	}
}

func TestResolveFSPath_AbsolutePath_NotInAllowed(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFSExternal}, "test-plugin")
	extDir := t.TempDir()
	h.service.plugin.ExternalPaths = []string{extDir}

	_, err := h.resolveFSPath("/not/in/allowed/dir")
	if err == nil {
		t.Fatal("expected error for path not in allowed dirs")
	}
}

func TestResolveFSPath_AbsolutePath_EscapeAllowed(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermFSExternal}, "test-plugin")
	extDir := t.TempDir()
	subDir := filepath.Join(extDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h.service.plugin.ExternalPaths = []string{subDir}

	_, err := h.resolveFSPath(extDir)
	if err == nil {
		t.Fatal("expected error for path escaping allowed dir")
	}
}
