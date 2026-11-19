package jsplugin

import "testing"

func TestBridgeHandlerPortCanBeUpdatedAfterBundleListen(t *testing.T) {
	h := &BridgeHandler{port: "0"}

	if got := mustHostURL(t, h); got != "http://localhost:0" {
		t.Fatalf("initial host URL = %q, want port 0", got)
	}
	h.setPort("33807")
	if got := mustHostURL(t, h); got != "http://localhost:33807" {
		t.Fatalf("updated host URL = %q, want actual listener port", got)
	}
}

func mustHostURL(t *testing.T, h *BridgeHandler) string {
	t.Helper()
	got, err := h.handlePlugin("plugin.getHostUrl", "")
	if err != nil {
		t.Fatalf("get host URL: %v", err)
	}
	return got
}

func TestEncodeNetworkAddresses_NilIsEmptyArray(t *testing.T) {
	if got := encodeNetworkAddresses(nil); got != "[]" {
		t.Fatalf("encodeNetworkAddresses(nil) = %q, want []", got)
	}
}

func TestEncodeNetworkAddresses_PreservesAddresses(t *testing.T) {
	got := encodeNetworkAddresses([]string{"http://192.168.1.10:58091"})
	if got != `["http://192.168.1.10:58091"]` {
		t.Fatalf("encodeNetworkAddresses() = %q, want address array", got)
	}
}
