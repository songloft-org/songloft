package services

import (
	"sync"
	"time"
)

// ConvertStatus 转换状态
type ConvertStatus string

const (
	ConvertStatusIdle       ConvertStatus = "idle"       // 空闲
	ConvertStatusRunning    ConvertStatus = "running"    // 转换中
	ConvertStatusCompleted  ConvertStatus = "completed"  // 已完成
	ConvertStatusFailed     ConvertStatus = "failed"     // 失败
	ConvertStatusCancelling ConvertStatus = "cancelling" // 取消中
	ConvertStatusCancelled  ConvertStatus = "cancelled"  // 已取消
)

// ConvertUpdateType 转换进度更新类型
type ConvertUpdateType string

const (
	ConvertUpdateConverted ConvertUpdateType = "converted" // 已转换
	ConvertUpdateSkipped   ConvertUpdateType = "skipped"   // 已跳过
	ConvertUpdateFailed    ConvertUpdateType = "failed"    // 失败
)

// ConvertProgress 转换进度信息
type ConvertProgress struct {
	PlaylistID     int64         `json:"playlist_id"`     // 歌单 ID
	Status         ConvertStatus `json:"status"`          // 当前状态
	TotalSongs     int           `json:"total_songs"`     // 待转换总数
	ProcessedSongs int           `json:"processed_songs"` // 已处理数(成功+跳过+失败)
	ConvertedSongs int           `json:"converted_songs"` // 已转换数
	SkippedSongs   int           `json:"skipped_songs"`   // 跳过数(本地歌曲、已转过)
	FailedSongs    int           `json:"failed_songs"`    // 失败数
	CurrentSong    string        `json:"current_song"`    // 当前正在处理的歌曲
	Waiting        bool          `json:"waiting"`         // 是否处于风控限速等待中
	Errors         []string      `json:"errors"`          // 错误明细
	StartTime      *time.Time    `json:"start_time"`      // 开始时间
	EndTime        *time.Time    `json:"end_time"`        // 结束时间
	Error          string        `json:"error"`           // 致命错误
}

// convertEntry 单个歌单转换的状态条目
type convertEntry struct {
	progress ConvertProgress
	cancel   chan struct{}
}

// ConvertProgressManager 转换进度管理器(按 playlistID 隔离)
type ConvertProgressManager struct {
	mu      sync.RWMutex
	entries map[int64]*convertEntry
}

// NewConvertProgressManager 创建转换进度管理器
func NewConvertProgressManager() *ConvertProgressManager {
	return &ConvertProgressManager{
		entries: make(map[int64]*convertEntry),
	}
}

// IsRunning 指定歌单是否正在转换
func (m *ConvertProgressManager) IsRunning(playlistID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.entries[playlistID]
	if !ok {
		return false
	}
	return entry.progress.Status == ConvertStatusRunning
}

// Start 开始转换,返回 false 表示该歌单已有任务运行中
func (m *ConvertProgressManager) Start(playlistID int64, total int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, ok := m.entries[playlistID]; ok {
		if entry.progress.Status == ConvertStatusRunning {
			return false
		}
	}

	now := time.Now()
	m.entries[playlistID] = &convertEntry{
		progress: ConvertProgress{
			PlaylistID: playlistID,
			Status:     ConvertStatusRunning,
			TotalSongs: total,
			StartTime:  &now,
			Errors:     []string{},
		},
		cancel: make(chan struct{}),
	}
	return true
}

// GetCancelChannel 获取取消通道(若歌单无任务则返回 nil)
func (m *ConvertProgressManager) GetCancelChannel(playlistID int64) <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if entry, ok := m.entries[playlistID]; ok {
		return entry.cancel
	}
	return nil
}

// UpdateCurrent 更新当前正在处理的歌曲(不计入处理数)
func (m *ConvertProgressManager) UpdateCurrent(playlistID int64, current string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.entries[playlistID]; ok {
		entry.progress.CurrentSong = current
	}
}

// SetWaiting 设置等待标志(风控限速 sleep 阶段)
func (m *ConvertProgressManager) SetWaiting(playlistID int64, waiting bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.entries[playlistID]; ok {
		entry.progress.Waiting = waiting
	}
}

// UpdateProgress 处理完一首歌后更新进度
func (m *ConvertProgressManager) UpdateProgress(playlistID int64, current string, updateType ConvertUpdateType, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[playlistID]
	if !ok {
		return
	}
	entry.progress.ProcessedSongs++
	entry.progress.CurrentSong = current
	switch updateType {
	case ConvertUpdateConverted:
		entry.progress.ConvertedSongs++
	case ConvertUpdateSkipped:
		entry.progress.SkippedSongs++
	case ConvertUpdateFailed:
		entry.progress.FailedSongs++
		if errMsg != "" && len(entry.progress.Errors) < 50 {
			entry.progress.Errors = append(entry.progress.Errors, errMsg)
		}
	}
}

// Complete 标记完成
func (m *ConvertProgressManager) Complete(playlistID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[playlistID]
	if !ok {
		return
	}
	now := time.Now()
	entry.progress.Status = ConvertStatusCompleted
	entry.progress.EndTime = &now
	entry.progress.CurrentSong = ""
	entry.progress.Waiting = false
	if entry.cancel != nil {
		select {
		case <-entry.cancel:
		default:
			close(entry.cancel)
		}
		entry.cancel = nil
	}
}

// Fail 标记失败(致命错误,例如歌单不存在)
func (m *ConvertProgressManager) Fail(playlistID int64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[playlistID]
	if !ok {
		return
	}
	now := time.Now()
	entry.progress.Status = ConvertStatusFailed
	entry.progress.EndTime = &now
	entry.progress.CurrentSong = ""
	entry.progress.Waiting = false
	if err != nil {
		entry.progress.Error = err.Error()
	}
	if entry.cancel != nil {
		select {
		case <-entry.cancel:
		default:
			close(entry.cancel)
		}
		entry.cancel = nil
	}
}

// Cancel 取消转换,返回 false 表示该歌单无运行中任务
func (m *ConvertProgressManager) Cancel(playlistID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[playlistID]
	if !ok || entry.progress.Status != ConvertStatusRunning {
		return false
	}
	entry.progress.Status = ConvertStatusCancelling
	if entry.cancel != nil {
		select {
		case <-entry.cancel:
		default:
			close(entry.cancel)
		}
		entry.cancel = nil
	}
	return true
}

// SetCancelled 标记为已取消
func (m *ConvertProgressManager) SetCancelled(playlistID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[playlistID]
	if !ok {
		return
	}
	now := time.Now()
	entry.progress.Status = ConvertStatusCancelled
	entry.progress.EndTime = &now
	entry.progress.CurrentSong = ""
	entry.progress.Waiting = false
}

// GetProgress 获取转换进度,不存在时返回 idle 状态
func (m *ConvertProgressManager) GetProgress(playlistID int64) ConvertProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if entry, ok := m.entries[playlistID]; ok {
		return entry.progress
	}
	return ConvertProgress{
		PlaylistID: playlistID,
		Status:     ConvertStatusIdle,
		Errors:     []string{},
	}
}
