-- +goose Up
ALTER TABLE songs ADD COLUMN metadata_override_mask INTEGER NOT NULL DEFAULT 0;
ALTER TABLE songs ADD COLUMN metadata_edited_at INTEGER NOT NULL DEFAULT 0;

UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'wv')
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      NOT EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) = 'wv'
      )
    ELSE 0 END
  ELSE 0 END;

UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'aac')
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      NOT EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) = 'aac'
      )
    ELSE 0 END
  ELSE 0 END;


UPDATE configs
SET value = json_set(value, '$.supported_formats[#]', 'dff')
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      NOT EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) = 'dff'
      )
    ELSE 0 END
  ELSE 0 END;

-- +goose Down
UPDATE configs
SET value = json_set(value, '$.supported_formats',
  (SELECT json_group_array(e.value)
     FROM json_each(configs.value, '$.supported_formats') e
    WHERE e.type <> 'text' OR lower(e.value) NOT IN ('wv', 'aac', 'dff')))
WHERE key = 'scan_config'
  AND CASE WHEN json_valid(value) THEN
    CASE WHEN json_type(value, '$.supported_formats') = 'array' THEN
      EXISTS (
        SELECT 1 FROM json_each(configs.value, '$.supported_formats') e
        WHERE e.type = 'text' AND lower(e.value) IN ('wv', 'aac', 'dff')
      )
    ELSE 0 END
  ELSE 0 END;

ALTER TABLE songs DROP COLUMN metadata_edited_at;
ALTER TABLE songs DROP COLUMN metadata_override_mask;
