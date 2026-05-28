package jsplugin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// sha256Hex 计算字节数据的 SHA256 十六进制字符串
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// readEntryFromZip 从 ZIP 字节中读取入口文件
// 优先级：baseName.jsc > baseName.js
// 返回：文件内容、实际文件名、错误
func readEntryFromZip(zipData []byte, mainField string) ([]byte, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, "", fmt.Errorf("open zip: %w", err)
	}

	// 构造候选文件名列表（优先 .jsc）
	baseName := strings.TrimSuffix(mainField, filepath.Ext(mainField))
	candidates := []string{
		baseName + ".jsc",
		mainField, // 原始 .js 文件
	}

	for _, candidate := range candidates {
		for _, f := range reader.File {
			if f.Name == candidate {
				rc, err := f.Open()
				if err != nil {
					return nil, "", fmt.Errorf("open entry %q in zip: %w", candidate, err)
				}
				defer rc.Close()
				content, err := io.ReadAll(rc)
				if err != nil {
					return nil, "", fmt.Errorf("read entry %q in zip: %w", candidate, err)
				}
				return content, candidate, nil
			}
		}
	}

	return nil, "", fmt.Errorf("entry file not found in zip (tried: %v)", candidates)
}

// extractStaticFromZip 从 ZIP 中解压 static/ 目录到指定路径
func extractStaticFromZip(zipData []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	const staticPrefix = "static/"
	hasStatic := false

	for _, f := range reader.File {
		if !strings.HasPrefix(f.Name, staticPrefix) {
			continue
		}
		hasStatic = true

		// 计算目标路径
		relPath := strings.TrimPrefix(f.Name, staticPrefix)
		if relPath == "" {
			continue
		}
		destPath := filepath.Join(targetDir, relPath)

		// 安全检查：防止路径遍历
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(targetDir)) {
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("mkdir %q: %w", destPath, err)
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("mkdir parent for %q: %w", destPath, err)
		}

		// 解压文件
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %q in zip: %w", f.Name, err)
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create %q: %w", destPath, err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			rc.Close()
			return fmt.Errorf("extract %q: %w", f.Name, err)
		}

		outFile.Close()
		rc.Close()
	}

	if !hasStatic {
		// 没有 static/ 目录，不算错误
		return nil
	}

	return nil
}

// loadBytecodeCache 尝试加载缓存的字节码
// 检查流程：
// 1. 读取 .sha256 文件获取记录的 source hash
// 2. 对比 source hash 与当前 entryHash（源码变了则缓存失效）
// 3. 读取 .jsc 文件
// 4. 计算 .jsc 的 SHA256 并验证完整性
// 返回 (字节码内容, 是否有效)
func loadBytecodeCache(jscPath, hashPath, currentEntryHash string) ([]byte, bool) {
	// 读取 hash 文件 — 格式为两行：第一行 source_hash，第二行 jsc_hash
	hashData, err := os.ReadFile(hashPath)
	if err != nil {
		return nil, false // 无缓存
	}

	lines := strings.Split(strings.TrimSpace(string(hashData)), "\n")
	if len(lines) != 2 {
		return nil, false
	}

	savedSourceHash := lines[0]
	savedJscHash := lines[1]

	// 源码 hash 变了 → 缓存失效
	if savedSourceHash != currentEntryHash {
		os.Remove(jscPath)
		os.Remove(hashPath)
		return nil, false
	}

	// 读取 .jsc 文件
	jscData, err := os.ReadFile(jscPath)
	if err != nil {
		return nil, false
	}

	// 校验 .jsc 文件完整性
	actualJscHash := sha256Hex(jscData)
	if actualJscHash != savedJscHash {
		// hash 不匹配 = 被篡改，删除缓存
		slog.Warn("bytecode cache tampered, removing", "path", jscPath)
		os.Remove(jscPath)
		os.Remove(hashPath)
		return nil, false
	}

	return jscData, true
}

// saveBytecodeCache 保存字节码缓存和 hash 文件
func saveBytecodeCache(jscPath, hashPath string, bytecode []byte, sourceEntryHash string) {
	dir := filepath.Dir(jscPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("create cache dir failed", "error", err)
		return
	}

	// 写入 .jsc
	if err := os.WriteFile(jscPath, bytecode, 0o644); err != nil {
		slog.Warn("write bytecode cache failed", "error", err)
		return
	}

	// 计算 .jsc hash 并写入 hash 文件（两行：source_hash + jsc_hash）
	jscHash := sha256Hex(bytecode)
	hashContent := sourceEntryHash + "\n" + jscHash
	if err := os.WriteFile(hashPath, []byte(hashContent), 0o644); err != nil {
		slog.Warn("write hash file failed", "error", err)
		os.Remove(jscPath) // 清理不完整的缓存
		return
	}

	slog.Info("bytecode cached", "path", jscPath)
}

// readPluginManifestFromZip 从 ZIP 中读取并解析 plugin.json
func readPluginManifestFromZip(zipData []byte) (*PluginManifest, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, f := range reader.File {
		if f.Name == "plugin.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open plugin.json in zip: %w", err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("read plugin.json: %w", err)
			}

			var manifest PluginManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("parse plugin.json: %w", err)
			}

			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("plugin.json not found in zip")
}
