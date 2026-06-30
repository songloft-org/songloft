package cue

import "fmt"

// CUESheet 统一的 CUE Sheet 结构体
type CUESheet struct {
	Title     string
	Performer string
	Genre     string
	Date      string
	Files     []CUEFile
}

// CUEFile 一个 FILE 指令块
type CUEFile struct {
	Filename string
	FileType string
	Tracks   []CUETrack
}

// CUETrack 一个 TRACK
type CUETrack struct {
	Number    int
	Title     string
	Performer string
	ISRC      string
	Start     CUETime // INDEX 01 时间点
}

// CUETime MM:SS:FF 时间格式 (FF = frames, 1/75 秒)
type CUETime struct {
	Minutes int
	Seconds int
	Frames  int
}

// ToSeconds 转换为秒
func (t CUETime) ToSeconds() float64 {
	return float64(t.Minutes)*60 + float64(t.Seconds) + float64(t.Frames)/75.0
}

// IsZero 判断是否为零值
func (t CUETime) IsZero() bool {
	return t.Minutes == 0 && t.Seconds == 0 && t.Frames == 0
}

// FormatFFmpeg 返回 ffmpeg -ss/-to 可用的时间字符串
func (t CUETime) FormatFFmpeg() string {
	totalSeconds := t.ToSeconds()
	hours := int(totalSeconds) / 3600
	minutes := (int(totalSeconds) % 3600) / 60
	secs := totalSeconds - float64(hours*3600+minutes*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", hours, minutes, secs)
}
