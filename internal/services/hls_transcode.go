package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"songloft/internal/mediaexec"
	"strings"
	"sync"
	"time"
)

const HLSIdleTimeout = 120 * time.Second

// FFmpeg 只连接回环网关，所有 HLS 网络引用由网关解析和校验。
func hlsInputArgs(formats string) []string {
	return []string{"-http_proxy", "", "-protocol_whitelist", "http,tcp,crypto", "-format_whitelist", formats}
}

// HLSTranscodeError 是可公开给客户端的错误，不包含上游地址、凭证或进程输出。
type HLSTranscodeError struct {
	Status  int
	Message string
}

func (e *HLSTranscodeError) Error() string { return e.Message }

func hlsError(status int, message string) error {
	return &HLSTranscodeError{Status: status, Message: message}
}

// HLSTranscodeOptions 仅支持第一条 H.264 视频轨复制和第一条音频轨转 AAC-LC。
type HLSTranscodeOptions struct {
	URL          string `json:"url" validate:"required" example:"https://example.com/live.m3u8"`
	AudioBitrate int    `json:"audio_bitrate" example:"128"`
	UserAgent    string `json:"user_agent"`
	Referer      string `json:"referer"`
}

type hlsSession struct {
	id, owner, dir string
	ctx            context.Context
	cancel         context.CancelFunc
	ready, done    chan struct{}
	mu             sync.Mutex
	err            error
	lastAccess     time.Time
	readers        int
	removed        bool
	started        bool
}

// HLSTranscodeService 管理短期直播会话，不使用歌曲仓储或音乐文件缓存。
type HLSTranscodeService struct {
	cache                                                   *CacheService
	allowlist                                               func() []string
	root                                                    string
	mu                                                      sync.Mutex
	sessions                                                map[string]*hlsSession
	closed                                                  bool
	ctx                                                     context.Context
	cancel                                                  context.CancelFunc
	wg                                                      sync.WaitGroup
	idleTimeout, startupTimeout, stallTimeout, pollInterval time.Duration
	maxBytes                                                int64
}

func NewHLSTranscodeService(cache *CacheService, allowlist func() []string) (*HLSTranscodeService, error) {
	root, err := os.MkdirTemp("", "songloft-hls-")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &HLSTranscodeService{cache: cache, allowlist: allowlist, root: root,
		ctx: ctx, cancel: cancel,
		sessions: make(map[string]*hlsSession), idleTimeout: HLSIdleTimeout,
		startupTimeout: 30 * time.Second, stallTimeout: 30 * time.Second,
		pollInterval: 200 * time.Millisecond, maxBytes: 128 << 20}, nil
}

// StreamAudio 将直播源的第一条音轨实时转成 MP3。请求结束即停止拉流。
func (s *HLSTranscodeService) StreamAudio(ctx context.Context, w io.Writer, opts HLSTranscodeOptions) error {
	if err := validateHLSURL(opts.URL); err != nil {
		return err
	}
	if s.cache == nil || s.cache.getFFmpegPath() == "" {
		return hlsError(503, "ffmpeg 未配置")
	}
	ffmpeg, err := safeLookPath(s.cache.getFFmpegPath())
	if err != nil {
		return hlsError(503, "ffmpeg 不可用")
	}
	select {
	case seekStreamSem <- struct{}{}:
		defer func() { <-seekStreamSem }()
	default:
		return hlsError(503, "转码并发已满")
	}
	if s.ctx.Err() != nil {
		return hlsError(503, "转码服务已关闭")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	gateway, err := newHLSPullGateway(ctx, opts, s.allowlist)
	if err != nil {
		var publicErr *HLSTranscodeError
		if errors.As(err, &publicErr) {
			return publicErr
		}
		return hlsError(500, "无法初始化直播拉流服务")
	}
	defer gateway.Close()
	args := append([]string{"-nostdin", "-hide_banner", "-loglevel", "error"}, hlsInputArgs("hls,mpegts,mov,flv")...)
	args = append(args, "-rw_timeout", "15000000", "-re", "-i", gateway.URL(opts.URL),
		"-map", "0:a:0", "-vn", "-c:a", "libmp3lame", "-b:a", "128k",
		"-write_xing", "0", "-id3v2_version", "0", "-f", "mp3", "pipe:1")
	err = streamRadio(ctx, w, ffmpeg, args, s.startupTimeout)
	if errors.Is(err, ErrRadioTranscodeUnavailable) {
		return gateway.failure("无法从直播源提取音频")
	}
	return err
}

// Create 预留共享转码槽位；只有首个完整分片和播放列表就绪后才交付会话。
func (s *HLSTranscodeService) Create(ctx context.Context, owner string, opts HLSTranscodeOptions) (string, error) {
	return s.create(ctx, owner, opts, "hls,mpegts,mov")
}

// CreateFLV creates the compatibility stream for a saved HTTP-FLV station.
func (s *HLSTranscodeService) CreateFLV(ctx context.Context, owner string, opts HLSTranscodeOptions) (string, error) {
	return s.create(ctx, owner, opts, "flv")
}

func (s *HLSTranscodeService) create(ctx context.Context, owner string, opts HLSTranscodeOptions, inputFormats string) (string, error) {
	if owner == "" {
		return "", hlsError(401, "缺少客户端身份")
	}
	if opts.AudioBitrate == 0 {
		opts.AudioBitrate = 128
	}
	switch opts.AudioBitrate {
	case 64, 96, 128, 192, 256:
	default:
		return "", hlsError(400, "audio_bitrate 仅支持 64/96/128/192/256")
	}
	if len(opts.URL) > 8192 || len(opts.UserAgent) > 2048 || len(opts.Referer) > 8192 || strings.ContainsAny(opts.UserAgent+opts.Referer, "\r\n") {
		return "", hlsError(400, "无效的 URL 或拉流请求头")
	}
	if err := validateHLSURL(opts.URL); err != nil {
		return "", err
	}
	if s.cache == nil || s.cache.getFFmpegPath() == "" {
		return "", hlsError(503, "ffmpeg 未配置")
	}
	if _, err := safeLookPath(s.cache.getFFmpegPath()); err != nil {
		return "", hlsError(503, "ffmpeg 不可用")
	}
	if _, err := safeLookPath(s.cache.resolveFFprobePath()); err != nil {
		return "", hlsError(503, "ffprobe 不可用")
	}
	select {
	case seekStreamSem <- struct{}{}:
	default:
		return "", hlsError(503, "转码并发已满")
	}
	var random [24]byte
	if _, err := rand.Read(random[:]); err != nil {
		<-seekStreamSem
		return "", err
	}
	id := hex.EncodeToString(random[:])
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		<-seekStreamSem
		return "", hlsError(503, "转码服务已关闭")
	}
	if len(s.sessions) >= 16 {
		s.mu.Unlock()
		<-seekStreamSem
		return "", hlsError(503, "直播会话数量已满")
	}
	dir := filepath.Join(s.root, id)
	if err := os.Mkdir(dir, 0700); err != nil {
		s.mu.Unlock()
		<-seekStreamSem
		return "", err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	session := &hlsSession{id: id, owner: owner, dir: dir, ctx: runCtx, cancel: cancel,
		ready: make(chan struct{}), done: make(chan struct{}), lastAccess: time.Now()}
	s.sessions[id] = session
	s.wg.Add(1)
	s.mu.Unlock()
	go s.run(session, opts, inputFormats)
	timer := time.NewTimer(s.startupTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		cancel()
		<-session.done
		return "", ctx.Err()
	case <-timer.C:
		cancel()
		<-session.done
		return "", hlsError(504, "直播转码启动超时")
	case <-session.ready:
		session.mu.Lock()
		err := session.err
		session.lastAccess = time.Now()
		session.mu.Unlock()
		if ctx.Err() != nil {
			cancel()
			<-session.done
			return "", ctx.Err()
		}
		if err != nil {
			cancel()
			<-session.done
			return "", err
		}
		return id, nil
	}
}

func (s *HLSTranscodeService) run(session *hlsSession, opts HLSTranscodeOptions, inputFormats string) {
	defer s.wg.Done()
	defer close(session.done)
	defer s.remove(session)
	defer session.cancel()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { <-seekStreamSem }) }
	defer release()
	var readyOnce sync.Once
	ready := func(err error) {
		readyOnce.Do(func() {
			session.mu.Lock()
			session.err = err
			session.started = err == nil
			session.lastAccess = time.Now()
			session.mu.Unlock()
			close(session.ready)
		})
	}
	defer ready(hlsError(502, "直播转码已停止"))
	processCtx, stopProcess := context.WithCancel(session.ctx)
	defer stopProcess()
	gateway, err := newHLSPullGateway(processCtx, opts, s.allowlist)
	if err != nil {
		ready(err)
		return
	}
	defer gateway.Close()
	input := gateway.URL(opts.URL)
	// 只接受调用方指定的媒体格式；所有网络引用先经过受控网关。
	// FFmpeg 只连接本机网关，不能继承环境 http_proxy 把能力 URL 发给外部代理。
	inputArgs := hlsInputArgs(inputFormats)
	probeCtx, probeCancel := context.WithTimeout(processCtx, s.startupTimeout)
	probeArgs := append([]string{"-v", "error"}, inputArgs...)
	probeArgs = append(probeArgs, "-analyzeduration", "2000000", "-probesize", "2097152", "-show_entries", "stream=codec_type,codec_name,field_order:stream_disposition=attached_pic", "-of", "json", input)
	probe := mediaexec.FFprobe(probeCtx, s.cache.resolveFFprobePath(), probeArgs...)
	var probeData strings.Builder
	probe.Stdout = &capWriter{w: &probeData, remaining: 1 << 20}
	err = probe.Run()
	probeCancel()
	if err != nil {
		ready(gateway.failure("无法探测上游媒体"))
		return
	}
	var metadata struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			FieldOrder  string `json:"field_order"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if json.Unmarshal([]byte(probeData.String()), &metadata) != nil {
		ready(hlsError(502, "上游媒体信息无效"))
		return
	}
	video, audio := false, false
	for _, stream := range metadata.Streams {
		if stream.CodecType == "video" && !video {
			if stream.CodecName != "h264" || stream.Disposition.AttachedPic != 0 || (stream.FieldOrder != "" && stream.FieldOrder != "unknown" && stream.FieldOrder != "progressive") {
				ready(hlsError(422, "仅支持逐行 H.264 视频复制"))
				return
			}
			video = true
		}
		if stream.CodecType == "audio" {
			audio = true
		}
	}
	if !video || !audio {
		ready(hlsError(422, "源必须同时包含视频轨和音频轨"))
		return
	}
	args := append([]string{"-nostdin", "-hide_banner", "-loglevel", "error"}, inputArgs...)
	args = append(args, "-re", "-i", input, "-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy", "-c:a", "aac", "-profile:a", "aac_low", "-ac", "2", "-ar", "48000", "-b:a", fmt.Sprintf("%dk", opts.AudioBitrate),
		"-f", "hls", "-hls_segment_type", "mpegts", "-hls_time", "4", "-hls_list_size", "6", "-hls_delete_threshold", "3", "-hls_flags", "delete_segments+temp_file", "-hls_segment_filename", filepath.Join(session.dir, "segment-%09d.ts"), filepath.Join(session.dir, "playlist.m3u8"))
	cmd := mediaexec.FFmpeg(processCtx, s.cache.getFFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &capWriter{w: &stderr, remaining: 4096}
	if err := cmd.Start(); err != nil {
		ready(hlsError(503, "无法启动 ffmpeg"))
		return
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	running := true
	defer func() {
		if running {
			stopProcess()
			<-wait
		}
	}()
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	lastProgress := time.Now()
	var lastMod time.Time
	for {
		select {
		case <-session.ctx.Done():
			return
		case err := <-wait:
			running = false
			wait = nil
			release()
			if err != nil && processCtx.Err() == nil {
				slog.Warn("HLS transcode failed", "session", session.id, "stderr", regexp.MustCompile(`https?://[^\s'\"]+`).ReplaceAllString(stderr.String(), "[redacted-url]"))
				session.mu.Lock()
				session.err = gateway.failure("直播转码异常结束")
				session.mu.Unlock()
				ready(gateway.failure("直播转码异常结束"))
			}
		case now := <-ticker.C:
			session.mu.Lock()
			idle := session.started && now.Sub(session.lastAccess) > s.idleTimeout
			failed := session.err != nil
			session.mu.Unlock()
			if idle {
				return
			}
			if failed {
				continue
			}
			if err := s.checkStorage(session.dir); err != nil {
				session.mu.Lock()
				session.err = err
				session.mu.Unlock()
				ready(err)
				stopProcess()
				continue
			}
			if info, err := os.Stat(filepath.Join(session.dir, "playlist.m3u8")); err == nil && info.ModTime() != lastMod {
				lastMod = info.ModTime()
				lastProgress = now
				if s.playlistReady(session.dir) {
					ready(nil)
				}
			}
			if running && now.Sub(lastProgress) > s.stallTimeout {
				session.mu.Lock()
				session.err = hlsError(502, "直播源长时间没有生成分片")
				session.mu.Unlock()
				ready(hlsError(502, "直播源长时间没有生成分片"))
				stopProcess()
			}
			if !running {
				ready(hlsError(502, "上游未产生可播放分片"))
			}
		}
	}
}

func (s *HLSTranscodeService) playlistReady(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if hlsSegmentName.MatchString(line) {
			info, err := os.Stat(filepath.Join(dir, line))
			return err == nil && info.Size() > 0
		}
	}
	return false
}

func (s *HLSTranscodeService) checkStorage(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return hlsError(502, "无法读取直播分片目录")
	}
	var total int64
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil {
			total += info.Size()
		}
	}
	if total > s.maxBytes {
		return hlsError(502, "直播临时缓冲超过空间上限")
	}
	return nil
}

func (s *HLSTranscodeService) get(id, owner string) (*hlsSession, error) {
	s.mu.Lock()
	session := s.sessions[id]
	s.mu.Unlock()
	if session == nil || owner == "" || session.owner != owner {
		return nil, hlsError(404, "直播会话不存在或已回收")
	}
	return session, nil
}

func (s *HLSTranscodeService) Playlist(id, owner string) ([]byte, error) {
	session, err := s.get(id, owner)
	if err != nil {
		return nil, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.removed {
		return nil, hlsError(404, "直播会话已回收")
	}
	if session.err != nil {
		return nil, session.err
	}
	data, err := os.ReadFile(filepath.Join(session.dir, "playlist.m3u8"))
	if err != nil {
		return nil, hlsError(502, "无法读取直播播放列表")
	}
	session.lastAccess = time.Now()
	return data, nil
}

var hlsSegmentName = regexp.MustCompile(`^segment-[0-9]{9,20}\.ts$`)

// OpenSegment 的关闭回调持有文件租约，确保 Windows 上也先关闭文件再删除目录。
func (s *HLSTranscodeService) OpenSegment(id, owner, name string) (*os.File, func(), error) {
	if !hlsSegmentName.MatchString(name) {
		return nil, nil, hlsError(400, "无效的分片名称")
	}
	session, err := s.get(id, owner)
	if err != nil {
		return nil, nil, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.removed {
		return nil, nil, hlsError(404, "直播会话已回收")
	}
	if session.err != nil {
		return nil, nil, session.err
	}
	f, err := os.Open(filepath.Join(session.dir, name))
	if err != nil {
		return nil, nil, hlsError(404, "分片不存在或已过期")
	}
	session.readers++
	session.lastAccess = time.Now()
	var once sync.Once
	closeFile := func() {
		once.Do(func() {
			_ = f.Close()
			session.mu.Lock()
			defer session.mu.Unlock()
			session.readers--
			if session.removed && session.readers == 0 {
				_ = os.RemoveAll(session.dir)
				s.mu.Lock()
				if s.closed {
					_ = os.Remove(s.root)
				}
				s.mu.Unlock()
			}
		})
	}
	return f, closeFile, nil
}

func (s *HLSTranscodeService) Delete(id, owner string) error {
	session, err := s.get(id, owner)
	if err != nil {
		return err
	}
	session.cancel()
	<-session.done
	return nil
}

func (s *HLSTranscodeService) remove(session *hlsSession) {
	s.mu.Lock()
	delete(s.sessions, session.id)
	s.mu.Unlock()
	session.mu.Lock()
	defer session.mu.Unlock()
	session.removed = true
	if session.readers == 0 {
		_ = os.RemoveAll(session.dir)
	}
}

func (s *HLSTranscodeService) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	for _, session := range s.sessions {
		session.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
	_ = os.Remove(s.root)
}

// 保持错误判断可供 handler 使用，不把 ffmpeg 原始错误作为 detail 外发。
func HLSTranscodeStatus(err error) int {
	var e *HLSTranscodeError
	if errors.As(err, &e) {
		return e.Status
	}
	if errors.Is(err, os.ErrNotExist) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
