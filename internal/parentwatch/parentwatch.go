// Package parentwatch implements the opt-in desktop parent lifetime pipe.
package parentwatch

import "io"

// Done closes when the inherited pipe closes or breaks. No bytes are retained.
// A process crash closes its write handle on Windows and Unix alike.
func Done(parent io.Reader) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, parent)
		close(done)
	}()
	return done
}
