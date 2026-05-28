package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// migrateLegacyDB performs the one-shot v1.x (mimusic.db) -> v2.0 (songloft.db) rename.
//
// Behavior:
//   - Compute legacy path as filepath.Join(dir(dbPath), "mimusic.db").
//   - If dbPath already equals the legacy path, do nothing.
//   - If dbPath exists, do nothing (user is already on the new layout).
//   - If legacy path exists and dbPath does not, rename it in place.
//   - Any os.Stat error other than NotExist is propagated; the rename
//     error is propagated as-is.
//
// This is the only compatibility point retained by the Songloft v2.0
// rebrand (see MIGRATION.md).
func migrateLegacyDB(dbPath string) error {
	legacyDBPath := filepath.Join(filepath.Dir(dbPath), "mimusic.db")
	if dbPath == legacyDBPath {
		return nil
	}

	if _, err := os.Stat(dbPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat target %q: %w", dbPath, err)
	}

	if _, err := os.Stat(legacyDBPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat legacy %q: %w", legacyDBPath, err)
	}

	if err := os.Rename(legacyDBPath, dbPath); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", legacyDBPath, dbPath, err)
	}
	slog.Info("migrated legacy mimusic.db to songloft.db", "from", legacyDBPath, "to", dbPath)
	return nil
}
