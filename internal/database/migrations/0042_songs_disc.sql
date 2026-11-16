-- +goose Up
ALTER TABLE songs ADD COLUMN disc TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE songs DROP COLUMN disc;
