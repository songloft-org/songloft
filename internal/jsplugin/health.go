package jsplugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"songloft/internal/jsruntime"
)

// HealthStatus 健康检查结果状态
type HealthStatus int

const (
	HealthStatusHealthy   HealthStatus = iota // 健康
	HealthStatusUnhealthy                     // 不健康
	HealthStatusIdle                          // 空闲
	HealthStatusBusy                          // VM 正在处理长请求（持锁中），非死亡
)

// String 返回健康状态的字符串表示
func (h HealthStatus) String() string {
	switch h {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusUnhealthy:
		return "unhealthy"
	case HealthStatusIdle:
		return "idle"
	case HealthStatusBusy:
		return "busy"
	default:
		return "unknown"
	}
}

// recoveryAttempt 记录一个 error 状态插件的自愈尝试进度。
type recoveryAttempt struct {
	attempts  int       // 已尝试次数（用于挑选下一档延迟）
	nextRetry time.Time // 下一次允许尝试的时间点
}

// recoveryBackoff 是 error → active 自愈的退避序列。
// 1m → 5m → 15m → 30m → 60m，超出后保持 60m 周期。
var recoveryBackoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
	60 * time.Minute,
}

// HealthChecker 管理插件健康状态
type HealthChecker struct {
	manager       *Manager
	checkInterval time.Duration // 默认 60s
	maxFailures   int           // 默认 3 次
	idleTimeout   time.Duration // 默认 10 分钟
	// maxBusyRounds 决定连续多少轮 Busy 后升级为 Unhealthy 候选（防真死锁兜底）。
	// 默认 5：5 × 60s = 5 分钟一直忙，认为不正常。
	maxBusyRounds int
	// wakeupLead 是定时器触发前的提前唤醒时间，默认 2 分钟。
	// 用于补偿 VM 重建 + onInit + 可能的初始化 fetch 开销，确保定时器准时触发。
	wakeupLead time.Duration

	failures       map[string]int              // entryPath -> 连续失败次数
	busyHits       map[string]int              // entryPath -> 连续 Busy 次数
	recoveries     map[string]*recoveryAttempt // entryPath -> error 状态自愈进度
	wakeupSchedule map[string]time.Time        // entryPath -> 唤醒时间（休眠插件等待定时器触发）
	mu             sync.Mutex
	cancel         context.CancelFunc
	done           chan struct{}
}

// HealthOption 健康检查器配置选项
type HealthOption func(*HealthChecker)

// WithCheckInterval 设置健康检查间隔
func WithCheckInterval(d time.Duration) HealthOption {
	return func(hc *HealthChecker) {
		if d > 0 {
			hc.checkInterval = d
		}
	}
}

// WithMaxFailures 设置最大连续失败次数
func WithMaxFailures(n int) HealthOption {
	return func(hc *HealthChecker) {
		if n > 0 {
			hc.maxFailures = n
		}
	}
}

// WithIdleTimeout 设置空闲超时时间
func WithIdleTimeout(d time.Duration) HealthOption {
	return func(hc *HealthChecker) {
		if d > 0 {
			hc.idleTimeout = d
		}
	}
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(manager *Manager, opts ...HealthOption) *HealthChecker {
	hc := &HealthChecker{
		manager:        manager,
		checkInterval:  60 * time.Second,
		maxFailures:    3,
		idleTimeout:    10 * time.Minute,
		maxBusyRounds:  5,
		wakeupLead:     2 * time.Minute,
		failures:       make(map[string]int),
		busyHits:       make(map[string]int),
		recoveries:     make(map[string]*recoveryAttempt),
		wakeupSchedule: make(map[string]time.Time),
		done:           make(chan struct{}),
	}

	for _, opt := range opts {
		opt(hc)
	}

	return hc
}

// Start 启动定期健康检查 goroutine
func (hc *HealthChecker) Start(ctx context.Context) {
	ctx, hc.cancel = context.WithCancel(ctx)

	go func() {
		defer close(hc.done)

		ticker := time.NewTicker(hc.checkInterval)
		defer ticker.Stop()

		slog.Info("health checker started",
			"interval", hc.checkInterval,
			"maxFailures", hc.maxFailures,
			"idleTimeout", hc.idleTimeout,
		)

		for {
			select {
			case <-ctx.Done():
				slog.Info("health checker stopped")
				return
			case <-ticker.C:
				hc.runChecks(ctx)
			}
		}
	}()
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	if hc.cancel != nil {
		hc.cancel()
		<-hc.done
	}
}

// runChecks 执行一轮健康检查
func (hc *HealthChecker) runChecks(ctx context.Context) {
	// error 状态自愈：在执行健康检查前先扫一轮，让到达 nextRetry 的插件
	// 抢先恢复，避免 health probe 把刚恢复的插件当作 missing 处理。
	hc.runRecoveryAttempts(ctx)

	// 唤醒因长定时器而休眠的插件：在 health probe 之前唤醒，让本轮就能纳入
	// 检查队列，避免下次循环才发现刚加载的插件。
	hc.runWakeupChecks(ctx)

	services := hc.manager.ListServices()

	for _, svc := range services {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 先检查空闲状态
		if hc.checkIdle(svc) {
			continue
		}

		// 执行健康检查（不走 scheduler 队列，避免被长 fetch 阻塞）
		status := hc.checkHealth(svc)
		switch status {
		case HealthStatusHealthy:
			// 健康，重置失败计数和 Busy 计数
			hc.mu.Lock()
			delete(hc.failures, svc.Name())
			delete(hc.busyHits, svc.Name())
			hc.mu.Unlock()
		case HealthStatusBusy:
			// VM 正在处理请求，不计失败也不计成功；连续 maxBusyRounds 次仍 Busy
			// 才升级为 Unhealthy 候选，作为真死锁的兜底信号。
			hc.mu.Lock()
			hc.busyHits[svc.Name()]++
			hits := hc.busyHits[svc.Name()]
			hc.mu.Unlock()
			if hits >= hc.maxBusyRounds {
				slog.Warn("plugin busy for too long, escalating to unhealthy",
					"plugin", svc.Name(),
					"consecutiveBusyRounds", hits,
				)
				hc.handleUnhealthy(ctx, svc)
				hc.mu.Lock()
				delete(hc.busyHits, svc.Name())
				hc.mu.Unlock()
			} else {
				slog.Debug("plugin busy, skip health check",
					"plugin", svc.Name(),
					"consecutiveBusyRounds", hits,
				)
			}
		case HealthStatusUnhealthy:
			hc.handleUnhealthy(ctx, svc)
		}
	}
}

// checkHealth 通过 jsruntime.HealthProbe 直连 VM 探针，绕开 scheduler 串行队列。
//
// 这样设计是为了避免如下级联失败：
//
//	长 fetch 持锁 30s → MsgHealthCheck 消息排队 → scheduler.Call 5s 超时 →
//	连续 3 次假阳性 → 插件被错误地标记为 error 状态。
//
// 现在改为直接对 env.mu 做 TryLock：抢到锁就 eval 1+1 验证 VM 存活；
// 抢不到就返回 Busy（不计失败）。
func (hc *HealthChecker) checkHealth(svc *JSService) HealthStatus {
	envID := svc.EnvID()
	if envID == "" {
		// service 还没 Load 完，Init 失败等场景；视作 Busy 跳过本轮。
		return HealthStatusBusy
	}

	switch hc.manager.jsManager.HealthProbe(envID) {
	case jsruntime.ProbeStatusHealthy:
		return HealthStatusHealthy
	case jsruntime.ProbeStatusBusy:
		return HealthStatusBusy
	case jsruntime.ProbeStatusMissing:
		// env 已被销毁；service 还在 services map 中说明状态不一致，
		// 视作不健康让 handleUnhealthy 走清理流程。
		slog.Warn("health check: env missing", "plugin", svc.Name())
		return HealthStatusUnhealthy
	default:
		return HealthStatusUnhealthy
	}
}

// handleUnhealthy 处理不健康插件
// 连续 maxFailures 次失败 → 标记为 error 状态，禁用插件
func (hc *HealthChecker) handleUnhealthy(ctx context.Context, svc *JSService) {
	entryPath := svc.Name()
	plugin := svc.Plugin()

	hc.mu.Lock()
	hc.failures[entryPath]++
	failCount := hc.failures[entryPath]
	hc.mu.Unlock()

	slog.Warn("plugin health check failed",
		"plugin", entryPath,
		"consecutiveFailures", failCount,
		"maxFailures", hc.maxFailures,
	)

	if failCount >= hc.maxFailures {
		slog.Error("plugin exceeded max health check failures, disabling",
			"plugin", entryPath,
			"failures", failCount,
		)

		// 标记为 error 状态
		if err := hc.manager.repo.UpdateStatus(ctx, plugin.ID, JSPluginStatusError); err != nil {
			slog.Error("failed to update plugin status to error", "plugin", entryPath, "error", err)
		}

		// 卸载插件
		if err := hc.manager.UnloadPlugin(ctx, entryPath); err != nil {
			slog.Error("failed to unload unhealthy plugin", "plugin", entryPath, "error", err)
		}

		// 清理失败计数 + 启动指数退避自愈：插件 1 分钟后开始尝试恢复，
		// 失败则按 1m/5m/15m/30m/60m 顺序退避。
		hc.mu.Lock()
		delete(hc.failures, entryPath)
		delete(hc.busyHits, entryPath)
		hc.recoveries[entryPath] = &recoveryAttempt{
			attempts:  0,
			nextRetry: time.Now().Add(recoveryBackoff[0]),
		}
		hc.mu.Unlock()
	}
}

// checkIdle 检查是否空闲。
// 使用 service.LastActive() 判断；超过 idleTimeout → 销毁 VM 释放资源
// （但保持数据库状态为 active，下次请求时重新加载）。
//
// 定时器决策：
//   - 无定时器 → 直接休眠。
//   - 下一个定时器在 3 × idleTimeout 内 → 保持活跃。
//   - 下一个定时器在 3 × idleTimeout 之外 → 休眠并记录唤醒时间，
//     由 runWakeupChecks 在定时器执行前 wakeupLead 时间唤醒。
func (hc *HealthChecker) checkIdle(svc *JSService) bool {
	entryPath := svc.Name()
	lastActive := svc.LastActive()

	if time.Since(lastActive) <= hc.idleTimeout {
		return false
	}

	// 有活跃 WebSocket 连接的插件不休眠
	envID := svc.EnvID()
	if envID != "" && hc.manager.jsManager.HasActiveWebSockets(envID) {
		slog.Debug("plugin has active WebSocket connections, not idle",
			"plugin", entryPath,
		)
		return false
	}

	// 有运行中子进程的插件不休眠
	if svc.HasRunningProcesses() {
		slog.Debug("plugin has running background processes, not idle",
			"plugin", entryPath,
		)
		return false
	}

	var wakeupAt time.Time // 零值表示无需唤醒（无定时器）

	if envID != "" {
		nextDeadline := hc.manager.jsManager.GetNextTimerDeadline(envID)
		if !nextDeadline.IsZero() {
			timeUntilNext := time.Until(nextDeadline)
			sleepThreshold := 3 * hc.idleTimeout

			if timeUntilNext < sleepThreshold {
				slog.Debug("plugin has near-term timer, not idle",
					"plugin", entryPath,
					"nextTimerIn", timeUntilNext,
					"threshold", sleepThreshold,
				)
				return false
			}
			wakeupAt = nextDeadline
		}
	}

	slog.Info("plugin idle timeout, unloading to free resources",
		"plugin", entryPath,
		"lastActive", lastActive,
		"idleTimeout", hc.idleTimeout,
		"hasWakeup", !wakeupAt.IsZero(),
	)

	// 在卸载前记录唤醒时间（卸载后 EnvID 失效，无法再查询定时器）
	if !wakeupAt.IsZero() {
		hc.scheduleWakeup(entryPath, wakeupAt)
	}

	// 卸载插件但不修改数据库状态（保持 active，下次请求时重新加载）
	if err := hc.manager.UnloadPlugin(context.Background(), entryPath); err != nil {
		slog.Warn("failed to unload idle plugin", "plugin", entryPath, "error", err)
		// 卸载失败回滚唤醒记录，避免下次唤醒重复触发
		hc.cancelWakeup(entryPath)
	}

	return true
}

// scheduleWakeup 记录插件在指定定时器 deadline 之前 wakeupLead 时间唤醒。
// 如果计算出的唤醒时间已过，立即用 time.Now()——下一轮 runWakeupChecks 即刻处理。
func (hc *HealthChecker) scheduleWakeup(entryPath string, deadline time.Time) {
	wakeAt := deadline.Add(-hc.wakeupLead)
	if wakeAt.Before(time.Now()) {
		wakeAt = time.Now()
	}
	hc.mu.Lock()
	hc.wakeupSchedule[entryPath] = wakeAt
	hc.mu.Unlock()
	slog.Info("plugin scheduled for wakeup",
		"plugin", entryPath,
		"wakeAt", wakeAt,
		"originalDeadline", deadline,
	)
}

// cancelWakeup 在插件被显式 disable / 卸载回滚等场景清理唤醒记录。
func (hc *HealthChecker) cancelWakeup(entryPath string) {
	hc.mu.Lock()
	delete(hc.wakeupSchedule, entryPath)
	hc.mu.Unlock()
}

// runWakeupChecks 扫描 wakeupSchedule，对到期项触发 LoadPlugin。
// 由 runChecks 在每轮 health check 末尾调用，最大唤醒延迟 = checkInterval（60s）。
func (hc *HealthChecker) runWakeupChecks(ctx context.Context) {
	now := time.Now()

	hc.mu.Lock()
	var due []string
	for entryPath, wakeAt := range hc.wakeupSchedule {
		if !now.Before(wakeAt) {
			due = append(due, entryPath)
		}
	}
	for _, ep := range due {
		delete(hc.wakeupSchedule, ep)
	}
	hc.mu.Unlock()

	for _, entryPath := range due {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 已被其他路径加载（懒加载/手动 enable）则跳过
		if _, ok := hc.manager.GetService(entryPath); ok {
			continue
		}

		plugin, err := hc.manager.repo.GetByEntryPath(ctx, entryPath)
		if err != nil || plugin == nil {
			slog.Debug("wakeup skipped: plugin not found",
				"plugin", entryPath, "error", err)
			continue
		}
		if plugin.Status != JSPluginStatusActive {
			slog.Debug("wakeup skipped: plugin not active",
				"plugin", entryPath, "status", plugin.Status)
			continue
		}

		if err := hc.manager.LoadPlugin(ctx, plugin); err != nil {
			slog.Warn("wakeup load failed", "plugin", entryPath, "error", err)
			continue
		}
		slog.Info("plugin woken up for timer", "plugin", entryPath)
	}
}

// ClearRecovery 清空指定插件的退避计数。
// 在用户主动 enable / 重新安装 / 手动 RecoverPlugin 时调用，
// 让下一次 error 退避重新从 1 分钟开始。
func (hc *HealthChecker) ClearRecovery(entryPath string) {
	hc.mu.Lock()
	delete(hc.recoveries, entryPath)
	hc.mu.Unlock()
}

// runRecoveryAttempts 扫描 DB 中 status=error 的插件，对到达退避窗口的尝试 LoadPlugin。
//
// 行为：
//   - 成功 → DB 改回 active，清空 recovery 进度。
//   - 失败 → attempts++，按下一档延迟重排；超出表尾保持最长间隔继续尝试。
//
// 退避表前 5 档（1m/5m/15m/30m/60m）失败时记 Warn 日志；之后降为 Debug，避免刷屏。
func (hc *HealthChecker) runRecoveryAttempts(ctx context.Context) {
	plugins, err := hc.manager.repo.GetAll(ctx)
	if err != nil {
		slog.Warn("recovery: list plugins failed", "error", err)
		return
	}

	now := time.Now()
	for _, plugin := range plugins {
		if plugin.Status != JSPluginStatusError {
			continue
		}

		hc.mu.Lock()
		ra, exists := hc.recoveries[plugin.EntryPath]
		if !exists {
			// 启动期遇到的历史 error 状态（进程没崩前未走过 handleUnhealthy 路径）：
			// 立即首次尝试，并以最短延迟初始化退避。
			ra = &recoveryAttempt{attempts: 0, nextRetry: now}
			hc.recoveries[plugin.EntryPath] = ra
		}
		if now.Before(ra.nextRetry) {
			hc.mu.Unlock()
			continue
		}
		hc.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		default:
		}

		// 在修改 DB 前重新确认当前状态仍为 error：
		// 用户可能在 GetAll 快照之后调用了 DisablePlugin（status → inactive），
		// 此时不应覆盖用户意图。
		fresh, err := hc.manager.repo.GetByID(ctx, plugin.ID)
		if err != nil || fresh.Status != JSPluginStatusError {
			continue
		}

		// 进入恢复尝试。先把 DB 状态推回 active 再调 LoadPlugin：
		// LoadPlugin 自身不检查 DB status，状态切换的语义保留在 EnablePlugin/Recovery 路径。
		if err := hc.manager.repo.UpdateStatus(ctx, plugin.ID, JSPluginStatusActive); err != nil {
			hc.scheduleNextRetry(plugin.EntryPath, ra, fmt.Errorf("update status active: %w", err))
			continue
		}

		if err := hc.manager.LoadPlugin(ctx, plugin); err != nil {
			// 加载失败回滚 DB 为 error 并按下一档退避
			if rollback := hc.manager.repo.UpdateStatus(ctx, plugin.ID, JSPluginStatusError); rollback != nil {
				slog.Error("recovery: rollback to error failed",
					"plugin", plugin.EntryPath, "error", rollback)
			}
			hc.scheduleNextRetry(plugin.EntryPath, ra, err)
			continue
		}

		slog.Info("plugin recovered automatically",
			"plugin", plugin.EntryPath,
			"attempts", ra.attempts+1,
		)

		hc.mu.Lock()
		delete(hc.recoveries, plugin.EntryPath)
		delete(hc.failures, plugin.EntryPath)
		delete(hc.busyHits, plugin.EntryPath)
		hc.mu.Unlock()
	}
}

// scheduleNextRetry 在自愈失败时按退避表推进下一档延迟。
func (hc *HealthChecker) scheduleNextRetry(entryPath string, ra *recoveryAttempt, cause error) {
	hc.mu.Lock()
	ra.attempts++
	idx := ra.attempts
	if idx >= len(recoveryBackoff) {
		idx = len(recoveryBackoff) - 1
	}
	ra.nextRetry = time.Now().Add(recoveryBackoff[idx])
	attempts := ra.attempts
	nextIn := recoveryBackoff[idx]
	hc.mu.Unlock()

	if attempts <= len(recoveryBackoff) {
		slog.Warn("recovery: load failed, scheduling retry",
			"plugin", entryPath,
			"attempts", attempts,
			"nextIn", nextIn,
			"error", cause,
		)
	} else {
		slog.Debug("recovery: load failed (long-term retry)",
			"plugin", entryPath,
			"attempts", attempts,
			"nextIn", nextIn,
			"error", cause,
		)
	}
}

// RecoverPlugin 手动恢复被禁用的插件
func (hc *HealthChecker) RecoverPlugin(ctx context.Context, pluginID int64) error {
	plugin, err := hc.manager.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin by id %d: %w", pluginID, err)
	}

	if plugin.Status != JSPluginStatusError {
		return fmt.Errorf("plugin %q is not in error state (current: %s)", plugin.EntryPath, plugin.Status)
	}

	slog.Info("recovering plugin from error state", "plugin", plugin.EntryPath, "id", pluginID)

	// 更新状态为 active
	if err := hc.manager.repo.UpdateStatus(ctx, pluginID, JSPluginStatusActive); err != nil {
		return fmt.Errorf("update plugin status: %w", err)
	}

	// 重新加载
	if err := hc.manager.LoadPlugin(ctx, plugin); err != nil {
		// 加载失败，回滚状态
		_ = hc.manager.repo.UpdateStatus(ctx, pluginID, JSPluginStatusError)
		return fmt.Errorf("load plugin after recovery: %w", err)
	}

	// 清理失败计数 + 退避计数（手动恢复让自愈周期重新从 1m 起）
	hc.mu.Lock()
	delete(hc.failures, plugin.EntryPath)
	delete(hc.busyHits, plugin.EntryPath)
	delete(hc.recoveries, plugin.EntryPath)
	hc.mu.Unlock()

	slog.Info("plugin recovered successfully", "plugin", plugin.EntryPath, "id", pluginID)
	return nil
}
