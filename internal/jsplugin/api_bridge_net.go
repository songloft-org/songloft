package jsplugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"golang.org/x/net/ipv4"
)

const (
	maxSocketsPerPlugin = 8
	udpReadBufferSize   = 65535
)

type managedUDPSocket struct {
	id        string
	conn      *net.UDPConn
	cancel    context.CancelFunc
	done      chan struct{}
	mu        sync.Mutex
	multicast []net.IP
}

func (h *BridgeHandler) handleNet(action, data string) (string, error) {
	switch action {
	case "net.udpBind":
		return h.netUDPBind(data)
	case "net.udpSend":
		return h.netUDPSend(data)
	case "net.udpClose":
		return h.netUDPClose(data)
	case "net.udpJoinMulticast":
		return h.netUDPJoinMulticast(data)
	case "net.udpLeaveMulticast":
		return h.netUDPLeaveMulticast(data)
	case "net.udpGetLocalAddr":
		return h.netUDPGetLocalAddr(data)
	default:
		return "", fmt.Errorf("unknown net action: %s", action)
	}
}

func (h *BridgeHandler) netUDPBind(data string) (string, error) {
	var params struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("netUDPBind: %w", err)
	}

	count := 0
	h.udpSockets.Range(func(_, _ any) bool {
		count++
		return count < maxSocketsPerPlugin
	})
	if count >= maxSocketsPerPlugin {
		return "", fmt.Errorf("netUDPBind: max %d sockets per plugin", maxSocketsPerPlugin)
	}

	addr := params.Address
	if addr == "" {
		addr = ":0"
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return "", fmt.Errorf("netUDPBind: resolve %q: %w", addr, err)
	}

	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return "", fmt.Errorf("netUDPBind: listen: %w", err)
	}

	socketID := fmt.Sprintf("udp-%d", h.socketIDSeq.Add(1))
	ctx, cancel := context.WithCancel(context.Background())

	sock := &managedUDPSocket{
		id:     socketID,
		conn:   conn,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	h.udpSockets.Store(socketID, sock)

	go h.udpReadLoop(ctx, sock)

	localAddr := conn.LocalAddr().String()
	slog.Info("jsplugin: UDP socket bound",
		"plugin", h.service.plugin.EntryPath,
		"socketId", socketID,
		"localAddr", localAddr)

	result, _ := json.Marshal(map[string]string{
		"socketId":  socketID,
		"localAddr": localAddr,
	})
	return string(result), nil
}

func (h *BridgeHandler) udpReadLoop(ctx context.Context, sock *managedUDPSocket) {
	defer close(sock.done)

	buf := make([]byte, udpReadBufferSize)
	entryPath := h.service.plugin.EntryPath

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, remoteAddr, err := sock.conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Debug("jsplugin: UDP read error",
				"plugin", entryPath,
				"socketId", sock.id,
				"error", err)
			return
		}

		event := &NetDataEvent{
			SocketID:   sock.id,
			Data:       base64.StdEncoding.EncodeToString(buf[:n]),
			RemoteAddr: remoteAddr.String(),
		}
		eventJSON, err := json.Marshal(event)
		if err != nil {
			slog.Debug("jsplugin: UDP event marshal failed",
				"plugin", entryPath,
				"socketId", sock.id,
				"error", err)
			continue
		}

		if err := h.postHostEvent("net_data", sock.id, string(eventJSON)); err != nil {
			slog.Debug("jsplugin: UDP host event push failed",
				"plugin", entryPath,
				"socketId", sock.id,
				"error", err)
		}
	}
}

func (h *BridgeHandler) netUDPSend(data string) (string, error) {
	var params struct {
		SocketID string `json:"socketId"`
		Data     string `json:"data"`
		Addr     string `json:"addr"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("netUDPSend: %w", err)
	}

	val, ok := h.udpSockets.Load(params.SocketID)
	if !ok {
		return "", fmt.Errorf("netUDPSend: socket %q not found", params.SocketID)
	}
	sock := val.(*managedUDPSocket)

	remoteAddr, err := net.ResolveUDPAddr("udp4", params.Addr)
	if err != nil {
		return "", fmt.Errorf("netUDPSend: resolve %q: %w", params.Addr, err)
	}

	payload, err := base64.StdEncoding.DecodeString(params.Data)
	if err != nil {
		payload = []byte(params.Data)
	}

	_, err = sock.conn.WriteToUDP(payload, remoteAddr)
	if err != nil {
		return "", fmt.Errorf("netUDPSend: %w", err)
	}
	return "", nil
}

func (h *BridgeHandler) netUDPJoinMulticast(data string) (string, error) {
	var params struct {
		SocketID string `json:"socketId"`
		Group    string `json:"group"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("netUDPJoinMulticast: %w", err)
	}

	val, ok := h.udpSockets.Load(params.SocketID)
	if !ok {
		return "", fmt.Errorf("netUDPJoinMulticast: socket %q not found", params.SocketID)
	}
	sock := val.(*managedUDPSocket)

	groupIP := net.ParseIP(params.Group)
	if groupIP == nil {
		return "", fmt.Errorf("netUDPJoinMulticast: invalid group %q", params.Group)
	}

	p := ipv4.NewPacketConn(sock.conn)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("netUDPJoinMulticast: list interfaces: %w", err)
	}

	joined := 0
	for i := range ifaces {
		iface := &ifaces[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		if err := p.JoinGroup(iface, &net.UDPAddr{IP: groupIP}); err != nil {
			slog.Debug("jsplugin: multicast join failed on interface",
				"plugin", h.service.plugin.EntryPath,
				"iface", iface.Name,
				"group", params.Group,
				"error", err)
			continue
		}
		joined++
	}

	if joined == 0 {
		return "", fmt.Errorf("netUDPJoinMulticast: failed to join %q on any interface", params.Group)
	}

	sock.mu.Lock()
	sock.multicast = append(sock.multicast, groupIP)
	sock.mu.Unlock()

	slog.Info("jsplugin: joined multicast group",
		"plugin", h.service.plugin.EntryPath,
		"socketId", params.SocketID,
		"group", params.Group,
		"interfaces", joined)

	return "", nil
}

func (h *BridgeHandler) netUDPLeaveMulticast(data string) (string, error) {
	var params struct {
		SocketID string `json:"socketId"`
		Group    string `json:"group"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("netUDPLeaveMulticast: %w", err)
	}

	val, ok := h.udpSockets.Load(params.SocketID)
	if !ok {
		return "", fmt.Errorf("netUDPLeaveMulticast: socket %q not found", params.SocketID)
	}
	sock := val.(*managedUDPSocket)

	groupIP := net.ParseIP(params.Group)
	if groupIP == nil {
		return "", fmt.Errorf("netUDPLeaveMulticast: invalid group %q", params.Group)
	}

	p := ipv4.NewPacketConn(sock.conn)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("netUDPLeaveMulticast: list interfaces: %w", err)
	}

	for i := range ifaces {
		iface := &ifaces[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		_ = p.LeaveGroup(iface, &net.UDPAddr{IP: groupIP})
	}

	sock.mu.Lock()
	filtered := sock.multicast[:0]
	for _, ip := range sock.multicast {
		if !ip.Equal(groupIP) {
			filtered = append(filtered, ip)
		}
	}
	sock.multicast = filtered
	sock.mu.Unlock()

	return "", nil
}

func (h *BridgeHandler) netUDPClose(data string) (string, error) {
	var params struct {
		SocketID string `json:"socketId"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("netUDPClose: %w", err)
	}

	val, ok := h.udpSockets.LoadAndDelete(params.SocketID)
	if !ok {
		return "", nil
	}
	sock := val.(*managedUDPSocket)

	sock.cancel()
	sock.conn.Close()
	<-sock.done

	slog.Info("jsplugin: UDP socket closed",
		"plugin", h.service.plugin.EntryPath,
		"socketId", params.SocketID)

	return "", nil
}

func (h *BridgeHandler) netUDPGetLocalAddr(data string) (string, error) {
	var params struct {
		SocketID string `json:"socketId"`
	}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return "", fmt.Errorf("netUDPGetLocalAddr: %w", err)
	}

	val, ok := h.udpSockets.Load(params.SocketID)
	if !ok {
		return "", fmt.Errorf("netUDPGetLocalAddr: socket %q not found", params.SocketID)
	}
	sock := val.(*managedUDPSocket)

	result, _ := json.Marshal(map[string]string{
		"localAddr": sock.conn.LocalAddr().String(),
	})
	return string(result), nil
}

func (h *BridgeHandler) cleanupUDPSockets() {
	h.udpSockets.Range(func(key, value any) bool {
		sock := value.(*managedUDPSocket)
		sock.cancel()
		sock.conn.Close()
		<-sock.done
		h.udpSockets.Delete(key)
		slog.Info("jsplugin: closed UDP socket on cleanup",
			"plugin", h.service.plugin.EntryPath,
			"socketId", key)
		return true
	})
}
