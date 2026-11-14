package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"songloft/internal/models"
	"songloft/internal/services"

	"github.com/go-chi/chi/v5"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

// fakeRewrite 把绝对 URL 改为可识别的 sentinel，便于断言。
func fakeRewrite(absURL string, isPlaylist bool) string {
	if isPlaylist {
		return "PL[" + absURL + "]"
	}
	return "SEG[" + absURL + "]"
}

func TestParseAttrList_QuotedComma(t *testing.T) {
	// 引号内的逗号不能切分 —— 手写解析最易踩坑点
	got := parseAttrList(`BANDWIDTH=1280000,CODECS="avc1.42c01e,mp4a.40.2",NAME="x"`)
	want := []hlsAttr{
		{Key: "BANDWIDTH", Value: "1280000"},
		{Key: "CODECS", Value: "avc1.42c01e,mp4a.40.2", Quoted: true},
		{Key: "NAME", Value: "x", Quoted: true},
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (%+v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("attr[%d]: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestParseAttrLine_NonAttrList(t *testing.T) {
	// #EXTINF:5.0,title 不是 attribute list
	tag, attrs := parseAttrLine("#EXTINF:5.0,title")
	if tag != "EXTINF" {
		t.Errorf("tag: got %q want EXTINF", tag)
	}
	if attrs != nil {
		t.Errorf("attrs: expect nil for EXTINF, got %+v", attrs)
	}
}

func TestParseAttrLine_NoColon(t *testing.T) {
	tag, attrs := parseAttrLine("#EXTM3U")
	if tag != "EXTM3U" || attrs != nil {
		t.Errorf("got (%q, %+v)", tag, attrs)
	}
}

func TestFormatAttrLine_RoundTrip(t *testing.T) {
	in := `#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aac",NAME="en",DEFAULT=YES,URI="a.m3u8"`
	tag, attrs := parseAttrLine(in)
	out := formatAttrLine(tag, attrs)
	if out != in {
		t.Errorf("round trip:\n in:  %s\n out: %s", in, out)
	}
}

func TestRewriteM3U8_MissingExtM3U(t *testing.T) {
	_, err := rewriteM3U8([]byte("not a playlist\n"), mustURL(t, "http://up.example/p.m3u8"), fakeRewrite)
	if err == nil {
		t.Fatal("expected error for missing #EXTM3U")
	}
}

func TestRewriteM3U8_Empty(t *testing.T) {
	_, err := rewriteM3U8([]byte(""), mustURL(t, "http://up.example/p.m3u8"), fakeRewrite)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestRewriteM3U8_BOMAndCRLF(t *testing.T) {
	in := "\xEF\xBB\xBF#EXTM3U\r\n#EXTINF:5.0,\r\nseg1.ts\r\n"
	base := mustURL(t, "http://up.example/dir/p.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "SEG[http://up.example/dir/seg1.ts]") {
		t.Errorf("seg not rewritten: %s", got)
	}
	if strings.Contains(got, "\xEF\xBB\xBF") {
		t.Errorf("BOM should be stripped: %q", got)
	}
}

func TestRewriteM3U8_ThreeLineEndings(t *testing.T) {
	// 混合 \n / \r\n / \r 行尾，全部应能正确切分
	in := "#EXTM3U\n#EXTINF:5.0,\r\nseg1.ts\rseg2.ts\n"
	base := mustURL(t, "http://up.example/p.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"SEG[http://up.example/seg1.ts]",
		"SEG[http://up.example/seg2.ts]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRewriteM3U8_MasterPlaylist_StreamInf(t *testing.T) {
	in := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1280000,CODECS="avc1.42c01e,mp4a.40.2"
720p/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2560000
1080p/index.m3u8
`
	base := mustURL(t, "http://up.example/master.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"PL[http://up.example/720p/index.m3u8]",
		"PL[http://up.example/1080p/index.m3u8]",
		// CODECS 必须原样保留（含逗号）
		`CODECS="avc1.42c01e,mp4a.40.2"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRewriteM3U8_NakedPlaylistLine(t *testing.T) {
	// 少数源会把子 playlist 作为裸 URL 行输出，但缺少 EXT-X-STREAM-INF。
	// 这种情况下仍应走 playlist 端点，否则 player 会把返回的 m3u8 当成切片处理。
	in := `#EXTM3U
audio/index.m3u8
#EXTINF:5.0,
seg.ts
`
	base := mustURL(t, "http://up.example/live/master.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "PL[http://up.example/live/audio/index.m3u8]") {
		t.Errorf("naked m3u8 should be rewritten as playlist:\n%s", got)
	}
	if !strings.Contains(got, "SEG[http://up.example/live/seg.ts]") {
		t.Errorf("normal segment should stay segment:\n%s", got)
	}
}

func TestRewriteM3U8_MediaTag(t *testing.T) {
	in := `#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aac",NAME="en",DEFAULT=YES,URI="audio/en.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="sub",NAME="cc",URI="sub/cc.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=1000000,AUDIO="aac",SUBTITLES="sub"
video.m3u8
`
	base := mustURL(t, "http://up.example/master.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		`URI="PL[http://up.example/audio/en.m3u8]"`,
		`URI="PL[http://up.example/sub/cc.m3u8]"`,
		"PL[http://up.example/video.m3u8]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRewriteM3U8_KeyAndMapAndSegments(t *testing.T) {
	in := `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:5
#EXT-X-MAP:URI="init.mp4"
#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x00112233
#EXTINF:5.0,
seg-0.m4s
#EXTINF:5.0,
seg-1.m4s
#EXT-X-ENDLIST
`
	base := mustURL(t, "http://up.example/media/")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		`URI="SEG[http://up.example/media/init.mp4]"`,
		`URI="SEG[http://up.example/media/key.bin]"`,
		"SEG[http://up.example/media/seg-0.m4s]",
		"SEG[http://up.example/media/seg-1.m4s]",
		"IV=0x00112233",  // 未识别属性原样
		"#EXT-X-ENDLIST", // 未识别 tag 原样
		"#EXT-X-VERSION:6",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRewriteM3U8_KeyMethodNone(t *testing.T) {
	// METHOD=NONE 时 EXT-X-KEY 没有 URI，不应崩
	in := `#EXTM3U
#EXT-X-KEY:METHOD=NONE
#EXTINF:5.0,
seg.ts
`
	base := mustURL(t, "http://up.example/p.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "#EXT-X-KEY:METHOD=NONE") {
		t.Errorf("EXT-X-KEY:METHOD=NONE should pass through:\n%s", got)
	}
}

func TestRewriteM3U8_LLHLS(t *testing.T) {
	in := `#EXTM3U
#EXT-X-VERSION:9
#EXT-X-TARGETDURATION:4
#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=3.0
#EXT-X-PART-INF:PART-TARGET=1.0
#EXT-X-PART:DURATION=1.0,URI="part-0.0.m4s"
#EXT-X-PART:DURATION=1.0,URI="part-0.1.m4s",INDEPENDENT=YES
#EXT-X-PRELOAD-HINT:TYPE=PART,URI="part-0.2.m4s"
#EXT-X-RENDITION-REPORT:URI="audio.m3u8",LAST-MSN=10,LAST-PART=2
`
	base := mustURL(t, "http://up.example/live/")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		`URI="SEG[http://up.example/live/part-0.0.m4s]"`,
		`URI="SEG[http://up.example/live/part-0.1.m4s]"`,
		`URI="SEG[http://up.example/live/part-0.2.m4s]"`,
		`URI="PL[http://up.example/live/audio.m3u8]"`,
		`INDEPENDENT=YES`, // 未识别属性原样保留
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRewriteM3U8_AbsoluteAndRelative(t *testing.T) {
	in := `#EXTM3U
#EXTINF:5.0,
http://up.example/abs.ts
#EXTINF:5.0,
./rel.ts
#EXTINF:5.0,
../up.ts
#EXTINF:5.0,
/root.ts
`
	base := mustURL(t, "http://up.example/a/b/p.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"SEG[http://up.example/abs.ts]",
		"SEG[http://up.example/a/b/rel.ts]",
		"SEG[http://up.example/a/up.ts]",
		"SEG[http://up.example/root.ts]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRewriteM3U8_CrossOriginRewritten(t *testing.T) {
	in := `#EXTM3U
#EXTINF:5.0,
http://other.example/seg.ts
#EXT-X-KEY:METHOD=AES-128,URI="http://other.example/key.bin"
`
	base := mustURL(t, "http://up.example/p.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "SEG[http://other.example/seg.ts]") {
		t.Errorf("cross-origin seg should be proxied:\n%s", got)
	}
	if !strings.Contains(got, `URI="SEG[http://other.example/key.bin]"`) {
		t.Errorf("cross-origin key should be proxied:\n%s", got)
	}
}

func TestRewriteM3U8_DifferentPort(t *testing.T) {
	// 同 host 不同 port 的 CDN 资源也需要代理。
	in := `#EXTM3U
#EXTINF:5.0,
http://up.example:8443/seg.ts
`
	base := mustURL(t, "http://up.example/p.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "SEG[http://up.example:8443/seg.ts]") {
		t.Errorf("different port should be proxied:\n%s", out)
	}
}

func TestRewriteM3U8_DateRangeAssetURI(t *testing.T) {
	in := `#EXTM3U
#EXT-X-DATERANGE:ID="ad1",START-DATE="2026-01-01T00:00:00Z",DURATION=15.0,X-ASSET-URI="ad/sub.m3u8"
#EXT-X-DATERANGE:ID="ad2",START-DATE="2026-01-01T00:00:30Z",X-ASSET-URI="ad/seg.ts"
#EXT-X-DATERANGE:ID="ad3",START-DATE="2026-01-01T00:01:00Z",X-ASSET-LIST="ad/list.json"
`
	base := mustURL(t, "http://up.example/main.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	// X-ASSET-URI 指向 .m3u8 → playlist 端点
	if !strings.Contains(got, `X-ASSET-URI="PL[http://up.example/ad/sub.m3u8]"`) {
		t.Errorf("X-ASSET-URI -> m3u8 should go to playlist endpoint:\n%s", got)
	}
	// X-ASSET-URI 指向 .ts → segment 端点
	if !strings.Contains(got, `X-ASSET-URI="SEG[http://up.example/ad/seg.ts]"`) {
		t.Errorf("X-ASSET-URI -> ts should go to segment endpoint:\n%s", got)
	}
	// X-ASSET-LIST 原样透传不改写（MVP 不支持 JSON 子代理）
	if !strings.Contains(got, `X-ASSET-LIST="ad/list.json"`) {
		t.Errorf("X-ASSET-LIST should pass through unchanged:\n%s", got)
	}
}

func TestRewriteM3U8_SessionKeyAndSessionData(t *testing.T) {
	in := `#EXTM3U
#EXT-X-SESSION-KEY:METHOD=AES-128,URI="skey.bin"
#EXT-X-SESSION-DATA:DATA-ID="com.x.title",URI="meta.json"
#EXT-X-STREAM-INF:BANDWIDTH=1000000
v.m3u8
`
	base := mustURL(t, "http://up.example/m.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		`URI="SEG[http://up.example/skey.bin]"`,
		`URI="SEG[http://up.example/meta.json]"`,
		"PL[http://up.example/v.m3u8]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRewriteM3U8_IFrameStreamInf(t *testing.T) {
	in := `#EXTM3U
#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=200000,URI="iframes/index.m3u8"
`
	base := mustURL(t, "http://up.example/m.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `URI="PL[http://up.example/iframes/index.m3u8]"`) {
		t.Errorf("I-FRAME-STREAM-INF URI not rewritten:\n%s", out)
	}
}

func TestRewriteM3U8_ByteRangePreserved(t *testing.T) {
	// EXT-X-BYTERANGE 是元数据，原样透传；前面的 segment URL 仍要改写
	in := `#EXTM3U
#EXT-X-VERSION:4
#EXTINF:5.0,
#EXT-X-BYTERANGE:1000@0
seg.ts
`
	base := mustURL(t, "http://up.example/m.m3u8")
	out, err := rewriteM3U8([]byte(in), base, fakeRewrite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "#EXT-X-BYTERANGE:1000@0") {
		t.Errorf("BYTERANGE should pass through:\n%s", got)
	}
	if !strings.Contains(got, "SEG[http://up.example/seg.ts]") {
		t.Errorf("seg should be rewritten:\n%s", got)
	}
}

func TestLooksLikePlaylist(t *testing.T) {
	cases := map[string]bool{
		"http://x/p.m3u8":     true,
		"http://x/p.m3u":      true,
		"http://x/P.M3U8":     true, // 大小写
		"http://x/p.m3u8?t=1": true, // 忽略 query
		"http://x/seg.ts":     false,
		"http://x/init.mp4":   false,
		"http://x/key.bin":    false,
	}
	for in, want := range cases {
		if got := looksLikePlaylist(in); got != want {
			t.Errorf("looksLikePlaylist(%q) = %v, want %v", in, got, want)
		}
	}
}

// ============================================================
// 集成测试：mock 上游 master→media→segment 三跳
// ============================================================

// newMockUpstream 起一个 mock 上游，模拟 HLS 主播放列表 → 子播放列表 → 切片三跳。
// 所有路径都用相对 URL（HLS 常见做法），由 player resolve 到 absolute。
func newMockUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/live/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", hlsContentType)
		fmt.Fprint(w, `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1280000
720p/index.m3u8
`)
	})
	mux.HandleFunc("/live/720p/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", hlsContentType)
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:5
#EXT-X-MAP:URI="init.mp4"
#EXTINF:5.0,
seg-0.m4s
#EXTINF:5.0,
seg-1.m4s
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/live/720p/seg-0.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/iso.segment")
		w.Header().Set("Content-Length", "8")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Write([]byte("seg-0-OK"))
	})
	mux.HandleFunc("/live/720p/init.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("INITDATA"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newTestHLSStack 构造 song + HLSHandler + chi router，方便集成测试。
func newTestHLSStack(t *testing.T, songURL string) (songID int64, handler *HLSHandler, router chi.Router) {
	t.Helper()
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	id := seedSong(t, repo, &models.Song{
		Type:  models.TypeRadio,
		Title: "test radio",
		URL:   songURL,
	})
	// configService 传 nil：本测试栈不走 IsEnabled 判定，直接调 ServeProxy/HandlePlaylist/HandleSegment
	h := NewHLSHandler(songService, nil)
	// 测试时上游用 httptest.Server（127.0.0.1，正常会被 IsHostnameAllowed 拦掉）；
	// 注入 always-allow 让同源校验做主防线
	h.allowHost = func(string) bool { return true }
	r := chi.NewRouter()
	r.Get("/api/v1/songs/{id}/hls/playlist", h.HandlePlaylist)
	r.Head("/api/v1/songs/{id}/hls/playlist", h.HandlePlaylist)
	r.Get("/api/v1/songs/{id}/hls/segment", h.HandleSegment)
	r.Head("/api/v1/songs/{id}/hls/segment", h.HandleSegment)
	return id, h, r
}

func TestHLSIntegration_ServeProxyRewritesMasterPlaylist(t *testing.T) {
	upstream := newMockUpstream(t)
	songURL := upstream.URL + "/live/master.m3u8"
	songID, handler, _ := newTestHLSStack(t, songURL)
	_ = songID

	// 模拟 serveRadio 调 ServeProxy：直接进入 HLSHandler 顶层入口
	song := &models.Song{ID: songID, URL: songURL, Type: models.TypeRadio}
	req := httptest.NewRequest("GET", "/api/v1/songs/"+strconv.FormatInt(songID, 10)+"/play", nil)
	rr := httptest.NewRecorder()
	handler.ServeProxy(rr, req, song)

	if rr.Code != http.StatusOK {
		t.Fatalf("ServeProxy status: got %d want 200, body=%q", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != hlsContentType {
		t.Errorf("Content-Type: got %q want %q", ct, hlsContentType)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q want no-store", cc)
	}

	body := rr.Body.String()
	// master 内的 720p/index.m3u8 被改写为相对路径 "hls/playlist?u=..."
	// 顶层 servePlaylist 必须带 "hls/" 前缀，否则 player 按 RFC 3986 解析后
	// 落到 /api/v1/songs/{id}/playlist 而非 /api/v1/songs/{id}/hls/playlist
	if !strings.Contains(body, "hls/playlist?u=") {
		t.Errorf("master playlist not rewritten to hls/playlist endpoint:\n%s", body)
	}
	if strings.Count(body, "hls/playlist?u=") != 1 {
		t.Errorf("unexpected hls/playlist rewrite count:\n%s", body)
	}
}

func TestHLSIntegration_PlayerFollowsRewrittenChain(t *testing.T) {
	upstream := newMockUpstream(t)
	songURL := upstream.URL + "/live/master.m3u8"
	songID, handler, router := newTestHLSStack(t, songURL)

	// === Step 1: master playlist 改写（顶层 base URL = /api/v1/songs/{id}/play）===
	song := &models.Song{ID: songID, URL: songURL, Type: models.TypeRadio}
	topBaseURL := mustURL(t, "http://srv/api/v1/songs/"+strconv.FormatInt(songID, 10)+"/play")
	rr1 := httptest.NewRecorder()
	handler.ServeProxy(rr1, httptest.NewRequest("GET", topBaseURL.String(), nil), song)
	if rr1.Code != 200 {
		t.Fatalf("master: %d %s", rr1.Code, rr1.Body.String())
	}

	// 从改写后的 master 中提取 media playlist 的相对 URL（带 "hls/" 前缀）
	mediaProxyRef := extractFirstURLByTag(t, rr1.Body.String(), "hls/playlist?u=")
	if mediaProxyRef == "" {
		t.Fatal("could not find rewritten media playlist URL in master")
	}

	// === Step 2: 模拟 player 用顶层 base URL 解析相对路径（真实 RFC 3986 行为）===
	// 期望解析为 /api/v1/songs/{id}/hls/playlist?u=...
	mediaAbs := topBaseURL.ResolveReference(mustURL(t, mediaProxyRef))
	wantMediaPath := "/api/v1/songs/" + strconv.FormatInt(songID, 10) + "/hls/playlist"
	if mediaAbs.Path != wantMediaPath {
		t.Fatalf("media path resolved wrong: got %q want %q", mediaAbs.Path, wantMediaPath)
	}
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, httptest.NewRequest("GET", mediaAbs.RequestURI(), nil))
	if rr2.Code != 200 {
		t.Fatalf("media: %d %s", rr2.Code, rr2.Body.String())
	}
	mediaBody := rr2.Body.String()
	// 子层 m3u8 应改写为同目录相对路径 segment?u=...（不带 "hls/" 前缀）
	if c := strings.Count(mediaBody, "segment?u="); c != 3 {
		t.Errorf("media should contain 3 segment refs, got %d:\n%s", c, mediaBody)
	}
	if strings.Contains(mediaBody, "hls/segment?u=") {
		t.Errorf("media playlist should not contain hls/ prefix (子层同目录解析):\n%s", mediaBody)
	}

	// === Step 3: 模拟 player 在 media playlist URL 上解析 segment 相对路径 ===
	segRef := extractFirstURLByTag(t, mediaBody, "segment?u=")
	if segRef == "" {
		t.Fatal("could not find rewritten segment URL in media playlist")
	}
	segAbs := mediaAbs.ResolveReference(mustURL(t, segRef))
	wantSegPath := "/api/v1/songs/" + strconv.FormatInt(songID, 10) + "/hls/segment"
	if segAbs.Path != wantSegPath {
		t.Fatalf("segment path resolved wrong: got %q want %q", segAbs.Path, wantSegPath)
	}
	rr3 := httptest.NewRecorder()
	router.ServeHTTP(rr3, httptest.NewRequest("GET", segAbs.RequestURI(), nil))
	if rr3.Code != 200 {
		t.Fatalf("segment: %d %s", rr3.Code, rr3.Body.String())
	}
	if rr3.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("segment Cache-Control should be no-store, got %q", rr3.Header().Get("Cache-Control"))
	}
	// init.mp4 是 segment 里的第一个被改写 URI（EXT-X-MAP），切片是 init.mp4 字节
	expected := "INITDATA"
	if got := rr3.Body.String(); got != expected {
		t.Errorf("segment body: got %q want %q", got, expected)
	}
}

// TestHLSIntegration_AccessTokenPropagated 验证原始请求里的 access_token
// 被注入到所有改写后的子 URL 中（顶层 + 子层都要透传）。
func TestHLSIntegration_AccessTokenPropagated(t *testing.T) {
	upstream := newMockUpstream(t)
	songURL := upstream.URL + "/live/master.m3u8"
	songID, handler, router := newTestHLSStack(t, songURL)

	// === Step 1: 带 access_token 调顶层入口 ===
	song := &models.Song{ID: songID, URL: songURL, Type: models.TypeRadio}
	topURL := "http://srv/api/v1/songs/" + strconv.FormatInt(songID, 10) + "/play?access_token=TOKEN-AAA&format=mp3"
	rr1 := httptest.NewRecorder()
	handler.ServeProxy(rr1, httptest.NewRequest("GET", topURL, nil), song)
	if rr1.Code != 200 {
		t.Fatalf("master: %d %s", rr1.Code, rr1.Body.String())
	}
	if !strings.Contains(rr1.Body.String(), "access_token=TOKEN-AAA") {
		t.Errorf("master m3u8 must propagate access_token:\n%s", rr1.Body.String())
	}

	// === Step 2: 子层入口收到 access_token 后也要继续透传 ===
	mediaRef := extractFirstURLByTag(t, rr1.Body.String(), "hls/playlist?u=")
	mediaAbs := mustURL(t, topURL).ResolveReference(mustURL(t, mediaRef))
	// player 跟随相对路径时 base 的 query 不继承，access_token 是从改写出的 URL 自带的
	if mediaAbs.Query().Get("access_token") != "TOKEN-AAA" {
		t.Fatalf("media URL must carry access_token, got: %s", mediaAbs.String())
	}
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, httptest.NewRequest("GET", mediaAbs.RequestURI(), nil))
	if rr2.Code != 200 {
		t.Fatalf("media: %d %s", rr2.Code, rr2.Body.String())
	}
	if !strings.Contains(rr2.Body.String(), "access_token=TOKEN-AAA") {
		t.Errorf("media m3u8 must propagate access_token to segment URLs:\n%s", rr2.Body.String())
	}
}

func TestHLSIntegration_CrossOriginRejected(t *testing.T) {
	upstream := newMockUpstream(t)
	songURL := upstream.URL + "/live/master.m3u8"
	songID, _, router := newTestHLSStack(t, songURL)

	// 构造一个指向 evil.example 的 segment 请求 —— 同源校验应该拦截
	evilURL := "http://evil.example/leak.ts"
	encoded := base64.RawURLEncoding.EncodeToString([]byte(evilURL))
	path := "/api/v1/songs/" + strconv.FormatInt(songID, 10) + "/hls/segment?u=" + encoded

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
	if rr.Code != http.StatusForbidden {
		t.Errorf("cross-origin should be 403, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestHLSIntegration_BadBase64Rejected(t *testing.T) {
	upstream := newMockUpstream(t)
	songURL := upstream.URL + "/live/master.m3u8"
	songID, _, router := newTestHLSStack(t, songURL)

	path := "/api/v1/songs/" + strconv.FormatInt(songID, 10) + "/hls/segment?u=NOT_BASE64!!!"
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad base64 should be 400, got %d", rr.Code)
	}
}

func TestHLSIntegration_UpstreamErrorPassThrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/bad.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "upstream gone")
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	songURL := upstream.URL + "/bad.m3u8"
	songID, handler, _ := newTestHLSStack(t, songURL)

	song := &models.Song{ID: songID, URL: songURL, Type: models.TypeRadio}
	rr := httptest.NewRecorder()
	handler.ServeProxy(rr, httptest.NewRequest("GET", "/x", nil), song)

	// 上游 404 应该原样透传给 player
	if rr.Code != http.StatusNotFound {
		t.Errorf("upstream 404 should pass through, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "upstream gone") {
		t.Errorf("upstream body should pass through, got %q", rr.Body.String())
	}
}

func TestHLSIntegration_HeadRequest(t *testing.T) {
	upstream := newMockUpstream(t)
	songURL := upstream.URL + "/live/master.m3u8"
	songID, _, router := newTestHLSStack(t, songURL)

	encoded := base64.RawURLEncoding.EncodeToString([]byte(songURL))
	path := "/api/v1/songs/" + strconv.FormatInt(songID, 10) + "/hls/playlist?u=" + encoded
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest("HEAD", path, nil))
	if rr.Code != 200 {
		t.Fatalf("HEAD playlist: %d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Type") != hlsContentType {
		t.Errorf("HEAD Content-Type: got %q", rr.Header().Get("Content-Type"))
	}
}

// extractFirstURLByTag 从 m3u8 文本中找包含 needle 的第一个 token（去引号、去前缀路径）。
// 用于断言改写后的相对 URL，返回 "playlist?u=..." 或 "segment?u=..."
func extractFirstURLByTag(t *testing.T, content, needle string) string {
	t.Helper()
	idx := strings.Index(content, needle)
	if idx < 0 {
		return ""
	}
	// 向前找单词起点（空格 / 引号 / 行首）
	start := idx
	for start > 0 && content[start-1] != ' ' && content[start-1] != '"' && content[start-1] != '\n' && content[start-1] != '=' {
		start--
	}
	// 向后找单词结束
	end := idx + len(needle)
	for end < len(content) && content[end] != '"' && content[end] != '\n' && content[end] != ' ' {
		end++
	}
	return content[start:end]
}

func TestIsSameOrigin(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"http://x.com/a", "http://x.com/b", true},
		{"http://X.com/a", "http://x.COM/b", true},       // 大小写不敏感
		{"http://x.com/a", "https://x.com/a", false},     // scheme
		{"http://x.com/a", "http://y.com/a", false},      // host
		{"http://x.com/a", "http://x.com:8443/a", false}, // port
	}
	for _, c := range cases {
		got := isSameOrigin(mustURL(t, c.a), mustURL(t, c.b))
		if got != c.want {
			t.Errorf("isSameOrigin(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
