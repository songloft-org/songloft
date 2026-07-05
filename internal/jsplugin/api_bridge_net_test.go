package jsplugin

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func newTestNetBridgeHandler(t *testing.T) *BridgeHandler {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	plugin := &models.JSPlugin{
		EntryPath:   "test-net-plugin",
		Permissions: []string{PermNet},
	}

	scheduler := NewServiceScheduler(1)
	t.Cleanup(func() { scheduler.Close() })

	svc := &JSService{
		plugin:    plugin,
		scheduler: scheduler,
	}
	// Register a dummy handler so Send doesn't return ErrServiceNotFound
	if err := scheduler.RegisterService(plugin.EntryPath, &discardHandler{}, 64); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}

	dataDir := t.TempDir()
	return NewBridgeHandler(svc, dataDir, db, nil, nil, nil, "", "")
}

type discardHandler struct{}

func (d *discardHandler) HandleMessage(_ *Message) *Message { return nil }

func TestNetUDPBind_Success(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	result, err := h.netUDPBind(`{"address": ":0"}`)
	if err != nil {
		t.Fatalf("netUDPBind: %v", err)
	}

	var resp struct {
		SocketID  string `json:"socketId"`
		LocalAddr string `json:"localAddr"`
	}
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if resp.SocketID == "" {
		t.Fatal("expected non-empty socketId")
	}
	if resp.LocalAddr == "" {
		t.Fatal("expected non-empty localAddr")
	}

	// Cleanup
	_, _ = h.netUDPClose(`{"socketId":"` + resp.SocketID + `"}`)
}

func TestNetUDPBind_DefaultAddress(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	result, err := h.netUDPBind(`{}`)
	if err != nil {
		t.Fatalf("netUDPBind with empty address: %v", err)
	}

	var resp struct {
		SocketID string `json:"socketId"`
	}
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.SocketID == "" {
		t.Fatal("expected non-empty socketId")
	}

	_, _ = h.netUDPClose(`{"socketId":"` + resp.SocketID + `"}`)
}

func TestNetUDPBind_MaxSockets(t *testing.T) {
	h := newTestNetBridgeHandler(t)
	var socketIDs []string

	for i := 0; i < maxSocketsPerPlugin; i++ {
		result, err := h.netUDPBind(`{"address": ":0"}`)
		if err != nil {
			t.Fatalf("netUDPBind #%d: %v", i, err)
		}
		var resp struct {
			SocketID string `json:"socketId"`
		}
		_ = json.Unmarshal([]byte(result), &resp)
		socketIDs = append(socketIDs, resp.SocketID)
	}

	// The next bind should fail
	_, err := h.netUDPBind(`{"address": ":0"}`)
	if err == nil {
		t.Fatal("expected error when exceeding max sockets")
	}

	for _, id := range socketIDs {
		_, _ = h.netUDPClose(`{"socketId":"` + id + `"}`)
	}
}

func TestNetUDPClose_NonExistent(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	_, err := h.netUDPClose(`{"socketId":"nonexistent"}`)
	if err != nil {
		t.Fatalf("close nonexistent should not error: %v", err)
	}
}

func TestNetUDPSend_SocketNotFound(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	_, err := h.netUDPSend(`{"socketId":"nonexistent","data":"aGVsbG8=","addr":"127.0.0.1:9999"}`)
	if err == nil {
		t.Fatal("expected error for nonexistent socket")
	}
}

func TestNetUDPSendReceive(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	// Bind a socket
	result, err := h.netUDPBind(`{"address": ":0"}`)
	if err != nil {
		t.Fatalf("netUDPBind: %v", err)
	}
	var bindResp struct {
		SocketID  string `json:"socketId"`
		LocalAddr string `json:"localAddr"`
	}
	_ = json.Unmarshal([]byte(result), &bindResp)

	// Create a separate UDP conn to send data to our socket
	targetAddr, err := net.ResolveUDPAddr("udp4", bindResp.LocalAddr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	sender, err := net.DialUDP("udp4", nil, targetAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer sender.Close()

	_, err = sender.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	// Wait a bit for readLoop to process and push the host event.
	time.Sleep(100 * time.Millisecond)

	_, _ = h.netUDPClose(`{"socketId":"` + bindResp.SocketID + `"}`)
}

func TestNetUDPGetLocalAddr(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	result, err := h.netUDPBind(`{"address": ":0"}`)
	if err != nil {
		t.Fatalf("netUDPBind: %v", err)
	}
	var bindResp struct {
		SocketID  string `json:"socketId"`
		LocalAddr string `json:"localAddr"`
	}
	_ = json.Unmarshal([]byte(result), &bindResp)

	addrResult, err := h.netUDPGetLocalAddr(`{"socketId":"` + bindResp.SocketID + `"}`)
	if err != nil {
		t.Fatalf("netUDPGetLocalAddr: %v", err)
	}

	var addrResp struct {
		LocalAddr string `json:"localAddr"`
	}
	_ = json.Unmarshal([]byte(addrResult), &addrResp)

	if addrResp.LocalAddr != bindResp.LocalAddr {
		t.Fatalf("localAddr mismatch: got %q, want %q", addrResp.LocalAddr, bindResp.LocalAddr)
	}

	_, _ = h.netUDPClose(`{"socketId":"` + bindResp.SocketID + `"}`)
}

func TestNetUDPGetLocalAddr_NotFound(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	_, err := h.netUDPGetLocalAddr(`{"socketId":"nonexistent"}`)
	if err == nil {
		t.Fatal("expected error for nonexistent socket")
	}
}

func TestCleanupUDPSockets(t *testing.T) {
	h := newTestNetBridgeHandler(t)

	for i := 0; i < 3; i++ {
		_, err := h.netUDPBind(`{"address": ":0"}`)
		if err != nil {
			t.Fatalf("netUDPBind #%d: %v", i, err)
		}
	}

	// Verify sockets exist
	count := 0
	h.udpSockets.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count != 3 {
		t.Fatalf("expected 3 sockets, got %d", count)
	}

	h.cleanupUDPSockets()

	// Verify all sockets cleaned up
	count = 0
	h.udpSockets.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count != 0 {
		t.Fatalf("expected 0 sockets after cleanup, got %d", count)
	}
}

func TestHasActiveUDPSockets(t *testing.T) {
	h := newTestNetBridgeHandler(t)
	svc := h.service
	svc.bridgeHandler = h

	if svc.HasActiveUDPSockets() {
		t.Fatal("expected no active sockets initially")
	}

	result, err := h.netUDPBind(`{"address": ":0"}`)
	if err != nil {
		t.Fatalf("netUDPBind: %v", err)
	}

	if !svc.HasActiveUDPSockets() {
		t.Fatal("expected active sockets after bind")
	}

	var resp struct {
		SocketID string `json:"socketId"`
	}
	_ = json.Unmarshal([]byte(result), &resp)
	_, _ = h.netUDPClose(`{"socketId":"` + resp.SocketID + `"}`)

	if svc.HasActiveUDPSockets() {
		t.Fatal("expected no active sockets after close")
	}
}

func TestNetPermission(t *testing.T) {
	perm := extractPermFromAction("net.udpBind")
	if perm != PermNet {
		t.Fatalf("expected %q, got %q", PermNet, perm)
	}
}
