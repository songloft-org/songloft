package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

func newTestConfigHandler(t *testing.T) *ConfigHandler {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	return NewConfigHandler(services.NewConfigService(mdb.ConfigRepository()), mdb.JSPluginRepository())
}

// seedTestPlugin 在库中预置一个插件记录。
// tab-config 的限额统计与孤儿清理都基于插件安装状态，条目必须指向真实插件。
func seedTestPlugin(t *testing.T, mdb *database.SQLiteDB, entryPath string, status models.JSPluginStatus) {
	t.Helper()
	p := &models.JSPlugin{
		Name:      entryPath,
		Version:   "1.0.0",
		EntryPath: entryPath,
		Main:      "main.js",
		Status:    status,
		FilePath:  entryPath + ".jsplugin.zip",
	}
	if err := mdb.JSPluginRepository().Create(context.Background(), p); err != nil {
		t.Fatalf("seed plugin %q: %v", entryPath, err)
	}
}

func TestTabConfigSetting_Default(t *testing.T) {
	h := newTestConfigHandler(t)

	rr := httptest.NewRecorder()
	h.GetTabConfigSetting(rr, httptest.NewRequest("GET", "/api/v1/settings/tab-config", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200, body=%s", rr.Code, rr.Body.String())
	}
	var resp tabConfigSetting
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rr.Body.String())
	}
	if !resp.ShowLibrary {
		t.Error("default show_library should be true")
	}
	if !resp.ShowHome {
		t.Error("default show_home should be true")
	}
	if !resp.ShowPlaylists {
		t.Error("default show_playlists should be true")
	}
	if len(resp.PluginTabs) != 0 {
		t.Errorf("default plugin_tabs should be empty, got %d", len(resp.PluginTabs))
	}
}

func TestTabConfigSetting_ShowHomeLegacyCompatibility(t *testing.T) {
	h := newTestConfigHandler(t)
	const path = "/api/v1/settings/tab-config"
	const legacy = `{"show_library":false,"plugin_tabs":[]}`
	if err := h.configService.SetJSON(tabConfigKey, json.RawMessage(legacy)); err != nil {
		t.Fatal(err)
	}

	check := func(wantHome bool, body string) {
		t.Helper()
		if body != "" {
			rr := httptest.NewRecorder()
			h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", path, strings.NewReader(body)))
			if rr.Code != http.StatusOK {
				t.Fatalf("PUT: %d %s", rr.Code, rr.Body)
			}
		}

		rr := httptest.NewRecorder()
		h.GetTabConfigSetting(rr, httptest.NewRequest("GET", path, nil))
		var cfg tabConfigSetting
		if err := json.Unmarshal(rr.Body.Bytes(), &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.ShowHome != wantHome || cfg.ShowLibrary {
			t.Fatalf("GET: got %s, want show_home=%t and show_library=false", rr.Body, wantHome)
		}
	}
	check(true, "")
	check(false, `{"show_home":false,"show_library":false,"plugin_tabs":[]}`)
	check(false, legacy)
	check(true, `{"show_home":true,"show_library":false,"plugin_tabs":[]}`)
}

func TestTabConfigSetting_UpdateThenRead(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	seedTestPlugin(t, mdb, "subsonic", models.JSPluginStatusActive)
	h := NewConfigHandler(services.NewConfigService(mdb.ConfigRepository()), mdb.JSPluginRepository())

	body := `{"show_library":false,"show_playlists":true,"plugin_tabs":[{"plugin_id":1,"entry_path":"subsonic","name":"Subsonic"}]}`
	rr1 := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr1, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(body)))
	if rr1.Code != http.StatusOK {
		t.Fatalf("PUT status: got %d want 200, body=%s", rr1.Code, rr1.Body.String())
	}

	rr2 := httptest.NewRecorder()
	h.GetTabConfigSetting(rr2, httptest.NewRequest("GET", "/api/v1/settings/tab-config", nil))
	var resp tabConfigSetting
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ShowLibrary {
		t.Error("show_library should be false after update")
	}
	if !resp.ShowPlaylists {
		t.Error("show_playlists should be true")
	}
	if len(resp.PluginTabs) != 1 {
		t.Fatalf("plugin_tabs length: got %d want 1", len(resp.PluginTabs))
	}
	if resp.PluginTabs[0].EntryPath != "subsonic" {
		t.Errorf("entry_path: got %q want subsonic", resp.PluginTabs[0].EntryPath)
	}
}

// TestTabConfigSetting_PlaylistsNotCounted 回归 #416：
// show_playlists 是歌单并入曲库后遗留的兼容字段（恒为 true），不得参与限额统计，
// 否则真实可保存的可选项上限比前端承诺的少 1 个，用户在计数 11 时就被后端拒绝。
func TestTabConfigSetting_PlaylistsNotCounted(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	for i := 0; i < 9; i++ {
		seedTestPlugin(t, mdb, string(rune('a'+i)), models.JSPluginStatusActive)
	}
	h := NewConfigHandler(services.NewConfigService(mdb.ConfigRepository()), mdb.JSPluginRepository())

	// show_library + 9 个 active 插件 = 10 个可选项（show_playlists=true 不计入）
	plugins := make([]pluginTabEntry, 9)
	for i := range plugins {
		plugins[i] = pluginTabEntry{PluginID: i + 1, EntryPath: string(rune('a' + i)), Name: string(rune('A' + i))}
	}
	req := tabConfigSetting{ShowLibrary: true, ShowPlaylists: true, PluginTabs: plugins}
	body, _ := json.Marshal(req)
	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(string(body))))
	if rr.Code != http.StatusOK {
		t.Errorf("10 optional tabs should pass: got %d want 200, body=%s", rr.Code, rr.Body.String())
	}
}

func TestTabConfigSetting_ExceedLimit(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	for i := 0; i < 10; i++ {
		seedTestPlugin(t, mdb, string(rune('a'+i)), models.JSPluginStatusActive)
	}
	h := NewConfigHandler(services.NewConfigService(mdb.ConfigRepository()), mdb.JSPluginRepository())

	// 11 个可选项 = 超过上限 10（show_library + 10 个 active 插件）
	plugins := make([]pluginTabEntry, 10)
	for i := range plugins {
		plugins[i] = pluginTabEntry{PluginID: i + 1, EntryPath: string(rune('a' + i)), Name: string(rune('A' + i))}
	}
	req := tabConfigSetting{ShowLibrary: true, ShowPlaylists: true, PluginTabs: plugins}
	body, _ := json.Marshal(req)
	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(string(body))))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("exceed limit: got %d want 400, body=%s", rr.Code, rr.Body.String())
	}
}

func TestTabConfigSetting_ManyTabs(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	for i := 0; i < 6; i++ {
		seedTestPlugin(t, mdb, string(rune('a'+i)), models.JSPluginStatusActive)
	}
	h := NewConfigHandler(services.NewConfigService(mdb.ConfigRepository()), mdb.JSPluginRepository())

	// 7 个可选项 = 在上限 10 以内（show_library + 6 个 active 插件）
	plugins := make([]pluginTabEntry, 6)
	for i := range plugins {
		plugins[i] = pluginTabEntry{PluginID: i + 1, EntryPath: string(rune('a' + i)), Name: string(rune('A' + i))}
	}
	req := tabConfigSetting{ShowLibrary: true, ShowPlaylists: true, PluginTabs: plugins}
	body, _ := json.Marshal(req)
	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(string(body))))
	if rr.Code != http.StatusOK {
		t.Errorf("many tabs should succeed: got %d want 200, body=%s", rr.Code, rr.Body.String())
	}

	// 验证读取返回正确数量
	rr2 := httptest.NewRecorder()
	h.GetTabConfigSetting(rr2, httptest.NewRequest("GET", "/api/v1/settings/tab-config", nil))
	var resp tabConfigSetting
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.PluginTabs) != 6 {
		t.Errorf("plugin_tabs length: got %d want 6", len(resp.PluginTabs))
	}
}

// TestTabConfigSetting_PruneOrphanEntries 回归 #416：
// 插件卸载后遗留的孤儿条目应在保存时被静默清理，且不占用限额名额。
func TestTabConfigSetting_PruneOrphanEntries(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	seedTestPlugin(t, mdb, "alive", models.JSPluginStatusActive)
	h := NewConfigHandler(services.NewConfigService(mdb.ConfigRepository()), mdb.JSPluginRepository())

	req := tabConfigSetting{
		ShowLibrary: true,
		PluginTabs: []pluginTabEntry{
			{PluginID: 1, EntryPath: "alive", Name: "Alive"},
			{PluginID: 2, EntryPath: "ghost", Name: "已卸载的插件"},
		},
	}
	body, _ := json.Marshal(req)
	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(string(body))))
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status: got %d want 200, body=%s", rr.Code, rr.Body.String())
	}
	var resp tabConfigSetting
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.PluginTabs) != 1 {
		t.Fatalf("orphan entry should be pruned: got %d entries want 1, body=%s", len(resp.PluginTabs), rr.Body.String())
	}
	if resp.PluginTabs[0].EntryPath != "alive" {
		t.Errorf("kept entry: got %q want alive", resp.PluginTabs[0].EntryPath)
	}

	// 再读一次确认持久化的也是清理后的配置
	rr2 := httptest.NewRecorder()
	h.GetTabConfigSetting(rr2, httptest.NewRequest("GET", "/api/v1/settings/tab-config", nil))
	var persisted tabConfigSetting
	if err := json.Unmarshal(rr2.Body.Bytes(), &persisted); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(persisted.PluginTabs) != 1 {
		t.Errorf("persisted plugin_tabs length: got %d want 1", len(persisted.PluginTabs))
	}
}

// TestTabConfigSetting_DisabledPluginKeptButNotCounted：
// 禁用插件的条目保留（重新启用后 Tab 自动恢复），但不占用限额名额。
func TestTabConfigSetting_DisabledPluginKeptButNotCounted(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	for i := 0; i < 9; i++ {
		seedTestPlugin(t, mdb, string(rune('a'+i)), models.JSPluginStatusActive)
	}
	seedTestPlugin(t, mdb, "disabled", models.JSPluginStatusInactive)
	h := NewConfigHandler(services.NewConfigService(mdb.ConfigRepository()), mdb.JSPluginRepository())

	// 9 个 active + 1 个 inactive + show_library：实际占名额 10 个，恰好在上限内
	plugins := make([]pluginTabEntry, 0, 10)
	for i := 0; i < 9; i++ {
		plugins = append(plugins, pluginTabEntry{PluginID: i + 1, EntryPath: string(rune('a' + i)), Name: string(rune('A' + i))})
	}
	plugins = append(plugins, pluginTabEntry{PluginID: 100, EntryPath: "disabled", Name: "Disabled"})
	req := tabConfigSetting{ShowLibrary: true, PluginTabs: plugins}
	body, _ := json.Marshal(req)
	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(string(body))))
	if rr.Code != http.StatusOK {
		t.Fatalf("disabled plugin should not consume quota: got %d want 200, body=%s", rr.Code, rr.Body.String())
	}
	var resp tabConfigSetting
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.PluginTabs) != 10 {
		t.Errorf("disabled plugin entry should be kept: got %d entries want 10", len(resp.PluginTabs))
	}
}

func TestTabConfigSetting_DuplicateEntryPath(t *testing.T) {
	h := newTestConfigHandler(t)

	body := `{"show_library":false,"show_playlists":false,"plugin_tabs":[{"plugin_id":1,"entry_path":"same","name":"A"},{"plugin_id":2,"entry_path":"same","name":"B"}]}`
	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(body)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("duplicate entry_path: got %d want 400", rr.Code)
	}
}

func TestTabConfigSetting_EmptyEntryPath(t *testing.T) {
	h := newTestConfigHandler(t)

	body := `{"show_library":false,"show_playlists":false,"plugin_tabs":[{"plugin_id":1,"entry_path":"","name":"A"}]}`
	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(body)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("empty entry_path: got %d want 400", rr.Code)
	}
}

func TestTabConfigSetting_BadJSON(t *testing.T) {
	h := newTestConfigHandler(t)

	rr := httptest.NewRecorder()
	h.UpdateTabConfigSetting(rr, httptest.NewRequest("PUT", "/api/v1/settings/tab-config",
		strings.NewReader(`not json`)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad JSON: got %d want 400", rr.Code)
	}
}

// TestRemoveTabConfigEntry 卸载插件时应移除 tab_config 中对应条目（#416）。
func TestRemoveTabConfigEntry(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	cfgSvc := services.NewConfigService(mdb.ConfigRepository())

	// 预置含两个条目的配置
	cfg := tabConfigSetting{
		ShowLibrary: true,
		PluginTabs: []pluginTabEntry{
			{PluginID: 1, EntryPath: "keep", Name: "Keep"},
			{PluginID: 2, EntryPath: "gone", Name: "Gone"},
		},
	}
	if err := cfgSvc.SetJSON(tabConfigKey, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	h := NewJSPluginHandler(nil, nil, nil, nil, cfgSvc, nil)
	h.removeTabConfigEntry("gone")

	var got tabConfigSetting
	if err := cfgSvc.GetJSON(tabConfigKey, &got); err != nil {
		t.Fatalf("read config: %v", err)
	}
	if len(got.PluginTabs) != 1 || got.PluginTabs[0].EntryPath != "keep" {
		t.Errorf("plugin_tabs after removal: got %+v, want only keep", got.PluginTabs)
	}

	// 移除不存在的条目应为幂等无操作
	h.removeTabConfigEntry("not-there")
	var got2 tabConfigSetting
	if err := cfgSvc.GetJSON(tabConfigKey, &got2); err != nil {
		t.Fatalf("read config: %v", err)
	}
	if len(got2.PluginTabs) != 1 {
		t.Errorf("idempotent removal changed entries: got %d want 1", len(got2.PluginTabs))
	}
}
