package jsplugin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// MessageType 消息类型
type MessageType int

const (
	MsgHTTPRequest MessageType = iota // HTTP 路由请求
	MsgTimerFire                      // 定时器触发
	MsgInterPlugin                    // 插件间通信
	MsgLifecycle                      // 生命周期事件（init/deinit）
	MsgHostCall                       // 宿主函数调用结果
	MsgHealthCheck                    // 健康检查
	MsgPlayEvent                      // 播放事件（play/finish/skip）
	MsgNetData                        // 网络数据事件（UDP 收包推送）
)

const (
	defaultQueueSize   = 256
	defaultCallTimeout = 30 * time.Second
)

// 错误定义
var (
	ErrSchedulerClosed     = errors.New("scheduler: closed")
	ErrServiceNotFound     = errors.New("scheduler: service not found")
	ErrServiceExists       = errors.New("scheduler: service already registered")
	ErrQueueFull           = errors.New("scheduler: message queue full (backpressure)")
	ErrCallTimeout         = errors.New("scheduler: call timeout")
	ErrServiceUnregistered = errors.New("scheduler: service unregistered during call")
)

// Message 统一消息格式（借鉴 Skynet 的 session + type 机制）
type Message struct {
	ID       uint64          // 全局递增消息 ID
	Type     MessageType     // 消息类型
	Source   string          // 发送方 service name（空字符串表示系统消息）
	Target   string          // 目标 service name
	Session  uint64          // 请求-响应配对 session
	Data     interface{}     // 消息体（具体类型由 MessageType 决定）
	RespChan chan *Message   // 同步调用的响应通道（nil 表示异步/不需要响应）
	Ctx      context.Context // 请求上下文（用于超时控制）
}

// MessageHandler 消息处理接口
type MessageHandler interface {
	HandleMessage(msg *Message) *Message // 处理消息，返回响应（可为 nil）
}

// serviceEntry 内部服务条目
type serviceEntry struct {
	name     string
	msgQueue chan *Message
	handler  MessageHandler
	cancel   context.CancelFunc
	done     chan struct{} // worker 退出信号
	closed   atomic.Bool   // 标记是否已关闭入口
}

// ServiceScheduler 借鉴 Skynet 的消息分发机制
type ServiceScheduler struct {
	services   map[string]*serviceEntry
	mu         sync.RWMutex
	msgIDSeq   atomic.Uint64
	sessionSeq atomic.Uint64
	workerSize int
	closed     atomic.Bool
}

// NewServiceScheduler 创建调度器
func NewServiceScheduler(workerSize int) *ServiceScheduler {
	if workerSize <= 0 {
		workerSize = 1
	}
	return &ServiceScheduler{
		services:   make(map[string]*serviceEntry),
		workerSize: workerSize,
	}
}

// RegisterService 注册一个服务
func (s *ServiceScheduler) RegisterService(name string, handler MessageHandler, queueSize int) error {
	if s.closed.Load() {
		return ErrSchedulerClosed
	}
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.services[name]; exists {
		return ErrServiceExists
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &serviceEntry{
		name:     name,
		msgQueue: make(chan *Message, queueSize),
		handler:  handler,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	s.services[name] = entry

	// 启动 worker goroutine
	go s.runWorker(ctx, entry)

	return nil
}

// runWorker 服务的 worker 协程，串行处理消息
func (s *ServiceScheduler) runWorker(ctx context.Context, entry *serviceEntry) {
	defer close(entry.done)

	for {
		select {
		case msg, ok := <-entry.msgQueue:
			if !ok {
				// channel 已关闭，排空处理
				return
			}
			s.processMessage(entry, msg)
		case <-ctx.Done():
			// 上下文取消，排空剩余消息
			s.drainQueue(entry)
			return
		}
	}
}

// processMessage 处理单条消息
func (s *ServiceScheduler) processMessage(entry *serviceEntry, msg *Message) {
	// 客户端在排队期间已经放弃（如用户快速切歌触发 abort）的请求，直接跳过，
	// 避免 worker 被串行化的 ExecuteJS 卡住，新切的歌排在它后面一直 pending。
	// 已取消的消息往往 RespChan 也无人接收（Call 已 return），无需回填 resp。
	if msg.Ctx != nil && msg.Ctx.Err() != nil {
		if msg.RespChan != nil {
			// 兜底回个 nil，防止某些等待者卡死；Call 端已被 callCtx 唤醒，
			// 不会真正消费这里的值。
			select {
			case msg.RespChan <- nil:
			default:
			}
		}
		return
	}

	resp := entry.handler.HandleMessage(msg)
	if msg.RespChan != nil {
		select {
		case msg.RespChan <- resp:
		default:
			// 响应通道已满或无人接收，丢弃
		}
	}
}

// drainQueue 排空消息队列中的剩余消息
func (s *ServiceScheduler) drainQueue(entry *serviceEntry) {
	for {
		select {
		case msg, ok := <-entry.msgQueue:
			if !ok {
				return
			}
			s.processMessage(entry, msg)
		default:
			return
		}
	}
}

// UnregisterService 注销服务（等待队列消息处理完毕或超时）
func (s *ServiceScheduler) UnregisterService(name string, timeout time.Duration) error {
	s.mu.Lock()
	entry, exists := s.services[name]
	if !exists {
		s.mu.Unlock()
		return ErrServiceNotFound
	}
	// 标记为已关闭，不再接受新消息
	entry.closed.Store(true)
	// 从注册表移除
	delete(s.services, name)
	s.mu.Unlock()

	// 关闭消息队列入口，worker 会处理完剩余消息
	close(entry.msgQueue)

	// 等待 worker 完成或超时
	if timeout <= 0 {
		timeout = defaultCallTimeout
	}
	select {
	case <-entry.done:
		return nil
	case <-time.After(timeout):
		entry.cancel()
		<-entry.done
		return fmt.Errorf("scheduler: unregister %q timed out, worker force stopped", name)
	}
}

// Send 异步发送消息（不等待响应）
func (s *ServiceScheduler) Send(msg *Message) error {
	if s.closed.Load() {
		return ErrSchedulerClosed
	}
	msg.ID = s.msgIDSeq.Add(1)
	msg.RespChan = nil // 异步发送不需要响应通道
	return s.dispatch(msg)
}

// Call 同步调用（等待响应或超时）
// timeout 为 0 时使用默认 30s 超时
func (s *ServiceScheduler) Call(ctx context.Context, target, source string, msgType MessageType, data interface{}, timeout time.Duration) (*Message, error) {
	if s.closed.Load() {
		return nil, ErrSchedulerClosed
	}
	if timeout <= 0 {
		timeout = defaultCallTimeout
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session := s.sessionSeq.Add(1)
	respChan := make(chan *Message, 1)

	msg := &Message{
		ID:       s.msgIDSeq.Add(1),
		Type:     msgType,
		Source:   source,
		Target:   target,
		Session:  session,
		Data:     data,
		RespChan: respChan,
		Ctx:      callCtx,
	}

	if err := s.dispatch(msg); err != nil {
		return nil, err
	}

	select {
	case resp := <-respChan:
		return resp, nil
	case <-callCtx.Done():
		return nil, ErrCallTimeout
	}
}

// dispatch 投递消息到目标服务队列
func (s *ServiceScheduler) dispatch(msg *Message) error {
	s.mu.RLock()
	entry, exists := s.services[msg.Target]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %q", ErrServiceNotFound, msg.Target)
	}

	if entry.closed.Load() {
		return fmt.Errorf("%w: %q", ErrServiceUnregistered, msg.Target)
	}

	select {
	case entry.msgQueue <- msg:
		return nil
	default:
		return fmt.Errorf("%w: target=%q, queue capacity=%d", ErrQueueFull, msg.Target, cap(entry.msgQueue))
	}
}

// HasService 检查服务是否存在
func (s *ServiceScheduler) HasService(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.services[name]
	return exists
}

// ServiceNames 返回所有已注册服务名（排序后返回）
func (s *ServiceScheduler) ServiceNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.services))
	for name := range s.services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Close 关闭调度器（停止所有 worker，排空消息队列）
func (s *ServiceScheduler) Close() error {
	if s.closed.Swap(true) {
		return ErrSchedulerClosed // 已经关闭过
	}

	s.mu.Lock()
	entries := make([]*serviceEntry, 0, len(s.services))
	for _, entry := range s.services {
		entries = append(entries, entry)
	}
	// 清空注册表
	s.services = make(map[string]*serviceEntry)
	s.mu.Unlock()

	// 关闭所有服务
	for _, entry := range entries {
		entry.closed.Store(true)
		close(entry.msgQueue)
	}

	// 等待所有 worker 退出
	timeout := time.After(10 * time.Second)
	for _, entry := range entries {
		select {
		case <-entry.done:
		case <-timeout:
			entry.cancel()
		}
	}

	return nil
}
