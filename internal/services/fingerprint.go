package services

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"songloft/internal/database"
	"songloft/internal/mediaexec"
)

var (
	chromaprintAvailable bool
	chromaprintOnce      sync.Once
	resolvedFFmpegPath   string
	durationRe           = regexp.MustCompile(`Duration:\s+(\d+):(\d+):(\d+)\.(\d+)`)
)

// configuredFFmpegPath 由 app 启动时从 config 表的 ffmpeg_path 注入，空则回退到 PATH 查找。
var configuredFFmpegPath atomic.Value // stores string

// SetFingerprintFFmpegPath 注入 ffmpeg 可执行文件路径（config 表的 ffmpeg_path）。
// 必须在首次 IsChromaprintAvailable 之前调用，否则 sync.Once 已经用 PATH 结果定型。
func SetFingerprintFFmpegPath(path string) {
	configuredFFmpegPath.Store(path)
}

// IsChromaprintAvailable 检测 ffmpeg 是否支持 chromaprint muxer（首次调用时检测，结果缓存）。
func IsChromaprintAvailable() bool {
	chromaprintOnce.Do(func() {
		name := ""
		if v := configuredFFmpegPath.Load(); v != nil {
			name = v.(string)
		}
		if name == "" {
			name = "ffmpeg"
		}
		path, err := safeLookPath(name)
		if err != nil {
			return
		}
		out, err := mediaexec.FFmpeg(context.Background(), path, "-hide_banner", "-muxers").Output()
		if err == nil && strings.Contains(string(out), "chromaprint") {
			chromaprintAvailable = true
			resolvedFFmpegPath = path
		}
	})
	return chromaprintAvailable
}

func parseDurationFromStderr(stderr string) float64 {
	matches := durationRe.FindStringSubmatch(stderr)
	if len(matches) < 5 {
		return 0
	}
	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	frac, _ := strconv.Atoi(matches[4])
	divisor := 1.0
	for i := 0; i < len(matches[4]); i++ {
		divisor *= 10
	}
	return float64(hours)*3600 + float64(minutes)*60 + float64(seconds) + float64(frac)/divisor
}

const (
	// fingerprintSampleSeconds 是指纹的采样长度。AcoustID 的事实标准就是前 120 秒；
	// 不限长会让有声书 / 整轨镜像这类长音频每次都全解码，把 CPU 打满
	// （songloft-org/songloft#323）。
	fingerprintSampleSeconds = 120
	// fingerprintTimeout 是单个文件的硬超时。有了采样上限后正常文件远低于此，
	// 留足余量是因为失败会被永久标记，宁可宽松也不要误判。
	fingerprintTimeout = 30 * time.Second
)

// ExtractFingerprint 调用 ffmpeg chromaprint muxer 提取音频指纹。
// startSeconds / endSeconds 仅 CUE 轨非零：CUE 轨的 filePath 指向整轨镜像，
// 必须按区间采样，否则同一镜像下的所有 track 会拿到完全相同的指纹。
func ExtractFingerprint(ctx context.Context, filePath string, startSeconds, endSeconds float64) (string, float64, error) {
	ctx, cancel := context.WithTimeout(ctx, fingerprintTimeout)
	defer cancel()

	args := make([]string, 0, 12)
	if startSeconds > 0 {
		args = append(args, "-ss", strconv.FormatFloat(startSeconds, 'f', 3, 64))
	}
	args = append(args, "-i", filePath, "-map", "0:a:0", "-map_metadata", "-1")
	// CUE 轨要夹在轨内，避免采样溢出到下一轨。
	sample := float64(fingerprintSampleSeconds)
	if trackLen := endSeconds - startSeconds; trackLen > 0 && trackLen < sample {
		sample = trackLen
	}
	args = append(args, "-t", strconv.FormatFloat(sample, 'f', 3, 64),
		"-f", "chromaprint", "-fp_format", "base64", "-")

	cmd := mediaexec.FFmpeg(ctx, resolvedFFmpegPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// ffmpeg 若派生了持有管道的子进程，SIGKILL 后 Wait 仍可能挂住 worker。
	cmd.WaitDelay = 5 * time.Second

	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("ffmpeg chromaprint: %w (%s)", err, stderr.String())
	}

	fingerprint := strings.TrimSpace(stdout.String())
	if nl := strings.IndexByte(fingerprint, '\n'); nl >= 0 {
		fingerprint = fingerprint[:nl]
	}
	if fingerprint == "" {
		return "", 0, fmt.Errorf("ffmpeg chromaprint returned empty fingerprint")
	}

	// CUE 轨的 stderr Duration 是整轨镜像的时长，用轨长才有意义
	// （去重时会按这个时长做二次护栏）。
	if trackLen := endSeconds - startSeconds; trackLen > 0 {
		return fingerprint, trackLen, nil
	}
	duration := parseDurationFromStderr(stderr.String())
	// CUE 的**最后一轨** endSeconds 为 0（含义是「到文件末尾」，见 pkg/cue.ResolvedTrack），
	// 此时 trackLen 为负走不到上面的分支。若直接返回 stderr 的时长，最后一轨会拿到整轨
	// 镜像的时长（同专辑其他轨拿的是轨长），去重时永远无法与它真正的副本聚到一簇。
	if startSeconds > 0 && duration > startSeconds {
		return fingerprint, duration - startSeconds, nil
	}
	return fingerprint, duration, nil
}

// FingerprintProgress 指纹计算进度。
type FingerprintProgress struct {
	Status   string `json:"status"` // idle, running, done, cancelled
	Computed int64  `json:"computed"`
	Total    int64  `json:"total"`
	Failed   int64  `json:"failed"`
}

// FingerprintService 管理指纹计算的异步任务。
type FingerprintService struct {
	songs SongRepository

	mu       sync.Mutex
	running  bool
	cancelFn context.CancelFunc
	done     chan struct{}
	progress FingerprintProgress
}

// NewFingerprintService 创建指纹服务。
func NewFingerprintService(songs SongRepository) *FingerprintService {
	return &FingerprintService{
		songs:    songs,
		progress: FingerprintProgress{Status: "idle"},
	}
}

// GetProgress 返回当前计算进度。
func (s *FingerprintService) GetProgress() FingerprintProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

// fingerprintComputeMode 控制 startCompute 启动前的预处理。
type fingerprintComputeMode int

const (
	// fpModeMissing 只算从未尝试过的歌曲。
	fpModeMissing fingerprintComputeMode = iota
	// fpModeRecomputeAll 清空全部指纹后重算。
	fpModeRecomputeAll
	// fpModeRetryFailed 仅重置失败标记后重试，保留已算好的指纹。
	fpModeRetryFailed
)

// ComputeMissing 为所有缺失指纹的本地歌曲计算指纹。
// 若已有任务在运行，打断旧任务后重新启动。
func (s *FingerprintService) ComputeMissing() (int, error) {
	return s.startCompute(fpModeMissing)
}

// RecomputeAll 清空所有已有指纹后重新计算全部本地歌曲的指纹。
// 会同时重置「已尝试」标记，因此此前失败的歌曲也会被重试。
// 若已有任务在运行，打断旧任务后重新启动。
func (s *FingerprintService) RecomputeAll() (int, error) {
	return s.startCompute(fpModeRecomputeAll)
}

// RetryFailed 仅重置失败项的「已尝试」标记后重试，已算好的指纹保持不动。
// 适用于 ffmpeg 能力升级（如新增 mpeg 解复用器）后恢复此前因解码器缺失
// 而失败的歌曲，代价远低于「重新计算全部」。
// 若已有任务在运行，打断旧任务后重新启动。
func (s *FingerprintService) RetryFailed() (int, error) {
	return s.startCompute(fpModeRetryFailed)
}

// Cancel 中断正在运行的指纹计算任务并等待其收尾。
// 返回是否确实中断了一个在跑的任务。
// 指纹任务不挂在扫描的 cancelCh 上（扫描 Complete() 后该通道已关闭置 nil），
// 所以需要一个独立入口，否则大库跑起来只能靠重启进程停下。
func (s *FingerprintService) Cancel() bool {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return false
	}
	s.cancelFn()
	done := s.done
	s.mu.Unlock()

	<-done
	s.mu.Lock()
	// 等待期间可能已有新任务启动（如扫描尾部的 runAutoFingerprint）。
	// 那时 s.done 已被换成新 channel，绝不能把新任务的进度覆写成 cancelled，
	// 也不能报告 cancelled=true——否则前端会停掉轮询、显示「已停止」，
	// 而 ffmpeg 仍在后台跑，正是 #323 要消灭的状态。
	stopped := s.done == done
	if stopped {
		s.progress.Status = "cancelled"
	}
	s.mu.Unlock()
	return stopped
}

func (s *FingerprintService) startCompute(mode fingerprintComputeMode) (int, error) {
	s.mu.Lock()
	// 必须用 for 而不是 if：两个并发的 startCompute（双击「重新计算全部」、
	// 或手动触发撞上扫描尾部的 runAutoFingerprint）会等在同一个 done 上，
	// 用 if 时后醒的那个会直接覆盖前一个刚装好的任务状态——
	// 两个 doCompute 同时在跑（ffmpeg 翻倍）且可能 close 同一 channel 两次而 panic。
	for s.running {
		s.cancelFn()
		done := s.done
		s.mu.Unlock()
		<-done
		s.mu.Lock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	// 启动前预处理：清库重算 / 仅重置失败标记，失败时回滚任务状态。
	var prepErr error
	switch mode {
	case fpModeRecomputeAll:
		if err := s.songs.ClearAllFingerprints(ctx); err != nil {
			prepErr = fmt.Errorf("clear fingerprints: %w", err)
		}
	case fpModeRetryFailed:
		if err := s.songs.ResetFailedFingerprintAttempts(ctx); err != nil {
			prepErr = fmt.Errorf("reset failed attempts: %w", err)
		}
	}
	if prepErr != nil {
		cancel()
		s.mu.Lock()
		s.running = false
		close(s.done)
		s.progress = FingerprintProgress{Status: "idle"}
		s.mu.Unlock()
		return 0, prepErr
	}

	missing, err := s.songs.ListLocalWithoutFingerprint(ctx)
	if err != nil {
		cancel()
		s.mu.Lock()
		s.running = false
		close(s.done)
		s.progress = FingerprintProgress{Status: "idle"}
		s.mu.Unlock()
		return 0, fmt.Errorf("list missing: %w", err)
	}

	total := len(missing)
	s.mu.Lock()
	s.progress = FingerprintProgress{Status: "running", Total: int64(total)}
	s.mu.Unlock()

	if total == 0 {
		cancel()
		s.mu.Lock()
		s.running = false
		close(s.done)
		s.progress = FingerprintProgress{Status: "done", Total: 0}
		s.mu.Unlock()
		return 0, nil
	}

	go s.doCompute(ctx, missing)
	return total, nil
}

// fpWorkerCount 返回指纹计算的并发度。
// 指纹是后台低优先级任务，只取 GOMAXPROCS 的四分之一（下限 1、上限 4），
// 避免在 4 核 NAS 这类设备上把 CPU 打满（songloft-org/songloft#323）。
// Go 的 GOMAXPROCS 感知 cgroup CPU 限额，所以 Docker 限核也能正确收敛。
func fpWorkerCount() int {
	n := runtime.GOMAXPROCS(0) / 4
	if n < 1 {
		return 1
	}
	if n > 4 {
		return 4
	}
	return n
}

func (s *FingerprintService) doCompute(ctx context.Context, items []database.SongIDPath) {
	// 捕获本任务自己的 done：绝不能关 s.done 字段。字段可能已被后启动的任务换掉，
	// 那样既会漏掉本任务的等待者（永久阻塞），又可能把别人的 channel 关两次而 panic。
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.progress.Status = "done"
		close(done)
		s.mu.Unlock()
	}()

	workers := fpWorkerCount()
	var computed, failed atomic.Int64
	ch := make(chan database.SongIDPath, workers*2)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range ch {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// 文件当前访问不到（网络存储掉线、卷还没挂上）属**短暂性**失败，
				// 不能打「已尝试」标记：否则 NAS 掉线一次就会把整库标成永久失败，
				// 只能靠「重新计算全部」全量重算才能恢复。
				if _, statErr := os.Stat(item.FilePath); statErr != nil {
					slog.Warn("fingerprint skipped, file unreachable (will retry later)",
						"id", item.ID, "path", item.FilePath, "err", statErr)
					failed.Add(1)
					s.mu.Lock()
					s.progress.Failed = failed.Load()
					s.mu.Unlock()
					continue
				}

				// ffmpeg 真的读了文件仍失败（无音轨 / 损坏 / 超时）才落库「已尝试」时间戳：
				// 否则失败项会在每轮扫描里被反复捞出来重跑 ffmpeg，形成永久 CPU 占用。
				attemptedAt := time.Now().Unix()
				fp, dur, err := ExtractFingerprint(ctx, item.FilePath, item.CueStartSeconds, item.CueEndSeconds)
				if err != nil {
					// 任务被取消时不标记，留给下次重试。
					if ctx.Err() != nil {
						return
					}
					slog.Info("fingerprint failed", "id", item.ID, "path", item.FilePath, "err", err)
					if markErr := s.songs.MarkFingerprintAttempted(ctx, item.ID, attemptedAt, err.Error()); markErr != nil {
						slog.Warn("mark fingerprint attempted failed", "id", item.ID, "err", markErr)
					}
					failed.Add(1)
				} else {
					if err := s.songs.UpdateFingerprint(ctx, item.ID, fp, dur, attemptedAt); err != nil {
						slog.Warn("fingerprint save failed", "id", item.ID, "err", err)
						failed.Add(1)
					} else {
						computed.Add(1)
					}
				}
				s.mu.Lock()
				s.progress.Computed = computed.Load()
				s.progress.Failed = failed.Load()
				s.mu.Unlock()
			}
		}()
	}

loop:
	for _, item := range items {
		select {
		case <-ctx.Done():
			break loop
		case ch <- item:
		}
	}
	close(ch)
	wg.Wait()

	slog.Info("fingerprint computation done",
		"computed", computed.Load(), "failed", failed.Load(), "total", len(items),
		"workers", workers, "cancelled", ctx.Err() != nil)
}
