package jsruntime

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"

	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"

	"songloft/internal/httputil"

	"modernc.org/quickjs"
)

// sharedTransport 供 doHTTPRequest 使用的共享 Transport
// 使用默认 TLS 配置（与 WASM 版本行为一致），不强制 TLS 版本或 HTTP 协议
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// sharedHTTPClient 供 doHTTPRequest 使用的共享 HTTP 客户端
// 相比 http.DefaultClient 提供：合理超时、连接池管理、idle 连接自动回收
var sharedHTTPClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: sharedTransport,
}

// noRedirectHTTPClient 不跟随重定向的 HTTP 客户端
// 当请求头包含 X-Fetch-No-Redirect 时使用，让 JS 侧手动处理重定向链
var noRedirectHTTPClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: sharedTransport,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// 默认 JS 执行超时时间
const defaultJSTimeout = 30 * time.Second

// maxFetchBodyBytes 限制单次 fetch 响应体读入内存的上限，防止异常/恶意大响应 OOM。
const maxFetchBodyBytes = 64 << 20 // 64 MiB

// polyfill 字节码缓存：polyfillJS 有 600+ 行，冷启动时约 48% 的时间花在解析它。
// 首次编译为字节码后缓存，后续所有 env 冷启动直接 EvalBytecode 加载，跳过解析编译。
// QuickJS 字节码经 WriteObject/ReadObject 序列化 atoms，可跨 runtime 复用（项目
// 已用于 .jsc 插件缓存跨进程加载）。编译失败时回退到源码 eval。
var (
	polyfillBytecodeOnce sync.Once
	polyfillBytecode     []byte
)

// loadPolyfill 在 vm 中注入 polyfill。优先用缓存的字节码，首次调用时用当前 vm
// 编译并缓存。编译失败则回退到源码 eval（保证功能不受影响）。
func loadPolyfill(vm *quickjs.VM) error {
	polyfillBytecodeOnce.Do(func() {
		bc, err := vm.Compile(polyfillJS, quickjs.EvalGlobal)
		if err != nil {
			slog.Warn("compile polyfill to bytecode failed, falling back to source eval", "error", err)
			return // polyfillBytecode 保持 nil，走源码回退
		}
		polyfillBytecode = bc
		slog.Debug("polyfill compiled to bytecode", "size", len(bc))
	})

	if polyfillBytecode != nil {
		v, err := vm.EvalBytecodeValue(polyfillBytecode)
		if err != nil {
			return fmt.Errorf("eval polyfill bytecode: %w", err)
		}
		v.Free()
		return nil
	}

	// 回退：源码 eval
	v, err := vm.EvalValue(polyfillJS, quickjs.EvalGlobal)
	if err != nil {
		return fmt.Errorf("eval polyfill source: %w", err)
	}
	v.Free()
	return nil
}

// BridgeCallback 是插件层注册的桥接回调函数类型
// 当 JS 调用 __go_bridge(action, data) 时，由此回调处理
type BridgeCallback func(action, data string) (string, error)

// asyncResult 是后台 goroutine 完成异步任务后投递给主事件循环的结果。
//
// ID 与 JS 侧 __asyncCallbacks 注册表的 key 对应，形如 "fetch:42"，前缀部分
// 同时被 JS 侧 __resolveAsync 用于决定如何包装 payload（fetch 包装成 Response
// 对象、bridge 透传字符串等）。
//
// OK == true 时 Data 为载荷（fetch 的完整响应 JSON 字符串、或 bridge 调用结果）；
// OK == false 时 Data 为错误信息。
type asyncResult struct {
	ID   string // 如 "fetch:42"
	Type string // 如 "fetch"，对应 __resolveAsync 的 type 参数
	OK   bool
	Data string
}

// hostEvent 是宿主侧主动推送给 JS 事件循环的事件。
//
// 与 asyncResult 不同，它不对应某个 JS Promise 的 resolve/reject，而是浏览器事件
// 那样分发给已注册的回调，例如 UDP onData 或入站 WebSocket message/close。
type hostEvent struct {
	Type string // 如 "net_data" / "inbound_ws_message"
	ID   string // 事件关联对象 ID，如 socketId / connId
	Data string // 原始 JSON payload，由插件 bootstrap 解析
}

// wsConn 封装一条 Go 侧管理的 WebSocket 连接
type wsConn struct {
	id     string
	conn   *websocket.Conn
	mu     sync.Mutex // 保护写操作（ReadMessage 由独立 goroutine 串行调用，不需锁）
	closed atomic.Bool
}

// JSEnv 代表一个 JS 运行时环境
type JSEnv struct {
	vm             *quickjs.VM
	envID          string
	pluginID       int64
	created        time.Time
	mu             sync.Mutex         // 串行化所有 VM 访问（quickjs 非线程安全）
	events         chan JSEventResult // 每环境事件缓冲
	sourceCode     string             // 保存初始化代码，用于字节码编译
	bridgeCallback BridgeCallback     // 插件层桥接回调（__go_bridge 的处理函数）
	// 真异步基础设施：__go_fetch / __go_bridge_async 等耗时桥接调用
	// 通过此通道把结果回送给主事件循环（ExecuteJS 内部）。
	asyncResults chan asyncResult
	// asyncSignal 是单容量缓冲通道，goroutine 完成异步任务或宿主投递事件后非阻塞
	// send 一次，用于唤醒 ExecuteJS 的 select。容量 1 足够：多次唤醒会被合并，
	// 事件循环醒来后会一次性 drain asyncResults/hostEvents。
	asyncSignal chan struct{}
	// asyncSeq 单调递增，作为 asyncResult.ID 的序号部分。
	asyncSeq atomic.Uint64
	// asyncInflight 记录当前正在飞行的异步任务数；ExecuteJS 事件循环根据
	// 此计数判断是否仍需等待新结果。
	asyncInflight atomic.Int32
	// hostEvents 保存宿主主动投递的事件。独立于 asyncResults，避免 UDP/WebSocket
	// 高频事件挤占 fetch/bridge 的 Promise 回包通道。
	hostEvents chan hostEvent
	// wsConns 管理该 env 下的所有 WebSocket 连接（connId → *wsConn）
	wsConns sync.Map
	// wsConnSeq WebSocket 连接 ID 递增序号
	wsConnSeq atomic.Uint64
}

// JSEventResult 封装收集到的 JS 事件
type JSEventResult struct {
	EnvID string
	Name  string
	Data  string
}

// ExecuteResult 封装 ExecuteJS 的返回结果
type ExecuteResult struct {
	Result string
	Events []JSEventResult
}

// ParallelCall 描述一次并行 JS 调用
type ParallelCall struct {
	EnvID          string
	Code           string
	TimeoutMs      int64
	WaitEventNames []string
}

// ParallelResult 单个并行调用的结果
type ParallelResult struct {
	Index  int
	Result *ExecuteResult
	Err    error
}

// JSEnvManager 管理多个 JS 运行时环境（进程内）
type JSEnvManager struct {
	mu           sync.Mutex
	envs         map[string]*JSEnv
	pluginEnvs   map[int64]map[string]bool // plugin_id -> set of env_ids
	shutdownCh   chan struct{}             // 关闭信号：让 ExecuteJSAndWaitEvents 等阻塞操作提前返回
	shutdownOnce sync.Once
}

// NewJSEnvManager 创建 JS 运行时管理器（进程内，无需外部二进制）
func NewJSEnvManager() *JSEnvManager {
	mgr := &JSEnvManager{
		envs:       make(map[string]*JSEnv),
		pluginEnvs: make(map[int64]map[string]bool),
		shutdownCh: make(chan struct{}),
	}
	slog.Info("JSEnvManager 已启动（进程内 quickjs）")
	return mgr
}

// SignalShutdown 通知所有正在阻塞的 JS 操作尽快退出。
// 必须在 Close 之前调用：让 ExecuteJSAndWaitEvents 的 polling loop 提前返回，
// 释放 env.mu，避免 Close 在 env.mu.Lock 上死等（典型场景：批量加载音源时按下 Ctrl+C）。
// 幂等，可重复调用。
func (m *JSEnvManager) SignalShutdown() {
	m.shutdownOnce.Do(func() {
		close(m.shutdownCh)
		slog.Info("JSEnvManager: shutdown signaled")
	})
}

// CreateEnv 创建一个新的 JS 运行时环境
func (m *JSEnvManager) CreateEnv(envID, initCode string, pluginID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.envs[envID]; exists {
		return fmt.Errorf("env %s already exists", envID)
	}

	vm, err := quickjs.NewVM()
	if err != nil {
		return fmt.Errorf("create quickjs VM for env %s: %w", envID, err)
	}

	// 设置栈溢出检测阈值为合理值，防止 jsjiami 混淆代码死循环
	SetMaxStackSize(vm)

	env := &JSEnv{
		vm:           vm,
		envID:        envID,
		pluginID:     pluginID,
		created:      time.Now(),
		events:       make(chan JSEventResult, 64),
		sourceCode:   initCode,
		asyncResults: make(chan asyncResult, 256),
		asyncSignal:  make(chan struct{}, 1),
		hostEvents:   make(chan hostEvent, 512),
	}

	// 注册 Go 桥接函数
	if err := registerBridgeFunctions(vm, env); err != nil {
		vm.Close()
		return fmt.Errorf("register bridge functions for env %s: %w", envID, err)
	}

	// 注入 JS polyfill 代码（优先字节码，跳过解析）
	if err := loadPolyfill(vm); err != nil {
		vm.Close()
		return fmt.Errorf("inject polyfills for env %s: %w", envID, err)
	}

	// 执行初始化代码
	if initCode != "" {
		if v, err := vm.EvalValue(initCode, quickjs.EvalGlobal); err != nil {
			vm.Close()
			return fmt.Errorf("init code for env %s: %w", envID, err)
		} else {
			v.Free()
		}
	}

	// 记录归属关系
	m.envs[envID] = env
	if m.pluginEnvs[pluginID] == nil {
		m.pluginEnvs[pluginID] = make(map[string]bool)
	}
	m.pluginEnvs[pluginID][envID] = true

	slog.Info("JS 环境已创建", "envID", envID, "pluginID", pluginID)
	return nil
}

// CreateEnvWithBytecode 创建 JS 环境，先执行 bootstrap 源码，再加载预编译字节码
// 用于重启后从 .jsc 缓存加载插件，避免将二进制字节码当作 JS 源码解析
func (m *JSEnvManager) CreateEnvWithBytecode(envID, bootstrapCode string, bytecode []byte, pluginID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.envs[envID]; exists {
		return fmt.Errorf("env %s already exists", envID)
	}

	vm, err := quickjs.NewVM()
	if err != nil {
		return fmt.Errorf("create quickjs VM for env %s: %w", envID, err)
	}

	SetMaxStackSize(vm)

	env := &JSEnv{
		vm:           vm,
		envID:        envID,
		pluginID:     pluginID,
		created:      time.Now(),
		events:       make(chan JSEventResult, 64),
		asyncResults: make(chan asyncResult, 256),
		asyncSignal:  make(chan struct{}, 1),
		hostEvents:   make(chan hostEvent, 512),
		// 字节码模式无源码可编译，sourceCode 留空
	}

	// 注册 Go 桥接函数
	if err := registerBridgeFunctions(vm, env); err != nil {
		vm.Close()
		return fmt.Errorf("register bridge functions for env %s: %w", envID, err)
	}

	// 注入 JS polyfill 代码（优先字节码，跳过解析）
	if err := loadPolyfill(vm); err != nil {
		vm.Close()
		return fmt.Errorf("inject polyfills for env %s: %w", envID, err)
	}

	// 执行 bootstrap 源码（bridge 函数定义等）
	if bootstrapCode != "" {
		if v, err := vm.EvalValue(bootstrapCode, quickjs.EvalGlobal); err != nil {
			vm.Close()
			return fmt.Errorf("bootstrap code for env %s: %w", envID, err)
		} else {
			v.Free()
		}
	}

	// 加载预编译字节码
	if v, err := vm.EvalBytecodeValue(bytecode); err != nil {
		vm.Close()
		return fmt.Errorf("eval bytecode for env %s: %w", envID, err)
	} else {
		v.Free()
	}

	// 记录归属关系
	m.envs[envID] = env
	if m.pluginEnvs[pluginID] == nil {
		m.pluginEnvs[pluginID] = make(map[string]bool)
	}
	m.pluginEnvs[pluginID][envID] = true

	slog.Info("JS 环境已创建（字节码模式）", "envID", envID, "pluginID", pluginID)
	return nil
}

// ExecuteJS 在指定环境中执行 JS 代码，等待所有由其触发的 Promise 链 settled 后返回。
//
// ctx 用于异步等待阶段的提前取消（如客户端 abort 旧请求）：
//   - 慢路径 select 中 ctx.Done() 会立即返回 ctx.Err()，让上层 worker 释放
//     被放弃的请求所占的位置，给新切歌的请求让路。
//   - 同步 eval 阶段持锁不响应 ctx；vm.SetEvalTimeout 作为 JS 内死循环兜底。
//   - 传 nil 时退化为仅依赖 wall-clock + shutdownCh（内部 health/lifecycle 用）。
//
// 真异步事件循环：
//
//  1. 持锁 eval(code)，得到 val。
//  2. 快速路径：val 非 thenable 且无 in-flight 异步任务 → 直接返回（健康探针、
//     1+1、ProcessTimers 等内部调用都走这条）。
//  3. 慢速路径：把 val 挂到 globalThis.__execjs_pending，附加 .then 链把结果
//     回填到 globalThis.__execjs_done/value/error。然后进入事件循环：
//     pumpAsyncResults → ExecutePendingJobs → processExpiredTimers → 检查
//     done flag。未完成则释放 env.mu，select { asyncSignal | timer-tick |
//     wall-clock | shutdown | ctx.Done }，被唤醒后重新加锁继续。
//
// 设计保证：
//   - 真异步 await 期间 env.mu 被释放，HealthProbe / 定时器 / 同插件其他请求可抢锁。
//   - 调用方仍是同步语义（拿到最终值或错误），不需要改 service.go 的 handleHTTPRequest 契约。
//   - 超时由 wall-clock 控制；vm.SetEvalTimeout 作为 JS 内死循环兜底。
func (m *JSEnvManager) ExecuteJS(ctx context.Context, envID, code string, timeoutMs int64) (*ExecuteResult, error) {
	return m.executeInEnv(ctx, envID, timeoutMs, func(vm *quickjs.VM) (quickjs.Value, error) {
		return vm.EvalValue(code, quickjs.EvalGlobal)
	})
}

// ExecuteJSCall 与 ExecuteJS 语义相同，但通过 vm.CallValue 按名调用一个预定义的
// 全局函数并传入参数，而非把参数内联进源码字符串再解析。args 为 Go 值，会被
// quickjs 转换为 JS 参数（string → 原生 JS 字符串，不经源码 parse）。
//
// 用于 HTTP 请求 / 事件分发等热路径：以前每次都拼接 `(async()=>...(<内联大JSON>))()`
// 源码并让 QuickJS 重新 parse/compile（大 body 时开销随体积放大）。改为传入 JSON
// 字符串给持久 dispatcher（如 __dispatchHTTP），dispatcher 内用原生 JSON.parse，
// 避免每请求编译一个内联大对象字面量的匿名函数。
func (m *JSEnvManager) ExecuteJSCall(ctx context.Context, envID, fnName string, timeoutMs int64, args ...any) (*ExecuteResult, error) {
	return m.executeInEnv(ctx, envID, timeoutMs, func(vm *quickjs.VM) (quickjs.Value, error) {
		return vm.CallValue(fnName, args...)
	})
}

// executeInEnv 是 ExecuteJS / ExecuteJSCall 共享的执行核心：持锁执行 invoke 取得
// 初始值后，复用同一套真异步事件循环等待 Promise 链 settled。invoke 负责产出初始
// JS 值（eval 源码 或 CallValue 调用函数），其余逻辑完全一致。
func (m *JSEnvManager) executeInEnv(ctx context.Context, envID string, timeoutMs int64, invoke func(vm *quickjs.VM) (quickjs.Value, error)) (*ExecuteResult, error) {
	env, err := m.getEnv(envID)
	if err != nil {
		return nil, err
	}

	timeout := defaultJSTimeout
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}
	deadline := time.Now().Add(timeout)

	// nil ctx 视为不可取消，简化内部调用方
	if ctx == nil {
		ctx = context.Background()
	}

	env.mu.Lock()
	vm := env.vm
	vm.SetEvalTimeout(timeout)
	slog.Debug("ExecuteJS: eval start", "envID", envID)

	// 在 eval 前排空异步结果和宿主事件通道：WebSocket/UDP 读循环 goroutine 不计入
	// asyncInflight，如果不在此处 pump，后续 quick path 会跳过事件分发。
	if len(env.asyncResults) > 0 {
		if pumped := pumpAsyncResults(vm); pumped > 0 {
			ExecutePendingJobs(vm)
		}
	}
	if len(env.hostEvents) > 0 {
		if pumped := pumpHostEvents(vm, env); pumped > 0 {
			ExecutePendingJobs(vm)
		}
	}

	val, evalErr := invoke(vm)
	ExecutePendingJobs(vm)

	if evalErr != nil {
		events := drainEnvEvents(env)
		env.mu.Unlock()
		slog.Info("ExecuteJS: eval error", "envID", envID, "error", evalErr, "eventsCount", len(events))
		return &ExecuteResult{Events: events}, evalErr
	}

	// 快速路径：内部健康探针、ProcessTimers 等同步调用走这里。
	if env.asyncInflight.Load() == 0 && !valueIsThenable(vm, val) {
		result := &ExecuteResult{
			Result: valueToString(&val),
			Events: drainEnvEvents(env),
		}
		val.Free()
		env.mu.Unlock()
		slog.Debug("ExecuteJS: sync done", "envID", envID, "eventsCount", len(result.Events))
		return result, nil
	}

	// 慢速路径：把 val 挂到全局，附加 settle 钩子。
	if err := setupAwaitProbe(vm, val); err != nil {
		val.Free()
		env.mu.Unlock()
		return nil, fmt.Errorf("setup await probe: %w", err)
	}
	val.Free() // SetPropertyValue 内部 Dup 过，本地引用可释放。

	// 事件循环
	for {
		// 1) 处理已就绪的异步结果（resolve/reject 对应 Promise）
		if pumped := pumpAsyncResults(vm); pumped > 0 {
			ExecutePendingJobs(vm)
		}
		// 2) 处理宿主推送事件（UDP 收包 / 入站 WebSocket message/close）
		if pumped := pumpHostEvents(vm, env); pumped > 0 {
			ExecutePendingJobs(vm)
		}
		// 3) 处理到期定时器（setTimeout-based await 链 / interval 回调）
		if processExpiredTimers(vm) > 0 {
			ExecutePendingJobs(vm)
		}
		// 4) 检查 done flag
		if isAwaitDone(vm) {
			break
		}
		// 5) deadline 检查
		remaining := time.Until(deadline)
		if remaining <= 0 {
			cleanupAwaitProbe(vm)
			events := drainEnvEvents(env)
			env.mu.Unlock()
			slog.Warn("ExecuteJS: wall-clock timeout",
				"envID", envID, "timeout", timeout,
				"asyncInflight", env.asyncInflight.Load())
			return &ExecuteResult{Events: events}, fmt.Errorf("ExecuteJS wall-clock timeout after %v", timeout)
		}

		// 6) 释放锁让 goroutine 推结果/事件、健康探针抢锁、同插件其他请求处理。
		env.mu.Unlock()

		// 50ms 兜底 tick：处理 setInterval 等仅靠定时器驱动的进展场景；
		// 不能太长（否则 setTimeout(fn, 0) 也要等 50ms），不能太短（CPU 空转）。
		tickTimeout := min(50*time.Millisecond, remaining)

		select {
		case <-m.shutdownCh:
			// shutdown 路径：上层会强制 Close，本协程不再回到持锁阶段。
			env.mu.Lock()
			// 检查环境是否已被销毁（DestroyEnv 会在关闭 VM 后将 env.vm 置为 nil）
			if env.vm != nil {
				cleanupAwaitProbe(vm)
			}
			events := drainEnvEvents(env)
			env.mu.Unlock()
			return &ExecuteResult{Events: events}, fmt.Errorf("jsenv manager shutting down")
		case <-ctx.Done():
			// 调用方取消（如客户端 abort 旧切歌请求）：清理 await probe，
			// 让 worker 立即处理下一条消息，避免无谓占用单 worker 配额。
			// 同时 vm 内 Promise 仍可能在后续 ProcessTimers 时被推进，
			// drain 一次事件后即返回；残留状态由下一次 ExecuteJS 清理。
			env.mu.Lock()
			if env.vm != nil {
				cleanupAwaitProbe(vm)
			}
			events := drainEnvEvents(env)
			env.mu.Unlock()
			slog.Info("ExecuteJS: ctx canceled", "envID", envID, "err", ctx.Err())
			return &ExecuteResult{Events: events}, ctx.Err()
		case <-env.asyncSignal:
			// 收到一个或多个新结果 → 重新加锁推进
		case <-time.After(tickTimeout):
			// 周期性醒来推进定时器
		}
		env.mu.Lock()

		// 检查环境是否已被销毁
		if env.vm == nil {
			env.mu.Unlock()
			slog.Warn("ExecuteJS: 环境已被销毁，提前退出", "envID", envID)
			return nil, fmt.Errorf("env %s destroyed", envID)
		}
	}

	resultStr, errMsg := readAwaitResult(vm)
	cleanupAwaitProbe(vm)
	events := drainEnvEvents(env)
	env.mu.Unlock()

	slog.Debug("ExecuteJS: async done", "envID", envID, "eventsCount", len(events), "hasError", errMsg != "")
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		for i, evt := range events {
			dataPreview := evt.Data
			if len(dataPreview) > 300 {
				dataPreview = dataPreview[:300] + "..."
			}
			slog.Debug("ExecuteJS: event detail", "envID", envID, "index", i, "name", evt.Name, "dataLen", len(evt.Data), "dataPreview", dataPreview)
		}
	}

	if errMsg != "" {
		return &ExecuteResult{Events: events}, fmt.Errorf("%s", errMsg)
	}
	return &ExecuteResult{Result: resultStr, Events: events}, nil
}

// pumpAsyncResults 在持锁状态下调入 JS 的 __pumpAsyncResults() 排空 asyncResults。
// 返回处理的结果数；0 表示通道当前为空。用 CallValue 按名调用，避免每次解析源码。
func pumpAsyncResults(vm *quickjs.VM) int {
	val, err := vm.CallValue("__pumpAsyncResults")
	if err != nil {
		slog.Warn("pumpAsyncResults: call failed", "error", err)
		return 0
	}
	defer val.Free()
	if r, anyErr := val.Any(); anyErr == nil {
		return valueAsInt(r)
	}
	return 0
}

// pumpHostEvents 在持锁状态下把宿主主动投递的事件分发到 JS。
// 分发函数由上层 bootstrap 提供；事件处理采用 fire-and-forget 语义，不复用
// executeInEnv 的全局 await probe，避免打断当前正在等待的 HTTP/生命周期调用。
func pumpHostEvents(vm *quickjs.VM, env *JSEnv) int {
	n := 0
	for {
		select {
		case evt := <-env.hostEvents:
			val, err := vm.CallValue("__dispatchHostEvent", evt.Type, evt.ID, evt.Data)
			if err != nil {
				slog.Debug("pumpHostEvents: dispatch failed",
					"envID", env.envID,
					"type", evt.Type,
					"id", evt.ID,
					"error", err)
				continue
			}
			val.Free()
			n++
		default:
			return n
		}
	}
}

// valueIsThenable 用 JS 探测 val 是否带 then 函数。把 val 挂到 globalThis 临时槽
// 调一次预定义的 __isThenable 判断后清理。
func valueIsThenable(vm *quickjs.VM, val quickjs.Value) bool {
	if val.IsUndefined() {
		return false
	}
	if err := bindGlobalValue(vm, "__execjs_probe", val); err != nil {
		return false
	}
	defer unsetGlobalValue(vm, "__execjs_probe")

	r, err := vm.CallValue("__isThenable")
	if err != nil {
		return false
	}
	defer r.Free()
	if x, anyErr := r.Any(); anyErr == nil {
		if b, ok := x.(bool); ok {
			return b
		}
	}
	return false
}

// setupAwaitProbe 把 val 挂到全局，附加 .then 钩子，把结果回填到
// __execjs_done / __execjs_value / __execjs_error。
func setupAwaitProbe(vm *quickjs.VM, val quickjs.Value) error {
	if err := bindGlobalValue(vm, "__execjs_pending", val); err != nil {
		return err
	}
	r, err := vm.CallValue("__setupAwaitProbe")
	if err != nil {
		return err
	}
	r.Free()
	// then 注册产生了一个微任务（非 Promise 路径会立即 resolve）；先消化一轮。
	ExecutePendingJobs(vm)
	return nil
}

// isAwaitDone 检查 setupAwaitProbe 设置的 done flag 是否为 true。
func isAwaitDone(vm *quickjs.VM) bool {
	v, err := vm.CallValue("__isAwaitDone")
	if err != nil {
		return false
	}
	defer v.Free()
	if r, anyErr := v.Any(); anyErr == nil {
		if b, ok := r.(bool); ok {
			return b
		}
	}
	return false
}

// readAwaitResult 读取 setupAwaitProbe 写入的 value/error。errMsg 非空表示
// Promise rejected。value 经过 valueToString 处理，可能为 ""。
func readAwaitResult(vm *quickjs.VM) (string, string) {
	// 先读 error
	errV, err := vm.CallValue("__readAwaitError")
	if err == nil {
		errStr := valueToString(&errV)
		errV.Free()
		if errStr != "" {
			return "", errStr
		}
	} else {
		errV.Free()
	}
	// 读 value：直接取字符串化值
	valV, err := vm.CallValue("__readAwaitValue")
	if err != nil {
		return "", ""
	}
	defer valV.Free()
	return valueToString(&valV), ""
}

// cleanupAwaitProbe 清理 setupAwaitProbe 写入的全局变量，避免污染下一次调用。
func cleanupAwaitProbe(vm *quickjs.VM) {
	v, err := vm.CallValue("__cleanupAwaitProbe")
	if err == nil {
		v.Free()
	}
}

// bindGlobalValue 把 val 挂到 globalThis[name]。
// 使用 SetPropertyValue（内部 Dup），不消耗 val。
func bindGlobalValue(vm *quickjs.VM, name string, val quickjs.Value) error {
	g := vm.GlobalObject()
	defer g.Free()
	a, err := vm.NewAtom(name)
	if err != nil {
		return err
	}
	return vm.SetPropertyValue(g, a, val)
}

// unsetGlobalValue 把 globalThis[name] 设为 undefined（最简单的清理）。
func unsetGlobalValue(vm *quickjs.VM, name string) {
	v, err := vm.EvalValue(`globalThis['`+name+`'] = undefined`, quickjs.EvalGlobal)
	if err == nil {
		v.Free()
	}
}

// ExecuteJSAndWaitEvents 执行 JS 代码并等待指定名称的事件到达
//
// ctx 用于轮询等待阶段的提前取消；传 nil 视为 Background。
func (m *JSEnvManager) ExecuteJSAndWaitEvents(ctx context.Context, envID, code string, timeoutMs int64, waitEventNames []string) (*ExecuteResult, error) {
	env, err := m.getEnv(envID)
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	env.mu.Lock()

	timeout := defaultJSTimeout
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	// 执行代码
	vm := env.vm
	vm.SetEvalTimeout(timeout)
	slog.Info("ExecuteJSAndWaitEvents: eval start", "envID", envID, "codeLen", len(code), "waitEventNames", waitEventNames)
	val, evalErr := vm.EvalValue(code, quickjs.EvalGlobal)

	// 处理到期定时器
	processJobs(vm)

	execResult := &ExecuteResult{}

	if evalErr != nil {
		execResult.Events = drainEnvEvents(env)
		slog.Info("ExecuteJSAndWaitEvents: eval error", "envID", envID, "error", evalErr, "eventsCount", len(execResult.Events))
		env.mu.Unlock()
		return execResult, evalErr
	}

	execResult.Result = valueToString(&val)
	val.Free()

	// 构建等待事件名称的 set
	waitSet := make(map[string]bool, len(waitEventNames))
	for _, name := range waitEventNames {
		waitSet[name] = true
	}

	if len(waitSet) == 0 {
		execResult.Events = drainEnvEvents(env)
		env.mu.Unlock()
		return execResult, nil
	}

	// 先收集已有事件，检查是否已经有匹配的
	execResult.Events = drainEnvEvents(env)
	for _, evt := range execResult.Events {
		if waitSet[evt.Name] {
			slog.Info("ExecuteJSAndWaitEvents: 立即收到匹配事件", "envID", envID, "eventName", evt.Name)
			env.mu.Unlock()
			return execResult, nil
		}
	}

	env.mu.Unlock()

	// 没有立即收到匹配事件，开始轮询等待
	waitTimeout := min(timeout, 30*time.Second)
	deadline := time.Now().Add(waitTimeout)

	slog.Info("ExecuteJSAndWaitEvents: 开始等待事件", "envID", envID, "waitEventNames", waitEventNames, "timeout", waitTimeout)

	for time.Now().Before(deadline) {
		// 关闭/取消信号优先：让上层（runBatchLoader 等）释放主 env 锁，
		// 避免 jsManager.Close 死等，也让客户端 abort 旧请求时尽快放行。
		select {
		case <-m.shutdownCh:
			slog.Info("ExecuteJSAndWaitEvents: 收到关闭信号，提前返回", "envID", envID)
			return execResult, fmt.Errorf("jsenv manager shutting down")
		case <-ctx.Done():
			slog.Info("ExecuteJSAndWaitEvents: ctx canceled", "envID", envID, "err", ctx.Err())
			return execResult, ctx.Err()
		default:
		}

		env.mu.Lock()

		// 检查环境是否已被销毁（DestroyEnv 会在关闭 VM 后将 env.vm 置为 nil）
		if env.vm == nil {
			env.mu.Unlock()
			slog.Warn("ExecuteJSAndWaitEvents: 环境已被销毁，提前退出", "envID", envID)
			return execResult, fmt.Errorf("env %s destroyed", envID)
		}

		// 处理定时器回调（可能产生新事件）
		processJobs(env.vm)

		newEvents := drainEnvEvents(env)
		env.mu.Unlock()

		execResult.Events = append(execResult.Events, newEvents...)

		for _, evt := range newEvents {
			if waitSet[evt.Name] {
				slog.Info("ExecuteJSAndWaitEvents: 收到匹配事件", "envID", envID, "eventName", evt.Name)
				return execResult, nil
			}
		}

		// 用 select 替代 time.Sleep，关闭时也能立即唤醒
		select {
		case <-m.shutdownCh:
			slog.Info("ExecuteJSAndWaitEvents: 收到关闭信号，提前返回", "envID", envID)
			return execResult, fmt.Errorf("jsenv manager shutting down")
		case <-ctx.Done():
			slog.Info("ExecuteJSAndWaitEvents: ctx canceled", "envID", envID, "err", ctx.Err())
			return execResult, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}

	slog.Warn("ExecuteJSAndWaitEvents: 等待事件超时", "envID", envID, "waitEventNames", waitEventNames)
	return execResult, nil
}

// DestroyEnv 销毁指定的 JS 运行时环境
func (m *JSEnvManager) DestroyEnv(envID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, exists := m.envs[envID]
	if !exists {
		return fmt.Errorf("env %s not found", envID)
	}

	// 关闭该 env 下的所有 WebSocket 连接
	env.wsConns.Range(func(key, value any) bool {
		wsc := value.(*wsConn)
		if wsc.closed.CompareAndSwap(false, true) {
			wsc.mu.Lock()
			_ = wsc.conn.Close()
			wsc.mu.Unlock()
		}
		env.wsConns.Delete(key)
		return true
	})

	// 关闭 VM
	env.mu.Lock()
	if err := env.vm.Close(); err != nil {
		slog.Warn("关闭 JS VM 失败", "envID", envID, "error", err)
	}
	env.vm = nil // 标记为已销毁，防止其他 goroutine 继续使用
	env.mu.Unlock()

	// 清理归属记录
	delete(m.envs, envID)
	if envSet, ok := m.pluginEnvs[env.pluginID]; ok {
		delete(envSet, envID)
		if len(envSet) == 0 {
			delete(m.pluginEnvs, env.pluginID)
		}
	}

	slog.Info("JS 环境已销毁", "envID", envID, "pluginID", env.pluginID)
	return nil
}

// DestroyPluginEnvs 销毁指定插件创建的所有 JS 环境
func (m *JSEnvManager) DestroyPluginEnvs(pluginID int64) error {
	m.mu.Lock()
	envIDs := make([]string, 0)
	if envSet, ok := m.pluginEnvs[pluginID]; ok {
		for envID := range envSet {
			envIDs = append(envIDs, envID)
		}
	}
	m.mu.Unlock()

	if len(envIDs) == 0 {
		return nil
	}

	slog.Info("批量销毁插件 JS 环境", "pluginID", pluginID, "count", len(envIDs))

	var lastErr error
	for _, envID := range envIDs {
		if err := m.DestroyEnv(envID); err != nil {
			slog.Warn("销毁 JS 环境失败", "envID", envID, "error", err)
			lastErr = err
		}
	}

	return lastErr
}

// Close 关闭所有 JS 运行时环境
func (m *JSEnvManager) Close() error {
	// 先广播关闭信号，让 ExecuteJSAndWaitEvents 等 polling 操作尽快释放 env 锁，
	// 否则下面的 env.mu.Lock 会无限期阻塞。
	m.SignalShutdown()

	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("关闭 JSEnvManager")

	var lastErr error
	for envID, env := range m.envs {
		// 短时等锁（shutdownCh 已关闭，polling 循环应当在 ~10ms 内退出）；
		// 若仍拿不到锁说明有阻塞性 JS（罕见），跳过 vm.Close 避免并发 UB——
		// 反正进程马上退出，OS 会回收内存。
		if !tryLockWithTimeout(&env.mu, 3*time.Second) {
			slog.Warn("关闭 JSEnvManager: env 锁等待超时，跳过 VM 关闭", "envID", envID)
			continue
		}
		if err := env.vm.Close(); err != nil {
			slog.Warn("关闭 JS VM 失败", "envID", envID, "error", err)
			lastErr = err
		}
		env.mu.Unlock()
	}

	m.envs = make(map[string]*JSEnv)
	m.pluginEnvs = make(map[int64]map[string]bool)

	return lastErr
}

// tryLockWithTimeout 在 timeout 内尝试获取锁；成功返回 true 并已持有锁，超时返回 false。
func tryLockWithTimeout(mu *sync.Mutex, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if mu.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// HasActiveWebSockets 检查指定环境是否有活跃的 WebSocket 连接。
// 由 HealthChecker.checkIdle 调用，防止有活跃连接的插件被休眠。
func (m *JSEnvManager) HasActiveWebSockets(envID string) bool {
	env, err := m.getEnv(envID)
	if err != nil {
		return false
	}
	hasActive := false
	env.wsConns.Range(func(_, value any) bool {
		wsc := value.(*wsConn)
		if !wsc.closed.Load() {
			hasActive = true
			return false // 找到一个就够了
		}
		return true
	})
	return hasActive
}

// SetBridgeCallback 为指定环境设置桥接回调（__go_bridge 的处理函数）
// 必须在执行调用 __go_bridge 的代码之前调用
func (m *JSEnvManager) SetBridgeCallback(envID string, cb BridgeCallback) error {
	env, err := m.getEnv(envID)
	if err != nil {
		return err
	}
	env.bridgeCallback = cb
	return nil
}

// PostHostEvent 将宿主侧事件非阻塞投递到指定 JS 环境。
// 事件会在 ExecuteJS 的 await 循环或后台 ProcessTimers tick 中分发给 JS。
func (m *JSEnvManager) PostHostEvent(envID, eventType, id, data string) error {
	env, err := m.getEnv(envID)
	if err != nil {
		return err
	}

	select {
	case env.hostEvents <- hostEvent{Type: eventType, ID: id, Data: data}:
	default:
		return fmt.Errorf("host event queue full: env=%s type=%s id=%s", envID, eventType, id)
	}

	select {
	case env.asyncSignal <- struct{}{}:
	default:
	}
	return nil
}

// ProbeStatus 是 HealthProbe 的返回值。直接放在 jsruntime 包以保持
// runtime 与 jsplugin 包之间无环依赖。
type ProbeStatus int

const (
	ProbeStatusHealthy   ProbeStatus = iota // VM 存活且抢到锁、eval 1+1=2
	ProbeStatusUnhealthy                    // 抢到锁但 eval 失败 → VM 已坏
	ProbeStatusBusy                         // 没抢到锁 → 当前正在跑请求，非死亡
	ProbeStatusMissing                      // env 不在 map 中 → 已被销毁
)

// HealthProbe 是一个轻量的 VM 存活探针。
//
// 关键设计：通过 TryLock 直接访问 VM，不经过 ServiceScheduler 的串行队列。
// 这样长 fetch 持锁期间，探针会返回 ProbeStatusBusy 而不是被排队 5s 后超时。
// 调用方（HealthChecker）将 Busy 视作活着，从而不会把"忙"误判为"死"。
//
// 仅在 TryLock 成功后调用 vm.EvalValue("1+1")，必须断言结果 == 2 以验证 VM
// 没有被 jsjiami 等代码破坏。
func (m *JSEnvManager) HealthProbe(envID string) ProbeStatus {
	env, err := m.getEnv(envID)
	if err != nil {
		return ProbeStatusMissing
	}

	if !env.mu.TryLock() {
		return ProbeStatusBusy
	}
	defer env.mu.Unlock()

	// 设置一个很短的 eval 超时，VM 真死的话 1ms 都跑不出来；
	// 默认的 30s 超时会让 health probe 在罕见死循环场景下被卡住。
	// 探针完成后必须恢复默认超时，否则 ProcessTimers 会继承这个短超时，
	// 导致定时器驱动的长链操作（如语音指令搜索歌曲）被误中断。
	env.vm.SetEvalTimeout(500 * time.Millisecond)
	defer env.vm.SetEvalTimeout(defaultJSTimeout)

	val, err := env.vm.EvalValue("1+1", quickjs.EvalGlobal)
	if err != nil {
		slog.Warn("HealthProbe: eval failed", "envID", envID, "error", err)
		return ProbeStatusUnhealthy
	}
	defer val.Free()

	if r, anyErr := val.Any(); anyErr != nil || valueAsInt(r) != 2 {
		slog.Warn("HealthProbe: unexpected eval result", "envID", envID, "result", r)
		return ProbeStatusUnhealthy
	}

	return ProbeStatusHealthy
}

// valueAsInt 与 valToInt 类似但接收 Any() 已经返回的 interface{}，避免重复 Free。
func valueAsInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

// CompileToBytecode 将指定环境的已加载代码编译为字节码
// 使用 quickjs.VM.Compile 将源码编译为字节码，可通过 EvalBytecode 加载
func (m *JSEnvManager) CompileToBytecode(envID string) ([]byte, error) {
	env, err := m.getEnv(envID)
	if err != nil {
		return nil, err
	}

	env.mu.Lock()
	defer env.mu.Unlock()

	if env.sourceCode == "" {
		return nil, fmt.Errorf("env %q has no source code to compile", envID)
	}

	// 使用 QuickJS VM.Compile 将源码编译为字节码
	bytecode, err := env.vm.Compile(env.sourceCode, quickjs.EvalGlobal)
	if err != nil {
		return nil, fmt.Errorf("compile bytecode for env %q: %w", envID, err)
	}

	slog.Debug("bytecode compiled", "envID", envID, "size", len(bytecode))
	return bytecode, nil
}

// ExecuteJSParallel 并行执行多个 JS 环境中的代码，竞速返回第一个成功结果
// maxConcurrent <= 0 表示全部并行，> 0 表示窗口并发（每批最多 maxConcurrent 个）
func (m *JSEnvManager) ExecuteJSParallel(calls []ParallelCall, maxConcurrent int) (successIndex int, result *ExecuteResult, errs []string) {
	if len(calls) == 0 {
		return -1, nil, nil
	}

	// 初始化错误列表
	errs = make([]string, len(calls))

	// 确定并发窗口大小
	windowSize := len(calls)
	if maxConcurrent > 0 && maxConcurrent < len(calls) {
		windowSize = maxConcurrent
	}

	// 按窗口分批执行
	for start := 0; start < len(calls); start += windowSize {
		end := min(start+windowSize, len(calls))
		batchCalls := calls[start:end]

		// 启动本批 goroutine
		resultCh := make(chan ParallelResult, len(batchCalls))
		for i, call := range batchCalls {
			go func(idx int, c ParallelCall) {
				var execResult *ExecuteResult
				var execErr error
				// 并行执行场景（搜索/批量元数据）目前没有外部 ctx 取消语义，传 Background。
				if len(c.WaitEventNames) > 0 {
					execResult, execErr = m.ExecuteJSAndWaitEvents(context.Background(), c.EnvID, c.Code, c.TimeoutMs, c.WaitEventNames)
				} else {
					execResult, execErr = m.ExecuteJS(context.Background(), c.EnvID, c.Code, c.TimeoutMs)
				}
				resultCh <- ParallelResult{
					Index:  idx,
					Result: execResult,
					Err:    execErr,
				}
			}(start+i, call)
		}

		// 等待本批所有结果
		var batchSuccess *ParallelResult
		remaining := len(batchCalls)
		for remaining > 0 {
			pr := <-resultCh
			remaining--
			if pr.Err != nil {
				errs[pr.Index] = pr.Err.Error()
				slog.Warn("ExecuteJSParallel: 调用失败", "index", pr.Index, "envID", calls[pr.Index].EnvID, "error", pr.Err)
			} else if hasErrorEvent(pr.Result) {
				// 事件层面的失败（如 dispatchError），不算竞速成功
				errMsg := extractEventError(pr.Result)
				errs[pr.Index] = errMsg
				slog.Warn("ExecuteJSParallel: 调用失败（事件错误）", "index", pr.Index, "envID", calls[pr.Index].EnvID, "error", errMsg)
			} else {
				errs[pr.Index] = ""
				// 记录第一个成功结果
				if batchSuccess == nil {
					tmp := pr
					batchSuccess = &tmp
					slog.Info("ExecuteJSParallel: 调用成功", "index", pr.Index, "envID", calls[pr.Index].EnvID)
				}
			}
		}

		// 本批有成功结果，立即返回
		if batchSuccess != nil {
			return batchSuccess.Index, batchSuccess.Result, errs
		}

		slog.Warn("ExecuteJSParallel: 本批全部失败，继续下一批", "batchStart", start, "batchEnd", end)
	}

	// 全部失败
	return -1, nil, errs
}

// hasErrorEvent 检查执行结果的事件中是否包含错误事件（事件名包含 "Error"）
func hasErrorEvent(result *ExecuteResult) bool {
	if result == nil {
		return false
	}
	for _, evt := range result.Events {
		if strings.Contains(evt.Name, "Error") {
			return true
		}
	}
	return false
}

// extractEventError 从事件中提取错误信息
func extractEventError(result *ExecuteResult) string {
	if result == nil {
		return "unknown error"
	}
	for _, evt := range result.Events {
		if strings.Contains(evt.Name, "Error") {
			return fmt.Sprintf("event %s: %s", evt.Name, evt.Data)
		}
	}
	return "unknown error"
}

// --- internal helpers ---

func (m *JSEnvManager) getEnv(envID string) (*JSEnv, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	env, exists := m.envs[envID]
	if !exists {
		return nil, fmt.Errorf("env %s not found", envID)
	}
	return env, nil
}

// valueToString 安全地将 quickjs.Value 转为字符串（避免对象循环引用导致 JSON.stringify 失败）
func valueToString(v *quickjs.Value) string {
	if v.IsUndefined() {
		return ""
	}
	r, err := v.Any()
	if err != nil {
		// 对象可能含循环引用，无法 JSON 序列化，返回空
		return ""
	}
	if r == nil {
		return ""
	}
	switch val := r.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

func eventNames(events []JSEventResult) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.Name
	}
	return names
}

func drainEnvEvents(env *JSEnv) []JSEventResult {
	var events []JSEventResult
	for {
		select {
		case evt := <-env.events:
			events = append(events, evt)
		default:
			return events
		}
	}
}

// ProcessTimers 推进指定环境中的后台任务（非阻塞）。
// 使用 TryLock：如果 VM 正忙（HTTP 请求处理中），立即返回 false，不等待。
// 由外部 ticker goroutine 周期性调用。
// 返回 true 表示至少推进了一项后台工作（异步回包、宿主事件或定时器）。
func (m *JSEnvManager) ProcessTimers(envID string) bool {
	env, err := m.getEnv(envID)
	if err != nil {
		return false
	}

	// TryLock: 如果锁被 HTTP 请求持有，直接跳过本轮
	if !env.mu.TryLock() {
		return false
	}
	defer env.mu.Unlock()

	vm := env.vm
	vm.SetEvalTimeout(defaultJSTimeout)
	didAnyWork := false

	// 单轮 pump→host events→jobs→timers，做几次让 fetch().then(setTimeout(...)) 这种链
	// 在一个 tick 里推进尽量多步；任何一步无产出即可退出，避免长时间占锁。
	for range 8 {
		didWork := false

		// 1. 排空异步桥接结果（让 fetch/songloft.* 的 Promise resolve）
		if pumpAsyncResults(vm) > 0 {
			didWork = true
		}

		// 2. 分发宿主事件（UDP / 入站 WebSocket）
		if pumpHostEvents(vm, env) > 0 {
			didWork = true
		}

		// 3. 执行原生 Promise 微任务（async/await 恢复等）
		if ExecutePendingJobs(vm) > 0 {
			didWork = true
		}

		// 4. 处理一轮到期定时器
		if processExpiredTimers(vm) > 0 {
			didWork = true
		}

		if !didWork {
			break
		}
		didAnyWork = true
	}

	// 排空事件通道（防止阻塞）
	// 注意：日志通过 __go_console 直接输出到 slog，不走事件通道
	// __go_send 事件在定时器回调中极少产生，安全丢弃
	drainEnvEvents(env)

	return didAnyWork
}

// GetNextTimerDeadline 返回下一个定时器的执行时间。
// 无活跃定时器返回零值 time.Time（IsZero() == true）。
// 使用 TryLock，VM 忙时返回当前时间——
// 保守地认为"立即有事做"，让空闲检测跳过本轮休眠决策。
func (m *JSEnvManager) GetNextTimerDeadline(envID string) time.Time {
	env, err := m.getEnv(envID)
	if err != nil {
		return time.Time{}
	}

	if !env.mu.TryLock() {
		return time.Now()
	}
	defer env.mu.Unlock()

	val, err := env.vm.EvalValue(
		"typeof __getNextTimerDeadline === 'function' ? __getNextTimerDeadline() : 0",
		quickjs.EvalGlobal)
	if err != nil {
		return time.Time{}
	}
	ms := int64(valToInt(val))
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// processJobs 处理所有待执行的工作：异步桥接结果 + 原生 Promise 微任务 + 定时器回调。
// 模仿旧 cqjs 的 env_process_jobs 逻辑：
//  1. 排空 asyncResults 通道，让 fetch/songloft.* 等桥接 Promise resolve
//  2. 执行 JS_ExecutePendingJob（原生 async/await 恢复）
//  3. 处理到期定时器
//  4. 如果有工作被执行，重复 1-3
//  5. 如果还有未到期定时器，sleep 后重试（最多连续 500ms 无工作则提前退出）
//
// 注：步骤 1 是必需的——ExecuteJSAndWaitEvents / ProcessTimers 都靠 processJobs
// 推进异步链；若不 pump，子 env 里的 fetch().then(...) 永远不会 resolve。
func processJobs(vm *quickjs.VM) {
	deadline := time.Now().Add(30 * time.Second)
	idleCount := 0
	const maxIdleIterations = 50 // 连续无工作最多等 500ms (50 × 10ms)

	for time.Now().Before(deadline) {
		didWork := false

		// 1. 排空异步桥接结果（fetch / songloft.* 的 Promise resolve）
		if pumped := pumpAsyncResults(vm); pumped > 0 {
			didWork = true
		}

		// 2. 执行原生 Promise 微任务（async/await 恢复等）
		jobCount := ExecutePendingJobs(vm)
		if jobCount > 0 {
			didWork = true
		}

		// 3. 处理到期定时器
		timerCount := processExpiredTimers(vm)
		if timerCount > 0 {
			didWork = true
		}

		// 如果有工作被执行，继续循环（可能产生新的微任务/定时器）
		if didWork {
			idleCount = 0
			continue
		}

		// 3. 检查是否还有待执行的定时器
		pending := getPendingTimerCount(vm)
		if pending == 0 {
			break
		}

		// 连续无工作超过 maxIdleIterations 次，提前退出避免长时间阻塞
		idleCount++
		if idleCount >= maxIdleIterations {
			break
		}

		// 还有未到期的定时器，sleep 后重试
		time.Sleep(10 * time.Millisecond)
	}
}

// processExpiredTimers 处理所有已到期的 JS 定时器，返回触发数量
func processExpiredTimers(vm *quickjs.VM) int {
	fired := 0
	for range 100 {
		val, err := vm.EvalValue("typeof __processExpiredTimers === 'function' ? __processExpiredTimers() : 0", quickjs.EvalGlobal)
		if err != nil {
			break
		}
		count := valToInt(val)
		if count == 0 {
			break
		}
		fired += count
	}
	return fired
}

// getPendingTimerCount 获取待执行的一次性定时器数量（不含 interval 定时器）
// interval 定时器会一直存在，不应阻止 processJobs 退出
func getPendingTimerCount(vm *quickjs.VM) int {
	val, err := vm.EvalValue("typeof __getPendingOneShotTimerCount === 'function' ? __getPendingOneShotTimerCount() : (typeof __timers !== 'undefined' ? __timers.size : 0)", quickjs.EvalGlobal)
	if err != nil {
		return 0
	}
	return valToInt(val)
}

// valToInt 安全地从 quickjs.Value 提取整数
func valToInt(val quickjs.Value) int {
	r, err := val.Any()
	val.Free()
	if err != nil {
		return 0
	}
	switch v := r.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

// --- Go 桥接函数注册 ---

func registerBridgeFunctions(vm *quickjs.VM, env *JSEnv) error {
	// __go_send(name, data) — 事件派发
	if err := vm.RegisterFunc("__go_send", func(name, data string) {
		slog.Debug("__go_send called", "envID", env.envID, "name", name, "dataLen", len(data))
		select {
		case env.events <- JSEventResult{EnvID: env.envID, Name: name, Data: data}:
		default:
			slog.Warn("JS event channel full, dropping event", "envID", env.envID, "name", name)
		}
	}, false); err != nil {
		return fmt.Errorf("register __go_send: %w", err)
	}

	// __go_console(level, msg) — console 日志
	if err := vm.RegisterFunc("__go_console", func(level, msg string) {
		switch strings.ToUpper(level) {
		case "ERROR":
			slog.Error("[JS]"+msg, "envID", env.envID)
		case "WARN":
			slog.Warn("[JS]"+msg, "envID", env.envID)
		case "DEBUG":
			slog.Info("[JS]"+msg, "envID", env.envID)
		default:
			slog.Info("[JS]"+msg, "envID", env.envID)
		}
	}, false); err != nil {
		return fmt.Errorf("register __go_console: %w", err)
	}

	// __go_fetch_async(url, method, headersJSON, bodyHex) -> id string
	// 真异步 HTTP：立即返回 id（形如 "fetch:42"），后台 goroutine 跑 HTTP，
	// 完成后把结果投递到 env.asyncResults，由事件循环 resolve 对应 Promise。
	// 详见 polyfill.go 中的 fetch / __resolveAsync 设计说明。
	if err := vm.RegisterFunc("__go_fetch_async", func(url, method, headersJSON, bodyHex string) string {
		seq := env.asyncSeq.Add(1)
		id := fmt.Sprintf("fetch:%d", seq)
		env.asyncInflight.Add(1)
		go func() {
			defer env.asyncInflight.Add(-1)
			payload := doHTTPRequest(url, method, headersJSON, bodyHex)
			result := asyncResult{
				ID:   id,
				Type: "fetch",
				OK:   true,
				Data: payload,
			}
			// 通道满（>256 in-flight）时丢弃 + log。这种情况下 JS 侧的
			// Promise 会一直 pending，由 ExecuteJS 的 wall-clock 超时兜底
			// reject。避免 goroutine 泄漏比单条 reject 更重要。
			select {
			case env.asyncResults <- result:
			default:
				slog.Warn("asyncResults channel full, drop fetch result",
					"envID", env.envID, "id", id, "url", url)
			}
			// 非阻塞唤醒事件循环；容量 1，多次 send 自然合并。
			select {
			case env.asyncSignal <- struct{}{}:
			default:
			}
		}()
		return id
	}, false); err != nil {
		return fmt.Errorf("register __go_fetch_async: %w", err)
	}

	// __go_pop_async_result() -> "" or framed result
	// 由 JS 侧 __pumpAsyncResults 调用，非阻塞地从 asyncResults 通道弹出
	// 一个就绪结果。空队列返回 ""；JS 侧据此结束 pump 循环。
	//
	// 分隔符分帧："<id>\t<ok>\t<type>\n<raw data>"。不用 json.Marshal envelope，
	// 因为 data 常是大 JSON 字符串（如 songs.list 整库），封装进 JSON 会把它整体
	// 再转义一次（Go 侧），JS 侧还要多 JSON.parse 一遍 wrapper。分帧让 data 原样
	// 透传，JS 只按第一个 '\n' 切出 header。id/type 均为受控字符串（"bridge:42"/
	// "fetch"/"ws_msg" 等），不含 '\t'/'\n'，data 部分内容任意也不影响切分。
	if err := vm.RegisterFunc("__go_pop_async_result", func() string {
		select {
		case r := <-env.asyncResults:
			ok := byte('0')
			if r.OK {
				ok = '1'
			}
			var sb strings.Builder
			sb.Grow(len(r.ID) + len(r.Type) + len(r.Data) + 4)
			sb.WriteString(r.ID)
			sb.WriteByte('\t')
			sb.WriteByte(ok)
			sb.WriteByte('\t')
			sb.WriteString(r.Type)
			sb.WriteByte('\n')
			sb.WriteString(r.Data)
			return sb.String()
		default:
			return ""
		}
	}, false); err != nil {
		return fmt.Errorf("register __go_pop_async_result: %w", err)
	}

	// __go_now_ms() — 当前时间毫秒戳
	if err := vm.RegisterFunc("__go_now_ms", func() int64 {
		return time.Now().UnixMilli()
	}, false); err != nil {
		return fmt.Errorf("register __go_now_ms: %w", err)
	}

	// __go_buffer_from(data, encoding) — Buffer.from 桥接
	if err := vm.RegisterFunc("__go_buffer_from", goBufferFrom, false); err != nil {
		return fmt.Errorf("register __go_buffer_from: %w", err)
	}

	// __go_buffer_to_string(dataHex, encoding) — Buffer.toString 桥接
	if err := vm.RegisterFunc("__go_buffer_to_string", goBufferToString, false); err != nil {
		return fmt.Errorf("register __go_buffer_to_string: %w", err)
	}

	// __go_crypto_md5(str) — MD5 hex
	if err := vm.RegisterFunc("__go_crypto_md5", func(str string) string {
		h := md5.Sum([]byte(str))
		return hex.EncodeToString(h[:])
	}, false); err != nil {
		return fmt.Errorf("register __go_crypto_md5: %w", err)
	}

	// __go_crypto_sha256(str) — SHA256 hex（输入按 UTF-8 字节）
	if err := vm.RegisterFunc("__go_crypto_sha256", func(str string) string {
		h := sha256.Sum256([]byte(str))
		return hex.EncodeToString(h[:])
	}, false); err != nil {
		return fmt.Errorf("register __go_crypto_sha256: %w", err)
	}

	// __go_crypto_sha1(str) — SHA1 hex（输入按 UTF-8 字节）。仅为兼容小米等旧 API
	// 的签名（clientSign = base64(sha1("nonce=...&ssecurity"))）；SHA1 已不安全，
	// 不要用于新的安全场景。
	if err := vm.RegisterFunc("__go_crypto_sha1", func(str string) string {
		h := sha1.Sum([]byte(str))
		return hex.EncodeToString(h[:])
	}, false); err != nil {
		return fmt.Errorf("register __go_crypto_sha1: %w", err)
	}

	// __go_crypto_sha256_bytes(dataHex) — 对任意二进制做 SHA256（hex 入，hex 出）。
	// 与 __go_crypto_sha256(str) 的区别：后者只能哈希 UTF-8 字符串，无法喂任意字节。
	// 供需要哈希二进制的插件（如 miot 签名 sha256(key+nonce)）把纯 JS sha256 换成原生。
	if err := vm.RegisterFunc("__go_crypto_sha256_bytes", func(dataHex string) string {
		data, err := hex.DecodeString(dataHex)
		if err != nil {
			slog.Warn("__go_crypto_sha256_bytes: bad hex input", "error", err)
			return ""
		}
		h := sha256.Sum256(data)
		return hex.EncodeToString(h[:])
	}, false); err != nil {
		return fmt.Errorf("register __go_crypto_sha256_bytes: %w", err)
	}

	// __go_crypto_rc4(keyHex, dataHex) — RC4 流加密（hex 入，hex 出）。
	// QuickJS 无 WebCrypto，插件（如 miot 的小米 API）以往在纯 JS 里实现 RC4
	// （S 盒 256 轮 + 逐字节异或），在解释执行下极慢（~1.28ms/次）。原生仅 ~µs。
	if err := vm.RegisterFunc("__go_crypto_rc4", func(keyHex, dataHex string) string {
		key, err := hex.DecodeString(keyHex)
		if err != nil || len(key) == 0 {
			slog.Warn("__go_crypto_rc4: bad key hex", "error", err)
			return ""
		}
		data, err := hex.DecodeString(dataHex)
		if err != nil {
			slog.Warn("__go_crypto_rc4: bad data hex", "error", err)
			return ""
		}
		c, err := rc4.NewCipher(key)
		if err != nil {
			slog.Warn("__go_crypto_rc4: new cipher", "error", err)
			return ""
		}
		out := make([]byte, len(data))
		c.XORKeyStream(out, data)
		return hex.EncodeToString(out)
	}, false); err != nil {
		return fmt.Errorf("register __go_crypto_rc4: %w", err)
	}

	// __go_crypto_random_bytes(size) — 随机字节 hex
	// 注意：返回值必须是 string（不能是 (string, error)），
	// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]。
	if err := vm.RegisterFunc("__go_crypto_random_bytes", func(size int) string {
		buf := make([]byte, size)
		if _, err := rand.Read(buf); err != nil {
			slog.Error("__go_crypto_random_bytes error", "error", err)
			return ""
		}
		return hex.EncodeToString(buf)
	}, false); err != nil {
		return fmt.Errorf("register __go_crypto_random_bytes: %w", err)
	}

	// __go_crypto_aes_encrypt(dataHex, mode, keyHex, ivHex) — AES 加密
	if err := vm.RegisterFunc("__go_crypto_aes_encrypt", goCryptoAesEncrypt, false); err != nil {
		return fmt.Errorf("register __go_crypto_aes_encrypt: %w", err)
	}

	// __go_crypto_aes_decrypt(dataHex, mode, keyHex, ivHex) — AES 解密
	if err := vm.RegisterFunc("__go_crypto_aes_decrypt", goCryptoAesDecrypt, false); err != nil {
		return fmt.Errorf("register __go_crypto_aes_decrypt: %w", err)
	}

	// __go_crypto_rsa_encrypt(dataHex, keyPEM) — RSA 公钥加密
	if err := vm.RegisterFunc("__go_crypto_rsa_encrypt", goCryptoRsaEncrypt, false); err != nil {
		return fmt.Errorf("register __go_crypto_rsa_encrypt: %w", err)
	}

	// __go_zlib_inflate(dataHex) — zlib 解压
	if err := vm.RegisterFunc("__go_zlib_inflate", goZlibInflate, false); err != nil {
		return fmt.Errorf("register __go_zlib_inflate: %w", err)
	}

	// __go_zlib_deflate(dataHex) — zlib 压缩
	if err := vm.RegisterFunc("__go_zlib_deflate", goZlibDeflate, false); err != nil {
		return fmt.Errorf("register __go_zlib_deflate: %w", err)
	}

	// __go_raw_inflate(dataHex) — raw DEFLATE 解压（无 zlib 头，用于 ZIP 文件解析）
	if err := vm.RegisterFunc("__go_raw_inflate", goRawInflate, false); err != nil {
		return fmt.Errorf("register __go_raw_inflate: %w", err)
	}

	// --- WebSocket 桥接函数 ---

	// __go_ws_connect_async(url, headersJSON) -> id string
	// 异步连接 WebSocket，返回 asyncResult ID。成功时 data 为 connId，
	// 随后读循环自动推送 ws_msg / ws_close / ws_err 事件。
	if err := vm.RegisterFunc("__go_ws_connect_async", func(url, headersJSON string) string {
		seq := env.asyncSeq.Add(1)
		id := fmt.Sprintf("ws_connect:%d", seq)
		env.asyncInflight.Add(1)

		go func() {
			defer env.asyncInflight.Add(-1)

			header := http.Header{}
			if headersJSON != "" && headersJSON != "{}" {
				var h map[string]string
				if err := json.Unmarshal([]byte(headersJSON), &h); err == nil {
					for k, v := range h {
						header.Set(k, v)
					}
				}
			}

			dialer := websocket.Dialer{
				HandshakeTimeout: 15 * time.Second,
			}
			conn, resp, err := dialer.Dial(url, header)
			if err != nil {
				env.asyncResults <- asyncResult{ID: id, Type: "ws_connect", OK: false, Data: formatWebSocketDialError(err, resp)}
				select {
				case env.asyncSignal <- struct{}{}:
				default:
				}
				return
			}

			connSeq := env.wsConnSeq.Add(1)
			connId := fmt.Sprintf("ws_%d", connSeq)
			wsc := &wsConn{id: connId, conn: conn}
			env.wsConns.Store(connId, wsc)

			env.asyncResults <- asyncResult{ID: id, Type: "ws_connect", OK: true, Data: connId}
			select {
			case env.asyncSignal <- struct{}{}:
			default:
			}

			// 读循环：接收消息并推送到 asyncResults
			go func() {
				for {
					messageType, data, err := conn.ReadMessage()
					if err != nil {
						if wsc.closed.Load() {
							// 主动关闭，推送 ws_close
							closeData, _ := json.Marshal(map[string]any{
								"connId": connId, "code": 1000, "reason": "",
							})
							env.asyncResults <- asyncResult{
								ID: connId, Type: "ws_close", OK: true, Data: string(closeData),
							}
						} else {
							closeErr := websocket.IsCloseError(err,
								websocket.CloseNormalClosure,
								websocket.CloseGoingAway,
							)
							if closeErr {
								closeData, _ := json.Marshal(map[string]any{
									"connId": connId, "code": 1001, "reason": err.Error(),
								})
								env.asyncResults <- asyncResult{
									ID: connId, Type: "ws_close", OK: true, Data: string(closeData),
								}
							} else {
								env.asyncResults <- asyncResult{
									ID: connId, Type: "ws_err", OK: true, Data: err.Error(),
								}
							}
						}
						wsc.closed.Store(true)
						env.wsConns.Delete(connId)
						select {
						case env.asyncSignal <- struct{}{}:
						default:
						}
						return
					}

					isBinary := messageType == websocket.BinaryMessage
					dataHex := hex.EncodeToString(data)
					msgJSON, _ := json.Marshal(map[string]any{
						"connId":   connId,
						"dataHex":  dataHex,
						"isBinary": isBinary,
					})
					select {
					case env.asyncResults <- asyncResult{
						ID: connId, Type: "ws_msg", OK: true, Data: string(msgJSON),
					}:
					default:
						slog.Warn("asyncResults channel full, drop ws_msg",
							"envID", env.envID, "connId", connId)
					}
					select {
					case env.asyncSignal <- struct{}{}:
					default:
					}
				}
			}()
		}()
		return id
	}, false); err != nil {
		return fmt.Errorf("register __go_ws_connect_async: %w", err)
	}

	// __go_ws_send(connId, dataHex, isBinary) -> errMsg
	if err := vm.RegisterFunc("__go_ws_send", func(connId, dataHex string, isBinary bool) string {
		val, ok := env.wsConns.Load(connId)
		if !ok {
			return "connection not found: " + connId
		}
		wsc := val.(*wsConn)
		if wsc.closed.Load() {
			return "connection already closed"
		}

		data, err := hex.DecodeString(dataHex)
		if err != nil {
			return "hex decode error: " + err.Error()
		}

		msgType := websocket.TextMessage
		if isBinary {
			msgType = websocket.BinaryMessage
		}

		wsc.mu.Lock()
		err = wsc.conn.WriteMessage(msgType, data)
		wsc.mu.Unlock()
		if err != nil {
			return "write error: " + err.Error()
		}
		return ""
	}, false); err != nil {
		return fmt.Errorf("register __go_ws_send: %w", err)
	}

	// __go_ws_close(connId, code, reason) -> errMsg
	if err := vm.RegisterFunc("__go_ws_close", func(connId string, code int, reason string) string {
		val, ok := env.wsConns.Load(connId)
		if !ok {
			return ""
		}
		wsc := val.(*wsConn)
		if !wsc.closed.CompareAndSwap(false, true) {
			return ""
		}

		closeMsg := websocket.FormatCloseMessage(code, reason)
		wsc.mu.Lock()
		_ = wsc.conn.WriteMessage(websocket.CloseMessage, closeMsg)
		_ = wsc.conn.Close()
		wsc.mu.Unlock()
		return ""
	}, false); err != nil {
		return fmt.Errorf("register __go_ws_close: %w", err)
	}

	// __go_ws_state(connId) -> readyState int (0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED)
	if err := vm.RegisterFunc("__go_ws_state", func(connId string) int {
		val, ok := env.wsConns.Load(connId)
		if !ok {
			return 3 // CLOSED
		}
		wsc := val.(*wsConn)
		if wsc.closed.Load() {
			return 3
		}
		return 1 // OPEN
	}, false); err != nil {
		return fmt.Errorf("register __go_ws_state: %w", err)
	}

	// __go_bridge(action, data) -> id string
	// 真异步桥接：立即返回 id（形如 "bridge:42"），后台 goroutine 调
	// env.bridgeCallback 处理动作（DB / 文件 / 跨插件通信），完成后把结果
	// 投递到 env.asyncResults，由事件循环 resolve 对应 Promise。
	//
	// 即使 storage 这类轻量操作也走异步路径，是为了保证 JS 端 API 一致：
	// 所有 songloft.* 方法返回 Promise<T>，无需为不同 action 区分。
	// 内部开销（goroutine 创建 + 通道发送 + 唤醒事件循环）单次 ~10us，
	// 远小于 SQLite 一次查询 / 文件读写。
	if err := vm.RegisterFunc("__go_bridge", func(action, data string) string {
		seq := env.asyncSeq.Add(1)
		id := fmt.Sprintf("bridge:%d", seq)
		env.asyncInflight.Add(1)

		// 在 goroutine 启动前快照 callback：避免 Stop/重载竞争。
		cb := env.bridgeCallback

		go func() {
			defer env.asyncInflight.Add(-1)
			result := asyncResult{ID: id, Type: "bridge"}
			if cb == nil {
				slog.Error("__go_bridge: no callback registered",
					"envID", env.envID, "action", action)
				result.OK = false
				result.Data = "no bridge callback registered"
			} else {
				out, err := cb(action, data)
				if err != nil {
					slog.Warn("__go_bridge error",
						"envID", env.envID, "action", action, "error", err)
					result.OK = false
					result.Data = err.Error()
				} else {
					result.OK = true
					result.Data = out
				}
			}
			select {
			case env.asyncResults <- result:
			default:
				slog.Warn("asyncResults channel full, drop bridge result",
					"envID", env.envID, "id", id, "action", action)
			}
			select {
			case env.asyncSignal <- struct{}{}:
			default:
			}
		}()
		return id
	}, false); err != nil {
		return fmt.Errorf("register __go_bridge: %w", err)
	}

	return nil
}

// --- Go 桥接函数实现 ---

// doHTTPRequest 执行一次 HTTP 请求并把结果序列化为 JSON 字符串。
//
// 这是 Go 侧的内部实现，不对 JS 直接暴露：JS 侧调 fetch() / __go_fetch_async()
// 触发后台 goroutine 调用本函数，结果通过 asyncResults 通道回投给事件循环。
// 因此函数虽然内部是阻塞 net/http 调用，但 QuickJS VM 锁不会被持有。
//
// 支持 X-Fetch-No-Redirect 请求头：存在时不自动跟随重定向，让 JS 侧处理
// 重定向链（如 xiaomi 登录流程的 Cookie 收集）。
// 支持 X-Fetch-Timeout-Ms 请求头：设置单次请求超时（100-30000ms），该内部头不会转发。
func doHTTPRequest(url, method, headersJSON, bodyHex string) string {
	slog.Debug("doHTTPRequest", "url", url, "method", method, "headers", headersJSON)

	// 解析并设置请求头
	var headers map[string]string
	noRedirect := false
	timeout := 30 * time.Second
	if headersJSON != "" && headersJSON != "{}" {
		if jsonErr := json.Unmarshal([]byte(headersJSON), &headers); jsonErr == nil {
			for k, v := range headers {
				if strings.EqualFold(k, "X-Fetch-No-Redirect") {
					noRedirect = true
					continue // 不传递此内部控制头
				}
				if strings.EqualFold(k, "X-Fetch-Timeout-Ms") {
					if ms, parseErr := strconv.Atoi(strings.TrimSpace(v)); parseErr == nil && ms > 0 {
						if ms < 100 {
							ms = 100
						} else if ms > 30000 {
							ms = 30000
						}
						timeout = time.Duration(ms) * time.Millisecond
					}
					continue
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var reqBody io.Reader
	if bodyHex != "" {
		bodyBytes, err := hex.DecodeString(bodyHex)
		if err != nil {
			return marshalFetchError("request body hex decode error: " + err.Error())
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return marshalFetchError(err.Error())
	}

	if headers != nil {
		for k, v := range headers {
			if strings.EqualFold(k, "X-Fetch-No-Redirect") || strings.EqualFold(k, "X-Fetch-Timeout-Ms") {
				continue
			}
			req.Header.Set(k, v)
		}
	}

	// 补充默认 User-Agent（仅在 JS 侧未显式设置时填充）
	// 不自动补充 Accept/Accept-Encoding/Connection，让请求指纹与 WASM 版本一致
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "MiHome/6.0 (Linux; Android 10; Redmi Note 5 Build/QQ3A.200805.001)")
	}
	httputil.ApplyBasicAuthFromURL(req)
	url = req.URL.String()

	// 诊断日志：记录实际发送的请求头（内部控制头已剥离，默认头已补充）
	// header map 构造仅在 Debug 级别时进行，避免热路径无谓分配。
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		actualHeaders := make(map[string]string, len(req.Header))
		for k, vals := range req.Header {
			if len(vals) > 0 {
				actualHeaders[k] = vals[0]
			}
		}
		slog.Debug("doHTTPRequest actual request headers", "url", url, "noRedirect", noRedirect, "timeout", timeout, "headers", actualHeaders)
	}

	// 根据是否需要跟随重定向选择客户端
	client := sharedHTTPClient
	if noRedirect {
		client = noRedirectHTTPClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return marshalFetchError(err.Error())
	}
	defer resp.Body.Close()

	// 限制读入内存的响应体大小，防止异常/恶意大响应 OOM。
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBodyBytes+1))
	if err != nil {
		return marshalFetchError(err.Error())
	}
	if int64(len(bodyBytes)) > maxFetchBodyBytes {
		return marshalFetchError(fmt.Sprintf("response body exceeds limit of %d MiB", maxFetchBodyBytes>>20))
	}

	// 收集响应头
	respHeaders := make(map[string]string)
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			respHeaders[k] = strings.Join(vals, ", ")
		}
	}

	// 诊断日志：记录响应状态和头信息
	slog.Debug("doHTTPRequest response", "url", url, "status", resp.StatusCode, "headers", respHeaders, "bodyLen", len(bodyBytes))
	if resp.StatusCode == 401 || resp.StatusCode >= 400 {
		slog.Warn("doHTTPRequest error response", "url", url, "status", resp.StatusCode, "body", string(bodyBytes))
	}

	result := map[string]any{
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"headers":    respHeaders,
	}
	// 文本（有效 UTF-8）响应：仅回 body 字符串。JS 侧 text()/json() 直接用 body，
	// arrayBuffer() 通过 __go_buffer_from(body,'utf8') 回退无损还原字节，无需 bodyHex。
	// 二进制响应：body 经 JSON 会损坏且无意义，仅通过 bodyHex 传递，避免冗余双份编码。
	if utf8.Valid(bodyBytes) {
		result["body"] = string(bodyBytes)
	} else {
		result["body"] = ""
		result["bodyHex"] = hex.EncodeToString(bodyBytes)
	}

	data, _ := json.Marshal(result)
	return string(data)
}

func marshalFetchError(msg string) string {
	result := map[string]string{"error": msg}
	data, _ := json.Marshal(result)
	return string(data)
}

func formatWebSocketDialError(err error, resp *http.Response) string {
	if resp == nil {
		return err.Error()
	}
	body := ""
	if resp.Body != nil {
		defer resp.Body.Close()
		if b, readErr := io.ReadAll(io.LimitReader(resp.Body, 512)); readErr == nil {
			body = strings.TrimSpace(string(b))
		}
	}
	if body == "" {
		return fmt.Sprintf("%s (HTTP %d %s)", err.Error(), resp.StatusCode, resp.Status)
	}
	return fmt.Sprintf("%s (HTTP %d %s): %s", err.Error(), resp.StatusCode, resp.Status, body)
}

// goBufferFrom 将数据按指定编码转为 hex 内部表示
// 注意：返回值必须是 string（不能是 (string, error)），
// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]，
// 导致 JS 端收到的是数组而非字符串。
func goBufferFrom(data, encoding string) string {
	switch strings.ToLower(encoding) {
	case "utf8", "utf-8", "":
		return hex.EncodeToString([]byte(data))
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			// 尝试 RawStdEncoding
			decoded, err = base64.RawStdEncoding.DecodeString(data)
			if err != nil {
				slog.Error("goBufferFrom base64 decode error", "error", err)
				return ""
			}
		}
		return hex.EncodeToString(decoded)
	case "hex":
		// 验证 hex 有效性
		if _, err := hex.DecodeString(data); err != nil {
			slog.Error("goBufferFrom hex validation error", "error", err)
			return ""
		}
		return data
	case "binary", "latin1":
		buf := make([]byte, len(data))
		for i := 0; i < len(data); i++ {
			buf[i] = data[i]
		}
		return hex.EncodeToString(buf)
	default:
		return hex.EncodeToString([]byte(data))
	}
}

// goBufferToString 将 hex 内部表示转为指定编码字符串
// 注意：返回值必须是 string（不能是 (string, error)），
// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]。
func goBufferToString(dataHex, encoding string) string {
	decoded, err := hex.DecodeString(dataHex)
	if err != nil {
		slog.Error("goBufferToString hex decode error", "error", err, "dataHexLen", len(dataHex))
		return ""
	}

	switch strings.ToLower(encoding) {
	case "utf8", "utf-8", "":
		return string(decoded)
	case "base64":
		return base64.StdEncoding.EncodeToString(decoded)
	case "hex":
		return dataHex
	case "binary", "latin1":
		return string(decoded)
	default:
		return string(decoded)
	}
}

// goCryptoAesEncrypt AES 加密
// 注意：返回值必须是 string（不能是 (string, error)），
// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]。
func goCryptoAesEncrypt(dataHex, mode, keyHex, ivHex string) string {
	data, err := hex.DecodeString(dataHex)
	if err != nil {
		slog.Error("goCryptoAesEncrypt data hex decode error", "error", err)
		return ""
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		slog.Error("goCryptoAesEncrypt key hex decode error", "error", err)
		return ""
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		slog.Error("goCryptoAesEncrypt aes new cipher error", "error", err)
		return ""
	}

	// PKCS7 padding
	blockSize := block.BlockSize()
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	data = append(data, padText...)

	encrypted := make([]byte, len(data))

	switch strings.ToLower(mode) {
	case "cbc":
		iv, err := hex.DecodeString(ivHex)
		if err != nil {
			slog.Error("goCryptoAesEncrypt iv hex decode error", "error", err)
			return ""
		}
		cbc := cipher.NewCBCEncrypter(block, iv)
		cbc.CryptBlocks(encrypted, data)
	case "ecb":
		for i := 0; i < len(data); i += blockSize {
			block.Encrypt(encrypted[i:i+blockSize], data[i:i+blockSize])
		}
	default:
		slog.Error("goCryptoAesEncrypt unsupported mode", "mode", mode)
		return ""
	}

	return hex.EncodeToString(encrypted)
}

// goCryptoAesDecrypt AES 解密
// 注意：返回值必须是 string（不能是 (string, error)），
// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]。
func goCryptoAesDecrypt(dataHex, mode, keyHex, ivHex string) string {
	data, err := hex.DecodeString(dataHex)
	if err != nil {
		slog.Error("goCryptoAesDecrypt data hex decode error", "error", err)
		return ""
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		slog.Error("goCryptoAesDecrypt key hex decode error", "error", err)
		return ""
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		slog.Error("goCryptoAesDecrypt aes new cipher error", "error", err)
		return ""
	}

	blockSize := block.BlockSize()
	if len(data) == 0 || len(data)%blockSize != 0 {
		slog.Error("goCryptoAesDecrypt invalid data length", "dataLen", len(data), "blockSize", blockSize)
		return ""
	}

	decrypted := make([]byte, len(data))
	switch strings.ToLower(mode) {
	case "cbc":
		iv, err := hex.DecodeString(ivHex)
		if err != nil {
			slog.Error("goCryptoAesDecrypt iv hex decode error", "error", err)
			return ""
		}
		if len(iv) != blockSize {
			slog.Error("goCryptoAesDecrypt invalid iv length", "ivLen", len(iv), "blockSize", blockSize)
			return ""
		}
		cbc := cipher.NewCBCDecrypter(block, iv)
		cbc.CryptBlocks(decrypted, data)
	case "ecb":
		for i := 0; i < len(data); i += blockSize {
			block.Decrypt(decrypted[i:i+blockSize], data[i:i+blockSize])
		}
	default:
		slog.Error("goCryptoAesDecrypt unsupported mode", "mode", mode)
		return ""
	}

	padding := int(decrypted[len(decrypted)-1])
	if padding <= 0 || padding > blockSize || padding > len(decrypted) {
		slog.Error("goCryptoAesDecrypt invalid pkcs7 padding", "padding", padding, "dataLen", len(decrypted))
		return ""
	}
	for i := len(decrypted) - padding; i < len(decrypted); i++ {
		if int(decrypted[i]) != padding {
			slog.Error("goCryptoAesDecrypt invalid pkcs7 padding content")
			return ""
		}
	}

	return hex.EncodeToString(decrypted[:len(decrypted)-padding])
}

// goCryptoRsaEncrypt RSA 公钥加密
// 注意：返回值必须是 string（不能是 (string, error)），
// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]。
func goCryptoRsaEncrypt(dataHex, keyPEM string) string {
	data, err := hex.DecodeString(dataHex)
	if err != nil {
		slog.Error("goCryptoRsaEncrypt data hex decode error", "error", err)
		return ""
	}

	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		slog.Error("goCryptoRsaEncrypt failed to decode PEM block")
		return ""
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// 尝试 PKCS1
		pubKey, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 != nil {
			slog.Error("goCryptoRsaEncrypt parse public key error", "error", err)
			return ""
		}
		pub = pubKey
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		slog.Error("goCryptoRsaEncrypt not an RSA public key")
		return ""
	}

	// 老牌音源平台（网易云等）的握手协议固定要求 PKCS#1 v1.5；不能改 OAEP。
	//lint:ignore SA1019 required by upstream music platform protocols (NetEase etc.)
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, data)
	if err != nil {
		slog.Error("goCryptoRsaEncrypt rsa encrypt error", "error", err)
		return ""
	}

	return hex.EncodeToString(encrypted)
}

// goZlibInflate zlib 解压
// 注意：返回值必须是 string（不能是 (string, error)），
// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]。
func goZlibInflate(dataHex string) string {
	data, err := hex.DecodeString(dataHex)
	if err != nil {
		slog.Error("goZlibInflate hex decode error", "error", err)
		return ""
	}

	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		slog.Error("goZlibInflate zlib new reader error", "error", err)
		return ""
	}
	defer reader.Close()

	result, err := io.ReadAll(reader)
	if err != nil {
		slog.Error("goZlibInflate zlib read error", "error", err)
		return ""
	}

	return hex.EncodeToString(result)
}

// goZlibDeflate zlib 压缩
// 注意：返回值必须是 string（不能是 (string, error)），
// 因为 QuickJS 的 RegisterFunc 会将多返回值包装为数组 [value, error]。
func goZlibDeflate(dataHex string) string {
	data, err := hex.DecodeString(dataHex)
	if err != nil {
		slog.Error("goZlibDeflate hex decode error", "error", err)
		return ""
	}

	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		slog.Error("goZlibDeflate zlib write error", "error", err)
		return ""
	}
	if err := writer.Close(); err != nil {
		slog.Error("goZlibDeflate zlib close error", "error", err)
		return ""
	}

	return hex.EncodeToString(buf.Bytes())
}

// goRawInflate raw DEFLATE 解压（无 zlib 头/尾），用于 ZIP 文件中的 DEFLATE 压缩数据
func goRawInflate(dataHex string) string {
	data, err := hex.DecodeString(dataHex)
	if err != nil {
		slog.Error("goRawInflate hex decode error", "error", err)
		return ""
	}

	reader := flate.NewReader(bytes.NewReader(data))
	defer reader.Close()

	result, err := io.ReadAll(reader)
	if err != nil {
		slog.Error("goRawInflate flate read error", "error", err)
		return ""
	}

	return hex.EncodeToString(result)
}
