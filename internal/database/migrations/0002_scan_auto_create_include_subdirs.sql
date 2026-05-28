-- +goose Up
INSERT OR IGNORE INTO configs (key, value) VALUES
    ('scan_auto_create_include_subdirs', 'false');

-- +goose Down
DELETE FROM configs WHERE key = 'scan_auto_create_include_subdirs';
