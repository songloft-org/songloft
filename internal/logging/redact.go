package logging

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// 脱敏规则集合。导出日志给用户提交 issue 前，逐条 apply 到日志文本，抹掉：
//   - 密钥/token/密码等键值（jwt_secret=xxx、"access_token":"xxx"）
//   - Authorization / Cookie 头
//   - Bearer token
//   - URL 里的内嵌凭证（scheme://user:pass@host）
//   - 客户端 IP（保留网段，抹掉主机位）
//   - 用户主目录路径中的用户名段（/home/<user>、/Users/<user>、C:\Users\<user>）
//
// 这是「导出时」的防御纵深：写入侧已尽量不打印明文密码/凭证（见 app.go、runtime.go），
// 但插件对外请求头等仍可能在 debug 级别落日志，故导出前统一再过一遍。
var (
	// 键值型密钥：key<sep>value。sep 覆盖 slog 文本格式（key=value）与 JSON（"key":"value"）。
	// value 取到下一个分隔符（空白、引号、逗号、右括号）为止。
	reSecretKV = regexp.MustCompile(`(?i)(jwt_secret|refresh_token|access_token|api_?key|secret|password|passwd|token)(["']?\s*[:=]\s*["']?)([^"'\s,}\]]+)`)

	// 中文标签密钥：默认密码: xxx / 密钥：xxx（防御纵深，兼顾中文日志）。
	// 要求标签后带分隔符或空格，避免误伤「密钥已生成」这类无值的正常日志。
	reSecretCN = regexp.MustCompile(`(密码|密钥|令牌)([:：=]\s*|\s+)(\S+)`)

	// 认证类头部（map/JSON 形式）：Authorization:xxx、Cookie:xxx、Set-Cookie:xxx。
	reAuthHeader = regexp.MustCompile(`(?i)(authorization|set-cookie|cookie)(["']?\s*[:=]\s*["']?)([^"'\s,}\]]+)`)

	// Bearer token（值中带空格，单独处理）。
	reBearer = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)

	// URL 内嵌凭证：scheme://user:pass@host。
	reURLCred = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)([^:/@\s]+):([^@/\s]+)@`)

	// IPv4 字面量。用 ReplaceAllFunc 保留网段、跳过回环/未指定地址。
	reIPv4 = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)

	// Unix 用户主目录：/home/<user> 或 /Users/<user>。
	reUnixHome = regexp.MustCompile(`(/home/|/Users/)([^/\s]+)`)

	// Windows 用户目录：C:\Users\<user>。
	reWinHome = regexp.MustCompile(`(?i)([A-Z]:\\Users\\)([^\\/\s]+)`)
)

// Redact 返回脱敏后的文本。对整段文本一次性 apply 所有规则，可安全用于多行内容。
func Redact(s string) string {
	s = reSecretKV.ReplaceAllString(s, "${1}${2}***")
	s = reSecretCN.ReplaceAllString(s, "${1}${2}***")
	// 先处理 Bearer（值带空格），再处理认证头，避免认证头规则把 "Bearer <token>" 截断成半个。
	s = reBearer.ReplaceAllString(s, "Bearer ***")
	s = reAuthHeader.ReplaceAllStringFunc(s, func(m string) string {
		sub := reAuthHeader.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		if strings.EqualFold(sub[3], "Bearer") { // 已由 reBearer 处理，勿再截断
			return m
		}
		return sub[1] + sub[2] + "***"
	})
	s = reURLCred.ReplaceAllString(s, "${1}***:***@")
	s = reIPv4.ReplaceAllStringFunc(s, redactIPv4)
	s = reUnixHome.ReplaceAllString(s, "${1}<user>")
	s = reWinHome.ReplaceAllString(s, "${1}<user>")
	return s
}

// redactIPv4 保留前两段网段、抹掉主机位（1.2.3.4 → 1.2.*.*）。
// 回环（127.x）与未指定地址（0.0.0.0）不脱敏——它们不含隐私且对排查本地/绑定问题有用。
func redactIPv4(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}
	if parts[0] == "127" || ip == "0.0.0.0" {
		return ip
	}
	return parts[0] + "." + parts[1] + ".*.*"
}

// RedactStream 逐行读取 src、脱敏后写入 dst，避免把整份日志读进内存。
// 单行超长时 bufio.Scanner 默认上限为 64KiB，这里放宽到 1MiB 以容纳偶发的长行。
func RedactStream(dst io.Writer, src io.Reader) error {
	sc := bufio.NewScanner(src)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if _, err := io.WriteString(dst, Redact(sc.Text())); err != nil {
			return err
		}
		if _, err := io.WriteString(dst, "\n"); err != nil {
			return err
		}
	}
	return sc.Err()
}
