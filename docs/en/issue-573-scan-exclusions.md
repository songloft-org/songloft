# #573 scan exclusion diagnostics

Related: [songloft-org/songloft#573](https://github.com/songloft-org/songloft/issues/573#issuecomment-6946173537). On 2026-11-17 the reporter identified the inherited `yt-dlp` exclusion path. The user chose explanatory diagnostics and validation while retaining exclusion rules, without a temporary bypass.

## Behavior and contract

- `POST /scan/validate` shares path normalization and exclusion matching with scanning. It returns eligible scan roots in `paths` and excluded roots in `excluded_directories` (`path/rule_type/rule`). Missing/empty input paths check the music root. It does not walk subtrees, start jobs or change configuration.
- `POST /scan` checks the current rules again. If every root is excluded it returns 400 and leaves progress unchanged. Mixed roots retain the existing scan scope and report excluded roots in the response. Existing boundary checks, deduplication, incremental/reimport behavior and cleanup scope remain in place.
- Traversal prunes excluded directories and records their paths and matching rules in `/scan/progress.excluded_directories`. A pruned subtree counts as one directory, never an estimated song count. It is not walked for diagnostics; each new scan resets the list.
- Flutter and Lynx explain exclusions after selection and recheck before starting. All-excluded roots or failed validation prevent the scan request while keeping the controls available. Mixed selections show skipped directories; results distinguish excluded directories from already-existing skipped songs.
- Settings explain scan exclusions versus automatic playlist exclusions. Use `auto_create_exclude_dirs` to avoid ordinary one-song folder playlists while still importing songs. Existing cleanup on saving scan exclusions remains unchanged.
- Swagger was regenerated. The new client diagnostics require the matching server update. No ignore-exclusion parameter was added.

## Verified code and evidence

Date: 2026-11-17. Linux amd64; Go 1.26.0; Flutter 3.44.8 / Dart 3.12.2; Node 22.17.1; pnpm 11.15.1; Docker headless Chrome 121. Parent baseline `4e73b078d86472e0c3409d24a87a6aca2b82380a`, Flutter `c333cf80a035020e38672f9c041fd907e4f65c67`, Lynx `034cf838a46e59a26d1c24df91bf6326f0562e6b`, each plus this uncommitted worktree. Existing parent-watch and Lynx desktop work was preserved and not committed by this task. Source file hashes: `/tmp/songloft-573-exclusions/source-identities.json`.

| Check | Result / logs under `/tmp/songloft-573-exclusions/` |
| --- | --- |
| `make check` | Formatting, vet, complete backend and script checks pass; `make-check-final.log` |
| Scanner/service and handler checks | Pass; `go-targeted-final.log`, `go-handler-final.log`; name/path/inheritance/prefix boundaries, mixed roots, traversal rejection, all-excluded refusal and progress reset |
| `go test -race ./internal/services -run TestScanExclusionDiagnostics -count=1` | Pass; `scan-race.log` |
| Go overlays removing the start guard or traversal diagnostics | Both regressions fail for the expected behavior; `negative-handler.log`, `negative-scanner.log`; working files remain intact |
| `make swagger`, full embedded and lite backend builds | Pass; `swagger-final.log`, `full-backend-build.log`, `backend-build-final.log`; `/scan/validate` and diagnostic response types verified |
| Flutter complete/final focused tests and analysis | 853 complete tests, 4 final focused tests and analysis pass; `flutter-tests.log`, `flutter-targeted-final.log`, `flutter-analyze-delivery.log` |
| Flutter isolated original scan page | New regression fails because exclusion diagnostics are absent; `negative-flutter-final.log`. Initial missing-resource/local-dependency compilation failures do not count as negative verification |
| Lynx focused/complete tests, types and builds | 230 focused tests, 368 files/3854 complete tests, types, both Lynx/Web outputs and embedded Web pass; `lynx-targeted-final.log`, `lynx-tests.log`, `lynx-types-final.log`, `lynx-build.log`, `lynx-web-build.log`. The first type check encountered an existing desktop candidate error; subsequent checking passed |
| Lynx isolated removed guards/diagnostics | Two UI regressions fail because diagnostics are missing; `negative-lynx.log`. The initial isolated dependency-resolution failure does not count as negative verification |
| Temporary real backend HTTP scenarios | All-excluded start returns 400 and remains idle; mixed roots import one song and report one excluded directory; full scanning reports both name/path exclusions; playlist-only exclusion permits import without creating the corresponding folder playlist. `http-results.json`, `http_probe.py` |
| Flutter Web artifact | Build passes; `flutter-web-build.log` |
| Flutter browser mouse interaction | Login, settings navigation, directory selection, all-excluded refusal, mixed scanning and results pass. The two start clicks produce exactly one real scan request. `ui-flutter-results.json`, `ui-flutter.js`, `flutter-{blocked,result}-desktop.png` |
| Lynx browser page events with real backend | An excluded descendant displays the inherited rule; all-excluded roots do not start, mixed roots produce exactly one real scan, terminal results retain the rule and no pageerror occurs. `ui-lynx-results.json`, `ui-lynx-final.js`, `lynx-{blocked,result}-desktop.png`. Worker requests are verified against backend logs rather than page request events alone |

Lynx mouse/touch automation did not pass: Chrome 121 focus delegation moves clicks to the exclusion-name input lower on the page. Failures remain in `ui-lynx-debug-raw.json`, `ui-lynx-touch-raw.json` and `lynx-debug.png`. Subsequent page-event integration uses DOM `click()` to avoid that default focus action. It verifies scan interaction/backend behavior, not real mouse acceptance. Generic focus handling was not changed in this batch.

The temporary library is `/home/hanxi/.cache/songloft-573-exclusions-music`, outside the default excluded `tmp` name, with synthesized MP3s and isolated SQLite. No user library or production configuration was changed.

Not run: Windows native packages, Android/iOS/HarmonyOS devices, the reporter's Docker mounts and private audio. The user subsequently authorized this batch's commits, pushes and Issue reply. Actual commits, parent gitlinks, same-SHA CI and comment links are tracked in `/tmp/songloft-573-exclusions/delivery.json`. No stable release is created; existing desktop candidate and parent-watch changes are outside this batch's commits.
