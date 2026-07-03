package jsruntime

import (
	"context"
	"testing"
)

// TestNativeSHA256Bytes 用已知向量验证 __go_crypto_sha256_bytes（hex 入 hex 出）。
func TestNativeSHA256Bytes(t *testing.T) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "test-sha256-bytes"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		t.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	cases := []struct{ inHex, wantHex string }{
		// sha256("")
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		// sha256("abc") = sha256(616263)
		{"616263", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}
	for _, c := range cases {
		res, err := m.ExecuteJS(context.Background(), envID, `__go_crypto_sha256_bytes("`+c.inHex+`")`, 1000)
		if err != nil {
			t.Fatalf("ExecuteJS(%q): %v", c.inHex, err)
		}
		if res.Result != c.wantHex {
			t.Errorf("sha256_bytes(%q) = %q, want %q", c.inHex, res.Result, c.wantHex)
		}
	}
}

// TestNativeSHA1 用已知向量验证 __go_crypto_sha1（UTF-8 字符串入，hex 出）。
func TestNativeSHA1(t *testing.T) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "test-sha1"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		t.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	cases := []struct{ in, wantHex string }{
		{"", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
	}
	for _, c := range cases {
		res, err := m.ExecuteJS(context.Background(), envID, `crypto.sha1(`+jsQuote(c.in)+`)`, 1000)
		if err != nil {
			t.Fatalf("ExecuteJS(%q): %v", c.in, err)
		}
		if res.Result != c.wantHex {
			t.Errorf("sha1(%q) = %q, want %q", c.in, res.Result, c.wantHex)
		}
	}
}

func jsQuote(s string) string { return "\"" + s + "\"" }

// TestNativeRC4 用维基百科 RC4 测试向量验证 __go_crypto_rc4（hex 入 hex 出）。
func TestNativeRC4(t *testing.T) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "test-rc4"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		t.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	// Key="Key" (4b6579), Plaintext="Plaintext" (506c61696e74657874)
	// → ciphertext BBF316E8D940AF0AD3 （RC4 经典测试向量，小写）
	res, err := m.ExecuteJS(context.Background(), envID,
		`__go_crypto_rc4("4b6579", "506c61696e74657874")`, 1000)
	if err != nil {
		t.Fatalf("ExecuteJS: %v", err)
	}
	want := "bbf316e8d940af0ad3"
	if res.Result != want {
		t.Errorf("rc4 = %q, want %q", res.Result, want)
	}

	// RC4 对称：再次用密文当明文加密应还原出原明文
	res2, err := m.ExecuteJS(context.Background(), envID,
		`__go_crypto_rc4("4b6579", "`+want+`")`, 1000)
	if err != nil {
		t.Fatalf("ExecuteJS roundtrip: %v", err)
	}
	if res2.Result != "506c61696e74657874" {
		t.Errorf("rc4 roundtrip = %q, want 506c61696e74657874", res2.Result)
	}
}

// TestNativeCryptoWrapper 验证 globalThis.crypto.sha256Bytes / rc4 包装可用。
func TestNativeCryptoWrapper(t *testing.T) {
	m := NewJSEnvManager()
	defer m.SignalShutdown()
	envID := "test-crypto-wrapper"
	if err := m.CreateEnv(envID, polyfillJS, 1); err != nil {
		t.Fatalf("CreateEnv: %v", err)
	}
	defer m.DestroyEnv(envID)

	// crypto.sha256Bytes({_hex}) 应等价于直接 __go_crypto_sha256_bytes
	res, err := m.ExecuteJS(context.Background(), envID,
		`crypto.sha256Bytes({_hex:"616263"}).toString('hex')`, 1000)
	if err != nil {
		t.Fatalf("ExecuteJS: %v", err)
	}
	if res.Result != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("crypto.sha256Bytes = %q", res.Result)
	}
}
