# #563 Playlist membership event verification

Target: [songloft-org/songloft#563](https://github.com/songloft-org/songloft/issues/563). Implementation and local validation completed on 2026-11-11. Pre-commit objects are retained below; delivery details are at the end.

## Verification object

- Main repository baseline: `c686994c13cb4a6611a29ef410b3f13236516158` plus this working tree.
- Client baselines: Flutter `87a7699d2580b8c4463fa43e159d7bff18f6df29`, Lynx `bc692c14be23190aef4d3468e428dc292fd456af`, each identifying pre-commit validation; final child commits are listed below.
- SDK repository `songloft-org/plugin-toolchain`: `75d22c4c655bf86f2c2417b0ae2fac4ed3953b69` plus uncommitted `packages/plugin-sdk/src/{global.d.ts,index.ts}` edits. The final release commit is listed below.
- Final file SHA-256 manifest: `/tmp/songloft-563-extra/worktree-final.json`. The existing `songloft-562-implementation/` directory was preserved and excluded from this change fingerprint.
- Environment: Linux amd64, Go 1.26.0, Node 22.17.1, pnpm 11.15.1; real in-memory SQLite migrations, QuickJS and a Chi HTTP test server.

## Behavior and scope

Adds `songloft.events.onPlaylistSongsChange` / `offPlaylistSongsChange`, requiring `playlists.read`. Events contain playlist ID, song ID, add/remove, Unix milliseconds and a title/artist snapshot. Favorites and regular playlists share the API. Single/batch membership writes and atomic replacement through the playlist service are covered, including client HTTP and plugin bridge calls. Rollbacks, skipped/duplicate entries and sorting emit nothing. A later chunk failure still notifies earlier committed additions.

Subscriptions survive idle eviction and wake the plugin. New VMs recheck permission and restore the handler. Explicit unsubscribe, disable and deleted-plugin cleanup remove identity. Operations enter queues as batches, including operations exceeding actor queue capacity. Writes do not wait for plugin network callbacks. Delivery uses bounded queues and is best effort, without persistence or replay.

See the [plugin guide](js-plugin-development-guide.md#songloftevents--playlist-membership-changes) for usage and detailed boundaries. The follow-up adds an authenticated `/playlists/favorites/changes` long-poll endpoint, separate `playlist_added_at` timestamps and automatic refresh in both clients. SQLC and Swagger were regenerated; the database schema is unchanged. Cascading deletion of playlists/library songs, backup restoration and internal conversion replacements do not emit this event.

## Initial core-event results (before the additional requirements)

| Command / scenario | Result and evidence |
| --- | --- |
| `CGO_ENABLED=0 go test -tags dev,lite ./...` | Passed; `/tmp/songloft-563/go-tests-delivery.log` |
| `CGO_ENABLED=0 go vet -tags dev,lite ./...` | Passed; `/tmp/songloft-563/go-vet-final.log` |
| Targeted race regressions (full command below) | All four packages passed three iterations; `/tmp/songloft-563/events-race-final.log` |
| Favorites / regular playlist HTTP additions and removals | Real Chi handlers verify responses and events; duplicate input emits only one add |
| QuickJS lifecycle | Real JS registration, replacement registration, unsubscribe, writes from its own HTTP callback, sequential add/remove, idle wakeup, disable, deleted identity cleanup, permission revocation on reload and a 272-member batch passed |
| SQLite transactions | Membership differences, clearing, silent rollback, partial chunk failures and committed IDs/counts passed |
| Negative checks | Removing notification loses add; restoring the old batch method reports 501 when only 500 commit; removing reload validation inherits revoked permission. All three isolated overlays failed as intended. `/tmp/songloft-563/negative-{notification,batch-baseline,reload}.log` |
| `node --test scripts/tests/*.test.mjs` | All 59 passed; `/tmp/songloft-563/scripts-tests.log` |
| SDK `pnpm --filter @songloft/plugin-sdk typecheck` / `build` | Both passed. `/tmp/songloft-563/sdk-consumer.ts` also compiled against the generated exported declarations; `/tmp/songloft-563/sdk-consumer.log` |
| `npm run docs:build` in `docs/` | Passed; `/tmp/songloft-563/docs-build-final.log` |
| `gofmt -w .`, `git diff --check` in both repositories | Passed |

```bash
CGO_ENABLED=1 go test -race -tags dev,lite ./internal/jsplugin ./internal/services ./internal/database ./internal/handlers -run '^TestPlaylist(Membership|BatchChanges|Events)' -count=3
```

The initial race run exceeded a 10-second queued HTTP read because the fixture repeatedly persisted its entire growing history, causing quadratic serialization. No data race was reported. The fixture now persists only the short lifecycle history and asserts large batches in memory; all four packages then passed three race iterations. Early fixture corrections also addressed a test-package import cycle and incorrectly resetting package hashes. The commands above identify the final results.

## Not run and delivery

- The follow-up changes Flutter/Lynx clients; additional checks and actual Web interactions are recorded below. Not run: mobile/desktop devices and complete embedded release packages.
- Not run: love/unlove against an actual Last.fm account, npm publication or remote CI. No sync plugin was changed, and publication/push were not authorized.
- Suggested commits: SDK `feat(sdk): 添加歌单成员变更事件类型（Ref songloft-org/songloft#563）`; main repository `feat(jsplugin): 广播已提交的歌单成员变更（Fixes #563）`. Commit the SDK first, then update the parent pointer.

## 2026-11-11 additional requirements

- Full, paged, filtered, HTTP and QuickJS full/brief playlist member results include `playlist_added_at`; `Song.added_at` retains library time. Atomic replacement preserves retained member rows, applying membership differences and positions without resetting retained timestamps.
- Authenticated `GET /api/v1/playlists/favorites/changes` immediately returns `changed=true` for initial/stale cursors. Matching cursors hang for up to 25 seconds; committed song/radio favorite changes wake all waiting clients. Cursors include process identity, invalidating across restart. Disconnect and shutdown release waiters; no persistent event log is required.
- Flutter subscribes from the favorite Provider; Lynx subscribes at app scope, including fullscreen playback. Notifications reload complete IDs, update hearts and invalidate built-in playlist views. Late responses cannot cross session changes. Background pauses, resume reconciliation, failure backoff and older-host 404 shutdown are handled. Flutter also prevents an old initialization restoring IDs after session reset.
- Main repository evidence: `/tmp/songloft-563-extra/go-tests.log`, `go-vet.log`, `go-race.log`. Checks cover separate member time, retained timestamps, authenticated 401, multiple waiters, cancellation, restart and shutdown. Targeted Go race checks passed three iterations.
- Negative checks: restoring clear/recreate replacement fails retained member time; removing favorite invalidation fails built-in favorite notifications. `negative-time-final.log`, `negative-notify.log`. An initial timestamp overlay selected the wrong range and failed compilation; only the corrected behavioral failure counts.
- SDK typecheck/build: `sdk-types.log`, `sdk-build.log`. SQLC/Swagger generation and field/route checks passed. All follow-up logs are under `/tmp/songloft-563-extra/`.
- Final Flutter checks: Flutter 3.44.8 / Dart 3.12.2, clean `flutter analyze`, all 831 tests passed. Web built with `flutter build web --profile --no-wasm-dry-run --no-web-resources-cdn --dart-define=DEPLOY_MODE=embedded --output=/tmp/songloft-563-extra/flutter-web`. `flutter-{analyze-delivery,tests,build-delivery}.log`. Profile Web validation does not establish release-package/native compilation.
- Lynx: `pnpm exec tsc -b`, all 359 files / 3793 tests passed. `pnpm exec rspeedy build --environment lynx` and `pnpm run build:web` completed both targets. `lynx-{types-delivery,tests-delivery,native-build,web}.log`.
- Docker Chrome 121 at 1280×900, temporary backend/authentication and real `verify563` QuickJS plugin. Its HTTP handler calls `playlists.addSongs/removeSongs`. Without clicking hearts or reloading, the same open page changes Add→Remove→Add in Lynx and Favorite→Unfavorite→Favorite in Flutter. Final backend executable: `songloft-delivery`; JSON: `lynx-ui-evidence-delivery.json`, `flutter-ui-evidence-delivery.json`; screenshots: `{lynx,flutter}-delivery-{addedShot,removedShot}.png`. Request credentials are redacted. Final file/artifact hashes: this batch's `worktree-final.json`.
- Full Go tests, vet and three targeted race iterations across four packages passed; SDK types/build, SQLC/Swagger and documentation build passed. Final documentation log: `docs-build-delivery.log`.
- Retained intermediate failures: parallel Flutter/Lynx compilation hit memory pressure, killing Lynx with exit 137. Flutter was stopped, then built in Profile mode sequentially. The first Lynx full suite had one failure from the missing `dist/main.lynx.bundle`; the complete 3793-test rerun passed after builds. The initial Flutter browser script looked for a heart on a playlist row that does not display one by default; navigation was corrected to the actual library row, without product UI edits.
- Client records: Flutter `docs/cn/issue-563.md` / `docs/en/issue-563.md`; Lynx verification/handoff have separate bilingual files. Existing `docs/project/progress.md` intentionally keeps its bilingual single-file history rather than introducing a full English history copy. Nothing committed, pushed, released or updated in gitlinks.

## Commits, SDK version and delivery

The user subsequently authorized main/submodule commits, pushes, SDK version publication and an Issue reply. Formal client version bumps and Issue closure are outside this authorization.

- Flutter: [`c0cfa70`](https://github.com/songloft-org/songloft-player/commit/c0cfa70d4f5d85edc969dc71b258dc56308fb9e9); main triggers dev builds.
- Lynx: [`84a1f56`](https://github.com/songloft-org/songloft-player-lynx/commit/84a1f5601382c4d17c0c50c97f134d9198c03a60); main triggers dev builds.
- SDK: feature commit `82b58c8`, release commit [`bc58da6`](https://github.com/songloft-org/plugin-toolchain/commit/bc58da63333521f2cb7192c44fcfffcf61656616). `node scripts/release.mjs minor --yes` advances `2.18.0` to `2.19.0` and pushes [`v2.19.0`](https://github.com/songloft-org/plugin-toolchain/releases/tag/v2.19.0), updating linked publication packages as the script specifies. Actual npm availability is determined by that Release workflow and npm.
- Child repositories are committed/pushed first; the parent commit updates all three gitlinks. `Ref #563` avoids automatically closing the Issue without separate authorization.
- Required CI is followed on the same SHAs. The final Issue reply distinguishes actual publication status and unverified native-device/Last.fm boundaries. Delivery logs and final SHAs are under `/tmp/songloft-563-publish/`; the earlier `worktree-final.json` retains pre-commit source fingerprints.
