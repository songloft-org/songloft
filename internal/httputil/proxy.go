package httputil

import (
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var globalProxy proxyConfig

type proxyConfig struct {
	mu       sync.RWMutex
	proxyURL *url.URL
}

func (pc *proxyConfig) set(rawURL string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if rawURL == "" {
		pc.proxyURL = nil
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	pc.proxyURL = u
	return nil
}

func (pc *proxyConfig) get() string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if pc.proxyURL == nil {
		return ""
	}
	return pc.proxyURL.String()
}

func (pc *proxyConfig) proxyFunc(req *http.Request) (*url.URL, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if pc.proxyURL == nil {
		return nil, nil
	}
	host := req.URL.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil, nil
	}
	return pc.proxyURL, nil
}

func newProxyTransport(responseHeaderTimeout time.Duration) *http.Transport {
	return &http.Transport{
		Proxy: globalProxy.proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}
}

var sharedTransport = newProxyTransport(0)

var streamingTransport = newProxyTransport(15 * time.Second)

// downloadTransport 用于「下载到本地」的大文件传输:无整请求超时(慢但持续的
// 下载靠 StallReader 做停滞检测),响应头超时给到 60s——比电台 streaming 的 15s
// 宽松,避免慢速代理/梯子下首字节偏慢时被误判死。(issue #265)
var downloadTransport = newProxyTransport(60 * time.Second)

// SetGlobalProxy sets the global HTTP proxy used by all clients created via NewClient.
// Pass an empty string to clear the proxy (direct connection).
func SetGlobalProxy(rawURL string) error {
	if err := globalProxy.set(rawURL); err != nil {
		return err
	}
	sharedTransport.CloseIdleConnections()
	streamingTransport.CloseIdleConnections()
	downloadTransport.CloseIdleConnections()
	return nil
}

// GetGlobalProxy returns the current global HTTP proxy URL, or "" if not set.
func GetGlobalProxy() string {
	return globalProxy.get()
}

// ProxyFromGlobal selects the current configured proxy for a request, bypassing
// loopback destinations. Custom transports can use it as their Proxy function
// to follow live settings updates while retaining their own TLS and pool policy.
func ProxyFromGlobal(req *http.Request) (*url.URL, error) {
	return globalProxy.proxyFunc(req)
}

// NewClient creates an http.Client that uses the global HTTP proxy.
// Requests to loopback addresses bypass the proxy automatically.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: sharedTransport,
		Timeout:   timeout,
	}
}

// NewStreamingClient creates an http.Client for long-lived streaming responses.
// It has no whole-request timeout, but it does time out while waiting for
// response headers so dead radio/HLS endpoints do not hang forever.
func NewStreamingClient() *http.Client {
	return &http.Client{
		Transport: streamingTransport,
	}
}

// NewDownloadClient creates an http.Client for downloading large files to local
// storage. It has no whole-request timeout (slow-but-steady transfers must not
// be killed mid-body — pair it with StallReader for stall detection) and a
// generous 60s response-header timeout to tolerate slow proxies/VPNs. (issue #265)
func NewDownloadClient() *http.Client {
	return &http.Client{
		Transport: downloadTransport,
	}
}
