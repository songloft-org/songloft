package jsplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"songloft/internal/jsruntime"
)

// ServiceStatus 服务运行状态
type ServiceStatus int

const (
	ServiceStatusReady   ServiceStatus = iota // 就绪（已加载，可接收消息）
	ServiceStatusRunning                      // 处理中
	ServiceStatusFrozen                       // 冻结（热更新期间）
	ServiceStatusStopped                      // 已停止
)

// String 返回状态的字符串表示
func (s ServiceStatus) String() string {
	switch s {
	case ServiceStatusReady:
		return "ready"
	case ServiceStatusRunning:
		return "running"
	case ServiceStatusFrozen:
		return "frozen"
	case ServiceStatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// HTTPRequestData 是 MsgHTTPRequest 消息的 Data 类型
type HTTPRequestData struct {
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Headers      map[string]string `json:"headers"`
	Body         string            `json:"body"`
	Query        string            `json:"query"`
	BodyEncoding string            `json:"bodyEncoding,omitempty"` // "base64" 当 body 含非 UTF-8 二进制数据时
}

// HTTPResponseData 是 HTTP 请求响应的 Data 类型
type HTTPResponseData struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string]string   `json:"headers"`
	Body       string              `json:"body"`
	ServeFile  *ServeFileDirective `json:"serveFile,omitempty"`
}

// PlayEventData 是 MsgPlayEvent 消息的 Data 类型
type PlayEventData struct {
	Type      string        `json:"type"`
	Song      PlayEventSong `json:"song"`
	Source    string        `json:"source"`
	Timestamp int64         `json:"timestamp"`
}

// PlayEventSong 包含播放事件中的歌曲信息
type PlayEventSong struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// ServeFileDirective 指示 Go 层直接 serve 文件（绕过 QuickJS string 管道）。
// JS 做业务决策（认证、路由），Go 做文件 I/O（零拷贝、Range、HTTP 缓存）。
type ServeFileDirective struct {
	SongID   int64  `json:"songId,omitempty"`   // serve 系统内歌曲（需 songs.read 权限）
	FilePath string `json:"filePath,omitempty"` // serve 文件（路径解析规则见 resolveServeFilePath）
}

// NetDataEvent 是 UDP readLoop 推送给 JS 的数据事件。
type NetDataEvent struct {
	SocketID   string `json:"socketId"`
	Data       string `json:"data"`       // base64 编码的原始数据
	RemoteAddr string `json:"remoteAddr"` // "ip:port"
}

// WebSocketRequestData 是入站 WebSocket 握手请求的轻量表示。
type WebSocketRequestData struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Query      string            `json:"query"`
	RemoteAddr string            `json:"remoteAddr"`
}

// WebSocketOpenData 是 MsgWebSocketOpen 消息的 Data 类型。
type WebSocketOpenData struct {
	ConnID  string                `json:"connId"`
	Request *WebSocketRequestData `json:"request"`
}

// WebSocketMessageData 是入站 WebSocket 收到消息时推送给 JS 的事件。
type WebSocketMessageData struct {
	ConnID   string `json:"connId"`
	DataHex  string `json:"dataHex"` // 原始消息字节的 hex 编码
	IsBinary bool   `json:"isBinary"`
}

// WebSocketCloseData 是入站 WebSocket 关闭时推送给 JS 的事件。
type WebSocketCloseData struct {
	ConnID   string `json:"connId"`
	Code     int    `json:"code"`
	Reason   string `json:"reason"`
	WasClean bool   `json:"wasClean"`
}

// JSService 代表一个运行中的 JS 插件实例（Actor）
type JSService struct {
	plugin        *JSPlugin               // 插件元数据
	envID         string                  // jsruntime 环境 ID
	scheduler     *ServiceScheduler       // 所属调度器
	jsManager     *jsruntime.JSEnvManager // JS 运行时管理器
	bridgeHandler *BridgeHandler          // 桥接处理器
	status        ServiceStatus           // 运行状态
	mu            sync.RWMutex
	lastActive    time.Time     // 最后活跃时间
	timerStop     chan struct{} // 定时器 goroutine 停止信号
}

// NewJSService 创建新的 JS 服务实例
func NewJSService(plugin *JSPlugin, scheduler *ServiceScheduler, jsManager *jsruntime.JSEnvManager) *JSService {
	return &JSService{
		plugin:     plugin,
		scheduler:  scheduler,
		jsManager:  jsManager,
		status:     ServiceStatusStopped,
		lastActive: time.Now(),
	}
}

// Load 加载插件（双层 hash 校验 + 从 ZIP 读取代码 + 创建 JS 环境）
// pluginsDir: data/jsplugins/ 目录路径
// dataDir: data/jsplugins_data/ 目录路径
func (s *JSService) Load(pluginsDir, dataDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// [1] 读取 ZIP 文件
	zipPath := filepath.Join(pluginsDir, s.plugin.FilePath)

	const maxPluginZipSize = 50 << 20 // 50MB
	zipInfo, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("stat zip file %q: %w", zipPath, err)
	}
	if zipInfo.Size() > maxPluginZipSize {
		return fmt.Errorf("zip file %q too large: %d bytes (max %d)", zipPath, zipInfo.Size(), maxPluginZipSize)
	}

	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		return fmt.Errorf("read zip file %q: %w", zipPath, err)
	}

	// [2] Layer 1: 计算规范化 ZIP hash（排除 plugin.json 自身）
	zipHash, err := ComputeCanonicalZipHash(zipData)
	if err != nil {
		return fmt.Errorf("compute canonical zip hash: %w", err)
	}

	// [3] 校验 zip_hash。DB 中的 hash 是权威值，开发期不做空容忍。
	// hash 不一致但 mtime 未变 → 判定 tampered；mtime 已变 → 合法更新，沿用新 hash。
	if s.plugin.ZipHash != zipHash {
		info, statErr := os.Stat(zipPath)
		if statErr != nil {
			return fmt.Errorf("stat zip file: %w", statErr)
		}
		if info.ModTime().Format(time.RFC3339) == s.plugin.FileModTime {
			return fmt.Errorf("zip file tampered: hash mismatch but mtime unchanged")
		}
		slog.Info("ZIP hash changed (legitimate update)", "plugin", s.plugin.EntryPath)
	}

	// [4] 从 ZIP 内存读取入口文件（不落盘）
	entryCode, entryFileName, err := readEntryFromZip(zipData, s.plugin.Main)
	if err != nil {
		return fmt.Errorf("read entry from zip: %w", err)
	}

	// [5] Layer 2: 计算入口文件内容 SHA256
	entryHash := sha256Hex(entryCode)

	// [6] 校验 entry_hash（同样不做空容忍）
	if s.plugin.EntryHash != entryHash {
		if s.plugin.ZipHash == zipHash {
			return fmt.Errorf("entry file tampered: hash mismatch within verified zip")
		}
		// zip_hash 刚更新 = 合法更新
		slog.Info("entry hash changed (legitimate update)", "plugin", s.plugin.EntryPath)
	}

	// [7] 更新 hash 到插件对象（调用者负责持久化到 DB）
	info, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("stat zip file for mtime: %w", err)
	}
	s.plugin.ZipHash = zipHash
	s.plugin.EntryHash = entryHash
	s.plugin.FileModTime = info.ModTime().Format(time.RFC3339)

	// [8] 处理字节码缓存
	cacheDir := filepath.Join(dataDir, s.plugin.EntryPath, "cache")
	jscCachePath := filepath.Join(cacheDir, "main.jsc")
	jscHashPath := filepath.Join(cacheDir, "main.jsc.sha256")

	var jsCode string
	var isBytecode bool

	if strings.HasSuffix(entryFileName, ".jsc") {
		// ZIP 内自带字节码，直接使用（无需缓存）
		jsCode = string(entryCode)
		isBytecode = true
	} else {
		// 源码模式 — 尝试加载字节码缓存
		if cached, ok := loadBytecodeCache(jscCachePath, jscHashPath, entryHash); ok {
			jsCode = string(cached)
			isBytecode = true
			slog.Debug("loaded bytecode from cache", "plugin", s.plugin.EntryPath)
		} else {
			// 无有效缓存，使用源码
			jsCode = string(entryCode)
			isBytecode = false
		}
	}

	// [9] 创建 JS 环境并加载代码（bootstrap + 插件代码）
	s.envID = fmt.Sprintf("jsplugin-%s", s.plugin.EntryPath)
	if isBytecode {
		// 字节码模式：bootstrap 源码先执行，再加载预编译字节码
		if err := s.jsManager.CreateEnvWithBytecode(s.envID, GetBootstrapCode(), []byte(jsCode), s.plugin.ID); err != nil {
			return fmt.Errorf("create js env (bytecode): %w", err)
		}
	} else {
		// 源码模式：bootstrap + 插件源码拼接后一起执行
		initCode := GetBootstrapCode() + "\n" + jsCode
		if err := s.jsManager.CreateEnv(s.envID, initCode, s.plugin.ID); err != nil {
			return fmt.Errorf("create js env: %w", err)
		}
	}

	// [9.5] 注册桥接回调（__go_bridge 的处理函数）
	// 必须在 onInit() 调用之前完成，以便插件代码可以通过 songloft.storage/songs/playlists 访问 Go 服务
	if s.bridgeHandler != nil {
		if err := s.jsManager.SetBridgeCallback(s.envID, s.bridgeHandler.HandleBridgeCall); err != nil {
			return fmt.Errorf("set bridge callback for env %s: %w", s.envID, err)
		}
	}

	// [10] 解压 static/ 到 dataDir（如需 HTTP 服务）
	staticDir := filepath.Join(dataDir, s.plugin.EntryPath, "static")
	if err := extractStaticFromZip(zipData, staticDir); err != nil {
		slog.Warn("extract static files failed (non-fatal)", "plugin", s.plugin.EntryPath, "error", err)
	}

	// [10.1] 解压 bin/ 到 dataDir（如需执行外部命令）
	binDir := filepath.Join(dataDir, s.plugin.EntryPath, "bin")
	if err := extractBinFromZip(zipData, binDir); err != nil {
		slog.Warn("extract bin files failed (non-fatal)", "plugin", s.plugin.EntryPath, "error", err)
	}

	// [11] 源码加载成功后，异步编译并缓存字节码
	if !isBytecode {
		go func() {
			bytecode, err := s.jsManager.CompileToBytecode(s.envID)
			if err != nil {
				slog.Warn("compile bytecode failed", "plugin", s.plugin.EntryPath, "error", err)
				return
			}
			saveBytecodeCache(jscCachePath, jscHashPath, bytecode, entryHash)
		}()
	}

	s.status = ServiceStatusReady
	s.lastActive = time.Now()
	slog.Info("JS plugin loaded", "plugin", s.plugin.EntryPath, "envID", s.envID)
	return nil
}

// Init 调用插件的 onInit() 生命周期回调
func (s *JSService) Init() error {
	s.mu.RLock()
	if s.status != ServiceStatusReady {
		s.mu.RUnlock()
		return fmt.Errorf("cannot init: service status is %s", s.status)
	}
	s.mu.RUnlock()

	// 生命周期回调不接受外部取消，传 nil（runtime 内部会退化为 Background）
	_, err := s.jsManager.ExecuteJS(context.Background(), s.envID, "onInit()", 10000)
	if err != nil {
		return fmt.Errorf("onInit() failed: %w", err)
	}

	// 启动定时器处理 goroutine
	s.timerStop = make(chan struct{})
	go s.runTimerProcessor()

	slog.Info("JS plugin initialized", "plugin", s.plugin.EntryPath)
	return nil
}

// Deinit 调用插件的 onDeinit() 生命周期回调
func (s *JSService) Deinit() error {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()

	if status == ServiceStatusStopped {
		return nil // 已停止，无需 deinit
	}

	_, err := s.jsManager.ExecuteJS(context.Background(), s.envID, "onDeinit()", 10000)
	if err != nil {
		slog.Warn("onDeinit() failed", "plugin", s.plugin.EntryPath, "error", err)
		return fmt.Errorf("onDeinit() failed: %w", err)
	}
	slog.Info("JS plugin deinitialized", "plugin", s.plugin.EntryPath)
	return nil
}

// Stop 停止服务（Deinit + 销毁 JS 环境）
func (s *JSService) Stop() error {
	s.mu.Lock()
	if s.status == ServiceStatusStopped {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// 停止定时器 goroutine（在 Deinit 之前，避免 Deinit 期间定时器仍在跑）
	if s.timerStop != nil {
		close(s.timerStop)
		s.timerStop = nil
	}

	// 调用 deinit（忽略错误，确保后续清理继续）
	_ = s.Deinit()

	// 清理桥接处理器资源（终止后台进程等）
	if s.bridgeHandler != nil {
		s.bridgeHandler.Cleanup()
	}

	// 销毁 JS 环境（包含本插件创建的所有子 env，例如 songloft.jsenv.create 的）
	// DestroyPluginEnvs 按 pluginID 批量回收，root env 也在其中。
	if s.plugin != nil {
		if err := s.jsManager.DestroyPluginEnvs(s.plugin.ID); err != nil {
			slog.Warn("destroy plugin envs failed", "plugin", s.plugin.EntryPath, "error", err)
		}
	}

	s.mu.Lock()
	s.status = ServiceStatusStopped
	s.mu.Unlock()

	slog.Info("JS plugin stopped", "plugin", s.plugin.EntryPath)
	return nil
}

// runTimerProcessor 独立 goroutine，周期性处理 JS 定时器。
// 使用 TryLock 确保不阻塞 HTTP 请求处理。
func (s *JSService) runTimerProcessor() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 如果定时器实际执行了，更新 lastActive 时间戳
			// 这样有活跃定时器的插件不会被误判为空闲
			if s.jsManager.ProcessTimers(s.envID) {
				s.mu.Lock()
				s.lastActive = time.Now()
				s.mu.Unlock()
			}
		case <-s.timerStop:
			return
		}
	}
}

// HandleMessage 实现 MessageHandler 接口
func (s *JSService) HandleMessage(msg *Message) *Message {
	s.mu.Lock()
	s.lastActive = time.Now()
	s.status = ServiceStatusRunning
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.status = ServiceStatusReady
		s.mu.Unlock()
	}()

	switch msg.Type {
	case MsgHTTPRequest:
		return s.handleHTTPRequest(msg)
	case MsgInterPlugin:
		return s.handleInterPlugin(msg)
	case MsgLifecycle:
		return s.handleLifecycle(msg)
	case MsgHealthCheck:
		return s.handleHealthCheck(msg)
	case MsgPlayEvent:
		return s.handlePlayEvent(msg)
	case MsgNetData:
		return s.handleNetData(msg)
	case MsgWebSocketOpen:
		return s.handleWebSocketOpen(msg)
	case MsgWebSocketMessage:
		return s.handleWebSocketMessage(msg)
	case MsgWebSocketClose:
		return s.handleWebSocketClose(msg)
	default:
		return nil
	}
}

// Name 返回服务名（即 plugin.EntryPath）
func (s *JSService) Name() string {
	return s.plugin.EntryPath
}

// Plugin 返回插件元数据
func (s *JSService) Plugin() *JSPlugin {
	return s.plugin
}

// Status 返回当前服务状态
func (s *JSService) Status() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// LastActive 返回最后活跃时间
func (s *JSService) LastActive() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActive
}

// EnvID 返回插件根 JS 环境 ID。Load 之前为空字符串。
// 用于 HealthChecker 直接对 jsruntime 做 TryLock 探针。
func (s *JSService) EnvID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.envID
}

// HasRunningProcesses 检查插件是否有运行中的后台子进程。
// 由 HealthChecker.checkIdle 调用，防止有活跃子进程的插件被休眠。
func (s *JSService) HasRunningProcesses() bool {
	if s.bridgeHandler == nil {
		return false
	}
	hasProc := false
	s.bridgeHandler.processes.Range(func(_, _ any) bool {
		hasProc = true
		return false
	})
	return hasProc
}

// HasActiveUDPSockets 检查插件是否有活跃的 UDP socket。
// 由 HealthChecker.checkIdle 调用，防止有活跃 socket 的插件被休眠。
func (s *JSService) HasActiveUDPSockets() bool {
	if s.bridgeHandler == nil {
		return false
	}
	hasSocket := false
	s.bridgeHandler.udpSockets.Range(func(_, _ any) bool {
		hasSocket = true
		return false
	})
	return hasSocket
}

// HasActiveInboundWebSockets 检查插件是否有活跃的入站 WebSocket 连接。
func (s *JSService) HasActiveInboundWebSockets() bool {
	if s.bridgeHandler == nil {
		return false
	}
	hasSocket := false
	s.bridgeHandler.inboundWebSockets.Range(func(_, _ any) bool {
		hasSocket = true
		return false
	})
	return hasSocket
}

// --- 内部消息处理方法 ---

func errorHTTPResponse(msg *Message, statusCode int, errMsg, detail string) *Message {
	body, _ := json.Marshal(map[string]string{"error": errMsg, "detail": detail})
	return &Message{
		ID: msg.ID, Session: msg.Session,
		Data: &HTTPResponseData{
			StatusCode: statusCode,
			Headers:    map[string]string{"Content-Type": "application/json; charset=utf-8"},
			Body:       string(body),
		},
	}
}

func (s *JSService) handleHTTPRequest(msg *Message) *Message {
	reqData, ok := msg.Data.(*HTTPRequestData)
	if !ok {
		return errorHTTPResponse(msg, 400, "invalid request data", "bad_request")
	}

	// 将请求序列化为 JSON 传入 JS
	reqJSON, err := json.Marshal(reqData)
	if err != nil {
		return errorHTTPResponse(msg, 500, "internal error", "marshal request: "+err.Error())
	}

	// 通过持久 dispatcher（__dispatchHTTP / __dispatchHTTPB64）传入请求 JSON 字符串，
	// 而非把 JSON 内联进源码再让引擎 parse/compile（大 body 时开销随体积放大）。
	// base64 body 场景由 __dispatchHTTPB64 内部用 atob 解码回二进制字符串
	// （latin1，每 char 一字节），这正是 JS 侧 multipart/ZIP 解析器所期望的格式。
	// onHTTPRequest 统一为 async function，dispatcher 内 await 后 JSON.stringify；
	// ExecuteJSCall 的事件循环会等待最终 Promise settle 后返回结果字符串。
	dispatcher := "__dispatchHTTP"
	if reqData.BodyEncoding == "base64" {
		slog.Debug("jsplugin-http: using base64 body decoding via atob",
			"envID", s.envID, "path", reqData.Path, "bodyBase64Len", len(reqData.Body))
		dispatcher = "__dispatchHTTPB64"
	}

	// 传 msg.Ctx：客户端 abort 旧切歌请求时，scheduler.Call 会 cancel 这个 ctx，
	// ExecuteJSCall 的事件循环会立即退出，让 worker 处理下一条消息，避免被 30s
	// 上限的执行卡住，新切的歌排在它后面一直 pending（issue #79 的关键根因）。
	result, err := s.jsManager.ExecuteJSCall(msg.Ctx, s.envID, dispatcher, 30000, string(reqJSON))
	if err != nil {
		statusCode := 500
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			statusCode = 499
		}
		return errorHTTPResponse(msg, statusCode, "plugin execution failed", err.Error())
	}

	// 解析 JS 返回的响应。
	// result.Result == "" 意味着 onHTTPRequest 的 Promise resolve 成了 undefined
	// (handler 漏 return,或包了一层异步逻辑没把响应往外抛)。这是插件协议违例,
	// 直接返 502 + 明确说明,避免上层 InvokeHTTP 把 StatusCode=0 升级成 200 + 空 body,
	// 让 source.fetcher 的 json.Unmarshal 报 "unexpected end of JSON input"。
	if result == nil || result.Result == "" {
		slog.Warn("jsplugin-http: empty result from JS (onHTTPRequest resolved to undefined)",
			"envID", s.envID, "path", reqData.Path, "method", reqData.Method)
		return errorHTTPResponse(msg, 502, "plugin protocol error",
			"onHTTPRequest resolved to undefined or empty (handler likely missing return)")
	}

	var resp HTTPResponseData
	if jsonErr := json.Unmarshal([]byte(result.Result), &resp); jsonErr != nil {
		return errorHTTPResponse(msg, 500, "internal error", "unmarshal response: "+jsonErr.Error())
	}

	return &Message{ID: msg.ID, Session: msg.Session, Data: &resp}
}

func (s *JSService) handleInterPlugin(msg *Message) *Message {
	ipMsg, ok := msg.Data.(*InterPluginMessage)
	if !ok {
		slog.Warn("inter-plugin message: invalid data type", "plugin", s.plugin.EntryPath)
		return &Message{
			Session: msg.Session,
			Data:    &InterPluginResponse{Success: false, Error: "invalid message data type"},
		}
	}

	// 序列化为 JSON 传入 JS
	msgJSON, err := json.Marshal(ipMsg)
	if err != nil {
		return &Message{
			Session: msg.Session,
			Data:    &InterPluginResponse{Success: false, Error: "marshal message: " + err.Error()},
		}
	}

	// __handleInterPluginMessage 是 async function（见 communication.go），本身接收
	// JSON 字符串并内部 JSON.parse；通过 CallValue 直接传字符串参数，免去 JSEscapeString
	// 拼接与源码 parse。ExecuteJSCall 的事件循环会等待 Promise settle 再取结果。
	// 传 msg.Ctx：插件间同步调用同样应该感知调用方取消。
	result, err := s.jsManager.ExecuteJSCall(msg.Ctx, s.envID, "__handleInterPluginMessage", 10000, string(msgJSON))
	if err != nil {
		slog.Warn("inter-plugin message error", "plugin", s.plugin.EntryPath, "error", err)
		return &Message{
			Session: msg.Session,
			Data:    &InterPluginResponse{Success: false, Error: err.Error()},
		}
	}

	if result == nil || result.Result == "" {
		return &Message{
			Session: msg.Session,
			Data:    &InterPluginResponse{Success: true},
		}
	}

	var resp InterPluginResponse
	if err := json.Unmarshal([]byte(result.Result), &resp); err != nil {
		// 无法解析为 InterPluginResponse，将原始结果作为 Data 返回
		return &Message{
			Session: msg.Session,
			Data:    &InterPluginResponse{Success: true, Data: json.RawMessage(result.Result)},
		}
	}

	return &Message{Session: msg.Session, Data: &resp}
}

func (s *JSService) handleHealthCheck(msg *Message) *Message {
	// 健康检查不接受外部取消，传 Background
	result, err := s.jsManager.ExecuteJS(context.Background(), s.envID, "1+1", 5000)
	if err != nil {
		return &Message{
			ID:      msg.ID,
			Session: msg.Session,
			Data:    "unhealthy: " + err.Error(),
		}
	}
	if result == nil || result.Result != "2" {
		return &Message{
			ID:      msg.ID,
			Session: msg.Session,
			Data:    "unhealthy: unexpected eval result",
		}
	}
	return &Message{
		ID:      msg.ID,
		Session: msg.Session,
		Data:    "healthy",
	}
}

func (s *JSService) handleLifecycle(msg *Message) *Message {
	// 生命周期事件
	action, ok := msg.Data.(string)
	if !ok {
		return nil
	}
	switch action {
	case "init":
		if err := s.Init(); err != nil {
			slog.Warn("lifecycle init error", "plugin", s.plugin.EntryPath, "error", err)
		}
	case "deinit":
		if err := s.Deinit(); err != nil {
			slog.Warn("lifecycle deinit error", "plugin", s.plugin.EntryPath, "error", err)
		}
	}
	return nil
}

func (s *JSService) handlePlayEvent(msg *Message) *Message {
	eventData, ok := msg.Data.(*PlayEventData)
	if !ok {
		slog.Warn("play event: invalid data type", "plugin", s.plugin.EntryPath)
		return nil
	}

	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		slog.Warn("play event: marshal failed", "plugin", s.plugin.EntryPath, "error", err)
		return nil
	}

	_, err = s.jsManager.ExecuteJSCall(context.Background(), s.envID, "__dispatchPlayEvent", 5000, string(eventJSON))
	if err != nil {
		slog.Warn("onPlayEvent failed", "plugin", s.plugin.EntryPath, "error", err)
	}
	return nil
}

func (s *JSService) handleNetData(msg *Message) *Message {
	eventData, ok := msg.Data.(*NetDataEvent)
	if !ok {
		slog.Warn("net data: invalid data type", "plugin", s.plugin.EntryPath)
		return nil
	}

	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		slog.Warn("net data: marshal failed", "plugin", s.plugin.EntryPath, "error", err)
		return nil
	}

	_, err = s.jsManager.ExecuteJSCall(context.Background(), s.envID, "__dispatchNetData", 5000, eventData.SocketID, string(eventJSON))
	if err != nil {
		slog.Debug("onNetData failed", "plugin", s.plugin.EntryPath, "socketId", eventData.SocketID, "error", err)
	}
	return nil
}

func (s *JSService) handleWebSocketOpen(msg *Message) *Message {
	eventData, ok := msg.Data.(*WebSocketOpenData)
	if !ok {
		slog.Warn("websocket open: invalid data type", "plugin", s.plugin.EntryPath)
		return &Message{ID: msg.ID, Session: msg.Session, Data: fmt.Errorf("invalid websocket open data")}
	}

	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		return &Message{ID: msg.ID, Session: msg.Session, Data: fmt.Errorf("marshal websocket open: %w", err)}
	}

	_, err = s.jsManager.ExecuteJSCall(context.Background(), s.envID, "__dispatchWSOpen", 10000, string(eventJSON))
	if err != nil {
		slog.Warn("onWebSocket failed", "plugin", s.plugin.EntryPath, "connId", eventData.ConnID, "error", err)
		return &Message{ID: msg.ID, Session: msg.Session, Data: err}
	}

	return &Message{ID: msg.ID, Session: msg.Session, Data: nil}
}

func (s *JSService) handleWebSocketMessage(msg *Message) *Message {
	eventData, ok := msg.Data.(*WebSocketMessageData)
	if !ok {
		slog.Warn("websocket message: invalid data type", "plugin", s.plugin.EntryPath)
		return nil
	}

	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		slog.Warn("websocket message: marshal failed", "plugin", s.plugin.EntryPath, "error", err)
		return nil
	}

	_, err = s.jsManager.ExecuteJSCall(context.Background(), s.envID, "__dispatchWSMessage", 30000, string(eventJSON))
	if err != nil {
		slog.Debug("onWebSocket message failed", "plugin", s.plugin.EntryPath, "connId", eventData.ConnID, "error", err)
	}
	return nil
}

func (s *JSService) handleWebSocketClose(msg *Message) *Message {
	eventData, ok := msg.Data.(*WebSocketCloseData)
	if !ok {
		slog.Warn("websocket close: invalid data type", "plugin", s.plugin.EntryPath)
		return nil
	}

	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		slog.Warn("websocket close: marshal failed", "plugin", s.plugin.EntryPath, "error", err)
		return nil
	}

	_, err = s.jsManager.ExecuteJSCall(context.Background(), s.envID, "__dispatchWSClose", 5000, string(eventJSON))
	if err != nil {
		slog.Debug("onWebSocket close failed", "plugin", s.plugin.EntryPath, "connId", eventData.ConnID, "error", err)
	}
	return nil
}
