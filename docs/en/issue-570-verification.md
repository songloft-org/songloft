# Issue #570: remote audio cache validation

Related: [songloft-org/songloft#570](https://github.com/songloft-org/songloft/issues/570). [Chinese record](../issue-570-verification.md).

## Validation target

- Date: 2026-11-15.
- Parent baseline: `6b3f38c923cf15358a1e7156c6290cac9d43b8c2`; this section validates the pre-commit working tree on that baseline.
- Content manifest and SHA-256 for the eight changed Go files: `/tmp/songloft-570-code-object.json`; aggregate digest `4d10f4ea8db24823cbf213686715e6dd2551fc16bd4b434b067d2fa28afc64c5`. This covers implementation and tests, excluding documentation.
- Environment: Linux x86_64, Go 1.26.0, FFmpeg 5.1.9; temporary directories, real in-memory SQLite and local HTTP upstreams. No production data writes.

## Behavior and boundaries

- `moveToCache` runs `ProbeForValidation` and the existing `source.Validate` before moving files. This covers external `Get`, HTTP 200 stream finalization, HTTP 206 background full downloads and cache format conversion. Failure leaves `cache_path` untouched; callers clean temporary files. `FinalizeCache` returns `InvalidAudioError`.
- `FindCachedFileBySong` and `Get` validate both structured and legacy cache hits. Invalid files and derivatives for the same song are evicted, the database path is cleared, and callers acquire audio again. Playback, audio-track probing and explicit persistence share this lookup; playback no longer bypasses it through `song.CachePath`.
- Technical metadata for at most 1024 files is memoized against file identity, size and modification time. Every hit still evaluates the current policy and expected song duration. Request cancellation is not classified as invalid audio and does not evict files.
- Plugin-source failures record `OutcomeValidationFail`. An application callback starts the existing per-song deduplicated `AsyncReassign`; its `ModeFallback` handles retries, discovery and successful source persistence. Plain URLs have no plugin source to discover automatically, so invalid cache files are rejected without cross-source search.
- Initial playback remains streaming; full-file validation happens after the background download. A prompt from the initial upstream response can still be heard. The fix prevents persistence and subsequent cache hits; it does not identify every content error or change player retries when all sources fail. Setting `source_validation.enabled=false` still disables validation.
- No lxmusic / miot changes, new settings or API endpoints. Local songs, CUE segments and on-demand transcode derivatives gain no new whole-song duration restriction.

## Regressions and reverse validation

- Services: real PCM covers short prompts, low/high duration mismatch, low bitrate, unparseable files, successful external downloads, stale cache refetch, derivative eviction and database path clearing.
- Hits: runtime policy changes, changed expected duration, equal size/mtime with a different file identity, in-place rewrites, concurrent reads and canceled requests.
- HTTP: FFmpeg generates valid 10- and 40-second MP3 files. Real HTTP 200 and Range 206 proxy responses preserve their bytes; short samples are rejected and valid samples persist intact. An actual `GetSongPlay` handler request removes an old prompt and caches the healthy replacement.
- Fallback: real `SourceFetcher`, `SourceResolver`, `SourceOrchestrator`, SQLite and HTTP audio, with controlled plugin invocation fixtures. Both finalization rejection and stale-cache rejection trigger primary retries, successful alternate-source persistence and healthy audio on the next fetch. Health failure records and temporary-file cleanup are checked.
- A reverse overlay restores unchecked admission/hits and the original playback bypass, retaining current type signatures to run the same tests. Thirteen failing subcases reproduce invalid persistence, stale-file delivery or missing fallback; healthy-audio controls pass. Log: `/tmp/songloft-570-before.log`; overlay location: `/tmp/songloft-570-overlay-path`. The working tree is never overwritten.
- The existing seek-argument test used text as fake cached audio and correctly stopped hitting the cache once validation was enabled. It now uses complete 200-second PCM and continues checking the same seek behavior.

## Checks

| Command | Result / log |
| --- | --- |
| `gofmt -w .`, `git diff --check` | Passed |
| `node docs/node_modules/vitepress/bin/vitepress.js build docs` | Passed; `/tmp/songloft-570-docs.log` (existing bundle-size warning retained) |
| `CGO_ENABLED=0 go test -tags dev,lite ./...` | Passed; `/tmp/songloft-570-tests.log` |
| `CGO_ENABLED=0 go vet -tags dev,lite ./...` | Passed; `/tmp/songloft-570-vet.log` |
| `CGO_ENABLED=1 go test -race -tags dev,lite ./internal/services ./internal/handlers -run TestCacheValidation -count=3` | Passed; `/tmp/songloft-570-race.log` |

Not run: macOS/OrbStack arm64 image, live NetEase sources, Xiaomi speaker and the reporter's instance; this Linux environment has none of these devices or instances. Client UI, native packaging and release CI were not run during local validation; only backend cache behavior changed. Post-push CI and version availability follow the delivery reply on #570. Script tests are not applicable because scripts are unchanged.

Final working-tree evidence is collected with `scripts/ai-review-context.mjs commits` in `/tmp/songloft-570-worktree-20261115-final.json`; its source manifest matches the validation target above.

Suggested commit: `fix(cache): 统一校验远程音频缓存并淘汰无效文件 (#570)`.
