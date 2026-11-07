package jsplugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// playEventJSCode 是一个播放事件订阅插件：onInit 里设置 onPlayEvent 处理器，
// 收到事件后把 type:songId 写入插件 storage，供测试轮询验证事件确实到达 JS 层。
// 订阅注册（RegisterPlayEvent）由测试显式注入，模拟 bridge plugin.registerPlayEvent
// 的效果（与 idle_wakeup_test 注入 RegisterLyricProvider 同约定）。
const playEventJSCode = `
function onInit() {
    globalThis.onPlayEvent = async (event) => {
        await songloft.storage.set('last_play_event', event.type + ':' + event.song.id);
    };
}
function onDeinit() {}
function onHTTPRequest(req) {
    return { statusCode: 200, headers: {"Content-Type": "application/json"}, body: "{}" };
}
`

// setupPlayEventPlugin 按标准流程落 zip、建 DB 记录、建 Manager 并加载插件。
// 返回 manager、plugin 记录（含 ID）与 dataDir。
func setupPlayEventPlugin(t *testing.T, entryPath string) (*Manager, *JSPlugin, string) {
	t.Helper()
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	ctx := context.Background()

	manifest := testManifest(entryPath)
	zipData := createTestPluginZip(t, manifest, playEventJSCode)

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
	return manager, plugin, dataDir
}

// waitForPlayEventMarker 轮询插件 storage 里的 last_play_event 标记文件，
// 返回其内容（JSON 字符串，如 "\"finish:42\""）。
func waitForPlayEventMarker(t *testing.T, dataDir, entryPath string, timeout time.Duration) string {
	t.Helper()
	path := filepath.Join(dataDir, entryPath, "data", "last_play_event")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			return string(b)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("等待播放事件标记超时: %s", path)
	return ""
}

func broadcastTestEvent(m *Manager) {
	m.BroadcastPlayEvent(&PlayEventSong{ID: 42, Title: "Test Song", Artist: "Test Artist"}, "finish", "web")
}

// TestUnloadPlugin_PreservesPlayEventSubscription 验证空闲驱逐（UnloadPlugin）
// 不再清除播放事件订阅：驱逐后插件仍留在订阅集合里，BroadcastPlayEvent 才有
// 唤醒它补投事件的机会（否则统计类插件在驱逐窗口内静默丢数据）。
func TestUnloadPlugin_PreservesPlayEventSubscription(t *testing.T) {
	ctx := context.Background()
	const entryPath = "test-play-evicted"
	manager, _, _ := setupPlayEventPlugin(t, entryPath)

	manager.RegisterPlayEvent(entryPath)

	if err := manager.UnloadPlugin(ctx, entryPath); err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}
	if _, ok := manager.GetService(entryPath); ok {
		t.Fatal("前置条件：UnloadPlugin 后插件应不在内存")
	}
	if !manager.HasPlayEventSubscriber(entryPath) {
		t.Fatal("UnloadPlugin 不应清除播放事件订阅（驱逐窗口内的事件靠它触发唤醒补投）")
	}
}

// TestBroadcastPlayEvent_DeliversToLoadedSubscriber 验证事件送达在内存的订阅插件，
// 且 JS 层 onPlayEvent 处理器真正执行（storage 落盘标记）。
func TestBroadcastPlayEvent_DeliversToLoadedSubscriber(t *testing.T) {
	const entryPath = "test-play-deliver"
	manager, _, dataDir := setupPlayEventPlugin(t, entryPath)

	manager.RegisterPlayEvent(entryPath)
	broadcastTestEvent(manager)

	got := waitForPlayEventMarker(t, dataDir, entryPath, 10*time.Second)
	var event string
	if err := json.Unmarshal([]byte(got), &event); err != nil {
		t.Fatalf("解析标记内容 %q: %v", got, err)
	}
	if event != "finish:42" {
		t.Errorf("期望事件 finish:42，got %q", event)
	}
}

// TestBroadcastPlayEvent_WakesEvictedSubscriber 核心场景：订阅插件被空闲驱逐后，
// 播放事件广播经 EnsureLoaded 同步唤醒它并补投事件（与歌词 provider 的 #303 同策略）。
func TestBroadcastPlayEvent_WakesEvictedSubscriber(t *testing.T) {
	ctx := context.Background()
	const entryPath = "test-play-wake"
	manager, _, dataDir := setupPlayEventPlugin(t, entryPath)

	manager.RegisterPlayEvent(entryPath)

	// 模拟空闲驱逐：卸 VM，订阅保留（生产同款行为）
	if err := manager.UnloadPlugin(ctx, entryPath); err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}
	if _, ok := manager.GetService(entryPath); ok {
		t.Fatal("前置条件：驱逐后插件应处于休眠（不在内存）")
	}
	if !manager.HasPlayEventSubscriber(entryPath) {
		t.Fatal("前置条件：驱逐后订阅应保留")
	}

	broadcastTestEvent(manager)

	// 唤醒后应回到内存，且事件补投到 JS 层
	if _, ok := manager.GetService(entryPath); !ok {
		t.Fatal("BroadcastPlayEvent 后插件应被唤醒并留存内存")
	}
	got := waitForPlayEventMarker(t, dataDir, entryPath, 10*time.Second)
	var event string
	if err := json.Unmarshal([]byte(got), &event); err != nil {
		t.Fatalf("解析标记内容 %q: %v", got, err)
	}
	if event != "finish:42" {
		t.Errorf("期望补投事件 finish:42，got %q", event)
	}
}

// TestDisablePlugin_RemovesPlayEventSubscription 验证禁用插件显式撤销订阅——
// 包括"已被空闲驱逐后禁用"的情形（UnloadPlugin 对不在内存的插件提前返回，
// DisablePlugin 里的显式 Delete 是唯一清理机会，此前该情形会泄漏订阅）。
func TestDisablePlugin_RemovesPlayEventSubscription(t *testing.T) {
	ctx := context.Background()
	const entryPath = "test-play-disable"
	manager, plugin, _ := setupPlayEventPlugin(t, entryPath)

	manager.RegisterPlayEvent(entryPath)

	// 先驱逐再禁用：复现"UnloadPlugin 提前返回"的泄漏路径
	if err := manager.UnloadPlugin(ctx, entryPath); err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}
	if err := manager.DisablePlugin(ctx, plugin.ID); err != nil {
		t.Fatalf("DisablePlugin failed: %v", err)
	}
	if manager.HasPlayEventSubscriber(entryPath) {
		t.Fatal("DisablePlugin 应显式清除播放事件订阅（包括驱逐后禁用的情形）")
	}

	// 禁用后广播不得再唤醒它
	broadcastTestEvent(manager)
	if _, ok := manager.GetService(entryPath); ok {
		t.Fatal("禁用插件的订阅已清除，广播不应把它拉起")
	}
}

// TestBroadcastPlayEvent_LazyCleansDeletedSubscription 验证对已删除插件（DB 无记录）
// 的残留订阅，广播时经 EnsureLoaded→ErrPluginNotFound 惰性清理，不会反复白试。
func TestBroadcastPlayEvent_LazyCleansDeletedSubscription(t *testing.T) {
	manager, _, _ := setupPlayEventPlugin(t, "test-play-ghost")

	const ghost = "ghost-deleted-plugin"
	manager.RegisterPlayEvent(ghost)

	broadcastTestEvent(manager)

	if manager.HasPlayEventSubscriber(ghost) {
		t.Fatal("对已删除插件的残留订阅应在广播时被惰性清理")
	}
}
