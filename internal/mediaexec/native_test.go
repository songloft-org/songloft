//go:build linux && bundle_ffmpeg && cgo

package mediaexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeFixture(t *testing.T, dst string, args ...string) {
	t.Helper()
	args = append([]string{"-hide_banner", "-loglevel", "error"}, args...)
	cmd := exec.Command("ffmpeg", append(args, "-y", dst)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v: %s", err, out)
	}
}

func TestNativeMediaOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dir := t.TempDir()
	input := filepath.Join(dir, "source.wav")
	makeFixture(t, input, "-f", "lavfi", "-i", "sine=frequency=450:duration=10")

	for _, format := range []struct{ encoder, muxer, codec string }{
		{"libmp3lame", "mp3", "mp3"},
		{"libvorbis", "ogg", "vorbis"},
		{"aac", "ipod", "aac"},
		{"flac", "flac", "flac"},
		{"pcm_s16le", "wav", "pcm_s16le"},
	} {
		t.Run(format.muxer, func(t *testing.T) {
			output := filepath.Join(dir, format.muxer)
			cmd := FFmpeg(ctx, "", "-ss", "1", "-i", input, "-t", "2", "-map", "0:a:0",
				"-c:a", format.encoder, "-f", format.muxer, output)
			if _, err := cmd.CombinedOutput(); err != nil {
				t.Fatal(err)
			}
			raw, err := FFprobe(ctx, "", "-show_streams", "-select_streams", "a", output).Output()
			if err != nil {
				t.Fatal(err)
			}
			var probe struct {
				Streams []struct {
					Codec string `json:"codec_name"`
				}
			}
			if err := json.Unmarshal(raw, &probe); err != nil || len(probe.Streams) != 1 || probe.Streams[0].Codec != format.codec {
				t.Fatalf("unexpected probe result: %s (%v)", raw, err)
			}
		})
	}

	cmd := FFmpeg(ctx, "", "-ss", "1", "-i", input, "-map", "0:a:0", "-map_metadata", "-1",
		"-t", "7", "-f", "chromaprint", "-fp_format", "base64", "-")
	var duration bytes.Buffer
	cmd.Stderr = &duration
	if out, err := cmd.Output(); err != nil || len(strings.TrimSpace(string(out))) == 0 || !strings.Contains(duration.String(), "00:00:10.") {
		t.Fatalf("chromaprint: bytes=%d duration=%s err=%v", len(out), &duration, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "Songloft CI" || r.Referer() != "https://example.test/" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, input)
	}))
	defer server.Close()
	streamCtx, stopStream := context.WithCancel(ctx)
	defer stopStream()
	stream := FFmpeg(streamCtx, "", "-user_agent", "Songloft CI", "-headers", "Referer: https://example.test/\r\n",
		"-i", server.URL+"/source.wav", "-map", "0:a:0", "-c:a", "libmp3lame", "-af", "atempo=1.25",
		"-b:a", "128k", "-write_xing", "0", "-f", "mp3", "pipe:1")
	pipe, err := stream.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()
	if err := stream.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(pipe, make([]byte, 4096)); err != nil {
		stopStream()
		t.Fatalf("stream: %v (wait: %v)", err, stream.Wait())
	}
	stopStream()
	if err := stream.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("stream wait: %v", err)
	}
}

func TestNativeVideoHLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dir := t.TempDir()
	input := filepath.Join(dir, "source.mkv")
	makeFixture(t, input, "-f", "lavfi", "-i", "testsrc2=size=128x96:rate=25:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=450:duration=2", "-c:v", "mpeg4", "-c:a", "aac")
	playlist := filepath.Join(dir, "master.m3u8")
	_, err := FFmpeg(ctx, "", "-i", input, "-map", "0:v:0", "-map", "0:a:0", "-c:v", "libx264",
		"-preset", "veryfast", "-crf", "23", "-c:a", "aac", "-ac", "2", "-b:a", "128k",
		"-f", "hls", "-hls_time", "4", "-hls_list_size", "0", "-hls_playlist_type", "event",
		"-hls_segment_filename", filepath.Join(dir, "%04d.ts"), "-y", playlist).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(playlist)
	if err != nil || !bytes.Contains(data, []byte("#EXT-X-ENDLIST")) {
		t.Fatalf("playlist: %s (%v)", data, err)
	}
	segment, err := FFprobe(ctx, "", "-show_streams", filepath.Join(dir, "0000.ts")).Output()
	if err != nil || !bytes.Contains(segment, []byte(`"codec_name":"h264"`)) ||
		!bytes.Contains(segment, []byte(`"codec_name":"aac"`)) {
		t.Fatalf("segment: %s (%v)", segment, err)
	}
}
