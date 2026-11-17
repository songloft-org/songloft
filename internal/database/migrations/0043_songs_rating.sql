-- +goose Up
ALTER TABLE songs ADD COLUMN rating INTEGER NOT NULL DEFAULT 0;

-- 评级维度索引：星标筛选/排序按 rating 列直接过滤，0 表示未评级。
CREATE INDEX IF NOT EXISTS idx_songs_rating ON songs(rating);

-- +goose Down
DROP INDEX IF EXISTS idx_songs_rating;
ALTER TABLE songs DROP COLUMN rating;
