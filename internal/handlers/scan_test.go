package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mimusic/internal/services"
)

// TestNewScanHandler 测试创建扫描处理器
func TestNewScanHandler(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewScanHandler(songService, nil)

	if handler == nil {
		t.Error("NewScanHandler() returned nil")
	}

	if handler.songService == nil {
		t.Error("NewScanHandler() songService should not be nil")
	}
}

// TestScanHandlerStructure 测试扫描处理器结构
func TestScanHandlerStructure(t *testing.T) {
	repo := newTestSongRepo(t)
	songService := services.NewSongService(repo, nil, nil, nil, nil, nil)
	handler := NewScanHandler(songService, nil)

	// 验证处理器结构正确
	if handler.songService != songService {
		t.Error("ScanHandler songService should match the provided service")
	}
}

// TestScanAndImportSuccess 测试成功的扫描导入
func TestScanAndImportSuccess(t *testing.T) {
	repo := newTestSongRepo(t)

	// 创建临时测试目录
	tempDir := t.TempDir()

	// 创建 scanner 和 extractor
	scanner := services.NewScanner(&services.ScanConfig{
		MusicPath:        tempDir,
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3", "flac"},
	})
	extractor := services.NewMetadataExtractor(&services.MetadataConfig{
		FFProbePath: "ffprobe",
	})

	songService := services.NewSongService(repo, nil, extractor, scanner, nil, nil)
	handler := NewScanHandler(songService, scanner)

	req := httptest.NewRequest("POST", "/api/v1/scan", nil)
	rr := httptest.NewRecorder()

	handler.ScanAndImport(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// 异步扫描立即返回"扫描任务已启动"
	if response["message"] != "扫描任务已启动" {
		t.Errorf("handler returned wrong message: got %v want 扫描任务已启动", response["message"])
	}
}

// TestScanAndImportError 测试扫描导入失败
// 注意：异步扫描即使路径不存在也会立即返回 200 OK，错误在异步处理中
func TestScanAndImportError(t *testing.T) {
	repo := newTestSongRepo(t)

	// 创建会返回错误的 scanner（传入不存在的路径）
	scanner := services.NewScanner(&services.ScanConfig{
		MusicPath:        "/nonexistent/path/that/does/not/exist",
		ExcludeDirs:      []string{},
		SupportedFormats: []string{"mp3"},
	})
	extractor := services.NewMetadataExtractor(&services.MetadataConfig{
		FFProbePath: "ffprobe",
	})

	songService := services.NewSongService(repo, nil, extractor, scanner, nil, nil)
	handler := NewScanHandler(songService, scanner)

	req := httptest.NewRequest("POST", "/api/v1/scan", nil)
	rr := httptest.NewRecorder()

	handler.ScanAndImport(rr, req)

	// 异步扫描即使路径不存在也会返回 200，错误在异步任务中处理
	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}
