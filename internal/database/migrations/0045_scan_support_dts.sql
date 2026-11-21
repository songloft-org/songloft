-- +goose Up
-- 默认扫描 .dts，保留已有配置及已添加的大小写变体。
UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'dts')
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      NOT EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) = 'dts'
      )
    ELSE 0 END
  ELSE 0 END;

-- +goose Down
UPDATE configs
SET value = json_set(value, '$.supported_formats',
  (SELECT json_group_array(e.value)
     FROM json_each(configs.value, '$.supported_formats') e
    WHERE e.type <> 'text' OR lower(e.value) <> 'dts'))
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) = 'dts'
      )
    ELSE 0 END
  ELSE 0 END;
