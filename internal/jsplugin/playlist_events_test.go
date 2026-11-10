package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

const playlistEventsJS = `
var received = [];
async function subscribe() {
    await songloft.events.onPlaylistSongsChange(async function(event) {
        received.push(event);
        // Persist the short lifecycle history; keep the large synthetic dispatch
        // batch in memory to avoid quadratic fixture serialization under -race.
        if (received.length <= 4) { await songloft.storage.set('changes', received); }
    });
}
async function onInit() { received = await songloft.storage.get('changes') || []; await subscribe(); }
function onDeinit() {}
async function onHTTPRequest(req) {
    var result = {};
    if (req.path === '/off') { await songloft.events.offPlaylistSongsChange(); }
    else if (req.path === '/on') { await subscribe(); }
    else if (req.path === '/write') {
        var input = JSON.parse(req.body);
        await songloft.playlists.addSongs(input.playlistId, input.songIds);
        await songloft.playlists.removeSongs(input.playlistId, input.songIds);
    }
    else { result = received; }
    return { statusCode: 200, headers: {}, body: JSON.stringify(result) };
}
`

func TestPlaylistEventsPluginLifecycle(t *testing.T) {
	ctx := context.Background()
	db := testutil.OpenMemoryDB(t)
	pluginsDir, dataDir := t.TempDir(), t.TempDir()
	repo := db.JSPluginRepository()
	manifest := testManifest("membership-events")
	manifest.Permissions = []string{"storage", "playlists.read", "playlists.write"}
	zipName := manifest.EntryPath + ".jsplugin.zip"
	if err := os.WriteFile(filepath.Join(pluginsDir, zipName), createTestPluginZip(t, manifest, playlistEventsJS), 0o644); err != nil {
		t.Fatal(err)
	}
	plugin := &JSPlugin{Name: manifest.Name, Version: manifest.Version, EntryPath: manifest.EntryPath, Main: manifest.Main, Permissions: manifest.Permissions, Status: JSPluginStatusActive, FilePath: zipName}
	if err := repo.Create(ctx, plugin); err != nil {
		t.Fatal(err)
	}
	m := NewManager(repo, pluginsDir, dataDir, "", nil, db)
	t.Cleanup(func() { _ = m.Close() })
	svc := services.NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), nil)
	svc.SetTransactor(db)
	m.SetServices(nil, svc, nil)
	if err := m.LoadPlugin(ctx, plugin); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.playlistSongsSubscribers.Load(plugin.EntryPath); !ok {
		t.Fatal("onInit did not register through bridge")
	}
	router := chi.NewRouter()
	m.RegisterAPIRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	request := func(method, path, body string) string {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, result)
		}
		return string(result)
	}
	pluginPath := "/api/v1/jsplugin/" + plugin.EntryPath
	readEvents := func() []models.PlaylistSongsChangeEvent {
		t.Helper()
		var events []models.PlaylistSongsChangeEvent
		if err := json.Unmarshal([]byte(request("GET", pluginPath+"/events", "")), &events); err != nil {
			t.Fatal(err)
		}
		return events
	}
	waitEvents := func(count int) []models.PlaylistSongsChangeEvent {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			events := readEvents()
			if len(events) == count {
				return events
			}
			if len(events) > count {
				t.Fatalf("extra events: %+v", events)
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("expected %d events, got %+v", count, readEvents())
		return nil
	}
	song := &models.Song{Title: "Love ' 雪", Artist: "Artist", Type: models.TypeRemote, URL: "https://example.com/audio"}
	if err := db.SongRepository().Create(ctx, song); err != nil {
		t.Fatal(err)
	}
	addFavorite := func() {
		t.Helper()
		if _, _, err := svc.AddSongs(ctx, 1, []int64{song.ID, song.ID}); err != nil {
			t.Fatal(err)
		}
	}
	removeFavorite := func() {
		t.Helper()
		if err := svc.RemoveSong(ctx, 1, song.ID); err != nil {
			t.Fatal(err)
		}
	}
	addFavorite()
	first := waitEvents(1)[0]
	if first.Type != "add" || first.PlaylistID != 1 || first.SongID != song.ID || first.Song.Title != song.Title || first.Song.Artist != song.Artist || first.Timestamp == 0 {
		t.Fatalf("wrong payload: %+v", first)
	}
	// Eviction preserves subscription, reloads onInit, then dispatches the removal.
	if err := m.UnloadPlugin(ctx, plugin.EntryPath); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.GetService(plugin.EntryPath); ok {
		t.Fatal("plugin still loaded")
	}
	removeFavorite()
	// Wait using storage so an HTTP read cannot itself wake the plugin.
	marker := filepath.Join(dataDir, plugin.EntryPath, "data", "changes")
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, _ := os.ReadFile(marker)
		var events []models.PlaylistSongsChangeEvent
		_ = json.Unmarshal(data, &events)
		if len(events) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("evicted subscriber was not woken: %s", data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if event := waitEvents(2)[1]; event.Type != "remove" || event.Song.Title != song.Title {
		t.Fatalf("wrong removal: %+v", event)
	}
	// Duplicate registration replaces the handler without adding a second listener.
	request("POST", pluginPath+"/on", "")
	request("POST", pluginPath+"/on", "")
	request("POST", pluginPath+"/off", "")
	if _, ok := m.playlistSongsSubscribers.Load(plugin.EntryPath); ok {
		t.Fatal("unsubscribe did not reach manager")
	}
	addFavorite()
	if events := readEvents(); len(events) != 2 {
		t.Fatalf("unsubscribed events: %+v", events)
	}
	removeFavorite()
	request("POST", pluginPath+"/on", "")
	// Writes inside the subscriber's own HTTP callback must not deadlock.
	request("POST", pluginPath+"/write", fmt.Sprintf(`{"playlistId":1,"songIds":[%d]}`, song.ID))
	events := waitEvents(4)
	if events[2].Type != "add" || events[3].Type != "remove" {
		t.Fatalf("out of order: %+v", events)
	}
	// A single membership operation can exceed the actor queue's capacity.
	batch := make([]models.PlaylistSongsChangeEvent, defaultQueueSize+16)
	for i := range batch {
		batch[i] = first
		batch[i].SongID = int64(i + 1000)
	}
	m.BroadcastPlaylistSongsChange(batch)
	events = waitEvents(4 + len(batch))
	for i, event := range events[4:] {
		if event.SongID != batch[i].SongID {
			t.Fatalf("batch order at %d: %+v", i, event)
		}
	}
	if err := m.UnloadPlugin(ctx, plugin.EntryPath); err != nil {
		t.Fatal(err)
	}
	if err := m.DisablePlugin(ctx, plugin.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.playlistSongsSubscribers.Load(plugin.EntryPath); ok {
		t.Fatal("disabled plugin retains subscription")
	}
	addFavorite()
	if _, ok := m.GetService(plugin.EntryPath); ok {
		t.Fatal("disabled plugin woke up")
	}
}

func TestPlaylistEventsReloadRechecksPermission(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	manifest := testManifest("changed-permission")
	manifest.Permissions = []string{"storage", "playlists.read"}
	zipName := manifest.EntryPath + ".jsplugin.zip"
	if err := os.WriteFile(filepath.Join(pluginsDir, zipName), createTestPluginZip(t, manifest, playlistEventsJS), 0o644); err != nil {
		t.Fatal(err)
	}
	plugin := &JSPlugin{Name: manifest.Name, Version: manifest.Version, EntryPath: manifest.EntryPath, Main: manifest.Main, Permissions: manifest.Permissions, Status: JSPluginStatusActive, FilePath: zipName}
	if err := repo.Create(context.Background(), plugin); err != nil {
		t.Fatal(err)
	}
	m := NewManager(repo, pluginsDir, dataDir, "", nil, nil)
	defer m.Close()
	if err := m.LoadPlugin(context.Background(), plugin); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.playlistSongsSubscribers.Load(plugin.EntryPath); !ok {
		t.Fatal("initial registration failed")
	}
	if err := m.UnloadPlugin(context.Background(), plugin.EntryPath); err != nil {
		t.Fatal(err)
	}
	// A replacement package no longer has read permission.
	manifest.Permissions = []string{"storage"}
	plugin.Permissions = manifest.Permissions
	if err := os.WriteFile(filepath.Join(pluginsDir, zipName), createTestPluginZip(t, manifest, playlistEventsJS), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadPlugin(context.Background(), plugin); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.playlistSongsSubscribers.Load(plugin.EntryPath); ok {
		t.Fatal("reload inherited revoked permission")
	}
}

func TestPlaylistEventsRegistrationPermission(t *testing.T) {
	for _, tc := range []struct {
		perms   []string
		allowed bool
	}{
		{nil, false}, {[]string{PermPlaylistsWrite}, false}, {[]string{PermPlaylistsRead}, true}, {[]string{"playlists.*"}, true},
	} {
		h := &BridgeHandler{permissions: tc.perms, service: &JSService{plugin: &JSPlugin{EntryPath: "permissions"}}}
		registered := false
		h.onPlaylistSongsChangeRegister = func(string) { registered = true }
		_, err := h.HandleBridgeCall("plugin.registerPlaylistSongsChange", "")
		if (err == nil) != tc.allowed || registered != tc.allowed {
			t.Fatalf("permissions %v: registered=%v err=%v", tc.perms, registered, err)
		}
	}
}

func TestPlaylistEventsCleansDeletedSubscriber(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	m := NewManager(db.JSPluginRepository(), t.TempDir(), t.TempDir(), "", nil, db)
	defer m.Close()
	m.RegisterPlaylistSongsChange("deleted")
	m.deliverPlaylistChanges(context.Background(), playlistChangeBatch{targets: []string{"deleted"}})
	if _, ok := m.playlistSongsSubscribers.Load("deleted"); ok {
		t.Fatal("deleted subscription retained")
	}
}
