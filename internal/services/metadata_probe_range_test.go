package services

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestFFProbeHeadersRangeInjection(t *testing.T) {
	t.Run("nil headers with injection", func(t *testing.T) {
		got := ffprobeHeaders(nil, true)
		if got["Range"] != "bytes=0-" {
			t.Fatalf("Range = %q, want bytes=0-", got["Range"])
		}
	})

	t.Run("inject without mutating caller map", func(t *testing.T) {
		caller := map[string]string{"Authorization": "Bearer x"}
		got := ffprobeHeaders(caller, true)
		if got["Range"] != "bytes=0-" {
			t.Fatalf("Range = %q, want bytes=0-", got["Range"])
		}
		if got["Authorization"] != "Bearer x" {
			t.Fatalf("Authorization = %q, want Bearer x", got["Authorization"])
		}
		if len(caller) != 1 {
			t.Fatalf("caller map mutated: %v", caller)
		}
		if _, ok := caller["Range"]; ok {
			t.Fatal("caller map should not gain a Range key")
		}
	})

	t.Run("caller provided Range is preserved", func(t *testing.T) {
		caller := map[string]string{"Range": "bytes=100-"}
		got := ffprobeHeaders(caller, true)
		if got["Range"] != "bytes=100-" {
			t.Fatalf("Range = %q, want caller's bytes=100-", got["Range"])
		}
	})

	t.Run("no injection when disabled", func(t *testing.T) {
		caller := map[string]string{"Authorization": "Bearer x"}
		got := ffprobeHeaders(caller, false)
		if got["Authorization"] != "Bearer x" {
			t.Fatalf("Authorization = %q", got["Authorization"])
		}
		if _, ok := got["Range"]; ok {
			t.Fatal("Range should not be injected when disabled")
		}
	})
}

// writeFakeFFProbe 写一个可执行的假 ffprobe：遍历参数，遇到含 needle 的参数
// 即以 exitCode 退出；否则向 stdout 输出一份合法的 ffprobe JSON。
func writeFakeFFProbe(t *testing.T, needle string, exitCode int, counterPath string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake ffprobe script test requires a unix shell")
	}
	script := `#!/bin/sh
printf x >> ` + counterPath + `
for arg in "$@"; do
  case "$arg" in
    *` + needle + `*) exit ` + strconv.Itoa(exitCode) + ` ;;
  esac
done
cat <<'JSON'
{"format":{"duration":"3.5","format_name":"mp3","bit_rate":"128000"},"streams":[{"codec_type":"audio","sample_rate":"44100"}]}
JSON
`
	path := filepath.Join(t.TempDir(), "fake-ffprobe.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}
	return path
}

// writeAlwaysFailFFProbe 写一个无条件失败（exit 1）的假 ffprobe。
func writeAlwaysFailFFProbe(t *testing.T, counterPath string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake ffprobe script test requires a unix shell")
	}
	script := `#!/bin/sh
printf x >> ` + counterPath + `
exit 1
`
	path := filepath.Join(t.TempDir(), "fake-ffprobe-fail.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}
	return path
}

// countProbeInvocations 从计数文件读取探测执行次数（每次执行追加一个 x）。
func countProbeInvocations(t *testing.T, counterPath string) int {
	t.Helper()
	data, err := os.ReadFile(counterPath)
	if err != nil {
		return 0
	}
	return len(string(data))
}

// 注入的 Range 头导致 ffprobe 失败时，自动去掉注入头重试并成功。
func TestProbeWithFFProbeRetriesWithoutRange(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "probe-count")
	scriptPath := writeFakeFFProbe(t, `"bytes=0-"`, 3, counter)

	m := NewMetadataExtractor(&MetadataConfig{FFProbePath: scriptPath})

	var result RemoteProbeResult
	if err := m.probeWithFFProbe(context.Background(), "http://example.invalid/a.mp3", nil, &result); err != nil {
		t.Fatalf("probeWithFFProbe() error = %v", err)
	}

	if result.Duration != 3.5 {
		t.Fatalf("Duration = %v, want 3.5", result.Duration)
	}
	if result.Format != "mp3" {
		t.Fatalf("Format = %q, want mp3", result.Format)
	}
	if result.BitRate != 128 {
		t.Fatalf("BitRate = %v, want 128", result.BitRate)
	}
	if result.SampleRate != 44100 {
		t.Fatalf("SampleRate = %v, want 44100", result.SampleRate)
	}
	if got := countProbeInvocations(t, counter); got != 2 {
		t.Fatalf("ffprobe invocations = %d, want 2 (fail with Range, retry without)", got)
	}
}

// 调用方自带 Range 头时失败不重试（失败与注入无关）。
func TestProbeWithFFProbeNoRetryWhenCallerProvidesRange(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "probe-count")
	scriptPath := writeAlwaysFailFFProbe(t, counter)

	m := NewMetadataExtractor(&MetadataConfig{FFProbePath: scriptPath})

	var result RemoteProbeResult
	err := m.probeWithFFProbe(context.Background(), "http://example.invalid/a.mp3",
		map[string]string{"Range": "bytes=0-100"}, &result)
	if err == nil {
		t.Fatal("expected error when caller Range probe fails")
	}
	if got := countProbeInvocations(t, counter); got != 1 {
		t.Fatalf("ffprobe invocations = %d, want 1 (no retry when caller provides Range)", got)
	}
	if result.Duration != 0 || result.Format != "" {
		t.Fatalf("result should stay untouched on failure, got %+v", result)
	}
}