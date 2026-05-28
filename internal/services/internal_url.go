package services

import (
	"fmt"
	"strings"
)

// InternalURLResolver 把以 "/" 开头的相对路径(JS 插件代理路径、本机端点等)
// 解析成 http://127.0.0.1:{port}{path}?access_token={token} 形态,
// 供后端内部 HTTP 调用(例如 convert_service 下载歌曲、SongHandler 拉歌词)使用。
//
// 绝对 URL(http://、https://)或空字符串原样返回。
type InternalURLResolver struct {
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

// Resolve 解析为可直接访问的绝对 URL。
func (r *InternalURLResolver) Resolve(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	if !strings.HasPrefix(rawURL, "/") {
		return rawURL
	}
	if r == nil || r.serverPort <= 0 || r.internalToken == "" {
		return rawURL
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s%saccess_token=%s",
		r.serverPort, rawURL, sep, r.internalToken)
}
