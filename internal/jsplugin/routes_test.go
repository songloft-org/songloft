package jsplugin

import (
	"regexp"
	"strings"
	"testing"
)

// TestInjectHTMLHeadAssetVersioning 验证注入的 common.css / common.js URL 带内容哈希版本号（#278）。
// 若丢失版本号，jsplugin-assets 的 immutable 长缓存会让老浏览器永远拿不到 common.css 更新。
func TestInjectHTMLHeadAssetVersioning(t *testing.T) {
	out := string(injectHTMLHead([]byte("<head></head><body></body>"), "demo", ""))

	// common.css / common.js 均应带 ?v=<8位hex>
	reCSS := regexp.MustCompile(`common\.css\?v=[0-9a-f]{8}"`)
	if !reCSS.MatchString(out) {
		t.Errorf("注入的 common.css 缺少版本号 (?v=hash)，实际输出:\n%s", out)
	}
	reJS := regexp.MustCompile(`common\.js\?v=[0-9a-f]{8}"`)
	if !reJS.MatchString(out) {
		t.Errorf("注入的 common.js 缺少版本号 (?v=hash)，实际输出:\n%s", out)
	}

	// 版本号应与实际嵌入内容的哈希一致
	if v := assetVersions["common.css"]; v == "" || !strings.Contains(out, "common.css?v="+v) {
		t.Errorf("common.css 版本号与嵌入内容哈希不一致: got version %q", v)
	}
}

// TestAssetURLFallback 无对应资源版本时回退到无版本 URL，不产生裸 "?v=" 尾巴。
func TestAssetURLFallback(t *testing.T) {
	got := assetURL("/base/", "does-not-exist.css")
	if got != "/base/does-not-exist.css" {
		t.Errorf("未知资源应回退无版本 URL，got %q", got)
	}
}
