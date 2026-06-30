package cue

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	content := `REM GENRE Rock
REM DATE 2005
PERFORMER "Artist Name"
TITLE "Album Name"
FILE "album.flac" WAVE
  TRACK 01 AUDIO
    TITLE "First Song"
    PERFORMER "Singer A"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Second Song"
    INDEX 00 03:45:10
    INDEX 01 03:47:00
  TRACK 03 AUDIO
    TITLE "Third Song"
    PERFORMER "Singer B"
    ISRC USRC17607839
    INDEX 01 07:22:50
`
	sheet, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if sheet.Title != "Album Name" {
		t.Errorf("expected album title 'Album Name', got %q", sheet.Title)
	}
	if sheet.Performer != "Artist Name" {
		t.Errorf("expected performer 'Artist Name', got %q", sheet.Performer)
	}
	if sheet.Genre != "Rock" {
		t.Errorf("expected genre 'Rock', got %q", sheet.Genre)
	}
	if sheet.Date != "2005" {
		t.Errorf("expected date '2005', got %q", sheet.Date)
	}
	if len(sheet.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(sheet.Files))
	}

	f := sheet.Files[0]
	if f.Filename != "album.flac" {
		t.Errorf("expected filename 'album.flac', got %q", f.Filename)
	}
	if f.FileType != "WAVE" {
		t.Errorf("expected file type 'WAVE', got %q", f.FileType)
	}
	if len(f.Tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(f.Tracks))
	}

	// Track 1
	if f.Tracks[0].Number != 1 {
		t.Errorf("track 1 number: got %d", f.Tracks[0].Number)
	}
	if f.Tracks[0].Title != "First Song" {
		t.Errorf("track 1 title: got %q", f.Tracks[0].Title)
	}
	if f.Tracks[0].Performer != "Singer A" {
		t.Errorf("track 1 performer: got %q", f.Tracks[0].Performer)
	}
	if f.Tracks[0].Start.ToSeconds() != 0 {
		t.Errorf("track 1 start: got %f", f.Tracks[0].Start.ToSeconds())
	}

	// Track 2: INDEX 01 at 03:47:00
	if f.Tracks[1].Number != 2 {
		t.Errorf("track 2 number: got %d", f.Tracks[1].Number)
	}
	if f.Tracks[1].Title != "Second Song" {
		t.Errorf("track 2 title: got %q", f.Tracks[1].Title)
	}
	expected := 3*60 + 47.0
	if f.Tracks[1].Start.ToSeconds() != expected {
		t.Errorf("track 2 start: expected %f, got %f", expected, f.Tracks[1].Start.ToSeconds())
	}

	// Track 3
	if f.Tracks[2].ISRC != "USRC17607839" {
		t.Errorf("track 3 ISRC: got %q", f.Tracks[2].ISRC)
	}
	if f.Tracks[2].Performer != "Singer B" {
		t.Errorf("track 3 performer: got %q", f.Tracks[2].Performer)
	}
}

func TestParseMultiFile(t *testing.T) {
	content := `TITLE "Multi File Album"
PERFORMER "Various Artists"
FILE "disc1.flac" WAVE
  TRACK 01 AUDIO
    TITLE "Song 1"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Song 2"
    INDEX 01 04:30:00
FILE "disc2.flac" WAVE
  TRACK 03 AUDIO
    TITLE "Song 3"
    INDEX 01 00:00:00
`
	sheet, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(sheet.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(sheet.Files))
	}
	if len(sheet.Files[0].Tracks) != 2 {
		t.Errorf("file 1: expected 2 tracks, got %d", len(sheet.Files[0].Tracks))
	}
	if len(sheet.Files[1].Tracks) != 1 {
		t.Errorf("file 2: expected 1 track, got %d", len(sheet.Files[1].Tracks))
	}
	if sheet.Files[1].Filename != "disc2.flac" {
		t.Errorf("file 2 filename: got %q", sheet.Files[1].Filename)
	}
}

func TestParseEmpty(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestParseNoTracks(t *testing.T) {
	content := `FILE "album.flac" WAVE
`
	_, err := Parse(content)
	if err == nil {
		t.Error("expected error for no tracks")
	}
}

func TestCueTimeToSeconds(t *testing.T) {
	tests := []struct {
		time     CUETime
		expected float64
	}{
		{CUETime{0, 0, 0}, 0.0},
		{CUETime{1, 0, 0}, 60.0},
		{CUETime{0, 30, 0}, 30.0},
		{CUETime{0, 0, 75}, 1.0},
		{CUETime{3, 47, 0}, 227.0},
		{CUETime{0, 0, 37}, 37.0 / 75.0},
	}

	for _, tt := range tests {
		got := tt.time.ToSeconds()
		if got != tt.expected {
			t.Errorf("CUETime{%d,%d,%d}.ToSeconds() = %f, want %f",
				tt.time.Minutes, tt.time.Seconds, tt.time.Frames, got, tt.expected)
		}
	}
}

func TestCueTimeFormatFFmpeg(t *testing.T) {
	ct := CUETime{3, 47, 0}
	got := ct.FormatFFmpeg()
	if got != "00:03:47.000" {
		t.Errorf("FormatFFmpeg() = %q, want %q", got, "00:03:47.000")
	}

	ct2 := CUETime{0, 0, 37}
	got2 := ct2.FormatFFmpeg()
	expected := "00:00:00.493"
	if got2 != expected {
		t.Errorf("FormatFFmpeg() = %q, want %q", got2, expected)
	}
}

func TestDecodeToUTF8_ValidUTF8(t *testing.T) {
	data := []byte("Hello, 世界")
	result := decodeToUTF8(data)
	if result != "Hello, 世界" {
		t.Errorf("expected 'Hello, 世界', got %q", result)
	}
}

func TestDecodeToUTF8_BOM(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("Hello")...)
	result := decodeToUTF8(data)
	if result != "Hello" {
		t.Errorf("expected 'Hello', got %q", result)
	}
}
