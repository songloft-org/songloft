package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// InterPluginMessage 插件间通信的消息数据
type InterPluginMessage struct {
	From    string          `json:"from"`    // 发送方 entryPath
	To      string          `json:"to"`      // 接收方 entryPath
	Action  string          `json:"action"`  // 动作名称
	Payload json.RawMessage `json:"payload"` // 消息负载（任意 JSON）
}

// InterPluginResponse 插件间通信的响应
type InterPluginResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// DefaultCallTimeout 插件间 call 的默认超时时间
const DefaultCallTimeout = 10 * time.Second

// Communicator 管理插件间通信
type Communicator struct {
	scheduler *ServiceScheduler
}

// NewCommunicator 创建通信管理器
func NewCommunicator(scheduler *ServiceScheduler) *Communicator {
	return &Communicator{scheduler: scheduler}
}

// Send 异步发送消息到目标插件（不等待响应）
// 调用者需要有 "inter-plugin" 权限
func (c *Communicator) Send(from, to, action string, payload json.RawMessage) error {
	if !c.scheduler.HasService(to) {
		return fmt.Errorf("target plugin '%s' not found or not running", to)
	}

	msg := &Message{
		Type:   MsgInterPlugin,
		Source: from,
		Target: to,
		Data: &InterPluginMessage{
			From:    from,
			To:      to,
			Action:  action,
			Payload: payload,
		},
	}

	return c.scheduler.Send(msg)
}

// Call 同步调用目标插件并等待响应
// timeout 为 0 时使用 DefaultCallTimeout
func (c *Communicator) Call(ctx context.Context, from, to, action string, payload json.RawMessage, timeout time.Duration) (*InterPluginResponse, error) {
	if timeout == 0 {
		timeout = DefaultCallTimeout
	}

	data := &InterPluginMessage{
		From:    from,
		To:      to,
		Action:  action,
		Payload: payload,
	}

	resp, err := c.scheduler.Call(ctx, to, from, MsgInterPlugin, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("inter-plugin call to '%s' failed: %w", to, err)
	}

	// 解析响应
	if resp == nil || resp.Data == nil {
		return &InterPluginResponse{Success: true}, nil
	}

	if ipResp, ok := resp.Data.(*InterPluginResponse); ok {
		return ipResp, nil
	}

	return &InterPluginResponse{Success: true}, nil
}

// GenerateCommJS 生成注入到 JS 环境中的通信 API 代码。
// 所有方法都是 async；handler 也允许返回 Promise，框架会自动 await。
func GenerateCommJS() string {
	return `
// === mimusic.comm — 插件间通信 ===
mimusic.comm = {
    // 异步发送消息到其他插件（fire-and-forget）
    // 返回 Promise 让调用方能 await 投递完成；不需要时可不 await。
    send: async function(target, action, payload) {
        await __callBridge('comm.send', JSON.stringify({
            to: target,
            action: action,
            payload: payload || null
        }));
    },

    // 调用其他插件并等待响应。返回 Promise<InterPluginResponse>。
    call: async function(target, action, payload, timeoutMs) {
        var s = await __callBridge('comm.call', JSON.stringify({
            to: target,
            action: action,
            payload: payload || null,
            timeout: timeoutMs || 10000
        }));
        return s ? JSON.parse(s) : { success: false, error: 'empty response' };
    },

    // 注册消息处理器（当收到其他插件的消息时调用）。
    // handler 可返回值或 Promise；框架会 await 后再 JSON.stringify。
    _handlers: {},
    onMessage: function(action, handler) {
        this._handlers[action] = handler;
    }
};

// 内部：处理插件间消息的入口（由 Go 侧 ExecuteJS 调用，事件循环会 await 返回的 Promise）。
async function __handleInterPluginMessage(msgJSON) {
    var msg = JSON.parse(msgJSON);
    var handler = mimusic.comm._handlers[msg.action];
    if (!handler) {
        return JSON.stringify({ success: false, error: 'no handler for action: ' + msg.action });
    }
    try {
        var result = await handler(msg.payload, msg.from);
        return JSON.stringify({ success: true, data: result || null });
    } catch (e) {
        return JSON.stringify({ success: false, error: (e && e.message) ? e.message : String(e) });
    }
}
`
}
