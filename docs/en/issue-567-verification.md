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

## 2026-11-14 follow-up: real ysp-live / CCTV-1 verification

- Songloft: committed `3ad14b4a8aed4b82eb912366ff80212a2947c4b8`, with no production code changes. ysp-live: [pinned upstream source](https://github.com/tmxk2020/ysp-live/blob/311d7bb5222f16ce7f5ee20709c6c5b73aea2af0/ysp-live-v9.5.py), v9.5.1, source SHA-256 `5354ba6aaecb8d5e514cc8df12656722e30652ba0372b20ea761e5bff6b81b9e`.
- Temporary deployment: `python -u ysp-live.py 18767 --bind 127.0.0.1 --no-4k --no-engine --data-dir <temporary-directory>`. Uses its real 1080p JCE fallback without simulated playlists or CDN responses. The subscription contains 64 channels; CCTV-1 returns 14 segments targeting `tlivecloud-playback-cdn.ysp.cctv.cn`.
- Method: real SQLite and the current HLS handler; a Go overlay adds only the test driver. To retain production private-host policy, connections for the test source URL's `8.8.8.8:18767` are mapped exclusively to local ysp-live. CDN connections use the real network with unchanged host checks. This verifies the real source and proxy, not private-source support or complete client import acceptance.
- Old-code overlay on the same captured real playlist fails: 0/14 cross-origin segments rewritten. Current code passes: 14/14. Logs: `ysp-real/captured-{before,after}.log`.
- ffprobe through Songloft identifies H.264 1920×1080 and AAC 44.1 kHz. FFmpeg decodes 12 seconds / 299 frames and exits 0. Logs contain MPEG-TS boundary corrupt-packet warnings; this is not described as a warning-free run. Full log: `ysp-real/ffmpeg.log`; progress: `ffmpeg-progress.txt`.
- Direct and proxied copies of a real segment are both 3,285,488 bytes with SHA-256 `3d06f08bf9383501ea70f0c1f3d598b831f13c467a42ab01c6740c089baebe8d`, byte-for-byte identical; see `segment-integrity.json`.
- Docker Chrome / hls.js playback advances from 56.883789 to 58.927471 seconds and from 30 to 81 decoded video frames; 1920×1080, `readyState=4`, `paused=false`, no media/HLS errors, one dropped frame. A CCTV-1 video frame was captured and visually checked. Evidence: `browser-final.json`, `browser-frame.png`. Initial browser screenshot calls timed out; a subsequent continuous-progress, decoded-frame, and canvas video-frame capture succeeded before browser playback was considered verified.
- The first real run records 19 upstream requests: eight ysp-live playlists returning 200 and eleven CCTV CDN segments (six returning 200, five returning 206); see `upstream-requests-first.json`. Both test-driver runs pass. Evidence lives under `/tmp/songloft-567/ysp-real/`; temporary services are stopped after verification.
- Not run: the reporter's private deployment, full Songloft client interactions, native devices, 4K/device protocol/Web WASM engine, and other channels. This establishes only that a separately deployed v9.5.1 CCTV-1 1080p live stream can be proxied and played through the repaired HLS path.
