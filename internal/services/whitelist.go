package services

import (
	"fmt"
	"net"
	"strings"
)

// IsHostnameAllowed 检查域名是否允许代理
// 采用内网封禁策略：阻止访问内网地址以防止 SSRF，允许所有外网域名。
// 等价于 IsHostnameAllowedWithAllowlist(hostname, nil)：不带白名单时私网一律拒绝。
func IsHostnameAllowed(hostname string) bool {
	return IsHostnameAllowedWithAllowlist(hostname, nil)
}

// IsHostnameAllowedWithAllowlist 在内网封禁策略基础上，允许命中 allowlist 的私网地址通过。
//
// 判定规则：
//   - 外网 IP：恒放行
//   - 私网 / 回环 / 链路本地 IP：仅当被 allowlist 中某条网段覆盖时放行，否则整体拒绝
//   - 空 hostname：拒绝
//   - DNS 解析失败：放行（交由后续 HTTP 请求自行报错，与历史行为一致）
//
// allowlist 为 nil / 空时退化为纯内网封禁（历史行为）。
func IsHostnameAllowedWithAllowlist(hostname string, allowlist []*net.IPNet) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))

	// 阻止明显的内网主机名（白名单仅按 IP/CIDR 匹配，无法覆盖这些名字）
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".local") {
		return false
	}

	// DNS 解析域名（字面量 IP 也走此函数，返回单元素切片），检查解析结果
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// 解析失败也放行，交由后续 HTTP 请求自行报错
		return true
	}

	for _, ip := range ips {
		if isPrivateIP(ip) && !allowlistContains(allowlist, ip) {
			return false
		}
	}

	return true
}

// ParseAllowlist 将文本白名单条目解析为 CIDR 网段列表。
// 支持两种格式：
//   - CIDR 网段：如 "192.168.1.0/24"、"fd00::/8"
//   - 单个 IP：如 "192.168.1.100"、"::1"（自动补 /32 或 /128）
//
// 空字符串条目被忽略；任一条目非法时返回 error（供 PUT 校验拒绝）。
func ParseAllowlist(entries []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		// 优先按 CIDR 解析
		if _, ipNet, err := net.ParseCIDR(entry); err == nil {
			nets = append(nets, ipNet)
			continue
		}

		// 退回按单个 IP 解析，补足全掩码
		if ip := net.ParseIP(entry); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}

		return nil, fmt.Errorf("非法的白名单条目: %q（需为 IP 或 CIDR 网段）", entry)
	}
	return nets, nil
}

// allowlistContains 判断 ip 是否被 allowlist 中任一网段覆盖。
func allowlistContains(allowlist []*net.IPNet, ip net.IP) bool {
	for _, n := range allowlist {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
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
