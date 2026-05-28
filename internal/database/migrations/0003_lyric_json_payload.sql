-- +goose Up
-- +goose StatementBegin
-- 把 songs.lyric 字段统一升级为 LyricPayload JSON 形态。
-- 同时新增 lyric_remote_url 列,把原本混在 lyric 字段里的 URL 字符串分流出去,
-- 让 lyric 列语义统一:始终是 JSON({"lyric":"...","tlyric":"...",...})或空。
ALTER TABLE songs ADD COLUMN lyric_remote_url TEXT NOT NULL DEFAULT '';

-- 1) url 来源:把 lyric 列里的 URL 搬到 lyric_remote_url,原列清空。
--    保留 lyric_source = 'url',运行时由 LyricFetcher 按需拉取。
UPDATE songs
SET lyric_remote_url = lyric,
    lyric = ''
WHERE lyric_source = 'url' AND lyric != '';

-- 2) 其它来源:把裸 LRC 包装成 LyricPayload JSON。json_valid 守卫保证幂等
--    (若已经是 JSON 不会二次包装)。
UPDATE songs
SET lyric = json_object('lyric', lyric)
WHERE lyric != ''
  AND lyric_source != 'url'
  AND json_valid(lyric) = 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 反向迁移:JSON 解包回裸 LRC、URL 搬回 lyric 列。
UPDATE songs
SET lyric = COALESCE(json_extract(lyric, '$.lyric'), '')
WHERE lyric != ''
  AND lyric_source != 'url'
  AND json_valid(lyric) = 1;

UPDATE songs
SET lyric = lyric_remote_url
WHERE lyric_source = 'url' AND lyric_remote_url != '';

ALTER TABLE songs DROP COLUMN lyric_remote_url;
-- +goose StatementEnd
