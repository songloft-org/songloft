package handlers

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	// 注册常见封面格式的解码器（jpeg 已随 image/jpeg 注册，这里补 png/gif）。
	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"
)

// coverThumbMaxWidth 是 ?w= 允许的最大目标宽度（物理像素）。
// 超大值会退化成"按原图服务"，避免被恶意放大耗 CPU/内存。封面显示场景 1024 足够。
const coverThumbMaxWidth = 1024

// coverThumbJPEGQuality 缩略图 JPEG 编码质量。封面为装饰性小图，85 在体积/画质间平衡。
const coverThumbJPEGQuality = 85

// serveCoverFile 统一的本地封面文件服务。
//
// 无 ?w=（或非法/超限）时：等价于旧行为，直接 http.ServeFile 原图（保留 ETag/Range/304）。
//
// 有 ?w=N 时：把封面等比缩放到目标宽度 N（物理像素，绝不放大）后以 JPEG 返回。
// 这是 songloft-org/songloft#309 的服务端配合：Web 端封面改回浏览器原生 <img>
// （ImageRenderMethodForWeb.HtmlImage）路径以规避 HttpGet + flutter_cache_manager
// web 内存管线的"滚回/队列重建时静默 stall"重显示 bug；而 <img> 会按图片"固有尺寸"
// 上传 GPU 纹理、memCacheWidth 在该路径不生效，故改由服务端把封面缩到显示尺寸，
// 既拿回浏览器缓存的稳健重显示，又保住移动端小纹理（不再顶爆 WebGL 显存变黑）。
//
// 缓存：命中 If-None-Match 直接 304（不解码，最省）；否则按需解码+缩放+编码。
// max-age=1 年，浏览器侧每张缩略图基本只请求一次。诊断信息（是否缩放、原始/目标尺寸、
// 回退原因、耗时）全部打进 slog，便于排查（songloft-org/songloft#309）。
func serveCoverFile(w http.ResponseWriter, r *http.Request, path string) {
	widthStr := strings.TrimSpace(r.URL.Query().Get("w"))
	if widthStr == "" {
		// 无缩略请求：老路径，直接服务原图。
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		http.ServeFile(w, r, path)
		return
	}

	width, err := strconv.Atoi(widthStr)
	if err != nil || width <= 0 {
		slog.Debug("封面缩略参数非法，回退原图", "path", path, "w", widthStr, "error", err)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		http.ServeFile(w, r, path)
		return
	}
	if width > coverThumbMaxWidth {
		slog.Debug("封面缩略宽度超限，收敛到上限", "path", path, "requested", width, "max", coverThumbMaxWidth)
		width = coverThumbMaxWidth
	}

	info, err := os.Stat(path)
	if err != nil {
		slog.Warn("封面文件不可读，返回 404", "path", path, "error", err)
		http.Error(w, "cover not found", http.StatusNotFound)
		return
	}

	// ETag 只依赖 路径+修改时间+大小+目标宽度，不需解码即可算——命中 If-None-Match 时
	// 直接 304，跳过解码/缩放/编码这条最贵的路径。
	etag := coverThumbETag(path, info, width)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, etag) {
		slog.Debug("封面缩略 ETag 命中，返回 304", "path", path, "w", width)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	start := time.Now()
	data, srcW, srcH, dstW, dstH, resized, err := decodeAndResizeCover(path, width)
	if err != nil {
		// 解码失败（如 webp 等未注册解码器、损坏文件）：优雅降级为原图直出，不阻塞显示。
		slog.Warn("封面解码/缩放失败，回退原图直出",
			"path", path, "w", width, "error", err)
		// 已写过 ETag，但原图直出用 ServeFile 会自带自己的校验，这里删掉避免语义冲突。
		w.Header().Del("ETag")
		http.ServeFile(w, r, path)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if resized {
		slog.Debug("封面缩略已生成",
			"path", path, "src", fmt.Sprintf("%dx%d", srcW, srcH),
			"dst", fmt.Sprintf("%dx%d", dstW, dstH), "bytes", len(data),
			"dur_ms", float64(time.Since(start).Microseconds())/1000.0)
	} else {
		slog.Debug("封面原图不大于目标宽度，重编码直出（未放大）",
			"path", path, "src", fmt.Sprintf("%dx%d", srcW, srcH), "w", width,
			"bytes", len(data), "dur_ms", float64(time.Since(start).Microseconds())/1000.0)
	}
	if _, err := w.Write(data); err != nil {
		slog.Debug("封面缩略写出失败（客户端可能已断开）", "path", path, "error", err)
	}
}

// decodeAndResizeCover 解码 path 处的封面并等比缩放到宽度 targetW（绝不放大），
// 返回 JPEG 字节及原始/目标尺寸、是否发生了缩放。
func decodeAndResizeCover(path string, targetW int) (data []byte, srcW, srcH, dstW, dstH int, resized bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, 0, 0, false, fmt.Errorf("打开封面失败：%w", err)
	}
	defer f.Close()

	src, format, err := image.Decode(f)
	if err != nil {
		return nil, 0, 0, 0, 0, false, fmt.Errorf("解码封面失败（格式 %q）：%w", format, err)
	}

	b := src.Bounds()
	srcW, srcH = b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, srcW, srcH, 0, 0, false, fmt.Errorf("封面尺寸非法：%dx%d", srcW, srcH)
	}

	// 绝不放大：原图不宽于目标就按原尺寸重编码（统一成 JPEG，texture 也就是原尺寸）。
	dstW, dstH = srcW, srcH
	if srcW > targetW {
		dstW = targetW
		dstH = max(int(float64(srcH)*float64(targetW)/float64(srcW)), 1)
		resized = true
	}

	out := src
	if resized {
		dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
		// CatmullRom：高质量缩放，封面小图这点开销可忽略。
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
		out = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: coverThumbJPEGQuality}); err != nil {
		return nil, srcW, srcH, dstW, dstH, resized, fmt.Errorf("编码 JPEG 失败：%w", err)
	}
	return buf.Bytes(), srcW, srcH, dstW, dstH, resized, nil
}

// coverThumbETag 由文件身份（路径+mtime+size）与目标宽度派生一个强 ETag。
func coverThumbETag(path string, info os.FileInfo, width int) string {
	h := sha1.Sum(fmt.Appendf(nil, "%s|%d|%d|w%d|q%d",
		path, info.ModTime().UnixNano(), info.Size(), width, coverThumbJPEGQuality))
	return fmt.Sprintf("\"%x\"", h[:])
}

// etagMatches 判断 If-None-Match 头是否命中给定 etag（支持逗号分隔的多值与 * 通配）。
func etagMatches(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "*" {
		return true
	}
	for candidate := range strings.SplitSeq(header, ",") {
		candidate = strings.TrimSpace(candidate)
		// 兼容弱校验前缀 W/。
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}
