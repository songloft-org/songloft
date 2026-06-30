package cue

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ParseFLACCueSheetBlock 解析 FLAC CUESHEET metadata block 的二进制数据。
// sampleRate 用于将 sample offset 转换为秒。
func ParseFLACCueSheetBlock(data []byte, sampleRate int) (*CUESheet, error) {
	if len(data) < 396 {
		return nil, fmt.Errorf("FLAC CUESHEET block too short: %d bytes", len(data))
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate: %d", sampleRate)
	}

	// 128 bytes: media catalog number (NUL-padded ASCII)
	// 8 bytes: lead-in samples
	// 1 byte: bit 7 = is_cd, bits 6-0 + 258 bytes reserved
	// 1 byte: number of tracks
	offset := 128 + 8 + 1 + 258 + 1
	if len(data) < offset {
		return nil, fmt.Errorf("FLAC CUESHEET block too short for header")
	}

	numTracks := int(data[offset-1])
	if numTracks == 0 {
		return nil, fmt.Errorf("no tracks in FLAC CUESHEET block")
	}

	sheet := &CUESheet{}
	cueFile := CUEFile{FileType: "WAVE"}
	pos := offset

	for i := 0; i < numTracks; i++ {
		if pos+36 > len(data) {
			break
		}

		trackOffsetSamples := binary.BigEndian.Uint64(data[pos : pos+8])
		trackNumber := int(data[pos+8])
		isrc := strings.TrimRight(string(data[pos+9:pos+21]), "\x00")
		// bit 0 of data[pos+21]: type (0=audio, 1=non-audio)
		isAudio := (data[pos+21] & 0x80) == 0
		numIndexPoints := int(data[pos+35])
		pos += 36

		for j := 0; j < numIndexPoints; j++ {
			if pos+12 > len(data) {
				break
			}
			pos += 12
		}

		// track 170 (0xAA) 是 lead-out，跳过
		if trackNumber == 170 || !isAudio {
			continue
		}

		startSeconds := float64(trackOffsetSamples) / float64(sampleRate)
		t := secondsToCueTime(startSeconds)
		track := CUETrack{
			Number: trackNumber,
			ISRC:   isrc,
			Start:  t,
		}
		cueFile.Tracks = append(cueFile.Tracks, track)
	}

	if len(cueFile.Tracks) == 0 {
		return nil, fmt.Errorf("no audio tracks found in FLAC CUESHEET")
	}

	sheet.Files = append(sheet.Files, cueFile)
	return sheet, nil
}

func secondsToCueTime(seconds float64) CUETime {
	totalFrames := int(seconds * 75)
	m := totalFrames / (75 * 60)
	totalFrames -= m * 75 * 60
	s := totalFrames / 75
	f := totalFrames % 75
	return CUETime{Minutes: m, Seconds: s, Frames: f}
}
