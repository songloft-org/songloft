// Package tracelycfg holds Tracely client configuration injected at build time.
//
// AppSecret 与 Host 通过 -ldflags "-X mimusic/internal/tracelycfg.AppSecret=..." 注入，
// 仅在私有构建时由 CI/Makefile 提供。开源构建默认两者为空，对应 Tracely 客户端不会被初始化。
package tracelycfg

var (
	AppSecret = ""
	Host      = ""
)

// Enabled 报告是否启用 Tracely 上报（AppSecret 与 Host 都已注入）。
func Enabled() bool {
	return AppSecret != "" && Host != ""
}
