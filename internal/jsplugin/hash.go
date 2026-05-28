package jsplugin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
)

// hash 错误定义（开发期严格校验，不做任何「空容忍」降级）。
var (
	// ErrManifestHashMissing plugin.json 缺少 entryHash 或 zipHash 字段。
	ErrManifestHashMissing = errors.New("plugin.json: entryHash / zipHash is required")
	// ErrManifestHashInvalid hash 字段格式不是 64 位小写 hex。
	ErrManifestHashInvalid = errors.New("plugin.json: entryHash / zipHash must be 64-char lowercase hex")
	// ErrManifestHashMismatch hash 字段值与 zip 实际内容不一致。
	ErrManifestHashMismatch = errors.New("plugin.json: entryHash / zipHash does not match zip content")
)

// manifestHashRegexp 64 位小写 hex 字符串。
var manifestHashRegexp = regexp.MustCompile(`^[0-9a-f]{64}$`)

// sha256HexSum 计算字节数据的 SHA256 十六进制字符串（64 位小写 hex）。
// 与旧 loader.go 中的 sha256Hex 行为一致，保留用于非规范化场景。
func sha256HexSum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ValidateHashField 校验 hash 字段是否为 64 位小写 hex；空 / 非法直接返回错误。
func ValidateHashField(field, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is empty", ErrManifestHashMissing, field)
	}
	if !manifestHashRegexp.MatchString(value) {
		return fmt.Errorf("%w: %s=%q", ErrManifestHashInvalid, field, value)
	}
	return nil
}

// ComputeEntryHash 计算 zip 内指定入口文件的 sha256（64 位小写 hex）。
//
// 注意：mainPath 是 plugin.json 中的 main 字段（例如 "main.js"）。
func ComputeEntryHash(zipData []byte, mainPath string) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	for _, f := range reader.File {
		if f.Name != mainPath {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open entry %q: %w", mainPath, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return "", fmt.Errorf("read entry %q: %w", mainPath, err)
		}
		return sha256HexSum(data), nil
	}
	return "", fmt.Errorf("entry %q not found in zip", mainPath)
}

// ComputeCanonicalZipHash 按规范化算法计算 zip 的 hash（64 位小写 hex）。
//
// 算法（与 @songloft/plugin-builder 保持一致）：
//  1. 枚举 zip 内所有**非 plugin.json**的普通文件（跳过目录与 plugin.json 本身）
//  2. 按文件名 Unicode 升序排序
//  3. 对每个文件写入：`<path>\n<sha256Hex(content)>\n`
//  4. 对最终拼接串再算 sha256，返回 64 位小写 hex
//
// 该算法对 zip 内文件顺序、元数据（modtime / 压缩方式）不敏感，
// 任意机器重打包结果一致；且排除 plugin.json 自身，避免 hash 写回
// plugin.json 引起的循环依赖。
func ComputeCanonicalZipHash(zipData []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}

	type entry struct {
		path string
		hash string
	}
	entries := make([]entry, 0, len(reader.File))

	for _, f := range reader.File {
		// 跳过目录条目。
		if f.FileInfo().IsDir() {
			continue
		}
		// 规范化算法的核心：排除 plugin.json 自身，避免循环依赖。
		if f.Name == "plugin.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open %q: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return "", fmt.Errorf("read %q: %w", f.Name, err)
		}
		entries = append(entries, entry{path: f.Name, hash: sha256HexSum(data)})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	hasher := sha256.New()
	for _, e := range entries {
		hasher.Write([]byte(e.path))
		hasher.Write([]byte{'\n'})
		hasher.Write([]byte(e.hash))
		hasher.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
