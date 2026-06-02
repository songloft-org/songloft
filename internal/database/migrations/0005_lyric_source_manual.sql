-- +goose Up
-- 放宽 songs.lyric_source 的 CHECK 约束，新增 'manual' 取值。
-- 用于标记「用户手动调整过的歌词」，scanner 重扫时识别该标记后跳过覆盖，
-- 避免不支持回写音频文件的扩展名（如 .wav）丢失用户调整。
--
-- SQLite 不支持 ALTER TABLE 修改 CHECK 约束，需要重建表。
-- 注意：每条 SQL 必须独立成 StatementBegin/End 块 —— 否则 sqlite3 driver
-- 在单次 Exec 里只会执行第一条 prepared statement，后续 SQL 静默吞掉。

-- +goose StatementBegin
CREATE TABLE songs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('local', 'remote', 'radio')),
    title TEXT NOT NULL,
    artist TEXT NOT NULL DEFAULT '',
    album TEXT NOT NULL DEFAULT '',
    duration REAL NOT NULL DEFAULT 0,
    file_path TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    cover_path TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    lyric TEXT NOT NULL DEFAULT '',
    lyric_source TEXT NOT NULL DEFAULT '' CHECK(lyric_source IN ('file', 'embedded', 'scraped', 'url', 'cached', 'manual', '')),
    file_size INTEGER NOT NULL DEFAULT 0,
    format TEXT NOT NULL DEFAULT '',
    bit_rate INTEGER NOT NULL DEFAULT 0,
    sample_rate INTEGER NOT NULL DEFAULT 0,
    is_live INTEGER NOT NULL DEFAULT 0,
    plugin_entry_path TEXT NOT NULL DEFAULT '',
    source_data TEXT NOT NULL DEFAULT '',
    dedup_key TEXT NOT NULL DEFAULT '',
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lyric_remote_url TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO songs_new (
    id, type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, file_size, format,
    bit_rate, sample_rate, is_live, plugin_entry_path, source_data,
    dedup_key, added_at, updated_at, lyric_remote_url
)
SELECT
    id, type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, file_size, format,
    bit_rate, sample_rate, is_live, plugin_entry_path, source_data,
    dedup_key, added_at, updated_at, lyric_remote_url
FROM songs;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE songs;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE songs_new RENAME TO songs;
-- +goose StatementEnd

-- 重建索引（DROP TABLE 会顺带 drop 同表索引）

-- +goose StatementBegin
CREATE INDEX idx_songs_type ON songs(type);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_title ON songs(title);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_artist ON songs(artist);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_added_at ON songs(added_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_plugin_entry_path ON songs(plugin_entry_path) WHERE plugin_entry_path != '';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE UNIQUE INDEX idx_songs_dedup_key_unique ON songs(plugin_entry_path, dedup_key) WHERE dedup_key != '';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_file_path ON songs(file_path);
-- +goose StatementEnd

-- 重建触发器
-- +goose StatementBegin
CREATE TRIGGER update_songs_updated_at
AFTER UPDATE ON songs
FOR EACH ROW
BEGIN
    UPDATE songs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd


-- +goose Down
-- 反向迁移：先把 manual 改回 cached（保持文本不丢），再恢复旧的 CHECK 约束。

-- +goose StatementBegin
UPDATE songs SET lyric_source = 'cached' WHERE lyric_source = 'manual';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE songs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('local', 'remote', 'radio')),
    title TEXT NOT NULL,
    artist TEXT NOT NULL DEFAULT '',
    album TEXT NOT NULL DEFAULT '',
    duration REAL NOT NULL DEFAULT 0,
    file_path TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    cover_path TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    lyric TEXT NOT NULL DEFAULT '',
    lyric_source TEXT NOT NULL DEFAULT '' CHECK(lyric_source IN ('file', 'embedded', 'scraped', 'url', 'cached', '')),
    file_size INTEGER NOT NULL DEFAULT 0,
    format TEXT NOT NULL DEFAULT '',
    bit_rate INTEGER NOT NULL DEFAULT 0,
    sample_rate INTEGER NOT NULL DEFAULT 0,
    is_live INTEGER NOT NULL DEFAULT 0,
    plugin_entry_path TEXT NOT NULL DEFAULT '',
    source_data TEXT NOT NULL DEFAULT '',
    dedup_key TEXT NOT NULL DEFAULT '',
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lyric_remote_url TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO songs_new (
    id, type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, file_size, format,
    bit_rate, sample_rate, is_live, plugin_entry_path, source_data,
    dedup_key, added_at, updated_at, lyric_remote_url
)
SELECT
    id, type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, file_size, format,
    bit_rate, sample_rate, is_live, plugin_entry_path, source_data,
    dedup_key, added_at, updated_at, lyric_remote_url
FROM songs;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE songs;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE songs_new RENAME TO songs;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_type ON songs(type);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_title ON songs(title);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_artist ON songs(artist);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_added_at ON songs(added_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_plugin_entry_path ON songs(plugin_entry_path) WHERE plugin_entry_path != '';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE UNIQUE INDEX idx_songs_dedup_key_unique ON songs(plugin_entry_path, dedup_key) WHERE dedup_key != '';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_songs_file_path ON songs(file_path);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER update_songs_updated_at
AFTER UPDATE ON songs
FOR EACH ROW
BEGIN
    UPDATE songs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd
