-- +goose Up
-- 将 mp4 加入已有部署的 scan_config.supported_formats，使本地 mp4 文件能被扫描到。
-- mp4 与 m4a/mov 同族（ISO-BMFF 容器），常见于电子书、网课等音视频混合内容，仅播放其音频轨。
-- 幂等：仅当数组中尚无 "mp4" 时追加。
UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'mp4')
WHERE key = 'scan_config'
  AND value LIKE '%"supported_formats"%'
  AND value NOT LIKE '%"mp4"%';

-- +goose Down
UPDATE configs
SET value = json_set(value, '$.supported_formats',
  (SELECT json_group_array(e.value)
     FROM json_each(json_extract(configs.value, '$.supported_formats')) e
    WHERE e.value <> 'mp4'))
WHERE key = 'scan_config' AND value LIKE '%"mp4"%';
