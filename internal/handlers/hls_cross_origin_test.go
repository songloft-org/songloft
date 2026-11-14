package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"songloft/internal/models"
	"songloft/internal/services"

	"github.com/go-chi/chi/v5"
)

// Public URL hosts are routed to a local HTTP fixture without replacing the
// production hostname policy or redirect callback. No external requests occur.
func newCrossOriginHLSStack(t *testing.T, upstream http.Handler) (*models.Song, *HLSHandler, http.Handler, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		upstream.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	source := "http://8.8.8.8/live.m3u8"
	repo := newTestSongRepo(t)
	id := seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "radio", URL: source})
	seedSong(t, repo, &models.Song{Type: models.TypeRadio, Title: "other", URL: source})
	h := NewHLSHandler(services.NewSongService(repo, nil, nil, nil, nil, nil), nil)
	// chi supplies the path parameter consumed by resolveEndpoint.
	router := chi.NewRouter()
	router.HandleFunc("/api/v1/songs/{id}/hls/playlist", h.HandlePlaylist)
	router.HandleFunc("/api/v1/songs/{id}/hls/segment", h.HandleSegment)
	transport := h.client.Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	h.client.Transport = transport
	t.Cleanup(transport.CloseIdleConnections)
	return &models.Song{ID: id, Type: models.TypeRadio, Title: "radio", URL: source}, h, router, &hits
}

func proxyHLSRequest(t *testing.T, handler http.Handler, base *url.URL, ref, method string) (*httptest.ResponseRecorder, *url.URL) {
	t.Helper()
	target := base.ResolveReference(mustURL(t, ref))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, target.RequestURI(), nil)
	if method == http.MethodGet && strings.Contains(target.Path, "/segment") {
		req.Header.Set("Range", "bytes=0-3")
	}
	handler.ServeHTTP(rr, req)
	return rr, target
}

func TestHLSIntegration_CrossOriginChain(t *testing.T) {
	song, h, router, _ := newCrossOriginHLSStack(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "" || r.URL.Query().Get("sig") != "" {
			t.Error("proxy credentials leaked upstream")
		}
		switch r.Host + r.URL.Path {
		case "8.8.8.8/live.m3u8":
			http.Redirect(w, r, "http://1.1.1.1/live/master.m3u8", http.StatusFound)
		case "1.1.1.1/live/master.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=128000\nhttp://9.9.9.9/media.m3u8\n")
		case "9.9.9.9/media.m3u8":
			http.Redirect(w, r, "http://1.1.1.1/cdn/index.m3u8", http.StatusFound)
		case "1.1.1.1/cdn/index.m3u8":
			fmt.Fprint(w, `#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="http://9.9.9.9/key.bin"
#EXT-X-MAP:URI="init.mp4"
#EXT-X-PART:DURATION=1,URI="http://9.9.9.9/part.m4s"
#EXTINF:5,
http://9.9.9.9/seg.ts?timeshift=1&starttime=123&endtime=456
#EXTINF:5,
relative.ts
`)
		default:
			switch r.Host + r.URL.Path {
			case "9.9.9.9/key.bin", "9.9.9.9/part.m4s", "9.9.9.9/seg.ts",
				"1.1.1.1/cdn/init.mp4", "1.1.1.1/cdn/relative.ts":
			default:
				t.Errorf("unexpected resource URL after redirects: %s", r.Host+r.URL.Path)
				http.NotFound(w, r)
				return
			}
			if r.Header.Get("Range") != "bytes=0-3" && r.Method != http.MethodHead {
				t.Errorf("Range not forwarded: %v", r.Header)
			}
			if r.URL.Path == "/seg.ts" && r.URL.RawQuery != "timeshift=1&starttime=123&endtime=456" {
				t.Errorf("upstream query changed: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "video/MP2T")
			w.Header().Set("Content-Range", "bytes 0-3/8")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			if r.Method != http.MethodHead {
				fmt.Fprint(w, "DATA")
			}
		}
	}))
	base := mustURL(t, fmt.Sprintf("http://srv/music/api/v1/songs/%d/play.m3u8?access_token=TOKEN", song.ID))
	rr := httptest.NewRecorder()
	h.ServeProxy(rr, httptest.NewRequest("GET", base.String(), nil), song)
	if rr.Code != 200 {
		t.Fatalf("master: %d %s", rr.Code, rr.Body.String())
	}
	ref := extractFirstURLByTag(t, rr.Body.String(), "hls/playlist?u=")
	if ref == "" {
		t.Fatalf("cross-origin child not proxied: %s", rr.Body.String())
	}
	// Drop only the deployment prefix to use the test router, then restore it to
	// verify the player's resolution of relative child URLs.
	routerWithPrefix := http.StripPrefix("/music", router)
	child, childURL := proxyHLSRequest(t, routerWithPrefix, base, ref, "GET")
	if child.Code != 200 {
		t.Fatalf("child: %d %s", child.Code, child.Body.String())
	}
	body := child.Body.String()
	if strings.Count(body, "segment?u=") != 5 {
		t.Fatalf("resources not all proxied: %s", body)
	}
	for _, line := range strings.Split(body, "\n") {
		var ref string
		if strings.HasPrefix(line, "segment?u=") {
			ref = line
		} else {
			_, attrs := parseAttrLine(line)
			for _, attr := range attrs {
				if attr.Key == "URI" {
					ref = attr.Value
				}
			}
		}
		if ref == "" {
			continue
		}
		resource, target := proxyHLSRequest(t, routerWithPrefix, childURL, ref, "GET")
		if !strings.HasPrefix(target.Path, "/music/api/v1/songs/") || target.Query().Get("access_token") != "TOKEN" {
			t.Errorf("prefix/token not preserved: %s", target)
		}
		if resource.Code != 206 || resource.Body.String() != "DATA" || resource.Header().Get("Content-Range") != "bytes 0-3/8" {
			t.Errorf("resource: %d %v %s", resource.Code, resource.Header(), resource.Body.String())
		}
		head, _ := proxyHLSRequest(t, routerWithPrefix, childURL, ref, "HEAD")
		if head.Code != 206 || head.Body.Len() != 0 {
			t.Errorf("HEAD: %d %s", head.Code, head.Body.String())
		}
	}
}

func TestHLSIntegration_CrossOriginAuthorization(t *testing.T) {
	song, h, router, hits := newCrossOriginHLSStack(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXTINF:5,\nhttp://1.1.1.1/seg.ts\n")
	}))
	base := mustURL(t, fmt.Sprintf("http://srv/api/v1/songs/%d/play.m3u8", song.ID))
	rr := httptest.NewRecorder()
	h.ServeProxy(rr, httptest.NewRequest("GET", base.String(), nil), song)
	ref := extractFirstURLByTag(t, rr.Body.String(), "hls/segment?u=")
	if ref == "" {
		t.Fatalf("cross-origin segment not proxied: %s", rr.Body.String())
	}
	signed := base.ResolveReference(mustURL(t, ref))
	for _, tc := range []struct {
		name   string
		change func(*url.URL)
	}{
		{"unsigned", func(u *url.URL) { q := u.Query(); q.Del("sig"); u.RawQuery = q.Encode() }},
		{"signature", func(u *url.URL) { q := u.Query(); q.Set("sig", "invalid"); u.RawQuery = q.Encode() }},
		{"target", func(u *url.URL) {
			q := u.Query()
			q.Set("u", "aHR0cDovLzEuMS4xLjEvb3RoZXIudHM")
			u.RawQuery = q.Encode()
		}},
		{"endpoint", func(u *url.URL) { u.Path = strings.Replace(u.Path, "/segment", "/playlist", 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := *signed
			tc.change(&target)
			before := hits.Load()
			got, _ := proxyHLSRequest(t, router, base, target.String(), "GET")
			if got.Code != 403 || hits.Load() != before {
				t.Errorf("unauthorized request: %d %s, hits=%d", got.Code, got.Body.String(), hits.Load()-before)
			}
		})
	}
	// A signature must not transfer to a different station, even one with the same source.
	other := *signed
	other.Path = strings.Replace(other.Path, fmt.Sprintf("/songs/%d/", song.ID), fmt.Sprintf("/songs/%d/", song.ID+1), 1)
	denied, _ := proxyHLSRequest(t, router, base, other.String(), "GET")
	if denied.Code != 403 {
		t.Errorf("signature transferred to another station: %d %s", denied.Code, denied.Body.String())
	}
	// Editing the source invalidates signatures issued for the former URL.
	song.URL = "http://8.8.8.8/changed.m3u8"
	if err := h.songService.Update(context.Background(), song); err != nil {
		t.Fatal(err)
	}
	denied, _ = proxyHLSRequest(t, router, base, signed.String(), "GET")
	if denied.Code != 403 {
		t.Errorf("signature survived a source edit: %d %s", denied.Code, denied.Body.String())
	}
}

func TestHLSIntegration_PrivateTargetsAndRedirects(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		redirect     bool
	}{
		{"private", "http://192.168.50.10/seg.ts", false},
		{"loopback", "http://127.0.0.1/seg.ts", false},
		{"metadata", "http://169.254.169.254/latest/meta-data", false},
		{"ipv6", "http://[::1]/seg.ts", false},
		{"redirect-segment", "http://127.0.0.1/private", true},
		{"redirect-playlist", "http://192.168.50.10/private.m3u8", true},
		{"redirect-scheme", "file:///etc/passwd", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			song, h, router, hits := newCrossOriginHLSStack(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host == "8.8.8.8" {
					target := tc.target
					if tc.redirect {
						target = "http://1.1.1.1/public.ts"
						if tc.name == "redirect-playlist" {
							target = "http://1.1.1.1/public.m3u8"
						}
					}
					fmt.Fprintf(w, "#EXTM3U\n#EXTINF:5,\n%s\n", target)
					return
				}
				http.Redirect(w, r, tc.target, 302)
			}))
			base := mustURL(t, fmt.Sprintf("http://srv/api/v1/songs/%d/play.m3u8", song.ID))
			rr := httptest.NewRecorder()
			h.ServeProxy(rr, httptest.NewRequest("GET", base.String(), nil), song)
			ref := extractFirstURLByTag(t, rr.Body.String(), "hls/")
			if ref == "" {
				t.Fatalf("resource not proxied: %s", rr.Body.String())
			}
			got, _ := proxyHLSRequest(t, router, base, ref, "GET")
			wantStatus, wantHits := 403, int64(1)
			if tc.redirect {
				wantStatus, wantHits = 502, 2
			}
			if got.Code != wantStatus || hits.Load() != wantHits {
				t.Errorf("got status=%d hits=%d body=%s; want %d/%d", got.Code, hits.Load(), got.Body.String(), wantStatus, wantHits)
			}
		})
	}
}

func TestRewriteM3U8_NonHTTPReferencesPreserved(t *testing.T) {
	input := "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://key\"\n#EXT-X-KEY:METHOD=AES-128,URI=\"data:application/octet-stream;base64,AAAA\"\nfile:///etc/passwd\n"
	output, err := rewriteM3U8([]byte(input), mustURL(t, "https://origin.example/live.m3u8"), fakeRewrite)
	if err != nil || string(output) != input {
		t.Fatalf("non-HTTP references changed: %s, %v", output, err)
	}
}

func TestHLSIntegration_CrossOriginPlaybackFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	segment, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-c:a", "aac", "-f", "mpegts", "pipe:1").Output()
	if err != nil {
		t.Fatalf("generate audio segment: %v", err)
	}
	song, h, router, hits := newCrossOriginHLSStack(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "8.8.8.8" {
			w.Header().Set("Content-Type", hlsContentType)
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXTINF:2.1,\nhttp://1.1.1.1/audio.ts\n#EXT-X-ENDLIST\n")
			return
		}
		w.Header().Set("Content-Type", "video/MP2T")
		w.Write(segment)
	}))
	playerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/play.m3u8") {
			h.ServeProxy(w, r, song)
			return
		}
		router.ServeHTTP(w, r)
	}))
	defer playerServer.Close()
	playURL := fmt.Sprintf("%s/api/v1/songs/%d/play.m3u8", playerServer.URL, song.ID)
	// Proxy endpoints have no media suffix. Newer FFmpeg security backports
	// require a second extension option; older builds expose only the first.
	args := []string{"-v", "error", "-allowed_extensions", "ALL"}
	help, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-h", "demuxer=hls").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(help), "allowed_segment_extensions") {
		args = append(args, "-allowed_segment_extensions", "ALL")
	}
	if strings.Contains(string(help), "extension_picky") {
		args = append(args, "-extension_picky", "0")
	}
	args = append(args, "-i", playURL, "-t", "2", "-ac", "1", "-ar", "8000", "-f", "s16le", "pipe:1")
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	pcm, err := cmd.Output()
	if err != nil || len(pcm) < 30000 || hits.Load() < 2 {
		t.Fatalf("decode proxied HLS: pcm=%d hits=%d err=%v stderr=%s", len(pcm), hits.Load(), err, stderr.String())
	}
}
