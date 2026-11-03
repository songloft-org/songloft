package cue

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	xunicode "golang.org/x/text/encoding/unicode"
)

func parseEncodedFile(t *testing.T, data []byte) (*CUESheet, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "album.cue")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return ParseFile(path)
}

func TestParseFileCP950Sample(t *testing.T) {
	// The reporter supplied equivalent CP950 and UTF-8 files in songloft-org/songloft#554.
	want, err := ParseFile("testdata/issue554.utf8.cue")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseFile("testdata/issue554.cp950.cue")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CP950 metadata differs from UTF-8 reference:\ngot: %+v\nwant: %+v", got, want)
	}
	if got.Title != "CD10 金牌台灣歌16首-2" || len(got.Files[0].Tracks) != 16 {
		t.Fatalf("unexpected reference metadata: %+v", got)
	}
	data, err := os.ReadFile("testdata/issue554.cp950.cue")
	if err != nil {
		t.Fatal(err)
	}
	got, err = parseEncodedFile(t, bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n")))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("CRLF sample differs from UTF-8 reference: %+v, %v", got, err)
	}
}

func TestParseFileEncodings(t *testing.T) {
	tests := []struct {
		name string
		text string
		enc  encoding.Encoding
	}{
		{"Big5", "繁體中文音樂專輯 群星 月亮代表我的心 愛的故事 台灣金曲精選", traditionalchinese.Big5},
		{"GB18030", "简体中文音乐专辑 群星 月亮代表我的心 爱的故事 台湾金曲精选 𠀀", simplifiedchinese.GB18030},
		{"ShiftJIS", "日本の音楽 アルバム 坂本龍一 桜の花が咲く頃 夢の続き 愛の歌", japanese.ShiftJIS},
		{"EUCKR", "한국 음악 앨범 사랑의 노래 아름다운 세상 그리운 마음 행복한 하루", korean.EUCKR},
		{"UTF16LE", "繁體中文音樂專輯", xunicode.UTF16(xunicode.LittleEndian, xunicode.UseBOM)},
		{"UTF16BE", "繁體中文音樂專輯", xunicode.UTF16(xunicode.BigEndian, xunicode.UseBOM)},
		{"UTF8", "ÀÁÂÃÄÅ Édith Piaf", nil},
		{"ASCII", "Album", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Include a non-ASCII filename, album, artist and track title.
			content := "TITLE \"" + tt.text + "\"\nPERFORMER \"" + tt.text + "\"\nFILE \"" + tt.text + ".wav\" WAVE\n TRACK 01 AUDIO\n TITLE \"" + tt.text + "\"\n INDEX 01 00:00:00\n"
			data := []byte(content)
			if tt.enc != nil {
				var err error
				data, err = tt.enc.NewEncoder().Bytes(data)
				if err != nil {
					t.Fatal(err)
				}
			}
			got, err := parseEncodedFile(t, data)
			if err != nil {
				t.Fatal(err)
			}
			want, err := Parse(content)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("decoded metadata differs: got %+v, want %+v", got, want)
			}
		})
	}
}

func TestParseFileCP950Extensions(t *testing.T) {
	data, err := os.ReadFile("testdata/issue554.cp950.cue")
	if err != nil {
		t.Fatal(err)
	}
	// CP950 extensions F9D6-F9DC and A3E1, encoded independently of x/text.
	data = bytes.Replace(data, []byte("CD10 "), []byte("CD10 \xf9\xd6\xf9\xd7\xf9\xd8\xf9\xd9\xf9\xda\xf9\xdb\xf9\xdc\xa3\xe1 "), 1)
	got, err := parseEncodedFile(t, data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CD10 碁銹裏墻恒粧嫺€ 金牌台灣歌16首-2"; got.Title != want {
		t.Fatalf("CP950 extensions: got %q, want %q", got.Title, want)
	}
}

func TestParseFileBig5AlsoDecodesAsGB18030(t *testing.T) {
	content := "TITLE \"中文歌曲精選\"\nPERFORMER \"群星\"\nFILE \"album.wav\" WAVE\n TRACK 01 AUDIO\n TITLE \"中文歌曲精選\"\n INDEX 01 00:00:00\n"
	data, err := traditionalchinese.Big5.NewEncoder().Bytes([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	// Both decoders succeed without replacement characters. Byte validity alone
	// must not allow GB18030 to win just because it appears first in a list.
	wrong, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
	if err != nil || bytes.Contains(wrong, []byte("�")) || string(wrong) == content {
		t.Fatalf("fixture must also decode losslessly to different GB18030 text: %q, %v", wrong, err)
	}
	got, err := parseEncodedFile(t, data)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("valid GB18030 decoding incorrectly won over Big5: %+v", got)
	}
}

func TestParseFileUncertainEncoding(t *testing.T) {
	base := "TITLE \"%s\"\nFILE \"album.wav\" WAVE\n TRACK 01 AUDIO\n INDEX 01 00:00:00\n"
	for name, data := range map[string][]byte{
		"ambiguous":        []byte(strings.Replace(base, "%s", "\xa4\xa4\xa4\xe5", 1)), // Big5 中文, also valid GB18030 bytes.
		"invalid":          []byte(strings.Replace(base, "%s", "\xff\xff\xff", 1)),
		"invalid UTF8 BOM": append([]byte{0xef, 0xbb, 0xbf}, []byte(strings.Replace(base, "%s", "\xff", 1))...),
		"truncated UTF16":  {0xff, 0xfe, 0x54},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseEncodedFile(t, data)
			if err == nil || !strings.Contains(err.Error(), "UTF-8") {
				t.Fatalf("want actionable encoding error mentioning UTF-8, got %v", err)
			}
		})
	}
}
