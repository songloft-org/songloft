package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hanxi/tag"
)

type formatTags struct {
	Title       string `json:"TITLE,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	Composer    string `json:"composer,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Date        string `json:"date,omitempty"`
	Track       string `json:"track,omitempty"`
	Disc        string `json:"disc,omitempty"`
	Comment     string `json:"comment,omitempty"`
	Lyrics      string `json:"lyrics,omitempty"`
}

type formatInfo struct {
	Filename       string      `json:"filename"`
	FormatName     string      `json:"format_name"`
	FormatLongName string      `json:"format_long_name"`
	Duration       string      `json:"duration,omitempty"`
	Size           string      `json:"size,omitempty"`
	BitRate        string      `json:"bit_rate,omitempty"`
	Tags           *formatTags `json:"tags,omitempty"`
	Cover          string      `json:"cover,omitempty"`
}

type streamInfo struct {
	Index      int    `json:"index"`
	CodecType  string `json:"codec_type"`
	SampleRate string `json:"sample_rate,omitempty"`
	BitRate    string `json:"bit_rate,omitempty"`
	Duration   string `json:"duration,omitempty"`
}

type output struct {
	Streams []streamInfo `json:"streams"`
	Format  formatInfo   `json:"format"`
}

var formatLongNames = map[tag.FileType]string{
	tag.MP3:  "MP3 (MPEG audio layer 3)",
	tag.M4A:  "QuickTime / MOV",
	tag.M4B:  "QuickTime / MOV",
	tag.M4P:  "QuickTime / MOV",
	tag.ALAC: "QuickTime / MOV",
	tag.FLAC: "raw FLAC",
	tag.OGG:  "Ogg",
	tag.DSF:  "DSD Stream File (DSF)",
	tag.WAV:  "WAV / WAVE (Waveform Audio)",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <audio-file>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting file info: %v\n", err)
		os.Exit(1)
	}

	metadata, err := tag.ReadFrom(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading tags: %v\n", err)
		os.Exit(1)
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}

	tags := buildTags(metadata)

	fileType := metadata.FileType()
	longName := formatLongNames[fileType]
	if longName == "" {
		longName = string(fileType)
	}

	info := formatInfo{
		Filename:       absPath,
		FormatName:     strings.ToLower(string(fileType)),
		FormatLongName: longName,
		Size:           fmt.Sprintf("%d", fileInfo.Size()),
		Tags:           tags,
	}

	duration := metadata.Duration()
	if duration > 0 {
		info.Duration = fmt.Sprintf("%f", duration.Seconds())
	}

	// bit_rate 以 bps 字符串形式输出,与 ffprobe 对齐(tag 库内部为 kbps,这里 *1000)
	if br := metadata.BitRate(); br > 0 {
		info.BitRate = fmt.Sprintf("%d", br*1000)
	}

	audioStream := streamInfo{
		Index:     0,
		CodecType: "audio",
		Duration:  info.Duration,
	}
	if sr := metadata.SampleRate(); sr > 0 {
		audioStream.SampleRate = fmt.Sprintf("%d", sr)
	}
	if br := metadata.BitRate(); br > 0 {
		audioStream.BitRate = fmt.Sprintf("%d", br*1000)
	}

	if picture := metadata.Picture(); picture != nil {
		ext := picture.Ext
		if ext == "" {
			ext = "jpg"
		}
		coverFile, err := os.CreateTemp("", "tag-cover-*."+ext)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating cover temp file: %v\n", err)
			os.Exit(1)
		}
		if _, err := coverFile.Write(picture.Data); err != nil {
			coverFile.Close()
			fmt.Fprintf(os.Stderr, "error writing cover data: %v\n", err)
			os.Exit(1)
		}
		coverFile.Close()
		info.Cover = coverFile.Name()
	}

	result := output{
		Streams: []streamInfo{audioStream},
		Format:  info,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func buildTags(metadata tag.Metadata) *formatTags {
	tags := &formatTags{
		Title:       metadata.Title(),
		Artist:      metadata.Artist(),
		Album:       metadata.Album(),
		AlbumArtist: metadata.AlbumArtist(),
		Composer:    metadata.Composer(),
		Genre:       metadata.Genre(),
		Lyrics:      metadata.Lyrics(),
		Comment:     metadata.Comment(),
	}

	if year := metadata.Year(); year > 0 {
		tags.Date = fmt.Sprintf("%d", year)
	}

	track, trackTotal := metadata.Track()
	if track > 0 {
		if trackTotal > 0 {
			tags.Track = fmt.Sprintf("%d/%d", track, trackTotal)
		} else {
			tags.Track = fmt.Sprintf("%d", track)
		}
	}

	disc, discTotal := metadata.Disc()
	if disc > 0 {
		if discTotal > 0 {
			tags.Disc = fmt.Sprintf("%d/%d", disc, discTotal)
		} else {
			tags.Disc = fmt.Sprintf("%d", disc)
		}
	}

	if isEmpty(tags) {
		return nil
	}
	return tags
}

func isEmpty(tags *formatTags) bool {
	return tags.Title == "" &&
		tags.Artist == "" &&
		tags.Album == "" &&
		tags.AlbumArtist == "" &&
		tags.Composer == "" &&
		tags.Genre == "" &&
		tags.Date == "" &&
		tags.Track == "" &&
		tags.Disc == "" &&
		tags.Comment == "" &&
		tags.Lyrics == ""
}
