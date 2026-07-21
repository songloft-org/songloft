-- +goose Up
-- 将 mka 加入已有部署的 scan_config.supported_formats，使本地 mka 文件能被扫描入库（songloft-org/songloft#297）。
-- mka（Matroska 音频容器）常见于原伴唱双音轨等音频资源；仅播放其音频轨，客户端不原生支持时由服务端转码。
-- 幂等：仅当数组中尚无 "mka" 时追加。
UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'mka')
WHERE key = 'scan_config'
  AND value LIKE '%"supported_formats"%'
  AND value NOT LIKE '%"mka"%';

-- +goose Down
UPDATE configs
SET value = json_set(value, '$.supported_formats',
  (SELECT json_group_array(e.value)
     FROM json_each(json_extract(configs.value, '$.supported_formats')) e
    WHERE e.value <> 'mka'))
WHERE key = 'scan_config' AND value LIKE '%"mka"%';
