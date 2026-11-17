# #573 investigation and verification

Target: [songloft-org/songloft#573](https://github.com/songloft-org/songloft/issues/573). This change confirms and fixes scan-related playlist ordering defects. Missing songs after moving files have not been reproduced, so this does not establish that the entire issue is resolved.

## Verified code

- Date: 2026-11-17; Linux amd64, Go 1.26.0, system ffmpeg/ffprobe.
- Parent baseline: `8177b2f208d8436f24a97e08a4ae237f372f88c5` plus this uncommitted worktree.
- Content identity of the four source/test files: `cde1222c76aa23b4e539a16c15a7ff0bae49fd532a6494d530106e2cf9ce180b`. Sort paths, concatenate each `path + NUL + contents + NUL`, and compute SHA-256. File list: `/tmp/songloft-573-20261117/source-identity.json`.
- `clients/player-lynx` already had uncommitted changes at the start. This task did not modify submodules.
- Unrelated proxy, JS runtime, and corresponding documentation/test changes appeared during execution; this task did not modify them. The full build finished at 15:42; `make check` started at 15:42 and finished at 15:43, and does not establish acceptance of concurrent edits. This task's four-file content identity and final targeted database tests are recorded separately.

## Findings and changes

1. The default playlist order is `position ASC, updated_at DESC`. Automatic creation omitted position, using the default 0, which could put new playlists before existing ordered ones. It now uses the maximum position plus one within the same transaction, matching regular creation.
2. Rebuilding automatic playlists after a scan unconditionally issued UPDATE, triggering `update_playlists_updated_at`. Even unchanged metadata altered the ordering of older playlists with equal positions. SQL now updates only when the name, description, or cover changes. Membership still refreshes from scan results; actual metadata changes still update the timestamp. Existing IDs, custom positions, pins, and sorting preferences remain intact.
3. Investigation covered the body, all comments (none at inspection), events, #267, three screenshots, and the attached logs. Zero imported, skipped, and failed files points toward no audio being discovered for processing. The logs lack requested directories, exclusion settings, and file listings, so the cause remains unconfirmed. Visibility over a Windows share does not establish visibility inside the container. The user was asked for the music root, excluded directory names/paths, and the complete selected container path.

## Verification

| Command or scenario | Result and evidence |
| --- | --- |
| `make sqlc`, root `gofmt -w .` | Passed; generated files synchronized; `sqlc.log` |
| `make check` | Formatting, vet, and full Go suite passed; `check.log`. The existing explanatory `TestSongServiceScanAndImportNote` remains skipped; actual scanning was separately exercised over HTTP |
| `go test ./internal/database -run TestAutoCreate -count=1` | Final tests passed; `order-after.log` and `order-final.log`. Covers equal-position legacy ordering, member additions, filling a missing cover, appending new playlists, and retaining custom positions/pins/view sorting |
| Go overlay restoring baseline generated queries, running the final two ordering regressions | Both failed for the expected behaviors: changed legacy timestamps/order and new playlists preceding a manual playlist; `order-negative-final.log`. Current files were not reverted |
| `go build -tags dev -o /tmp/songloft-573-20261117/songloft .` | Full embedded build passed |
| HTTP against a temporary real backend, SQLite, and synthetic MP3 files | Initial full scan imported 2 songs. Moving one from a wrong directory into the Chinese directory and scanning that directory imported 1 and skipped 1. Path filtering returned 2 songs; the automatic playlist contained both; manual addition to another playlist added 2. Repeated scanning retained list order and timestamps. A later full scan removed 1 stale original-path record. See `http-results.json`, `http_probe.py`, and `http-server.log` |
| `git diff --check` | Passed |

Logs are under `/tmp/songloft-573-20261117/`. The temporary library is under `/home/hanxi/.cache/`, avoiding the default excluded name `tmp`. No private audio or existing database was modified. Retaining the out-of-scope original-path record during a scoped scan follows the existing contract; the full scan subsequently removes it.

Not run: the reporter's Docker mounts, actual configuration, and original audio; Windows Flutter UI interactions; frontend builds and other devices; remote CI/release for this fix had not run at local verification time. The API is unchanged, so Swagger regeneration is unnecessary. Commits, pushes, CI, and version availability follow subsequent Issue replies. Commit: `fix(scan): 保留自动歌单扫描顺序并追加新歌单 (Ref #573)`.
