package services

import (
	"os"
	"path/filepath"
	"testing"

	"songloft/internal/models"
)

func TestNormalizeFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mp3", "mp3"},
		{"MP3", "mp3"},
		{"mpeg", "mp3"},
		{".mp3", "mp3"},
		{"m4a", "m4a"},
		{"aac", "m4a"},
		{"mp4", "m4a"},
		{"ogg", "ogg"},
		{"vorbis", "ogg"},
		{"flac", "flac"},
		{"wav", "wav"},
		{"wave", "wav"},
		{"wma", "wma"},
		{"asf", "wma"},
		{"ape", "ape"},
		{"unknown", "unknown"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeFormat(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeFormat(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNeedsTranscode(t *testing.T) {
	tests := []struct {
		src    string
		target string
		want   bool
	}{
		{"mp3", "", false},
		{"mp3", "mp3", false},
		{"mpeg", "mp3", false},
		{"MP3", "mp3", false},
		{"wma", "mp3", true},
		{"ape", "flac", true},
		{"flac", "mp3", true},
		{"m4a", "aac", false},
		{"", "mp3", false}, // 未知源格式不转码，避免误判
	}
	for _, tt := range tests {
		got := NeedsTranscode(tt.src, tt.target)
		if got != tt.want {
			t.Errorf("NeedsTranscode(%q, %q) = %v, want %v", tt.src, tt.target, got, tt.want)
		}
	}
}

func TestNeedsTranscodeForServe(t *testing.T) {
	dir := t.TempDir()

	// 真实 MP3 样本（tag.ReadFrom 识别为 MP3）
	realMP3 := filepath.Join(dir, "real.mp3")
	mp3Data, err := os.ReadFile("../../pkg/tag/testdata/with_tags/sample.id3v23.mp3")
	if err != nil {
		t.Fatalf("read sample mp3: %v", err)
	}
	if err := os.WriteFile(realMP3, mp3Data, 0644); err != nil {
		t.Fatalf("write real mp3: %v", err)
	}

	// 「伪 mp3」：WebM/EBML 内容却用 .mp3 扩展名（tag.ReadFrom 无法识别为 MP3）。
	// EBML magic 0x1A45DFA3 开头，模拟 YouTube 音频被错误落盘。
	fakeMP3 := filepath.Join(dir, "fake.mp3")
	if err := os.WriteFile(fakeMP3, []byte("\x1a\x45\xdf\xa3not really mp3 bytes"), 0644); err != nil {
		t.Fatalf("write fake mp3: %v", err)
	}

	song := &models.Song{ID: 1, Type: "remote"}

	tests := []struct {
		name    string
		srcPath string
		target  string
		want    bool
	}{
		{"empty target never transcodes", realMP3, "", false},
		{"real mp3 to mp3 short-circuits", realMP3, "mp3", false},
		{"fake mp3 (webm) forces transcode", fakeMP3, "mp3", true},
		{"cross-format still transcodes", realMP3, "flac", true},
		{"unknown source format not probed", filepath.Join(dir, "noext"), "mp3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsTranscodeForServe(song, tt.srcPath, tt.target)
			if got != tt.want {
				t.Errorf("NeedsTranscodeForServe(%q, %q) = %v, want %v", tt.srcPath, tt.target, got, tt.want)
			}
		})
	}
}

func TestParseBitrate(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"128", 128},
		{"192", 192},
		{"320", 320},
		{"128k", 128},
		{"192K", 192},
		{"320k", 320},
		{"", 0},
		{"0", 0},
		{"64", 0},
		{"256", 0},
		{"abc", 0},
		{" 128 ", 128},
		{"128kk", 128},
	}
	for _, tt := range tests {
		got := ParseBitrate(tt.input)
		if got != tt.want {
			t.Errorf("ParseBitrate(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFfmpegArgs(t *testing.T) {
	tests := []struct {
		format  string
		bitrate int
		encoder string
		wantArg string // 第一个 qualityArg 的期望值，用于验证 CBR/VBR
		wantErr bool
	}{
		{"mp3", 0, "libmp3lame", "-q:a", false},
		{"mp3", 128, "libmp3lame", "-b:a", false},
		{"mp3", 320, "libmp3lame", "-b:a", false},
		{"ogg", 0, "libvorbis", "-q:a", false},
		{"ogg", 192, "libvorbis", "-b:a", false},
		{"m4a", 0, "aac", "-b:a", false},
		{"m4a", 128, "aac", "-b:a", false},
		{"flac", 0, "flac", "", false},
		{"flac", 128, "flac", "", false}, // 无损格式忽略 bitrate
		{"wav", 0, "pcm_s16le", "", false},
		{"xyz", 0, "", "", true},
	}
	for _, tt := range tests {
		enc, qargs, _, err := ffmpegArgs(tt.format, tt.bitrate)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ffmpegArgs(%q, %d) should error", tt.format, tt.bitrate)
			}
			continue
		}
		if err != nil {
			t.Errorf("ffmpegArgs(%q, %d) unexpected error: %v", tt.format, tt.bitrate, err)
			continue
		}
		if enc != tt.encoder {
			t.Errorf("ffmpegArgs(%q, %d) encoder = %q, want %q", tt.format, tt.bitrate, enc, tt.encoder)
		}
		if tt.wantArg != "" {
			if len(qargs) == 0 || qargs[0] != tt.wantArg {
				t.Errorf("ffmpegArgs(%q, %d) qualityArgs[0] = %v, want %q", tt.format, tt.bitrate, qargs, tt.wantArg)
			}
		}
	}
}

func TestTranscodedFileName(t *testing.T) {
	cs := &CacheService{cacheDir: "/tmp/test"}

	// 本地歌曲（无 cacheKey），无 bitrate
	local := &models.Song{ID: 42, Type: "local"}
	name := cs.transcodedFileName(local, "mp3", 0, -1)
	if name != "42.tc.mp3" {
		t.Errorf("transcodedFileName(local, 0) = %q, want %q", name, "42.tc.mp3")
	}

	// 本地歌曲，有 bitrate
	name = cs.transcodedFileName(local, "mp3", 128, -1)
	if name != "42.tc.128k.mp3" {
		t.Errorf("transcodedFileName(local, 128) = %q, want %q", name, "42.tc.128k.mp3")
	}

	// 插件来源歌曲（有 cacheKey），无 bitrate
	remote := &models.Song{
		ID:              123,
		Type:            "remote",
		PluginEntryPath: "my-source",
		DedupKey:        "platform:12345",
	}
	name = cs.transcodedFileName(remote, "ogg", 0, -1)
	expected := "123.my-source_platform_12345.tc.ogg"
	if name != expected {
		t.Errorf("transcodedFileName(remote, 0) = %q, want %q", name, expected)
	}

	// 插件来源歌曲，有 bitrate
	name = cs.transcodedFileName(remote, "mp3", 192, -1)
	expected = "123.my-source_platform_12345.tc.192k.mp3"
	if name != expected {
		t.Errorf("transcodedFileName(remote, 192) = %q, want %q", name, expected)
	}
}

func TestFindTranscodedFile(t *testing.T) {
	tmpDir := t.TempDir()
	cs := &CacheService{cacheDir: tmpDir}

	song := &models.Song{ID: 100, Type: "local", Format: "wma"}

	// 不存在时应 miss
	if _, ok := cs.FindTranscodedFile(song, "mp3", 0, -1); ok {
		t.Error("FindTranscodedFile should miss when file does not exist")
	}

	// 创建 format-only 转码文件
	dir, _ := cs.getCachePath(song.ID, "")
	os.MkdirAll(dir, 0755)
	name := cs.transcodedFileName(song, "mp3", 0, -1)
	path := filepath.Join(dir, name)
	os.WriteFile(path, []byte("fake mp3"), 0644)

	// format-only 应命中
	found, ok := cs.FindTranscodedFile(song, "mp3", 0, -1)
	if !ok {
		t.Fatal("FindTranscodedFile should hit after creating file")
	}
	if found != path {
		t.Errorf("FindTranscodedFile path = %q, want %q", found, path)
	}

	// 不同格式应 miss
	if _, ok := cs.FindTranscodedFile(song, "ogg", 0, -1); ok {
		t.Error("FindTranscodedFile should miss for different format")
	}

	// 带 bitrate 的应 miss（不同文件名）
	if _, ok := cs.FindTranscodedFile(song, "mp3", 128, -1); ok {
		t.Error("FindTranscodedFile should miss for same format but different bitrate")
	}

	// 创建带 bitrate 的转码文件
	name128 := cs.transcodedFileName(song, "mp3", 128, -1)
	path128 := filepath.Join(dir, name128)
	os.WriteFile(path128, []byte("fake 128k mp3"), 0644)

	// 带 bitrate 应命中
	found, ok = cs.FindTranscodedFile(song, "mp3", 128, -1)
	if !ok {
		t.Fatal("FindTranscodedFile should hit for bitrate file")
	}
	if found != path128 {
		t.Errorf("FindTranscodedFile path = %q, want %q", found, path128)
	}

	// 不同 bitrate 应 miss
	if _, ok := cs.FindTranscodedFile(song, "mp3", 320, -1); ok {
		t.Error("FindTranscodedFile should miss for different bitrate")
	}
}
