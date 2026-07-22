package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"songloft/internal/logging"
)

// LogExportHandler 暴露 GET /api/v1/logs/export：把后端落盘的日志文件按时间顺序
// 拼接、逐行脱敏后作为附件返回，供用户下载并附到 issue。
//
// 与 LogHandler（日志等级配置）分开：本 handler 只依赖日志落盘目录，不涉及配置读写。
type LogExportHandler struct {
	logDir string // 日志落盘目录（<data_dir>/logs/）；与 app 侧 RotateWriter 一致
}

// NewLogExportHandler 构造 LogExportHandler。logDir 为空时导出会返回空内容（附提示行）。
func NewLogExportHandler(logDir string) *LogExportHandler {
	return &LogExportHandler{logDir: logDir}
}

// ExportLogs 处理 GET /api/v1/logs/export
// @Summary 导出后端日志
// @Description 将后端落盘的日志文件（<data_dir>/logs/ 下按天轮转的文件）按时间从旧到新拼接，逐行脱敏后作为纯文本附件返回，触发浏览器下载。脱敏会抹除密钥/token/密码、Authorization/Cookie 头、URL 内嵌凭证、客户端 IP 主机位、用户主目录名等敏感信息，便于用户安全地附到 issue。远程服务器、桌面 Bundle、移动 Bundle 三种模式下均可用（均由同一份后端提供该端点）。无日志文件时返回仅含提示行的文本。
// @Tags 设置
// @Produce plain
// @Success 200 {file} binary "脱敏后的后端日志（text/plain）"
// @Failure 500 {object} map[string]string "读取日志目录失败"
// @Security BearerAuth
// @Router /logs/export [get]
func (h *LogExportHandler) ExportLogs(w http.ResponseWriter, r *http.Request) {
	filename := fmt.Sprintf("songloft-backend-logs-%s.log", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if h.logDir == "" {
		fmt.Fprintln(w, "# 无可用的后端日志（日志落盘未启用）")
		return
	}

	files, err := logging.ListLogFiles(h.logDir)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "读取日志目录失败", err)
		return
	}
	if len(files) == 0 {
		fmt.Fprintln(w, "# 暂无后端日志文件")
		return
	}

	// 逐文件、逐行流式脱敏写出，避免把整份日志读进内存。
	// 头部已发出，途中出错无法改状态码，只能记录并中断。
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			slog.Warn("导出日志：打开文件失败，跳过", "path", path, "error", err)
			continue
		}
		if err := logging.RedactStream(w, f); err != nil {
			slog.Warn("导出日志：脱敏写出中断", "path", path, "error", err)
			f.Close()
			return
		}
		f.Close()
	}
}
