package services

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"songloft/internal/httputil"
	"songloft/internal/mediaexec"
)

// ErrURLTranscodeUnavailable 表示远程 URL 转码在吐出任何音频字节之前失败
// （ffmpeg 未配置 / 启动失败 / 并发已满 / 上游不可解码 / 鉴权失败）。
// 调用方拿到此错误时尚未向 w 写入字节，可安全返回 503。
var ErrURLTranscodeUnavailable = errors.New("url transcode unavailable")

// URLTranscodeOptions 远程音频 URL 实时转码参数。
type URLTranscodeOptions struct {
	UpstreamURL string  // 远程音频直链，ffmpeg -i 直拉
	Format      string  // 目前仅 mp3（pipe 不可 seek，CBR 让音箱估算时长）
	Bitrate     int     // 目标码率 kbps，0 → 320
	Duration    float64 // 音频时长（秒），用于硬超时；<=0 未知（用 3h 兜底）
	UserAgent   string  // 拉流 UA，空则由 handler 层填默认浏览器 UA
	Referer     string  // 拉流 Referer，空不带
}

// StreamTranscodedURL 阻塞运行 ffmpeg，把远程音频 URL 实时转码为 mp3 流写入 w，
// 直到 ffmpeg 结束或 ctx 取消（客户端断开）。
//
// 用途：miot「不入库直接播放」场景下，外部搜索源返回 webm/opus 等音箱无法解码的
// 直链时，把本端点 URL 推给音箱（songloft-org/songloft#394）。
//
// 与 runFFmpeg 不同，本函数不占用 transcodeSem（进程存活整首歌时长），而是与 seek /
// 均衡流共享 seekStreamSem——同一类长存活全解码 ffmpeg 进程，统一 CPU 上限。满了不
// 排队（排队 = 音箱干等，比 503 更糟）。不落盘、不入库、不缓存（no-import 语义）。
//
// 输出刻意用 CBR 320k + -write_xing 0 + -map 0:a:0 -vn，与 StreamSeekedMP3 一致：
// pipe 不可 seek，音箱只能按「首帧码率 × 字节数」估算时长，VBR 会偏 7%+ 导致提前切歌。
//
// 返回值：
//   - nil：正常结束，或 ctx 已取消（客户端断开，无人在听，无害）。
//   - ErrURLTranscodeUnavailable：写出任何字节前失败，调用方应返回 503（此时 w 未被写入）。
//   - 其他 error：已写出部分字节后中途失败，无法再降级，调用方仅记录日志。
func (c *CacheService) StreamTranscodedURL(ctx context.Context, w io.Writer, opts URLTranscodeOptions) error {
	if opts.UpstreamURL == "" {
		return fmt.Errorf("%w: empty url", ErrURLTranscodeUnavailable)
	}
	ffmpegPath := c.getFFmpegPath()
	if ffmpegPath == "" {
		return fmt.Errorf("%w: ffmpeg not configured", ErrURLTranscodeUnavailable)
	}
	// 仅支持 mp3 输出：其他格式与「pipe 不可 seek、靠字节估算时长」的假设冲突。
	if opts.Format != "" && NormalizeFormat(opts.Format) != "mp3" {
		return fmt.Errorf("%w: unsupported format %q", ErrURLTranscodeUnavailable, opts.Format)
	}

	// 与 seek / 均衡流共享 cap=4：同类长存活全解码 ffmpeg，统一 CPU 上限。
	select {
	case seekStreamSem <- struct{}{}:
		defer func() { <-seekStreamSem }()
	default:
		return fmt.Errorf("%w: %d concurrent transcode streams already running",
			ErrURLTranscodeUnavailable, seekStreamMaxConcurrent)
	}

	bitrate := opts.Bitrate
	if bitrate <= 0 {
		bitrate = 320
	}

	args := []string{"-hide_banner", "-loglevel", "error"}
	if opts.UserAgent != "" {
		args = append(args, "-user_agent", opts.UserAgent)
	}
	if opts.Referer != "" {
		// ffmpeg 的 http 输入通过 -headers 追加请求头，多个头以 \r\n 分隔。
		args = append(args, "-headers", "Referer: "+opts.Referer+"\r\n")
	}
	// 复用全局 http-proxy（/settings/http-proxy）：上游可能在 GFW 后，ffmpeg 直拉远程
	// URL 不走 httputil.NewClient 的代理，这里显式注入 -http_proxy（input option，须在
	// -i 之前）。仅 http/https 代理；SOCKS5 不在本方法支持范围。
	if proxyURL := httputil.GetGlobalProxy(); proxyURL != "" {
		args = append(args, "-http_proxy", proxyURL)
	}
	args = append(args,
		"-i", opts.UpstreamURL,
		"-map", "0:a:0", "-vn",
		"-codec:a", "libmp3lame", "-b:a", fmt.Sprintf("%dk", bitrate),
		"-write_xing", "0", "-f", "mp3", "pipe:1",
	)

	// 硬超时：duration 已知则 remaining+宽限；未知 3h 兜底，靠 r.Context() 回收孤儿进程。
	timeout := seekStreamTimeout(opts.Duration, 1.0)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := mediaexec.FFmpeg(runCtx, ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%w: stdout pipe: %v", ErrURLTranscodeUnavailable, err)
	}
	// stderr 收集有限长度用于诊断；不用 CombinedOutput 因为 stdout 要流式转发。
	var stderr bytes.Buffer
	cmd.Stderr = &capWriter{w: &stderr, remaining: 4096}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start: %v", ErrURLTranscodeUnavailable, err)
	}

	reader := bufio.NewReaderSize(stdout, 64*1024)
	// 预读第一字节：只有确认 ffmpeg 真的产出了音频才提交响应。若在任何字节之前就 EOF/失败
	// （坏源、鉴权失败、容器不可解码等），返回 ErrURLTranscodeUnavailable 让调用方返回 503
	// ——此时尚未写出任何字节。
	first, _ := reader.Peek(1)
	if len(first) == 0 {
		_ = cmd.Wait()
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "no output"
		}
		return fmt.Errorf("%w: %s", ErrURLTranscodeUnavailable, msg)
	}

	// 已确认有音频输出：从此刻起本函数拥有响应，流式转发直到结束或断开。
	_, copyErr := io.Copy(&flushingWriter{w: w}, reader)
	// copyErr 时必须先 cancel 再 Wait：ffmpeg 此刻正阻塞在写满的 stdout 管道上，而 cmd.Wait()
	// 要等进程退出才关管道——两边互等，只能靠 runCtx 到点解开，把本 goroutine 连同一个
	// seekStreamSem 槽位挂到硬超时。ctx 已取消的场景由 CommandContext 自己解决。
	if copyErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()

	// ctx 取消（客户端断开）是正常收尾，不当作错误。
	if ctx.Err() != nil {
		return nil
	}
	if copyErr != nil {
		return copyErr
	}
	// 硬超时到点：音频被截断，但字节已写出、响应正常闭合，客户端会以为播完了。
	// 唯一能做的是留下可归因的日志——正常结束与被杀掉在 waitErr 里长得一样。
	if runCtx.Err() != nil {
		slog.Error("url transcode truncated by hard timeout",
			"url", opts.UpstreamURL, "duration", opts.Duration, "timeout", timeout)
		return nil
	}
	if waitErr != nil {
		slog.Warn("url transcode ffmpeg exited with error",
			"url", opts.UpstreamURL, "stderr", strings.TrimSpace(stderr.String()), "error", waitErr)
	}
	return nil
}
