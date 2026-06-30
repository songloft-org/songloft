package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanConfig 扫描配置
type ScanConfig struct {
	MusicPath        string   // 音乐目录路径
	ExcludeDirs      []string // 排除的目录名称（按名称匹配，路径中任何层级包含该名称都会被排除）
	ExcludePaths     []string // 排除的完整路径（精确匹配，仅排除指定路径及其子目录）
	SupportedFormats []string // 支持的音频格式
}

// DirEntry 目录条目（用于目录树懒加载）
type DirEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasChildren bool   `json:"has_children"`
}

// Scanner 文件扫描器
type Scanner struct {
	config *ScanConfig
}

// NewScanner 创建新的扫描器
func NewScanner(config *ScanConfig) *Scanner {
	return &Scanner{
		config: config,
	}
}

// ScanFiles 扫描音乐文件。onProgress 为可选回调，每发现一批音频文件时调用，参数为当前已发现的文件总数。
func (s *Scanner) ScanFiles(ctx context.Context, onProgress func(count int)) ([]string, error) {
	// 检查目录是否存在
	if _, err := os.Stat(s.config.MusicPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("music directory does not exist: %s", s.config.MusicPath)
	}

	var files []string
	// 使用 map 记录已访问的真实路径，防止循环软链接
	visited := make(map[string]bool)

	// 递归扫描目录
	err := s.scanDir(ctx, s.config.MusicPath, visited, &files, onProgress)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return files, nil
}

// ScanResult 扫描结果
type ScanResult struct {
	AudioFiles []string
	CueFiles   []string // .cue 文件绝对路径
}

// ScanFilesWithCue 扫描音频文件和 .cue 文件。
func (s *Scanner) ScanFilesWithCue(ctx context.Context, onProgress func(count int)) (*ScanResult, error) {
	if _, err := os.Stat(s.config.MusicPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("music directory does not exist: %s", s.config.MusicPath)
	}

	result := &ScanResult{}
	visited := make(map[string]bool)

	err := s.scanDirWithCue(ctx, s.config.MusicPath, visited, result, onProgress)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return result, nil
}

func (s *Scanner) scanDirWithCue(ctx context.Context, dirPath string, visited map[string]bool, result *ScanResult, onProgress func(count int)) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	realPath, err := filepath.EvalSymlinks(dirPath)
	if err != nil {
		realPath = dirPath
	}
	if visited[realPath] {
		return nil
	}
	visited[realPath] = true

	if s.ShouldExcludeDir(dirPath) {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entryPath := filepath.Join(dirPath, entry.Name())
		info, err := os.Stat(entryPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			if err := s.scanDirWithCue(ctx, entryPath, visited, result, onProgress); err != nil {
				return err
			}
		} else {
			if s.IsAudioFile(entryPath) {
				result.AudioFiles = append(result.AudioFiles, entryPath)
				if onProgress != nil && len(result.AudioFiles)%scanProgressInterval == 0 {
					onProgress(len(result.AudioFiles))
				}
			} else if strings.EqualFold(filepath.Ext(entryPath), ".cue") {
				result.CueFiles = append(result.CueFiles, entryPath)
			}
		}
	}

	return nil
}

const scanProgressInterval = 100

// scanDir 递归扫描目录，支持软链接并防止循环
func (s *Scanner) scanDir(ctx context.Context, dirPath string, visited map[string]bool, files *[]string, onProgress func(count int)) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 获取目录的真实路径（解析软链接）用于循环检测
	realPath, err := filepath.EvalSymlinks(dirPath)
	if err != nil {
		// EvalSymlinks 在某些平台（如 Android FUSE）可能失败，退回原始路径
		realPath = dirPath
	}

	// 检查是否已访问过该真实路径，防止循环
	if visited[realPath] {
		return nil
	}
	visited[realPath] = true

	// 检查是否需要排除该目录
	if s.ShouldExcludeDir(dirPath) {
		return nil
	}

	// 读取目录内容
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entryPath := filepath.Join(dirPath, entry.Name())

		// 获取文件信息（跟随软链接）
		info, err := os.Stat(entryPath)
		if err != nil {
			// 如果无法获取信息（如软链接目标不存在），跳过
			continue
		}

		if info.IsDir() {
			// 递归扫描子目录（包括软链接目录）
			if err := s.scanDir(ctx, entryPath, visited, files, onProgress); err != nil {
				return err
			}
		} else {
			// 如果是音频文件，添加到列表
			if s.IsAudioFile(entryPath) {
				*files = append(*files, entryPath)
				if onProgress != nil && len(*files)%scanProgressInterval == 0 {
					onProgress(len(*files))
				}
			}
		}
	}

	return nil
}

// IsAudioFile 判断是否为音频文件
func (s *Scanner) IsAudioFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}

	// 移除点号
	ext = strings.TrimPrefix(ext, ".")

	// 检查是否在支持的格式列表中
	for _, format := range s.config.SupportedFormats {
		if ext == strings.ToLower(format) {
			return true
		}
	}

	return false
}

// ShouldExcludeDir 判断目录是否应该被排除
// 支持两种排除模式：
// 1. 按完整路径精确排除（ExcludePaths）：目录路径以排除路径为前缀时排除
// 2. 按名称模式匹配排除（ExcludeDirs）：路径中任何层级包含该名称时排除
func (s *Scanner) ShouldExcludeDir(dirPath string) bool {
	cleanPath := filepath.Clean(dirPath)

	// 1. 检查完整路径排除列表（精确匹配）
	for _, excludePath := range s.config.ExcludePaths {
		cleanExclude := filepath.Clean(excludePath)
		// 目录路径等于排除路径，或者是排除路径的子目录
		if cleanPath == cleanExclude || strings.HasPrefix(cleanPath, cleanExclude+string(os.PathSeparator)) {
			return true
		}
	}

	// 2. 检查名称匹配排除列表
	parts := strings.Split(cleanPath, string(os.PathSeparator))
	for _, part := range parts {
		for _, excludeDir := range s.config.ExcludeDirs {
			if part == excludeDir {
				return true
			}
		}
	}

	return false
}

// IsFileInExcludedArea 判断文件路径是否在排除区域内
// 用于 CleanInvalidSongs 判断已导入的歌曲是否应该被清理
func (s *Scanner) IsFileInExcludedArea(filePath string) bool {
	// 获取文件所在目录
	dirPath := filepath.Dir(filePath)
	return s.ShouldExcludeDir(dirPath)
}

// GetMusicPath 获取音乐目录路径
func (s *Scanner) GetMusicPath() string {
	return s.config.MusicPath
}

// ListSubDirs 返回指定目录下的一级子目录列表（用于目录树懒加载）
func (s *Scanner) ListSubDirs(dirPath string) ([]DirEntry, error) {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var dirs []DirEntry
	for _, entry := range entries {
		// 跳过非目录
		if !entry.IsDir() {
			// 也检查软链接指向的目录
			entryPath := filepath.Join(dirPath, entry.Name())
			info, err := os.Stat(entryPath)
			if err != nil || !info.IsDir() {
				continue
			}
		}

		entryPath := filepath.Join(dirPath, entry.Name())

		// 检查是否有子目录
		hasChildren := false
		subEntries, err := os.ReadDir(entryPath)
		if err == nil {
			for _, sub := range subEntries {
				if sub.IsDir() {
					hasChildren = true
					break
				}
				// 也检查软链接指向的目录
				subPath := filepath.Join(entryPath, sub.Name())
				subInfo, err := os.Stat(subPath)
				if err == nil && subInfo.IsDir() {
					hasChildren = true
					break
				}
			}
		}

		dirs = append(dirs, DirEntry{
			Name:        entry.Name(),
			Path:        entryPath,
			HasChildren: hasChildren,
		})
	}

	return dirs, nil
}

// CollectAllDirNames 递归收集音乐目录下所有唯一的目录名称（用于自动补全）
func (s *Scanner) CollectAllDirNames(ctx context.Context) ([]string, error) {
	if _, err := os.Stat(s.config.MusicPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("music directory does not exist: %s", s.config.MusicPath)
	}

	nameSet := make(map[string]bool)
	visited := make(map[string]bool)

	err := s.collectDirNames(ctx, s.config.MusicPath, visited, nameSet)
	if err != nil {
		return nil, fmt.Errorf("failed to collect directory names: %w", err)
	}

	// 转换为排序后的切片
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)

	return names, nil
}

// collectDirNames 递归收集目录名称
func (s *Scanner) collectDirNames(ctx context.Context, dirPath string, visited map[string]bool, nameSet map[string]bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 解析软链接，防止循环
	realPath, err := filepath.EvalSymlinks(dirPath)
	if err != nil {
		realPath = dirPath
	}
	if visited[realPath] {
		return nil
	}
	visited[realPath] = true

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())

		// 获取文件信息（跟随软链接）
		info, err := os.Stat(entryPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			// 收集目录名称
			nameSet[entry.Name()] = true
			// 递归收集子目录
			if err := s.collectDirNames(ctx, entryPath, visited, nameSet); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetFileInfo 获取文件基本信息
func (s *Scanner) GetFileInfo(filePath string) (*FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &FileInfo{
		Path:    filePath,
		Name:    filepath.Base(filePath),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
		Format:  strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), "."),
	}, nil
}

// FileInfo 文件信息
type FileInfo struct {
	Path    string
	Name    string
	Size    int64
	ModTime interface{}
	Format  string
}
