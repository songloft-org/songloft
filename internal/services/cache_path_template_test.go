package services

import (
	"testing"

	"songloft/internal/models"
)

func TestParsePathTemplate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"default", "{artist}-{album}/{title}", false},
		{"simple", "{title}", false},
		{"all placeholders", "{artist}/{album}/{title}-{year}-{genre}", false},
		{"empty", "", true},
		{"no title", "{artist}/{album}", true},
		{"unknown placeholder", "{artist}/{foo}", true},
		{"unclosed brace", "{artist/{title}", true},
		{"path traversal", "{artist}/../{title}", true},
		{"double slash", "{artist}//{title}", true},
		{"leading slash", "/{artist}/{title}", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePathTemplate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePathTemplate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestPathTemplateRender(t *testing.T) {
	tests := []struct {
		name     string
		template string
		song     *models.Song
		want     string
	}{
		{
			name:     "default template",
			template: "{artist}-{album}/{title}",
			song:     &models.Song{Title: "夜曲", Artist: "周杰伦", Album: "十一月的萧邦"},
			want:     "周杰伦-十一月的萧邦/夜曲",
		},
		{
			name:     "with year",
			template: "{artist}/{album} ({year})/{title}",
			song:     &models.Song{Title: "夜曲", Artist: "周杰伦", Album: "十一月的萧邦", Year: 2005},
			want:     "周杰伦/十一月的萧邦 (2005)/夜曲",
		},
		{
			name:     "empty artist and album",
			template: "{artist}-{album}/{title}",
			song:     &models.Song{Title: "Test Song"},
			want:     "Unknown Artist-Unknown Album/Test Song",
		},
		{
			name:     "empty title uses default",
			template: "{title}",
			song:     &models.Song{},
			want:     "Unknown",
		},
		{
			name:     "unsafe characters sanitized",
			template: "{artist}/{title}",
			song:     &models.Song{Title: "Hello: World?", Artist: "Art*ist"},
			want:     "Art_ist/Hello_World",
		},
		{
			name:     "slash in title not treated as separator",
			template: "{artist}-{album}/{title}",
			song:     &models.Song{Title: "AC/DC Song", Artist: "Various", Album: "Best"},
			want:     "Various-Best/AC_DC Song",
		},
		{
			name:     "slash in artist not treated as separator",
			template: "{artist}/{title}",
			song:     &models.Song{Title: "Track", Artist: "A/B"},
			want:     "A_B/Track",
		},
		{
			name:     "nil song",
			template: "{artist}-{album}/{title}",
			song:     nil,
			want:     "Unknown Artist-Unknown Album/Unknown",
		},
		{
			name:     "year zero omitted",
			template: "{artist}-{year}/{title}",
			song:     &models.Song{Title: "Song", Artist: "Artist", Year: 0},
			want:     "Artist/Song",
		},
		{
			name:     "simple title only",
			template: "{title}",
			song:     &models.Song{Title: "My Song"},
			want:     "My Song",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl, err := ParsePathTemplate(tt.template)
			if err != nil {
				t.Fatalf("ParsePathTemplate(%q) error: %v", tt.template, err)
			}
			got := tpl.Render(tt.song)
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizePathComponent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"hello:world", "hello_world"},
		{`a*b?c"d`, "a_b_c_d"},
		{"  spaces  ", "spaces"},
		{"trailing...", "trailing"},
		{"", ""},
		{"中文测试", "中文测试"},
		{"a/b", "a/b"}, // "/" is NOT sanitized here (handled at template level)
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizePathComponent(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePathComponent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
