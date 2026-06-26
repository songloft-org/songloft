package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"songloft/internal/httputil"
)

// Prober 是 MetadataExtractor.ProbeForValidation 的抽象。
// 通过接口注入,避免 source 包反向依赖 services 包。
type Prober interface {
	ProbeForValidation(ctx context.Context, filePath string) (AudioInfoLike, error)
}

// PluginInvoker 是 jsplugin.Manager.InvokeHTTP 的抽象。
// 同样通过接口注入,避免依赖具体类型。
type PluginInvoker interface {
	InvokeHTTP(
		ctx context.Context,
		entryPath, method, path string,
		query interface{}, // url.Values 或 nil(用 interface{} 避免在此包导入 net/url)
		body []byte,
	) (statusCode int, respHeaders map[string]string, respBody []byte, err error)
}

// SongInfo 是 Fetcher / Resolver / Orchestrator 共用的歌曲身份信息,
// 通过接口抽象,避免依赖 models.Song 类型。
type SongInfo struct {
	ID              int64
	Title           string
	Artist          string
	Album           string
	Duration        float64
	PluginEntryPath string
	SourceData      string
}

// FetchResult 一次 Fetch 成功的产物。调用方负责清理 TempPath。
type FetchResult struct {
	TempPath          string         // 临时文件绝对路径
	Info              *AudioInfoCopy // 探测结果(用 source 包内类型,与外部解耦)
	PluginEntryPath   string         // 实际生效的源插件(可能因 L1/L2 fallback 而与原 song 不同)
	SourceData        string         // 实际生效的 source_data(同上)
	UpdatedSourceData string         // 若插件触发了 L1 自搜返回新 source_data,在此非空;调用方应回写 song
	UsedFallback      bool           // 是否触发了插件 L1 自搜
}

// AudioInfoCopy source 包内部的探测结果副本,实现 AudioInfoLike。
// 跟 services.AudioInfo 字段一致,只是为了让 FetchResult 不引外部类型。
type AudioInfoCopy struct {
	Duration   float64
	Format     string
	BitRate    int
	SampleRate int
	Size       int64
}

func (a *AudioInfoCopy) GetDuration() float64 { return a.Duration }
func (a *AudioInfoCopy) GetSize() int64       { return a.Size }
func (a *AudioInfoCopy) GetFormat() string    { return a.Format }

// FetcherOpts 注入到 SourceFetcher 的依赖与配置。
type FetcherOpts struct {
	Prober             Prober
	PluginInvoker      PluginInvoker
	Metrics            *SourceMetrics
	HTTPClient         *http.Client
	LoadValidationOpts func() ValidationOpts // 每次 Fetch 时读最新配置,允许运维热改
	// HTTPTimeout 单次下载的总超时
	HTTPTimeout time.Duration
}

// SourceFetcher 通过 (plugin_entry_path, source_data) 拉取一个临时文件并完成校验。
// 失败时上报 metrics、清理临时文件,返回分类后的错误供 Orchestrator 决定是否 fallback。
type SourceFetcher struct {
	opts FetcherOpts
}

func NewSourceFetcher(opts FetcherOpts) *SourceFetcher {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 120 * time.Second}
	}
	if opts.HTTPTimeout == 0 {
		opts.HTTPTimeout = 120 * time.Second
	}
	if opts.LoadValidationOpts == nil {
		def := DefaultValidationOpts()
		opts.LoadValidationOpts = func() ValidationOpts { return def }
	}
	return &SourceFetcher{opts: opts}
}

// musicURLRequest 调用插件 /api/music/url 的请求体。
// fallback 字段允许插件内自搜(L1 兜底)。
type musicURLRequest struct {
	SourceData json.RawMessage   `json:"source_data"`
	Fallback   *musicURLFallback `json:"fallback,omitempty"`
}

type musicURLFallback struct {
	Enabled  bool    `json:"enabled"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Duration float64 `json:"duration,omitempty"`
}

// musicURLResponse 插件 /api/music/url 的响应。
type musicURLResponse struct {
	URL          string          `json:"url"`
	SourceData   json.RawMessage `json:"source_data,omitempty"` // L1 自搜后返回的新音源(可空)
	UsedFallback bool            `json:"used_fallback,omitempty"`
}

// ResolveURL 仅调用插件 /api/music/url 解析出可下载的音频 URL，不下载、不探测、不校验。
// 用于流式代理场景：先解析 URL，再由 handler 直接代理到客户端。
func (f *SourceFetcher) ResolveURL(
	ctx context.Context,
	entryPath, sourceData string,
	song *SongInfo,
	allowPluginFallback bool,
) (string, error) {
	resp, err := f.invokePluginMusicURL(ctx, entryPath, sourceData, song, allowPluginFallback)
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}

// invokePluginMusicURL 调用插件 /api/music/url 接口解析真实下载 URL。
// Fetch 和 ResolveURL 共用此方法。
func (f *SourceFetcher) invokePluginMusicURL(
	ctx context.Context,
	entryPath, sourceData string,
	song *SongInfo,
	allowPluginFallback bool,
) (*musicURLResponse, error) {
	reqBody := musicURLRequest{SourceData: json.RawMessage(sourceData)}
	if allowPluginFallback && song != nil {
		reqBody.Fallback = &musicURLFallback{
			Enabled:  true,
			Title:    song.Title,
			Artist:   song.Artist,
			Duration: song.Duration,
		}
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &PluginInvocationError{PluginEntryPath: entryPath, Reason: "marshal request", Err: err}
	}

	status, _, respBody, err := f.opts.PluginInvoker.InvokeHTTP(
		ctx, entryPath, http.MethodPost, "/api/music/url", nil, bodyBytes,
	)
	if err != nil {
		return nil, &PluginInvocationError{PluginEntryPath: entryPath, Reason: "invoke failed", Err: err}
	}
	if status != http.StatusOK {
		return nil, &PluginInvocationError{PluginEntryPath: entryPath, StatusCode: status, Reason: string(respBody)}
	}
	if len(respBody) == 0 {
		return nil, &PluginInvocationError{PluginEntryPath: entryPath, Reason: "empty response body"}
	}

	var resp musicURLResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, &PluginInvocationError{PluginEntryPath: entryPath, Reason: "decode response", Err: err}
	}
	if resp.URL == "" {
		return nil, &PluginInvocationError{PluginEntryPath: entryPath, Reason: "empty url"}
	}
	return &resp, nil
}

// Fetch 通过 (entryPath, sourceData) 获取临时文件并完成校验。
//
// allowPluginFallback:
//   - true:在 music/url 请求里带 fallback hint,允许插件内部 L1 自搜
//   - false:严格模式,仅尝试当前 source_data(用于 Orchestrator L2 跨插件 fallback 时,
//     避免对每个候选都再调一次 L1 自搜,导致无限重试)
//
// 错误分类:
//   - 插件调用失败 → *PluginInvocationError(metrics 记 OutcomePluginInvocationFail)
//   - HTTP 下载失败 → *NetworkError(metrics 记 OutcomeNetworkFail)
//   - ffprobe 失败 → *NetworkError 或 *InvalidAudioError(reason=probe_failed)
//   - 校验失败 → *InvalidAudioError(metrics 记 OutcomeValidationFail)
//   - 成功 → nil(metrics 记 OutcomeSuccess)
//
// 失败时自动清理临时文件;成功时返回路径由调用方处置(orchestrator 决定是落 cache 还是 convert temp)。
func (f *SourceFetcher) Fetch(
	ctx context.Context,
	entryPath, sourceData string,
	song *SongInfo,
	allowPluginFallback bool,
) (*FetchResult, error) {
	start := time.Now()
	report := func(result OutcomeResult, reason string, size int64) {
		if f.opts.Metrics != nil {
			f.opts.Metrics.Record(Outcome{
				PluginEntryPath: entryPath,
				Result:          result,
				Reason:          reason,
				Latency:         time.Since(start),
				SizeBytes:       size,
				Timestamp:       time.Now(),
			})
		}
	}

	// 1. 调用插件 music/url
	resp, err := f.invokePluginMusicURL(ctx, entryPath, sourceData, song, allowPluginFallback)
	if err != nil {
		report(OutcomePluginInvocationFail, err.Error(), 0)
		return nil, err
	}

	// 2. HTTP 下载到临时文件
	dlCtx, cancel := context.WithTimeout(ctx, f.opts.HTTPTimeout)
	defer cancel()
	tmpPath, size, err := f.downloadToTemp(dlCtx, resp.URL)
	if err != nil {
		report(OutcomeNetworkFail, err.Error(), 0)
		return nil, &NetworkError{Op: "get", URL: resp.URL, Err: err}
	}

	cleanup := func() { _ = os.Remove(tmpPath) }

	// 3. 探测
	info, err := f.opts.Prober.ProbeForValidation(ctx, tmpPath)
	if err != nil {
		cleanup()
		report(OutcomeProbeFail, err.Error(), size)
		return nil, &InvalidAudioError{Reason: ReasonProbeFailed}
	}

	// 4. 校验
	expected := 0.0
	if song != nil {
		expected = song.Duration
	}
	vres := Validate(info, expected, f.opts.LoadValidationOpts())
	if !vres.Valid {
		cleanup()
		report(OutcomeValidationFail, string(vres.Reason), size)
		return nil, &InvalidAudioError{Reason: vres.Reason, Expected: vres.Expected, Actual: vres.Actual}
	}

	// 5. 成功
	report(OutcomeSuccess, "", size)

	// 把 AudioInfoLike 投影成 AudioInfoCopy(避免 FetchResult 引外部类型)
	infoCopy := &AudioInfoCopy{
		Duration: info.GetDuration(),
		Size:     info.GetSize(),
		Format:   info.GetFormat(),
	}

	result := &FetchResult{
		TempPath:        tmpPath,
		Info:            infoCopy,
		PluginEntryPath: entryPath,
		SourceData:      sourceData,
		UsedFallback:    resp.UsedFallback,
	}
	if resp.UsedFallback && len(resp.SourceData) > 0 {
		result.UpdatedSourceData = string(resp.SourceData)
	}
	return result, nil
}

// downloadToTemp 把 url 流式拉到临时文件,返回路径与写入字节数。
// 不做 Content-Type 校验,所有内容审计由后续 Probe + Validate 兜底,
// 因为部分 CDN 返回 application/octet-stream 是正常的。
func (f *SourceFetcher) downloadToTemp(ctx context.Context, url string) (string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	httputil.ApplyBasicAuthFromURL(req)

	resp, err := f.opts.HTTPClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("http status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "songloft-source-*")
	if err != nil {
		return "", 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	written, err := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, fmt.Errorf("write temp: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", 0, fmt.Errorf("close temp: %w", closeErr)
	}

	return tmpPath, written, nil
}

// 编译期断言,避免 errors import 在没用时被裁掉
var _ = errors.Is
