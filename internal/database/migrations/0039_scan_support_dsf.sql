-- +goose Up
-- 将 dsf 加入默认扫描格式，覆盖新安装和已有部署（songloft-org/songloft#512）。
-- 保留自定义配置；大小写无关检查数组元素，避免重复追加。
UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'dsf')
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      NOT EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) = 'dsf'
      )
    ELSE 0 END
  ELSE 0 END;

-- +goose Down
-- 与其他扫描格式迁移一致，回滚移除该格式（含用户事先添加的值）。
UPDATE configs
SET value = json_set(value, '$.supported_formats',
  (SELECT json_group_array(e.value)
     FROM json_each(configs.value, '$.supported_formats') e
    WHERE e.type <> 'text' OR lower(e.value) <> 'dsf'))
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) = 'dsf'
      )
    ELSE 0 END
  ELSE 0 END;
