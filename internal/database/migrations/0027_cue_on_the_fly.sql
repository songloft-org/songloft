-- +goose Up
ALTER TABLE songs ADD COLUMN cue_start_seconds REAL NOT NULL DEFAULT 0;
ALTER TABLE songs ADD COLUMN cue_end_seconds REAL NOT NULL DEFAULT 0;

-- +goose Down
