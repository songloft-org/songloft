package services

import (
	"net"
	"strings"
)

// IsHostnameAllowed 检查域名是否允许代理
// 采用内网封禁策略：阻止访问内网地址以防止 SSRF，允许所有外网域名
func IsHostnameAllowed(hostname string) bool {
	hostname = strings.ToLower(hostname)

	// 阻止明显的内网主机名
	if hostname == "localhost" || strings.HasSuffix(hostname, ".local") || hostname == "" {
		return false
	}

	// DNS 解析域名，检查解析结果是否为内网 IP
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// 解析失败也放行，交由后续 HTTP 请求自行报错
		return true
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return false
		}
	}

	return true
}

// isPrivateIP 检查 IP 是否为内网/保留地址
func isPrivateIP(ip net.IP) bool {
	// 回环地址: 127.0.0.0/8, ::1
	if ip.IsLoopback() {
		return true
	}
	// 私有地址: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7
	if ip.IsPrivate() {
		return true
	}
	// 链路本地: 169.254.0.0/16, fe80::/10
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// 未指定地址: 0.0.0.0, ::
	if ip.IsUnspecified() {
		return true
	}
	return false
}
