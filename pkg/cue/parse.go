package cue

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ParseFile 解析 .cue 文件，自动检测编码并转为 UTF-8
func ParseFile(path string) (*CUESheet, error) {
	const maxCUEFileSize = 1 << 20 // 1MB

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat cue file: %w", err)
	}
	if info.Size() > maxCUEFileSize {
		return nil, fmt.Errorf("CUE file too large: %d bytes (max %d)", info.Size(), maxCUEFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cue file: %w", err)
	}
	content, err := decodeCUEText(data)
	if err != nil {
		return nil, fmt.Errorf("decode cue file: %w", err)
	}
	return Parse(content)
}

// Parse 从 UTF-8 字符串解析 CUE 内容
func Parse(content string) (*CUESheet, error) {
	sheet := &CUESheet{}
	var currentFile *CUEFile
	var currentTrack *CUETrack

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		cmd, rest := splitCommand(line)
		switch cmd {
		case "REM":
			parseREM(sheet, rest)
		case "TITLE":
			title := unquote(rest)
			if currentTrack != nil {
				currentTrack.Title = title
			} else {
				sheet.Title = title
			}
		case "PERFORMER":
			performer := unquote(rest)
			if currentTrack != nil {
				currentTrack.Performer = performer
			} else {
				sheet.Performer = performer
			}
		case "FILE":
			if currentTrack != nil && currentFile != nil {
				currentFile.Tracks = append(currentFile.Tracks, *currentTrack)
				currentTrack = nil
			}
			f := parseFileCommand(rest)
			sheet.Files = append(sheet.Files, f)
			currentFile = &sheet.Files[len(sheet.Files)-1]
		case "TRACK":
			if currentTrack != nil && currentFile != nil {
				currentFile.Tracks = append(currentFile.Tracks, *currentTrack)
			}
			t := parseTrackCommand(rest)
			currentTrack = &t
		case "INDEX":
			if currentTrack != nil {
				parseIndex(currentTrack, rest)
			}
		case "ISRC":
			if currentTrack != nil {
				currentTrack.ISRC = strings.TrimSpace(rest)
			}
		}
	}

	if currentTrack != nil && currentFile != nil {
		currentFile.Tracks = append(currentFile.Tracks, *currentTrack)
	}

	if len(sheet.Files) == 0 {
		return nil, fmt.Errorf("no FILE entries found in CUE sheet")
	}

	totalTracks := 0
	for _, f := range sheet.Files {
		totalTracks += len(f.Tracks)
	}
	if totalTracks == 0 {
		return nil, fmt.Errorf("no TRACK entries found in CUE sheet")
	}

	return sheet, nil
}

func splitCommand(line string) (cmd, rest string) {
	i := strings.IndexByte(line, ' ')
	if i < 0 {
		return strings.ToUpper(line), ""
	}
	return strings.ToUpper(line[:i]), strings.TrimSpace(line[i+1:])
}

func parseREM(sheet *CUESheet, rest string) {
	key, val := splitCommand(rest)
	val = unquote(val)
	switch key {
	case "GENRE":
		sheet.Genre = val
	case "DATE":
		sheet.Date = val
	}
}

func parseFileCommand(rest string) CUEFile {
	f := CUEFile{}
	if idx := strings.LastIndexByte(rest, ' '); idx > 0 {
		f.Filename = unquote(strings.TrimSpace(rest[:idx]))
		f.FileType = strings.ToUpper(strings.TrimSpace(rest[idx+1:]))
	} else {
		f.Filename = unquote(rest)
	}
	return f
}

func parseTrackCommand(rest string) CUETrack {
	t := CUETrack{}
	parts := strings.Fields(rest)
	if len(parts) >= 1 {
		t.Number, _ = strconv.Atoi(parts[0])
	}
	return t
}

func parseIndex(track *CUETrack, rest string) {
	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return
	}
	indexNum, _ := strconv.Atoi(parts[0])
	if indexNum != 1 {
		return // 只关心 INDEX 01
	}
	track.Start = parseTime(parts[1])
}

func parseTime(s string) CUETime {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return CUETime{}
	}
	m, _ := strconv.Atoi(parts[0])
	sec, _ := strconv.Atoi(parts[1])
	f, _ := strconv.Atoi(parts[2])
	return CUETime{Minutes: m, Seconds: sec, Frames: f}
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
