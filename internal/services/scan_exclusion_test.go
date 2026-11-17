package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanExclusionDiagnostics(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"allowed/song.mp3", "yt-dlp/album/a.mp3", "yt-dlp/album/b.mp3", ".cache/sub/a.mp3"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	scanner := NewScanner(&ScanConfig{MusicPath: root, ExcludePaths: []string{filepath.Join(root, "yt-dlp")}, ExcludeDirs: []string{".cache"}, SupportedFormats: []string{"mp3"}})

	// Validation checks inherited rules, including nonexistent descendants, without
	// walking subdirectories or treating similarly prefixed names as matches.
	validation := scanner.ValidateRoots([]string{filepath.Join(root, "yt-dlp", "album"), filepath.Join(root, ".cache", "missing"), filepath.Join(root, "yt-dlp-other")})
	if len(validation.Paths) != 1 || len(validation.ExcludedDirectories) != 2 {
		t.Fatalf("validation: %+v", validation)
	}
	if validation.ExcludedDirectories[0].RuleType != "path" || validation.ExcludedDirectories[1].RuleType != "name" {
		t.Fatalf("rules: %+v", validation)
	}

	for _, roots := range [][]string{nil, {filepath.Join(root, "yt-dlp", "album"), filepath.Join(root, "allowed"), filepath.Join(root, ".cache")}} {
		result, err := scanner.ScanFilesWithCueInDirs(context.Background(), roots, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.AudioFiles) != 1 || len(result.ExcludedDirectories) != 2 {
			t.Fatalf("scan roots=%v: %+v", roots, result)
		}
		// A pruned subtree counts as one skipped directory, not its descendant
		// directories or unknown number of audio files.
		for _, excluded := range result.ExcludedDirectories {
			if excluded.Rule == "" || !scanner.ShouldExcludeDir(excluded.Path) {
				t.Fatalf("invalid diagnostic: %+v", excluded)
			}
		}
	}

	manager := NewScanProgressManager()
	manager.Start()
	manager.SetExcludedDirectories(validation.ExcludedDirectories)
	manager.Complete()
	if len(manager.GetProgress().ExcludedDirectories) != 2 {
		t.Fatal("completion lost exclusions")
	}
	manager.Start()
	if len(manager.GetProgress().ExcludedDirectories) != 0 {
		t.Fatal("new scan inherited previous exclusions")
	}
}
