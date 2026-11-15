package jsplugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLynxFallbackRealmHeaders(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)
	plugin := &JSPlugin{Name: "Lynx", Version: "1.0.0", EntryPath: "lynx", Main: "main.js", RenderEngine: RenderEngineLynx, Status: JSPluginStatusActive, FilePath: "lynx.zip"}
	if err := repo.Create(context.Background(), plugin); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<head></head><lynx-view></lynx-view>"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(repo, pluginsDir, dataDir, "", nil, nil)
	t.Cleanup(func() { manager.Close() })
	rec := httptest.NewRecorder()
	if !manager.tryServeStaticFile(rec, httptest.NewRequest(http.MethodGet, "/api/v1/jsplugin/lynx/", nil), root, "index.html", "lynx") {
		t.Fatal("fallback not served")
	}
	if got := rec.Header().Get("Cross-Origin-Embedder-Policy"); got != "" {
		t.Fatalf("Lynx fallback must allow web-core srcdoc bootstrap, got %q", got)
	}
	for _, asset := range []string{"lynx-plugin-bridge-shim.js", "lynx-plugin-bridge-module.js"} {
		if assetVersions[asset] == "" {
			t.Fatalf("missing asset version: %s", asset)
		}
	}
}

// TestInjectHTMLHeadAssetVersioning 验证注入的每个公共资源 URL 都带内容哈希版本号（#278）。
// 若丢失版本号，jsplugin-assets 的 immutable 长缓存会让老浏览器永远拿不到资源更新。
func TestInjectHTMLHeadAssetVersioning(t *testing.T) {
	out := string(injectHTMLHead([]byte("<head></head><body></body>"), "demo", ""))

	// 三个公共资源（theme.css / components.css / common.js）均应带 ?v=<8位hex>，
	// 且版本号与嵌入内容哈希一致。
	for _, name := range []string{"theme.css", "components.css", "common.js"} {
		re := regexp.MustCompile(regexp.QuoteMeta(name) + `\?v=[0-9a-f]{8}"`)
		if !re.MatchString(out) {
			t.Errorf("注入的 %s 缺少版本号 (?v=hash)，实际输出:\n%s", name, out)
		}
		if v := assetVersions[name]; v == "" || !strings.Contains(out, name+"?v="+v) {
			t.Errorf("%s 版本号与嵌入内容哈希不一致: got version %q", name, v)
		}
	}
}

// TestAssetURLFallback 无对应资源版本时回退到无版本 URL，不产生裸 "?v=" 尾巴。
func TestAssetURLFallback(t *testing.T) {
	got := assetURL("/base/", "does-not-exist.css")
	if got != "/base/does-not-exist.css" {
		t.Errorf("未知资源应回退无版本 URL，got %q", got)
	}
}

// TestTryServeStaticFileCOEPHeader 验证插件 HTML 响应携带 COEP: credentialless 头。
//
// 宿主页选择 COEP: require-corp 时，跨源 iframe 要求其文档响应自身也带 COEP 头——缺失时
// Chrome/Firefox 直接 ERR_BLOCKED_BY_RESPONSE，插件 tab 呈空白且无应用层报错
// （corpMiddleware 的 CORP 头只覆盖子资源，盖不住 iframe 文档这一层）。
// 同时验证非 HTML 资源不受影响（强缓存路径保持原样）。
func TestTryServeStaticFileCOEPHeader(t *testing.T) {
	pluginsDir, dataDir, repo, _ := setupTestEnv(t)

	staticRoot := filepath.Join(dataDir, "demo", "static")
	if err := os.MkdirAll(staticRoot, 0o755); err != nil {
		t.Fatalf("create static dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticRoot, "index.html"), []byte("<head></head><body>hi</body>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticRoot, "style.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("write style.css: %v", err)
	}

	manager := NewManager(repo, pluginsDir, dataDir, "", nil, nil)
	t.Cleanup(func() { manager.Close() })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jsplugin/demo/", nil)
	if !manager.tryServeStaticFile(rec, req, staticRoot, "index.html", "demo") {
		t.Fatal("expected index.html to be served")
	}
	if got := rec.Header().Get("Cross-Origin-Embedder-Policy"); got != "credentialless" {
		t.Errorf("插件 HTML 响应应带 COEP: credentialless（Web 宿主页 iframe 嵌入依赖），got %q", got)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/jsplugin/demo/static/style.css", nil)
	if !manager.tryServeStaticFile(rec2, req2, staticRoot, "style.css", "demo") {
		t.Fatal("expected style.css to be served")
	}
	if got := rec2.Header().Get("Cross-Origin-Embedder-Policy"); got != "" {
		t.Errorf("非 HTML 资源不应带 COEP 头，got %q", got)
	}
}

// TestCommonJSThemeAppearanceBridge 防止主题包的非颜色参数在插件桥中丢失。
func TestCommonJSThemeAppearanceBridge(t *testing.T) {
	content, err := pluginAssets.ReadFile("assets/common.js")
	if err != nil {
		t.Fatalf("read common.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		"songloft-theme-appearance",
		"data-navigation-style",
		"--sl-theme-player-gradient",
		"--sl-theme-glass-fill",
		"--sl-theme-glass-border",
		"e.data.appearance || DEFAULT_THEME_APPEARANCE",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("common.js 缺少主题外观桥接片段 %q", want)
		}
	}
}
