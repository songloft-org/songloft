-- +goose Up
ALTER TABLE songs ADD COLUMN radio_mode TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE songs DROP COLUMN radio_mode;
