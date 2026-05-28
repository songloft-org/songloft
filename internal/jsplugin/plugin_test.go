package jsplugin

import (
	"encoding/json"
	"testing"
)

func TestParseManifest_Full(t *testing.T) {
	data := []byte(`{
		"$schema": "https://example.com/plugin.schema.json",
		"name": "My Test Plugin",
		"version": "1.2.3",
		"description": "A test plugin",
		"author": "Test Author",
		"homepage": "https://example.com",
		"license": "MIT",
		"entryPath": "my-test-plugin",
		"main": "main.js",
		"minHostVersion": "1.0.0",
		"permissions": ["network", "storage"],
		"updateUrl": "https://example.com/update.json",
		"download_url": "https://example.com/download.zip"
	}`)

	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if m.Name != "My Test Plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "My Test Plugin")
	}
	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", m.Version, "1.2.3")
	}
	if m.Description != "A test plugin" {
		t.Errorf("Description = %q, want %q", m.Description, "A test plugin")
	}
	if m.Author != "Test Author" {
		t.Errorf("Author = %q, want %q", m.Author, "Test Author")
	}
	if m.Homepage != "https://example.com" {
		t.Errorf("Homepage = %q, want %q", m.Homepage, "https://example.com")
	}
	if m.License != "MIT" {
		t.Errorf("License = %q, want %q", m.License, "MIT")
	}
	if m.EntryPath != "my-test-plugin" {
		t.Errorf("EntryPath = %q, want %q", m.EntryPath, "my-test-plugin")
	}
	if m.Main != "main.js" {
		t.Errorf("Main = %q, want %q", m.Main, "main.js")
	}
	if m.MinHostVersion != "1.0.0" {
		t.Errorf("MinHostVersion = %q, want %q", m.MinHostVersion, "1.0.0")
	}
	if len(m.Permissions) != 2 || m.Permissions[0] != "network" || m.Permissions[1] != "storage" {
		t.Errorf("Permissions = %v, want [network storage]", m.Permissions)
	}
	if m.UpdateURL != "https://example.com/update.json" {
		t.Errorf("UpdateURL = %q, want %q", m.UpdateURL, "https://example.com/update.json")
	}
	if m.DownloadURL != "https://example.com/download.zip" {
		t.Errorf("DownloadURL = %q, want %q", m.DownloadURL, "https://example.com/download.zip")
	}
	if m.Schema != "https://example.com/plugin.schema.json" {
		t.Errorf("Schema = %q, want %q", m.Schema, "https://example.com/plugin.schema.json")
	}
}

func TestParseManifest_MinimalOptionalFields(t *testing.T) {
	data := []byte(`{
		"name": "Minimal Plugin",
		"version": "0.1.0",
		"description": "",
		"author": "",
		"entryPath": "minimal",
		"main": "index.js",
		"permissions": []
	}`)

	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if m.Name != "Minimal Plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "Minimal Plugin")
	}
	if m.Homepage != "" {
		t.Errorf("Homepage should be empty, got %q", m.Homepage)
	}
	if m.License != "" {
		t.Errorf("License should be empty, got %q", m.License)
	}
	if m.MinHostVersion != "" {
		t.Errorf("MinHostVersion should be empty, got %q", m.MinHostVersion)
	}
	if m.UpdateURL != "" {
		t.Errorf("UpdateURL should be empty, got %q", m.UpdateURL)
	}
	if m.DownloadURL != "" {
		t.Errorf("DownloadURL should be empty, got %q", m.DownloadURL)
	}
	if len(m.Permissions) != 0 {
		t.Errorf("Permissions should be empty, got %v", m.Permissions)
	}
}

func TestParseManifest_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestValidateManifest_Valid(t *testing.T) {
	m := &PluginManifest{
		Name:        "Valid Plugin",
		Version:     "1.0.0",
		EntryPath:   "valid-plugin",
		Main:        "main.js",
		Permissions: []string{},
		EntryHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ZipHash:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	if err := ValidateManifest(m); err != nil {
		t.Errorf("ValidateManifest should pass, got: %v", err)
	}
}

func TestValidateManifest_NameTooShort(t *testing.T) {
	m := &PluginManifest{
		Name:        "A",
		Version:     "1.0.0",
		EntryPath:   "test",
		Main:        "main.js",
		Permissions: []string{},
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for name too short")
	}
}

func TestValidateManifest_NameTooLong(t *testing.T) {
	longName := ""
	for i := 0; i < 51; i++ {
		longName += "a"
	}
	m := &PluginManifest{
		Name:        longName,
		Version:     "1.0.0",
		EntryPath:   "test",
		Main:        "main.js",
		Permissions: []string{},
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for name too long")
	}
}

func TestValidateManifest_MissingVersion(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test Plugin",
		Version:     "",
		EntryPath:   "test",
		Main:        "main.js",
		Permissions: []string{},
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for missing version")
	}
}

func TestValidateManifest_InvalidVersion(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test Plugin",
		Version:     "abc",
		EntryPath:   "test",
		Main:        "main.js",
		Permissions: []string{},
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for invalid version format")
	}
}

func TestValidateManifest_MissingEntryPath(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test Plugin",
		Version:     "1.0.0",
		EntryPath:   "",
		Main:        "main.js",
		Permissions: []string{},
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for missing entryPath")
	}
}

func TestValidateManifest_InvalidEntryPath(t *testing.T) {
	cases := []string{
		"MyPlugin",  // 大写
		"1plugin",   // 数字开头
		"my plugin", // 空格
		"my_plugin", // 下划线
		"my.plugin", // 点号
		"-plugin",   // 连字符开头
	}
	for _, ep := range cases {
		m := &PluginManifest{
			Name:        "Test Plugin",
			Version:     "1.0.0",
			EntryPath:   ep,
			Main:        "main.js",
			Permissions: []string{},
		}
		if err := ValidateManifest(m); err == nil {
			t.Errorf("expected error for entryPath %q", ep)
		}
	}
}

func TestValidateManifest_ValidEntryPath(t *testing.T) {
	cases := []string{
		"plugin",
		"my-plugin",
		"a1",
		"test-plugin-123",
	}
	for _, ep := range cases {
		m := &PluginManifest{
			Name:        "Test Plugin",
			Version:     "1.0.0",
			EntryPath:   ep,
			Main:        "main.js",
			Permissions: []string{},
			EntryHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ZipHash:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		if err := ValidateManifest(m); err != nil {
			t.Errorf("entryPath %q should be valid, got: %v", ep, err)
		}
	}
}

func TestValidateManifest_MissingMain(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test Plugin",
		Version:     "1.0.0",
		EntryPath:   "test",
		Main:        "",
		Permissions: []string{},
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for missing main")
	}
}

func TestValidateManifest_MainNotJS(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test Plugin",
		Version:     "1.0.0",
		EntryPath:   "test",
		Main:        "main.ts",
		Permissions: []string{},
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for main not ending with .js or .jsc")
	}
}

func TestValidateManifest_MainJSC(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test Plugin",
		Version:     "1.0.0",
		EntryPath:   "test",
		Main:        "main.jsc",
		Permissions: []string{},
		EntryHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ZipHash:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	if err := ValidateManifest(m); err != nil {
		t.Errorf("ValidateManifest should accept .jsc, got: %v", err)
	}
}

func TestValidateManifest_NilPermissions(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test Plugin",
		Version:     "1.0.0",
		EntryPath:   "test",
		Main:        "main.js",
		Permissions: nil,
	}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for nil permissions")
	}
}

func TestEntryPathFromZipName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"myplugin.jsplugin.zip", "myplugin"},
		{"test-plugin.jsplugin.zip", "test-plugin"},
		{"simple.zip", "simple"},
		{"plugin", "plugin"},
		{"myplugin.jsplugin", "myplugin"},
	}

	for _, tc := range cases {
		got := EntryPathFromZipName(tc.input)
		if got != tc.want {
			t.Errorf("EntryPathFromZipName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseManifest_PermissionsRoundTrip(t *testing.T) {
	m := &PluginManifest{
		Name:        "Test",
		Version:     "1.0.0",
		EntryPath:   "test",
		Main:        "main.js",
		Permissions: []string{"network", "fs"},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	m2, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if len(m2.Permissions) != 2 || m2.Permissions[0] != "network" || m2.Permissions[1] != "fs" {
		t.Errorf("Permissions roundtrip failed: got %v", m2.Permissions)
	}
}
