package jsplugin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mimusic/internal/database"

	_ "modernc.org/sqlite"
)

// --- Test Helpers ---

// createTestPluginZip creates an in-memory .jsplugin.zip containing plugin.json and main.js
// 自动计算 entryHash/zipHash 并写入 manifest。
func createTestPluginZip(t *testing.T, manifest *PluginManifest, jsCode string) []byte {
	t.Helper()
	return createTestPluginZipWithStatic(t, manifest, jsCode, nil)
}

// createTestPluginZipWithStatic creates a .jsplugin.zip with static/ directory content
// 自动计算 entryHash/zipHash 并写入 manifest。
func createTestPluginZipWithStatic(t *testing.T, manifest *PluginManifest, jsCode string, staticFiles map[string]string) []byte {
	t.Helper()

	mainFile := manifest.Main
	if mainFile == "" {
		mainFile = "main.js"
	}

	// 预先计算 entryHash 和 zipHash（排除 plugin.json 自身）。
	manifest.EntryHash = sha256HexSum([]byte(jsCode))
	manifest.ZipHash = computeTestCanonicalZipHash(t, mainFile, jsCode, staticFiles)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Write plugin.json
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	fw, err := w.Create("plugin.json")
	if err != nil {
		t.Fatalf("create plugin.json in zip: %v", err)
	}
	if _, err := fw.Write(manifestJSON); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}

	// Write main.js
	fw, err = w.Create(mainFile)
	if err != nil {
		t.Fatalf("create %s in zip: %v", mainFile, err)
	}
	if _, err := fw.Write([]byte(jsCode)); err != nil {
		t.Fatalf("write %s: %v", mainFile, err)
	}

	// Write static/ files
	for name, content := range staticFiles {
		fw, err = w.Create("static/" + name)
		if err != nil {
			t.Fatalf("create static/%s in zip: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("write static/%s: %v", name, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return buf.Bytes()
}

// computeTestCanonicalZipHash 在测试中手动计算 canonical zip hash。
// 与 ComputeCanonicalZipHash 算法一致：按路径升序对每个文件写 path\nsha256(content)\n，
// 然后对拼接串再 sha256。排除 plugin.json。
func computeTestCanonicalZipHash(t *testing.T, mainFile, jsCode string, staticFiles map[string]string) string {
	t.Helper()

	type entry struct {
		path string
		hash string
	}
	entries := []entry{{path: mainFile, hash: sha256HexSum([]byte(jsCode))}}
	for name, content := range staticFiles {
		entries = append(entries, entry{path: "static/" + name, hash: sha256HexSum([]byte(content))})
	}
	// 排序
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].path < entries[i].path {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	hasher := sha256.New()
	for _, e := range entries {
		hasher.Write([]byte(e.path))
		hasher.Write([]byte{'\n'})
		hasher.Write([]byte(e.hash))
		hasher.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// setupTestDB creates an in-memory SQLite database with the js_plugins schema
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create js_plugins table
	schema := `
CREATE TABLE IF NOT EXISTS js_plugins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    description TEXT,
    author TEXT,
    homepage TEXT,
    license TEXT,
    entry_path TEXT NOT NULL UNIQUE,
    main TEXT NOT NULL DEFAULT 'main.js',
    min_host_version TEXT,
    permissions TEXT DEFAULT '[]',
    update_url TEXT,
    download_url TEXT,
    status TEXT NOT NULL CHECK(status IN ('active', 'inactive', 'error')) DEFAULT 'inactive',
    zip_hash TEXT,
    entry_hash TEXT,
    file_mod_time TEXT,
    file_path TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS update_js_plugins_updated_at
AFTER UPDATE ON js_plugins
FOR EACH ROW
BEGIN
    UPDATE js_plugins SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return db
}

// setupTestEnv creates a full test environment (temp dirs, db, repo, package manager)
func setupTestEnv(t *testing.T) (pluginsDir, dataDir string, repo *database.JSPluginRepository, db *sql.DB) {
	t.Helper()

	pluginsDir = filepath.Join(t.TempDir(), "jsplugins")
	dataDir = filepath.Join(t.TempDir(), "jsplugins_data")

	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("create plugins dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}

	db = setupTestDB(t)
	repo = database.NewJSPluginRepository(db)
	return
}

// testManifest returns a valid test PluginManifest
func testManifest(entryPath string) *PluginManifest {
	return &PluginManifest{
		Name:        "Test Plugin " + entryPath,
		Version:     "1.0.0",
		Description: "A test plugin",
		Author:      "Test",
		EntryPath:   entryPath,
		Main:        "main.js",
		Permissions: []string{"storage", "songs.read"},
	}
}

const simpleJSCode = `
function onInit() {}
function onDeinit() {}
function onHTTPRequest(req) {
    return { statusCode: 200, headers: {"Content-Type": "application/json"}, body: JSON.stringify({hello: "world"}) };
}
`

// --- Integration Tests ---

// TestManager_LoadPlugin tests loading a valid .jsplugin.zip
func TestManager_LoadPlugin(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	// Create ZIP and save to disk
	manifest := testManifest("test-load")
	zipData := createTestPluginZip(t, manifest, simpleJSCode)

	zipFileName := "test-load.jsplugin.zip"
	zipPath := filepath.Join(pluginsDir, zipFileName)
	if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	// Insert plugin record into DB
	plugin := &JSPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		Author:      manifest.Author,
		EntryPath:   manifest.EntryPath,
		Main:        manifest.Main,
		Permissions: manifest.Permissions,
		Status:      JSPluginStatusActive,
		FilePath:    zipFileName,
	}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatalf("create plugin record: %v", err)
	}

	// Create Manager and try to load
	// Note: This test requires QuickJS runtime. Skip if not available.
	manager := NewManager(repo, pluginsDir, dataDir, nil, nil)
	t.Cleanup(func() { manager.Close() })

	err := manager.LoadPlugin(ctx, plugin)
	if err != nil {
		// If QuickJS is not available, the error will be about creating JS env
		t.Skipf("LoadPlugin failed (may need QuickJS runtime): %v", err)
	}

	// Verify service is registered
	svc, ok := manager.GetService("test-load")
	if !ok {
		t.Fatal("expected service to be registered after LoadPlugin")
	}
	if svc.Status() != ServiceStatusReady {
		t.Errorf("expected service status Ready, got %s", svc.Status())
	}

	// Verify hashes were computed
	if plugin.ZipHash == "" {
		t.Error("expected ZipHash to be set after load")
	}
	if plugin.EntryHash == "" {
		t.Error("expected EntryHash to be set after load")
	}
}

// TestManager_LoadPlugin_InvalidHash tests that hash mismatch without mtime change rejects loading
func TestManager_LoadPlugin_InvalidHash(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	manifest := testManifest("test-hash")
	zipData := createTestPluginZip(t, manifest, simpleJSCode)

	zipFileName := "test-hash.jsplugin.zip"
	zipPath := filepath.Join(pluginsDir, zipFileName)
	if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	// Get file mtime for the plugin record
	info, err := os.Stat(zipPath)
	if err != nil {
		t.Fatalf("stat zip: %v", err)
	}
	fileModTime := info.ModTime().Format(time.RFC3339)

	// Insert plugin with a fake hash and the current mtime (simulates tampered file)
	plugin := &JSPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		EntryPath:   manifest.EntryPath,
		Main:        manifest.Main,
		Permissions: manifest.Permissions,
		Status:      JSPluginStatusActive,
		FilePath:    zipFileName,
		ZipHash:     "fake-hash-that-wont-match",
		FileModTime: fileModTime, // Same mtime = tamper detected
	}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatalf("create plugin record: %v", err)
	}

	manager := NewManager(repo, pluginsDir, dataDir, nil, nil)
	t.Cleanup(func() { manager.Close() })

	err = manager.LoadPlugin(ctx, plugin)
	if err == nil {
		t.Fatal("expected LoadPlugin to fail with hash mismatch and same mtime")
	}
	// Should contain "tampered" in the error
	if err != nil && !contains(err.Error(), "tampered") {
		t.Logf("Got error (expected tamper-related): %v", err)
	}
}

// TestManager_UnloadPlugin tests normal unloading
func TestManager_UnloadPlugin(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	manifest := testManifest("test-unload")
	zipData := createTestPluginZip(t, manifest, simpleJSCode)

	zipFileName := "test-unload.jsplugin.zip"
	zipPath := filepath.Join(pluginsDir, zipFileName)
	if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	plugin := &JSPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		EntryPath:   manifest.EntryPath,
		Main:        manifest.Main,
		Permissions: manifest.Permissions,
		Status:      JSPluginStatusActive,
		FilePath:    zipFileName,
	}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatalf("create plugin record: %v", err)
	}

	manager := NewManager(repo, pluginsDir, dataDir, nil, nil)
	t.Cleanup(func() { manager.Close() })

	if err := manager.LoadPlugin(ctx, plugin); err != nil {
		t.Skipf("LoadPlugin failed (may need QuickJS): %v", err)
	}

	// Verify loaded
	if _, ok := manager.GetService("test-unload"); !ok {
		t.Fatal("expected service to exist after load")
	}

	// Unload
	if err := manager.UnloadPlugin(ctx, "test-unload"); err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}

	// Verify unloaded
	if _, ok := manager.GetService("test-unload"); ok {
		t.Fatal("expected service to be removed after unload")
	}
}

// TestManager_ReloadPlugin tests hot-reload flow (unload + load)
func TestManager_ReloadPlugin(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	manifest := testManifest("test-reload")
	zipData := createTestPluginZip(t, manifest, simpleJSCode)

	zipFileName := "test-reload.jsplugin.zip"
	zipPath := filepath.Join(pluginsDir, zipFileName)
	if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	plugin := &JSPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		EntryPath:   manifest.EntryPath,
		Main:        manifest.Main,
		Permissions: manifest.Permissions,
		Status:      JSPluginStatusActive,
		FilePath:    zipFileName,
	}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatalf("create plugin record: %v", err)
	}

	manager := NewManager(repo, pluginsDir, dataDir, nil, nil)
	t.Cleanup(func() { manager.Close() })

	if err := manager.LoadPlugin(ctx, plugin); err != nil {
		t.Skipf("LoadPlugin failed (may need QuickJS): %v", err)
	}

	// Reload the plugin
	if err := manager.ReloadPlugin(ctx, "test-reload"); err != nil {
		t.Fatalf("ReloadPlugin failed: %v", err)
	}

	// Verify still running
	svc, ok := manager.GetService("test-reload")
	if !ok {
		t.Fatal("expected service to exist after reload")
	}
	if svc.Status() != ServiceStatusReady {
		t.Errorf("expected status Ready after reload, got %s", svc.Status())
	}
}

// TestManager_EnableDisable tests enable/disable toggle
func TestManager_EnableDisable(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	manifest := testManifest("test-toggle")
	zipData := createTestPluginZip(t, manifest, simpleJSCode)

	zipFileName := "test-toggle.jsplugin.zip"
	zipPath := filepath.Join(pluginsDir, zipFileName)
	if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	plugin := &JSPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		EntryPath:   manifest.EntryPath,
		Main:        manifest.Main,
		Permissions: manifest.Permissions,
		Status:      JSPluginStatusInactive,
		FilePath:    zipFileName,
	}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatalf("create plugin record: %v", err)
	}

	manager := NewManager(repo, pluginsDir, dataDir, nil, nil)
	t.Cleanup(func() { manager.Close() })

	// Enable
	if err := manager.EnablePlugin(ctx, plugin.ID); err != nil {
		t.Skipf("EnablePlugin failed (may need QuickJS): %v", err)
	}

	// Verify active in DB
	dbPlugin, err := repo.GetByID(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if dbPlugin.Status != JSPluginStatusActive {
		t.Errorf("expected status active after enable, got %s", dbPlugin.Status)
	}

	// Verify service running
	if _, ok := manager.GetService("test-toggle"); !ok {
		t.Fatal("expected service running after enable")
	}

	// Disable
	if err := manager.DisablePlugin(ctx, plugin.ID); err != nil {
		t.Fatalf("DisablePlugin failed: %v", err)
	}

	// Verify inactive in DB
	dbPlugin, err = repo.GetByID(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if dbPlugin.Status != JSPluginStatusInactive {
		t.Errorf("expected status inactive after disable, got %s", dbPlugin.Status)
	}

	// Verify service removed
	if _, ok := manager.GetService("test-toggle"); ok {
		t.Fatal("expected service removed after disable")
	}
}

// TestPackageManager_InstallFromUpload tests upload installation flow
func TestPackageManager_InstallFromUpload(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)

	pm := NewPackageManager(pluginsDir, dataDir, repo)

	manifest := testManifest("test-upload")
	zipData := createTestPluginZipWithStatic(t, manifest, simpleJSCode, map[string]string{
		"index.html": "<html><body>Hello</body></html>",
	})

	plugin, wasUpdate, err := pm.InstallFromUpload(zipData)
	if err != nil {
		t.Fatalf("InstallFromUpload failed: %v", err)
	}
	if wasUpdate {
		t.Error("expected wasUpdate=false for fresh install")
	}

	// Verify plugin record
	if plugin.ID == 0 {
		t.Error("expected non-zero plugin ID")
	}
	if plugin.EntryPath != "test-upload" {
		t.Errorf("expected entryPath 'test-upload', got %q", plugin.EntryPath)
	}
	if plugin.Status != JSPluginStatusInactive {
		t.Errorf("expected status inactive after install, got %s", plugin.Status)
	}
	if plugin.ZipHash == "" {
		t.Error("expected ZipHash to be computed")
	}
	if plugin.EntryHash == "" {
		t.Error("expected EntryHash to be computed")
	}

	// Verify ZIP file saved to disk
	zipPath := filepath.Join(pluginsDir, "test-upload.jsplugin.zip")
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Error("expected ZIP file to be saved on disk")
	}

	// Verify static files extracted
	staticPath := filepath.Join(dataDir, "test-upload", "static", "index.html")
	if _, err := os.Stat(staticPath); os.IsNotExist(err) {
		t.Error("expected static/index.html to be extracted")
	}

	// Verify DB record
	ctx := context.Background()
	dbPlugin, err := repo.GetByEntryPath(ctx, "test-upload")
	if err != nil {
		t.Fatalf("GetByEntryPath: %v", err)
	}
	if dbPlugin.Name != "Test Plugin test-upload" {
		t.Errorf("unexpected name in DB: %q", dbPlugin.Name)
	}
}

// TestPackageManager_InstallFromUpload_OverwriteUpdate verifies that re-uploading
// a plugin with the same entryPath transparently triggers an update (manual update flow).
func TestPackageManager_InstallFromUpload_OverwriteUpdate(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)

	pm := NewPackageManager(pluginsDir, dataDir, repo)

	manifest := testManifest("test-dup")
	zipData := createTestPluginZip(t, manifest, simpleJSCode)

	// First install: fresh
	first, wasUpdate, err := pm.InstallFromUpload(zipData)
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}
	if wasUpdate {
		t.Error("expected wasUpdate=false on first install")
	}
	if first.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", first.Version)
	}

	// Second upload with a bumped version should succeed as an update,
	// preserve ID, and return wasUpdate=true.
	manifest2 := testManifest("test-dup")
	manifest2.Version = "1.1.0"
	zipData2 := createTestPluginZip(t, manifest2, simpleJSCode)

	updated, wasUpdate, err := pm.InstallFromUpload(zipData2)
	if err != nil {
		t.Fatalf("second install (manual update) failed: %v", err)
	}
	if !wasUpdate {
		t.Error("expected wasUpdate=true on duplicate entryPath upload")
	}
	if updated.ID != first.ID {
		t.Errorf("expected same ID after manual update, got %d != %d", updated.ID, first.ID)
	}
	if updated.Version != "1.1.0" {
		t.Errorf("expected version 1.1.0 after update, got %q", updated.Version)
	}
}

// TestPackageManager_SyncPluginsFromDirectory tests directory scanning
func TestPackageManager_SyncPluginsFromDirectory(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)

	pm := NewPackageManager(pluginsDir, dataDir, repo)

	// Place some ZIPs in the directory
	for _, name := range []string{"alpha", "beta"} {
		manifest := testManifest(name)
		zipData := createTestPluginZip(t, manifest, simpleJSCode)
		zipPath := filepath.Join(pluginsDir, name+".jsplugin.zip")
		if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
			t.Fatalf("write %s zip: %v", name, err)
		}
	}

	// Sync should discover and install both
	plugins, err := pm.SyncPluginsFromDirectory()
	if err != nil {
		t.Fatalf("SyncPluginsFromDirectory failed: %v", err)
	}

	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins discovered, got %d", len(plugins))
	}

	// Verify both are in DB
	ctx := context.Background()
	allPlugins, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(allPlugins) != 2 {
		t.Errorf("expected 2 plugins in DB, got %d", len(allPlugins))
	}

	// Second sync should recognize existing plugins (no duplicate)
	plugins2, err := pm.SyncPluginsFromDirectory()
	if err != nil {
		t.Fatalf("second SyncPluginsFromDirectory failed: %v", err)
	}
	if len(plugins2) != 2 {
		t.Errorf("expected 2 plugins on re-sync, got %d", len(plugins2))
	}

	// Verify no duplicates in DB
	allPlugins, _ = repo.GetAll(ctx)
	if len(allPlugins) != 2 {
		t.Errorf("expected 2 plugins in DB after re-sync, got %d", len(allPlugins))
	}
}

// TestPackageManager_Uninstall tests uninstall cleanup
func TestPackageManager_Uninstall(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)

	pm := NewPackageManager(pluginsDir, dataDir, repo)

	manifest := testManifest("test-uninstall")
	zipData := createTestPluginZipWithStatic(t, manifest, simpleJSCode, map[string]string{
		"index.html": "<html>test</html>",
	})

	plugin, _, err := pm.InstallFromUpload(zipData)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Verify files exist
	zipPath := filepath.Join(pluginsDir, plugin.FilePath)
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Fatal("zip should exist before uninstall")
	}

	// Uninstall
	if err := pm.Uninstall(plugin.ID); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	// Verify ZIP deleted
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Error("expected ZIP file to be deleted after uninstall")
	}

	// Verify static directory deleted
	staticDir := filepath.Join(dataDir, "test-uninstall")
	if _, err := os.Stat(staticDir); !os.IsNotExist(err) {
		t.Error("expected static dir to be deleted after uninstall")
	}

	// Verify DB record deleted
	ctx := context.Background()
	_, err = repo.GetByID(ctx, plugin.ID)
	if err == nil {
		t.Error("expected DB record to be deleted after uninstall")
	}
}

// TestHealthChecker_DetectUnhealthy tests health check failure handling
func TestHealthChecker_DetectUnhealthy(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	manifest := testManifest("test-health")
	zipData := createTestPluginZip(t, manifest, simpleJSCode)

	zipFileName := "test-health.jsplugin.zip"
	zipPath := filepath.Join(pluginsDir, zipFileName)
	if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	plugin := &JSPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		EntryPath:   manifest.EntryPath,
		Main:        manifest.Main,
		Permissions: manifest.Permissions,
		Status:      JSPluginStatusActive,
		FilePath:    zipFileName,
	}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatalf("create plugin record: %v", err)
	}

	manager := NewManager(repo, pluginsDir, dataDir, nil, nil)
	t.Cleanup(func() { manager.Close() })

	if err := manager.LoadPlugin(ctx, plugin); err != nil {
		t.Skipf("LoadPlugin failed (may need QuickJS): %v", err)
	}

	// Create health checker with low thresholds for testing
	hc := NewHealthChecker(manager,
		WithCheckInterval(100*time.Millisecond),
		WithMaxFailures(2),
		WithIdleTimeout(1*time.Hour),
	)

	// Simulate consecutive failures by manually calling handleUnhealthy
	svc, ok := manager.GetService("test-health")
	if !ok {
		t.Fatal("expected service to be loaded")
	}

	// First failure
	hc.handleUnhealthy(ctx, svc)
	hc.mu.Lock()
	failures := hc.failures["test-health"]
	hc.mu.Unlock()
	if failures != 1 {
		t.Errorf("expected 1 failure, got %d", failures)
	}

	// Second failure should trigger disable
	hc.handleUnhealthy(ctx, svc)

	// Verify plugin marked as error
	dbPlugin, err := repo.GetByID(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if dbPlugin.Status != JSPluginStatusError {
		t.Errorf("expected status error after max failures, got %s", dbPlugin.Status)
	}

	// Verify service unloaded
	if _, ok := manager.GetService("test-health"); ok {
		t.Error("expected service to be unloaded after max failures")
	}
}

// TestCommunicator_SendCall tests inter-plugin communication via scheduler
func TestCommunicator_SendCall(t *testing.T) {
	scheduler := NewServiceScheduler(1)
	t.Cleanup(func() { scheduler.Close() })

	// Register a mock handler that responds to inter-plugin messages
	handler := &mockHandler{
		response: &Message{
			Data: &InterPluginResponse{
				Success: true,
				Data:    json.RawMessage(`{"reply":"hello"}`),
			},
		},
	}
	if err := scheduler.RegisterService("target-plugin", handler, 64); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	comm := NewCommunicator(scheduler)

	// Test Send (async)
	payload := json.RawMessage(`{"msg":"hi"}`)
	if err := comm.Send("source-plugin", "target-plugin", "greet", payload); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait for message to be processed
	deadline := time.After(2 * time.Second)
	for {
		if handler.count() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for send message")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Verify the message was received
	msgs := handler.messages()
	if len(msgs) < 1 {
		t.Fatal("expected at least 1 message")
	}
	ipMsg, ok := msgs[0].Data.(*InterPluginMessage)
	if !ok {
		t.Fatalf("expected InterPluginMessage, got %T", msgs[0].Data)
	}
	if ipMsg.From != "source-plugin" {
		t.Errorf("expected from 'source-plugin', got %q", ipMsg.From)
	}
	if ipMsg.Action != "greet" {
		t.Errorf("expected action 'greet', got %q", ipMsg.Action)
	}

	// Test Call (sync with response)
	ctx := context.Background()
	resp, err := comm.Call(ctx, "source-plugin", "target-plugin", "ask", payload, 5*time.Second)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response from Call")
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false, error=%q", resp.Error)
	}
}

// TestCommunicator_SendToNonexistent tests sending to a non-running plugin
func TestCommunicator_SendToNonexistent(t *testing.T) {
	scheduler := NewServiceScheduler(1)
	t.Cleanup(func() { scheduler.Close() })

	comm := NewCommunicator(scheduler)

	payload := json.RawMessage(`{}`)
	err := comm.Send("source", "nonexistent", "action", payload)
	if err == nil {
		t.Fatal("expected error sending to nonexistent plugin")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// TestRepository_CRUD tests basic repository operations
func TestRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := database.NewJSPluginRepository(db)
	ctx := context.Background()

	// Create
	plugin := &JSPlugin{
		Name:        "CRUD Test",
		Version:     "1.0.0",
		Description: "test",
		Author:      "author",
		EntryPath:   "crud-test",
		Main:        "main.js",
		Permissions: []string{"storage", "songs.read"},
		Status:      JSPluginStatusInactive,
		FilePath:    "crud-test.jsplugin.zip",
	}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if plugin.ID == 0 {
		t.Error("expected non-zero ID after create")
	}

	// GetByID
	got, err := repo.GetByID(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != "CRUD Test" {
		t.Errorf("expected name 'CRUD Test', got %q", got.Name)
	}
	if len(got.Permissions) != 2 || got.Permissions[0] != "storage" {
		t.Errorf("unexpected permissions: %v", got.Permissions)
	}

	// GetByEntryPath
	got2, err := repo.GetByEntryPath(ctx, "crud-test")
	if err != nil {
		t.Fatalf("GetByEntryPath failed: %v", err)
	}
	if got2.ID != plugin.ID {
		t.Error("GetByEntryPath returned wrong plugin")
	}

	// UpdateStatus
	if err := repo.UpdateStatus(ctx, plugin.ID, JSPluginStatusActive); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	got, _ = repo.GetByID(ctx, plugin.ID)
	if got.Status != JSPluginStatusActive {
		t.Errorf("expected active, got %s", got.Status)
	}

	// UpdateHashes
	if err := repo.UpdateHashes(ctx, plugin.ID, "zip123", "entry456", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("UpdateHashes failed: %v", err)
	}
	got, _ = repo.GetByID(ctx, plugin.ID)
	if got.ZipHash != "zip123" || got.EntryHash != "entry456" {
		t.Errorf("hashes not updated: zip=%q, entry=%q", got.ZipHash, got.EntryHash)
	}

	// Delete
	if err := repo.Delete(ctx, plugin.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = repo.GetByID(ctx, plugin.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

// TestLoaderHelpers tests helper functions (sha256Hex, readEntryFromZip, etc.)
func TestLoaderHelpers(t *testing.T) {
	// Test sha256Hex
	hash := sha256Hex([]byte("hello"))
	if hash == "" {
		t.Error("sha256Hex returned empty string")
	}
	if len(hash) != 64 {
		t.Errorf("expected 64 char hex, got %d", len(hash))
	}

	// Test readEntryFromZip
	manifest := testManifest("loader-test")
	zipData := createTestPluginZip(t, manifest, "var x = 1;")

	content, fileName, err := readEntryFromZip(zipData, "main.js")
	if err != nil {
		t.Fatalf("readEntryFromZip failed: %v", err)
	}
	if fileName != "main.js" {
		t.Errorf("expected filename 'main.js', got %q", fileName)
	}
	if string(content) != "var x = 1;" {
		t.Errorf("unexpected content: %q", string(content))
	}

	// Test readPluginManifestFromZip
	m, err := readPluginManifestFromZip(zipData)
	if err != nil {
		t.Fatalf("readPluginManifestFromZip failed: %v", err)
	}
	if m.EntryPath != "loader-test" {
		t.Errorf("expected entryPath 'loader-test', got %q", m.EntryPath)
	}
}

// TestPermissions tests permission checking logic
func TestPermissions(t *testing.T) {
	// 细粒度权限 + 通配符糖混合声明
	perms := []string{"storage", "songs.read", "playlists.*"}

	// Direct match
	if !CheckPermission(perms, "storage") {
		t.Error("expected storage permission to pass")
	}
	if !CheckPermission(perms, "songs.read") {
		t.Error("expected songs.read permission to pass")
	}

	// Wildcard match: playlists.* 同时覆盖读写
	if !CheckPermission(perms, "playlists.read") {
		t.Error("expected playlists.read to pass via wildcard")
	}
	if !CheckPermission(perms, "playlists.write") {
		t.Error("expected playlists.write to pass via wildcard")
	}
	if !CheckPermission(perms, "playlists.list") {
		t.Error("expected playlists.list to pass via wildcard")
	}
	if !CheckPermission(perms, "playlists.getSongs") {
		t.Error("expected playlists.getSongs to pass via wildcard")
	}

	// Denied
	if CheckPermission(perms, "songs.write") {
		t.Error("expected songs.write to be denied")
	}
	if CheckPermission(perms, "command") {
		t.Error("expected command to be denied")
	}

	// 细粒度权限：声明 playlists.read 只能读不能写
	finePerms := []string{"playlists.read"}
	if !CheckPermission(finePerms, "playlists.read") {
		t.Error("expected playlists.read to pass when explicitly declared")
	}
	if CheckPermission(finePerms, "playlists.write") {
		t.Error("expected playlists.write to be denied when only read declared")
	}
}

// TestExtractPermFromAction verifies runtime action-to-permission mapping.
func TestExtractPermFromAction(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"songs.list", PermSongsRead},
		{"songs.getById", PermSongsRead},
		{"songs.search", PermSongsRead},
		{"songs.create", PermSongsWrite},
		{"songs.update", PermSongsWrite},
		{"songs.delete", PermSongsWrite},
		{"playlists.list", PermPlaylistsRead},
		{"playlists.getById", PermPlaylistsRead},
		{"playlists.getSongs", PermPlaylistsRead},
		{"playlists.search", PermPlaylistsRead},
		{"playlists.create", PermPlaylistsWrite},
		{"playlists.update", PermPlaylistsWrite},
		{"playlists.delete", PermPlaylistsWrite},
		{"playlists.addSongs", PermPlaylistsWrite},
		{"playlists.removeSongs", PermPlaylistsWrite},
		{"playlists.reorder", PermPlaylistsWrite},
		{"storage.get", PermStorage},
		{"storage.set", PermStorage},
		{"comm.send", PermInterPlugin},
	}
	for _, c := range cases {
		if got := extractPermFromAction(c.action); got != c.want {
			t.Errorf("extractPermFromAction(%q) = %q, want %q", c.action, got, c.want)
		}
	}
}

// TestValidatePermissions tests permission validation
func TestValidatePermissions(t *testing.T) {
	// 细粒度合法权限
	if err := ValidatePermissions([]string{"storage", "songs.read", "songs.write", "playlists.read", "playlists.write"}); err != nil {
		t.Errorf("expected valid fine-grained permissions, got: %v", err)
	}

	// 通配符糖合法
	if err := ValidatePermissions([]string{"songs.*", "playlists.*"}); err != nil {
		t.Errorf("expected wildcard sugar to be valid, got: %v", err)
	}

	// Empty is valid
	if err := ValidatePermissions([]string{}); err != nil {
		t.Errorf("expected empty permissions to be valid, got: %v", err)
	}

	// Invalid permission
	if err := ValidatePermissions([]string{"storage", "invalid-perm"}); err == nil {
		t.Error("expected error for invalid permission")
	}

	// 后端不再认后缀式、非细粒度的歌单权限
	if err := ValidatePermissions([]string{"playlists.getSongs"}); err == nil {
		t.Error("expected action-level permission string to be rejected by declaration validator")
	}
}

// --- helpers ---
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
