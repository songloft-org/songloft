package jsplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"songloft/internal/database"
	"songloft/internal/jsruntime"
	"songloft/internal/models"
	"songloft/internal/services"
)

// pluginBootstrapJS 是注入到每个插件 JS 环境的引导代码
// 定义了 songloft 全局对象的基础结构和生命周期回调桩。
//
// 设计原则：所有 __go_bridge 调用统一通过 __callBridge（polyfill 提供）
// 包装为 Promise<string>，songloft.* 接口对外都返回 Promise，
// 调用方必须 `await`。这与 fetch / setTimeout 等 Web 标准 API 行为一致，
// 也是 JS 接口"真异步"的核心契约。
const pluginBootstrapJS = `
// Songloft JS Plugin API 基础框架
var songloft = songloft || {};

// 插件生命周期默认空实现（插件代码通过 globalThis.xxx = ... 覆盖）。
// 注意：必须使用 globalThis 赋值而非 function 声明，
// 否则 QuickJS 中 Annex B 块级函数声明会创建 declarative binding，
// 导致插件 IIFE 中 globalThis.onHTTPRequest = ... 无法覆盖。
//
// onHTTPRequest 默认实现为 async：与异步 songloft.* API 自然组合。
if (typeof globalThis.onInit !== 'function') { globalThis.onInit = async function() {}; }
if (typeof globalThis.onDeinit !== 'function') { globalThis.onDeinit = async function() {}; }
if (typeof globalThis.onHTTPRequest !== 'function') {
    globalThis.onHTTPRequest = async function(req) {
        return { statusCode: 404, headers: {}, body: 'not implemented' };
    };
}

// 事件订阅（动态注册/取消注册，可在任意时刻调用）
songloft.events = {
    onPlayEvent: function(fn) {
        if (typeof fn === 'function') {
            globalThis.onPlayEvent = fn;
            __callBridge('plugin.registerPlayEvent', '');
        }
    },
    offPlayEvent: function() {
        globalThis.onPlayEvent = undefined;
        __callBridge('plugin.unregisterPlayEvent', '');
    }
};

// 日志（同步本地操作，无需 await）
songloft.log = {
    info: function(msg) { console.log('[plugin] ' + msg); },
    warn: function(msg) { console.warn('[plugin] ' + msg); },
    error: function(msg) { console.error('[plugin] ' + msg); }
};

// === songloft.storage（async）===
songloft.storage = {
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

// === songloft.persistentStorage（async）===
songloft.persistentStorage = {
    get: async function(key) {
        var s = await __callBridge('persistent-storage.get', key);
        return s ? JSON.parse(s) : null;
    },
    set: async function(key, value) {
        await __callBridge('persistent-storage.set', JSON.stringify({key: key, value: value}));
    },
    delete: async function(key) {
        await __callBridge('persistent-storage.delete', key);
    },
    keys: async function() {
        var s = await __callBridge('persistent-storage.keys', '');
        return s ? JSON.parse(s) : [];
    }
};

// === songloft.songs（async）===
songloft.songs = {
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
    },
    download: async function(songId, options) {
        var data = JSON.stringify(Object.assign({song_id: songId}, options || {}));
        var s = await __callBridge('songs.download', data);
        return s ? JSON.parse(s) : null;
    },
    create: async function(songs) {
        var s = await __callBridge('songs.create', JSON.stringify({songs: songs || []}));
        return s ? JSON.parse(s) : [];
    },
    update: async function(id, fields) {
        var data = JSON.stringify(Object.assign({id: id}, fields || {}));
        var s = await __callBridge('songs.update', data);
        return s ? JSON.parse(s) : null;
    },
    delete: async function(id) {
        await __callBridge('songs.delete', JSON.stringify({id: id}));
    },
    setAutoDownload: async function(options) {
        await __callBridge('songs.setAutoDownload', JSON.stringify(options || {}));
    }
};

// === songloft.playlists（async）===
songloft.playlists = {
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
    },
    search: async function(query, options) {
        var s = await __callBridge('playlists.search', JSON.stringify({query: query, limit: (options && options.limit) || 0, offset: (options && options.offset) || 0}));
        return s ? JSON.parse(s) : [];
    },
    create: async function(playlist) {
        var s = await __callBridge('playlists.create', JSON.stringify(playlist || {}));
        return s ? JSON.parse(s) : null;
    },
    update: async function(id, fields) {
        var data = JSON.stringify(Object.assign({id: id}, fields || {}));
        var s = await __callBridge('playlists.update', data);
        return s ? JSON.parse(s) : null;
    },
    delete: async function(id) {
        await __callBridge('playlists.delete', JSON.stringify({id: id}));
    },
    addSongs: async function(id, songIds) {
        var s = await __callBridge('playlists.addSongs', JSON.stringify({id: id, songIds: songIds || []}));
        return s ? JSON.parse(s) : {added: 0, skipped: 0};
    },
    removeSongs: async function(id, songIds) {
        await __callBridge('playlists.removeSongs', JSON.stringify({id: id, songIds: songIds || []}));
    },
    reorder: async function(id, songIds) {
        await __callBridge('playlists.reorder', JSON.stringify({id: id, songIds: songIds || []}));
    }
};

// === songloft.plugin（async）===
// 即使 getToken/getHostUrl 内部是 O(1) 的内存读取，也统一返回 Promise，
// 保证 songloft.* API 表面一致；插件代码用 const t = await songloft.plugin.getToken()。
songloft.plugin = {
    getToken: async function() {
        return await __callBridge('plugin.getToken', '');
    },
    getHostUrl: async function() {
        return await __callBridge('plugin.getHostUrl', '');
    },
    getFileUrl: async function(filePath) {
        var r = await __callBridge('plugin.getFileUrl', JSON.stringify({filePath: filePath}));
        return JSON.parse(r).url;
    },
    getNetworkAddresses: async function() {
        var r = await __callBridge('plugin.getNetworkAddresses', '');
        return r ? JSON.parse(r) : [];
    }
};

// === songloft.jsenv（async）===
// 子 JS 环境（独立 QuickJS VM）：用于在插件内创建隔离的 sandbox 跑用户脚本，
// 跨 env 真并行（ExecuteJSParallel），生命周期受 pluginID 管理（DestroyPluginEnvs 自动回收）。
songloft.jsenv = {
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

// === songloft.command（async）===
songloft.command = {
    exec: async function(program, args, options) {
        var s = await __callBridge('command.exec', JSON.stringify({
            program: program, args: args || [], timeout: (options && options.timeout) || 0,
            stdin: (options && options.stdin) || '', env: (options && options.env) || {}
        }));
        return s ? JSON.parse(s) : {exitCode: -1, stdout: '', stderr: ''};
    },
    start: async function(name, program, args, options) {
        var s = await __callBridge('command.start', JSON.stringify({
            name: name, program: program, args: args || [], env: (options && options.env) || {}
        }));
        return s ? JSON.parse(s) : {};
    },
    stop: async function(name) {
        await __callBridge('command.stop', JSON.stringify({name: name}));
    },
    isRunning: async function(name) {
        var s = await __callBridge('command.isRunning', JSON.stringify({name: name}));
        return s === 'true';
    },
    download: async function(url, filename, options) {
        var payload = {url: url, filename: filename};
        if (options) {
            if (options.extract) payload.extract = options.extract;
            if (options.extractTarget) payload.extractTarget = options.extractTarget;
        }
        await __callBridge('command.download', JSON.stringify(payload));
    },
    deleteBin: async function(filename) {
        await __callBridge('command.deleteBin', filename);
    },
    listBin: async function() {
        var s = await __callBridge('command.listBin', '');
        return s ? JSON.parse(s) : [];
    },
    exists: async function(filename) {
        var s = await __callBridge('command.exists', filename);
        return s === 'true';
    }
};
songloft.fs = {
    readFile: async function(path, options) {
        var enc = (options && options.encoding) || 'utf8';
        return await __callBridge('fs.readFile', JSON.stringify({path: path, encoding: enc}));
    },
    writeFile: async function(path, data, options) {
        var enc = (options && options.encoding) || 'utf8';
        await __callBridge('fs.writeFile', JSON.stringify({path: path, data: data, encoding: enc}));
    },
    appendFile: async function(path, data, options) {
        var enc = (options && options.encoding) || 'utf8';
        await __callBridge('fs.appendFile', JSON.stringify({path: path, data: data, encoding: enc}));
    },
    readdir: async function(path) {
        var s = await __callBridge('fs.readdir', JSON.stringify({path: path}));
        return s ? JSON.parse(s) : [];
    },
    unlink: async function(path) {
        await __callBridge('fs.unlink', JSON.stringify({path: path}));
    },
    exists: async function(path) {
        var s = await __callBridge('fs.exists', JSON.stringify({path: path}));
        return s === 'true';
    },
    mkdir: async function(path, options) {
        var recursive = (options && options.recursive) || false;
        await __callBridge('fs.mkdir', JSON.stringify({path: path, recursive: recursive}));
    },
    stat: async function(path) {
        var s = await __callBridge('fs.stat', JSON.stringify({path: path}));
        return JSON.parse(s);
    },
    rename: async function(oldPath, newPath) {
        await __callBridge('fs.rename', JSON.stringify({oldPath: oldPath, newPath: newPath}));
    }
};

// === songloft.lyrics — 歌词提供者注册 ===
songloft.lyrics = {
    registerProvider: function() {
        __callBridge('plugin.registerLyricProvider', '');
    },
    unregisterProvider: function() {
        __callBridge('plugin.unregisterLyricProvider', '');
    }
};

// === songloft.net — 网络 socket（UDP）===
songloft.net = {
    _handlers: {},
    udpBind: async function(options) {
        var s = await __callBridge('net.udpBind', JSON.stringify(options || {}));
        return s ? JSON.parse(s) : {};
    },
    udpSend: async function(socketId, data, addr) {
        await __callBridge('net.udpSend', JSON.stringify({socketId: socketId, data: btoa(data), addr: addr}));
    },
    udpJoinMulticast: async function(socketId, group) {
        await __callBridge('net.udpJoinMulticast', JSON.stringify({socketId: socketId, group: group}));
    },
    udpLeaveMulticast: async function(socketId, group) {
        await __callBridge('net.udpLeaveMulticast', JSON.stringify({socketId: socketId, group: group}));
    },
    udpGetLocalAddr: async function(socketId) {
        var s = await __callBridge('net.udpGetLocalAddr', JSON.stringify({socketId: socketId}));
        return s ? JSON.parse(s) : {};
    },
    udpClose: async function(socketId) {
        await __callBridge('net.udpClose', JSON.stringify({socketId: socketId}));
        delete songloft.net._handlers[socketId];
    },
    onData: function(socketId, handler) {
        songloft.net._handlers[socketId] = handler;
    }
};
`

// GetBootstrapCode 返回插件引导 JS 代码（含通信 API）
func GetBootstrapCode() string {
	return pluginBootstrapJS + GenerateCommJS()
}

// BridgeHandler 处理 JS 通过 __go_bridge 调用的请求
type BridgeHandler struct {
	service                   *JSService
	permissions               []string
	dataDir                   string                    // data/jsplugins_data/
	db                        database.DB               // 数据库访问（用于 songs/playlists 查询）
	songDownloader            *services.SongDownloader  // 歌曲下载服务（songs.download bridge 调用）
	songService               *services.SongService     // 歌曲服务（songs.create/update/delete）
	playlistService           *services.PlaylistService // 歌单服务（playlists 写操作）
	pluginToken               string                    // 插件专用的永久 JWT Token
	port                      string                    // 服务器监听端口（用于构造宿主 URL）
	processes                 sync.Map                  // map[name]*managedProcess — 后台进程跟踪
	udpSockets                sync.Map                  // map[socketID]*managedUDPSocket — UDP socket 跟踪
	socketIDSeq               atomic.Uint64             // UDP socket ID 递增序号
	onPlayEventRegister       func(entryPath string)    // 播放事件订阅回调
	onPlayEventUnregister     func(entryPath string)    // 播放事件取消订阅回调
	onLyricProviderRegister   func(entryPath string)    // 歌词提供者注册回调
	onLyricProviderUnregister func(entryPath string)    // 歌词提供者取消注册回调
}

// NewBridgeHandler 创建桥接处理器
func NewBridgeHandler(service *JSService, dataDir string, db database.DB, songDownloader *services.SongDownloader, songService *services.SongService, playlistService *services.PlaylistService, pluginToken string, port string) *BridgeHandler {
	return &BridgeHandler{
		service:         service,
		permissions:     service.plugin.Permissions,
		dataDir:         dataDir,
		db:              db,
		songDownloader:  songDownloader,
		songService:     songService,
		playlistService: playlistService,
		pluginToken:     pluginToken,
		port:            port,
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

	// fs.* 操作的权限按路径类型在 resolveFSPath 内细粒度检查，跳过通用检查
	if !strings.HasPrefix(action, "fs.") {
		requiredPerm := extractPermFromAction(action)
		if !CheckPermission(h.permissions, requiredPerm) {
			return "", fmt.Errorf("permission denied: requires '%s'", requiredPerm)
		}
	}

	// 分发到具体处理器
	switch {
	case strings.HasPrefix(action, "persistent-storage."):
		return h.handlePersistentStorage(action, data)
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
	case strings.HasPrefix(action, "command."):
		return h.handleCommand(action, data)
	case strings.HasPrefix(action, "fs."):
		return h.handleFS(action, data)
	case strings.HasPrefix(action, "net."):
		return h.handleNet(action, data)
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
	case "songs.create", "songs.update", "songs.delete", "songs.download", "songs.setAutoDownload":
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
	if strings.HasPrefix(action, "persistent-storage.") {
		return PermPersistentStorage
	}
	if strings.HasPrefix(action, "storage.") {
		return PermStorage
	}

	// 子 JS 环境权限（songloft.jsenv.*）
	if strings.HasPrefix(action, "jsenv.") {
		return PermJSEnv
	}

	// 命令执行权限（songloft.command.*）
	if strings.HasPrefix(action, "command.") {
		return PermCommand
	}

	// 文件系统权限（songloft.fs.*）
	if strings.HasPrefix(action, "fs.") {
		return PermFS
	}

	// 网络 socket 权限（songloft.net.*）
	if strings.HasPrefix(action, "net.") {
		return PermNet
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

const maxPersistentStorageBytes int64 = 10 << 20 // 10MB per plugin

func (h *BridgeHandler) handlePersistentStorage(action, data string) (string, error) {
	ctx := context.Background()
	repo := h.db.PluginStorageRepository()
	entryPath := h.service.plugin.EntryPath

	switch action {
	case "persistent-storage.get":
		key := data
		if err := validateStorageKey(key); err != nil {
			return "", fmt.Errorf("handlePersistentStorage: %w", err)
		}
		value, err := repo.Get(ctx, entryPath, key)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return "", nil
			}
			return "", fmt.Errorf("handlePersistentStorage: %w", err)
		}
		return value, nil

	case "persistent-storage.set":
		var req struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePersistentStorage: parse set request: %w", err)
		}
		if err := validateStorageKey(req.Key); err != nil {
			return "", fmt.Errorf("handlePersistentStorage: %w", err)
		}
		totalSize, err := repo.TotalSize(ctx, entryPath)
		if err != nil {
			return "", fmt.Errorf("handlePersistentStorage: check quota: %w", err)
		}
		newValueSize := int64(len(req.Value))
		if totalSize+newValueSize > maxPersistentStorageBytes {
			return "", fmt.Errorf("handlePersistentStorage: storage quota exceeded (limit %dMB)", maxPersistentStorageBytes>>20)
		}
		if err := repo.Set(ctx, entryPath, req.Key, string(req.Value)); err != nil {
			return "", fmt.Errorf("handlePersistentStorage: %w", err)
		}
		return "", nil

	case "persistent-storage.delete":
		key := data
		if err := validateStorageKey(key); err != nil {
			return "", fmt.Errorf("handlePersistentStorage: %w", err)
		}
		if err := repo.Delete(ctx, entryPath, key); err != nil && !errors.Is(err, database.ErrNotFound) {
			return "", fmt.Errorf("handlePersistentStorage: %w", err)
		}
		return "", nil

	case "persistent-storage.keys":
		keys, err := repo.Keys(ctx, entryPath)
		if err != nil {
			return "", fmt.Errorf("handlePersistentStorage: %w", err)
		}
		result, err := json.Marshal(keys)
		if err != nil {
			return "", fmt.Errorf("handlePersistentStorage: marshal keys: %w", err)
		}
		return string(result), nil

	default:
		return "", fmt.Errorf("handlePersistentStorage: unknown action: %s", action)
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
			PathPrefix string `json:"pathPrefix"`
			Type       string `json:"type"`
			OrderBy    string `json:"orderBy"`
			Order      string `json:"order"`
			Limit      int    `json:"limit"`
			Offset     int    `json:"offset"`
		}
		if data != "" {
			_ = json.Unmarshal([]byte(data), &req)
		}
		if req.Limit <= 0 {
			req.Limit = 20
		}
		filter := &database.SongFilter{
			PathPrefix: req.PathPrefix,
			Type:       req.Type,
			OrderBy:    req.OrderBy,
			Order:      req.Order,
			Limit:      req.Limit,
			Offset:     req.Offset,
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

	case "songs.download":
		if h.songDownloader == nil {
			return "", fmt.Errorf("handleSongs: download service not configured")
		}
		var req struct {
			SongID        int64  `json:"song_id"`
			TargetDir     string `json:"target_dir"`
			PathTemplate  string `json:"path_template"`
			EmbedMetadata *bool  `json:"embed_metadata"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handleSongs: parse download: %w", err)
		}
		embedMeta := true
		if req.EmbedMetadata != nil {
			embedMeta = *req.EmbedMetadata
		}
		dlResult, err := h.songDownloader.Download(ctx, req.SongID, services.SongDownloadOptions{
			TargetDir:     req.TargetDir,
			PathTemplate:  req.PathTemplate,
			EmbedMetadata: embedMeta,
		})
		if err != nil {
			return "", fmt.Errorf("handleSongs: download: %w", err)
		}
		result, err := json.Marshal(dlResult)
		if err != nil {
			return "", fmt.Errorf("handleSongs: marshal download: %w", err)
		}
		return string(result), nil

	case "songs.setAutoDownload":
		if h.songDownloader == nil {
			return "", fmt.Errorf("handleSongs: download service not configured")
		}
		var config services.AutoDownloadConfig
		if err := json.Unmarshal([]byte(data), &config); err != nil {
			return "", fmt.Errorf("handleSongs: parse auto-download config: %w", err)
		}
		h.songDownloader.SetAutoDownloadConfig(&config)
		return "", nil

	case "songs.create":
		if h.songService == nil {
			return "", fmt.Errorf("handleSongs: song service not configured")
		}
		var req struct {
			Songs []struct {
				URL            string  `json:"url"`
				Title          string  `json:"title"`
				Artist         string  `json:"artist"`
				Album          string  `json:"album"`
				CoverURL       string  `json:"coverUrl"`
				Duration       float64 `json:"duration"`
				SourceData     string  `json:"sourceData"`
				DedupKey       string  `json:"dedupKey"`
				Lyric          string  `json:"lyric"`
				LyricSource    string  `json:"lyricSource"`
				LyricRemoteURL string  `json:"lyricRemoteUrl"`
			} `json:"songs"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handleSongs: parse create: %w", err)
		}
		inputs := make([]services.RemoteSongInput, len(req.Songs))
		for i, s := range req.Songs {
			inputs[i] = services.RemoteSongInput{
				URL:             s.URL,
				Title:           s.Title,
				Artist:          s.Artist,
				Album:           s.Album,
				CoverURL:        s.CoverURL,
				Duration:        s.Duration,
				PluginEntryPath: h.service.plugin.EntryPath,
				SourceData:      s.SourceData,
				DedupKey:        s.DedupKey,
				Lyric:           s.Lyric,
				LyricSource:     s.LyricSource,
				LyricRemoteURL:  s.LyricRemoteURL,
			}
		}
		songs, err := h.songService.AddRemoteSongs(ctx, inputs)
		if err != nil {
			return "", fmt.Errorf("handleSongs: create: %w", err)
		}
		result, err := json.Marshal(songs)
		if err != nil {
			return "", fmt.Errorf("handleSongs: marshal create: %w", err)
		}
		return string(result), nil

	case "songs.update":
		if h.songService == nil {
			return "", fmt.Errorf("handleSongs: song service not configured")
		}
		var req struct {
			ID       int64    `json:"id"`
			Title    *string  `json:"title"`
			Artist   *string  `json:"artist"`
			Album    *string  `json:"album"`
			URL      *string  `json:"url"`
			CoverURL *string  `json:"coverUrl"`
			Duration *float64 `json:"duration"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handleSongs: parse update: %w", err)
		}
		song, err := h.db.SongRepository().GetByID(ctx, req.ID)
		if err != nil {
			return "", fmt.Errorf("handleSongs: update getById: %w", err)
		}
		if req.Title != nil {
			song.Title = *req.Title
		}
		if req.Artist != nil {
			song.Artist = *req.Artist
		}
		if req.Album != nil {
			song.Album = *req.Album
		}
		if req.URL != nil {
			song.URL = *req.URL
		}
		if req.CoverURL != nil {
			song.CoverURL = *req.CoverURL
		}
		if req.Duration != nil {
			song.Duration = *req.Duration
		}
		song.UpdatedAt = time.Now()
		if err := h.songService.Update(ctx, song); err != nil {
			return "", fmt.Errorf("handleSongs: update: %w", err)
		}
		result, err := json.Marshal(song)
		if err != nil {
			return "", fmt.Errorf("handleSongs: marshal update: %w", err)
		}
		return string(result), nil

	case "songs.delete":
		if h.songService == nil {
			return "", fmt.Errorf("handleSongs: song service not configured")
		}
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handleSongs: parse delete: %w", err)
		}
		if err := h.songService.Delete(ctx, req.ID, false); err != nil {
			return "", fmt.Errorf("handleSongs: delete: %w", err)
		}
		return "", nil

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
		slog.Info("bridge playlists.getSongs",
			"playlistID", req.ID, "limit", limit, "offset", offset, "count", len(songs))
		result, err := json.Marshal(songs)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal getSongs: %w", err)
		}
		return string(result), nil

	case "playlists.search":
		var req struct {
			Query  string `json:"query"`
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse search: %w", err)
		}
		if req.Limit <= 0 {
			req.Limit = 20
		}
		filter := &database.PlaylistFilter{
			Keyword: req.Query,
			Limit:   req.Limit,
			Offset:  req.Offset,
		}
		playlists, err := h.db.PlaylistRepository().List(ctx, filter)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: search: %w", err)
		}
		result, err := json.Marshal(playlists)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal search: %w", err)
		}
		return string(result), nil

	case "playlists.create":
		if h.playlistService == nil {
			return "", fmt.Errorf("handlePlaylists: playlist service not configured")
		}
		var req struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
			CoverURL    string `json:"coverUrl"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse create: %w", err)
		}
		playlist := &models.Playlist{
			Name:        req.Name,
			Type:        req.Type,
			Description: req.Description,
			CoverURL:    req.CoverURL,
		}
		if playlist.Type == "" {
			playlist.Type = "normal"
		}
		if err := h.playlistService.Create(ctx, playlist); err != nil {
			return "", fmt.Errorf("handlePlaylists: create: %w", err)
		}
		result, err := json.Marshal(playlist)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal create: %w", err)
		}
		return string(result), nil

	case "playlists.update":
		if h.playlistService == nil {
			return "", fmt.Errorf("handlePlaylists: playlist service not configured")
		}
		var req struct {
			ID          int64   `json:"id"`
			Name        *string `json:"name"`
			Description *string `json:"description"`
			CoverURL    *string `json:"coverUrl"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse update: %w", err)
		}
		playlist, err := h.db.PlaylistRepository().GetByID(ctx, req.ID)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: update getById: %w", err)
		}
		if req.Name != nil {
			playlist.Name = *req.Name
		}
		if req.Description != nil {
			playlist.Description = *req.Description
		}
		if req.CoverURL != nil {
			playlist.CoverURL = *req.CoverURL
		}
		if err := h.playlistService.Update(ctx, playlist); err != nil {
			return "", fmt.Errorf("handlePlaylists: update: %w", err)
		}
		result, err := json.Marshal(playlist)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal update: %w", err)
		}
		return string(result), nil

	case "playlists.delete":
		if h.playlistService == nil {
			return "", fmt.Errorf("handlePlaylists: playlist service not configured")
		}
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse delete: %w", err)
		}
		if err := h.playlistService.Delete(ctx, req.ID); err != nil {
			return "", fmt.Errorf("handlePlaylists: delete: %w", err)
		}
		return "", nil

	case "playlists.addSongs":
		if h.playlistService == nil {
			return "", fmt.Errorf("handlePlaylists: playlist service not configured")
		}
		var req struct {
			ID      int64   `json:"id"`
			SongIDs []int64 `json:"songIds"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse addSongs: %w", err)
		}
		added, skipped, err := h.playlistService.AddSongs(ctx, req.ID, req.SongIDs)
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: addSongs: %w", err)
		}
		result, err := json.Marshal(map[string]int{"added": added, "skipped": skipped})
		if err != nil {
			return "", fmt.Errorf("handlePlaylists: marshal addSongs: %w", err)
		}
		return string(result), nil

	case "playlists.removeSongs":
		if h.playlistService == nil {
			return "", fmt.Errorf("handlePlaylists: playlist service not configured")
		}
		var req struct {
			ID      int64   `json:"id"`
			SongIDs []int64 `json:"songIds"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse removeSongs: %w", err)
		}
		for _, songID := range req.SongIDs {
			if err := h.playlistService.RemoveSong(ctx, req.ID, songID); err != nil {
				if errors.Is(err, database.ErrNotFound) {
					continue
				}
				return "", fmt.Errorf("handlePlaylists: removeSongs (songId=%d): %w", songID, err)
			}
		}
		return "", nil

	case "playlists.reorder":
		if h.playlistService == nil {
			return "", fmt.Errorf("handlePlaylists: playlist service not configured")
		}
		var req struct {
			ID      int64   `json:"id"`
			SongIDs []int64 `json:"songIds"`
		}
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			return "", fmt.Errorf("handlePlaylists: parse reorder: %w", err)
		}
		if err := h.playlistService.ReorderSongs(ctx, req.ID, req.SongIDs); err != nil {
			return "", fmt.Errorf("handlePlaylists: reorder: %w", err)
		}
		return "", nil

	default:
		return "", fmt.Errorf("handlePlaylists: unknown action: %s", action)
	}
}

// handlePlugin 处理插件自身信息相关的桥接调用（无需权限检查）
func (h *BridgeHandler) handlePlugin(action, data string) (string, error) {
	switch action {
	case "plugin.getToken":
		return h.pluginToken, nil

	case "plugin.getHostUrl":
		port := h.port
		if port == "" {
			port = "58091"
		}
		return fmt.Sprintf("http://localhost:%s", port), nil

	case "plugin.getNetworkAddresses":
		port := h.port
		if port == "" {
			port = "58091"
		}
		return getNetworkAddresses(port)

	case "plugin.getFileUrl":
		var req struct {
			FilePath string `json:"filePath"`
		}
		if data != "" {
			_ = json.Unmarshal([]byte(data), &req)
		}
		if req.FilePath == "" {
			return "", fmt.Errorf("plugin.getFileUrl: filePath is required")
		}
		url := fmt.Sprintf("/api/v1/jsplugin/%s/files/%s?access_token=%s",
			h.service.plugin.EntryPath, req.FilePath, h.pluginToken)
		return fmt.Sprintf(`{"url":%q}`, url), nil

	case "plugin.registerPlayEvent":
		if h.onPlayEventRegister != nil {
			h.onPlayEventRegister(h.service.plugin.EntryPath)
		}
		return "", nil

	case "plugin.unregisterPlayEvent":
		if h.onPlayEventUnregister != nil {
			h.onPlayEventUnregister(h.service.plugin.EntryPath)
		}
		return "", nil

	case "plugin.registerLyricProvider":
		if h.onLyricProviderRegister != nil {
			h.onLyricProviderRegister(h.service.plugin.EntryPath)
		}
		return "", nil

	case "plugin.unregisterLyricProvider":
		if h.onLyricProviderUnregister != nil {
			h.onLyricProviderUnregister(h.service.plugin.EntryPath)
		}
		return "", nil

	default:
		return "", fmt.Errorf("handlePlugin: unknown action: %s", action)
	}
}

// getNetworkAddresses 返回本机局域网可达的 IPv4 地址列表，192.168.x.x 优先。
// 过滤规则：排除回环、IPv6、Docker 虚拟网桥接口、172.16.0.0/12 网段（Docker 默认使用）。
func getNetworkAddresses(port string) (string, error) {
	// 172.16.0.0/12 几乎全被 Docker bridge 占用，排除
	docker12 := net.IPNet{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "[]", nil
	}

	var preferred, others []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "virbr") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || docker12.Contains(ip) {
				continue
			}
			url := fmt.Sprintf("http://%s:%s", ip.String(), port)
			if strings.HasPrefix(ip.String(), "192.168.") {
				preferred = append(preferred, url)
			} else {
				others = append(others, url)
			}
		}
	}

	result := append(preferred, others...)
	data, _ := json.Marshal(result)
	return string(data), nil
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
		// 插件主动调用 jsenv.execute（如插件多 worker 协作），不接受外部取消
		res, err := mgr.ExecuteJS(context.Background(), fullID, req.Code, req.TimeoutMs)
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
		res, err := mgr.ExecuteJSAndWaitEvents(context.Background(), fullID, req.Code, req.TimeoutMs, req.WaitEvents)
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
