package app

import (
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Base62 字符集
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// decodeBase62 将 Base62 字符串解码为原始字符串
func decodeBase62(encoded string) (string, error) {
	if encoded == "" || encoded == "0" {
		return "", nil
	}

	// Base62 转大整数
	num := big.NewInt(0)
	base := big.NewInt(62)

	for _, c := range encoded {
		index := strings.IndexRune(base62Chars, c)
		if index < 0 {
			return "", nil // 无效字符，返回空
		}
		num.Mul(num, base)
		num.Add(num, big.NewInt(int64(index)))
	}

	// 大整数转字节数组
	bytes := num.Bytes()
	return string(bytes), nil
}

// getEncodedPathAndExtension 分离 Base62 编码的路径和扩展名
func getEncodedPathAndExtension(urlPath string) (encodedPath, ext string) {
	lastDot := strings.LastIndex(urlPath, ".")
	if lastDot > 0 {
		return urlPath[:lastDot], urlPath[lastDot:]
	}
	return urlPath, ""
}

// isPathUnderDir 验证文件路径是否在指定目录下
func isPathUnderDir(filePath, dir string) bool {
	// 清理并获取绝对路径
	absFilePath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return false
	}

	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return false
	}

	// 确保目录以路径分隔符结尾，防止前缀匹配问题
	if !strings.HasSuffix(absDir, string(filepath.Separator)) {
		absDir += string(filepath.Separator)
	}

	// 检查文件路径是否以目录为前缀
	return strings.HasPrefix(absFilePath, absDir) || absFilePath == strings.TrimSuffix(absDir, string(filepath.Separator))
}

// registerStaticFile 注册静态文件访问路由
// urlPrefix: URL 前缀（如 "/music/" 或 "/cover/"）
// configKey: 配置键（如 "music_path" 或 "cover_storage_path"）
// checkFileExists: 是否需要检查文件存在性
func (a *App) registerStaticFile(urlPrefix string, configKey string, checkFileExists bool) {
	a.router.Get(urlPrefix+"*", func(w http.ResponseWriter, r *http.Request) {
		// 从配置中读取路径
		var pathConfig struct {
			Path string `json:"path"`
		}
		if err := a.configService.GetJSON(configKey, &pathConfig); err != nil {
			http.Error(w, "Failed to get path config", http.StatusInternalServerError)
			return
		}

		// 使用公共辅助函数提供文件服务
		a.serveStaticFile(w, r, urlPrefix, pathConfig.Path, checkFileExists)
	})
}

// serveStaticFile 提供静态文件服务的公共逻辑
// urlPrefix: URL 前缀（如 "/music/" 或 "/cover/"）
// baseDir: 基础目录路径
// checkFileExists: 是否需要检查文件存在性
func (a *App) serveStaticFile(w http.ResponseWriter, r *http.Request, urlPrefix string, baseDir string, checkFileExists bool) {
	// 从 URL 参数获取 access_token
	accessToken := r.URL.Query().Get("access_token")
	if accessToken == "" {
		http.Error(w, "Missing access_token", http.StatusUnauthorized)
		return
	}

	// 验证 token
	ctx := r.Context()
	_, err := a.authService.ValidateToken(ctx, accessToken)
	if err != nil {
		http.Error(w, "Invalid access_token", http.StatusUnauthorized)
		return
	}

	// 获取 URL 路径（去掉前缀）
	urlPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	if urlPath == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// 分离 Base62 编码的路径和扩展名
	encodedPath, ext := getEncodedPathAndExtension(urlPath)

	// 解码 Base62
	decodedPath, err := decodeBase62(encodedPath)
	if err != nil || decodedPath == "" {
		http.Error(w, "Invalid path encoding", http.StatusBadRequest)
		return
	}

	// 重新组合完整文件路径
	filePath := decodedPath + ext

	// 安全验证：确保文件路径在指定目录下
	if !isPathUnderDir(filePath, baseDir) {
		slog.Info("serveStaticFile", "filePath", filePath, "baseDir", baseDir)
		http.Error(w, "Access denied: path outside directory", http.StatusForbidden)
		return
	}

	// 如果需要，检查文件是否存在
	if checkFileExists {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
	}

	slog.Debug("serveStaticFile", "filePath", filePath)

	// 设置浏览器缓存：URL 含 Base62 编码路径，内容变更时 URL 也会变，可安全永久缓存
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	// 提供文件服务
	http.ServeFile(w, r, filePath)
}
