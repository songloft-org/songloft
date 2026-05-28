package jsplugin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockHandler 用于测试的 mock 消息处理器
type mockHandler struct {
	received []*Message
	mu       sync.Mutex
	response *Message       // 如果非 nil，HandleMessage 返回此响应
	delay    time.Duration  // 模拟处理延迟
	onMsg    func(*Message) // 可选的回调
}

func (h *mockHandler) HandleMessage(msg *Message) *Message {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	h.mu.Lock()
	h.received = append(h.received, msg)
	if h.onMsg != nil {
		h.onMsg(msg)
	}
	h.mu.Unlock()
	return h.response
}

func (h *mockHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.received)
}

func (h *mockHandler) messages() []*Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make([]*Message, len(h.received))
	copy(cp, h.received)
	return cp
}

// TestSchedulerRegisterAndSend 注册服务，发送消息，验证 handler 被调用
func TestSchedulerRegisterAndSend(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	handler := &mockHandler{}
	if err := sched.RegisterService("svc1", handler, 16); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	// 验证服务已注册
	if !sched.HasService("svc1") {
		t.Fatal("expected svc1 to be registered")
	}
	names := sched.ServiceNames()
	if len(names) != 1 || names[0] != "svc1" {
		t.Fatalf("unexpected service names: %v", names)
	}

	// 发送消息
	err := sched.Send(&Message{
		Type:   MsgLifecycle,
		Source: "",
		Target: "svc1",
		Data:   "hello",
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// 等待消息被处理
	deadline := time.After(2 * time.Second)
	for {
		if handler.count() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for message to be processed")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	msgs := handler.messages()
	if msgs[0].Data != "hello" {
		t.Errorf("expected data 'hello', got %v", msgs[0].Data)
	}
	if msgs[0].Type != MsgLifecycle {
		t.Errorf("expected MsgLifecycle, got %d", msgs[0].Type)
	}
}

// TestSchedulerCall 同步调用，验证请求-响应配对正确
func TestSchedulerCall(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	handler := &mockHandler{
		response: &Message{Data: "pong"},
	}
	if err := sched.RegisterService("echo", handler, 16); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	resp, err := sched.Call(context.Background(), "echo", "caller", MsgInterPlugin, "ping", 5*time.Second)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Data != "pong" {
		t.Errorf("expected 'pong', got %v", resp.Data)
	}

	// 验证 handler 收到的消息
	msgs := handler.messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Source != "caller" {
		t.Errorf("expected source 'caller', got %q", msgs[0].Source)
	}
	if msgs[0].Session == 0 {
		t.Error("expected non-zero session")
	}
}

// TestSchedulerCallTimeout 同步调用超时，验证返回超时错误
func TestSchedulerCallTimeout(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	handler := &mockHandler{
		delay:    500 * time.Millisecond,
		response: &Message{Data: "late"},
	}
	if err := sched.RegisterService("slow", handler, 16); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	_, err := sched.Call(context.Background(), "slow", "caller", MsgInterPlugin, "ping", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, ErrCallTimeout) {
		t.Errorf("expected ErrCallTimeout, got %v", err)
	}
}

// TestSchedulerSendToNonexistent 向不存在的服务发消息，验证返回错误
func TestSchedulerSendToNonexistent(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	err := sched.Send(&Message{
		Type:   MsgLifecycle,
		Target: "nonexistent",
		Data:   "hello",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
	if !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

// TestSchedulerUnregister 注销服务，验证后续消息被拒绝
func TestSchedulerUnregister(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	handler := &mockHandler{}
	if err := sched.RegisterService("temp", handler, 16); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	if !sched.HasService("temp") {
		t.Fatal("expected service to exist")
	}

	if err := sched.UnregisterService("temp", 5*time.Second); err != nil {
		t.Fatalf("UnregisterService failed: %v", err)
	}

	if sched.HasService("temp") {
		t.Fatal("expected service to be removed")
	}

	// 发送到已注销的服务应失败
	err := sched.Send(&Message{
		Type:   MsgLifecycle,
		Target: "temp",
		Data:   "should fail",
	})
	if err == nil {
		t.Fatal("expected error after unregister")
	}
	if !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

// TestSchedulerConcurrency 并发发送多条消息，验证串行处理（无竞争）
func TestSchedulerConcurrency(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	var order []int
	var orderMu sync.Mutex

	handler := &mockHandler{
		onMsg: func(msg *Message) {
			// onMsg is called inside mockHandler's lock, so we need our own
			val := msg.Data.(int)
			orderMu.Lock()
			order = append(order, val)
			orderMu.Unlock()
		},
	}
	if err := sched.RegisterService("concurrent", handler, 256); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(v int) {
			defer wg.Done()
			err := sched.Send(&Message{
				Type:   MsgInterPlugin,
				Target: "concurrent",
				Data:   v,
			})
			if err != nil {
				t.Errorf("Send failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// 等待所有消息处理完
	deadline := time.After(5 * time.Second)
	for {
		if handler.count() >= n {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout: only %d of %d messages processed", handler.count(), n)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	if handler.count() != n {
		t.Errorf("expected %d messages, got %d", n, handler.count())
	}
}

// TestSchedulerMessageOrdering 验证消息按 FIFO 顺序处理
func TestSchedulerMessageOrdering(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	var order []int
	handler := &mockHandler{
		onMsg: func(msg *Message) {
			val := msg.Data.(int)
			order = append(order, val)
		},
	}
	if err := sched.RegisterService("ordered", handler, 256); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	const n = 50
	for i := 0; i < n; i++ {
		err := sched.Send(&Message{
			Type:   MsgInterPlugin,
			Target: "ordered",
			Data:   i,
		})
		if err != nil {
			t.Fatalf("Send %d failed: %v", i, err)
		}
	}

	// 等待所有消息处理完
	deadline := time.After(5 * time.Second)
	for {
		if handler.count() >= n {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout: only %d of %d messages processed", handler.count(), n)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// 验证 FIFO 顺序
	for i, v := range order {
		if v != i {
			t.Errorf("order[%d] = %d, expected %d", i, v, i)
		}
	}
}

// TestSchedulerClose 关闭调度器，验证所有 worker 停止
func TestSchedulerClose(t *testing.T) {
	sched := NewServiceScheduler(1)

	handler1 := &mockHandler{}
	handler2 := &mockHandler{}
	sched.RegisterService("svc1", handler1, 16)
	sched.RegisterService("svc2", handler2, 16)

	if len(sched.ServiceNames()) != 2 {
		t.Fatalf("expected 2 services, got %d", len(sched.ServiceNames()))
	}

	err := sched.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 发送到已关闭的调度器应失败
	err = sched.Send(&Message{
		Type:   MsgLifecycle,
		Target: "svc1",
		Data:   "should fail",
	})
	if err == nil {
		t.Fatal("expected error after Close")
	}
	if !errors.Is(err, ErrSchedulerClosed) {
		t.Errorf("expected ErrSchedulerClosed, got %v", err)
	}

	// 注册到已关闭的调度器应失败
	err = sched.RegisterService("new", &mockHandler{}, 16)
	if err == nil {
		t.Fatal("expected error registering to closed scheduler")
	}

	// 重复 Close 应返回 ErrSchedulerClosed
	err = sched.Close()
	if !errors.Is(err, ErrSchedulerClosed) {
		t.Errorf("expected ErrSchedulerClosed on double close, got %v", err)
	}
}

// TestSchedulerBackpressure 队列满时发送消息，验证返回背压错误
func TestSchedulerBackpressure(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	// 使用很小的队列和长延迟来制造背压
	handler := &mockHandler{
		delay: 100 * time.Millisecond,
	}
	queueSize := 4
	if err := sched.RegisterService("bp", handler, queueSize); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	// 快速填满队列：发送比队列容量多的消息
	var backpressureErr error
	for i := 0; i < queueSize+10; i++ {
		err := sched.Send(&Message{
			Type:   MsgInterPlugin,
			Target: "bp",
			Data:   i,
		})
		if err != nil {
			backpressureErr = err
			break
		}
	}

	if backpressureErr == nil {
		t.Fatal("expected backpressure error when queue is full")
	}
	if !errors.Is(backpressureErr, ErrQueueFull) {
		t.Errorf("expected ErrQueueFull, got %v", backpressureErr)
	}
}

// TestSchedulerRegisterDuplicate 重复注册同名服务应失败
func TestSchedulerRegisterDuplicate(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	handler := &mockHandler{}
	if err := sched.RegisterService("dup", handler, 16); err != nil {
		t.Fatalf("first RegisterService failed: %v", err)
	}

	err := sched.RegisterService("dup", handler, 16)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
	if !errors.Is(err, ErrServiceExists) {
		t.Errorf("expected ErrServiceExists, got %v", err)
	}
}

// TestSchedulerCallToNonexistent Call 到不存在的服务应返回错误
func TestSchedulerCallToNonexistent(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	_, err := sched.Call(context.Background(), "ghost", "caller", MsgInterPlugin, "data", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for Call to nonexistent service")
	}
	if !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

// TestSchedulerMessageID 验证消息 ID 全局递增
func TestSchedulerMessageID(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	handler := &mockHandler{}
	sched.RegisterService("idtest", handler, 64)

	const n = 20
	for i := 0; i < n; i++ {
		sched.Send(&Message{
			Type:   MsgInterPlugin,
			Target: "idtest",
			Data:   i,
		})
	}

	// 等待所有消息处理完
	deadline := time.After(3 * time.Second)
	for {
		if handler.count() >= n {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for messages")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	msgs := handler.messages()
	var lastID uint64
	for i, msg := range msgs {
		if msg.ID <= lastID && i > 0 {
			t.Errorf("message IDs not monotonically increasing: msg[%d].ID=%d, prev=%d", i, msg.ID, lastID)
		}
		lastID = msg.ID
	}
}

// TestSchedulerMultipleServices 多个服务并行工作
func TestSchedulerMultipleServices(t *testing.T) {
	sched := NewServiceScheduler(1)
	defer sched.Close()

	var count1, count2 atomic.Int32
	h1 := &mockHandler{
		onMsg: func(_ *Message) { count1.Add(1) },
	}
	h2 := &mockHandler{
		onMsg: func(_ *Message) { count2.Add(1) },
	}

	sched.RegisterService("a", h1, 64)
	sched.RegisterService("b", h2, 64)

	const n = 30
	for i := 0; i < n; i++ {
		sched.Send(&Message{Type: MsgInterPlugin, Target: "a", Data: i})
		sched.Send(&Message{Type: MsgInterPlugin, Target: "b", Data: i})
	}

	deadline := time.After(5 * time.Second)
	for {
		if count1.Load() >= n && count2.Load() >= n {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout: a=%d, b=%d", count1.Load(), count2.Load())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	if count1.Load() != int32(n) {
		t.Errorf("service a: expected %d, got %d", n, count1.Load())
	}
	if count2.Load() != int32(n) {
		t.Errorf("service b: expected %d, got %d", n, count2.Load())
	}
}
