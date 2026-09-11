package httputil

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultHeadSize = 256 * 1024 // 256KB，覆盖大多数格式的头部标签+封面
	defaultTailSize = 32 * 1024  // 32KB，覆盖 ID3v1 (128B) 和 APEv2 footer
)

var ErrGapRead = errors.New("read position falls in unfetched gap between head and tail buffers")

// HTTPReadSeeker 通过 HTTP Range 请求预取文件首尾数据，实现 io.ReadSeeker。
// 适用于只需读取文件头部和尾部的场景（如音频标签解析）。
type HTTPReadSeeker struct {
	head    []byte
	tail    []byte
	size    int64
	pos     int64
	tailOff int64 // tail 缓冲区在文件中的起始偏移
}

// NewHTTPReadSeeker 创建 HTTPReadSeeker（无自定义请求头）。
func NewHTTPReadSeeker(client *http.Client, url string) (*HTTPReadSeeker, error) {
	return NewHTTPReadSeekerWithHeaders(client, url, nil)
}

// NewHTTPReadSeekerWithHeaders 创建带自定义请求头的 HTTPReadSeeker。
// 发起 HEAD 请求获取文件大小，再通过 Range GET 预取首尾数据。
// 若服务端不支持 Range 请求，返回错误（调用方应 fallback）。
// headers 中含 CR/LF 的条目会被静默跳过。
func NewHTTPReadSeekerWithHeaders(client *http.Client, url string, headers map[string]string) (*HTTPReadSeeker, error) {
	size, err := fetchContentLength(client, url, headers)
	if err != nil {
		return nil, fmt.Errorf("http read seeker: %w", err)
	}
	if size <= 0 {
		return nil, fmt.Errorf("http read seeker: unknown content length")
	}

	rs := &HTTPReadSeeker{size: size}

	if size <= int64(defaultHeadSize) {
		data, err := fetchRange(client, url, 0, size-1, headers)
		if err != nil {
			return nil, fmt.Errorf("http read seeker: fetch small file: %w", err)
		}
		rs.head = data
		rs.tailOff = size
		return rs, nil
	}

	head, err := fetchRange(client, url, 0, int64(defaultHeadSize)-1, headers)
	if err != nil {
		return nil, fmt.Errorf("http read seeker: fetch head: %w", err)
	}
	rs.head = head

	tailStart := size - int64(defaultTailSize)
	if tailStart < int64(defaultHeadSize) {
		tailStart = int64(defaultHeadSize)
	}
	tail, err := fetchRange(client, url, tailStart, size-1, headers)
	if err != nil {
		return nil, fmt.Errorf("http read seeker: fetch tail: %w", err)
	}
	rs.tail = tail
	rs.tailOff = tailStart

	return rs, nil
}

func (r *HTTPReadSeeker) Read(p []byte) (int, error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}

	total := 0
	for len(p) > 0 && r.pos < r.size {
		n, err := r.readAt(p, r.pos)
		r.pos += int64(n)
		total += n
		p = p[n:]
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (r *HTTPReadSeeker) readAt(p []byte, off int64) (int, error) {
	headEnd := int64(len(r.head))

	// 在 head 范围内
	if off < headEnd {
		n := copy(p, r.head[off:])
		return n, nil
	}

	// 在 tail 范围内
	if off >= r.tailOff && len(r.tail) > 0 {
		idx := off - r.tailOff
		if idx >= int64(len(r.tail)) {
			return 0, io.EOF
		}
		n := copy(p, r.tail[idx:])
		return n, nil
	}

	// 在间隙中
	return 0, ErrGapRead
}

func (r *HTTPReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = r.pos + offset
	case io.SeekEnd:
		newPos = r.size + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if newPos < 0 {
		return 0, fmt.Errorf("negative seek position: %d", newPos)
	}
	r.pos = newPos
	return newPos, nil
}

// Size 返回文件总大小。
func (r *HTTPReadSeeker) Size() int64 {
	return r.size
}

// isSafeHeaderValue 检查 header key/value 是否含 CR/LF（防注入）。
func isSafeHeaderValue(s string) bool {
	return !strings.ContainsAny(s, "\r\n")
}

// setHeaders 将自定义 headers 注入请求（跳过含 CR/LF 的条目）。
func setHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		if isSafeHeaderValue(k) && isSafeHeaderValue(v) {
			req.Header.Set(k, v)
		}
	}
}

// fetchContentLength 获取远程文件总大小。
// 优先 HEAD；部分服务器不路由 HEAD（404/405， 仅注册
// GET/POST）或不回 Content-Length，此时回退 GET + Range: bytes=0-0 探测——
// 两条路径都只读响应头、不下载文件内容。
func fetchContentLength(client *http.Client, url string, headers map[string]string) (int64, error) {
	if size, err := fetchContentLengthByHead(client, url, headers); err == nil {
		return size, nil
	}
	return fetchContentLengthByRange(client, url, headers)
}

func fetchContentLengthByHead(client *http.Client, url string, headers map[string]string) (int64, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, fmt.Errorf("HEAD request failed: %w", err)
	}
	setHeaders(req, headers)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HEAD request failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD returned status %d", resp.StatusCode)
	}

	cl := resp.Header.Get("Content-Length")
	if cl == "" {
		return 0, fmt.Errorf("no Content-Length header")
	}

	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Content-Length %q: %w", cl, err)
	}
	return size, nil
}

// fetchContentLengthByRange HEAD 不可用时的兜底：GET + Range: bytes=0-0。
// - 206：从 Content-Range「bytes 0-0/total」解析总大小；
// - 416：部分服务器以「bytes */total」回告总大小，同样可解析；
// - 200：服务器忽略 Range 返回全量，Content-Length 即总大小
//   （响应头已到手，body 不读取直接断开，不会拉取文件内容）。
func fetchContentLengthByRange(client *http.Client, url string, headers map[string]string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("range probe request failed: %w", err)
	}
	setHeaders(req, headers)
	req.Header.Set("Range", "bytes=0-0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("range probe request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable:
		if total := parseContentRangeTotal(resp.Header.Get("Content-Range")); total > 0 {
			return total, nil
		}
		return 0, fmt.Errorf("range probe: status %d without parsable total (Content-Range %q)",
			resp.StatusCode, resp.Header.Get("Content-Range"))
	case http.StatusOK:
		cl := resp.Header.Get("Content-Length")
		if cl == "" {
			return 0, fmt.Errorf("range probe: 200 without Content-Length")
		}
		size, err := strconv.ParseInt(cl, 10, 64)
		if err != nil || size <= 0 {
			return 0, fmt.Errorf("range probe: invalid Content-Length %q", cl)
		}
		return size, nil
	default:
		return 0, fmt.Errorf("range probe returned status %d", resp.StatusCode)
	}
}

func fetchRange(client *http.Client, url string, start, end int64, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req, headers)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("range request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		if resp.ContentLength > 0 && resp.ContentLength == end-start+1 {
			return io.ReadAll(resp.Body)
		}
		if !strings.Contains(resp.Header.Get("Accept-Ranges"), "bytes") &&
			resp.Header.Get("Content-Range") == "" {
			return nil, fmt.Errorf("server does not support range requests")
		}
		return io.ReadAll(io.LimitReader(resp.Body, end-start+1))
	}

	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status %d for range request", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// FormatFFmpegHeaders 将 headers map 格式化为 ffprobe/ffmpeg -headers 参数值。
// 格式：每个 header 以 "\r\n" 结尾，多个拼接成一个字符串。
// 含 CR/LF 的 key/value 会被跳过。headers 为 nil 或空时返回 ""。
func FormatFFmpegHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range headers {
		if isSafeHeaderValue(k) && isSafeHeaderValue(v) {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteString("\r\n")
		}
	}
	return b.String()
}
