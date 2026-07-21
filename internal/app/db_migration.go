package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"songloft/internal/database"
	"songloft/pkg/cue"
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

// migrateCueSplitsToOnTheFly 一次性迁移：将旧版预分割的 CUE 记录迁移为按需提取模式。
// 检测条件：cue_source_path 非空但 cue_start_seconds 和 cue_end_seconds 均为 0。
// 迁移完成后删除旧的 cue_splits 目录释放磁盘空间。
func (a *App) migrateCueSplitsToOnTheFly(db database.DB) {
	sdb, ok := db.(*database.SQLiteDB)
	if !ok {
		return
	}
	rawDB := sdb.DB()

	// 查询需要迁移的 CUE 来源
	rows, err := rawDB.Query(`SELECT DISTINCT cue_source_path FROM songs
		WHERE cue_source_path != '' AND cue_start_seconds = 0 AND cue_end_seconds = 0`)
	if err != nil {
		slog.Warn("CUE 迁移查询失败", "error", err)
		return
	}
	var sources []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			sources = append(sources, s)
		}
	}
	rows.Close()

	if len(sources) == 0 {
		return
	}

	slog.Info("开始迁移 CUE 预分割记录为按需提取模式", "sources", len(sources))

	totalMigrated := 0
	var failedSources []string
	for _, sourcePath := range sources {
		n := a.migrateCueSource(rawDB, sourcePath)
		if n > 0 {
			totalMigrated += n
		} else {
			failedSources = append(failedSources, sourcePath)
		}
	}

	// 无法迁移的来源（如 FLAC 内嵌 CUESHEET），删除 DB 记录让下次扫描以新模式重新导入
	for _, sourcePath := range failedSources {
		n, err := rawDB.ExecContext(context.Background(),
			`DELETE FROM songs WHERE cue_source_path = ?`, sourcePath)
		if err == nil {
			if affected, _ := n.RowsAffected(); affected > 0 {
				slog.Info("CUE 迁移：删除无法迁移的来源记录，下次扫描将重新导入",
					"source", sourcePath, "deleted", affected)
			}
		}
	}

	// 清理旧的 cue_splits 目录
	cueSplitDir := filepath.Join(filepath.Dir(a.config.DBPath), "cue_splits")
	if fi, err := os.Stat(cueSplitDir); err == nil && fi.IsDir() {
		if err := os.RemoveAll(cueSplitDir); err != nil {
			slog.Warn("删除旧 cue_splits 目录失败", "dir", cueSplitDir, "error", err)
		} else {
			slog.Info("已删除旧 cue_splits 目录", "dir", cueSplitDir)
		}
	}

	slog.Info("CUE 迁移完成", "sources", len(sources), "tracks", totalMigrated)
}

// migrateCueSource 迁移单个 CUE 来源的所有 track 记录。
func (a *App) migrateCueSource(rawDB *sql.DB, sourcePath string) int {
	cueDir := filepath.Dir(sourcePath)

	// 尝试解析 CUE sheet（外部 .cue 文件或 FLAC 内嵌 CUESHEET）
	sheet, totalDurations := parseCueForMigration(sourcePath, cueDir)
	if sheet == nil {
		slog.Warn("CUE 迁移：无法解析来源，跳过", "source", sourcePath)
		return 0
	}

	tracks, err := cue.ResolveTracks(sheet, cueDir, totalDurations)
	if err != nil || len(tracks) == 0 {
		slog.Warn("CUE 迁移：track 解析失败", "source", sourcePath, "error", err)
		return 0
	}

	// 按 track_index 建索引
	trackMap := make(map[int]cue.ResolvedTrack, len(tracks))
	for _, t := range tracks {
		trackMap[t.TrackNumber] = t
	}

	// 查询该来源下的所有记录
	rows, err := rawDB.QueryContext(context.Background(),
		`SELECT id, cue_track_index FROM songs WHERE cue_source_path = ?`, sourcePath)
	if err != nil {
		slog.Warn("CUE 迁移：查询记录失败", "source", sourcePath, "error", err)
		return 0
	}
	defer rows.Close()

	type record struct {
		id         int64
		trackIndex int
	}
	var records []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.id, &r.trackIndex); err == nil {
			records = append(records, r)
		}
	}

	migrated := 0
	for _, r := range records {
		track, ok := trackMap[r.trackIndex]
		if !ok {
			continue
		}
		_, err := rawDB.ExecContext(context.Background(),
			`UPDATE songs SET file_path = ?, cue_start_seconds = ?, cue_end_seconds = ?,
				duration = CASE WHEN ? > 0 THEN ? ELSE duration END
			WHERE id = ?`,
			track.AudioFilePath,
			track.StartSeconds, track.EndSeconds,
			track.Duration(), track.Duration(),
			r.id)
		if err != nil {
			slog.Warn("CUE 迁移：更新记录失败", "id", r.id, "error", err)
			continue
		}
		migrated++
	}

	if migrated > 0 {
		slog.Info("CUE 来源迁移完成", "source", sourcePath, "tracks", migrated)
	}
	return migrated
}

// parseCueForMigration 解析 CUE 来源，支持外部 .cue 文件和 FLAC 内嵌 CUESHEET。
func parseCueForMigration(sourcePath, cueDir string) (*cue.CUESheet, map[string]float64) {
	// 外部 .cue 文件
	if ext := filepath.Ext(sourcePath); ext == ".cue" || ext == ".CUE" || ext == ".Cue" {
		sheet, err := cue.ParseFile(sourcePath)
		if err != nil {
			return nil, nil
		}
		return sheet, nil
	}

	// FLAC 内嵌 CUESHEET：sourcePath 就是 FLAC 文件路径
	// 迁移时简单跳过内嵌场景（下次扫描会以新模式重新处理）
	return nil, nil
}
