# Issue #567: Cross-origin HLS resource proxy verification

## Scope and code

- Repository / issue: songloft-org/songloft#567, https://github.com/songloft-org/songloft/issues/567.
- Baseline: `6115afb969ff1f4b7ced0fbe39d223350adb9e5c`; `main` started clean and remote main had the same SHA. This document records local verification during implementation; subsequent commits, pushes, CI, and replies are tracked in the issue delivery record.
- Environment / date: 2026-11-14, Linux 6.1 amd64, Go 1.26.0, FFmpeg 5.1.9, Docker HeadlessChrome 121 / hls.js 1.5.20. Real in-memory SQLite, HTTP servers, and media decoding; upstream public-address fixtures were routed locally without contacting services at those example IPs.
- Verified object: baseline plus this working tree. Core source SHA-256:

| File | SHA-256 |
| --- | --- |
| `internal/handlers/hls.go` | `90704fc4561a7ba8c1725ec172baa92ac62ef7641f4e67e68023013d8378a129` |
| `internal/handlers/hls_test.go` | `13ef894d0abf2d0c6f78fe12d6dc684501a139805656d6fa14f4ce3432e85cf8` |
| `internal/handlers/hls_cross_origin_test.go` | `a7513e0e5838114d8c8e9ded70a6f94b8dd660a91a68904029f813547f84d453` |

The complete working-tree identity is captured by `scripts/ai-review-context.mjs commits` in `/tmp/songloft-567/worktree-final.json`. Logs, screenshots, and isolated overlays live under `/tmp/songloft-567/` and contain no private source address from the reporter.

## Cause and resulting behavior

Previously, `rewriteRef` rewrote only references sharing the upstream playlist's origin, while endpoints required targets to share `song.URL`'s origin. A public IPTV service emitting cross-origin CDN URLs therefore left clients fetching those resources directly even with HLS proxying enabled. Relative resources after redirects could also be rejected by endpoints. The reporter's comments match this mechanism, but the complete live URL and exported logs were unavailable, so this does not establish every cause of the reported playback failure.

Recognized HTTP(S) resources (child playlists, segments, keys, init resources, and LL-HLS) now become relative proxy links carrying an HMAC-SHA256 `sig` bound to the song ID, current source URL, endpoint type, and complete resource URL. Unsigned requests retain origin checks. Tampering with signatures, URLs, songs, or endpoints returns 403; editing the source invalidates old signatures. Entry requests and every redirect independently apply the existing private-host policy without the generic proxy's private allowlist. References such as `data:` / `skd:` remain unchanged.

Relative resources resolve against the final redirected playlist URL. Deployment prefixes, `access_token`, upstream time-window queries, HEAD, Range / 206, and byte-level response headers are preserved. The random key lasts for the current process; refetch playlists after a restart.

## Verification results

| Command / scenario | Result and evidence |
| --- | --- |
| Old-code Go overlay running cross-origin child playlist, segment/key, and different-port regressions | Expected failures: resources remain direct URLs; `old-overlay.log`. Early regressions also recorded in `before.log` |
| Three separate overlays removing signature, entry private-host, or redirect-host checks | Expected failures detect unauthorized requests or extra upstream accesses; `signature-mutation.log`, `private-mutation.log`, `redirect-mutation.log` |
| `go test -tags dev,lite ./internal/handlers -run 'TestRewriteM3U8\|TestHLS\|TestIsSameOrigin' -count=1` | Passed; `after.log`. Final stronger redirected-path assertions are covered by the full and race runs below |
| `go test -tags dev,lite ./...` | Passed; `tests-final.log` |
| `go test -race -tags dev,lite ./internal/handlers -run 'TestRewriteM3U8\|TestHLS\|TestIsSameOrigin' -count=1` | Passed; `race-final.log` |
| `go vet -tags dev,lite ./...` | Passed; `vet-final.log` |
| `make swagger VERSION=2.13.9` | Passed; `swagger.log`. Both HLS paths contain `sig` parameters and updated 403 semantics; all three generated files synchronized |
| `npm run docs:build` (docs directory) | Passed; `docs-final.log`, with existing chunk-size warnings |
| `gofmt -w .`, `git diff --check`, changed-file UTF-8 validation | Passed |
| Real FFmpeg decoding cross-origin HLS over HTTP | Passed: generates two seconds of AAC/MPEG-TS and decodes the signed proxy stream into at least 30,000 bytes of mono 8 kHz PCM. Initial full / race runs failed because FFmpeg rejected extensionless proxy endpoints; reruns passed after configuring supported `allowed_extensions`, `allowed_segment_extensions`, and `extension_picky` options in the fixture. Initial logs remain in `tests.log` and `race.log` |
| Clicking the playback button in Docker Chrome | Passed: a separate fixture uses the production HLS handler and repository hls.js; no product UI changes. `currentTime=1.010818`, `readyState=4`, `paused=false`, no playback/page errors, all HTTP requests point to the local Songloft test server; `browser.json`, `browser.png`, `browser-fixture.log` |

The HTTP fixture retains production host policy and redirect callbacks, redirecting public-IP DialContext connections only to a local server. It covers cross-origin master and child redirects, relative resources under the final path, cross-origin keys / init / PART / segments, unsigned requests, signature and URL tampering, reuse across songs/endpoints, invalidation after source edits, and blocking private / loopback / IPv6 / metadata addresses and redirects.

## Not run and delivery limits

- The reporter's actual ysp-live deployment and CCTV stream: source address hidden, no reproducible deployment or complete logs supplied; only the equivalent proxy mechanism was verified.
- Complete Songloft client interactions, Android/iOS devices, video rendering, and specific codec compatibility: no client code changed; browser and FFmpeg used synthetic audio fixtures.
- Full embedded build, packaging, remote CI, and release: no commit or push had occurred during local verification; this document covers the `dev,lite` backend and documentation build.
- Private ysp-live sources remain blocked by HLS even if listed in the generic private allowlist. This preserves the existing product boundary; fixing public cross-origin proxying does not establish support for arbitrary private IPTV sources.

Suggested commit message: `fix(hls): 为跨域播放资源生成签名代理链接 (#567)`.
