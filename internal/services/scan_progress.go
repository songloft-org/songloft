package services

import (
	"sync"
	"time"
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	ScanStatusIdle              ScanStatus = "idle"               // 空闲
	ScanStatusScanning          ScanStatus = "scanning"           // 扫描文件中
	ScanStatusImporting         ScanStatus = "importing"          // 导入中
	ScanStatusCreatingPlaylists ScanStatus = "creating_playlists" // 自动创建歌单中
	ScanStatusCompleted         ScanStatus = "completed"          // 已完成
	ScanStatusFailed            ScanStatus = "failed"             // 失败
	ScanStatusCancelling        ScanStatus = "cancelling"         // 取消中
	ScanStatusCancelled         ScanStatus = "cancelled"          // 已取消
)

// ProgressUpdateType 进度更新类型
type ProgressUpdateType string

const (
	ProgressUpdateImported ProgressUpdateType = "imported" // 已导入
	ProgressUpdateSkipped  ProgressUpdateType = "skipped"  // 已跳过
	ProgressUpdateFailed   ProgressUpdateType = "failed"   // 失败
)

// ScanProgress 扫描进度信息
type ScanProgress struct {
	Status        ScanStatus `json:"status"`         // 当前状态
	TotalFiles    int        `json:"total_files"`    // 总文件数
	ScannedFiles  int        `json:"scanned_files"`  // 已扫描文件数
	ImportedFiles int        `json:"imported_files"` // 已导入文件数
	SkippedFiles  int        `json:"skipped_files"`  // 跳过的文件数（已存在）
	FailedFiles   int        `json:"failed_files"`   // 失败的文件数
	CleanedFiles  int        `json:"cleaned_files"`  // 清理的过期文件数
	CurrentFile   string     `json:"current_file"`   // 当前处理的文件
	StartTime     *time.Time `json:"start_time"`     // 开始时间
	EndTime       *time.Time `json:"end_time"`       // 结束时间
	Error         string     `json:"error"`          // 错误信息
}

// ScanProgressManager 扫描进度管理器
type ScanProgressManager struct {
	mu       sync.RWMutex
	progress ScanProgress
	cancel   chan struct{}
}

// NewScanProgressManager 创建扫描进度管理器
func NewScanProgressManager() *ScanProgressManager {
	return &ScanProgressManager{
		progress: ScanProgress{
			Status: ScanStatusIdle,
		},
	}
}

// GetProgress 获取当前进度
func (m *ScanProgressManager) GetProgress() ScanProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.progress
}

// IsScanning 是否正在扫描
func (m *ScanProgressManager) IsScanning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.progress.Status == ScanStatusScanning ||
		m.progress.Status == ScanStatusImporting ||
		m.progress.Status == ScanStatusCreatingPlaylists
}

// BeginCreatingPlaylists 切换到自动创建歌单阶段
func (m *ScanProgressManager) BeginCreatingPlaylists() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.Status = ScanStatusCreatingPlaylists
	m.progress.CurrentFile = ""
}

// Start 开始扫描
func (m *ScanProgressManager) Start() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已经在扫描中，返回 false
	if m.progress.Status == ScanStatusScanning || m.progress.Status == ScanStatusImporting {
		return false
	}

	now := time.Now()
	m.progress = ScanProgress{
		Status:    ScanStatusScanning,
		StartTime: &now,
	}
	m.cancel = make(chan struct{})
	return true
}

// SetTotalFiles 设置总文件数
func (m *ScanProgressManager) SetTotalFiles(total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.TotalFiles = total
	m.progress.Status = ScanStatusImporting
}

// UpdateProgress 更新进度
func (m *ScanProgressManager) UpdateProgress(currentFile string, updateType ProgressUpdateType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.progress.ScannedFiles++
	m.progress.CurrentFile = currentFile

	switch updateType {
	case ProgressUpdateImported:
		m.progress.ImportedFiles++
	case ProgressUpdateSkipped:
		m.progress.SkippedFiles++
	case ProgressUpdateFailed:
		m.progress.FailedFiles++
	}
}

// Complete 完成扫描
func (m *ScanProgressManager) Complete() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.progress.Status = ScanStatusCompleted
	m.progress.EndTime = &now
	m.progress.CurrentFile = ""

	// 关闭取消通道
	if m.cancel != nil {
		close(m.cancel)
		m.cancel = nil
	}
}

// Fail 扫描失败
func (m *ScanProgressManager) Fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.progress.Status = ScanStatusFailed
	m.progress.EndTime = &now
	m.progress.CurrentFile = ""
	if err != nil {
		m.progress.Error = err.Error()
	}

	// 关闭取消通道
	if m.cancel != nil {
		close(m.cancel)
		m.cancel = nil
	}
}

// Cancel 取消扫描
func (m *ScanProgressManager) Cancel() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 只有在扫描中才能取消（自动创建歌单阶段已经在 commit 事务，不允许取消）
	if m.progress.Status != ScanStatusScanning && m.progress.Status != ScanStatusImporting {
		return false
	}

	m.progress.Status = ScanStatusCancelling

	// 发送取消信号
	if m.cancel != nil {
		close(m.cancel)
		m.cancel = nil
	}

	return true
}

// SetCancelled 设置为已取消状态
func (m *ScanProgressManager) SetCancelled() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.progress.Status = ScanStatusCancelled
	m.progress.EndTime = &now
	m.progress.CurrentFile = ""
}

// GetCancelChannel 获取取消通道
func (m *ScanProgressManager) GetCancelChannel() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cancel
}

// SetCleanedFiles 设置清理的过期文件数
func (m *ScanProgressManager) SetCleanedFiles(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.CleanedFiles = count
}

// Reset 重置进度（仅在空闲或完成状态下可用）
func (m *ScanProgressManager) Reset() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.progress.Status == ScanStatusScanning || m.progress.Status == ScanStatusImporting {
		return false
	}

	m.progress = ScanProgress{
		Status: ScanStatusIdle,
	}
	return true
}
