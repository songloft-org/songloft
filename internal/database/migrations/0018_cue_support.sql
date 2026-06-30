-- +goose Up
ALTER TABLE songs ADD COLUMN cue_source_path TEXT NOT NULL DEFAULT '';
ALTER TABLE songs ADD COLUMN cue_track_index INTEGER NOT NULL DEFAULT 0;
ALTER TABLE songs ADD COLUMN cue_audio_path TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_songs_cue_source ON songs(cue_source_path) WHERE cue_source_path != '';

-- +goose Down
DROP INDEX IF EXISTS idx_songs_cue_source;
