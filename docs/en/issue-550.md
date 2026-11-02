# Built-in favorites ordering (songloft-org/songloft#550)

## Problem and fix

[Issue #550](https://github.com/songloft-org/songloft/issues/550) reports failed manual ordering in Favorites and Radio favorites on Docker 2.13.6 / Windows. The screenshot matches the Flutter client.

The backend applied built-in playlist identity protection to member positions in `ReorderSongs`, `MoveSong` and `SortSongs`, returning 403 from all three endpoints. Remove those identity guards while retaining playlist existence, full-list length, song membership and transactional update checks. Existing name, type, label, visibility and deletion protections remain in their original paths. Swagger now documents ordering support for built-ins.

Flutter already loads the complete playlist, submits `song_ids` and restores custom ascending order after success; its required fix is in the backend. Lynx also needs its built-in manual sorting entry enabled and its multi-row downward drag corrected: the first displaced row is not necessarily the dragged item. Manual editing uses custom ascending order.

## Verification object and results

- Date: 2026-11-02; Linux amd64, Go 1.26.0, Flutter 3.44.8 / Dart 3.12.2, Node 22.17.1, Docker Chrome 121.
- Backend baseline: `71635ae30ec76a6a0a8bf4efa94304fe2276a057`; Flutter: `ae85c9570943c5f06ac77397dd384ef5907e2811`; Lynx: `a90cded03be18426de61c6df436d08a29e7a3077`. Final objects are these baselines plus the uncommitted changes; fingerprints: `/tmp/songloft-550-verification.json`.
- New real SQLite / HTTP regressions against the old implementation failed with 403 in all 12 scenarios: both built-in IDs × full manual reorder, single-song move, name in both directions, number prefix and shuffle. They pass after the fix, checking persisted positions, membership, built-in identity and deletion protection.
- Passed: `CGO_ENABLED=0 go test -p 2 -tags dev,lite ./...`, matching `go vet ./...`, and `go test -race -tags dev,lite ./internal/handlers -run '^TestBuiltInPlaylistSongOrdering$'`. Also ran `gofmt -w .`, `make swagger` and diff checks.
- The first full Go run failed the existing `TestHLSTranscodeRealFFmpeg` with `playlist did not advance`. Its isolated rerun and the full suite with lower package concurrency passed. The initial failure log is retained.
- Logs: `/tmp/songloft-550-{backend-before,backend-focused,backend-tests,backend-tests-final,backend-vet,backend-race,hls-retry,swagger}.log`. Browser drivers, results and screenshots: `/tmp/songloft-550-ui/`. Client records: [Flutter](../../clients/player/docs/en/issue-550.md) and [Lynx](../../clients/player-lynx/docs/en/project/issue-550.md).

Not run: native Windows client, Android/iOS/HarmonyOS devices and remote CI. Browser checks do not establish native device behavior. No commit, push, release or Issue reply. The backend must be upgraded, and the Lynx entry fix requires a client update.
