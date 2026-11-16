# Album track order verification

Target: [songloft-org/songloft#571](https://github.com/songloft-org/songloft/issues/571). 2026-11-16, Linux amd64 / Go 1.26.0 / Node 22.17.1, temporary database and synthetic local audio.

## Verified code

| Repository | Baseline |
| --- | --- |
| songloft-org/songloft | `7180df38176ca7b44c1a7d8cf2b3ec269d35ac7d` |
| hanxi/tag (pkg/tag) | `64e12e525cc5c9a5ee66826c98dbaed42534af8b` |
| songloft-org/songloft-player-lynx | `b406af75043d2c2d898b7f73b0f282248d293d33` |

Local checks used these baselines plus this batch's worktree. Pre-commit tracked diffs, index and untracked file SHA-256 values are recorded outside the worktree in `/tmp/songloft-571-snapshot.json`. Delivery commits and pushes both child repositories before the parent and gitlinks; final commits and same-SHA CI results are reported in the #571 delivery reply. This batch creates no stable release.

## Behavior and causes

- HTTP previously always defaulted to addition time, with no repository track sort; Lynx album pages explicitly requested addition time descending. Editing tags could not change that rule.
- Added `sort=track`, also the default for requests with `album` and no explicit sort, ascending unless specified otherwise. Explicit other sorts still apply.
- SQLite sorts before LIMIT/OFFSET, shared by full songs and IDs: numeric disc → numeric track → file path → ID. Missing/invalid discs belong to disc 1; unnumbered tracks follow numbered tracks within a disc. Existing virtual CUE tracks fall back to `cue_track_index`.
- Goose 0042 adds `disc`, passed through scanning, reimport, repositories, song responses and tag writing. Successful extraction synchronizes track/disc, including external tag removal. Upgrading does not automatically rescan; reimport existing albums once to populate disc numbers.
- Tag writing supports disc numbers in MP3, FLAC, M4A, OGG, APE and AIFF; older callers omitting disc retain the file's existing disc. No new WAV disc mapping was introduced.
- FLAC/OGG readers previously used integer conversion on combined `TRACKNUMBER=02/12` and `DISCNUMBER=1/2`, losing the numbers. Combined/separate values and totaltracks/totaldiscs aliases now work.
- Flutter category and queue-completion requests omit explicit sorting and use the new server album default; Flutter source is unchanged. Lynx album pages explicitly request `track/asc`.

## Checks

| Check | Result and evidence |
| --- | --- |
| `CGO_ENABLED=0 go test -tags dev,lite ./...` | Passed; `/tmp/songloft-571-tests-final.log` |
| `CGO_ENABLED=0 go vet -tags dev,lite ./...` | Passed; `/tmp/songloft-571-vet-final.log` |
| Tag library `go test ./...` | Passed, including six formats and preservation for older callers; `/tmp/songloft-571-tag.log` |
| Real SQLite + HTTP regression | Passed numeric 1/2/10, multiple discs, missing/invalid tracks, CUE, pagination, IDs, explicit sorts and reorder after tag edits |
| Scan/reimport regression | Real FLAC → extraction → transactional insert → file edit → extraction/update → tag rewrite; also external number removal clears database values; `/tmp/songloft-571-service-final.log` |
| Race | Three repetitions of target handler/service regressions; `/tmp/songloft-571-race-final.log` |
| Lynx `pnpm exec tsc -b` | Passed; `/tmp/songloft-571-lynx-types.log` |
| Lynx `pnpm test` | 363 files, 3821 tests passed; `/tmp/songloft-571-lynx-tests.log`; final 12 targeted category checks passed |
| Lynx `pnpm run build` | Both Lynx/Web outputs passed; `/tmp/songloft-571-lynx-build.log`; existing unsupported CSS property warnings retained |
| `make sqlc` / `make swagger` | Regenerated; /songs and /songs/ids include track, song/tag request schemas include disc |

Negative checks use isolated copies/Go overlays without overwriting the working tree:

- Restoring the old sorting mechanism fails album order, pagination and ID assertions: `/tmp/songloft-571-negative/sort.log`.
- Removing disc extraction fails the scan regression with an empty disc: `metadata.log`.
- Removing MP3 TPOS writing fails disc readback/preservation: `tag.log`.
- The old Vorbis reader fails combined-value and alias regressions: `vorbis.log`.
- Restoring the old Lynx album page in an isolated copy fails the new rendered-page check because it requests added_at/desc: `lynx.log`.

## Browser usage

Six synthetic MP3s with real track/disc tags were scanned by the temporary server. Docker Chrome clicks navigated Library → Album → Issue571 → Play all → Queue. Both list and queue showed:

`D1T1 → D1T2 → D1T10 → D2T1 → D2T2 → D2T10`.

Audio time advanced to 0.688924 seconds and the playback path identified D1T1; zero page errors. Results/screenshot: `/tmp/songloft-571-browser-queue.json` and matching PNG. HTTP scan/tag evidence: `/tmp/songloft-571-http.json`.

The real tag endpoint changed D1T10 to disc 2, track 3 and returned `file_write=written`. Reopening the album rendered `D1T1 → D1T2 → D2T1 → D2T2 → D1T10 → D2T10`, with zero page errors. Evidence: `/tmp/songloft-571-browser-edit-final.json` and PNG. Editing used the API; this does not claim new track-editing controls in the client.

## Limits

Not run during local verification: Android/iOS/HarmonyOS devices, Flutter UI and Bundle builds, the reporter's album (no sample/platform supplied), remote CI and releases. Post-push CI is checked against the same SHA and reported on the Issue. Browser/SQLite regressions do not establish device acceptance. Initial browser scripts timed out because of selector/audio-object probing mistakes; corrected runs produced the evidence above. Timeouts are not counted as passes.
