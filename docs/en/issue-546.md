# Permanent access tokens (songloft-org/songloft#546)

Flutter (Settings → Account → Permanent access tokens) and Lynx (Settings → Permanent access tokens) can create separate device credentials, copy the full token or Authorization header once, and display creation/last-use times, revocation status and paginated metadata. The database stores only hashes. Tokens have no expiry and survive client logout and server restarts. See the [API specification](api_response.md) for the endpoint and invalidation contract.

## Implementation and boundaries

- `0040_personal_tokens.sql` adds a separate table; sqlc generates fixed queries accessed through the authentication repository.
- Credentials contain 32 bytes from `crypto/rand` with an `sl_pat_` prefix and a stored SHA-256 hash. Lists return only management IDs, names, short prefixes, timestamps and revocation status.
- Every authentication checks the database; revocation affects subsequent requests. Last use is written at most once per minute. Each token has its own ClientID for media-session isolation.
- Business APIs accept Bearer headers and `access_token` query parameters. Existing/new token management and logout require an ordinary signed-in Access Token and reject plugin, permanent and Refresh Tokens.
- Both clients retain the complete credential only in temporary dialog state. Lynx excludes creation responses from Query/Mutation caches; Flutter suppresses debug logging for these endpoints. Clipboard failure keeps the dialog open for retry or manual selection.
- The initial version grants administrator access to business APIs. Fine-grained scopes, automatic rotation and editable expiry are outside this implementation.

## Verification identity

2026-10-28, Linux x86_64, Go 1.26.0, Node 22.17.1, pnpm 11.15.1, Flutter 3.44.8 and FFmpeg 5.1.7. Tests cover these baselines plus the uncommitted worktrees:

| Repository | Baseline |
| --- | --- |
| songloft-org/songloft | `022a56cebe4b4b6bc1ff7ca043f56d426bddaf71` |
| songloft-org/songloft-player-flutter | `224fec0f74f31139761935c612e1ceb8d8e24717` |
| songloft-org/songloft-player-lynx | `1d9317a87ef2967b6acfd90ba6ac8b731fbe5c48` |

Final source fingerprints and log index: `/tmp/songloft-546-verification.json`. Fingerprints cover tracked diffs and untracked source content, excluding this record and progress/handoff documents to avoid self-reference.

Client delivery commits: Flutter `3941eca`, Lynx `29ebc94` (including preceding Markdown fixture correction `26b5a99`). The parent synchronizes these gitlinks and stores the actual screenshots below for the Issue reply. The repeated full verification uses `/tmp/songloft-546-review-*.log`: all 793 Flutter tests, 3724 Lynx tests, 89 Lynx release-tool checks, full backend tests and vet pass.

## Results

- Backend `CGO_ENABLED=0 go vet -tags dev,lite ./...`, full tests with the same tags, and race checks for new service/HTTP cases pass. App/handler/middleware tests were repeated after the final auth-route change. All 59 script tests pass. `gofmt -w .`, `make sqlc` and `make swagger` ran; Swagger includes the new paths and response models.
- New tests cover name validation, credential isolation, hash-only storage, real pagination, throttled last-use updates, service recreation, immediate/idempotent revocation, management identity restrictions, and header/query authentication. Isolated Go overlays removing revocation or signed-in-session checks make the corresponding tests fail as expected.
- Analysis and all 793 Flutter tests were repeated on the final worktree and pass. The release Web build and actual browser flow pass. New cases exercise creation, both copy formats, clipboard failure recovery, hiding the credential after closing, confirmed revocation and credential-free debug logs.
- Lynx types, dual bundles and embedded Web build pass. The first full run exposed two stale Markdown fixtures: the concise release template no longer contains their expected headings/tables. Both tests now read an independent bilingual table fixture, preserving all existing assertions. The final full suite passes all 3724 tests across 352 files; all 89 release-tool checks also pass. No checks were skipped or weakened.
- Actual HTTP with a temporary database confirms device credentials survive client logout and server process restart; revoking one immediately returns 401 while another remains 200. Management APIs return 403, and neither lists nor database records contain full credentials.

Browser evidence lives in `/tmp/songloft-546-{lynx,flutter}-ui-result.json` and matching PNGs; restart evidence is `/tmp/songloft-546-restart-result.json`. Media fixtures are a generated two-second audio file and embedded cover in the cache directory; no private music library was used.

Both clients complete actual creation, full token/header copying, clearing the credential after closing, confirmed revocation and subsequent 401 in Docker Chrome. Lynx additionally downloads a real cover using a query token (200 / 248 bytes). Clipboard permissions and page focus are enabled; no mock clipboard replaces browser behavior. Flutter list timestamps were subsequently localized to dates/minutes with focused tests, build and browser checks repeated. Both the Go dev full embedded and dev,lite builds pass.

Lynx completes the same flow at 1280×900 and 390×844 browser viewports, covering the detail pane and mobile route; viewport checks do not establish mobile-device behavior. The full embedded Go server serves its index, creates a permanent token and authenticates a song request using a query credential under `/music`. Evidence: `/tmp/songloft-546-full-smoke-result.json`.

## Actual UI screenshots

The Chinese UI was retested with an isolated account and synthetic media: creation, copying the token/header, hiding the credential after closing, cover download and 401 after revocation. Copy screenshots temporarily switch the browser's credential input to password display without changing its value; full credentials were not saved. Device names are test data.

![Flutter permanent token creation](../assets/issue-546/flutter-create.png)

![Flutter management list](../assets/issue-546/flutter-list.png)

![Lynx copying with the credential concealed](../assets/issue-546/lynx-copy.png)

![Lynx narrow copy view with the credential concealed](../assets/issue-546/lynx-mobile.png)

## Not run

The reporter's Synology, actual Home Assistant / ESPHome firmware, MIoT speakers, and Android/iOS/HarmonyOS/desktop native host builds and devices were not tested. Browser/shared-code checks do not replace those acceptance checks. This batch is authorized for commit, push and a reply to #546, delivering child repositories before parent gitlinks. Formal release and Issue closure are outside authorization. GitHub delivery records govern actual SHAs, CI and the reply.
