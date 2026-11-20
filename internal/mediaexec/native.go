//go:build bundle_ffmpeg && cgo

package mediaexec

/*
#cgo ios CFLAGS: -I${SRCDIR}/ffmpeg/include
#cgo ios LDFLAGS: -L${SRCDIR}/ffmpeg -lsongloft_ffmpeg -lc++ -framework Security -framework CoreFoundation
#cgo linux pkg-config: libavformat libavcodec libavfilter libavutil
#include <stdlib.h>
#include <libavutil/error.h>
#include "native.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime/cgo"
	"strconv"
	"strings"
	"unsafe"
)

const Native = true

type nativeSession struct {
	context.Context
	stdout, stderr io.Writer
	writeErr       error
}

func (s *nativeSession) write(data []byte) error {
	if err := s.Err(); err != nil {
		return err
	}
	if s.stdout != nil {
		n, err := s.stdout.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		return err
	}
	return nil
}

//export slGoCancelled
func slGoCancelled(handle C.uintptr_t) C.int {
	if cgo.Handle(handle).Value().(*nativeSession).Err() != nil {
		return 1
	}
	return 0
}

//export slGoWrite
func slGoWrite(handle C.uintptr_t, data *C.uint8_t, length C.int) C.int {
	session := cgo.Handle(handle).Value().(*nativeSession)
	if err := session.write(unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length))); err != nil {
		session.writeErr = err
		return -1
	}
	return length
}

//export slGoDuration
func slGoDuration(handle C.uintptr_t, duration C.int64_t) {
	session := cgo.Handle(handle).Value().(*nativeSession)
	if session.stderr == nil || duration <= 0 {
		return
	}
	us := int64(duration)
	fmt.Fprintf(session.stderr, "Duration: %02d:%02d:%02d.%06d\n", us/3600000000,
		us/60000000%60, us/1000000%60, us%1000000)
}

func runNative(ctx context.Context, probe bool, args []string, stdout, stderr io.Writer) error {
	session := &nativeSession{Context: ctx, stdout: stdout, stderr: stderr}
	handle := cgo.NewHandle(session)
	defer handle.Delete()
	options, err := parseNativeArgs(probe, args)
	if err != nil {
		return err
	}
	if options.muxers {
		name := C.CString("chromaprint")
		defer C.free(unsafe.Pointer(name))
		if C.av_guess_format(name, nil, nil) != nil {
			return session.write([]byte("chromaprint\n"))
		}
		return nil
	}
	var allocated []*C.char
	defer func() {
		for _, ptr := range allocated {
			C.free(unsafe.Pointer(ptr))
		}
	}()
	cString := func(value string) *C.char {
		ptr := C.CString(value)
		allocated = append(allocated, ptr)
		return ptr
	}
	native := (*C.SLMediaOptions)(C.malloc(C.sizeof_SLMediaOptions))
	if native == nil {
		return errors.New("could not allocate FFmpeg options")
	}
	defer C.free(unsafe.Pointer(native))
	var inputOptions, outputOptions *C.AVDictionary
	defer C.av_dict_free(&inputOptions)
	defer C.av_dict_free(&outputOptions)
	for key, value := range options.inputOptions {
		if err := nativeError(ctx, C.av_dict_set(&inputOptions, cString(key), cString(value), 0)); err != nil {
			return err
		}
	}
	for key, value := range options.outputOptions {
		if err := nativeError(ctx, C.av_dict_set(&outputOptions, cString(key), cString(value), 0)); err != nil {
			return err
		}
	}
	*native = C.SLMediaOptions{
		handle: C.uintptr_t(handle),
		input:  cString(options.input), output: cString(options.output), format: cString(options.format),
		audio_codec: cString(options.audioCodec), video_codec: cString(options.videoCodec),
		audio_filter:  cString(options.audioFilter),
		input_options: inputOptions, output_options: outputOptions,
		audio_index: C.int(options.audioIndex), video_index: C.int(options.videoIndex),
		channels: C.int(options.channels), sample_rate: C.int(options.rate), frames: C.int(options.frames),
		audio_bitrate: C.int64_t(options.bitrate), start_us: C.int64_t(options.start),
		duration_us: C.int64_t(options.duration), realtime: C.int(options.realtime),
		audio_quality: C.int(options.audioQuality), video_quality: C.int(options.videoQuality), no_metadata: C.int(options.noMetadata),
	}
	if probe {
		var format *C.AVFormatContext
		if result := C.sl_open(native, &format); result < 0 {
			return nativeError(ctx, result)
		}
		defer C.avformat_close_input(&format)
		return writeProbe(session, format, options.audioOnly)
	}
	result := C.sl_transcode(native)
	if err := ctx.Err(); err != nil {
		return err
	}
	if session.writeErr != nil {
		return session.writeErr
	}
	return nativeError(ctx, result)
}

func nativeError(ctx context.Context, result C.int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if result >= 0 {
		return nil
	}
	var message [256]C.char
	C.av_strerror(result, &message[0], C.size_t(len(message)))
	return fmt.Errorf("FFmpeg: %s", C.GoString(&message[0]))
}

func tags(dict *C.AVDictionary) map[string]string {
	var entry *C.AVDictionaryEntry
	empty := C.CString("")
	defer C.free(unsafe.Pointer(empty))
	result := make(map[string]string)
	for {
		entry = C.av_dict_get(dict, empty, entry, C.AV_DICT_IGNORE_SUFFIX)
		if entry == nil {
			return result
		}
		result[C.GoString(entry.key)] = C.GoString(entry.value)
	}
}

func writeProbe(session *nativeSession, format *C.AVFormatContext, audioOnly bool) error {
	streams := make([]map[string]any, 0, int(format.nb_streams))
	for index := C.uint(0); index < format.nb_streams; index++ {
		stream := C.sl_stream(format, index)
		media := stream.codecpar
		if audioOnly && media.codec_type != C.AVMEDIA_TYPE_AUDIO {
			continue
		}
		field := "unknown"
		if media.field_order == C.AV_FIELD_PROGRESSIVE {
			field = "progressive"
		} else if media.field_order != C.AV_FIELD_UNKNOWN {
			field = "interlaced"
		}
		attached := 0
		if stream.disposition&C.AV_DISPOSITION_ATTACHED_PIC != 0 {
			attached = 1
		}
		streams = append(streams, map[string]any{
			"codec_type":  C.GoString(C.av_get_media_type_string(media.codec_type)),
			"codec_name":  C.GoString(C.avcodec_get_name(media.codec_id)),
			"sample_rate": strconv.Itoa(int(media.sample_rate)),
			"field_order": field,
			"disposition": map[string]int{"attached_pic": attached},
			"tags":        tags(stream.metadata),
		})
	}
	duration := ""
	if format.duration > 0 {
		duration = strconv.FormatFloat(float64(format.duration)/1000000, 'f', 6, 64)
	}
	output, err := json.Marshal(map[string]any{
		"format": map[string]any{
			"format_name": C.GoString(format.iformat.name),
			"duration":    duration,
			"bit_rate":    strconv.FormatInt(int64(format.bit_rate), 10),
			"tags":        tags(format.metadata),
		},
		"streams": streams,
	})
	if err != nil {
		return err
	}
	return session.write(output)
}

type nativeArgs struct {
	input, output, format                                                    string
	audioCodec, videoCodec, audioFilter                                      string
	inputOptions, outputOptions                                              map[string]string
	audioIndex, videoIndex                                                   int
	channels, rate, frames, realtime, audioQuality, videoQuality, noMetadata int
	bitrate, start, duration                                                 int64
	muxers, audioOnly                                                        bool
}

// parseNativeArgs translates the fixed options emitted by Songloft. Unknown
// options fail explicitly so adding a CLI operation requires a native mapping.
func parseNativeArgs(probe bool, args []string) (nativeArgs, error) {
	o := nativeArgs{
		audioIndex: -1, videoIndex: -1, audioQuality: -1, videoQuality: -1,
		inputOptions: make(map[string]string), outputOptions: make(map[string]string),
	}
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return o, errors.New("invalid NUL byte in FFmpeg argument")
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-hide_banner", "-nostdin", "-y", "-an", "-vn", "-sn", "-dn",
			"-show_format", "-show_streams":
			continue
		case "-re":
			o.realtime = 1
			continue
		case "-muxers":
			o.muxers = true
			continue
		}
		if i == len(args)-1 {
			if probe {
				o.input = arg
			} else {
				o.output = arg
			}
			break
		}
		i++
		value := args[i]
		var err error
		switch arg {
		case "-i":
			o.input = value
		case "-f":
			o.format = value
		case "-map":
			if strings.HasPrefix(value, "0:a:") {
				o.audioIndex, err = strconv.Atoi(strings.TrimPrefix(value, "0:a:"))
			} else if strings.HasPrefix(value, "0:v:") {
				o.videoIndex, err = strconv.Atoi(strings.TrimPrefix(value, "0:v:"))
			} else {
				err = errors.New("unsupported stream mapping")
			}
		case "-codec:a", "-c:a":
			o.audioCodec = value
		case "-vcodec", "-c:v":
			o.videoCodec = value
		case "-af":
			o.audioFilter = value
		case "-vf":
			if value != "scale='min(720,iw)':-2" {
				err = errors.New("unsupported video filter")
			}
		case "-ac":
			o.channels, err = strconv.Atoi(value)
		case "-ar":
			o.rate, err = strconv.Atoi(value)
		case "-frames:v":
			o.frames, err = strconv.Atoi(value)
		case "-q:a":
			o.audioQuality, err = strconv.Atoi(value)
		case "-q:v":
			o.videoQuality, err = strconv.Atoi(value)
		case "-b:a":
			o.bitrate, err = strconv.ParseInt(strings.TrimSuffix(value, "k"), 10, 32)
			if strings.HasSuffix(value, "k") {
				o.bitrate *= 1000
			}
		case "-ss", "-t":
			var seconds float64
			seconds, err = strconv.ParseFloat(value, 64)
			if arg == "-ss" {
				o.start = int64(seconds * 1000000)
			} else {
				o.duration = int64(seconds * 1000000)
			}
		case "-select_streams":
			if value != "a" {
				err = errors.New("unsupported probe stream selection")
			}
			o.audioOnly = true
		case "-headers", "-user_agent", "-http_proxy", "-rw_timeout", "-protocol_whitelist",
			"-format_whitelist", "-analyzeduration", "-probesize":
			o.inputOptions[arg[1:]] = value
		case "-write_xing", "-id3v2_version", "-fp_format", "-hls_segment_type", "-hls_time",
			"-hls_list_size", "-hls_delete_threshold", "-hls_flags", "-hls_segment_filename", "-hls_playlist_type":
			o.outputOptions[arg[1:]] = value
		case "-profile:a", "-crf", "-preset":
			o.outputOptions[arg[1:]] = value
		case "-map_metadata":
			if value != "-1" {
				err = errors.New("unsupported metadata mapping")
			}
			o.noMetadata = 1
		case "-loglevel", "-v", "-print_format", "-of", "-show_entries", "-vsync":
			// The native runner reports errors directly, probes JSON, and
			// produces timestamps at the input cadence.
		default:
			err = errors.New("unsupported FFmpeg option")
		}
		if err != nil {
			return o, fmt.Errorf("native FFmpeg %s: %w", arg, err)
		}
	}
	if !o.muxers && (o.input == "" || (!probe && (o.output == "" || o.format == ""))) {
		return o, errors.New("native FFmpeg requires an input and output format")
	}
	return o, nil
}
