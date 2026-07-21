package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DownloadChunkSize 单个 Range 分块的大小,对齐 yt-dlp `--http-chunk-size` 默认值(10MiB)。
//
// YouTube googlevideo 等 CDN 会把单连接顺序读限速到约等于媒体播放码率(issue #305 用户实测
// 只有 30~40kB/s),而每发起一个新的 Range 请求都会重置该限速——分块顺序拉取即可跑满带宽,
// 与直接用 yt-dlp 下载时的速度一致。
const DownloadChunkSize int64 = 10 << 20

// downloadUserAgent 分块下载默认 UA。可被 headers 里的同名头覆盖。
const downloadUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

// ChunkedDownload 通过 HTTP Range 分块顺序把 url 拉取到 dst,返回写入字节数与首个响应头
// (供调用方取 Content-Type 等)。
//
// 每发起一个新的 Range 请求都会重置 YouTube googlevideo 等 CDN 对单连接顺序读的限速
// (issue #305);服务端不支持 Range(首块返回 200)时自动回退为整段流式下载,行为与裸 GET 一致。
//
// 每个分块都套 StallReader 做停滞检测(不限总时长,issue #265),并按各响应的 Content-Length
// 做截断校验:服务端声明了长度却写入不足即判死,交由上层同源重试兜住。
//
// dst 由调用方负责创建与清理;本函数只写入,出错时不回滚已写内容(调用方应丢弃 dst)。
func ChunkedDownload(
	ctx context.Context,
	client *http.Client,
	url string,
	headers map[string]string,
	stallTimeout time.Duration,
	dst io.Writer,
) (written int64, firstHeader http.Header, err error) {
	var total int64
	for {
		start := written
		end := start + DownloadChunkSize - 1
		if total > 0 && end > total-1 {
			end = total - 1
		}

		n, hdr, chunkTotal, ranged, cerr := fetchChunk(ctx, client, url, headers, stallTimeout, dst, start, end)
		if start == 0 {
			firstHeader = hdr
		}
		if cerr != nil {
			return written, firstHeader, cerr
		}
		written += n

		// 首块即判定服务端是否支持 Range:不支持时 fetchChunk 已把整段写入,直接收尾。
		if start == 0 && !ranged {
			return written, firstHeader, nil
		}
		// 中途某块突然不按 Range 响应(返回 200)会导致重复内容,判死交上层重试。
		if start > 0 && !ranged {
			return written, firstHeader, fmt.Errorf("server stopped honoring Range at offset %d", start)
		}
		if chunkTotal > 0 {
			total = chunkTotal
		}

		// 终止:已知总长且拉满 / 未知总长时出现短读(末块) / 无任何推进(防死循环)。
		if total > 0 {
			if written >= total {
				break
			}
		} else if n < end-start+1 {
			break
		}
		if n == 0 {
			break
		}
	}

	// 已知总长却写入不足 → 被慢代理干净切断(issue #265),显式判死让上层同源重试兜住。
	if total > 0 && written < total {
		return written, firstHeader, fmt.Errorf("truncated: got %d of %d bytes", written, total)
	}
	return written, firstHeader, nil
}

// fetchChunk 发起一个 Range 请求 [start,end] 并把响应体追加写入 dst(带停滞检测)。
// 返回:本次写入字节数、响应头、文件总长(仅从 206 的 Content-Range 解析得到,否则为 0)、
// 服务端是否按 Range 响应(206=true / 200=false)、错误。
func fetchChunk(
	ctx context.Context,
	client *http.Client,
	url string,
	headers map[string]string,
	stallTimeout time.Duration,
	dst io.Writer,
	start, end int64,
) (written int64, header http.Header, total int64, ranged bool, err error) {
	// 可取消的子 ctx:StallReader 在停滞超时到达时 cancel,掐断阻塞中的 body Read。
	dlCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, 0, false, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", downloadUserAgent)
	// 应用调用方传入的自定义头(如 Referer / User-Agent),覆盖默认值。
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	ApplyBasicAuthFromURL(req)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, 0, false, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPartialContent:
		ranged = true
		total = parseContentRangeTotal(resp.Header.Get("Content-Range"))
	case http.StatusOK:
		// 服务端忽略 Range,返回整段。仅首块(start==0)可接受,后续 io.Copy 会读完整个 body。
		ranged = false
	default:
		return 0, resp.Header, 0, false, fmt.Errorf("http status %d", resp.StatusCode)
	}

	sr := NewStallReader(resp.Body, cancel, stallTimeout)
	defer sr.Stop()
	written, err = io.Copy(dst, sr)
	if err != nil {
		return written, resp.Header, total, ranged, fmt.Errorf("write: %w", err)
	}

	// 单响应截断校验:服务端声明了本次响应体长度(resp.ContentLength >= 0,对 206 即分块长度、
	// 对 200 即整段长度)却写入不足,说明被慢代理干净切断了——io.Copy 只看到正常 EOF 不会报错。
	if resp.ContentLength >= 0 && written < resp.ContentLength {
		return written, resp.Header, total, ranged, fmt.Errorf("truncated: got %d of %d bytes", written, resp.ContentLength)
	}

	return written, resp.Header, total, ranged, nil
}

// parseContentRangeTotal 从 Content-Range 头(形如 "bytes 0-9999/12345")解析文件总长。
// 总长为 "*"(未知)或解析失败时返回 0。
func parseContentRangeTotal(cr string) int64 {
	idx := strings.LastIndex(cr, "/")
	if idx < 0 {
		return 0
	}
	totalStr := strings.TrimSpace(cr[idx+1:])
	if totalStr == "" || totalStr == "*" {
		return 0
	}
	n, err := strconv.ParseInt(totalStr, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
