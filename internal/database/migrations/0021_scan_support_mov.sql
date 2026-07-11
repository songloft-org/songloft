-- +goose Up
-- 将 mov 加入已有部署的 scan_config.supported_formats，使本地 mov 文件能被扫描到。
-- mov 与 m4a/mp4 同族（QuickTime/ISO-BMFF 容器），常见于 bilibili 等下载源。
-- 幂等：仅当数组中尚无 "mov" 时追加。
UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'mov')
WHERE key = 'scan_config'
  AND value LIKE '%"supported_formats"%'
  AND value NOT LIKE '%"mov"%';

-- +goose Down
UPDATE configs
SET value = json_set(value, '$.supported_formats',
  (SELECT json_group_array(e.value)
     FROM json_each(json_extract(configs.value, '$.supported_formats')) e
    WHERE e.value <> 'mov'))
WHERE key = 'scan_config' AND value LIKE '%"mov"%';
