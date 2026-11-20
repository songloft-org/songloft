package database

import (
	"reflect"
	"testing"
)

func TestParseSongTypes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "empty", raw: "", want: nil},
		{name: "single", raw: "radio", want: []string{"radio"}},
		{name: "csv trims and deduplicates", raw: " local, remote,local ", want: []string{"local", "remote"}},
		{name: "invalid", raw: "unsupported", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSongTypes(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSongTypes(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseSongTypes(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
