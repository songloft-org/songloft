package services

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
)

// ErrRadioTranscodeUnavailable 表示电台实时转码无法开始（ffmpeg 未配置、启动失败，或进程在吐出
// 任何音频字节之前就退出）。调用方拿到此错误时 **尚未** 向响应写入任何字节，可安全降级为原样代理。
var ErrRadioTranscodeUnavailable = errors.New("radio transcode unavailable")

// RadioTranscodeOptions 电台实时转码参数。
type RadioTranscodeOptions struct {
	UpstreamURL string // 上游电台 URL，直接交给 ffmpeg 拉取（支持 HLS .m3u8 与裸 ICY/HTTP 流）
	Format      string // 目标格式（已标准化：mp3/ogg/m4a/flac/wav）
	Bitrate     int    // 目标码率 kbps，0 表示编码器默认
	UserAgent   string // 拉流 UA（防盗链电台需要媒体播放器风格 UA，绝不用浏览器 UA）
	Referer     string // 拉流 Referer，空则不带
}

// StreamTranscodedRadio 阻塞运行 ffmpeg，把上游电台流实时转码为 opts.Format 后写入 w，
// 直到 ffmpeg 结束或 ctx 取消（客户端断开）。用于部分只支持 MP3、无法解码 AAC/HE-AAC 或不支持
// HLS 的音箱（songloft-org/songloft#275）。
//
// 与 runFFmpeg 不同，本函数 **不占用** transcodeSem：电台是直播流，ffmpeg 会持续运行整个播放时长
// （可能数小时），若持有串行信号量会长时间饿死其他有限文件的转码。
//
// 返回值：
//   - nil：正常结束（含客户端断开导致的 ctx 取消）。
//   - ErrRadioTranscodeUnavailable：转码在写出任何字节前失败，调用方应降级为原样代理（此时 w 未被写入）。
//   - 其他 error：转码已开始（w 已写入部分字节）后中途失败，无法再降级。
func (c *CacheService) StreamTranscodedRadio(ctx context.Context, w io.Writer, opts RadioTranscodeOptions) error {
	ffmpegPath := c.ffmpegPath
	if ffmpegPath == "" {
		return fmt.Errorf("%w: ffmpeg not configured", ErrRadioTranscodeUnavailable)
	}

	encoder, qualityArgs, muxer, err := ffmpegArgs(opts.Format, opts.Bitrate)
	if err != nil {
		// 不支持的目标格式：视为不可用，让调用方降级为原样代理。
		return fmt.Errorf("%w: %v", ErrRadioTranscodeUnavailable, err)
	}

	args := []string{"-hide_banner", "-loglevel", "error"}
	if opts.UserAgent != "" {
		args = append(args, "-user_agent", opts.UserAgent)
	}
	if opts.Referer != "" {
		// ffmpeg 的 http 输入通过 -headers 追加请求头，多个头以 \r\n 分隔。
		args = append(args, "-headers", "Referer: "+opts.Referer+"\r\n")
	}
	args = append(args, "-i", opts.UpstreamURL, "-vn", "-codec:a", encoder)
	args = append(args, qualityArgs...)
	args = append(args, "-f", muxer, "pipe:1")

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%w: stdout pipe: %v", ErrRadioTranscodeUnavailable, err)
	}
	// stderr 收集有限长度用于诊断；不用 CombinedOutput 因为 stdout 要流式转发。
	var stderr bytes.Buffer
	cmd.Stderr = &capWriter{w: &stderr, remaining: 4096}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start: %v", ErrRadioTranscodeUnavailable, err)
	}

	reader := bufio.NewReaderSize(stdout, 64*1024)
	// 预读第一字节：只有确认 ffmpeg 真的产出了音频，才提交响应头/首字节。
	// 若在任何字节之前就 EOF/失败（坏源、鉴权失败、编码器不支持容器等），返回
	// ErrRadioTranscodeUnavailable 让调用方降级为原样代理——此时尚未写出任何字节。
	first, _ := reader.Peek(1)
	if len(first) == 0 {
		_ = cmd.Wait()
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "no output"
		}
		return fmt.Errorf("%w: %s", ErrRadioTranscodeUnavailable, msg)
	}

	// 已确认有音频输出：从此刻起本函数拥有响应，流式转发直到结束或断开。
	_, copyErr := io.Copy(&flushingWriter{w: w}, reader)
	waitErr := cmd.Wait()

	// ctx 取消（客户端断开）是正常收尾，不当作错误。
	if ctx.Err() != nil {
		return nil
	}
	if copyErr != nil {
		return copyErr
	}
	if waitErr != nil {
		slog.Warn("radio transcode ffmpeg exited with error", "url", opts.UpstreamURL, "stderr", strings.TrimSpace(stderr.String()), "error", waitErr)
	}
	return nil
}

// flushingWriter 在每次 Write 后尝试 flush，保证转码后的字节尽快到达音箱（直播低延迟）。
type flushingWriter struct {
	w io.Writer
}

func (fw *flushingWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

// capWriter 最多向底层写入 remaining 字节，其余静默丢弃；用于收集有限长度的 ffmpeg stderr。
type capWriter struct {
	w         io.Writer
	remaining int
}

func (cw *capWriter) Write(p []byte) (int, error) {
	if cw.remaining <= 0 {
		return len(p), nil
	}
	if len(p) > cw.remaining {
		_, _ = cw.w.Write(p[:cw.remaining])
		cw.remaining = 0
		return len(p), nil
	}
	cw.remaining -= len(p)
	return cw.w.Write(p)
}
