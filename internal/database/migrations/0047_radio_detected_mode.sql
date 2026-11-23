-- +goose Up
ALTER TABLE songs ADD COLUMN radio_detected_mode TEXT NOT NULL DEFAULT '' CHECK (radio_detected_mode IN ('', 'hls', 'flv'));

-- +goose StatementBegin
CREATE TRIGGER songs_reset_radio_detected_mode
AFTER UPDATE OF url, radio_mode ON songs
WHEN NEW.radio_detected_mode != '' AND (OLD.url IS NOT NEW.url OR OLD.radio_mode IS NOT NEW.radio_mode)
BEGIN
    UPDATE songs SET radio_detected_mode = '' WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER songs_reset_radio_detected_mode;
ALTER TABLE songs DROP COLUMN radio_detected_mode;
