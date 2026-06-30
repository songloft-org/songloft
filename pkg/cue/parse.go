package cue

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"

	"github.com/hanxi/tag"
)

// ParseFile 解析 .cue 文件，自动检测编码并转为 UTF-8
func ParseFile(path string) (*CUESheet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cue file: %w", err)
	}
	content := decodeToUTF8(data)
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

// decodeToUTF8 检测编码并转换为 UTF-8
func decodeToUTF8(data []byte) string {
	// BOM 检测
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return string(data[3:])
	}
	if len(data) >= 2 {
		if data[0] == 0xFF && data[1] == 0xFE {
			return decodeUTF16LE(data[2:])
		}
		if data[0] == 0xFE && data[1] == 0xFF {
			return decodeUTF16BE(data[2:])
		}
	}

	if utf8.Valid(data) && !hasMojibakeHeuristic(data) {
		return string(data)
	}

	// 复用 tag.FixEncoding 处理 GBK/GB18030/GB2312
	fixed := tag.FixEncoding(data)
	if fixed != string(data) && utf8.ValidString(fixed) {
		return fixed
	}

	// 尝试 Shift-JIS
	if decoded, err := decodeWithTransform(data, japanese.ShiftJIS.NewDecoder()); err == nil {
		return decoded
	}
	// 尝试 EUC-KR
	if decoded, err := decodeWithTransform(data, korean.EUCKR.NewDecoder()); err == nil {
		return decoded
	}

	return string(data)
}

func decodeWithTransform(data []byte, t transform.Transformer) (string, error) {
	reader := transform.NewReader(bytes.NewReader(data), t)
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	result := string(decoded)
	if utf8.ValidString(result) {
		return result, nil
	}
	return "", fmt.Errorf("decoded result is not valid UTF-8")
}

func decodeUTF16LE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	var buf strings.Builder
	for i := 0; i+1 < len(data); i += 2 {
		r := rune(data[i]) | rune(data[i+1])<<8
		buf.WriteRune(r)
	}
	return buf.String()
}

func decodeUTF16BE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	var buf strings.Builder
	for i := 0; i+1 < len(data); i += 2 {
		r := rune(data[i])<<8 | rune(data[i+1])
		buf.WriteRune(r)
	}
	return buf.String()
}

func hasMojibakeHeuristic(data []byte) bool {
	s := string(data)
	latinCount := 0
	total := 0
	consecutive := 0
	maxConsecutive := 0
	for _, r := range s {
		total++
		if r >= 0x00C0 && r <= 0x00FF {
			latinCount++
			consecutive++
			if consecutive > maxConsecutive {
				maxConsecutive = consecutive
			}
		} else {
			consecutive = 0
		}
	}
	if total > 0 && float64(latinCount)/float64(total) > 0.2 {
		return true
	}
	return maxConsecutive >= 3
}

