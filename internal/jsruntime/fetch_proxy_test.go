package jsruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"songloft/internal/httputil"
)

// A reserved .invalid target makes bypassing the proxy fail without contacting
// LRCLIB. HTTPS CONNECT tunnels terminate at a local self-signed test server.
const proxyTestURL = "http://lyrics.invalid/api/get?track_name=Bohemian%20Rhapsody&artist_name=Queen"

func setFetchTestProxy(t *testing.T, proxy string) {
	t.Helper()
	previous := httputil.GetGlobalProxy()
	if err := httputil.SetGlobalProxy(proxy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = httputil.SetGlobalProxy(previous)
		sharedTransport.CloseIdleConnections()
		insecureTransport.CloseIdleConnections()
	})
}

func newFetchTestProxy(t *testing.T, marker string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close each tunneled connection so server cleanup cannot wait on idle TLS.
		w.Header().Set("Connection", "close")
		fmt.Fprint(w, marker)
	}))
	t.Cleanup(upstream.Close)
	hits := new(atomic.Int32)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method == http.MethodConnect {
			if r.Host != "lyrics.invalid:443" {
				t.Errorf("unexpected CONNECT target: %s", r.Host)
				http.Error(w, "unexpected target", http.StatusBadGateway)
				return
			}
			destination, err := net.DialTimeout("tcp", upstream.Listener.Addr().String(), time.Second)
			if err != nil {
				t.Errorf("connect to test upstream: %v", err)
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer destination.Close()
			connection, buffered, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack proxy connection: %v", err)
				return
			}
			defer connection.Close()
			_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
			_ = buffered.Flush()
			done := make(chan struct{})
			go func() {
				_, _ = io.Copy(destination, buffered)
				_ = destination.Close()
				close(done)
			}()
			_, _ = io.Copy(connection, destination)
			_ = connection.Close()
			<-done
			return
		}
		if r.URL.Host != "lyrics.invalid" || !r.URL.IsAbs() {
			t.Errorf("expected absolute proxy request, got %s", r.URL)
		}
		if r.Header.Get("X-Fetch-No-Redirect") != "" || r.Header.Get("X-Fetch-Insecure") != "" || r.Header.Get("X-Fetch-Timeout-Ms") != "" {
			t.Error("fetch control header leaked to proxy")
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, proxyTestURL, http.StatusFound)
			return
		}
		if r.URL.Query().Get("track_name") != "Bohemian Rhapsody" || r.URL.Query().Get("artist_name") != "Queen" {
			t.Errorf("query lost at proxy: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, marker)
	}))
	t.Cleanup(proxy.Close)
	return proxy, hits
}

func TestFetchGlobalProxyClientVariants(t *testing.T) {
	proxy, hits := newFetchTestProxy(t, "proxy-result")
	setFetchTestProxy(t, proxy.URL)
	for _, insecure := range []bool{false, true} {
		for _, noRedirect := range []bool{false, true} {
			t.Run(fmt.Sprintf("insecure=%v/noRedirect=%v", insecure, noRedirect), func(t *testing.T) {
				headers := map[string]string{"X-Fetch-Timeout-Ms": "1000"}
				if insecure {
					headers["X-Fetch-Insecure"] = "1"
				}
				if noRedirect {
					headers["X-Fetch-No-Redirect"] = "1"
				}
				headerJSON, _ := json.Marshal(headers)
				before := hits.Load()
				var result struct {
					Status int    `json:"status"`
					Body   string `json:"body"`
					Error  string `json:"error"`
				}
				raw := doHTTPRequest("http://lyrics.invalid/redirect", http.MethodGet, string(headerJSON), "", true)
				if err := json.Unmarshal([]byte(raw), &result); err != nil {
					t.Fatal(err)
				}
				wantStatus, wantHits := http.StatusOK, int32(2)
				if noRedirect {
					wantStatus, wantHits = http.StatusFound, 1
				}
				if result.Error != "" || result.Status != wantStatus || (!noRedirect && result.Body != "proxy-result") {
					t.Errorf("unexpected fetch result: %s", raw)
				}
				if got := hits.Load() - before; got != wantHits {
					t.Errorf("proxy requests = %d, want %d", got, wantHits)
				}
			})
		}
	}
}

func TestFetchGlobalProxyHTTPS(t *testing.T) {
	proxy, hits := newFetchTestProxy(t, "tunneled-result")
	setFetchTestProxy(t, proxy.URL)
	for _, tc := range []struct {
		name    string
		headers string
		allow   bool
		success bool
	}{
		{"verified", `{}`, false, false},
		{"verified-no-redirect", `{"X-Fetch-No-Redirect":"1"}`, false, false},
		{"insecure-denied", `{"X-Fetch-Insecure":"1"}`, false, false},
		{"insecure", `{"X-Fetch-Insecure":"1"}`, true, true},
		{"insecure-no-redirect", `{"X-Fetch-Insecure":"1","X-Fetch-No-Redirect":"1"}`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := hits.Load()
			raw := doHTTPRequest("https://lyrics.invalid/api/get", http.MethodGet, tc.headers, "", tc.allow)
			if tc.success {
				if !strings.Contains(raw, `"body":"tunneled-result"`) || !strings.Contains(raw, `"status":200`) {
					t.Errorf("HTTPS fetch did not return tunneled body: %s", raw)
				}
			} else if !strings.Contains(raw, "certificate") {
				t.Errorf("expected TLS certificate verification failure: %s", raw)
			}
			if got := hits.Load() - before; got != 1 {
				t.Errorf("CONNECT requests = %d, want 1", got)
			}
		})
	}
}

func TestFetchGlobalProxyLiveUpdateViaJSEnv(t *testing.T) {
	first, firstHits := newFetchTestProxy(t, "first")
	second, secondHits := newFetchTestProxy(t, "second")
	setFetchTestProxy(t, first.URL)
	manager := NewJSEnvManager()
	t.Cleanup(manager.SignalShutdown)
	const envID = "fetch-live-proxy"
	if err := manager.CreateEnv(envID, polyfillJS, 1); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.DestroyEnv(envID) })
	for _, tc := range []struct{ proxy, body string }{{first.URL, "first"}, {second.URL, "second"}} {
		if err := httputil.SetGlobalProxy(tc.proxy); err != nil {
			t.Fatal(err)
		}
		out := runProbeInEnv(t, manager, envID, fmt.Sprintf(`
			fetch(%q, {headers: {'X-Fetch-Timeout-Ms': '1000'}})
			.then(function(r) { return r.text(); })
			.then(function(body) { __out = body; })
			.catch(function(e) { __out = String(e); });`, proxyTestURL))
		if out != tc.body {
			t.Errorf("existing JS env fetch = %q, want %q", out, tc.body)
		}
	}
	if firstHits.Load() != 1 || secondHits.Load() != 1 {
		t.Errorf("proxy hits: first=%d second=%d, want 1 each", firstHits.Load(), secondHits.Load())
	}
}

func TestFetchGlobalProxyLoopbackBypass(t *testing.T) {
	proxy, hits := newFetchTestProxy(t, "unexpected-proxy")
	setFetchTestProxy(t, proxy.URL)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "local-result")
	}))
	t.Cleanup(local.Close)
	for _, headers := range []string{`{}`, `{"X-Fetch-Insecure":"1"}`} {
		raw := doHTTPRequest(local.URL, http.MethodGet, headers, "", true)
		if !strings.Contains(raw, `"body":"local-result"`) {
			t.Errorf("loopback request did not bypass proxy: %s", raw)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("loopback requests reached proxy: %d", hits.Load())
	}
}

func TestFetchEnvironmentProxy(t *testing.T) {
	// Go caches the proxy environment on first use, so run this test in a fresh process.
	const child = "SONGLOFT_TEST_FETCH_ENV_PROXY"
	if os.Getenv(child) == "" {
		t.Setenv(child, "1")
		cmd := exec.Command(os.Args[0], "-test.run=^TestFetchEnvironmentProxy$")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("proxy environment test: %v\n%s", err, output)
		}
		return
	}

	envProxy, envHits := newFetchTestProxy(t, "environment")
	settingsProxy, settingsHits := newFetchTestProxy(t, "settings")
	t.Setenv("HTTP_PROXY", envProxy.URL)
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", envProxy.URL)
	t.Setenv("NO_PROXY", "localhost,127.0.0.1,192.168.0.0/16")
	t.Setenv("REQUEST_METHOD", "")

	for _, tc := range []struct {
		name, setting, destination, headers, want string
	}{
		{"http", "", proxyTestURL, `{}`, "environment"},
		{"https", "", "https://lyrics.invalid/api/get", `{"X-Fetch-Insecure":"1"}`, "environment"},
		{"settings override", settingsProxy.URL, proxyTestURL, `{}`, "settings"},
		{"cleared setting", "", proxyTestURL, `{}`, "environment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFetchTestProxy(t, tc.setting)
			got := doHTTPRequest(tc.destination, http.MethodGet, tc.headers, "", true)
			if !strings.Contains(got, `"body":"`+tc.want+`"`) {
				t.Errorf("fetch response = %s, want body %q", got, tc.want)
			}
		})
	}
	if envHits.Load() != 3 || settingsHits.Load() != 1 {
		t.Errorf("proxy hits: environment=%d settings=%d, want 3 and 1", envHits.Load(), settingsHits.Load())
	}

	for _, destination := range []string{"http://192.168.1.100", "https://127.0.0.1", "http://[::1]"} {
		req := httptest.NewRequest(http.MethodGet, destination, nil)
		if proxy, err := sharedTransport.Proxy(req); proxy != nil || err != nil {
			t.Errorf("proxy for %s = %v, %v, want direct connection", destination, proxy, err)
		}
	}
}

func TestHTTPStreamGlobalProxy(t *testing.T) {
	proxy, hits := newFetchTestProxy(t, "stream-result")
	setFetchTestProxy(t, proxy.URL)
	stream := NewHTTPStream(context.Background(), proxyTestURL, http.MethodGet, `{"X-Fetch-Timeout-Ms":"1000"}`, "", false)
	defer stream.Close()
	status, _, err := stream.Headers()
	if err != nil || status != http.StatusOK {
		t.Fatalf("stream headers: status=%d err=%v", status, err)
	}
	var body strings.Builder
	for {
		chunk, err := stream.Read()
		if err != nil {
			t.Fatal(err)
		}
		if chunk == nil {
			break
		}
		body.Write(chunk)
	}
	if body.String() != "stream-result" || hits.Load() != 1 {
		t.Errorf("stream body=%q proxy hits=%d", body.String(), hits.Load())
	}
}
