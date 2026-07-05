package jsplugin

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const (
	webSocketOpenTimeout  = 10 * time.Second
	webSocketCloseTimeout = 1 * time.Second
	maxCloseReasonBytes   = 123
)

var inboundWebSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type managedInboundWebSocket struct {
	conn   *websocket.Conn
	closed atomic.Bool
	mu     sync.Mutex
}

func isWebSocketUpgrade(r *http.Request) bool {
	return websocket.IsWebSocketUpgrade(r)
}

func (m *Manager) handlePluginWebSocket(w http.ResponseWriter, r *http.Request, service *JSService, entryPath, subPath string) {
	if service == nil || service.bridgeHandler == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "plugin unavailable", "runtime_error")
		return
	}
	if !CheckPermission(service.plugin.Permissions, PermWebSocket) {
		writeJSONError(w, http.StatusForbidden, "permission denied", "requires 'websocket' permission")
		return
	}

	normalizedPath := subPath
	if normalizedPath != "" && normalizedPath[0] != '/' {
		normalizedPath = "/" + normalizedPath
	}

	conn, err := inboundWebSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Debug("jsplugin websocket upgrade failed",
			"plugin", entryPath,
			"path", normalizedPath,
			"error", err)
		return
	}

	connID := service.bridgeHandler.registerInboundWebSocket(conn)
	openData := &WebSocketOpenData{
		ConnID: connID,
		Request: &WebSocketRequestData{
			Method:     r.Method,
			Path:       normalizedPath,
			Headers:    flattenHeaders(r.Header),
			Query:      r.URL.RawQuery,
			RemoteAddr: r.RemoteAddr,
		},
	}

	openCtx, cancel := context.WithTimeout(context.Background(), webSocketOpenTimeout)
	resp, err := m.scheduler.Call(openCtx, entryPath, "", MsgWebSocketOpen, openData, webSocketOpenTimeout)
	cancel()
	if err != nil {
		service.bridgeHandler.closeInboundWebSocket(connID, websocket.CloseInternalServerErr, "open handler failed", false)
		slog.Warn("jsplugin websocket open call failed",
			"plugin", entryPath,
			"path", normalizedPath,
			"connId", connID,
			"error", err)
		return
	}
	if resp != nil && resp.Data != nil {
		if respErr, ok := resp.Data.(error); ok && respErr != nil {
			service.bridgeHandler.closeInboundWebSocket(connID, websocket.CloseInternalServerErr, "open handler failed", false)
			return
		}
	}

	go service.bridgeHandler.inboundWebSocketReadLoop(connID)
}

func (h *BridgeHandler) handleWebSocket(action, data string) (string, error) {
	switch action {
	case "websocket.send":
		return h.webSocketSend(data)
	case "websocket.close":
		return h.webSocketClose(data)
	default:
		return "", fmt.Errorf("unknown websocket action: %s", action)
	}
}

func (h *BridgeHandler) registerInboundWebSocket(conn *websocket.Conn) string {
	connID := fmt.Sprintf("inbound-ws-%d", h.inboundWebSocketIDSeq.Add(1))
	h.inboundWebSockets.Store(connID, newManagedInboundWebSocket(conn))
	slog.Info("jsplugin websocket connected",
		"plugin", h.service.plugin.EntryPath,
		"connId", connID)
	return connID
}

func newManagedInboundWebSocket(conn *websocket.Conn) *managedInboundWebSocket {
	return &managedInboundWebSocket{
		conn: conn,
	}
}

func (h *BridgeHandler) inboundWebSocketReadLoop(connID string) {
	val, ok := h.inboundWebSockets.Load(connID)
	if !ok {
		return
	}
	sock := val.(*managedInboundWebSocket)
	entryPath := h.service.plugin.EntryPath

	for {
		messageType, data, err := sock.conn.ReadMessage()
		if err != nil {
			code, reason, wasClean := webSocketCloseInfo(err)
			h.closeInboundWebSocket(connID, code, reason, wasClean)
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		event := &WebSocketMessageData{
			ConnID:   connID,
			DataHex:  hex.EncodeToString(data),
			IsBinary: messageType == websocket.BinaryMessage,
		}
		eventJSON, err := json.Marshal(event)
		if err != nil {
			slog.Warn("jsplugin websocket message marshal failed",
				"plugin", entryPath,
				"connId", connID,
				"error", err)
			continue
		}
		if err := h.postHostEvent("inbound_ws_message", connID, string(eventJSON)); err != nil {
			slog.Warn("jsplugin websocket host event push failed",
				"plugin", entryPath,
				"connId", connID,
				"error", err)
			h.closeInboundWebSocket(connID, websocket.CloseTryAgainLater, "plugin busy", false)
			return
		}
	}
}

func (h *BridgeHandler) webSocketSend(data string) (string, error) {
	var params struct {
		ConnID   string `json:"connId"`
		DataHex  string `json:"dataHex"`
		IsBinary bool   `json:"isBinary"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("webSocketSend: %w", err)
	}

	val, ok := h.inboundWebSockets.Load(params.ConnID)
	if !ok {
		return "", fmt.Errorf("webSocketSend: connection %q not found", params.ConnID)
	}
	sock := val.(*managedInboundWebSocket)
	if sock.closed.Load() {
		return "", fmt.Errorf("webSocketSend: connection %q already closed", params.ConnID)
	}

	payload, err := hex.DecodeString(params.DataHex)
	if err != nil {
		return "", fmt.Errorf("webSocketSend: hex decode: %w", err)
	}

	messageType := websocket.TextMessage
	if params.IsBinary {
		messageType = websocket.BinaryMessage
	}

	sock.mu.Lock()
	err = sock.conn.WriteMessage(messageType, payload)
	sock.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("webSocketSend: write: %w", err)
	}
	return "", nil
}

func (h *BridgeHandler) webSocketClose(data string) (string, error) {
	var params struct {
		ConnID string `json:"connId"`
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("webSocketClose: %w", err)
	}

	h.closeInboundWebSocket(params.ConnID, params.Code, params.Reason, true)
	return "", nil
}

func (h *BridgeHandler) closeInboundWebSocket(connID string, code int, reason string, wasClean bool) {
	val, ok := h.inboundWebSockets.Load(connID)
	if !ok {
		return
	}
	sock := val.(*managedInboundWebSocket)
	if !sock.closed.CompareAndSwap(false, true) {
		return
	}
	h.inboundWebSockets.Delete(connID)

	eventCode := code
	if eventCode == 0 {
		eventCode = websocket.CloseNormalClosure
	}
	wireCode := sanitizeWebSocketCloseCode(code)
	reason = truncateCloseReason(reason)

	sock.mu.Lock()
	_ = sock.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(wireCode, reason),
		time.Now().Add(webSocketCloseTimeout),
	)
	_ = sock.conn.Close()
	sock.mu.Unlock()

	h.notifyInboundWebSocketClose(connID, eventCode, reason, wasClean)
	slog.Info("jsplugin websocket closed",
		"plugin", h.service.plugin.EntryPath,
		"connId", connID,
		"code", eventCode,
		"reason", reason)
}

func (h *BridgeHandler) notifyInboundWebSocketClose(connID string, code int, reason string, wasClean bool) {
	if h.service == nil || h.service.plugin == nil {
		return
	}
	event := &WebSocketCloseData{
		ConnID:   connID,
		Code:     code,
		Reason:   reason,
		WasClean: wasClean,
	}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		slog.Debug("jsplugin websocket close marshal failed",
			"plugin", h.service.plugin.EntryPath,
			"connId", connID,
			"error", err)
		return
	}
	if err := h.postHostEvent("inbound_ws_close", connID, string(eventJSON)); err != nil {
		slog.Debug("jsplugin websocket close host event push failed",
			"plugin", h.service.plugin.EntryPath,
			"connId", connID,
			"error", err)
	}
}

func (h *BridgeHandler) cleanupInboundWebSockets() {
	h.inboundWebSockets.Range(func(key, value any) bool {
		connID := key.(string)
		sock := value.(*managedInboundWebSocket)
		if sock.closed.CompareAndSwap(false, true) {
			h.inboundWebSockets.Delete(connID)
			sock.mu.Lock()
			_ = sock.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "plugin stopped"),
				time.Now().Add(webSocketCloseTimeout),
			)
			_ = sock.conn.Close()
			sock.mu.Unlock()
		}
		return true
	})
}

func webSocketCloseInfo(err error) (code int, reason string, wasClean bool) {
	if closeErr, ok := err.(*websocket.CloseError); ok {
		wasClean = closeErr.Code == websocket.CloseNormalClosure || closeErr.Code == websocket.CloseGoingAway
		return closeErr.Code, closeErr.Text, wasClean
	}
	return websocket.CloseAbnormalClosure, err.Error(), false
}

func sanitizeWebSocketCloseCode(code int) int {
	if code == 0 {
		return websocket.CloseNormalClosure
	}
	if code < 1000 || code > 4999 {
		return websocket.CloseNormalClosure
	}
	switch code {
	case websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure, websocket.CloseTLSHandshake:
		return websocket.CloseNormalClosure
	}
	return code
}

func truncateCloseReason(reason string) string {
	for len(reason) > maxCloseReasonBytes {
		_, size := utf8.DecodeLastRuneInString(reason)
		if size <= 0 {
			reason = reason[:maxCloseReasonBytes]
			break
		}
		reason = reason[:len(reason)-size]
	}
	for !utf8.ValidString(reason) && len(reason) > 0 {
		reason = reason[:len(reason)-1]
	}
	return reason
}
