package jsruntime

import (
	"testing"
	"time"
)

func TestProcessTimers_ReturnsTrue_WhenTimerFires(t *testing.T) {
	manager := NewJSEnvManager()
	defer manager.SignalShutdown()

	envID := "test-timer-fire"
	pluginID := int64(1)

	// Create environment with a timer that fires immediately
	code := polyfillJS + `
		var fired = false;
		setTimeout(function(){ fired = true; }, 0);
	`
	if err := manager.CreateEnv(envID, code, pluginID); err != nil {
		t.Fatalf("CreateEnv failed: %v", err)
	}
	defer manager.DestroyEnv(envID)

	// Process timers - should return true because timer fires
	time.Sleep(10 * time.Millisecond) // Give timer a chance to be ready
	didFire := manager.ProcessTimers(envID)

	if !didFire {
		t.Error("Expected ProcessTimers to return true when timer fires")
	}

	// Verify the timer actually executed
	result, err := manager.ExecuteJS(envID, "fired", 1000)
	if err != nil {
		t.Fatalf("Failed to check fired variable: %v", err)
	}

	if result.Result != "true" {
		t.Errorf("Expected timer to have fired, got fired=%s", result.Result)
	}
}

func TestProcessTimers_ReturnsFalse_WhenNoTimerFires(t *testing.T) {
	manager := NewJSEnvManager()
	defer manager.SignalShutdown()

	envID := "test-no-timer-fire"
	pluginID := int64(1)

	// Create environment with no timers
	code := polyfillJS
	if err := manager.CreateEnv(envID, code, pluginID); err != nil {
		t.Fatalf("CreateEnv failed: %v", err)
	}
	defer manager.DestroyEnv(envID)

	// Process timers - should return false because no timers exist
	didFire := manager.ProcessTimers(envID)

	if didFire {
		t.Error("Expected ProcessTimers to return false when no timers exist")
	}
}

func TestGetNextTimerDeadline_NoTimers(t *testing.T) {
	manager := NewJSEnvManager()
	defer manager.SignalShutdown()

	envID := "test-next-empty"
	pluginID := int64(1)

	if err := manager.CreateEnv(envID, polyfillJS, pluginID); err != nil {
		t.Fatalf("CreateEnv failed: %v", err)
	}
	defer manager.DestroyEnv(envID)

	deadline := manager.GetNextTimerDeadline(envID)
	if !deadline.IsZero() {
		t.Errorf("expected zero time when no timers, got %v", deadline)
	}
}

func TestGetNextTimerDeadline_SingleTimer(t *testing.T) {
	manager := NewJSEnvManager()
	defer manager.SignalShutdown()

	envID := "test-next-single"
	pluginID := int64(1)

	if err := manager.CreateEnv(envID, polyfillJS, pluginID); err != nil {
		t.Fatalf("CreateEnv failed: %v", err)
	}
	defer manager.DestroyEnv(envID)

	before := time.Now()
	if _, err := manager.ExecuteJS(envID, "setTimeout(function(){}, 60000);", 1000); err != nil {
		t.Fatalf("setTimeout failed: %v", err)
	}

	deadline := manager.GetNextTimerDeadline(envID)
	if deadline.IsZero() {
		t.Fatal("expected non-zero deadline after setTimeout")
	}

	// 期望 deadline 大约在 before+60s 附近（容差 5s 处理 CI 延迟）
	expectedMin := before.Add(55 * time.Second)
	expectedMax := before.Add(65 * time.Second)
	if deadline.Before(expectedMin) || deadline.After(expectedMax) {
		t.Errorf("deadline %v outside expected range [%v, %v]", deadline, expectedMin, expectedMax)
	}
}

func TestGetNextTimerDeadline_PicksEarliest(t *testing.T) {
	manager := NewJSEnvManager()
	defer manager.SignalShutdown()

	envID := "test-next-earliest"
	pluginID := int64(1)

	if err := manager.CreateEnv(envID, polyfillJS, pluginID); err != nil {
		t.Fatalf("CreateEnv failed: %v", err)
	}
	defer manager.DestroyEnv(envID)

	before := time.Now()
	// 先注册一个 60s 的，再注册一个 10s 的，再注册一个 120s 的；期望选 10s 的。
	code := `
		setTimeout(function(){}, 60000);
		setTimeout(function(){}, 10000);
		setTimeout(function(){}, 120000);
	`
	if _, err := manager.ExecuteJS(envID, code, 1000); err != nil {
		t.Fatalf("setTimeout chain failed: %v", err)
	}

	deadline := manager.GetNextTimerDeadline(envID)
	if deadline.IsZero() {
		t.Fatal("expected non-zero deadline")
	}

	expectedMin := before.Add(5 * time.Second)
	expectedMax := before.Add(15 * time.Second)
	if deadline.Before(expectedMin) || deadline.After(expectedMax) {
		t.Errorf("deadline %v not picking the earliest (~10s) timer, expected in [%v, %v]",
			deadline, expectedMin, expectedMax)
	}
}

func TestGetNextTimerDeadline_IncludesInterval(t *testing.T) {
	manager := NewJSEnvManager()
	defer manager.SignalShutdown()

	envID := "test-next-interval"
	pluginID := int64(1)

	if err := manager.CreateEnv(envID, polyfillJS, pluginID); err != nil {
		t.Fatalf("CreateEnv failed: %v", err)
	}
	defer manager.DestroyEnv(envID)

	if _, err := manager.ExecuteJS(envID, "setInterval(function(){}, 30000);", 1000); err != nil {
		t.Fatalf("setInterval failed: %v", err)
	}

	deadline := manager.GetNextTimerDeadline(envID)
	if deadline.IsZero() {
		t.Error("expected non-zero deadline for setInterval")
	}
}

func TestProcessTimers_ReturnsFalse_WhenTimerNotYetExpired(t *testing.T) {
	manager := NewJSEnvManager()
	defer manager.SignalShutdown()

	envID := "test-timer-not-expired"
	pluginID := int64(1)

	// Create environment with a timer that won't fire for a while
	code := polyfillJS + `
		setTimeout(function(){}, 10000);
	`
	if err := manager.CreateEnv(envID, code, pluginID); err != nil {
		t.Fatalf("CreateEnv failed: %v", err)
	}
	defer manager.DestroyEnv(envID)

	// Process timers immediately - should return false because timer hasn't expired
	didFire := manager.ProcessTimers(envID)

	if didFire {
		t.Error("Expected ProcessTimers to return false when timer hasn't expired yet")
	}
}
