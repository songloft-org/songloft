package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hanxi/tag"
)

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
