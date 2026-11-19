package services

import "testing"

func TestInternalURLResolverUpdatesBundlePort(t *testing.T) {
	resolver := NewInternalURLResolver(0, "token")
	if got := resolver.Resolve("/api/v1/jsplugin/miot/player/status"); got != "/api/v1/jsplugin/miot/player/status" {
		t.Fatalf("expected unresolved URL before listener starts, got %q", got)
	}

	resolver.SetServerPort(33807)
	want := "http://127.0.0.1:33807/api/v1/jsplugin/miot/player/status?access_token=token"
	if got := resolver.Resolve("/api/v1/jsplugin/miot/player/status"); got != want {
		t.Fatalf("resolved URL = %q, want %q", got, want)
	}
}
