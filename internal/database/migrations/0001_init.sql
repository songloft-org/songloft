-- +goose Up
-- +goose StatementBegin
CREATE TABLE songs (
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
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE playlists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('normal', 'radio')),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cover_path TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    labels TEXT NOT NULL DEFAULT '[]',
    position INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE playlist_songs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlist_id INTEGER NOT NULL,
    song_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
    FOREIGN KEY (song_id) REFERENCES songs(id) ON DELETE CASCADE,
    UNIQUE(playlist_id, song_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE auth_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_id TEXT NOT NULL UNIQUE,
    token_type TEXT NOT NULL CHECK(token_type IN ('access', 'refresh')),
    client_info TEXT NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    revoked_at DATETIME,
    revoked_by TEXT NOT NULL DEFAULT '',
    revoked_reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE js_plugins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT '',
    homepage TEXT NOT NULL DEFAULT '',
    license TEXT NOT NULL DEFAULT '',
    entry_path TEXT NOT NULL UNIQUE,
    main TEXT NOT NULL DEFAULT 'main.js',
    min_host_version TEXT NOT NULL DEFAULT '',
    permissions TEXT NOT NULL DEFAULT '[]',
    update_url TEXT NOT NULL DEFAULT '',
    download_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('active', 'inactive', 'error')) DEFAULT 'inactive',
    zip_hash TEXT NOT NULL DEFAULT '',
    entry_hash TEXT NOT NULL DEFAULT '',
    file_mod_time TEXT NOT NULL DEFAULT '',
    file_path TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

CREATE INDEX idx_songs_type ON songs(type);
CREATE INDEX idx_songs_title ON songs(title);
CREATE INDEX idx_songs_artist ON songs(artist);
CREATE INDEX idx_songs_added_at ON songs(added_at DESC);
CREATE INDEX idx_playlists_type ON playlists(type);
CREATE INDEX idx_playlists_labels ON playlists(labels);
-- 歌单名全局唯一（不区分类型），DB 层兜底防止任何 INSERT 路径绕过应用层查重。
CREATE UNIQUE INDEX idx_playlists_name_unique ON playlists(name);
CREATE INDEX idx_playlist_songs_playlist ON playlist_songs(playlist_id);
CREATE INDEX idx_playlist_songs_position ON playlist_songs(playlist_id, position);
CREATE INDEX idx_configs_key ON configs(key);
CREATE INDEX idx_auth_tokens_token_id ON auth_tokens(token_id);
CREATE INDEX idx_auth_tokens_token_type ON auth_tokens(token_type);
CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens(expires_at);
CREATE INDEX idx_auth_tokens_revoked_at ON auth_tokens(revoked_at);
CREATE INDEX idx_songs_plugin_entry_path ON songs(plugin_entry_path) WHERE plugin_entry_path != '';
-- 同插件下 dedup_key 全局唯一，作为 remote song 去重 key。
CREATE UNIQUE INDEX idx_songs_dedup_key_unique ON songs(plugin_entry_path, dedup_key) WHERE dedup_key != '';
CREATE INDEX idx_js_plugins_status ON js_plugins(status);
CREATE INDEX idx_js_plugins_entry_path ON js_plugins(entry_path);

-- +goose StatementBegin
CREATE TRIGGER update_songs_updated_at
AFTER UPDATE ON songs
FOR EACH ROW
BEGIN
    UPDATE songs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER update_playlists_updated_at
AFTER UPDATE ON playlists
FOR EACH ROW
BEGIN
    UPDATE playlists SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER update_configs_updated_at
AFTER UPDATE ON configs
FOR EACH ROW
BEGIN
    UPDATE configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER update_js_plugins_updated_at
AFTER UPDATE ON js_plugins
FOR EACH ROW
BEGIN
    UPDATE js_plugins SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- 初始化内置歌单
INSERT INTO playlists (id, type, name, description, labels)
VALUES
    (1, 'normal', '收藏', '我喜欢的歌曲', '["built_in"]'),
    (2, 'radio', '电台收藏', '我喜欢的电台', '["built_in"]');

-- 初始化默认配置
INSERT INTO configs (key, value)
VALUES
    ('music_path', '{"path": "music", "exclude_dirs": ["@eaDir", "tmp"]}'),
    ('cover_storage_path', '{"path": "data/covers"}'),
    ('scan_config', '{"auto_scan": true, "scan_interval": 3600, "supported_formats": ["mp3", "flac", "wav", "ape", "ogg", "m4a", "mp4", "mov", "wma"]}'),
    ('ffprobe_path', '{"path": "ffprobe"}'),
    ('jwt_secret', lower(hex(randomblob(32)))),
    ('source_validation', '{"enabled": true, "min_duration": 30, "duration_ratio": 0.85, "max_duration_ratio": 1.5, "min_bitrate": 8}'),
    ('source_fallback', '{"enabled": true, "max_attempts": 4, "per_plugin_timeout_ms": 5000, "global_timeout_ms": 8000, "min_score": 0.6, "max_results": 5, "exclude_red_plugins": true, "cache_ttl_seconds": 300}'),
    ('source_metrics', '{"window_size": 200, "green_threshold": 0.8, "red_threshold": 0.4, "min_samples": 10}');

-- +goose Down
DROP TRIGGER IF EXISTS update_js_plugins_updated_at;
DROP TRIGGER IF EXISTS update_configs_updated_at;
DROP TRIGGER IF EXISTS update_playlists_updated_at;
DROP TRIGGER IF EXISTS update_songs_updated_at;
DROP TABLE IF EXISTS js_plugins;
DROP TABLE IF EXISTS auth_tokens;
DROP TABLE IF EXISTS configs;
DROP TABLE IF EXISTS playlist_songs;
DROP TABLE IF EXISTS playlists;
DROP TABLE IF EXISTS songs;
