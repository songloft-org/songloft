# #562 implementation and verification

2026-11-11, Linux amd64, Go 1.26.0, Node 22.17.1, Flutter 3.41.5, Docker browserless Chrome 121. Scope: source classification, unique names, artwork and layout for [songloft-org/songloft#562](https://github.com/songloft-org/songloft/issues/562). [中文](../issue-562-verification.md).

## Verified worktrees

Checks initially targeted uncommitted worktrees. The user subsequently authorized commits, pushes, gitlink updates and an Issue reply. The table records test baselines and code digests; delivery commits are listed below. Digests hash sorted names/content of changed non-Markdown regular files, excluding directories/gitlinks. Full object: `/tmp/songloft-562-implementation/worktrees.json`.

These fingerprints identify the same-day review fixes. Initial full suites and browser evidence identify the earlier object archived in `worktrees-before-review.json`; post-review checks are listed separately below.

| Repository path | HEAD | Code diff SHA-256 |
| --- | --- | --- |
| `.` | `22cd87edf308b53ad6301e0b8a418bd8a6e784a2` | `22b06392a880a41b75de04ef89efbe284d0fd755cb55c671667b39836708ed2e` |
| `clients/player-lynx` | `2a18bb57819886a2c9b2c0476bdf51a127f09ae1` | `6cbe92b0516e9ab5c7383be751bb78609485262f8baa703414216054b8bf3716` |
| `clients/player` | `59bc9c7def13e73ced81df045cf77e3ce32f0426` | `3930d76bc9d5aba547d6d6981e70cbf7efbd9136d0f52c9e0a95b7ef97b62920` |
| `plugins/src/songloft-plugin-navidrome` | `04d87b0d79c880d1e648d056375a680e97d78802` | `0d75bf56ef73feedc160d9dda9c396653aea3355a064634ab398701c5ff1cec9` |
| `plugins/toolchain` | `0380621b2a7072b7d2b9c931c37f418d1ab77668` | `33b88431a426ea604bb8de4ecfd337d7eb80398f946ec889042a639e67cc5d51` |

## Implementation

- Songs reuse `song_tags/song_tag_links`; playlists reference `tag:<id>` inside existing `labels`, with no additional link table. Added playlist tag GET/PUT, paginated filtering, separate visible-playlist counts, plugin bridge and SDK types. Replacement preserves system flags; metadata updates preserve current bindings.
- Tag deletion clears playlist references and history in one UnitOfWork, with song links cascading. Tests cover batch rollback, empty playlists, hidden counts, separate aggregates, ID reuse and stale metadata writes. Backup v2 carries definitions and remaps IDs by name, accepts v1 and preserves playlist-name merge behavior.
- Navidrome resolves source/optional connection tags each sync and binds playlists and imported songs. Conflicts allocate `收藏-Navidrome`, then `收藏-Navidrome (2)`. Allocations remain stable; explicit conflicts fail. Only verified owned legacy defaults migrate. Manual names survive and preview uses the same allocator.
- Nonempty playlists expose artwork endpoints. Own artwork takes precedence, then at most 20 members with artwork clues. Remote attempts share a five-second budget and resolve authenticated plugin-relative URLs. Member artwork revalidates; failed candidates fall through.
- Lynx/Flutter library, home and detail share tags, with separate playlist/song browsing. Playlist titles reuse scrolling components; recognized machine-only markers are hidden only for display. Phone covers are 96px; counts are larger. Rows reduce index/side spacing and move duration into subtitles while retaining menus and hit areas.

## Initial checks and limits

| Repository | Command/scenario | Result |
| --- | --- | --- |
| Backend | `go test -tags dev,lite ./...`, final affected four packages, `go vet -tags dev,lite ./...` | Passed |
| Generated files | `make sqlc`, `make swagger VERSION=2.13.9` | Passed; playlist tag routes/count generated, host version unchanged |
| Lynx | `pnpm exec tsc -b`, `pnpm test`, `pnpm run build` | Passed; 3,781 tests and Lynx/Web artifacts |
| Flutter | analyze and tests with `--no-pub` | Passed; final full 821 tests, plus 119 affected checks after home changes and 48 after the final count-font adjustment |
| Flutter Web | Embedded, no CDN, JavaScript target | Passed with `--no-wasm-dry-run --no-web-resources-cdn`; real Chrome home/detail artwork and tags verified |
| Navidrome | format/typecheck/test/validate/check:format | Passed; 29 tests including naming, migration and shared tags |
| SDK | SDK target build and examples build | Passed; initial recursive build lacked a CLI bin link; offline reinstall restored it and examples passed |
| Documentation | `node docs/node_modules/vitepress/bin/vitepress.js build docs` | Passed; bilingual design, verification and API notes synchronized |

Final Flutter tests use `--concurrency=2`. One earlier run alongside concurrent builds timed out in an audio interruption case; its 17-test group and the final full suite pass. An intermediate optional Wasm probe failed while the JavaScript build succeeded. The final build explicitly skips that probe; no Wasm-target verification is claimed.

Negative regression: a Go overlay removes only playlist-reference cleanup on tag deletion without changing the working tree. The stale-reference/ID-reuse test fails as expected, reporting restored `tag:1`; the actual implementation passes. Log: `/tmp/songloft-562-negative-tags.log`.

Actual Lynx Chrome with temporary real SQLite/HTTP backend: 320px/390px covers are 96px, counts 15px, song title widths 142px/212px, favorite/more targets 44px. At 1200px the cover remains 160px. Title transforms change over time; no page exceptions. Opened tag management, unchecked and saved, then confirmed an empty binding through GET. The built Navidrome page accepts, submits and restores the optional connection tag against a controlled API fixture.

The final Flutter Web artifact verifies home, detail and tag browsing against the same real backend. Opening Navidrome from library tags defaults to playlists, sends `GET /playlists?limit=30&offset=0&tag_id=1`, and displays its linked playlist with member artwork. All three flows have no page exceptions. Screenshots: `flutter-home.png`, `flutter-detail.png`, `flutter-tags.png`.

Evidence: `/tmp/songloft-562-implementation/`, including Lynx mobile/320/1200 screenshots, tag-edit JSON, Navidrome screenshot/JSON and Flutter evidence/logs. Temporary tokens are removed from archived browser evidence; authentication fixtures are excluded.

## Same-day code review and fixes

Confirmed and fixed four P2 issues. Each has a behavioral regression failing before and passing after the fix:

1. **Remote timeout blocks local fallback:** when own artwork or an earlier remote member exhausts the five-second budget, the original code cancels the following database read or stops iteration, returning 404 despite available local artwork. Remote attempts now use a separate budget; local reads retain the original request context and iteration continues. Tests cover both paths.
2. **Candidate changes incorrectly return 304:** after removing the first member, an older modification time on the next image satisfies the previous If-Modified-Since, leaving clients showing old artwork. Dynamic fallback removes date validation while retaining thumbnail ETags that include file identity.
3. **Tag deletion retains client playlist caches:** database references are cleared, but both clients only refreshed the tag catalog. Cached tag IDs could resolve to a different source after ID reuse. Flutter now invalidates playlist lists, details and bindings; the Lynx deletion mutation refreshes tag and playlist queries. An actual-delete widget test and a real QueryClient regression both retain `tag:1` before the fix.
4. **Playlist deletion/visibility retains old tag counts:** cached visible-playlist counts do not update on individual/batch deletion, hiding or restoring, potentially affecting the default classification tab. Both clients now invalidate tag lists and catalogs for these actions. All four operations return old counts before the fix and pass afterward in both clients.

Failure evidence: `/tmp/songloft-562-review-before.log` (404 on both timeout paths; 304 on candidate change), `/tmp/songloft-562-review-flutter-before.log` and `/tmp/songloft-562-review-lynx-before.log` (stale references).

Count regression failures: `/tmp/songloft-562-review-counts-flutter-before.log` and `/tmp/songloft-562-review-counts-lynx-before.log`. All four operations fail as expected in each client before the fix.

Post-review: full `go test -tags dev,lite ./...` and vet with the same tags pass. The full Lynx suite passes across 357 files and 3,786 tests; `tsc -b` passes. Flutter library/playlist/home-grid tests pass across 94 tests; analyze passes. Logs use `/tmp/songloft-562-review-*.log`. Navidrome and SDK code did not change during review, so their same-fingerprint verification remains applicable.

Post-review Lynx/Web artifacts and the Flutter embedded JavaScript Web build pass (Flutter: 204.8 seconds, output `/tmp/songloft-562-implementation/flutter-web-review`). The full initial browser flows were not repeated. New review behaviors are covered by actual HTTP handlers, widget delete interactions and QueryClient regressions.

Final full `pnpm test --maxWorkers=4` passes across 357 files and 3,786 tests in 258.59 seconds. Log: `/tmp/songloft-562-review-lynx-all-final.log`. Review documentation builds, all five repository diff/UTF-8 checks pass, and final code fingerprints match the table.

Not run: reporter Honor 30 Pro / HarmonyOS 4 Bundle device, local native Android/iOS/HarmonyOS builds and interactions, and a fresh real Navidrome + QuickJS end-to-end sync. Automated/browser checks do not replace these boundaries. The Issue remains open.

## Commits and delivery

On 2026-11-11 the user explicitly authorized commit, push and Issue reply. All four child repositories have been committed and pushed to main; the parent commit containing this record pins the revisions below. No formal host version was created manually. The Navidrome main push triggers its existing automatic plugin release; other CI/dev builds follow repository workflows. Check each SHA's GitHub Actions and the Issue reply for remote results and available artifacts. CI builds do not replace device acceptance.

| Repository | Commit |
| --- | --- |
| SDK | [75d22c4](https://github.com/songloft-org/plugin-toolchain/commit/75d22c4c655bf86f2c2417b0ae2fac4ed3953b69) |
| Navidrome | Source [a34e0b6](https://github.com/songloft-org/songloft-plugin-navidrome/commit/a34e0b6acebf20acdcb75ad6e69bc2a76570372c); automatic release metadata / parent pin [f271903](https://github.com/songloft-org/songloft-plugin-navidrome/commit/f271903dd3cd5b688c627f61d4e7824673ff9230) |
| Flutter | [87a7699](https://github.com/songloft-org/songloft-player/commit/87a7699d2580b8c4463fa43e159d7bff18f6df29) |
| Lynx | [bc692c1](https://github.com/songloft-org/songloft-player-lynx/commit/bc692c14be23190aef4d3468e428dc292fd456af) |

Before committing, code digests recalculated against the tested baselines match all verified objects. Flutter main additionally retains the existing CHANGELOG-only `65b7145`; business code matches the tested object. Child records saying uncommitted describe verification snapshots; this section and GitHub track delivery.

Navidrome [release CI](https://github.com/songloft-org/songloft-plugin-navidrome/actions/runs/42876372384) passed and published [v2026.11.11.0](https://github.com/songloft-org/songloft-plugin-navidrome/releases/tag/v2026.11.11.0). The manifest confirms `releaseSource=a34e0b6…`. Its automatic commit only changes versions, download URL and artifact hashes in plugin/package manifests and the lockfile. The parent pins that metadata commit; business code matches the tested source.
