package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mimusic/internal/database"
	"mimusic/internal/jsruntime"
)

// pluginBootstrapJS 是注入到每个插件 JS 环境的引导代码
// 定义了 mimusic 全局对象的基础结构和生命周期回调桩。
//
// 设计原则：所有 __go_bridge 调用统一通过 __callBridge（polyfill 提供）
// 包装为 Promise<string>，mimusic.* 接口对外都返回 Promise，
// 调用方必须 `await`。这与 fetch / setTimeout 等 Web 标准 API 行为一致，
// 也是 JS 接口"真异步"的核心契约。
const pluginBootstrapJS = `
// MiMusic JS Plugin API 基础框架
var mimusic = mimusic || {};

// 插件生命周期默认空实现（插件代码通过 globalThis.xxx = ... 覆盖）。
// 注意：必须使用 globalThis 赋值而非 function 声明，
// 否则 QuickJS 中 Annex B 块级函数声明会创建 declarative binding，
// 导致插件 IIFE 中 globalThis.onHTTPRequest = ... 无法覆盖。
//
// onHTTPRequest 默认实现为 async：与异步 mimusic.* API 自然组合。
if (typeof globalThis.onInit !== 'function') { globalThis.onInit = async function() {}; }
if (typeof globalThis.onDeinit !== 'function') { globalThis.onDeinit = async function() {}; }
if (typeof globalThis.onHTTPRequest !== 'function') {
    globalThis.onHTTPRequest = async function(req) {
        return { statusCode: 404, headers: {}, body: 'not implemented' };
    };
}

// 日志（同步本地操作，无需 await）
mimusic.log = {
    info: function(msg) { console.log('[plugin] ' + msg); },
    warn: function(msg) { console.warn('[plugin] ' + msg); },
    error: function(msg) { console.error('[plugin] ' + msg); }
};

// === mimusic.storage（async）===
mimusic.storage = {
    get: async function(key) {
        var s = await __callBridge('storage.get', key);
        return s ? JSON.parse(s) : null;
    },
    set: async function(key, value) {
        await __callBridge('storage.set', JSON.stringify({key: key, value: value}));
    },
    delete: async function(key) {
        await __callBridge('storage.delete', key);
    },
    keys: async function() {
        var s = await __callBridge('storage.keys', '');
        return s ? JSON.parse(s) : [];
    }
};

// === mimusic.songs（async）===
mimusic.songs = {
    list: async function(options) {
        var s = await __callBridge('songs.list', JSON.stringify(options || {}));
        return s ? JSON.parse(s) : [];
    },
    getById: async function(id) {
        var s = await __callBridge('songs.getById', JSON.stringify({id: id}));
        return s ? JSON.parse(s) : null;
    },
    search: async function(query) {
        var s = await __callBridge('songs.search', JSON.stringify({query: query}));
        return s ? JSON.parse(s) : [];
    }
};

// === mimusic.playlists（async）===
mimusic.playlists = {
    list: async function() {
        var s = await __callBridge('playlists.list', '');
        return s ? JSON.parse(s) : [];
    },
    getById: async function(id) {
        var s = await __callBridge('playlists.getById', JSON.stringify({id: id}));
        return s ? JSON.parse(s) : null;
    },
    getSongs: async function(id, options) {
        var s = await __callBridge('playlists.getSongs', JSON.stringify({id: id, options: options || {}}));
        return s ? JSON.parse(s) : [];
    }
};

// === mimusic.plugin（async）===
// 即使 getToken/getHostUrl 内部是 O(1) 的内存读取，也统一返回 Promise，
// 保证 mimusic.* API 表面一致；插件代码用 const t = await mimusic.plugin.getToken()。
mimusic.plugin = {
    getToken: async function() {
        return await __callBridge('plugin.getToken', '');
    },
    getHostUrl: async function() {
        return await __callBridge('plugin.getHostUrl', '');
    }
};

// === mimusic.jsenv（async）===
// 子 JS 环境（独立 QuickJS VM）：用于在插件内创建隔离的 sandbox 跑用户脚本，
// 跨 env 真并行（ExecuteJSParallel），生命周期受 pluginID 管理（DestroyPluginEnvs 自动回收）。
mimusic.jsenv = {
    create: async function(name, initCode) {
        var s = await __callBridge('jsenv.create', JSON.stringify({name: name, initCode: initCode || ''}));
        var p = s ? JSON.parse(s) : {};
        if (p.error) throw new Error('jsenv.create: ' + p.error);
        return p.envName;
    },
    execute: async function(name, code, timeoutMs) {
        var s = await __callBridge('jsenv.execute', JSON.stringify({name: name, code: code, timeoutMs: timeoutMs || 30000}));
        return s ? JSON.parse(s) : {result: '', events: []};
    },
    executeWait: async function(name, code, timeoutMs, waitEvents) {
        var s = await __callBridge('jsenv.executeWait', JSON.stringify({name: name, code: code, timeoutMs: timeoutMs || 30000, waitEvents: waitEvents || []}));
        return s ? JSON.parse(s) : {result: '', events: []};
    },
    executeParallel: async function(calls, maxConcurrent) {
        var s = await __callBridge('jsenv.executeParallel', JSON.stringify({calls: calls || [], maxConcurrent: maxConcurrent || 0}));
        return s ? JSON.parse(s) : {successIndex: -1, errors: []};
    },
    destroy: async function(name) {
        await __callBridge('jsenv.destroy', JSON.stringify({name: name}));
    },
    list: async function() {
        var s = await __callBridge('jsenv.list', '');
        return s ? JSON.parse(s) : [];
    }
};
`

// GetBootstrapCode 返回插件引导 JS 代码（含通信 API）
func GetBootstrapCode() string {
	return pluginBootstrapJS + GenerateCommJS()
}

// BridgeHandler 处理 JS 通过 __go_bridge 调用的请求
type BridgeHandler struct {
	service     *JSService
	permissions []string
	dataDir     string      // data/jsplugins_data/
	db          database.DB // 数据库访问（用于 songs/playlists 查询）
	pluginToken string      // 插件专用的永久 JWT Token
	port        string      // 服务器监听端口（用于构造宿主 URL）
}

// NewBridgeHandler 创建桥接处理器
func NewBridgeHandler(service *JSService, dataDir string, db database.DB, pluginToken string, port string) *BridgeHandler {
	return &BridgeHandler{
		service:     service,
		permissions: service.plugin.Permissions,
		dataDir:     dataDir,
		db:          db,
		pluginToken: pluginToken,
		port:        port,
	}
}

// HandleBridgeCall 处理桥接调用（由 jsruntime 层回调）
// action: 如 "http.get", "storage.set" 等
// data: JSON 参数
func (h *BridgeHandler) HandleBridgeCall(action, data string) (string, error) {
	// plugin.* 操作是插件内置能力，不需要额外权限声明
	if strings.HasPrefix(action, "plugin.") {
		return h.handlePlugin(action, data)
	}

	// 1. 解析 action 的权限前缀
	requiredPerm := extractPermFromAction(action)

	// 2. 检查权限
	if !CheckPermission(h.permissions, requiredPerm) {
		return "", fmt.Errorf("permission denied: requires '%s'", requiredPerm)
	}

	// 3. 分发到具体处理器
	switch {
	case strings.HasPrefix(action, "storage."):
		return h.handleStorage(action, data)
	case strings.HasPrefix(action, "songs."):
		return h.handleSongs(action, data)
	case strings.HasPrefix(action, "playlists."):
		return h.handlePlaylists(action, data)
	case strings.HasPrefix(action, "comm."):
		return h.handleComm(action, data)
	case strings.HasPrefix(action, "jsenv."):
		return h.handleJSEnv(action, data)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// extractPermFromAction 从 action 中提取所需权限的前缀
// 例如 "songs.list" → "songs.read"，"playlists.addSongs" → "playlists.write"
func extractPermFromAction(action string) string {
	// 插件间通信映射到 inter-plugin 权限
	if strings.HasPrefix(action, "comm.") {
		return PermInterPlugin
	}

	// 歌曲相关 action 映射到具体权限
	switch action {
	case "songs.list", "songs.getById", "songs.search":
		return PermSongsRead
	case "songs.create", "songs.update", "songs.delete":
		return PermSongsWrite
	}

	// 歌单相关 action 按读写细粒度映射
	switch action {
	case "playlists.list", "playlists.getById", "playlists.getSongs", "playlists.search":
		return PermPlaylistsRead
	case "playlists.create", "playlists.update", "playlists.delete",
		"playlists.addSongs", "playlists.removeSongs", "playlists.reorder":
		return PermPlaylistsWrite
	}

	// 存储权限
	if strings.HasPrefix(action, "storage.") {
		return PermStorage
	}

	// 子 JS 环境权限（mimusic.jsenv.*）
	if strings.HasPrefix(action, "jsenv.") {
		return PermJSEnv
	}

	// 未明确分类的 action：返回原样，仅对应的通配符声明者能通过。
	// 比如未来新增但未登记到上方 switch 的 playlists.xxx，
	// 只有声明了 playlists.* 的插件会通过。
	return action
}

// handleStorage 处理存储相关的桥接调用
func (h *BridgeHandler) handleStorage(action, data string) (string, error) {
	storageDir := filepath.Join(h.dataDir, h.service.plugin.EntryPath, "data")

	switch action {
	case "storage.get":
		// data 是直接的 key 字符串
		key := data
		if err := validateStorageKey(key); err != nil {
			return "", fmt.Errorf("handleStorage: %w", err)
		}
		content, err := os.ReadFile(filepath.Join(storageDir, key))
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", fmt.Errorf("handleStorage: read file: %w", err)
		}
		return string(content), nil

	case "storage.set":
		var req struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handleStorage: parse set request: %w", err)
		}
		if err := validateStorageKey(req.Key); err != nil {
			return "", fmt.Errorf("handleStorage: %w", err)
		}
		if err := os.MkdirAll(storageDir, 0755); err != nil {
			return "", fmt.Errorf("handleStorage: create dir: %w", err)
		}
		if err := os.WriteFile(filepath.Join(storageDir, req.Key), req.Value, 0644); err != nil {
			return "", fmt.Errorf("handleStorage: write file: %w", err)
		}
		return "", nil

	case "storage.delete":
		// data 是直接的 key 字符串
		key := data
		if err := validateStorageKey(key); err != nil {
			return "", fmt.Errorf("handleStorage: %w", err)
		}
		if err := os.Remove(filepath.Join(storageDir, key)); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("handleStorage: delete file: %w", err)
		}
		return "", nil

	case "storage.keys":
		entries, err := os.ReadDir(storageDir)
		if err != nil {
			if os.IsNotExist(err) {
				return "[]", nil
			}
			return "", fmt.Errorf("handleStorage: read dir: %w", err)
		}
		keys := make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				keys = append(keys, entry.Name())
			}
		}
		result, err := json.Marshal(keys)
		if err != nil {
			return "", fmt.Errorf("handleStorage: marshal keys: %w", err)
		}
		return string(result), nil

	default:
		return "", fmt.Errorf("handleStorage: unknown action: %s", action)
	}
}

// validateStorageKey 验证存储 key 的安全性，防止目录遍历
func validateStorageKey(key string) error {
	if key == "" {
		return fmt.Errorf("storage key cannot be empty")
	}
	if strings.Contains(key, "/") || strings.Contains(key, "\\") || strings.Contains(key, "..") {
		return fmt.Errorf("storage key contains invalid characters: %q", key)
	}
	return nil
}

// handleSongs 处理歌曲相关的桥接调用
func (h *BridgeHandler) handleSongs(action, data string) (string, error) {
	ctx := context.Background()

	switch action {
	case "songs.list":
		var req struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}
		if data != "" {
			_ = json.Unmarshal([]byte(data), &req)
		}
		if req.Limit <= 0 {
			req.Limit = 20
		}
		filter := &database.SongFilter{
			Limit:  req.Limit,
			Offset: req.Offset,
		}
		songs, err := h.db.SongRepository().List(ctx, filter)
		if err != nil {
			return "", fmt.Errorf("handleSongs: list: %w", err)
		}
		result, err := json.Marshal(songs)
		if err != nil {
			return "", fmt.Errorf("handleSongs: marshal list: %w", err)
		}
		return string(result), nil

	case "songs.getById":
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handleSongs: parse getById: %w", err)
		}
		song, err := h.db.SongRepository().GetByID(ctx, req.ID)
		if err != nil {
			return "", fmt.Errorf("handleSongs: getById: %w", err)
		}
		result, err := json.Marshal(song)
		if err != nil {
			return "", fmt.Errorf("handleSongs: marshal getById: %w", err)
		}
		return string(result), nil

	case "songs.search":
		var req struct {
			Query  string `json:"query"`
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handleSongs: parse search: %w", err)
		}
		if req.Limit <= 0 {
			req.Limit = 20
		}
		filter := &database.SongFilter{
			Keyword: req.Query,
			Limit:   req.Limit,
			Offset:  req.Offset,
		}
		songs, err := h.db.SongRepository().List(ctx, filter)
		if err != nil {
			return "", fmt.Errorf("handleSongs: search: %w", err)
		}
		result, err := json.Marshal(songs)
		if err != nil {
			return "", fmt.Errorf("handleSongs: marshal search: %w", err)
		}
		return string(result), nil

	default:
		return "", fmt.Errorf("handleSongs: unknown action: %s", action)
	}
}

// handlePlaylists 处理歌单相关的桥接调用
func (h *BridgeHandler) handlePlaylists(action, data string) (string, error) {
	ctx := context.Background()

	switch action {
	case "playlists.list":
		playlists, err := h.db.PlaylistRepository().List(ctx, &database.PlaylistFilter{})
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: list: %w", err)
		}
		result, err := json.Marshal(playlists)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal list: %w", err)
		}
		return string(result), nil

	case "playlists.getById":
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse getById: %w", err)
		}
		playlist, err := h.db.PlaylistRepository().GetByID(ctx, req.ID)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: getById: %w", err)
		}
		result, err := json.Marshal(playlist)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal getById: %w", err)
		}
		return string(result), nil

	case "playlists.getSongs":
		var req struct {
			ID      int64 `json:"id"`
			Limit   int   `json:"limit"`
			Offset  int   `json:"offset"`
			Options struct {
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			} `json:"options"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse getSongs: %w", err)
		}
		// 优先使用 options 中的 limit/offset
		limit := req.Limit
		offset := req.Offset
		if req.Options.Limit > 0 {
			limit = req.Options.Limit
		}
		if req.Options.Offset > 0 {
			offset = req.Options.Offset
		}
		if limit <= 0 {
			limit = 100000
		}
		songs, err := h.db.PlaylistSongRepository().GetSongsPaginated(ctx, req.ID, limit, offset)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: getSongs: %w", err)
		}
		result, err := json.Marshal(songs)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal getSongs: %w", err)
		}
		return string(result), nil

	default:
		return "", fmt.Errorf("handlePlaylists: unknown action: %s", action)
	}
}

// handlePlugin 处理插件自身信息相关的桥接调用（无需权限检查）
func (h *BridgeHandler) handlePlugin(action, _ string) (string, error) {
	switch action {
	case "plugin.getToken":
		return h.pluginToken, nil

	case "plugin.getHostUrl":
		port := h.port
		if port == "" {
			port = "58091"
		}
		return fmt.Sprintf("http://localhost:%s", port), nil

	default:
		return "", fmt.Errorf("handlePlugin: unknown action: %s", action)
	}
}

// handleComm 处理插件间通信的桥接调用
func (h *BridgeHandler) handleComm(action, data string) (string, error) {
	comm := NewCommunicator(h.service.scheduler)
	from := h.service.plugin.EntryPath

	var req struct {
		To      string          `json:"to"`
		Action  string          `json:"action"`
		Payload json.RawMessage `json:"payload"`
		Timeout int             `json:"timeout"` // ms, only for call
	}
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return "", fmt.Errorf("parse comm request: %w", err)
	}

	switch action {
	case "comm.send":
		if err := comm.Send(from, req.To, req.Action, req.Payload); err != nil {
			return "", err
		}
		return "", nil

	case "comm.call":
		timeout := time.Duration(req.Timeout) * time.Millisecond
		if timeout <= 0 {
			timeout = DefaultCallTimeout
		}
		resp, err := comm.Call(context.Background(), from, req.To, req.Action, req.Payload, timeout)
		if err != nil {
			return "", err
		}
		resultJSON, err := json.Marshal(resp)
		if err != nil {
			return "", fmt.Errorf("marshal comm response: %w", err)
		}
		return string(resultJSON), nil

	default:
		return "", fmt.Errorf("unknown comm action: %s", action)
	}
}

// === handleJSEnv: 子 JS 环境桥接 ===
//
// JS 侧只传 plugin-local `name`，桥接端拼成 `<rootEnvID>::<name>`，pluginID 沿用 root env 的，
// 这样 DestroyPluginEnvs(pluginID) 能自动连子 env 一并回收。
//
// 错误传递哲学：与 storage/songs 不同，jsenv.* 把错误也 marshal 进 JSON 返回 ({"error":"..."})，
// 让 JS 侧能识别（__go_bridge 自身错误会被吞返回 ""）。

type jsEnvEventJSON struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type jsEnvResultJSON struct {
	Result string           `json:"result"`
	Events []jsEnvEventJSON `json:"events"`
	Error  string           `json:"error,omitempty"`
}

type jsEnvCallJSON struct {
	Name       string   `json:"name"`
	Code       string   `json:"code"`
	TimeoutMs  int64    `json:"timeoutMs"`
	WaitEvents []string `json:"waitEvents"`
}

type jsEnvParallelResultJSON struct {
	SuccessIndex int              `json:"successIndex"`
	Result       *jsEnvResultJSON `json:"result,omitempty"`
	Errors       []string         `json:"errors"`
}

// qualifyEnvName 校验并把 plugin-local name 拼成 fully-qualified envID
func (h *BridgeHandler) qualifyEnvName(name string) (string, error) {
	if name == "" || strings.Contains(name, "::") || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid env name: %q", name)
	}
	return h.service.envID + "::" + name, nil
}

// toJSONResult 将 *ExecuteResult 转成 TS 友好的 jsEnvResultJSON
func toJSONResult(r *jsruntime.ExecuteResult, errMsg string) *jsEnvResultJSON {
	out := &jsEnvResultJSON{Error: errMsg}
	if r != nil {
		out.Result = r.Result
		out.Events = make([]jsEnvEventJSON, 0, len(r.Events))
		for _, evt := range r.Events {
			out.Events = append(out.Events, jsEnvEventJSON{Name: evt.Name, Data: evt.Data})
		}
	}
	if out.Events == nil {
		out.Events = []jsEnvEventJSON{}
	}
	return out
}

// marshalJSONOrErr 将任意结构 marshal 为字符串，marshal 失败时返回 ("", err)
func marshalJSONOrErr(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal jsenv response: %w", err)
	}
	return string(b), nil
}

func (h *BridgeHandler) handleJSEnv(action, data string) (string, error) {
	mgr := h.service.jsManager
	pluginID := h.service.plugin.ID

	switch action {
	case "jsenv.create":
		var req struct {
			Name     string `json:"name"`
			InitCode string `json:"initCode"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return marshalJSONOrErr(map[string]string{"error": "parse request: " + err.Error()})
		}
		fullID, err := h.qualifyEnvName(req.Name)
		if err != nil {
			return marshalJSONOrErr(map[string]string{"error": err.Error()})
		}
		if err := mgr.CreateEnv(fullID, req.InitCode, pluginID); err != nil {
			return marshalJSONOrErr(map[string]string{"error": err.Error()})
		}
		return marshalJSONOrErr(map[string]string{"envName": req.Name})

	case "jsenv.execute":
		var req jsEnvCallJSON
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return marshalJSONOrErr(toJSONResult(nil, "parse request: "+err.Error()))
		}
		fullID, err := h.qualifyEnvName(req.Name)
		if err != nil {
			return marshalJSONOrErr(toJSONResult(nil, err.Error()))
		}
		res, err := mgr.ExecuteJS(fullID, req.Code, req.TimeoutMs)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		return marshalJSONOrErr(toJSONResult(res, errMsg))

	case "jsenv.executeWait":
		var req jsEnvCallJSON
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return marshalJSONOrErr(toJSONResult(nil, "parse request: "+err.Error()))
		}
		fullID, err := h.qualifyEnvName(req.Name)
		if err != nil {
			return marshalJSONOrErr(toJSONResult(nil, err.Error()))
		}
		res, err := mgr.ExecuteJSAndWaitEvents(fullID, req.Code, req.TimeoutMs, req.WaitEvents)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		return marshalJSONOrErr(toJSONResult(res, errMsg))

	case "jsenv.executeParallel":
		var req struct {
			Calls         []jsEnvCallJSON `json:"calls"`
			MaxConcurrent int             `json:"maxConcurrent"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return marshalJSONOrErr(jsEnvParallelResultJSON{SuccessIndex: -1, Errors: []string{"parse request: " + err.Error()}})
		}
		calls := make([]jsruntime.ParallelCall, 0, len(req.Calls))
		for _, c := range req.Calls {
			fullID, err := h.qualifyEnvName(c.Name)
			if err != nil {
				return marshalJSONOrErr(jsEnvParallelResultJSON{SuccessIndex: -1, Errors: []string{err.Error()}})
			}
			calls = append(calls, jsruntime.ParallelCall{
				EnvID:          fullID,
				Code:           c.Code,
				TimeoutMs:      c.TimeoutMs,
				WaitEventNames: c.WaitEvents,
			})
		}
		idx, res, errs := mgr.ExecuteJSParallel(calls, req.MaxConcurrent)
		out := jsEnvParallelResultJSON{SuccessIndex: idx, Errors: errs}
		if errs == nil {
			out.Errors = []string{}
		}
		if res != nil {
			out.Result = toJSONResult(res, "")
		}
		return marshalJSONOrErr(out)

	case "jsenv.destroy":
		var req struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", nil // 忽略 parse 错误，destroy 是 best-effort
		}
		fullID, err := h.qualifyEnvName(req.Name)
		if err != nil {
			return "", nil
		}
		_ = mgr.DestroyEnv(fullID) // not-found 不报错
		return "", nil

	case "jsenv.list":
		// 列出本插件所有子 env 的 plugin-local name（去掉 prefix）
		prefix := h.service.envID + "::"
		// 这里没有公开 API 可枚举 envIDs；通过反向遍历 pluginEnvs 不可达。
		// 折中：返回空列表（list 是 optional），未来可加 mgr.ListEnvsByPlugin(pluginID)。
		// 占位实现：返回 []。
		_ = prefix
		return "[]", nil

	default:
		return marshalJSONOrErr(map[string]string{"error": "unknown jsenv action: " + action})
	}
}
