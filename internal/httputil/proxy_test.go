package httputil

import (
	"net/http"
	"testing"
)

func TestSetGetGlobalProxy(t *testing.T) {
	t.Cleanup(func() { _ = SetGlobalProxy("") })

	if got := GetGlobalProxy(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	if err := SetGlobalProxy("http://proxy.local:7890"); err != nil {
		t.Fatal(err)
	}
	if got := GetGlobalProxy(); got != "http://proxy.local:7890" {
		t.Fatalf("expected http://proxy.local:7890, got %q", got)
	}

	if err := SetGlobalProxy(""); err != nil {
		t.Fatal(err)
	}
	if got := GetGlobalProxy(); got != "" {
		t.Fatalf("expected empty after clear, got %q", got)
	}
}

func TestSetGlobalProxyInvalidURL(t *testing.T) {
	t.Cleanup(func() { _ = SetGlobalProxy("") })

	if err := SetGlobalProxy("://bad"); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestProxyFuncSkipsLoopback(t *testing.T) {
	t.Cleanup(func() { _ = SetGlobalProxy("") })

	if err := SetGlobalProxy("http://proxy.local:7890"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		url       string
		wantProxy bool
	}{
		{"http://github.com/foo", true},
		{"https://example.com/bar", true},
		{"http://localhost:8080/api", false},
		{"http://127.0.0.1:58091/test", false},
		{"http://[::1]:8080/test", false},
	}
	for _, tt := range tests {
		req, _ := http.NewRequest("GET", tt.url, nil)
		proxyURL, err := globalProxy.proxyFunc(req)
		if err != nil {
			t.Fatalf("proxyFunc(%s): %v", tt.url, err)
		}
		gotProxy := proxyURL != nil
		if gotProxy != tt.wantProxy {
			t.Errorf("proxyFunc(%s): gotProxy=%v, want=%v", tt.url, gotProxy, tt.wantProxy)
		}
	}
}

func TestProxyFuncNoProxy(t *testing.T) {
	t.Cleanup(func() { _ = SetGlobalProxy("") })

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	proxyURL, err := globalProxy.proxyFunc(req)
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL != nil {
		t.Fatal("expected nil proxy when not configured")
	}
}

func TestNewClientNotNil(t *testing.T) {
	c := NewClient(0)
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.Transport != sharedTransport {
		t.Fatal("expected shared transport")
	}
}
