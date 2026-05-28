-- name: ListJSPlugins :many
SELECT id, name, version, description, author, homepage, license,
    entry_path, main, min_host_version, permissions, update_url, download_url,
    status, zip_hash, entry_hash, file_mod_time, file_path, created_at, updated_at
FROM js_plugins ORDER BY id;

-- name: GetJSPluginByID :one
SELECT id, name, version, description, author, homepage, license,
    entry_path, main, min_host_version, permissions, update_url, download_url,
    status, zip_hash, entry_hash, file_mod_time, file_path, created_at, updated_at
FROM js_plugins WHERE id = ?;

-- name: GetJSPluginByEntryPath :one
SELECT id, name, version, description, author, homepage, license,
    entry_path, main, min_host_version, permissions, update_url, download_url,
    status, zip_hash, entry_hash, file_mod_time, file_path, created_at, updated_at
FROM js_plugins WHERE entry_path = ?;

-- name: CreateJSPlugin :execlastid
INSERT INTO js_plugins (name, version, description, author, homepage, license,
    entry_path, main, min_host_version, permissions, update_url, download_url,
    status, zip_hash, entry_hash, file_mod_time, file_path)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateJSPlugin :exec
UPDATE js_plugins SET name = ?, version = ?, description = ?, author = ?,
    homepage = ?, license = ?, entry_path = ?, main = ?, min_host_version = ?,
    permissions = ?, update_url = ?, download_url = ?, status = ?,
    zip_hash = ?, entry_hash = ?, file_mod_time = ?, file_path = ?
WHERE id = ?;

-- name: DeleteJSPlugin :exec
DELETE FROM js_plugins WHERE id = ?;

-- name: UpdateJSPluginStatus :exec
UPDATE js_plugins SET status = ? WHERE id = ?;

-- name: UpdateJSPluginHashes :exec
UPDATE js_plugins SET zip_hash = ?, entry_hash = ?, file_mod_time = ? WHERE id = ?;
