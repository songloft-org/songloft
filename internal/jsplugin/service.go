package jsplugin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	text_template "text/template"
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
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
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

	_, err := s.jsManager.ExecuteJS(s.envID, "onInit()", 10000)
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

	_, err := s.jsManager.ExecuteJS(s.envID, "onDeinit()", 10000)
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

	// 清理桥接处理器资源（如果将来有需要清理的）
	// bridgeHandler 当前无需额外清理

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

// --- 内部消息处理方法 ---

func (s *JSService) handleHTTPRequest(msg *Message) *Message {
	reqData, ok := msg.Data.(*HTTPRequestData)
	if !ok {
		return &Message{
			ID: msg.ID, Session: msg.Session,
			Data: &HTTPResponseData{StatusCode: 400, Body: "invalid request data"},
		}
	}

	// 将请求序列化为 JSON 传入 JS
	reqJSON, err := json.Marshal(reqData)
	if err != nil {
		return &Message{
			ID: msg.ID, Session: msg.Session,
			Data: &HTTPResponseData{StatusCode: 500, Body: "marshal request: " + err.Error()},
		}
	}

	// 如果 body 使用了 base64 编码，在调用 onHTTPRequest 前用 atob 解码回二进制字符串。
	// atob 将 base64 字符串解码为 latin1 字符串（每个 char 对应一个字节），
	// 这正是 JS 侧 multipart/ZIP 解析器所期望的格式。
	//
	// 包装为 (async () => ...)()：onHTTPRequest 现在统一为 async function，
	// 必须 await 才能拿到 HTTPResponse 对象再 JSON.stringify；
	// ExecuteJS 的事件循环会等待最终 Promise settle 后返回结果字符串。
	var code string
	if reqData.BodyEncoding == "base64" {
		slog.Info("jsplugin-http: using base64 body decoding via atob",
			"envID", s.envID, "path", reqData.Path, "bodyBase64Len", len(reqData.Body))
		code = fmt.Sprintf(`(async function(){var _r=%s;_r.body=atob(_r.body);delete _r.bodyEncoding;return JSON.stringify(await onHTTPRequest(_r));})()`, string(reqJSON))
	} else {
		code = fmt.Sprintf(`(async function(){return JSON.stringify(await onHTTPRequest(%s));})()`, string(reqJSON))
	}

	result, err := s.jsManager.ExecuteJS(s.envID, code, 30000)
	if err != nil {
		return &Message{
			ID: msg.ID, Session: msg.Session,
			Data: &HTTPResponseData{StatusCode: 500, Body: err.Error()},
		}
	}

	// 解析 JS 返回的响应。
	// result.Result == "" 意味着 onHTTPRequest 的 Promise resolve 成了 undefined
	// (handler 漏 return,或包了一层异步逻辑没把响应往外抛)。这是插件协议违例,
	// 直接返 502 + 明确说明,避免上层 InvokeHTTP 把 StatusCode=0 升级成 200 + 空 body,
	// 让 source.fetcher 的 json.Unmarshal 报 "unexpected end of JSON input"。
	if result == nil || result.Result == "" {
		slog.Warn("jsplugin-http: empty result from JS (onHTTPRequest resolved to undefined)",
			"envID", s.envID, "path", reqData.Path, "method", reqData.Method)
		return &Message{
			ID: msg.ID, Session: msg.Session,
			Data: &HTTPResponseData{
				StatusCode: 502,
				Body:       "plugin onHTTPRequest resolved to undefined or empty (handler likely missing return)",
			},
		}
	}

	var resp HTTPResponseData
	if jsonErr := json.Unmarshal([]byte(result.Result), &resp); jsonErr != nil {
		return &Message{
			ID: msg.ID, Session: msg.Session,
			Data: &HTTPResponseData{StatusCode: 500, Body: "unmarshal response: " + jsonErr.Error()},
		}
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

	// 使用 template.JSEscapeString 防止 JSON 中的特殊字符破坏 JS 代码。
	// __handleInterPluginMessage 现在是 async function（见 communication.go），
	// 调用本身会返回 Promise；ExecuteJS 的事件循环会等待 settle 再取结果。
	code := fmt.Sprintf(`__handleInterPluginMessage('%s')`, text_template.JSEscapeString(string(msgJSON)))

	result, err := s.jsManager.ExecuteJS(s.envID, code, 10000)
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
	// 执行简单的 JS 引擎 eval 验证 VM 存活
	result, err := s.jsManager.ExecuteJS(s.envID, "1+1", 5000)
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
