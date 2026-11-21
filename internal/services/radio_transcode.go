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
	"strings"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/mediaexec"
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
	ffmpegPath := c.getFFmpegPath()
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
	// 复用全局 http-proxy（/settings/http-proxy）：电台源可能在 GFW 后，ffmpeg 直拉
	// 远程 URL 不走 httputil.NewClient 的代理，这里显式注入 -http_proxy（input option，
	// 须在 -i 之前）。仅 http/https 代理；SOCKS5 不在本方法支持范围。
	if proxyURL := httputil.GetGlobalProxy(); proxyURL != "" {
		args = append(args, "-http_proxy", proxyURL)
	}
	args = append(args, "-i", opts.UpstreamURL, "-vn", "-codec:a", encoder)
	args = append(args, qualityArgs...)
	args = append(args, "-f", muxer, "pipe:1")
	_, err = streamRadio(ctx, w, ffmpegPath, args, 0, false)
	return err
}

// streamRadio reports a clean EOF only if the encoder produced audio and the client is still connected.
func streamRadio(ctx context.Context, w io.Writer, ffmpegPath string, args []string, startupTimeout time.Duration, bridgeSilence bool) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := mediaexec.FFmpeg(ctx, ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("%w: stdout pipe: %v", ErrRadioTranscodeUnavailable, err)
	}
	// stderr 收集有限长度用于诊断；不用 CombinedOutput 因为 stdout 要流式转发。
	var stderr bytes.Buffer
	cmd.Stderr = &capWriter{w: &stderr, remaining: 4096}

	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return false, fmt.Errorf("%w: start: %v", ErrRadioTranscodeUnavailable, err)
	}

	var deadline *time.Timer
	if startupTimeout > 0 {
		deadline = time.AfterFunc(startupTimeout, cancel)
		defer deadline.Stop()
	}
	reader := bufio.NewReaderSize(stdout, 64*1024)
	// 预读第一字节：只有确认 ffmpeg 真的产出了音频，才提交响应头/首字节。
	// 若在任何字节之前就 EOF/失败（坏源、鉴权失败、编码器不支持容器等），返回
	// ErrRadioTranscodeUnavailable 让调用方降级为原样代理——此时尚未写出任何字节。
	var first []byte
	if bridgeSilence {
		var err error
		first, err = peekRadioWithSilence(ctx, cancel, &flushingWriter{w: w}, reader)
		if err != nil {
			_ = cmd.Wait()
			return false, err
		}
	} else {
		first, _ = reader.Peek(1)
	}
	if deadline != nil && !deadline.Stop() {
		cancel()
		_ = cmd.Wait()
		return false, hlsError(http.StatusGatewayTimeout, "电台纯音频转码启动超时")
	}
	if len(first) == 0 {
		_ = cmd.Wait()
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "no output"
		}
		return false, fmt.Errorf("%w: %s", ErrRadioTranscodeUnavailable, msg)
	}

	// 已确认有音频输出：从此刻起本函数拥有响应，流式转发直到结束或断开。
	_, copyErr := io.Copy(&flushingWriter{w: w}, reader)
	canceled := ctx.Err() != nil
	if copyErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()

	// ctx 取消（客户端断开）是正常收尾，不当作错误。
	if canceled {
		return false, nil
	}
	if copyErr != nil {
		return false, copyErr
	}
	if ctx.Err() == nil && waitErr != nil {
		log := slog.Default()
		if startupTimeout == 0 {
			// Only the legacy command uses stderr for diagnostics. The gateway input
			// address contains an encoded upstream URL and must not be logged.
			log = log.With("stderr", strings.TrimSpace(stderr.String()))
		}
		log.Warn("radio transcode ffmpeg exited with error", "error", waitErr)
	}
	return waitErr == nil && ctx.Err() == nil, nil
}

func peekRadioWithSilence(ctx context.Context, cancel context.CancelFunc, w io.Writer, reader *bufio.Reader) ([]byte, error) {
	ready := make(chan []byte, 1)
	go func() {
		first, _ := reader.Peek(1)
		ready <- first
	}()

	// At 128 kbps and 48 kHz, each MPEG-1 Layer III frame is 384 bytes and lasts
	// 24 ms. Zero side information and main data encode silence in this format.
	const frameSize, framesPerTick = 384, 12
	var frames [frameSize * framesPerTick]byte
	for frame := 0; frame < len(frames); frame += frameSize {
		copy(frames[frame:], []byte{0xff, 0xfb, 0x94, 0x04})
	}

	// Keep the client's buffer fed during a short reconnect. Leave a longer outage
	// to the normal startup timeout, with only one goroutine writing to the response.
	ticker := time.NewTicker(framesPerTick * 24 * time.Millisecond)
	defer ticker.Stop()
	for range 20 {
		select {
		case first := <-ready:
			return first, nil
		case <-ctx.Done():
			return <-ready, nil
		case <-ticker.C:
			if _, err := w.Write(frames[:]); err != nil {
				cancel()
				<-ready
				return nil, err
			}
		}
	}
	return <-ready, nil
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
