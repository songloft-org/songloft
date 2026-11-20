//go:build !bundle_ffmpeg || !cgo

package mediaexec

import (
	"context"
	"errors"
	"io"
)

const Native = false

func runNative(context.Context, bool, []string, io.Writer, io.Writer) error {
	return errors.New("native FFmpeg is unavailable")
}
