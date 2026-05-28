package source

import (
	"errors"
	"fmt"
)

// 错误分类:供调用方(Orchestrator)区分"该重试切源" vs "该直接报错"。
//
// 设计原则:
//   - InvalidAudioError / NetworkError → 允许 fallback(可能是单个源/单次请求的问题)
//   - PluginInvocationError → 通常意味着插件本身有问题,fallback 到别的插件可能有用
//   - AllSourcesFailedError → 所有候选都试过,终态错误,交给上层

// InvalidAudioError 表示下载文件未通过完整性校验(reason 来自 validator)。
// 这是 fallback 决策的关键信号 —— 主源虽然返回了 HTTP 200,但内容不可用。
type InvalidAudioError struct {
	Reason   ValidationReason
	Expected float64
	Actual   float64
}

func (e *InvalidAudioError) Error() string {
	return fmt.Sprintf("audio invalid: reason=%s expected=%.1fs actual=%.1fs", e.Reason, e.Expected, e.Actual)
}

// NetworkError 表示 HTTP 层失败:DNS/连接/超时/读流中断/非 2xx 状态码。
type NetworkError struct {
	Op  string // get / head / read / status
	URL string
	Err error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network %s %s: %v", e.Op, e.URL, e.Err)
}
func (e *NetworkError) Unwrap() error { return e.Err }

// PluginInvocationError 表示插件 music/url 接口调用失败:
//   - 插件未加载 / 调用超时 / 返回非 200
//   - response body 解析失败
//
// 这种错误意味着该插件目前不可用,Resolver 排序时会通过 metrics 自然降权。
type PluginInvocationError struct {
	PluginEntryPath string
	StatusCode      int    // 0 表示底层错误未到 HTTP 层
	Reason          string // 详细原因
	Err             error
}

func (e *PluginInvocationError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("plugin %s music/url returned %d: %s", e.PluginEntryPath, e.StatusCode, e.Reason)
	}
	if e.Err != nil {
		return fmt.Sprintf("plugin %s invocation failed: %v", e.PluginEntryPath, e.Err)
	}
	return fmt.Sprintf("plugin %s invocation failed: %s", e.PluginEntryPath, e.Reason)
}
func (e *PluginInvocationError) Unwrap() error { return e.Err }

// AllSourcesFailedError 终态错误:主源 + 所有候选源都失败。
// LastErr 是最后一次尝试的具体错误,便于日志/前端展示;Tried 是总尝试次数。
type AllSourcesFailedError struct {
	Tried   int
	LastErr error
}

func (e *AllSourcesFailedError) Error() string {
	if e.LastErr != nil {
		return fmt.Sprintf("all %d source(s) failed, last error: %v", e.Tried, e.LastErr)
	}
	return fmt.Sprintf("all %d source(s) failed", e.Tried)
}
func (e *AllSourcesFailedError) Unwrap() error { return e.LastErr }

// IsFallbackable 判断错误是否值得尝试 fallback。
// InvalidAudioError / NetworkError / PluginInvocationError 都允许;
// 其他错误(如 context.Canceled、内部逻辑错误)不应触发 fallback。
func IsFallbackable(err error) bool {
	if err == nil {
		return false
	}
	var ia *InvalidAudioError
	var ne *NetworkError
	var pe *PluginInvocationError
	return errors.As(err, &ia) || errors.As(err, &ne) || errors.As(err, &pe)
}
