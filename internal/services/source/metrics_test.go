package source

import (
	"sync"
	"testing"
)

func TestSourceMetrics_BasicRateAndClass(t *testing.T) {
	opts := MetricsOpts{WindowSize: 10, GreenThreshold: 0.8, RedThreshold: 0.4, MinSamples: 5}
	m := NewSourceMetrics(opts)

	// 冷启动:samples 不足 → yellow,不看成功率
	for i := 0; i < 3; i++ {
		m.Record(Outcome{PluginEntryPath: "p1", Result: OutcomeSuccess})
	}
	if c := m.Class("p1"); c != HealthYellow {
		t.Errorf("cold start should be yellow, got %s", c)
	}

	// 补齐到 5 个,全部 success → green
	for i := 0; i < 2; i++ {
		m.Record(Outcome{PluginEntryPath: "p1", Result: OutcomeSuccess})
	}
	if rate, samples := m.SuccessRate("p1"); rate != 1 || samples != 5 {
		t.Errorf("rate=%v samples=%d, want 1/5", rate, samples)
	}
	if c := m.Class("p1"); c != HealthGreen {
		t.Errorf("all success should be green, got %s", c)
	}

	// 失败拖到 red:10 次中 8 次失败 → 0.2 < 0.4
	for i := 0; i < 8; i++ {
		m.Record(Outcome{PluginEntryPath: "p2", Result: OutcomeValidationFail})
	}
	for i := 0; i < 2; i++ {
		m.Record(Outcome{PluginEntryPath: "p2", Result: OutcomeSuccess})
	}
	if c := m.Class("p2"); c != HealthRed {
		t.Errorf("8/10 fail should be red, got %s", c)
	}
}

func TestSourceMetrics_RingBufferOverwrite(t *testing.T) {
	m := NewSourceMetrics(MetricsOpts{WindowSize: 3, GreenThreshold: 0.8, RedThreshold: 0.4, MinSamples: 1})
	// 写入 5 条,只保留最近 3 条
	m.Record(Outcome{PluginEntryPath: "p", Result: OutcomeValidationFail})
	m.Record(Outcome{PluginEntryPath: "p", Result: OutcomeValidationFail})
	m.Record(Outcome{PluginEntryPath: "p", Result: OutcomeSuccess})
	m.Record(Outcome{PluginEntryPath: "p", Result: OutcomeSuccess})
	m.Record(Outcome{PluginEntryPath: "p", Result: OutcomeSuccess})
	rate, samples := m.SuccessRate("p")
	if samples != 3 {
		t.Errorf("expected window size 3, got %d", samples)
	}
	if rate != 1 {
		t.Errorf("最近 3 次都 success,rate 应为 1,got %v", rate)
	}
}

func TestSourceMetrics_WeightedScore(t *testing.T) {
	m := NewSourceMetrics(DefaultMetricsOpts())
	// 冷启动 → 中性 0.8 → 0.3 + 0.7*0.8 = 0.86
	if got := m.WeightedScore("unknown"); got < 0.85 || got > 0.87 {
		t.Errorf("cold start weight should be ~0.86, got %v", got)
	}
}

func TestSourceMetrics_ConcurrentRecord(t *testing.T) {
	m := NewSourceMetrics(MetricsOpts{WindowSize: 1000, MinSamples: 1, GreenThreshold: 0.8, RedThreshold: 0.4})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Record(Outcome{PluginEntryPath: "p", Result: OutcomeSuccess})
			}
		}()
	}
	wg.Wait()
	_, samples := m.SuccessRate("p")
	if samples != 1000 {
		t.Errorf("expected 1000 (capped by window size), got %d", samples)
	}
}

func TestSourceMetrics_EmptyPluginIgnored(t *testing.T) {
	m := NewSourceMetrics(DefaultMetricsOpts())
	m.Record(Outcome{PluginEntryPath: "", Result: OutcomeSuccess})
	snap := m.Snapshot(5)
	if len(snap) != 0 {
		t.Errorf("empty plugin should be ignored, got %d entries", len(snap))
	}
}
