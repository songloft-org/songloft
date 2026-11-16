package services

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hanxi/tag"
)

// trackFromMeta 把读回的 (number, total) 格式化为 WriteOptions.Track 形态。
func trackFromMeta(m tag.Metadata) string {
	num, total := m.Track()
	if num <= 0 {
		return ""
	}
	if total > 0 {
		return strconv.Itoa(num) + "/" + strconv.Itoa(total)
	}
	return strconv.Itoa(num)
}

func copyTestFile(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestTagsUnchanged_IdenticalTags(t *testing.T) {
	path := copyTestFile(t, "../../pkg/tag/testdata/with_tags/sample.id3v23.mp3")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := tag.ReadFrom(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	opts := tag.WriteOptions{
		Title:       m.Title(),
		Artist:      m.Artist(),
		AlbumArtist: m.AlbumArtist(),
		Album:       m.Album(),
		Genre:       m.Genre(),
		Lyrics:      m.Lyrics(),
		Year:        m.Year(),
		Track:       trackFromMeta(m),
		Disc:        canonicalTrack(m.Disc()),
	}
	if pic := m.Picture(); pic != nil {
		opts.Picture = pic
	}

	if !tagsUnchanged(path, opts) {
		t.Error("expected tagsUnchanged=true for identical tags")
	}
}

func TestTagsUnchanged_DifferentTitle(t *testing.T) {
	path := copyTestFile(t, "../../pkg/tag/testdata/with_tags/sample.id3v23.mp3")

	opts := tag.WriteOptions{
		Title: "Completely Different Title",
	}

	if tagsUnchanged(path, opts) {
		t.Error("expected tagsUnchanged=false when title differs")
	}
}

func TestTagsUnchanged_DifferentYear(t *testing.T) {
	path := copyTestFile(t, "../../pkg/tag/testdata/with_tags/sample.id3v23.mp3")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := tag.ReadFrom(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	opts := tag.WriteOptions{
		Title:       m.Title(),
		Artist:      m.Artist(),
		AlbumArtist: m.AlbumArtist(),
		Album:       m.Album(),
		Genre:       m.Genre(),
		Lyrics:      m.Lyrics(),
		Year:        m.Year() + 1,
	}

	if tagsUnchanged(path, opts) {
		t.Error("expected tagsUnchanged=false when year differs")
	}
}

func TestTagsUnchanged_DifferentPicture(t *testing.T) {
	path := copyTestFile(t, "../../pkg/tag/testdata/with_tags/sample.id3v23.mp3")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := tag.ReadFrom(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	opts := tag.WriteOptions{
		Title:       m.Title(),
		Artist:      m.Artist(),
		AlbumArtist: m.AlbumArtist(),
		Album:       m.Album(),
		Genre:       m.Genre(),
		Lyrics:      m.Lyrics(),
		Year:        m.Year(),
		Picture: &tag.Picture{
			MIMEType: "image/png",
			Data:     []byte("fake-new-cover-data"),
		},
	}

	if tagsUnchanged(path, opts) {
		t.Error("expected tagsUnchanged=false when picture differs")
	}
}

func TestTagsUnchanged_NonexistentFile(t *testing.T) {
	opts := tag.WriteOptions{Title: "test"}
	if tagsUnchanged("/nonexistent/path.mp3", opts) {
		t.Error("expected tagsUnchanged=false for nonexistent file")
	}
}

func TestPictureEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b *tag.Picture
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil", nil, &tag.Picture{Data: []byte("x")}, false},
		{"b nil", &tag.Picture{Data: []byte("x")}, nil, false},
		{"same", &tag.Picture{MIMEType: "image/jpeg", Data: []byte("abc")}, &tag.Picture{MIMEType: "image/jpeg", Data: []byte("abc")}, true},
		{"diff mime", &tag.Picture{MIMEType: "image/jpeg", Data: []byte("abc")}, &tag.Picture{MIMEType: "image/png", Data: []byte("abc")}, false},
		{"diff data", &tag.Picture{MIMEType: "image/jpeg", Data: []byte("abc")}, &tag.Picture{MIMEType: "image/jpeg", Data: []byte("xyz")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pictureEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("pictureEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagsUnchanged_FLAC(t *testing.T) {
	path := copyTestFile(t, "../../pkg/tag/testdata/with_tags/sample.flac")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := tag.ReadFrom(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	opts := tag.WriteOptions{
		Title:       m.Title(),
		Artist:      m.Artist(),
		AlbumArtist: m.AlbumArtist(),
		Album:       m.Album(),
		Genre:       m.Genre(),
		Lyrics:      m.Lyrics(),
		Year:        m.Year(),
		Track:       trackFromMeta(m),
	}
	if pic := m.Picture(); pic != nil {
		opts.Picture = pic
	}

	if !tagsUnchanged(path, opts) {
		t.Error("expected tagsUnchanged=true for identical FLAC tags")
	}

	opts.Artist = "Different Artist"
	if tagsUnchanged(path, opts) {
		t.Error("expected tagsUnchanged=false when FLAC artist differs")
	}
}
