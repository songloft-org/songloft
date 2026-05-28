// Package source 实现网络歌曲音源的探测、校验、切换与健康度反馈。
//
// 包含五个核心模块:
//   - validator: 纯函数,校验下载文件的完整性(时长/码率/格式)
//   - metrics:   插件维度的成功率滚动窗口,用于排序与降权
//   - fetcher:   通过 (plugin_entry_path, source_data) 拉取文件 + 校验 + 上报 outcome
//   - resolver:  跨插件 fan-out 搜索,寻找备选音源候选
//   - orchestrator: 编排 fetcher + resolver + metrics,提供两种 Fetch 模式
package source

// ValidationOpts 文件完整性校验的可调参数。所有阈值都可通过 source_validation 配置 key 调整。
type ValidationOpts struct {
	// Enabled 总开关。false 时 Validate 直接返回 Valid,不做任何检查(灰度降级用)。
	Enabled bool `json:"enabled"`
	// MinDuration 实测时长绝对下限(秒)。expected==0 时仅靠此项兜底,过滤"几秒的截断文件"。
	MinDuration float64 `json:"min_duration"`
	// DurationRatio 实测/预期 时长容忍下限。例如 0.85 表示实测 ≥ 预期*0.85 才算合格。
	DurationRatio float64 `json:"duration_ratio"`
	// MaxDurationRatio 实测/预期 时长容忍上限。例如 1.5 表示实测 ≤ 预期*1.5;
	// 用于过滤"插件错位返回了整张专辑"这类情况。
	MaxDurationRatio float64 `json:"max_duration_ratio"`
	// MinBitrate 平均码率下限(kbps)。Size*8/Duration/1000 < 阈值则判失败。
	// 用于过滤"几秒静音 + padding"这种文件大小看起来正常但实际无效的样本。
	MinBitrate int `json:"min_bitrate"`
}

// DefaultValidationOpts 返回默认校验参数。运维若不在配置里覆盖则用这套。
func DefaultValidationOpts() ValidationOpts {
	return ValidationOpts{
		Enabled:          true,
		MinDuration:      30,
		DurationRatio:    0.85,
		MaxDurationRatio: 1.5,
		MinBitrate:       8,
	}
}

// ValidationReason 校验失败的原因枚举。用于 metrics 归类 + 错误透传给前端展示。
type ValidationReason string

const (
	ReasonProbeFailed          ValidationReason = "probe_failed"
	ReasonTooShort             ValidationReason = "too_short"
	ReasonDurationMismatchLow  ValidationReason = "duration_mismatch_low"
	ReasonDurationMismatchHigh ValidationReason = "duration_mismatch_high"
	ReasonBitrateTooLow        ValidationReason = "bitrate_too_low"
)

// ValidationResult 校验结果。Valid==false 时 Reason/Expected/Actual 提供失败细节。
type ValidationResult struct {
	Valid    bool
	Reason   ValidationReason
	Expected float64 // 预期时长(秒),来自 song.Duration 或 0
	Actual   float64 // 实测时长(秒)
}

// AudioInfoLike 抽象出 fetcher / metadata 都能提供的最小探测结果接口。
// 解耦 validator 与 metadata 包,避免循环依赖。
type AudioInfoLike interface {
	GetDuration() float64
	GetSize() int64
}

// SimpleAudioInfo 一个最小实现,供调用方在不想引 metadata 包的场景下使用。
type SimpleAudioInfo struct {
	Duration float64
	Size     int64
}

func (s SimpleAudioInfo) GetDuration() float64 { return s.Duration }
func (s SimpleAudioInfo) GetSize() int64       { return s.Size }

// Validate 判定下载文件是否完整。纯函数,无 IO,便于单测。
//
// 判定顺序(任一不通过即 Invalid):
//  1. opts.Enabled == false → Valid(灰度降级)
//  2. info == nil 或 Duration == 0 → probe_failed(无法判定即视为可疑)
//  3. Duration < MinDuration → too_short(绝对下限)
//  4. expected > 0 时:
//     - Duration < expected * DurationRatio → duration_mismatch_low
//     - Duration > expected * MaxDurationRatio → duration_mismatch_high
//  5. 平均码率 Size*8/Duration/1000 < MinBitrate → bitrate_too_low
//
// expected 来自 song.Duration(搜索阶段插件返回的元数据)。它可能不准
// (个别插件给的是估算值),所以 DurationRatio 取宽松值(默认 0.85)。
func Validate(info AudioInfoLike, expectedDuration float64, opts ValidationOpts) ValidationResult {
	if !opts.Enabled {
		return ValidationResult{Valid: true}
	}
	if info == nil {
		return ValidationResult{Valid: false, Reason: ReasonProbeFailed, Expected: expectedDuration}
	}
	actual := info.GetDuration()
	if actual <= 0 {
		return ValidationResult{Valid: false, Reason: ReasonProbeFailed, Expected: expectedDuration, Actual: actual}
	}
	if actual < opts.MinDuration {
		return ValidationResult{Valid: false, Reason: ReasonTooShort, Expected: expectedDuration, Actual: actual}
	}
	if expectedDuration > 0 {
		if opts.DurationRatio > 0 && actual < expectedDuration*opts.DurationRatio {
			return ValidationResult{Valid: false, Reason: ReasonDurationMismatchLow, Expected: expectedDuration, Actual: actual}
		}
		if opts.MaxDurationRatio > 0 && actual > expectedDuration*opts.MaxDurationRatio {
			return ValidationResult{Valid: false, Reason: ReasonDurationMismatchHigh, Expected: expectedDuration, Actual: actual}
		}
	}
	if opts.MinBitrate > 0 {
		// kbps = size_bytes * 8 / duration_sec / 1000
		size := info.GetSize()
		if size > 0 {
			avgKbps := float64(size) * 8 / actual / 1000
			if avgKbps < float64(opts.MinBitrate) {
				return ValidationResult{Valid: false, Reason: ReasonBitrateTooLow, Expected: expectedDuration, Actual: actual}
			}
		}
	}
	return ValidationResult{Valid: true, Expected: expectedDuration, Actual: actual}
}
