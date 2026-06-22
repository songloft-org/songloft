package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/models"
)

// LyricFetcher 从 lyric URL 拉取歌词 payload。
//
// 期望上游响应为 JSON: {"code": 0, "data": {"lyric": "...", "tlyric": "...", "rlyric": "...", "lxlyric": "..."}}
// 这是项目内 JS 插件返回歌词的统一格式。Fetcher 把 data 部分解析成
// models.LyricPayload,让调用方拿到主歌词+翻译+罗马音+逐字四个字段,
// 后端写库/前端 API 都用同一种载体表达。
type LyricFetcher struct {
	urlResolver *InternalURLResolver
	httpClient  *http.Client
}

// NewLyricFetcher 创建 LyricFetcher。httpClient 为 nil 时使用 30s 超时的默认客户端。
func NewLyricFetcher(urlResolver *InternalURLResolver, httpClient *http.Client) *LyricFetcher {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &LyricFetcher{urlResolver: urlResolver, httpClient: httpClient}
}

// Fetch 拉取并解包歌词 payload。
//
// 相对路径会被 urlResolver 拼成本机 server + access_token。
// 任何环节失败(网络、JSON 格式、code != 0)都返回 (空 payload, error),
// 调用方应作为非致命错误处理。data 为空 payload 不算错误,返回 (空 payload, nil)。
func (f *LyricFetcher) Fetch(ctx context.Context, lyricURL string) (models.LyricPayload, error) {
	resolved := lyricURL
	if f.urlResolver != nil {
		resolved = f.urlResolver.Resolve(lyricURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return models.LyricPayload{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	httputil.ApplyBasicAuthFromURL(req)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return models.LyricPayload{}, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.LyricPayload{}, fmt.Errorf("http status %d", resp.StatusCode)
	}

	// 限制读取 5 MB,防止异常响应耗尽内存
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return models.LyricPayload{}, fmt.Errorf("read body: %w", err)
	}

	var envelope struct {
		Code    int                 `json:"code"`
		Data    models.LyricPayload `json:"data"`
		Message string              `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return models.LyricPayload{}, fmt.Errorf("parse json: %w", err)
	}
	if envelope.Code != 0 {
		return models.LyricPayload{}, fmt.Errorf("lyric api returned code=%d msg=%q", envelope.Code, envelope.Message)
	}
	return envelope.Data, nil
}
