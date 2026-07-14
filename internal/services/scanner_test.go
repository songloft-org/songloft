package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// setupTestMusicDir 创建测试音乐目录
func setupTestMusicDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "music_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// 创建测试文件
	testFiles := []string{
		"song1.mp3",
		"song2.flac",
		"song3.wav",
		"song4.m4a",
		"subdir/song5.mp3",
		"@eaDir/ignore.mp3", // 应该被排除
		"tmpdir/ignore.mp3", // 应该被排除
		"readme.txt",        // 非音频文件
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", fullPath, err)
		}
	}

	return tmpDir
}

// TestNewScanner 测试创建扫描器
func TestNewScanner(t *testing.T) {
	tmpDir := setupTestMusicDir(t)
	defer os.RemoveAll(tmpDir)

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{"@eaDir", "tmpdir"},
		SupportedFormats: []string{"mp3", "flac", "wav", "ape", "ogg", "m4a"},
	}

	scanner := NewScanner(config)
	if scanner == nil {
		t.Error("NewScanner() returned nil")
	}
}

// TestScanFiles 测试扫描文件
func TestScanFiles(t *testing.T) {
	tmpDir := setupTestMusicDir(t)
	defer os.RemoveAll(tmpDir)

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{"@eaDir", "tmpdir"},
		SupportedFormats: []string{"mp3", "flac", "wav", "m4a"},
	}

	scanner := NewScanner(config)
	ctx := context.Background()

	files, err := scanner.ScanFiles(ctx, nil)
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	// 应该找到 5 个音频文件（排除 @eaDir 和 tmp 目录中的文件）
	if len(files) != 5 {
		t.Errorf("ScanFiles() found %d files, want 5", len(files))
	}

	// 验证文件路径
	foundFiles := make(map[string]bool)
	for _, file := range files {
		foundFiles[filepath.Base(file)] = true
	}

	expectedFiles := []string{"song1.mp3", "song2.flac", "song3.wav", "song4.m4a", "song5.mp3"}
	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("ScanFiles() missing expected file: %s", expected)
		}
	}

	// 验证排除的文件不在结果中
	for _, file := range files {
		if filepath.Base(filepath.Dir(file)) == "@eaDir" || filepath.Base(filepath.Dir(file)) == "tmpdir" {
			t.Errorf("ScanFiles() should not include files from excluded dirs: %s", file)
		}
	}
}

// TestIsAudioFile 测试音频文件判断
func TestIsAudioFile(t *testing.T) {
	config := &ScanConfig{
		SupportedFormats: []string{"mp3", "flac", "wav", "ape", "ogg", "m4a"},
	}

	scanner := NewScanner(config)

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"mp3 file", "song.mp3", true},
		{"flac file", "song.flac", true},
		{"wav file", "song.wav", true},
		{"m4a file", "song.m4a", true},
		{"uppercase ext", "song.MP3", true},
		{"txt file", "readme.txt", false},
		{"no extension", "song", false},
		{"hidden file", ".hidden.mp3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanner.IsAudioFile(tt.filename); got != tt.want {
				t.Errorf("IsAudioFile(%s) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

// TestShouldExcludeDir 测试目录排除判断
func TestShouldExcludeDir(t *testing.T) {
	config := &ScanConfig{
		ExcludeDirs: []string{"@eaDir", "tmp", ".git"},
	}

	scanner := NewScanner(config)

	tests := []struct {
		name    string
		dirPath string
		want    bool
	}{
		{"exclude @eaDir", "/music/@eaDir", true},
		{"exclude tmp", "/music/tmp", true},
		{"exclude .git", "/music/.git", true},
		{"normal dir", "/music/albums", false},
		{"nested exclude", "/music/subdir/@eaDir", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanner.ShouldExcludeDir(tt.dirPath); got != tt.want {
				t.Errorf("ShouldExcludeDir(%s) = %v, want %v", tt.dirPath, got, tt.want)
			}
		})
	}
}

// TestScanWithEmptyDir 测试空目录扫描
func TestScanWithEmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "empty_music_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3", "flac"},
	}

	scanner := NewScanner(config)
	ctx := context.Background()

	files, err := scanner.ScanFiles(ctx, nil)
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("ScanFiles() in empty dir found %d files, want 0", len(files))
	}
}

// TestScanWithNonExistentDir 测试不存在的目录
func TestScanWithNonExistentDir(t *testing.T) {
	config := &ScanConfig{
		MusicPath:        "/non/existent/path",
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3"},
	}

	scanner := NewScanner(config)
	ctx := context.Background()

	_, err := scanner.ScanFiles(ctx, nil)
	if err == nil {
		t.Error("ScanFiles() with non-existent dir should return error")
	}
}

// TestGetFileInfo 测试获取文件信息
func TestGetFileInfo(t *testing.T) {
	tmpDir := setupTestMusicDir(t)
	defer os.RemoveAll(tmpDir)

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3"},
	}

	scanner := NewScanner(config)

	// 测试获取存在的文件信息
	testFile := filepath.Join(tmpDir, "song1.mp3")
	info, err := scanner.GetFileInfo(testFile)
	if err != nil {
		t.Errorf("GetFileInfo() error = %v", err)
	}
	if info == nil {
		t.Error("GetFileInfo() should return file info")
	}
	if info.Size == 0 {
		t.Error("GetFileInfo() file size should not be 0")
	}

	// 测试获取不存在的文件信息
	_, err = scanner.GetFileInfo("/non/existent/file.mp3")
	if err == nil {
		t.Error("GetFileInfo() should return error for non-existent file")
	}
}

// TestScanWithNestedDirs 测试嵌套目录扫描
func TestScanWithNestedDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nested_music_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建嵌套目录结构
	dirs := []string{
		"artist1/album1",
		"artist1/album2",
		"artist2/album1",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	// 创建音频文件
	files := []string{
		"artist1/album1/song1.mp3",
		"artist1/album2/song2.mp3",
		"artist2/album1/song3.mp3",
	}
	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.WriteFile(fullPath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3"},
	}

	scanner := NewScanner(config)
	ctx := context.Background()

	foundFiles, err := scanner.ScanFiles(ctx, nil)
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	if len(foundFiles) != 3 {
		t.Errorf("ScanFiles() found %d files, want 3", len(foundFiles))
	}
}

// TestScanWithSymlinks 测试符号链接处理
func TestScanWithSymlinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "symlink_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建真实文件
	realFile := filepath.Join(tmpDir, "real.mp3")
	if err := os.WriteFile(realFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3"},
	}

	scanner := NewScanner(config)
	ctx := context.Background()

	files, err := scanner.ScanFiles(ctx, nil)
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	if len(files) != 1 {
		t.Errorf("ScanFiles() found %d files, want 1", len(files))
	}
}

// TestScanWithMixedFormats 测试混合格式
func TestScanWithMixedFormats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mixed_format_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建不同格式的文件（mp4 为音视频混合容器，应被识别为音频；txt/jpg 应被忽略）
	formats := []string{"mp3", "flac", "wav", "ape", "ogg", "m4a", "mp4", "txt", "jpg"}
	for _, format := range formats {
		file := filepath.Join(tmpDir, "test."+format)
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3", "flac", "wav", "ape", "ogg", "m4a", "mp4"},
	}

	scanner := NewScanner(config)
	ctx := context.Background()

	files, err := scanner.ScanFiles(ctx, nil)
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	// 应该只找到 7 个音频文件
	if len(files) != 7 {
		t.Errorf("ScanFiles() found %d files, want 7", len(files))
	}
}

// TestShouldExcludeDirWithPaths 测试按完整路径排除
func TestShouldExcludeDirWithPaths(t *testing.T) {
	config := &ScanConfig{
		ExcludeDirs:  []string{},
		ExcludePaths: []string{"/music/古典/练习曲", "/music/流行/低音质"},
	}

	scanner := NewScanner(config)

	tests := []struct {
		name    string
		dirPath string
		want    bool
	}{
		{"exact match", "/music/古典/练习曲", true},
		{"sub dir of excluded", "/music/古典/练习曲/子目录", true},
		{"parent dir not excluded", "/music/古典", false},
		{"other dir not excluded", "/music/摇滚", false},
		{"second exclude path", "/music/流行/低音质", true},
		{"sub of second exclude", "/music/流行/低音质/2024", true},
		{"sibling not excluded", "/music/流行/高音质", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanner.ShouldExcludeDir(tt.dirPath); got != tt.want {
				t.Errorf("ShouldExcludeDir(%s) = %v, want %v", tt.dirPath, got, tt.want)
			}
		})
	}
}

// TestShouldExcludeDirMixed 测试混合模式（名称 + 路径）
func TestShouldExcludeDirMixed(t *testing.T) {
	config := &ScanConfig{
		ExcludeDirs:  []string{"@eaDir", "tmp"},
		ExcludePaths: []string{"/music/古典/练习曲"},
	}

	scanner := NewScanner(config)

	tests := []struct {
		name    string
		dirPath string
		want    bool
	}{
		{"name match @eaDir", "/music/albums/@eaDir", true},
		{"name match tmp", "/music/tmp", true},
		{"path match", "/music/古典/练习曲", true},
		{"path match sub", "/music/古典/练习曲/sub", true},
		{"normal dir", "/music/albums/rock", false},
		{"nested name match", "/music/deep/nested/@eaDir", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanner.ShouldExcludeDir(tt.dirPath); got != tt.want {
				t.Errorf("ShouldExcludeDir(%s) = %v, want %v", tt.dirPath, got, tt.want)
			}
		})
	}
}

// TestIsFileInExcludedArea 测试文件排除区域判断
func TestIsFileInExcludedArea(t *testing.T) {
	config := &ScanConfig{
		ExcludeDirs:  []string{"@eaDir"},
		ExcludePaths: []string{"/music/古典/练习曲"},
	}

	scanner := NewScanner(config)

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"file in name-excluded dir", "/music/albums/@eaDir/song.mp3", true},
		{"file in path-excluded dir", "/music/古典/练习曲/song.mp3", true},
		{"file in sub of path-excluded", "/music/古典/练习曲/sub/song.mp3", true},
		{"file in normal dir", "/music/albums/rock/song.mp3", false},
		{"file in parent of excluded", "/music/古典/song.mp3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanner.IsFileInExcludedArea(tt.filePath); got != tt.want {
				t.Errorf("IsFileInExcludedArea(%s) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

// TestListSubDirs 测试子目录列表
func TestListSubDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "listsubdirs_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建目录结构
	dirs := []string{
		"古典/巴赫",
		"古典/莫扎特",
		"流行",
		"摇滚",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	// 创建一个文件（不应出现在目录列表中）
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	config := &ScanConfig{MusicPath: tmpDir}
	scanner := NewScanner(config)

	// 测试根目录
	dirs2, err := scanner.ListSubDirs(tmpDir)
	if err != nil {
		t.Fatalf("ListSubDirs() error = %v", err)
	}

	if len(dirs2) != 3 {
		t.Errorf("ListSubDirs() returned %d dirs, want 3", len(dirs2))
	}

	// 验证 HasChildren
	for _, d := range dirs2 {
		if d.Name == "古典" && !d.HasChildren {
			t.Error("古典 should have children")
		}
		if d.Name == "流行" && d.HasChildren {
			t.Error("流行 should not have children")
		}
	}

	// 测试子目录
	dirs3, err := scanner.ListSubDirs(filepath.Join(tmpDir, "古典"))
	if err != nil {
		t.Fatalf("ListSubDirs(古典) error = %v", err)
	}

	if len(dirs3) != 2 {
		t.Errorf("ListSubDirs(古典) returned %d dirs, want 2", len(dirs3))
	}

	// 测试不存在的目录
	_, err = scanner.ListSubDirs("/non/existent/path")
	if err == nil {
		t.Error("ListSubDirs() should return error for non-existent path")
	}
}

// TestCollectAllDirNames 测试目录名称收集
func TestCollectAllDirNames(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "collectdirnames_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建目录结构
	dirs := []string{
		"古典/巴赫",
		"古典/莫扎特",
		"流行/2024",
		"摇滚",
		"@eaDir",
		"tmp",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	config := &ScanConfig{MusicPath: tmpDir}
	scanner := NewScanner(config)
	ctx := context.Background()

	names, err := scanner.CollectAllDirNames(ctx)
	if err != nil {
		t.Fatalf("CollectAllDirNames() error = %v", err)
	}

	// 应该收集到所有唯一的目录名称
	expectedNames := []string{"2024", "@eaDir", "tmp", "古典", "巴赫", "摇滚", "流行", "莫扎特"}
	if len(names) != len(expectedNames) {
		t.Errorf("CollectAllDirNames() returned %d names, want %d: %v", len(names), len(expectedNames), names)
	}

	// 验证排序
	for i, expected := range expectedNames {
		if i < len(names) && names[i] != expected {
			t.Errorf("CollectAllDirNames()[%d] = %s, want %s", i, names[i], expected)
		}
	}

	// 测试不存在的目录
	config2 := &ScanConfig{MusicPath: "/non/existent/path"}
	scanner2 := NewScanner(config2)
	_, err = scanner2.CollectAllDirNames(ctx)
	if err == nil {
		t.Error("CollectAllDirNames() should return error for non-existent path")
	}
}

// TestGetMusicPath 测试获取音乐目录路径
func TestGetMusicPath(t *testing.T) {
	config := &ScanConfig{MusicPath: "/music"}
	scanner := NewScanner(config)

	if got := scanner.GetMusicPath(); got != "/music" {
		t.Errorf("GetMusicPath() = %s, want /music", got)
	}
}

// baseNames 收集扫描结果中的文件名（去路径），便于断言。
func baseNames(files []string) map[string]bool {
	m := make(map[string]bool, len(files))
	for _, f := range files {
		m[filepath.Base(f)] = true
	}
	return m
}

// TestScanFilesWithCueInDirs_EmptyRootsDegradesToFullScan 空 roots 退化为全库扫描。
func TestScanFilesWithCueInDirs_EmptyRootsDegradesToFullScan(t *testing.T) {
	tmpDir := setupTestMusicDir(t)
	defer os.RemoveAll(tmpDir)

	config := &ScanConfig{
		MusicPath:        tmpDir,
		ExcludeDirs:      []string{"@eaDir", "tmpdir"},
		SupportedFormats: []string{"mp3", "flac", "wav", "m4a"},
	}
	scanner := NewScanner(config)

	result, err := scanner.ScanFilesWithCueInDirs(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ScanFilesWithCueInDirs(nil) error = %v", err)
	}
	// 与全库 ScanFiles 一致：5 个音频文件（排除 @eaDir、tmpdir）。
	if len(result.AudioFiles) != 5 {
		t.Errorf("empty roots found %d files, want 5", len(result.AudioFiles))
	}
}

// TestScanFilesWithCueInDirs_ScopedToSubdir 只扫指定子目录。
func TestScanFilesWithCueInDirs_ScopedToSubdir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scoped_scan_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := []string{
		"A/a1.mp3",
		"A/a2.mp3",
		"B/b1.mp3",
		"C/c1.mp3",
	}
	for _, f := range files {
		full := filepath.Join(tmpDir, f)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("test"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	config := &ScanConfig{MusicPath: tmpDir, SupportedFormats: []string{"mp3"}}
	scanner := NewScanner(config)

	// 只扫 A 和 B，不该出现 C。
	roots := []string{filepath.Join(tmpDir, "A"), filepath.Join(tmpDir, "B")}
	result, err := scanner.ScanFilesWithCueInDirs(context.Background(), roots, nil)
	if err != nil {
		t.Fatalf("ScanFilesWithCueInDirs() error = %v", err)
	}
	names := baseNames(result.AudioFiles)
	if len(result.AudioFiles) != 3 {
		t.Errorf("scoped scan found %d files, want 3: %v", len(result.AudioFiles), result.AudioFiles)
	}
	for _, want := range []string{"a1.mp3", "a2.mp3", "b1.mp3"} {
		if !names[want] {
			t.Errorf("scoped scan missing %s", want)
		}
	}
	if names["c1.mp3"] {
		t.Error("scoped scan should not include c1.mp3 (outside scope)")
	}
}

// TestScanFilesWithCueInDirs_OverlappingRootsNoDuplicates 重叠 roots 不重复计数（共用 visited）。
func TestScanFilesWithCueInDirs_OverlappingRootsNoDuplicates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "overlap_scan_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	full := filepath.Join(tmpDir, "A", "sub", "s.mp3")
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte("test"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	config := &ScanConfig{MusicPath: tmpDir, SupportedFormats: []string{"mp3"}}
	scanner := NewScanner(config)

	// 同时传父目录 A 和其子目录 A/sub，s.mp3 只应出现一次。
	roots := []string{filepath.Join(tmpDir, "A"), filepath.Join(tmpDir, "A", "sub")}
	result, err := scanner.ScanFilesWithCueInDirs(context.Background(), roots, nil)
	if err != nil {
		t.Fatalf("ScanFilesWithCueInDirs() error = %v", err)
	}
	if len(result.AudioFiles) != 1 {
		t.Errorf("overlapping roots found %d files, want 1 (no dup): %v", len(result.AudioFiles), result.AudioFiles)
	}
}

// TestScanFilesWithCueInDirs_NonExistentRoot 全部 root 都不存在时返回错误。
func TestScanFilesWithCueInDirs_NonExistentRoot(t *testing.T) {
	tmpDir := setupTestMusicDir(t)
	defer os.RemoveAll(tmpDir)

	config := &ScanConfig{MusicPath: tmpDir, SupportedFormats: []string{"mp3"}}
	scanner := NewScanner(config)

	_, err := scanner.ScanFilesWithCueInDirs(context.Background(), []string{filepath.Join(tmpDir, "nope")}, nil)
	if err == nil {
		t.Error("ScanFilesWithCueInDirs() with all non-existent roots should return error")
	}
}

// TestScanFilesWithCueInDirs_SkipsMissingRootScansRest 部分 root 缺失时跳过缺失、继续扫其余。
func TestScanFilesWithCueInDirs_SkipsMissingRootScansRest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skip_missing_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	full := filepath.Join(tmpDir, "A", "a1.mp3")
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte("test"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	config := &ScanConfig{MusicPath: tmpDir, SupportedFormats: []string{"mp3"}}
	scanner := NewScanner(config)

	// A 存在、B 不存在：应跳过 B、正常扫到 A 的文件，不报错。
	roots := []string{filepath.Join(tmpDir, "A"), filepath.Join(tmpDir, "B")}
	result, err := scanner.ScanFilesWithCueInDirs(context.Background(), roots, nil)
	if err != nil {
		t.Fatalf("ScanFilesWithCueInDirs() with one missing root should not error: %v", err)
	}
	if len(result.AudioFiles) != 1 || !baseNames(result.AudioFiles)["a1.mp3"] {
		t.Errorf("expected a1.mp3 scanned from existing root, got %v", result.AudioFiles)
	}
}
