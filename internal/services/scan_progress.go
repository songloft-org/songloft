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
	ScanStatusSplittingCue      ScanStatus = "splitting_cue"      // CUE 整轨切分中
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
	ExcludedDirectories []ExcludedDirectory `json:"excluded_directories"` // 实际因规则跳过的目录
	Status              ScanStatus          `json:"status"`               // 当前状态
	DiscoveredFiles     int                 `json:"discovered_files"`     // scanning 阶段已发现的音频文件数
	TotalFiles          int                 `json:"total_files"`          // 总文件数
	ScannedFiles        int                 `json:"scanned_files"`        // 已扫描文件数
	ImportedFiles       int                 `json:"imported_files"`       // 已导入文件数
	SkippedFiles        int                 `json:"skipped_files"`        // 跳过的文件数（已存在）
	FailedFiles         int                 `json:"failed_files"`         // 失败的文件数
	CleanedFiles        int                 `json:"cleaned_files"`        // 清理的过期文件数
	CueSplitSources     int                 `json:"cue_split_sources"`    // splitting_cue 阶段已处理的 CUE 来源数
	LocalSongCount      int                 `json:"local_song_count"`     // 扫描完成后数据库中本地歌曲总数
	CurrentFile         string              `json:"current_file"`         // 当前处理的文件
	StartTime           *time.Time          `json:"start_time"`           // 开始时间
	EndTime             *time.Time          `json:"end_time"`             // 结束时间
	Error               string              `json:"error"`                // 错误信息
}

func (m *ScanProgressManager) SetExcludedDirectories(dirs []ExcludedDirectory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.ExcludedDirectories = append([]ExcludedDirectory{}, dirs...)
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
		m.progress.Status == ScanStatusSplittingCue ||
		m.progress.Status == ScanStatusCreatingPlaylists
}

// BeginSplittingCue 切换到 CUE 整轨切分阶段。
// 该阶段对大 CD 镜像可能耗时较长，需单独上报进度，避免前端误判为卡死。
func (m *ScanProgressManager) BeginSplittingCue() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.Status = ScanStatusSplittingCue
	m.progress.CurrentFile = ""
	m.progress.CueSplitSources = 0
}

// UpdateCueProgress 上报正在切分的 CUE 来源，并累加已处理来源数。
func (m *ScanProgressManager) UpdateCueProgress(currentSource string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.CueSplitSources++
	m.progress.CurrentFile = currentSource
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

	// 只有终态（idle/completed/failed/cancelled）才能开始新扫描
	switch m.progress.Status {
	case ScanStatusIdle, ScanStatusCompleted, ScanStatusFailed, ScanStatusCancelled:
		// OK
	default:
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

// SetDiscoveredFiles 更新 scanning 阶段已发现的音频文件数
func (m *ScanProgressManager) SetDiscoveredFiles(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.DiscoveredFiles = count
}

// SetTotalFiles 设置总文件数
func (m *ScanProgressManager) SetTotalFiles(total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.TotalFiles = total
	m.progress.Status = ScanStatusImporting
}

// SetCurrentFile 仅更新当前处理的文件，不改动任何计数。
// 供元数据提取阶段的 worker 调用：计数只在批次落库后累加（见 flushScanBatch），
// 一个目录组定稿前计数会停住，靠这里让前端看出扫描仍在推进而非卡死。
func (m *ScanProgressManager) SetCurrentFile(currentFile string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.CurrentFile = currentFile
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

	// 扫描 / 导入 / CUE 切分阶段可取消；
	// 自动创建歌单阶段已在 commit 事务，不允许取消。
	if m.progress.Status != ScanStatusScanning &&
		m.progress.Status != ScanStatusImporting &&
		m.progress.Status != ScanStatusSplittingCue {
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

// SetLocalSongCount 设置数据库中本地歌曲总数（扫描完成时调用）
func (m *ScanProgressManager) SetLocalSongCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.LocalSongCount = count
}

// Reset 重置进度（仅在非活跃状态下可用）
func (m *ScanProgressManager) Reset() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 只有终态（idle/completed/failed/cancelled）才能重置
	switch m.progress.Status {
	case ScanStatusIdle, ScanStatusCompleted, ScanStatusFailed, ScanStatusCancelled:
		// OK
	default:
		return false
	}

	m.progress = ScanProgress{
		Status: ScanStatusIdle,
	}
	return true
}
