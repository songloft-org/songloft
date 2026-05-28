package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyDB(t *testing.T) {
	const legacyContent = "legacy-mimusic-db-bytes"

	t.Run("renames legacy when target absent", func(t *testing.T) {
		dir := t.TempDir()
		legacy := filepath.Join(dir, "mimusic.db")
		target := filepath.Join(dir, "songloft.db")

		if err := os.WriteFile(legacy, []byte(legacyContent), 0o644); err != nil {
			t.Fatalf("seed legacy: %v", err)
		}

		if err := migrateLegacyDB(target); err != nil {
			t.Fatalf("migrate: %v", err)
		}

		if _, err := os.Stat(legacy); !os.IsNotExist(err) {
			t.Fatalf("legacy should be gone after rename, stat err = %v", err)
		}
		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read target: %v", err)
		}
		if string(got) != legacyContent {
			t.Fatalf("target content = %q, want %q", got, legacyContent)
		}
	})

	t.Run("noop when target already exists", func(t *testing.T) {
		dir := t.TempDir()
		legacy := filepath.Join(dir, "mimusic.db")
		target := filepath.Join(dir, "songloft.db")

		if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
			t.Fatalf("seed legacy: %v", err)
		}
		if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
			t.Fatalf("seed target: %v", err)
		}

		if err := migrateLegacyDB(target); err != nil {
			t.Fatalf("migrate: %v", err)
		}

		if got, _ := os.ReadFile(target); string(got) != "existing" {
			t.Fatalf("target was clobbered: %q", got)
		}
		if got, _ := os.ReadFile(legacy); string(got) != "legacy" {
			t.Fatalf("legacy was touched: %q", got)
		}
	})

	t.Run("noop when legacy absent", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "songloft.db")

		if err := migrateLegacyDB(target); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("target should not have been created, err = %v", err)
		}
	})

	t.Run("noop when target path is the legacy path itself", func(t *testing.T) {
		dir := t.TempDir()
		legacy := filepath.Join(dir, "mimusic.db")
		if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
			t.Fatalf("seed legacy: %v", err)
		}

		// User explicitly set --db data/mimusic.db; we must not touch it.
		if err := migrateLegacyDB(legacy); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if got, _ := os.ReadFile(legacy); string(got) != "legacy" {
			t.Fatalf("legacy at same path got mutated: %q", got)
		}
	})
}
