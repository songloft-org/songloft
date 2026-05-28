package source

import (
	"sync"
	"time"
)

// OutcomeResult 一次音源 Fetch 的结果分类。
type OutcomeResult string

const (
	OutcomeSuccess              OutcomeResult = "success"
	OutcomeNetworkFail          OutcomeResult = "network_fail"           // HTTP 错误、超时、读流中断
	OutcomeProbeFail            OutcomeResult = "probe_fail"             // ffprobe / tag.ReadFrom 失败
	OutcomeValidationFail       OutcomeResult = "validation_fail"        // 校验未通过(过短、码率过低等)
	OutcomePluginInvocationFail OutcomeResult = "plugin_invocation_fail" // 插件 music/url 返回错误或非 200
)

// Outcome 一次 Fetch 的结果记录。
type Outcome struct {
	PluginEntryPath string        `json:"plugin_entry_path"`
	Result          OutcomeResult `json:"result"`
	Reason          string        `json:"reason,omitempty"`
	Latency         time.Duration `json:"latency_ns" swaggertype:"primitive,integer"` // 纳秒;swagger 显式声明避免 time.Duration 解析失败
	SizeBytes       int64         `json:"size_bytes,omitempty"`
	Timestamp       time.Time     `json:"timestamp"`
}

// HealthClass 插件健康度三级分类(green / yellow / red)。
// 用于 Resolver 排序时降权,或直接过滤 red 插件。
type HealthClass string

const (
	HealthGreen  HealthClass = "green"  // 成功率 ≥ green_threshold 且 samples ≥ min_samples
	HealthYellow HealthClass = "yellow" // 介于 green 与 red 之间;或 samples 不足
	HealthRed    HealthClass = "red"    // 成功率 < red_threshold 且 samples ≥ min_samples
)

// MetricsOpts 健康度判定参数。所有阈值可由 music_cache_config.source_metrics 配置。
type MetricsOpts struct {
	WindowSize     int     // 每插件保留的 outcome 数(环形 buffer 大小);默认 200
	GreenThreshold float64 // ≥ 此值且样本足够 → green;默认 0.8
	RedThreshold   float64 // < 此值且样本足够 → red;默认 0.4
	MinSamples     int     // 样本数低于此值时不归类为 red(避免冷启动误杀);默认 10
}

// DefaultMetricsOpts 返回默认参数。
func DefaultMetricsOpts() MetricsOpts {
	return MetricsOpts{
		WindowSize:     200,
		GreenThreshold: 0.8,
		RedThreshold:   0.4,
		MinSamples:     10,
	}
}

// PluginHealthSnapshot 给 admin API 用的可读快照。
type PluginHealthSnapshot struct {
	PluginEntryPath string      `json:"plugin_entry_path"`
	Class           HealthClass `json:"class"`
	SuccessRate     float64     `json:"success_rate"`
	Samples         int         `json:"samples"`
	LastFailures    []Outcome   `json:"last_failures,omitempty"` // 最近 N 次失败,便于排查
}

// ringBuffer 内部环形 buffer,固定容量,FIFO 覆盖最旧。
type ringBuffer struct {
	items []Outcome
	head  int // 下一次写入位置
	count int // 当前实际元素数(≤ cap)
}

func newRingBuffer(cap int) *ringBuffer {
	if cap <= 0 {
		cap = 1
	}
	return &ringBuffer{items: make([]Outcome, cap)}
}

func (r *ringBuffer) push(o Outcome) {
	r.items[r.head] = o
	r.head = (r.head + 1) % len(r.items)
	if r.count < len(r.items) {
		r.count++
	}
}

// snapshot 返回当前所有元素的副本,从最早到最新。
func (r *ringBuffer) snapshot() []Outcome {
	if r.count == 0 {
		return nil
	}
	out := make([]Outcome, 0, r.count)
	start := r.head - r.count
	if start < 0 {
		start += len(r.items)
	}
	for i := 0; i < r.count; i++ {
		out = append(out, r.items[(start+i)%len(r.items)])
	}
	return out
}

// SourceMetrics 每个插件维护一个滚动窗口的 outcome 列表。
// 纯内存,进程重启清零;不持久化(写放大成本太高,且健康度本身是短期信号)。
type SourceMetrics struct {
	mu      sync.RWMutex
	buffers map[string]*ringBuffer
	opts    MetricsOpts
}

// NewSourceMetrics 创建新的指标收集器。opts 决定环形 buffer 大小与健康度阈值。
func NewSourceMetrics(opts MetricsOpts) *SourceMetrics {
	if opts.WindowSize <= 0 {
		opts = DefaultMetricsOpts()
	}
	return &SourceMetrics{
		buffers: make(map[string]*ringBuffer),
		opts:    opts,
	}
}

// Record 上报一次 Fetch 结果。线程安全。
// PluginEntryPath 为空时直接丢弃(纯外链场景没有插件维度)。
func (m *SourceMetrics) Record(o Outcome) {
	if o.PluginEntryPath == "" {
		return
	}
	if o.Timestamp.IsZero() {
		o.Timestamp = time.Now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	buf, ok := m.buffers[o.PluginEntryPath]
	if !ok {
		buf = newRingBuffer(m.opts.WindowSize)
		m.buffers[o.PluginEntryPath] = buf
	}
	buf.push(o)
}

// SuccessRate 返回某插件最近 WindowSize 次的成功率与样本数。
// samples == 0 时 rate 返回 0(调用方应另行判断是否为冷启动)。
func (m *SourceMetrics) SuccessRate(pluginEntryPath string) (rate float64, samples int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	buf, ok := m.buffers[pluginEntryPath]
	if !ok || buf.count == 0 {
		return 0, 0
	}
	success := 0
	items := buf.snapshot()
	for _, o := range items {
		if o.Result == OutcomeSuccess {
			success++
		}
	}
	return float64(success) / float64(len(items)), len(items)
}

// Class 返回插件健康度分类。冷启动(samples < MinSamples)统一返回 yellow,
// 避免新插件因前几次随机失败被立刻打成 red。
func (m *SourceMetrics) Class(pluginEntryPath string) HealthClass {
	rate, samples := m.SuccessRate(pluginEntryPath)
	if samples < m.opts.MinSamples {
		return HealthYellow
	}
	if rate >= m.opts.GreenThreshold {
		return HealthGreen
	}
	if rate < m.opts.RedThreshold {
		return HealthRed
	}
	return HealthYellow
}

// WeightedScore 给 Resolver 排序用的权重系数:
//
//	score = baseScore * (0.3 + 0.7 * successRate)
//
// 样本不足时取中性值 0.8。这样高质量插件优势明显,但不会让冷启动插件被零优先级。
func (m *SourceMetrics) WeightedScore(pluginEntryPath string) float64 {
	rate, samples := m.SuccessRate(pluginEntryPath)
	if samples < m.opts.MinSamples {
		return 0.3 + 0.7*0.8
	}
	return 0.3 + 0.7*rate
}

// Snapshot 返回所有插件的健康度快照,用于 admin API (`GET /api/v1/plugins/health`)。
// maxFailures 控制每个插件最多返回多少条最近失败记录(0 表示不返回失败列表)。
func (m *SourceMetrics) Snapshot(maxFailures int) []PluginHealthSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PluginHealthSnapshot, 0, len(m.buffers))
	for plugin, buf := range m.buffers {
		items := buf.snapshot()
		success := 0
		var failures []Outcome
		for i := len(items) - 1; i >= 0; i-- {
			if items[i].Result == OutcomeSuccess {
				success++
			} else if maxFailures > 0 && len(failures) < maxFailures {
				failures = append(failures, items[i])
			}
		}
		rate := 0.0
		if len(items) > 0 {
			rate = float64(success) / float64(len(items))
		}
		class := HealthYellow
		if len(items) >= m.opts.MinSamples {
			switch {
			case rate >= m.opts.GreenThreshold:
				class = HealthGreen
			case rate < m.opts.RedThreshold:
				class = HealthRed
			}
		}
		out = append(out, PluginHealthSnapshot{
			PluginEntryPath: plugin,
			Class:           class,
			SuccessRate:     rate,
			Samples:         len(items),
			LastFailures:    failures,
		})
	}
	return out
}
