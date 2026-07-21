package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"songloft/internal/models"
)

// TestCacheGet_InflightWaiterCanceledByOwnCtx 验证：当首请求 inflight 还没完成时，
// 第二个等待者被自己的 ctx 取消（如用户切歌）应**立即返回**，而不是死等首请求。
// 这是 issue #79 的残留点：原先 <-dl.done 是单 channel 等待，无法响应等待者的 ctx。
func TestCacheGet_InflightWaiterCanceledByOwnCtx(t *testing.T) {
	tmpDir := t.TempDir()
	cs := &CacheService{cacheDir: tmpDir}

	song := &models.Song{ID: 9001, Type: "remote", URL: "http://example.invalid/song.mp3"}

	// 模拟首请求已经在 inflight，dl.done 永不关闭
	state := getSongState()
	state.inflightMu.Lock()
	dl := &inflightDownload{done: make(chan struct{})}
	state.inflight[song.ID] = dl
	state.inflightMu.Unlock()
	t.Cleanup(func() {
		state.inflightMu.Lock()
		if state.inflight[song.ID] == dl {
			delete(state.inflight, song.ID)
		}
		state.inflightMu.Unlock()
	})

	// 第二个等待者：ctx 短时间内取消
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := cs.Get(ctx, song)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Get 应当返回错误（ctx canceled）")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Get 等待 %v 太久；应在 ~50ms cancel 后立即返回", elapsed)
	}
}

// TestGetOrTranscode_InflightWaiterCanceledByOwnCtx 与上类似，覆盖转码 inflight 路径。
func TestGetOrTranscode_InflightWaiterCanceledByOwnCtx(t *testing.T) {
	tmpDir := t.TempDir()
	cs := &CacheService{cacheDir: tmpDir}

	song := &models.Song{ID: 9002, Type: "local", Format: "wma"}
	targetFormat := "mp3"

	// 手动注入一个永不完成的转码 inflight（key 含 bitrate=0、trackIndex=-1）
	inflightKey := "tc_9002_mp3_0_t-1"
	state := getSongState()
	state.transcodeInflightMu.Lock()
	dl := &inflightDownload{done: make(chan struct{})}
	state.transcodeInflight[inflightKey] = dl
	state.transcodeInflightMu.Unlock()
	t.Cleanup(func() {
		state.transcodeInflightMu.Lock()
		delete(state.transcodeInflight, inflightKey)
		state.transcodeInflightMu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// srcPath 不存在不影响测试——我们只走到 inflight 等待分支就够了
	start := time.Now()
	_, err := cs.GetOrTranscode(ctx, "/nonexistent/src.wma", song, targetFormat, 0, -1)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("GetOrTranscode 应当返回错误（ctx canceled）")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("GetOrTranscode 等待 %v 太久；应在 ~50ms cancel 后立即返回", elapsed)
	}
}
