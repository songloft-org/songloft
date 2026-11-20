package services

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"songloft/internal/mediaexec"
	"strings"
	"time"
)

// ErrSeekStreamUnavailable 表示 seek 流无法开始（ffmpeg 未配置 / 启动失败 / 并发已满，
// 或进程在吐出任何音频字节之前就退出）。调用方拿到此错误时 **尚未** 向 w 写入任何字节，
// 可安全降级为「从头完整提供该文件」。
var ErrSeekStreamUnavailable = errors.New("seek stream unavailable")

// ErrSeekStreamAborted 表示已经写出部分字节，但驱动本次转码的 ctx 被 playActivity.Activate
// 提前取消（用户切歌，让 ffmpeg 让位省 CPU），而 connCtx（真正的 HTTP 请求/连接）仍然存活——
// 客户端可能仍在读取这条响应。此时绝不能让响应看起来"优雅结束"：ffmpeg 被杀导致音频被截断，
// 若放任 chunked 响应正常收尾，客户端（如 just_audio 的 LockCachingAudioSource）会把这段
// 截断内容当作下载成功永久缓存，且后续播放也不会再触发重新转码
// （songloft-org/songloft-player-flutter#35）。调用方收到此错误必须强制断开底层连接（如
// http.Hijacker + Close），而不能优雅返回。
var ErrSeekStreamAborted = errors.New("seek stream aborted by activity cancellation")

// seekStreamMaxConcurrent 同时存活的 seek / 均衡 / 变速 / 普通转码流上限。
//
// 这些流不占用 transcodeSem（见 StreamSeekedMP3 注释），所以必须自带闸门：seek 流由「用户每按一次
// 暂停/续播」触发，频率比电台转码高一个数量级；均衡流的存活时长取决于客户端读得多快（音箱贪婪缓冲
// 时几秒就流完，边播边读则接近整首）；分组播放还会给组内每台音箱各开一个。
// 满了直接降级而 **不排队**——排队意味着音箱那端干等，比降级更糟。
//
// 上限刻意保守：每条均衡流是一个 libmp3lame 全解码进程，4 条并发在弱 NAS 上已经吃满 CPU。
// 降级路径本身无损：seek 流降级为从头播，均衡流与普通转码流降级为原来的「阻塞整首转码」，
// 不比修复前更差——但那正是 songloft-org/songloft-plugin-miot#61 与 songloft-org/songloft#442
// 要消除的首字节卡顿，所以调用方应把流注册进 playActivity（`CatTranscode`），至少让用户切歌时
// 旧流的 ffmpeg 能被 Activate 掐掉、不白烧 CPU。
// 槽位本身要等 io.Copy 结束（客户端读完或断开）才归还，ctx 取消不会打断它。
//
// **接入 #442（普通 ?format= / ?quality= 转码播放）后没有提高这个上限**：普通转码播放的频率
// 比 seek/均衡高一个数量级，槽位更容易占满，但 #442 的报告环境正是 N5095 这类弱 CPU
// ——放大并发对它是伤害而非帮助，而占满时的降级本身无损。真占满只是回到修复前的行为。
const seekStreamMaxConcurrent = 4

// DefaultPipeBitrateKbps 是 pipe 流未指定 SeekStreamOptions.Bitrate 时的输出码率（kbps）。
//
// 导出是因为 handlers 侧要用同一个值算 Range 应答的 Content-Length：CBR 下
// 总字节 = 码率/8 × 时长（songloft-org/songloft#442）。两处各写一个 320 的字面量，
// 一旦有人只改了其中一处，承诺的长度与实际输出的比例就失配，客户端据此算出的
// 时间位置随之偏移——而症状是「拖动跳到错误位置」，极难归因。
const DefaultPipeBitrateKbps = 320

// seekStreamGrace 是 ffmpeg 硬超时相对于「剩余音频时长」的宽限。
// 音箱只挂着连接却不再读数据时 r.Context() 不会取消，靠这个上限回收孤儿进程。
const seekStreamGrace = 5 * time.Minute

// seekStreamUnknownDurationTimeout 是「剩余时长未知」时的硬超时。
//
// **不能**退化成裸的 seekStreamGrace：对整首流而言 5 分钟不是宽限而是硬上限，客户端边播边读时
// 恰好只送出 5 分钟音频就被 CommandContext 杀掉，表现为「播到第 5 分钟突然结束」，而响应正常闭合、
// 客户端以为播完了。`songs.duration == 0` 是常态而非理论情况（远程/插件歌曲元数据未刷新、
// 本地文件 tag 与 ffprobe 都拿不到时长），叠上均衡流「每次播放都走这条路」就会真踩到。
// 取一个足够覆盖有声书的上限；真正的兜底是 r.Context()（客户端断开即取消）。
const seekStreamUnknownDurationTimeout = 3 * time.Hour

var seekStreamSem = make(chan struct{}, seekStreamMaxConcurrent)

// SeekStreamOptions seek 流参数。
type SeekStreamOptions struct {
	// SourcePath 本机已存在的音频文件：原始文件、转码产物或 CUE 提取产物。
	// 必须是「曲内偏移 0 对应该文件开头」的文件——CUE 轨要先提取再 seek，
	// 否则两个 -ss 叠加会跳到整轨镜像的绝对位置。
	SourcePath string
	// StartSecond 起播秒数。必须 > 0，除非 Normalize 为 true（见下）。
	StartSecond float64
	// RemainingSecond 预计剩余音频时长（秒），用于设置 ffmpeg 硬超时；<= 0 表示未知（用固定兜底）。
	RemainingSecond float64
	// Normalize 边转边发音量均衡（EBU R128）。为 true 时强制重编码并插入 normalizeLoudnessFilter。
	//
	// 借用本函数是因为需要的正是同一套「pipe + Peek(1) 确认有输出才提交响应」骨架：
	// 均衡产物还没落盘时，整首 loudnorm 会把设备的首个 play 请求阻塞 20+ 秒
	// （songloft-org/songloft-plugin-miot#61）。此时 StartSecond 允许为 0（纯均衡、不 seek），
	// 也允许 > 0（续播 + 均衡，一条 ffmpeg 同时做 -ss 与 -af）。
	Normalize bool
	// Speed 播放倍速，0 或 1.0 表示不变速。取值需落在 atempo 单滤镜原生支持的 [0.5, 2.0]
	// 区间内（调用方已夹紧，此处不再二次校验）——单个 atempo 滤镜只支持这个范围，
	// 超出需要链式拼接多个 atempo，目前不支持。
	// 与 StartSecond=0 场景一样，Speed!=1.0 本身也应触发「必须走实时流」，
	// 因为 atempo 需要重编码，静态文件的 http.ServeFile 快路径做不到变速。
	Speed float64
	// Bitrate 目标码率（kbps），0 表示沿用本函数默认的 320k CBR。
	//
	// 为「普通转码播放也边转边发」而加（songloft-org/songloft#442）：`?quality=128/192/320`
	// 会被上游翻成 bitrate，而修复前本函数固定 320k，冒充不了其他码率，于是 quality 请求
	// 只能走阻塞整首转码。本字段让这条 pipe 能如实表达 quality。
	//
	// 值域与 ParseBitrate 一致（128/192/320）；本函数不再二次校验，非法值由调用方拦住。
	// 注意 Bitrate > 0 时**不能**吃下面的 copy 快路径：源是 mp3 也必须重编码，
	// 否则 copy 出来的是源码率，与请求的 quality 不符（客户端拿到的码率是假的）。
	Bitrate int
	// ForceTranscode 表示调用方已判定「这一次必须转码」，即使 StartSecond=0、不均衡、不变速、
	// 未指定码率也要出流（songloft-org/songloft#442 的 `?format=mp3` 场景）。
	//
	// 必须显式传而不能靠「源扩展名不是 mp3」推断：`NeedsTranscodeForServe` 的伪 mp3 分支
	// （扩展名 .mp3 但内容是 WebM/Opus，songloft-org/songloft#300）恰恰是扩展名为 mp3 却必须
	// 转码的一类，靠扩展名推断会把它判成「无需实时流」而被下面的守卫拒掉。
	//
	// 同理它**必须**一并禁掉 copy 快路径：对伪 mp3 走 `-c:a copy` 会让 mp3 muxer 拒收 Opus 流，
	// ffmpeg 零输出 → Peek(1) 触发降级 → 白起一个 ffmpeg 且延迟一点没改善（功能上安全，但
	// 修复的目的就落空了）。
	ForceTranscode bool
}

// speedActive 判断 speed 是否需要实际生效（区分「未指定」与「显式传 1.0」，两者效果一致）。
func speedActive(speed float64) bool {
	return speed != 0 && speed != 1.0
}

// seekStreamTimeout 根据剩余时长与倍速算 ffmpeg 硬超时。
//
// RemainingSecond 是歌曲域（曲内）剩余秒数，但变速后 ffmpeg 实际要写出的音频时长是
// RemainingSecond / Speed：2x 下一首剩 200 秒的歌只需 100 秒墙钟就能转完写完；0.5x 反而
// 要 400 秒。不按 Speed 换算的话，0.5x 播放会在音频还没转完时就被硬超时掐断（截断收尾，
// 参考 ErrSeekStreamAborted 的说明），表现为「变速播放到一半突然断掉」。
// 剩余时长未知（<= 0）时退化成 seekStreamUnknownDurationTimeout，不用倍速换算。
func seekStreamTimeout(remainingSecond, speed float64) time.Duration {
	if remainingSecond <= 0 {
		return seekStreamUnknownDurationTimeout
	}
	wallClockRemaining := remainingSecond
	if speedActive(speed) {
		wallClockRemaining = remainingSecond / speed
	}
	return seekStreamGrace + time.Duration(wallClockRemaining*float64(time.Second))
}

// StreamSeekedMP3 阻塞运行 ffmpeg，把 SourcePath 从 StartSecond 起的音频以 MP3 流写入 w，
// 直到 ffmpeg 结束或 ctx 取消。
//
// 用于不支持 HTTP Range seek 的推流客户端：小爱音箱经 player_play_url 收到 URL 后只会从头拉，
// 想「从第 N 秒续播」只能让服务端产出一条以第 N 秒为开头的流（songloft-plugin-miot#60）。
//
// opts.Normalize 时另兼「边转边发音量均衡」，此时 StartSecond 允许为 0——两件事需要的都是
// 同一套 pipe + Peek(1) 降级骨架，见 SeekStreamOptions.Normalize。
//
// opts.Speed 时同理兼「边转边发变速播放」（atempo 滤镜），StartSecond 同样允许为 0（从头即变速）；
// 可与 Normalize/StartSecond 任意组合，一条 ffmpeg 用逗号拼接的 -af 同时处理。
//
// opts.ForceTranscode / opts.Bitrate 时兼「普通转码播放（?format=mp3 / ?quality=）也边转边发」
// （songloft-org/songloft#442）：那条路径原先只有阻塞整首转码，首字节 = 整首转码墙钟时间，
// 在弱 CPU（如 N5095）上无损转 mp3 要等 5 秒以上。与 Normalize 同理复用这套骨架，
// StartSecond 允许为 0，也可与 seek/变速任意组合。
//
// 与 runFFmpeg 不同，本函数 **不占用** c.transcodeSem：进程会存活整首歌的剩余时长，
// 持串行信号量会长时间饿死其他有限文件的转码。并发由 seekStreamSem 单独限。
//
// ctx 与 connCtx 是两条独立的取消信号，必须分开传：
//   - ctx 驱动 ffmpeg 生命周期，可能被 playActivity.Activate 提前取消（用户切歌时为省 CPU
//     主动掐掉），这不代表客户端已经断开。
//   - connCtx 是真正的 HTTP 请求/连接 ctx（如 r.Context()），只在客户端真的断开 TCP 连接
//     时才会 Done。
//
// 二者不区分会导致 songloft-org/songloft-player-flutter#35：Activate 掐掉 ffmpeg 后，若把这当作
// "正常结束"，chunked 响应会优雅收尾，客户端（仍连着、仍在读）拿到一段语法完整但内容被
// 截断的 MP3，误判为下载成功并永久缓存，此后再也不会重新请求转码。
//
// 返回值语义：
//   - nil：正常结束，或 connCtx 也已取消（客户端真的断开，无人在听，无害）。
//   - ErrSeekStreamUnavailable：在写出任何字节前失败，调用方应降级为从头提供文件（此时 w 未被写入）。
//   - ErrSeekStreamAborted：已写出部分字节，ctx 被提前取消但 connCtx 仍存活——调用方必须强制
//     断开底层连接（如 http.Hijacker + Close），不能优雅返回。
//   - 其他 error：已写出部分字节后中途失败，无法再降级。
func (c *CacheService) StreamSeekedMP3(ctx, connCtx context.Context, w io.Writer, opts SeekStreamOptions) error {
	// 没有任何「必须走实时流」的理由就拒绝：调用方应改用 http.ServeFile（支持 Range、更快）。
	// Bitrate > 0 单独作为一个充分理由，这样调用方只想改码率时忘了 ForceTranscode 也不会被误拒。
	if opts.StartSecond <= 0 && !opts.Normalize && !speedActive(opts.Speed) &&
		!opts.ForceTranscode && opts.Bitrate == 0 {
		return fmt.Errorf("%w: nothing to do (no seek/normalize/speed/transcode)", ErrSeekStreamUnavailable)
	}
	ffmpegPath := c.getFFmpegPath()
	if ffmpegPath == "" {
		return fmt.Errorf("%w: ffmpeg not configured", ErrSeekStreamUnavailable)
	}

	select {
	case seekStreamSem <- struct{}{}:
		defer func() { <-seekStreamSem }()
	default:
		return fmt.Errorf("%w: %d concurrent seek streams already running", ErrSeekStreamUnavailable, seekStreamMaxConcurrent)
	}

	// 源已是 MP3 时无损 copy：input seek + copy 几乎零 CPU（实测 320k 整首 0.14s）。
	//
	// 需要重编码时用 CBR 而非项目别处的 VBR（-q:a 0）：pipe 不可 seek → 无法回写 Xing 帧
	// （见下面 -write_xing 0），拿到流的客户端只能按「首帧码率 × 字节数」估算时长。VBR 下这个
	// 估算会偏差 7%+（实测 105.4s 的流被估成 97.6s），CBR 下几乎精确。音箱正是这类只能靠估算的
	// 客户端，估短了可能提前判定播完。320k CBR 对无损源（flac/ape）也不构成可闻损失。
	//
	// 已知残留：源本身是 VBR MP3（含 force_mp3/normalize 转码产物，那条路径用 -q:a 0）时走 copy，
	// 这个时长估算同样会偏几个百分点。只影响客户端自己显示的总时长——插件的进度与自动切歌都由
	// 它本地计时驱动（见 PlaylistManager.playCurrent）——故不为此放弃零开销的 copy 快路径。
	// Normalize 必须重编码（loudnorm 是滤镜，copy 下滤镜不生效），Speed!=1.0 同理（atempo 是
	// 时域滤镜，直接影响采样点，copy 下不生效），Bitrate > 0 同理（copy 保留的是源码率，
	// 冒充不了请求的 quality），三者都不吃上面的 copy 快路径。
	//
	// Bitrate > 0 时按请求码率出 CBR（songloft-org/songloft#442 的 `?quality=` 场景）；
	// 为 0 时保持 320k —— 对无损源不构成可闻损失，也让上面那段「CBR 才能让音箱估准时长」
	// 的理由继续成立（各档 quality 同样是 CBR，估算精度不受影响）。
	encoder := "libmp3lame"
	var qualityArgs []string
	if !opts.Normalize && !speedActive(opts.Speed) && !opts.ForceTranscode && opts.Bitrate == 0 &&
		NormalizeFormat(filepath.Ext(opts.SourcePath)) == "mp3" {
		encoder = "copy"
	} else if opts.Bitrate > 0 {
		qualityArgs = []string{"-b:a", fmt.Sprintf("%dk", opts.Bitrate)}
	} else {
		qualityArgs = []string{"-b:a", fmt.Sprintf("%dk", DefaultPipeBitrateKbps)}
	}

	args := []string{"-hide_banner", "-loglevel", "error"}
	// -ss 放在 -i 之前用 input seek（O(1)），放后面会解码丢弃前 N 秒。
	// StartSecond 为 0（纯均衡/纯变速流）时不加 -ss：ffmpeg 认 -ss 0 但没必要，且少一个参数少一处踩坑面。
	if opts.StartSecond > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", opts.StartSecond))
	}
	args = append(args, "-i", opts.SourcePath)
	// -map 0:a:0 必需：mp3 muxer 只接受单条音频流，双音轨源（.mka，songloft-org/songloft#298）
	// 不显式选轨会让 ffmpeg 直接报错退出，静默降级回「从头播」。
	args = append(args, "-map", "0:a:0", "-vn", "-codec:a", encoder)
	// 滤镜链：atempo 放前面先变速，loudnorm 再对变速后的信号做响度分析，两者同开时结果符合直觉。
	// atempo 只支持单滤镜 [0.5, 2.0]，调用方已夹紧到该区间，此处不做二次校验/链式拼接。
	var filters []string
	if speedActive(opts.Speed) {
		filters = append(filters, fmt.Sprintf("atempo=%.3f", opts.Speed))
	}
	if opts.Normalize {
		filters = append(filters, c.normalizeLoudnessFilter())
	}
	if len(filters) > 0 {
		args = append(args, "-af", strings.Join(filters, ","))
	}
	args = append(args, qualityArgs...)
	// -write_xing 0 必需：pipe 不可 seek，mp3 muxer 无法回写 Xing 帧的真实时长，
	// 留下的占位值会让音箱显示错误时长甚至提前判定播完。
	args = append(args, "-write_xing", "0", "-f", "mp3", "pipe:1")

	// 音箱只挂着连接不再读数据时 r.Context() 不取消，用硬超时回收孤儿 ffmpeg。
	timeout := seekStreamTimeout(opts.RemainingSecond, opts.Speed)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := mediaexec.FFmpeg(runCtx, ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%w: stdout pipe: %v", ErrSeekStreamUnavailable, err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &capWriter{w: &stderr, remaining: 4096}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start: %v", ErrSeekStreamUnavailable, err)
	}

	reader := bufio.NewReaderSize(stdout, 64*1024)
	// 预读第一字节：只有确认 ffmpeg 真的产出了音频才提交响应。若在任何字节之前就 EOF/失败
	// （seek 越过文件尾、源不可解码、选轨失败等），返回 ErrSeekStreamUnavailable 让调用方
	// 降级为从头提供文件——此时尚未写出任何字节。
	first, _ := reader.Peek(1)
	if len(first) == 0 {
		_ = cmd.Wait()
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "no output"
		}
		return fmt.Errorf("%w: %s", ErrSeekStreamUnavailable, msg)
	}

	// 已确认有音频输出：从此刻起本函数拥有响应，流式转发直到结束或断开。
	_, copyErr := io.Copy(&flushingWriter{w: w}, reader)
	// copyErr 时必须先 cancel 再 Wait：ffmpeg 此刻正阻塞在写满的 stdout 管道上，而 cmd.Wait()
	// 要等进程退出才关管道 —— 两边互等，只能靠 runCtx 到点解开，把本 goroutine 连同一个
	// seekStreamSem 槽位挂到硬超时。ctx 已取消的场景由 CommandContext 自己解决。
	if copyErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		if connCtx == nil || connCtx.Err() != nil {
			// connCtx 也已取消：客户端真的断开了 TCP 连接，无人在听，无害。
			return nil
		}
		// ctx 被 playActivity.Activate 提前取消，但 connCtx（真实连接）仍存活——客户端可能仍在
		// 读这条响应。绝不能让它看起来"优雅结束"，否则截断的音频会被误判为下载完整并永久缓存。
		return ErrSeekStreamAborted
	}
	if copyErr != nil {
		return copyErr
	}
	// 硬超时到点：音频被截断了，但字节已经写出去、响应也正常闭合，客户端会以为播完了。
	// 唯一能做的是留下可归因的日志——正常结束与被杀掉在 waitErr 里长得一样。
	if runCtx.Err() != nil {
		slog.Error("seek stream truncated by hard timeout",
			"src", opts.SourcePath, "seek", opts.StartSecond, "normalize", opts.Normalize,
			"speed", opts.Speed, "remainingSecond", opts.RemainingSecond, "timeout", timeout)
		return nil
	}
	if waitErr != nil {
		slog.Warn("seek stream ffmpeg exited with error",
			"src", opts.SourcePath, "seek", opts.StartSecond,
			"stderr", strings.TrimSpace(stderr.String()), "error", waitErr)
	}
	return nil
}
