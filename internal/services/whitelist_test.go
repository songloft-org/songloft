package services

import (
	"net"
	"testing"
)

func TestParseAllowlist(t *testing.T) {
	t.Run("有效条目：CIDR 与单 IP（v4/v6）", func(t *testing.T) {
		nets, err := ParseAllowlist([]string{"192.168.1.0/24", "10.0.0.5", "fd00::/8", "::1", "  "})
		if err != nil {
			t.Fatalf("期望解析成功，得到 error: %v", err)
		}
		// 4 条有效 + 1 条空白被忽略
		if len(nets) != 4 {
			t.Fatalf("期望 4 个网段，得到 %d", len(nets))
		}
	})

	t.Run("非法条目返回 error", func(t *testing.T) {
		for _, bad := range []string{"999.1.1.1", "foo", "192.168.1.0/40", "not-an-ip"} {
			if _, err := ParseAllowlist([]string{bad}); err == nil {
				t.Errorf("条目 %q 期望返回 error，却通过了", bad)
			}
		}
	})

	t.Run("空列表返回空切片", func(t *testing.T) {
		nets, err := ParseAllowlist(nil)
		if err != nil || len(nets) != 0 {
			t.Fatalf("期望空切片无错误，得到 nets=%v err=%v", nets, err)
		}
	})
}

func TestIsHostnameAllowedWithAllowlist(t *testing.T) {
	mustParse := func(entries ...string) []*net.IPNet {
		nets, err := ParseAllowlist(entries)
		if err != nil {
			t.Fatalf("解析白名单 %v 失败: %v", entries, err)
		}
		return nets
	}

	cidr := mustParse("192.168.1.0/24")
	singleIP := mustParse("192.168.1.50")
	loopback := mustParse("127.0.0.1")

	cases := []struct {
		name      string
		hostname  string
		allowlist []*net.IPNet
		want      bool
	}{
		{"公网 IP 无白名单放行", "8.8.8.8", nil, true},
		{"私网 IP 空白名单拒绝", "192.168.1.50", nil, false},
		{"私网 IP 命中 CIDR 放行", "192.168.1.50", cidr, true},
		{"私网 IP 不在 CIDR 内拒绝", "192.168.2.50", cidr, false},
		{"私网 IP 命中单 IP 放行", "192.168.1.50", singleIP, true},
		{"回环 空白名单拒绝", "127.0.0.1", nil, false},
		{"回环 命中白名单放行", "127.0.0.1", loopback, true},
		{"空 hostname 拒绝", "", cidr, false},
		{"localhost 字符串封禁", "localhost", cidr, false},
		{".local 字符串封禁", "nas.local", cidr, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsHostnameAllowedWithAllowlist(c.hostname, c.allowlist); got != c.want {
				t.Errorf("IsHostnameAllowedWithAllowlist(%q) = %v, want %v", c.hostname, got, c.want)
			}
		})
	}
}

func TestIsHostnameAllowedEquivalence(t *testing.T) {
	// IsHostnameAllowed 应等价于 nil 白名单：私网一律拒绝，公网放行
	if IsHostnameAllowed("192.168.1.50") {
		t.Error("IsHostnameAllowed 应拒绝私网 IP")
	}
	if !IsHostnameAllowed("8.8.8.8") {
		t.Error("IsHostnameAllowed 应放行公网 IP")
	}
}
