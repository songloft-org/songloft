package app

import (
	"embed"
	"testing"

	"songloft/internal/config"
	"songloft/internal/services"
)

func TestSetListenerPortUpdatesRuntimeURLResolver(t *testing.T) {
	cfg := config.NewAppConfig("0", "", "admin", "password", "")
	a := NewApp(cfg, embed.FS{})
	a.urlResolver = services.NewInternalURLResolver(0, "token")

	a.SetListenerPort(33807)

	if cfg.Port != "33807" {
		t.Fatalf("config port = %q, want actual listener port", cfg.Port)
	}
	want := "http://127.0.0.1:33807/api/v1/health?access_token=token"
	if got := a.urlResolver.Resolve("/api/v1/health"); got != want {
		t.Fatalf("resolved URL = %q, want %q", got, want)
	}
}
