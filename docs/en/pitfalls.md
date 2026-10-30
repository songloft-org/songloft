# Business Constraints and Pitfalls

This page retains parent-repository business constraints, design trade-offs, and regression history, indexed by task in `AGENTS.en.md`. Read the relevant sections before editing; summaries do not replace the full constraints.

Code paths are relative to the repository root. Record new lessons through the [development and retrospective workflow](ai-development.md); do not copy entire topics back into the entry point. See the [Chinese version](../pitfalls.md).

<a id="plugin-identity"></a>

## Plugin store entry_path collisions (identity)

`entry_path` is nowhere near as simple as a "plugin ID" — it is simultaneously the registry
dedup key, the install-state match key, the `js_plugins.entry_path` UNIQUE constraint, the ZIP
filename (`<entryPath>.jsplugin.zip`), the static directory name
(`jsplugins_data/<entryPath>/static`), the route prefix (`/api/v1/jsplugin/{entryPath}/*`), the
manager/scheduler in-memory map key, and the ownership key behind
`plugin_storage.plugin_entry_path` / `songs.plugin_entry_path`. Two plugins from different
authors can absolutely collide on one entry_path (songloft-org/songloft#339). Read all of the
below before touching this area.

- **Two plugins sharing an entry_path cannot coexist locally** — that is a data-layer fact, not
  a UI limitation. #339 only fixed the store layer (show both rows, resolve install state per
  row, require confirmation before replacing). Real coexistence needs a genuine plugin id with
  entry_path demoted to a disambiguable route prefix, which means touching the DB constraint,
  the on-disk layout, routing, `EntryPathFromZipName`, the orphan cleanup in
  `SyncPluginsFromDirectory`, plus migrations for `plugin_storage` / `songs`
- **Identity = normalized author, falling back to the GitHub `owner/repo` from `update_url`**
  (`internal/jsplugin/identity.go`). Author normalization must strip `<email>` and `(notes)`
  before lowercasing — the same plugin is commonly written as `hanxi` in one registry and
  `Hanxi <a@b.com>` in another, and skipping normalization splits it into two store rows.
  **Non-GitHub self-hosted URLs are deliberately not inferred**: their path layout is arbitrary,
  so `/plugins/a/` and `/plugins/b/` would read as two repos and split one plugin's two mirrors
- **`SameIdentity` returns true when either side is empty** (treated as the same plugin). Better
  to miss a conflict than to invent one and block a legitimate upgrade just because the other
  side has no author
- **The cross-registry dedup key deliberately excludes the source URL**
  (`FetchAndMergeMulti`): the official registry and community aggregators routinely list the
  same plugin, and folding the source into the key would show it several times in "All" mode.
  Identity already separates genuinely different plugins
- **The manual-upload path deliberately skips conflict detection**: `InstallFromUpload` keeps its
  original behavior, and only store installs go through
  `InstallFromUploadWithOptions(RejectIdentityConflict: true)`. A manual upload is an explicit
  user-chosen file, and an author-spelling change during normal plugin iteration must not fail it
- **On conflict, return before writing anything**: `package.go` used to silently fall through to
  `Update` on a collision — overwriting the ZIP, re-extracting after `os.RemoveAll(staticDir)`,
  and rewriting the DB row in place (keeping the original ID and status). The original plugin was
  destroyed without a word, the newcomer inherited all of its `plugin_storage` data, and songs
  imported by the original (`songs.plugin_entry_path`) were reattributed to the newcomer
- **The store's `has_update` must use `CompareVersion(...) > 0`, never string `!=`**: on a
  collision the two versions differ by definition, so string comparison shows "update available"
  forever, and clicking it swaps the local plugin for a different author's plugin
- **Frontend in-place install-state updates must match on `(entryPath, identity)`**
  (`RegistryPluginEntry.matches`): matching on entryPath alone lights up every same-named row as
  "installed". List item `key`s use `rowKey` (`entryPath|identity`) rather than the index —
  `_RegistryPluginItem` holds local `_installing` state, and index-based Element reuse makes the
  spinner jump to another row

<a id="registry-cache"></a>

## Plugin store result cache

Store paging and search both operate on the **complete plugin list** (server-side slicing +
substring filtering), so every page turn and every search-term edit used to trigger a full
registry fetch — up to 500 `plugin.json` files, 8 concurrent, 15s timeout each.
`registry_cache.go` adds a 5-minute in-process TTL cache over fetch results.

- **`RegistryService` must be held long-term by the handler** (`JSPluginHandler.registrySvc`).
  It used to be `NewRegistryService()` per HTTP request, which would never hit the cache
- **`proxyDown` therefore must be a per-call local**, not a `RegistryService` field. It remembers
  "the GitHub proxy has failed during this fetch"; as a singleton field it would pin the service
  to **direct connections forever** after one failure (never retrying even once the proxy
  recovers) and let concurrent requests interfere. That's why the private method signatures all
  carry `proxyDown *atomic.Bool`
- **The cache key must include every input that affects the result**: mode (single/all), registry
  URLs, tokens, source order, and `github_proxy`. Source order cannot be omitted —
  `FetchAndMergeMulti` uses it to decide which registry wins for equal versions. **Hash the token
  before putting it in the key**: cache keys reach logs and debug output and must not carry
  plaintext credentials
- **Install state is deliberately not cached**: `installed` / `has_update` / `conflict` are
  recomputed from the DB by `buildInstalledMap` on every request, so paging right after an
  install still shows fresh state and no invalidation is needed
- **Failures are not cached** (and don't disturb an existing entry): otherwise one network blip
  would pin the error for a whole TTL. `FetchAndMergeMulti` returns no error (per-source failures
  only become warnings), so that path instead **declines to cache an empty result**
- **Only "refresh" and "retry after failure" pass `force: true`** on the frontend; paging,
  search, initial load, and source switching do not. Source switching and config edits re-fetch
  naturally because the cache key changes; `UpdateRegistriesSetting` additionally calls
  `InvalidateCache()` to free up entry quota

<a id="scan-title"></a>

## Scan title rules

- Default `scan_title_source=tag`: use the tag title when present, otherwise the filename minus its extension.
- Explicit `scan_title_source=filename` always uses the filename, not the tag; the branch is in `internal/services/metadata.go`.
- **Do not** apply "longest-common-substring dedup + concatenation" — it produces results like "Artist - Title" that redundantly stuff the artist into the title field
- Video container probe: when scanning containers like mp4/mov/m4v/mkv/webm/avi/ts/mpg/mpeg/flv/wmv/rm/rmvb/3gp, ffprobe detects whether a real video track is present (excluding the cover attached_pic) to set `songs.is_video`; the client uses this to render the picture / pick the cast mime
- **When verifying locally, don't put the music directory under `/tmp`**: the default `exclude_dirs` for
  `music_path` includes `tmp`, and `ShouldExcludeDir` matches **any path segment by directory name**, so
  the entire `/tmp/...` root gets excluded. The symptom is a scan that "completes successfully" with
  `discovered_files=0` — **no error, no warning** — easy to misread as your own change breaking things

<a id="scan-batches"></a>

## Streaming scan imports (large-library iron rule — songloft-org/songloft#447)

`doScanAndImport` imports **incrementally**: `scanBatchAccumulator` (`scan_batch.go`) groups results by
directory and calls `flushScanBatch` as soon as a directory is finalized. Do not revert to
"accumulate everything, then write once" — on a library of tens of thousands of tracks, progress stays
at 0 and not a single row is written for the entire extraction phase (tens of minutes), which users can
only read as a hang.

- **The finalization unit must be the directory, not a fixed count**: `fixSpamTags` groups by directory
  and flags spam tags when "more than half the files in this directory share the same
  `(title, artist)`", so it needs the whole directory as its sample. Cutting batches every 50 results
  splits one directory across several batches and makes the verdict drift with the cut points
- **`scanDirGroupCap` is the memory backstop**: a directory holding thousands of flat files finalizes
  early once it hits the cap, otherwise this degrades back to whole-library accumulation. The cost is
  that such a directory's spam-tag verdict uses a sub-sample, which is acceptable (the test is >50%
  and ≥3 matches)
- **Progress counters only advance after a batch lands in the DB** (`UpdateProgress` inside
  `flushScanBatch`). The extraction phase reports the current file via `SetCurrentFile` instead — never
  call `UpdateProgress` from the extraction phase, that double-counts
- **The write context and the cancellation context must be separate**: `doScanAndImport` derives a
  cancellable ctx from its argument (to kill ffmpeg subprocesses on cancel), but `flushScanBatch` must
  use the pre-derivation `flushCtx`. On cancel we still persist what was already extracted; using the
  cancelled ctx makes the transaction fail immediately, exhaust its three retries and mark everything
  failed — so "cancel keeps the work" silently does nothing
- **Cancel no longer discards extracted results** (behavior change after v2.12.0): results produced
  before the workers exit are still imported, and `sealAll` flushes leftover directory groups whose
  pending count never reaches zero. Previously the whole batch was dropped, so two consecutive cancels
  threw away twenty minutes of work

<a id="directory-walk"></a>

## Never `os.Stat` every directory entry (songloft-org/songloft#447)

`scanDirWithCue` / `collectDirNames` determine entry types via `resolveEntry` (`scanner.go`):
`DirEntry.Type()` from `os.ReadDir` comes straight from `d_type` on Linux at zero extra syscall cost,
and only symlinks and unknown types fall back to `os.Stat` (following symlinks, preserving
"symlinked directories are still recursed, dangling entries are skipped").

- A per-entry `os.Stat` adds tens of thousands of syscalls on a large library. That is exactly why
  `GET /scan/dir-names` exceeded the client's 10s timeout → `r.Context()` was cancelled → the walk
  aborted mid-way → 500, and **every retry started over from scratch**
- That endpoint therefore has two more layers of protection: a 60s TTL cache in `CollectAllDirNames`
  (`dirNamesMu` doubles as a single-flight lock against the frontend's retry storm), and
  `context.WithoutCancel(r.Context())` on the handler side — detaching from client cancellation so the
  first walk finishes and populates the cache, making retries instant. The cache lives on `Scanner`,
  and `app.go` rebuilds the Scanner when `music_path` changes, so it invalidates naturally
- On filesystems without `d_type`, `ReadDir` fills the type in via its own lstat, which is equivalent
  to the old behavior — no regression

<a id="dsf"></a>

## DSF (DSD) scanning

- `dsf` is a default scan format; keep both fallback lists in `app.go` aligned with migration `0039_scan_support_dsf.sql`, which appends it to existing configs idempotently (case-insensitive).
- `pkg/tag` reads DSF metadata and cover art; the raw DSD sample rate may be 8 times the PCM-equivalent value reported by ffprobe. Existing metadata tests handle this difference.
- Playback uses the existing libmpv / ffmpeg path; browsers and DLNA devices without DSD support need an explicit transcoding parameter such as `?format=mp3`; the current Flutter Web `AudioFormatHelper` does not add one automatically for DSF. DSF is outside the tag-writing matrix; use the existing fallback handling when writes are unsupported.

<a id="sidecar-lyrics"></a>

## Sidecar lyrics (.lrc)

- **Matching rules** (`FindSidecarLyricFile`): `<base>.lrc` / `.LRC` / `.Lrc`, then `<full filename>.lrc` / `.LRC` / `.Lrc`.
  The base variant wins. **Do NOT** switch to `ReadDir` + `EqualFold` traversal (O(songs × dir entries) is unacceptable).
- **Empty files are treated as not found**: `st.Size()==0` or a same-named directory is skipped. Reason: prevents
  an empty lrc from setting `lyric_source=file` + `lyric=""` which blocks frontend requests and plugin fallback.
- **Encoding handling** (`ReadSidecarLyric`): UTF-16 LE/BE detected by BOM → decoded via `x/text/encoding/unicode`;
  otherwise `tag.FixEncoding` (GBK fix).
- **Three-level scan skip short-circuit** (`needsSidecarLyricImport`):
  ① `LyricSource ∈ {file, manual}` → false (no IO);
  ② directory not in `ScanResult.LyricDirs` → false (in-memory map);
  ③ lrc file actually exists for this song → true (at most 6 Stats).
  Convergence: once imported, `lyric_source=file` → next scan hits ① short-circuit.
- **Runtime priority chain** (`GetSongLyric` handler):
  sidecar .lrc > DB url > DB payload > lyric search plugin. `manual` is not overridden by sidecar
  (`SidecarLyricForSong` excludes it).
- **`SyncSidecarLyric` does not write back to audio tags**: the .lrc file IS the persistent copy; embedding
  it into tags would leave a stale copy if the user deletes the .lrc. Also `WriteSongTags` is a full
  rebuild mode that reads/writes cover binary — heavy for a GET request.
- **`shouldApplyScanLyric` guard**: `manual` is never overwritten; empty new lyric does not wipe existing
  DB lyric/remote URL. This is a behavior change (previously re-import could clear lyrics), documented in CHANGELOG.
- **Sidecar hit replaces plugin translations** (`tlyric`/`rlyric`/`lxlyric`) entirely — no "main lyric from
  file + translation from plugin" merging because misaligned timelines are worse than missing translation.

<a id="tag-writing"></a>

## Tag writing (pkg/tag)

- `tag.WriteTag(filePath, opts)` dispatches by file extension; all formats write atomically with a temp file + `os.Rename`
- Support matrix:

| Format | Text fields | Lyrics | Cover |
|------|---------|------|------|
| MP3 | ID3v2.3 text frames | USLT | APIC |
| FLAC | Vorbis Comment | LYRICS | PICTURE block |
| M4A/MP4/M4B/M4V/MOV/3GP | iTunes atoms (©nam, etc.) | ©lyr | covr |
| OGG(.ogg/.oga/.opus) | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
| APE | APEv2 text items | Lyrics | Cover Art (Front) (binary item) |
| WAV | RIFF LIST INFO | ICMT | **Not supported** (format limitation) |
| AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |
- Unsupported formats → return `ErrUnsupportedWrite`; the caller **must** degrade to a log entry and **must not** block the main flow

<a id="fingerprints"></a>

## Audio fingerprints (fingerprint — cost-control rules)

Fingerprints (ffmpeg chromaprint) serve only two purposes: the "Duplicate detection" settings page and
an **optional** parameter for plugin lyric/cover search. It is an on-demand feature, **not** a required
step of scanning. Read the rules below before touching this area — `songloft-org/songloft#323` is exactly
what happens when they are missing: "scan reports completed but the CPU stays pinned at 100% forever".

- **Auto-fingerprint after scan is off by default**: business endpoint `GET/PUT /api/v1/settings/scan-auto-fingerprint`
  with body `{enabled: bool}`, config key `scan_auto_fingerprint`, default `false`. Checked at the top of
  `runAutoFingerprint` (`song_service.go`), symmetric with `scan_auto_create_playlists`. While off, users
  trigger `POST /scan/fingerprints` manually from the duplicate detection page
- **Failures must be persisted**: `songs.fingerprint_attempted_at` (unix seconds, 0 = never attempted).
  `ListLocalWithoutFingerprint` filters on `fingerprint = '' AND fingerprint_attempted_at = 0`.
  **Never** just log on failure — without the marker, AutoScanner (default 3600s) re-queues the same batch of
  doomed long/audio-less files for a full ffmpeg decode on every round. `ClearAllFingerprints` resets the
  marker, so "Recompute all" is the only way to retry failed items
- **120-second sampling cap**: `ExtractFingerprint` passes `-t 120` (constant `fingerprintSampleSeconds`),
  which is also the AcoustID de-facto standard. **Do not** remove it — decoding a 30-minute audiobook in
  full inevitably times out on a weak NAS; measured on one file: 3.8s full decode vs 0.35s for 120s sampling.
  30s timeout plus `cmd.WaitDelay` keeps a stuck ffmpeg child from pinning a worker
- **CUE tracks sample by range**: a CUE track's `file_path` points at the whole-disc image, so
  `cue_start_seconds/cue_end_seconds` must be passed through `-ss`; otherwise every track under the same
  image gets an **identical** fingerprint and they all flag each other as duplicates
- **Concurrency adapts to CPU**: `fpWorkerCount()` = `clamp(GOMAXPROCS/4, 1, 4)`. **Do not** revert to a
  hard-coded 4 — Go's GOMAXPROCS is cgroup-aware, so a CPU-limited Docker container converges correctly
- **Cancellable**: `POST /api/v1/scan/fingerprints/cancel` → `FingerprintService.Cancel()`. The fingerprint
  task uses its own `context.Background()` and **cannot** reuse the scan's cancelCh — `scanProgressManager.Complete()`
  already did `close(cancel); cancel = nil`, after which `GetCancelChannel()` returns nil. Songs cancelled
  mid-flight are not marked as attempted, so the next run resumes them
- **Dedup has a duration guard**: within one fingerprint, `ListDuplicateGroups` further clusters by
  `fingerprint_duration` (full-file length) using a 30-second tolerance between **adjacent** entries (0 = unknown never splits), because only the first 120 seconds are
  sampled and "audiobooks with a shared intro" can collide
- Migration `0029` clears legacy full-length fingerprints once (they are not comparable with 120s sampling);
  after upgrading they need to be recomputed
- `IsChromaprintAvailable` honors the `ffmpeg_path` config injected via `SetFingerprintFFmpegPath` instead of
  only searching PATH

<a id="play-history"></a>

## Play history (play_history -- per playback context)

Each "playback context" independently remembers the songs recently played in it (songloft-org/songloft#333).
A context is `(context_type, context_key)`: playlists are `("playlist", "<id>")`, facet dimensions are
`("artist", "Jay Chou")` and the like; `tag` is also supported. Valid types come from `IsValidPlayContextType`,
combining `models` constants and `database.IsSongFacetField` — **do not duplicate that enum**.
Read all of the following before touching this area.

- **Ordinals are unreliable, so the read endpoint does not return "which track number"**: facet lists go through
  `GET /songs?artist=X` with a default ordering of `added_at DESC`, but `added_at` is a second-precision
  `DATETIME` and a bulk scan inserts sequentially in one transaction → hundreds or thousands of songs end up with
  **identical** `added_at`; `applyOrder` (`filters.go`) emits a single-column `ORDER BY` with **no `id` tie-breaker**.
  Computing "the Nth song" from that is mathematically indeterminate and would start the wrong song. **Do not**
  add a tie-breaker to `applyOrder` for this -- that changes existing pagination behavior and is a separate issue
- **`absIndex` matching the pagination offset relies on the SQLite planner**: `/songs/ids` (one full sort) and
  `/songs?limit&offset` (may use a bounded sorter) are two different queries, and their relative order among tied
  `added_at` values holds only because the planner happens to agree (verified consistent at 350 and 30000 songs).
  Tightening this would require adding `, id DESC` to `applyOrder`'s default ordering -- which affects every `/songs`
  pagination caller and is a separate issue. Likewise `ListSongIDsOrdered` and `GetPlaylistSongsPaginated` both order
  by `position ASC` with no secondary key; they only diverge if a playlist ends up with duplicate positions
  (e.g. an interrupted concurrent reorder)
- **Client playback = play the first song immediately, fill circularly in the background**: the history entry carries
  the full `Song`, so it becomes the queue and starts playing with **zero requests**; the background then fetches the
  ordered ID list (`GET /playlists/{id}/song-ids` for playlists, `GET /songs/ids` for facets), locates it via
  `indexOf`, and appends "everything after the target" followed by the wrapped "beginning…up to the target".
  `currentIndex` stays 0 throughout, so random mode's `_playedIndices` / `_preSelectedNextIndex` are untouched;
  playlists and all 7 facets share one code path
- **Only `type=play` is persisted**: writing hooks into the `context_type`/`context_key` parameters of the existing
  reporting endpoint `POST /songs/{id}/played` (no second reporting channel, zero extra client requests). `finish`
  is a duplicate write for the same song; **`skip` is especially dangerous** -- it reports the **previous** song, and
  by then the client's context may have switched to a new playlist, filing the previous song under the wrong context.
  The frontend omits the context for skip/finish and the backend only accepts `play` -- a double safeguard. A write
  failure only produces `slog.Warn`; **the status code is always 204**
- **50-entry cap, upsert + trim in one transaction**: deduplication relies on `UNIQUE(context_type, context_key, song_id)`
  plus `ON CONFLICT DO UPDATE`, not application-level lookups. The trim uses
  `id NOT IN (... ORDER BY played_at DESC, id DESC LIMIT ?)` -- `played_at` collides at second precision, so `id DESC`
  is the deterministic tie-breaker. `MaxPlayHistoryPerContext` is a Go constant; **do not** turn it into a config item
- **`context_key` is TEXT, so no foreign key to playlists is possible**: playlist deletion relies on
  `PlaylistService.clearPlayHistory` clearing it explicitly. **Batch deletion must only clear playlists that were
  actually removed** -- the repository layer skips `built_in`, so blindly iterating the input ids would also wipe the
  history of "Favorites" (hit during implementation; now covered by `TestPlaylistDeleteClearsPlayHistory`)
- **`sourcePlaylistId` must keep its signature**: it is a public JS-plugin contract
  (`plugin_host_dispatch.dart` exports `source_playlist_id`) and has 5 "now playing" highlight consumers. When the
  frontend generalized to `PlaybackContext` it was demoted to a **derived getter**, so those consumers needed zero
  changes. Prefs reads fall back to the legacy `player_source_playlist_id` and writes maintain both keys, keeping
  Android hot-update rollback safe

<a id="hls-proxy"></a>

## HLS radio proxy mode (/settings/hls-proxy)

- Business toggle endpoint: `GET/PUT /api/v1/settings/hls-proxy` with body `{enabled: bool}`, default `false`
  - `false`: the radio `.m3u8` is 302-redirected straight to the player, which pulls the origin itself. Zero overhead but subject to origin anti-hotlinking/CORS restrictions
  - `true`: the server fetches and rewrites the m3u8 and proxies all segments/keys/init segments. **All segments consume this machine's bandwidth**, so mind the traffic cost
- When to switch: turn on the proxy when origin Referer/UA anti-hotlinking causes playback failures, or when CORS blocks in Web embedded mode
- Reverse-proxy endpoints: `/api/v1/songs/{id}/hls/playlist?u=<base64url>` and `/api/v1/songs/{id}/hls/segment?u=<base64url>`
- HLS radio song.url is forced to carry a `.m3u8` suffix (`/api/v1/songs/{id}/play.m3u8`): ExoPlayer/AVPlayer pick the MediaSource by URL suffix, and without a suffix it falls into ProgressiveMediaSource, making live streams unplayable
- Rewrite rules: classic HLS + the full LL-HLS set (PART/PRELOAD-HINT/RENDITION-REPORT) + `EXT-X-DATERANGE:X-ASSET-URI` (HLS Interstitials single URI). `X-ASSET-LIST` (JSON sub-proxy) is not yet implemented and is passed through verbatim when encountered
- Security: each endpoint entry performs a "same-origin check (scheme+host+port must exactly equal song.URL)" as the first line of defense, with `services.IsHostnameAllowed` as an SSRF backstop. **Non-same-origin URLs are left unchanged and not rewritten**, to avoid becoming an open proxy
- Player cross-origin: all rewritten URLs are relative paths (`playlist?u=...` / `segment?u=...`), sidestepping BASE_PATH sub-path deployment issues
- Upstream 4xx/5xx are passed through to the player; the playlist body is capped at 1 MB; the first line must be `#EXTM3U`

<a id="http-proxy"></a>

## Generic HTTP Proxy (/settings/http-proxy)

- Business endpoint: `GET/PUT /api/v1/settings/http-proxy` with body `{proxy: string}`, default `""` (direct connection)
- Only outbound requests using proxy-aware clients use this proxy; do not assume arbitrary `http.Client` instances or plugin `fetch` calls are automatically integrated.
- Typical value: `http://192.168.1.1:7890` (supports HTTP/HTTPS/SOCKS5 proxies)
- Loopback addresses (`localhost`/`127.0.0.1`/`::1`) automatically bypass the proxy, avoiding interference with internal requests
- **Coexists** with GitHub mirror acceleration (`github_proxy` URL-prefix concatenation): the mirror prefix is concatenated first, then forwarded through the HTTP Proxy
- Implementation: `globalProxy` (private `proxyConfig`) in `internal/httputil/proxy.go` is managed by `SetGlobalProxy` / `GetGlobalProxy`; `NewClient`, `NewStreamingClient`, and `NewDownloadClient` use shared transports suited to ordinary requests, streams, and downloads respectively.
- The saved proxy address is loaded from the config table at startup (`app.go`); a PUT takes effect immediately without a restart
- Examples include `jsplugin/registry.go`, `jsplugin/package.go`, `services/upgrade_service.go`, `handlers/jsplugin_registry.go`, and `handlers/hls.go`; use the `httputil.New*Client` call sites for the complete scope.
- ffmpeg remote-pull transcode paths (`services/radio_transcode.go` radio transcode, `services/url_transcode.go` transcode proxy) are also integrated: passed to ffmpeg via `-http_proxy`. **HTTP/HTTPS proxies only**; SOCKS5 is not supported (ffmpeg `-http_proxy` limitation)

<a id="private-allowlist"></a>

## Private network proxy allowlist (/settings/proxy-private-allowlist)

- Background: the generic resource proxy `GET /api/v1/proxy?url=` blocks all internal / loopback / link-local addresses via `services.IsHostnameAllowed` by default (anti-SSRF), which rejects the "public Songloft proxying a WebDAV reachable only on the LAN" scenario (songloft-org/songloft#313)
- Business endpoint: `GET/PUT /api/v1/settings/proxy-private-allowlist` with body `{allowlist: []string}`, default `[]` (empty = keep blocking everything, behavior unchanged)
- Each entry is a single IP (`192.168.1.100`) or CIDR range (`192.168.1.0/24`); PUT validates via `services.ParseAllowlist`, returning 400 on invalid entries
- Decision: `services.IsHostnameAllowedWithAllowlist(hostname, allowlist)` — public addresses always pass, private IPs pass only when covered by an allowlist range; `localhost`/`.local`/empty hostnames are still string-blocked (the allowlist matches by IP/CIDR only)
- **Affects both `/proxy` and `/proxy/transcode`**; HLS reverse proxy (`hls.go`) still calls `IsHostnameAllowed(hostname)` without an allowlist, leaving its private-network restriction unchanged.
- Implementation: `internal/services/whitelist.go` (`ParseAllowlist` / `IsHostnameAllowedWithAllowlist`) + `internal/handlers/proxy.go` (`ProxyHandler` holds a `*ConfigService`, config key `proxy_private_allowlist`)

<a id="url-transcode"></a>

## Audio transcode proxy (/proxy/transcode)

- Business endpoint: `GET /api/v1/proxy/transcode?url=&format=mp3[&bitrate=&duration=&user_agent=&referer=]` (`@Security BearerAuth`, token required; token must be the first query param to dodge speaker-firmware replacing `&` with space)
- The server fetches a remote audio URL and transcodes it to mp3 (CBR 320k) on the fly, streamed back. Clients use this endpoint to obtain an MP3 stream when the original remote audio format is unsupported (such as webm/opus, songloft-org/songloft#394)
- No disk persistence, no DB entry, no cache (consistent with no-import semantics; youtube direct links are re-signed each search, so caching yields zero benefit — replay benefit goes through the import path)
- SSRF protection: reuses `/settings/proxy-private-allowlist`, the same `IsHostnameAllowedWithAllowlist` check as the generic `/proxy`
- Output is mp3 CBR + `-write_xing 0` + `-map 0:a:0 -vn`: the pipe is non-seekable, the speaker estimates duration from byte count, CBR avoids premature track-switching (same trade-off as the seek stream)
- Concurrency is shared with seek/normalize streams via `seekStreamSem` (cap=4); returns 503 when full, no queueing
- ffmpeg pulls the remote URL directly (`-i <url>`), reusing the global http-proxy via `-http_proxy` (HTTP/HTTPS only)
- Implementation: `internal/services/url_transcode.go` (`StreamTranscodedURL`) + `internal/handlers/proxy.go` (`Transcode`; `ProxyHandler` holds a `*CacheService`)

<a id="hls-transcode"></a>

## Remote video HLS transcode sessions (#541)

- Entry: `POST /api/v1/proxy/transcode/hls/sessions`, JSON `{url, audio_bitrate?, user_agent?, referer?}`. `url` accepts an HTTP/HTTPS TS, MP4, or ordinary HLS source, without URL user credentials. Bitrate is 64/96/128/192/256 kbps, default 128. Requires the configured ffmpeg and ffprobe beside it or in PATH.
- Tool builds must enable both the **HLS input demuxer** and the **crypto input protocol**; an HLS output muxer does not enable reading m3u8. `ffprobe -hide_banner -demuxers` must list `hls`, and the Input section of `ffprobe -hide_banner -protocols` must list `crypto` for AES-128 HLS. Docker tools come from `tools/ffmpeg-builder`, whose artifact tests cover TS, plain HLS, and encrypted HLS. Songloft's binary updater does not replace container tools: after fixing them, rebuild the Songloft image, pull it, and recreate the container.
- The first version copies the first **progressive H.264** video track and encodes the first audio track as **AAC-LC / stereo / 48kHz**, delivering MPEG-TS HLS. HEVC, interlaced video, and missing video/audio tracks return 422; video is not automatically re-encoded. Source H.264 profile/level, resolution, and keyframes still need to suit the actual player; this does not guarantee Safari playback of every IPTV source.
- Creation waits up to 30 seconds for a complete first segment; success returns 201 `{id, playlist_url, idle_timeout_seconds:120}` with `Location`. `playlist_url` includes deployment `BASE_PATH` and omits the token. For Safari `<video>`, append `?access_token=<URL-encoded access token>`; the server propagates it to relative segment references. Expired access tokens still return 401; callers must refresh the token and update the playback URL, keeping the same session when the ClientID is unchanged. Players using Authorization must send that header on every child request.
- `GET/HEAD /api/v1/proxy/transcode/hls/sessions/{id}/playlist.m3u8` reads an existing session; `GET/HEAD .../{id}/segments/{name}` supports Range; `DELETE .../{id}` waits for process exit and returns 204. Every endpoint requires JWT, and sessions belong to their creator's `ClientID`. Other clients and expired sessions receive 404. A random session ID is not an authentication credential.
- Target segment length is 4 seconds, with actual boundaries depending on source keyframes. The playlist retains 6 segments plus 3 older segments for retries and is updated atomically. There is no program archive; pauses exceeding the idle limit require a new session. Playlists use `no-store`, segments use `private, no-store`.
- Sessions are not stored in the DB. Rolling segments use an independent system temporary directory and do not participate in music-cache settings, cleanup, or statistics. Input is read at real-time speed; storage is checked every 200ms and stops processing above 128 MiB, with possible brief overshoot between checks. A running process is also stopped after 30 seconds without a new playlist; subsequent reads return 502. Naturally completed sources remain playable until idle reclamation.
- The 120-second idle period starts when initial playback is ready and is renewed by successful playlist/segment requests. Deletion and app shutdown cancel and wait for the process. Already-open segments hold a file lease and are cleaned up after transfer closure. Natural completion/failure releases the running slot; at most 16 sessions are retained. Running probes/transcodes share 4 slots with seek/normalize/MP3 URL transcodes; a full pool returns 503 without queuing. Forced process termination may leave files subject to the system temporary-directory cleanup policy.
- An independent loopback HTTP gateway handles pulling: every upstream HLS URI (child playlists, segments, keys, init resources) and redirect is checked for HTTP/HTTPS and the private-network proxy allowlist. Direct dialing revalidates and pins resolved IPs. FFmpeg input protocols/containers are restricted, preventing direct local-file, UDP, or unrevised HTTPS access. Playlist variable substitution, content steering, and similar unsupported extensions return 422 in this first version.
- Reuses the global HTTP/HTTPS proxy, snapshotted at session creation; SOCKS5 returns 503. The administrator-configured proxy is a trusted outbound facility; destinations are still checked locally, while remote DNS resolution depends on the proxy. Existing radio HLS proxy toggles and same-origin restrictions are unaffected, and the MP3 `/proxy/transcode` contract is retained.
- Implementation: `internal/services/hls_transcode.go`, `hls_pull_gateway.go`, and `internal/handlers/hls_transcode.go`. Generated Swagger is the API contract.

<a id="music-cache"></a>

## Music caching (cache_service)

- When playing a remote song, the upstream audio is streamed and proxied to the client (non-blocking) while being written to the cache asynchronously in the background; subsequent playback hits the cache and is served directly from local
- Streaming proxy `ServeRemoteResourceWithCache`: on 200 OK, a TeeReader both proxies and writes a temp file; on 206 Partial, it proxies normally and triggers an asynchronous full download
- The cache path is persisted in the `songs.cache_path` field (DB level); lookups prefer `cache_path`, falling back to the old hash-bucketed directory format
- The cache directory defaults to `{data_dir}/music_cache/`, and can be customized to an absolute path via the `cache_dir` field of `PUT /api/v1/cache-manage/config`
- At startup the custom directory is read from the `music_cache_config` config; switching directories at runtime automatically rebuilds the LRU index and does not migrate old files
- LRU eviction: when exceeding `max_size` (default 1GB), evict by last access time; `max_size=0` means unlimited
- **Transcode on cache** (`transcode_format` / `transcode_quality`, `PUT /api/v1/cache-manage/config`, songloft-org/songloft#300): defaults to `""`, storing the upstream raw container (YouTube .mkv/.webm, Bilibili .mov); set to `mp3/m4a/ogg/flac/wav` to transcode network songs to that format when caching (supports audio clients that cannot play MKV). Performed by `EnsureCachedFormat` at the two playback-side cache producers — `FinalizeCache` (streaming playback + 206 async full download) and the prefetch prewarm in `prepareSongPlayback` (after `Get`); reuses `runFFmpeg`, deletes the raw file after transcoding and points `cache_path` to the new format. **Gracefully degrades** to the original format when ffmpeg is missing or transcoding fails. Does **not** affect `songs.download`'s explicit format handling (the download path reuses `Get` but carries its own `opts.Format`, so the transcode logic is attached only on the playback side and never touches `moveToCache`/`Get`). **Tradeoff**: `ffmpeg -vn` drops the video track, so once enabled, `media=video` casting of an `is_video` remote song yields audio only (expected, serving the YouTube MKV→mp3 primary need); `EnsureCachedFormat` carries a `cacheTranscodeTimeout` (15min) fallback to prevent a stuck ffmpeg from permanently holding `transcodeSem`
- `POST /api/v1/cache-manage/validate-dir` can validate a directory in advance (auto-create + writability check + return disk space)
- Inflight dedup: concurrent requests for the same `song.ID` download only once; when the first request is `ctx.Canceled`, later waiters retry automatically

<a id="normalization"></a>

## Volume normalization (`/songs/{id}/play?normalize=1`)

EBU R128 loudness normalization (songloft-org/songloft#315), removing loudness gaps between sources.
Clients request normalization with the `normalize=1` parameter in the playback URL.

- On-disk transcoding and live streams share `CacheService.normalizeLoudnessFilter()`, reading the target from `volume_normalize_loudness` (default -16 LUFS, allowed range -40 to -5). It remains **single-pass** dynamic `loudnorm=I=<target>:LRA=11:TP=-1.5`; the `loudnormFilter` constant is only a default-value test baseline. **Do not** inline a fixed filter: both paths must produce the same loudness.
- `transcodedFileName` and the inflight key use `normalizeLoudnessTag()`, including the target loudness (such as `norm-16` / `norm-14`); artifacts for different loudness targets or without normalization are **not interchangeable**.
- **Streams while transcoding when the artifact isn't ready** (`tryLiveTranscodeStream` →
  `StreamSeekedMP3`, `internal/handlers/music.go`): `GetOrTranscode` is synchronous, so
  time-to-first-byte equals the wall-clock time of transcoding the whole track. Whole-track loudnorm
  takes 20+ seconds; plain `?format=` / `?quality=` transcoding takes 5+ seconds before any sound
  on a weak CPU (N5095) when converting lossless to
  mp3 (songloft-org/songloft#442). Piping directly took time-to-first-byte from 10.03s to 0.088s in
  the normalize case. **Both `format` and `quality` are covered** (`SeekStreamOptions.Bitrate` lets
  the pipe faithfully express `?quality=` instead of being pinned to 320k); only applies when the
  target format is mp3 and it's not `media=video` / track extraction / CUE / HEAD, all other cases
  keep the original blocking path
- **`ForceTranscode` must be passed explicitly, never inferred from "the source extension isn't
  mp3"** (`SeekStreamOptions`): a plain `?format=mp3` neither seeks nor normalizes nor changes speed,
  so `StreamSeekedMP3`'s guard would judge it as "nothing to do" and refuse; and a fake mp3
  (extension `.mp3` but WebM/Opus content, songloft-org/songloft#300) is precisely a case where the
  extension *is* mp3 yet transcoding is mandatory — inferring from the extension would drop it into
  the `-c:a copy` fast path → the mp3 muxer rejects Opus → zero output → fallback, burning an ffmpeg
  process for no latency improvement at all
- **The generalized guard must be self-contained**: `tryLiveTranscodeStream` computes
  `normalize || bitrate > 0 || NeedsTranscodeForServe(...)` itself rather than relying on the `if`
  conditions at its two call sites (`serveLocal` / `serveCachedFile`) — getting it wrong burns a
  libmp3lame process on every single playback
- **Pipe streams must support Range, or seeking silently hangs** (`cbrRangePlan`,
  songloft-org/songloft#442): a chunked response with no `Content-Length` is judged un-seekable by
  browsers. Measured in Docker headless Chrome: after dragging the progress bar the browser issues
  **no new request at all**, `readyState` drops from 4 to 1, buffered stops growing, and `error` stays
  null — **playback silently hangs and the track must be re-started**. The progress bar's total length
  comes from the DB's `song.duration`, so nothing in the UI hints that this stream can't be seeked.
  Adding just `Accept-Ranges: bytes` does not help (measured: still one request, still hangs) — the
  browser needs the **total byte count** to map "second N" onto a byte offset. **Why it's computable**:
  the pipe always emits CBR (`-b:a`), so bytes and time are linear; combined with the known
  `song.duration` that yields both the total length and the seconds corresponding to any offset, and
  "start the stream at second N" is exactly `StreamSeekedMP3`'s `-ss` input seek. Range requests are
  therefore **genuinely** satisfied rather than papered over by "ignore Range, resend from the start"
  (which would desync the progress bar from the actual audio)
- **Use `planCBRRange` as the source of truth for Range eligibility**: a known track duration no greater
  than 24 hours, no speed change, and positive duration remaining after seek. `?seek=N` **can be combined
  with Range**: the total size is estimated from the remaining duration, Range addresses bytes within
  this new stream, and ffmpeg starts at `N + Range offset / bytes per second` (songloft-org/songloft#488).
  Do not restore the old "seek disables Range" restriction; speakers need Content-Length and 206 for
  segmented buffering. Copying VBR MP3 still introduces estimation error; `exactLengthWriter` guarantees
  response byte count, not exact audio timing or frame alignment. Otherwise fall back to chunked with no length
- **Promising a `Content-Length` means writing exactly that many bytes** (`exactLengthWriter`):
  ffmpeg's actual output never exactly equals "bitrate × duration" — LAME's CBR uses frame padding to
  track the target bitrate, the last frame may be partial, and `-ss` only aligns to frame boundaries,
  leaving an error of a few frames. Writing less makes the client treat it as a prematurely closed
  connection; writing more violates the HTTP/1.1 length contract. Hence truncate the overflow and pad
  the shortfall with zeros. **On overflow it must return an error to break `io.Copy`**
  (`errExactLengthReached`) rather than silently discarding: a closed range (`bytes=N-M`) wants only a
  slice while ffmpeg runs to the end of the source, so discarding silently lets a `bytes=0-100`
  request burn a full-track transcode
- **A 206's `WriteHeader` must be deferred until the first byte is actually written**
  (`deferredStatusWriter`): once WriteHeader is called the response is committed and can no longer be
  downgraded, while `StreamSeekedMP3`'s contract is "only write after `Peek(1)` confirms ffmpeg really
  produced output" — "first Write" is precisely that safe moment. For the same reason the fallback path
  must Del `Content-Length` / `Content-Range` / `Accept-Ranges`; a leftover length would conflict with
  the one `http.ServeFile` computes itself
- **Wrapper writers must forward `Flush()`**: the services side is `io.Copy(&flushingWriter{w: w}, ...)`
  and `flushingWriter` decides whether to flush via the **type assertion** `w.(http.Flusher)`. Without
  that method on `exactLengthWriter` / `deferredStatusWriter` the assertion is blocked, bytes pile up
  in the buffer instead of going out — and **the time-to-first-byte win evaporates**
- **The live stream deliberately does not kick off a background transcode** to fill the cache:
  running two ffmpeg processes over the same track doubles CPU on a weak NAS while the user is
  waiting for sound right now. Cache artifacts are produced by `?prefetch=1` (carrying the same
  format/quality/normalize)
- **`seekStreamMaxConcurrent` was *not* raised (still 4) when #442 was wired in**: plain transcoded
  playback happens an order of magnitude more often than seek/normalize so the slots fill up more
  easily, but #442's reported environment is exactly a weak CPU — widening concurrency hurts it,
  while falling back to the blocking path when full is itself lossless (just the pre-fix behavior)
- **Prewarming must carry normalize**: `prepareSongPlayback`'s `normalize` parameter must not be
  dropped, and the short-circuit check needs `!normalize` — for an mp3 source with `format=mp3`,
  `NeedsTranscodeForServe` is false, so without it prewarming returns immediately and does nothing
  (not a single `prefetch ready` line appears in the log)

<a id="seek-stream"></a>

## Server-side seek stream (`/songs/{id}/play?seek=<seconds>`)

Expresses a resume position for URL-push clients. A `seek=N` request produces a new stream that
**begins at second N**. Range expresses byte offsets within that stream; the two have independent
semantics. Implemented in `internal/services/seek_stream.go`, structurally identical
to `radio_transcode.go` (`Peek(1)` confirms real output before committing the response,
otherwise a sentinel error lets the handler degrade losslessly to serving the whole file).

- **Always outputs MP3**: mp3 sources use `-c:a copy` (input seek; measured 0.14s for a whole
  track, near-zero CPU), everything else `libmp3lame`. **Don't** grow this into a multi-format
  matrix — browsers have Range; this parameter only serves push-style clients
- **Re-encoding must be CBR (`-b:a 320k`)**: a pipe isn't seekable, so the Xing frame can't be
  rewritten (hence the explicit `-write_xing 0`) and clients can only estimate duration from
  "first-frame bitrate × byte count". **Don't** switch to `-q:a 0` like elsewhere in the project:
  a measured 105.4s stream gets estimated as 97.6s, and a speaker may declare it finished early
- **`-map 0:a:0` must stay**: the mp3 muxer accepts a single audio stream; a dual-track `.mka`
  (#298) without explicit track selection fails outright, showing up as a silent fallback to
  "play from the start" with a single warn line — extremely hard to attribute
- **Only applies to local songs and already-cached remote songs**: on a cache miss, obtaining a
  local file requires synchronously downloading the whole track, which would stall the resume
  keypress for an entire download. Radio (live, no position), `media=video` (`-vn` drops the
  picture) and HEAD all ignore it
- **seek must be clamped to at least 3s before the end** (`parseSeekSeconds`; clients must honor
  the same boundary): past the end of file → ffmpeg produces nothing → the
  fallback kicks in → **the whole track replays from the start**, which is worse than ignoring seek
- **Doesn't hold `transcodeSem`** (the process lives for the remaining track duration and would
  starve other transcodes); uses a separate `cap=4` semaphore in the same file that degrades
  immediately instead of queueing. `exec.CommandContext` also carries a "remaining duration +
  5min" hard timeout to reap orphaned processes. That semaphore is **shared** with the live
  normalization stream above, the speed stream, and the plain transcode stream (#442) —
  `StreamSeekedMP3` is the same function — so 4 is the total across all of them.
  How long a slot is held depends on how fast the client drains the stream, not on track duration —
  speakers buffer greedily; a measured 4-minute track streamed out and exited in 10 seconds.
  The slot is held by `io.Copy`, and **cancelling the ctx does not interrupt it** (io.Copy only
  watches for read-side EOF / write-side error), so registering with playActivity can only kill
  the ffmpeg process to save CPU — it can't release the slot early
- **When the remaining duration is unknown (`songs.duration == 0`) the hard timeout must not fall
  back to 5 minutes** (see `seekStreamUnknownDurationTimeout`): for a whole-track stream that's a
  hard ceiling rather than a grace period, so a client reading in real time gets killed at the
  5-minute mark while the response closes normally and the client thinks playback finished.
  `duration == 0` is a normal occurrence (remote song metadata not yet refreshed; a local file
  where neither the tag nor ffprobe yields a duration)
- With a known duration, no speed change, and the other `planCBRRange` prerequisites met, responses
  include `Content-Length` / `Accept-Ranges: bytes` and can return 206 for partial requests. Only other
  cases use chunked responses with no length. Both use `Cache-Control: no-store`

<a id="song-download"></a>

## Song persistence (song_downloader — plugin infrastructure)

- **Positioning**: this is a plugin infrastructure capability, not a user-facing feature of the main program. The main program provides the `songs.download` Bridge API, allowing plugins to persist remote songs from the user's own network storage (NAS/WebDAV/Subsonic, etc.) to the server's local `music_path`, converting them to the `local` type. **This capability is only for music the user legally owns and must not be used to download copyright-protected content from third-party commercial music platforms**
- Core service `SongDownloader.Download`: acquire the audio (copy directly on cache hit, otherwise download synchronously) → optional transcode → path-template rendering → optional metadata embedding (all supported formats) → update DB (type=local)
- **Download transcode** (`SongDownloadOptions.Format` / `Quality`): a plugin may pass `format` (mp3/m4a/ogg/flac/wav) plus optional `quality` (128/192/320), reusing the playback path's `CacheService.GetOrTranscode` to transcode into a standard audio container at download time. Typical use: sources like Bilibili produce a `.mov` video container that can't be scraped for lyrics, so transcoding to mp3 lands a scrapeable file. Empty `format` = no transcode, keep the source format; transcoding depends on ffmpeg and degrades gracefully when it's missing/fails (warn only, keep the source format, never block the download)
- **URL lyrics auto-fetch**: when `embed_metadata=true` and `lyric_source=url`, `LyricFetcher` fetches the lyrics → the primary lyrics are written to the file tag → the full payload (including translation/romanization) is cached to the DB → `lyric_source` is updated to `embedded`. A fetch failure only warns and does not block persistence
- Exposed to JS plugins via the Bridge API `songs.download`, with permission mapped to `PermSongsWrite`
- The official plugin `songloft-plugin-downloader` (separate repo `songloft-org/songloft-plugin-downloader`) is built on this API and provides the ability to download remote songs from the user's own network storage to local

<a id="file-move"></a>

## File moving: the cross-device rename trap

- `os.Rename` returns a `syscall.EXDEV` (cross-device link) error when src and dst are not on the same filesystem (mount point)
- Typical scenario: `os.CreateTemp("")` creates the temp file in the system `/tmp` (tmpfs), while the target cache/music directory is mounted on a separate disk or a Docker volume
- **Use** `fileutil.MoveFile(src, dst)` (`internal/fileutil/file_move.go`) instead of bare cross-device `os.Rename`: it tries rename first and falls back to copy + remove on EXDEV; Windows also recognizes `ERROR_NOT_SAME_DEVICE`.
- `pkg/tag`'s atomic write is unaffected: it uses `os.CreateTemp(dir, ...)` to create the temp file in the **same directory** as the source file, so the rename is always same-device
- New download/cache logic that needs to "write a temp file first, then move it to the target location" **must** use `fileutil.MoveFile` and **must not** use a bare `os.Rename`

<a id="config-endpoints"></a>

## The three configuration endpoint styles (2026-06 decision)

The project has three kinds of configuration endpoints. **User-visible feature toggles always go through business endpoints**; the generic KV store is only an admin entry point:

- **`/api/v1/settings/<kebab-case-name>` — default home for standalone config** (e.g. `hls-proxy`, `music-path`, `http-proxy`, `library-browse`, `proxy-private-allowlist`). Data is **strongly typed JSON** (e.g. `{enabled: bool}` or an aggregate object), not `{value: string}`. Defaults live in the handler (GET returns the business default when missing; PUT writes directly, so the frontend need not POST-create first). Side effects run inside PUT (e.g. a `music_path` PUT asynchronously rebuilds the Scanner). The handler also holds `*services.ConfigService`. Naming pattern: `Is<Name>Enabled/Set<Name>Enabled` business methods + `Get<Name>Setting/Update<Name>Setting` HTTP handlers.
- **`/api/v1/<module>/config` — in-module config**: config strongly coupled to the module's action endpoints stays under the module prefix instead of moving to `/settings/`. Existing example: `/cache-manage/config` (shares `CacheService` with `stats/clean`). Decision rule: standalone or cross-module → `/settings/<name>`; strongly in-module → `/<module>/config` or `/<module>/<sub-name>`.
- **`/api/v1/configs/{key}` — generic KV (admin editor only)**: only for generic config editors such as `config_manager.dart`. New business features **must not call it directly**: the generic PUT returns 404 when the key does not exist and has no strong typing, side effects, or defaults. After a business wrapper exists, the generic endpoint may still modify the same key, but the side effect must also be hooked into `configHandler.SetOnConfigChanged` (see `musicPathChanged` in `routers.go`).

Client conventions: `SettingsApi` (`clients/player/lib/features/settings/data/settings_api.dart`) wraps all `/settings/*` calls and business-feature Providers always go through it; `ConfigApi` is only used by `config_manager.dart` and admin UIs such as "list all configs".

History: this convention was introduced in 2026-06 — `hls_proxy_enabled` was not preloaded, so PUT `/configs/{key}` returned 404, revealing three coexisting styles (`/configs`, `/settings/*`, `/cache-manage/config`). Business endpoints were chosen as the single source for user-visible entry points, with the generic KV degraded to an admin back door.
