//go:build dev

package app

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
)

func init() {
	go func() {
		addr := "localhost:13060"
		slog.Info("pprof enabled", "addr", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("pprof server failed", "error", err)
		}
	}()
}
