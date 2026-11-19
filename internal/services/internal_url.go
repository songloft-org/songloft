package services

import (
	"fmt"
	"strings"
	"sync"
)

// InternalURLResolver 把以 "/" 开头的相对路径(JS 插件代理路径、本机端点等)
// 解析成 http://127.0.0.1:{port}{path}?access_token={token} 形态,
// 供后端内部 HTTP 调用(例如 convert_service 下载歌曲、SongHandler 拉歌词)使用。
//
// 绝对 URL(http://、https://)或空字符串原样返回。
type InternalURLResolver struct {
	mu            sync.RWMutex
	serverPort    int
	internalToken string
}

// NewInternalURLResolver 创建解析器。
// serverPort <= 0 或 internalToken 为空时,Resolve 对相对路径会原样返回(调用方需自行处理)。
func NewInternalURLResolver(serverPort int, internalToken string) *InternalURLResolver {
	return &InternalURLResolver{
		serverPort:    serverPort,
		internalToken: internalToken,
	}
}

// SetServerPort updates the port used for internally proxied URLs.
// Bundle mode asks the OS for an available port, so the real listener port is
// only known after the application has started listening.
func (r *InternalURLResolver) SetServerPort(serverPort int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.serverPort = serverPort
	r.mu.Unlock()
}

// Resolve 解析为可直接访问的绝对 URL。
func (r *InternalURLResolver) Resolve(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	if !strings.HasPrefix(rawURL, "/") {
		return rawURL
	}
	if r == nil {
		return rawURL
	}
	r.mu.RLock()
	serverPort := r.serverPort
	internalToken := r.internalToken
	r.mu.RUnlock()
	if serverPort <= 0 || internalToken == "" {
		return rawURL
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s%saccess_token=%s",
		serverPort, rawURL, sep, internalToken)
}
