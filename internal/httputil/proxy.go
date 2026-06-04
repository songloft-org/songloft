package httputil

import (
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

var sharedTransport = &http.Transport{
	Proxy:               globalProxy.proxyFunc,
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// SetGlobalProxy sets the global HTTP proxy used by all clients created via NewClient.
// Pass an empty string to clear the proxy (direct connection).
func SetGlobalProxy(rawURL string) error {
	if err := globalProxy.set(rawURL); err != nil {
		return err
	}
	sharedTransport.CloseIdleConnections()
	return nil
}

// GetGlobalProxy returns the current global HTTP proxy URL, or "" if not set.
func GetGlobalProxy() string {
	return globalProxy.get()
}

// NewClient creates an http.Client that uses the global HTTP proxy.
// Requests to loopback addresses bypass the proxy automatically.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: sharedTransport,
		Timeout:   timeout,
	}
}
