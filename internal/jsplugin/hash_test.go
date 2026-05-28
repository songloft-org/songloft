package jsplugin

import (
	"archive/zip"
	"bytes"
	"testing"
)

// makeZip 创建一个包含指定文件的测试 zip。
func makeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// makeZipOrdered 创建一个按指定顺序写入文件的测试 zip（验证顺序无关性）。
func makeZipOrdered(t *testing.T, entries []struct {
	name string
	data []byte
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := w.Create(e.name)
		if err != nil {
			t.Fatalf("create %s: %v", e.name, err)
		}
		if _, err := fw.Write(e.data); err != nil {
			t.Fatalf("write %s: %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestComputeCanonicalZipHash_ExcludesPluginJSON(t *testing.T) {
	mainJS := []byte("function onInit(){}")
	pluginJSON1 := []byte(`{"name":"A","version":"1.0.0"}`)
	pluginJSON2 := []byte(`{"name":"B","version":"2.0.0","extra":"field"}`)

	zip1 := makeZip(t, map[string][]byte{
		"plugin.json": pluginJSON1,
		"main.js":     mainJS,
	})
	zip2 := makeZip(t, map[string][]byte{
		"plugin.json": pluginJSON2,
		"main.js":     mainJS,
	})

	hash1, err := ComputeCanonicalZipHash(zip1)
	if err != nil {
		t.Fatalf("hash zip1: %v", err)
	}
	hash2, err := ComputeCanonicalZipHash(zip2)
	if err != nil {
		t.Fatalf("hash zip2: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("changing plugin.json should NOT affect zipHash\n  hash1=%s\n  hash2=%s", hash1, hash2)
	}
}

func TestComputeCanonicalZipHash_FileOrderIrrelevant(t *testing.T) {
	mainJS := []byte("function onInit(){}")
	staticHTML := []byte("<html></html>")

	zip1 := makeZipOrdered(t, []struct {
		name string
		data []byte
	}{
		{"main.js", mainJS},
		{"static/index.html", staticHTML},
		{"plugin.json", []byte("{}")},
	})
	zip2 := makeZipOrdered(t, []struct {
		name string
		data []byte
	}{
		{"plugin.json", []byte("{}")},
		{"static/index.html", staticHTML},
		{"main.js", mainJS},
	})

	hash1, err := ComputeCanonicalZipHash(zip1)
	if err != nil {
		t.Fatalf("hash zip1: %v", err)
	}
	hash2, err := ComputeCanonicalZipHash(zip2)
	if err != nil {
		t.Fatalf("hash zip2: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("file order in zip should NOT affect zipHash\n  hash1=%s\n  hash2=%s", hash1, hash2)
	}
}

func TestComputeCanonicalZipHash_ContentChangeAffectsHash(t *testing.T) {
	zip1 := makeZip(t, map[string][]byte{
		"plugin.json": []byte("{}"),
		"main.js":     []byte("function onInit(){}"),
	})
	zip2 := makeZip(t, map[string][]byte{
		"plugin.json": []byte("{}"),
		"main.js":     []byte("function onInit(){/* modified */}"),
	})

	hash1, err := ComputeCanonicalZipHash(zip1)
	if err != nil {
		t.Fatalf("hash zip1: %v", err)
	}
	hash2, err := ComputeCanonicalZipHash(zip2)
	if err != nil {
		t.Fatalf("hash zip2: %v", err)
	}

	if hash1 == hash2 {
		t.Error("modifying main.js should change zipHash")
	}
}

func TestComputeCanonicalZipHash_StaticFileChangeAffectsHash(t *testing.T) {
	zip1 := makeZip(t, map[string][]byte{
		"plugin.json":       []byte("{}"),
		"main.js":           []byte("ok"),
		"static/index.html": []byte("<html>A</html>"),
	})
	zip2 := makeZip(t, map[string][]byte{
		"plugin.json":       []byte("{}"),
		"main.js":           []byte("ok"),
		"static/index.html": []byte("<html>B</html>"),
	})

	hash1, _ := ComputeCanonicalZipHash(zip1)
	hash2, _ := ComputeCanonicalZipHash(zip2)

	if hash1 == hash2 {
		t.Error("modifying static file should change zipHash")
	}
}

func TestComputeEntryHash(t *testing.T) {
	mainJS := []byte("hello world")
	expected := sha256HexSum(mainJS)

	zipData := makeZip(t, map[string][]byte{
		"plugin.json": []byte("{}"),
		"main.js":     mainJS,
	})

	got, err := ComputeEntryHash(zipData, "main.js")
	if err != nil {
		t.Fatalf("ComputeEntryHash: %v", err)
	}
	if got != expected {
		t.Errorf("entryHash = %s, want %s", got, expected)
	}
}

func TestComputeEntryHash_NotFound(t *testing.T) {
	zipData := makeZip(t, map[string][]byte{
		"plugin.json": []byte("{}"),
		"main.js":     []byte("x"),
	})

	_, err := ComputeEntryHash(zipData, "index.js")
	if err == nil {
		t.Fatal("expected error for missing entry file")
	}
}

func TestValidateHashField_Valid(t *testing.T) {
	if err := ValidateHashField("entryHash", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Errorf("should be valid: %v", err)
	}
}

func TestValidateHashField_Empty(t *testing.T) {
	err := ValidateHashField("entryHash", "")
	if err == nil {
		t.Fatal("expected error for empty hash")
	}
}

func TestValidateHashField_InvalidHex(t *testing.T) {
	cases := []string{
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", // uppercase
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",   // too short (62)
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", // non-hex
	}
	for _, c := range cases {
		if err := ValidateHashField("zipHash", c); err == nil {
			t.Errorf("expected error for %q, got nil", c)
		}
	}
}

func TestSha256HexSum(t *testing.T) {
	// sha256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	got := sha256HexSum([]byte(""))
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("sha256HexSum('') = %s, want %s", got, want)
	}
}
