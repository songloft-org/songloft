package jsplugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

const udpDuringHTTPRequestJSCode = `
globalThis.onInit = async function() {};
globalThis.onDeinit = async function() {};
globalThis.onHTTPRequest = async function(req) {
    var received = [];
    var bind = await songloft.net.udpBind({ address: "127.0.0.1:0" });
    songloft.net.onData(bind.socketId, function(event) {
        received.push(atob(event.data));
    });
    await songloft.net.udpSend(bind.socketId, "hello", bind.localAddr);
    await new Promise(function(resolve) { setTimeout(resolve, 300); });
    await songloft.net.udpClose(bind.socketId);
    return {
        statusCode: 200,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ received: received })
    };
};
`

func loadNetIntegrationTestPlugin(t *testing.T, entryPath string) (*Manager, http.Handler) {
	t.Helper()

	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	manifest := testManifest(entryPath)
	manifest.Permissions = []string{PermNet}
	zipData := createTestPluginZip(t, manifest, udpDuringHTTPRequestJSCode)

	zipFileName := entryPath + ".jsplugin.zip"
	if err := os.WriteFile(filepath.Join(pluginsDir, zipFileName), zipData, 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}

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

	manager := NewManager(repo, pluginsDir, dataDir, "", nil, nil)
	t.Cleanup(func() { manager.Close() })

	if err := manager.LoadPlugin(ctx, plugin); err != nil {
		t.Skipf("LoadPlugin failed (may need QuickJS runtime): %v", err)
	}

	router := chi.NewRouter()
	manager.RegisterAPIRoutes(router)
	return manager, router
}

func TestUDPHostEventDeliveredDuringHTTPRequest(t *testing.T) {
	_, router := loadNetIntegrationTestPlugin(t, "udp-during-http")
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/jsplugin/udp-during-http/api/scan")
	if err != nil {
		t.Fatalf("GET plugin route: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Received []string `json:"received"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal body %s: %v", string(body), err)
	}
	if len(payload.Received) != 1 || payload.Received[0] != "hello" {
		t.Fatalf("received = %#v, want [hello]", payload.Received)
	}
}
