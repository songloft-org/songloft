package logging

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string // 期望脱敏后包含
		notWant string // 期望脱敏后不再包含（敏感原文）
	}{
		{
			name:    "slog 文本键值密钥",
			in:      `time=... level=INFO msg=start jwt_secret=abcDEF123 foo=bar`,
			notWant: "abcDEF123",
			want:    "jwt_secret=***",
		},
		{
			name:    "JSON 形式 access_token",
			in:      `{"access_token":"eyJhbGciOi","expires":3600}`,
			notWant: "eyJhbGciOi",
			want:    `"access_token":"***`,
		},
		{
			name:    "默认密码明文",
			in:      `默认管理员账号: admin，默认密码: s3cr3t`,
			notWant: "s3cr3t",
			want:    "***",
		},
		{
			name:    "Authorization 头",
			in:      `headers=map[Authorization:tokenvalue123 X-Foo:bar]`,
			notWant: "tokenvalue123",
			want:    "Authorization:***",
		},
		{
			name:    "Bearer token",
			in:      `Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.abc`,
			notWant: "eyJhbGciOiJIUzI1NiJ9.abc",
			want:    "Bearer ***",
		},
		{
			name:    "URL 内嵌凭证",
			in:      `proxy=http://user:passwd@192.168.1.1:7890`,
			notWant: "user:passwd@",
			want:    "http://***:***@",
		},
		{
			name:    "客户端 IP 脱敏保留网段",
			in:      `remote=203.0.113.45:51234`,
			notWant: "203.0.113.45",
			want:    "203.0.*.*",
		},
		{
			name: "回环地址不脱敏",
			in:   `listening on 127.0.0.1:58091`,
			want: "127.0.0.1",
		},
		{
			name:    "Unix 用户目录",
			in:      `music path=/home/alice/Music`,
			notWant: "/home/alice",
			want:    "/home/<user>",
		},
		{
			name:    "Windows 用户目录",
			in:      `path=C:\Users\Bob\AppData`,
			notWant: `\Users\Bob`,
			want:    `Users\<user>`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Redact(c.in)
			if c.notWant != "" && strings.Contains(got, c.notWant) {
				t.Errorf("脱敏后仍包含敏感内容 %q\n输入: %s\n输出: %s", c.notWant, c.in, got)
			}
			if c.want != "" && !strings.Contains(got, c.want) {
				t.Errorf("脱敏后未包含期望内容 %q\n输入: %s\n输出: %s", c.want, c.in, got)
			}
		})
	}
}

func TestRedactStream(t *testing.T) {
	in := "line1 password=hunter2\nline2 ok\n"
	var sb strings.Builder
	if err := RedactStream(&sb, strings.NewReader(in)); err != nil {
		t.Fatalf("RedactStream 出错: %v", err)
	}
	out := sb.String()
	if strings.Contains(out, "hunter2") {
		t.Errorf("流式脱敏后仍含密码: %s", out)
	}
	if !strings.Contains(out, "line2 ok") {
		t.Errorf("流式脱敏丢失了正常行: %s", out)
	}
	if strings.Count(out, "\n") != 2 {
		t.Errorf("流式脱敏行数不符，期望 2 个换行: %q", out)
	}
}
