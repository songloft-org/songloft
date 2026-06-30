package cue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTracks(t *testing.T) {
	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "album.flac")
	os.WriteFile(audioFile, []byte("fake audio"), 0644)

	sheet := &CUESheet{
		Title:     "Test Album",
		Performer: "Test Artist",
		Genre:     "Pop",
		Date:      "2024",
		Files: []CUEFile{
			{
				Filename: "album.flac",
				FileType: "WAVE",
				Tracks: []CUETrack{
					{Number: 1, Title: "Song One", Start: CUETime{0, 0, 0}},
					{Number: 2, Title: "Song Two", Performer: "Guest", Start: CUETime{3, 30, 0}},
					{Number: 3, Title: "Song Three", Start: CUETime{7, 15, 25}},
				},
			},
		},
	}

	totalDurations := map[string]float64{
		audioFile: 600.0,
	}

	tracks, err := ResolveTracks(sheet, tmpDir, totalDurations)
	if err != nil {
		t.Fatalf("ResolveTracks failed: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(tracks))
	}

	// Track 1
	if tracks[0].Album != "Test Album" {
		t.Errorf("track 1 album: got %q", tracks[0].Album)
	}
	if tracks[0].Performer != "Test Artist" {
		t.Errorf("track 1 performer: got %q", tracks[0].Performer)
	}
	if tracks[0].StartSeconds != 0 {
		t.Errorf("track 1 start: got %f", tracks[0].StartSeconds)
	}
	if tracks[0].EndSeconds != 210.0 { // 3:30
		t.Errorf("track 1 end: expected 210, got %f", tracks[0].EndSeconds)
	}

	// Track 2: performer should be "Guest" (track-level override)
	if tracks[1].Performer != "Guest" {
		t.Errorf("track 2 performer: got %q", tracks[1].Performer)
	}

	// Track 3: end should be total duration
	if tracks[2].EndSeconds != 600.0 {
		t.Errorf("track 3 end: expected 600, got %f", tracks[2].EndSeconds)
	}
}

func TestResolveTracksAudioNotFound(t *testing.T) {
	sheet := &CUESheet{
		Files: []CUEFile{
			{
				Filename: "nonexistent.flac",
				Tracks:   []CUETrack{{Number: 1, Title: "X", Start: CUETime{}}},
			},
		},
	}

	_, err := ResolveTracks(sheet, "/tmp/nonexistent", nil)
	if err == nil {
		t.Error("expected error for missing audio file")
	}
}

func TestResolveTracksMultiFile(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "disc1.flac")
	f2 := filepath.Join(tmpDir, "disc2.flac")
	os.WriteFile(f1, []byte("fake"), 0644)
	os.WriteFile(f2, []byte("fake"), 0644)

	sheet := &CUESheet{
		Title: "Multi Disc",
		Files: []CUEFile{
			{
				Filename: "disc1.flac",
				Tracks: []CUETrack{
					{Number: 1, Title: "D1T1", Start: CUETime{0, 0, 0}},
					{Number: 2, Title: "D1T2", Start: CUETime{5, 0, 0}},
				},
			},
			{
				Filename: "disc2.flac",
				Tracks: []CUETrack{
					{Number: 3, Title: "D2T1", Start: CUETime{0, 0, 0}},
				},
			},
		},
	}

	durations := map[string]float64{
		f1: 600,
		f2: 300,
	}

	tracks, err := ResolveTracks(sheet, tmpDir, durations)
	if err != nil {
		t.Fatalf("ResolveTracks failed: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(tracks))
	}

	if tracks[0].AudioFilePath != f1 {
		t.Errorf("track 1 audio path: got %q", tracks[0].AudioFilePath)
	}
	if tracks[2].AudioFilePath != f2 {
		t.Errorf("track 3 audio path: got %q", tracks[2].AudioFilePath)
	}
	if tracks[2].EndSeconds != 300 {
		t.Errorf("track 3 end: expected 300, got %f", tracks[2].EndSeconds)
	}
}

func TestResolvedTrackDuration(t *testing.T) {
	tr := ResolvedTrack{StartSeconds: 10, EndSeconds: 50}
	if tr.Duration() != 40 {
		t.Errorf("expected duration 40, got %f", tr.Duration())
	}

	tr2 := ResolvedTrack{StartSeconds: 10, EndSeconds: 0}
	if tr2.Duration() != 0 {
		t.Errorf("expected duration 0 for EndSeconds=0, got %f", tr2.Duration())
	}
}

func TestResolveTracksDefaultTitle(t *testing.T) {
	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "album.flac")
	os.WriteFile(audioFile, []byte("fake"), 0644)

	sheet := &CUESheet{
		Files: []CUEFile{
			{
				Filename: "album.flac",
				Tracks: []CUETrack{
					{Number: 5, Start: CUETime{0, 0, 0}},
				},
			},
		},
	}

	tracks, err := ResolveTracks(sheet, tmpDir, map[string]float64{audioFile: 300})
	if err != nil {
		t.Fatalf("ResolveTracks failed: %v", err)
	}
	if tracks[0].Title != "Track 05" {
		t.Errorf("expected default title 'Track 05', got %q", tracks[0].Title)
	}
}
