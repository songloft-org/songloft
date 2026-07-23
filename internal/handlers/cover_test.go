package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// writeTestPNG 生成一张 w×h 纯色 PNG 到临时文件，返回路径。
func writeTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 220, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "cover.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建测试 PNG 失败：%v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("编码测试 PNG 失败：%v", err)
	}
	return path
}

func TestDecodeAndResizeCover_Downscale(t *testing.T) {
	path := writeTestPNG(t, 600, 400)

	data, srcW, srcH, dstW, dstH, resized, err := decodeAndResizeCover(path, 100)
	if err != nil {
		t.Fatalf("缩放失败：%v", err)
	}
	if srcW != 600 || srcH != 400 {
		t.Fatalf("原始尺寸错误：%dx%d", srcW, srcH)
	}
	if !resized {
		t.Fatal("期望发生缩放")
	}
	if dstW != 100 {
		t.Fatalf("目标宽度错误：%d", dstW)
	}
	// 等比：400 * 100/600 ≈ 67
	if dstH < 66 || dstH > 68 {
		t.Fatalf("目标高度未等比：%d", dstH)
	}
	// 产物应为可解码 JPEG，且尺寸与 dst 一致。
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("产物无法解码：%v", err)
	}
	if format != "jpeg" {
		t.Fatalf("产物格式应为 jpeg，实际 %q", format)
	}
	if cfg.Width != dstW || cfg.Height != dstH {
		t.Fatalf("产物尺寸不符：%dx%d vs %dx%d", cfg.Width, cfg.Height, dstW, dstH)
	}
}

func TestDecodeAndResizeCover_NeverUpscale(t *testing.T) {
	path := writeTestPNG(t, 80, 80)

	data, _, _, dstW, dstH, resized, err := decodeAndResizeCover(path, 400)
	if err != nil {
		t.Fatalf("处理失败：%v", err)
	}
	if resized {
		t.Fatal("原图小于目标宽度时不应放大")
	}
	if dstW != 80 || dstH != 80 {
		t.Fatalf("尺寸应保持原样：%dx%d", dstW, dstH)
	}
	// 仍应是合法 JPEG（统一重编码）。
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("产物应为合法 JPEG：%v", err)
	}
}

func TestServeCoverFile_NoWidthServesOriginal(t *testing.T) {
	path := writeTestPNG(t, 200, 200)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1/cover", nil)
	rec := httptest.NewRecorder()
	serveCoverFile(rec, req, path)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实际 %d", rec.Code)
	}
	// 无 w：原图直出，Content-Type 应是 PNG（由 ServeFile 嗅探）。
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("无 w 应原图直出 PNG，实际 Content-Type=%q", ct)
	}
}

func TestServeCoverFile_WidthReturnsJPEG(t *testing.T) {
	path := writeTestPNG(t, 500, 500)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1/cover?w=120", nil)
	rec := httptest.NewRecorder()
	serveCoverFile(rec, req, path)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实际 %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("带 w 应返回 JPEG，实际 Content-Type=%q", ct)
	}
	if rec.Header().Get("ETag") == "" {
		t.Fatal("带 w 应设置 ETag")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("响应体无法解码：%v", err)
	}
	if cfg.Width != 120 {
		t.Fatalf("缩略宽度应为 120，实际 %d", cfg.Width)
	}
}

func TestServeCoverFile_ETag304(t *testing.T) {
	path := writeTestPNG(t, 500, 500)

	// 首次请求拿 ETag。
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1/cover?w=120", nil)
	rec1 := httptest.NewRecorder()
	serveCoverFile(rec1, req1, path)
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("首次请求应返回 ETag")
	}

	// 携带 If-None-Match 复请求，应 304 且无响应体。
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1/cover?w=120", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	serveCoverFile(rec2, req2, path)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("ETag 命中应 304，实际 %d", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("304 不应有响应体，实际 %d 字节", rec2.Body.Len())
	}
}

func TestServeCoverFile_UnsupportedFormatFallsBack(t *testing.T) {
	// 写一个非图片文件，模拟无法解码（如 webp 未注册解码器 / 损坏）。
	path := filepath.Join(t.TempDir(), "broken.jpg")
	if err := os.WriteFile(path, []byte("not-a-real-image"), 0644); err != nil {
		t.Fatalf("写测试文件失败：%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1/cover?w=120", nil)
	rec := httptest.NewRecorder()
	serveCoverFile(rec, req, path)

	// 解码失败应优雅回退为原图直出（ServeFile），而非 500。
	if rec.Code != http.StatusOK {
		t.Fatalf("解码失败应回退原图直出 200，实际 %d", rec.Code)
	}
	if rec.Header().Get("ETag") != "" {
		t.Fatal("回退原图直出后不应残留缩略 ETag")
	}
}

func TestEtagMatches(t *testing.T) {
	etag := "\"abc123\""
	cases := []struct {
		header string
		want   bool
	}{
		{etag, true},
		{"*", true},
		{"W/" + etag, true},
		{"\"other\", " + etag, true},
		{"\"other\"", false},
		{"", false},
	}
	for _, c := range cases {
		if got := etagMatches(c.header, etag); got != c.want {
			t.Errorf("etagMatches(%q) = %v，期望 %v", c.header, got, c.want)
		}
	}
}
