# Shared custom tags and unique playlist names

Status: implemented, locally verified and reviewed; the user authorized commit, push and Issue reply. 2026-11-11. Related: [songloft-org/songloft#562](https://github.com/songloft-org/songloft/issues/562) and [songloft-org/songloft#558](https://github.com/songloft-org/songloft/issues/558). [中文](../issue-562-design.md).

## 1. Decisions

- Songs and playlists share existing custom tag definitions. Songs retain `song_tag_links`; playlists reference tags through existing `playlists.labels`. Do not add `playlist_tag_links`.
- Keep globally unique playlist names, the database unique index, and repository conflict checks. Do not scope uniqueness by tags or add a display-name field.
- Use custom tags to classify, filter, and display sources. Automatic sync first tries the remote name; on conflict, use “original name-source”, adding ` (2)`, ` (3)` only if that also conflicts, instead of prefixing every name with its source.
- Persist and reuse allocated names alongside existing playlist ID mappings. Neither names nor tags establish sync ownership.
- Direct creation, manual renaming, and explicitly configured target names retain conflict errors rather than silently changing the requested name.

## 2. Pre-change implementation and constraints

Inspected revisions: main `22cd87edf308b53ad6301e0b8a418bd8a6e784a2`; Lynx `2a18bb57819886a2c9b2c0476bdf51a127f09ae1`; Flutter `59bc9c7def13e73ced81df045cf77e3ce32f0426`; Navidrome `04d87b0d79c880d1e648d056375a680e97d78802`; MIoT `1379528daab3841e7e3ca0edb05d14046ff14a17`.

| Current behavior | Evidence | Design implication |
| --- | --- | --- |
| Custom definitions use `song_tags`; songs use `song_tag_links` | `internal/models/song_tag.go`, migration `0036_song_tags.sql` | Reuse definitions and tag IDs |
| Playlists have a JSON string array, `labels`, with exact membership filtering in the repository | `internal/models/models.go`, `internal/database/playlist_repository.go` | Add references and API wiring, without a new link table |
| Playlist names have repository checks and a database unique index | `PlaylistRepository.Create/Update`, migration `0001_init.sql` | Keep both constraints; actual writes decide concurrent conflicts |
| Backup reuses playlists by name; automatic scanning matches groups by name | `BackupService.findOrCreatePlaylist`, `PlaylistRepository.AutoCreate` | Preserve existing name assumptions and merge rules |
| Navidrome uses connection, remote ID, playlist ID mappings, and description markers | Navidrome `src/sync.ts` | Rename existing IDs; never claim a playlist merely because its name matches |
| WebDAV already checks occupied names and retries conflicts | WebDAV `src/sync-task.ts` | Reference the approach without rewriting other plugins |

References: [playlist repository](https://github.com/songloft-org/songloft/blob/22cd87edf308b53ad6301e0b8a418bd8a6e784a2/internal/database/playlist_repository.go), [Navidrome sync](https://github.com/songloft-org/songloft-plugin-navidrome/blob/04d87b0d79c880d1e648d056375a680e97d78802/src/sync.ts).

## 3. Shared tags

### 3.1 Storage and writes

Keep `song_tags` and its existing tag-name uniqueness rules. Use “Custom tags” throughout the UI. If tag 12 is “Navidrome”, a playlist stores:

```json
{
  "id": 42,
  "name": "收藏-Navidrome",
  "labels": ["hidden", "tag:12"]
}
```

`tag:<positive decimal integer ID>` references a custom tag. Existing `hidden`, `built_in`, and `auto_created` markers remain owned by their respective features. Public tag APIs accept `tag_ids`, updating only `tag:*` entries while preserving other markers; they cannot alter system identity.

Validate all IDs and deduplicate before writing; any invalid ID fails the whole request. Preserve bindings when old clients omit tag fields. Both songs and playlists resolve names and colors from the same definition. Playlist bindings do not implicitly tag member songs, and song bindings do not alter containing playlists.

### 3.2 Deletion and transactions

Clear playlist references and delete a tag definition within one `RunInTx + UnitOfWork`; existing foreign keys cascade song-link deletion. Add the tag repository to UnitOfWork during implementation. Existence checks and binding writes must also serialize with deletion to prevent dangling references.

Tag IDs currently use plain `INTEGER PRIMARY KEY` and may be reused after deletion. Leave no stale references; plugins resolve configured tag names each sync rather than trusting cached IDs indefinitely. Recreating a deleted name does not restore old manual bindings. If source tagging remains enabled, a later sync may recreate its configured bindings.

### 3.3 Browsing and APIs

Tag details offer separate playlist and song views with independent search and pagination. The global playlist list supports tag filtering. Aggregate song and playlist counts separately to avoid multiplication; hidden-playlist counts match listing filters, and tagged empty playlists remain visible.

These APIs are implemented. Keep existing `/song-tags` paths and fields to minimize consumer migration.

| API / bridge | Behavior |
| --- | --- |
| `GET /playlists/{id}/tags` | Return playlist custom tag definitions |
| `PUT /playlists/{id}/tags`, `{tag_ids: [...]}` | Replace custom references while preserving system markers |
| `GET /playlists?tag_id=12` | Filter before pagination with identical conditions for results and totals |
| `GET /song-tags` / `GET /song-tags/{id}` | Add `playlist_count`; preserve existing `song_count` semantics |
| `songloft.tags.getPlaylistTags/bindPlaylists/unbindPlaylists` | Mirror song methods; bind/unbind only add/remove requested relationships |

Reuse the existing `tags` permission. Update host bridges and SDK types together. New handlers require swag annotations and generated API documentation.

## 4. Keep names unique; append the source on conflict

### 4.1 Naming rules

Default sync tries “original name”, “original name-source”, “original name-source (2)”, then “original name-source (3)”. For Navidrome, a conflicting name becomes “收藏-Navidrome”. Use an ASCII hyphen without surrounding spaces; numbers are only a fallback when the source suffix also conflicts. All playlists share one namespace regardless of source, connection, tags, or playlist type. Retain existing database comparison rules without changing case sensitivity.

| Situation | Playlist name | Custom tags |
| --- | --- | --- |
| Built-in favorites | 收藏 | No source tag required; retain built-in marker |
| Navidrome favorites conflicts with the built-in name | 收藏-Navidrome | Navidrome, Home NAS |
| A second Navidrome connection also has favorites | 收藏-Navidrome (2) | Navidrome, Office NAS |
| Remote name is available | 钢琴曲 | Navidrome |
| Another playlist already uses that name | 钢琴曲-Navidrome | Navidrome |

Treat hyphens, source text, parentheses, and numbers in the original name literally; do not guess or strip existing suffixes. The source suffix uses a stable plugin-defined name, separate from editable display tags; renaming the “Navidrome” tag does not rename playlists. Connection tags distinguish multiple connections to the same source; use numbers as the final naming fallback to limit title length. Allocate only on creation, initial migration, or an effective remote-name change. An explicit target name takes precedence; remote renames do not rename a playlist while its explicit target configuration remains unchanged.

### 4.2 Stable sync and conflicts

1. Resolve the existing connection + remote ID mapping and validate ownership. Recover missing mappings only through existing ownership rules. Never merge with or overwrite another playlist based on its name.
2. For initial allocation, load the complete occupied-name set, excluding the current playlist. Confirm candidates through actual create/update operations; preflight checks do not reserve names.
3. Try another candidate only for confirmed name conflicts, with a finite retry limit. Return network, permission, and ownership failures directly. Review conflict identification in the current string-based plugin bridge instead of treating arbitrary errors as collisions.
4. Extend plugin-private state alongside existing ID mappings with naming mode, requested base name, and allocated name; add no server fields. Recover from interruptions using the existing playlist ID, ownership, and actual name.
5. Once “收藏-Navidrome (2)” is allocated, keep it even if “收藏-Navidrome” is deleted; do not shorten it when the unsuffixed name becomes available either. Tag edits and connection display-name changes also do not trigger renaming. Preserve a manual host-side rename until target-name configuration changes or the remote name changes in default mode.
6. On an effective remote rename, allocate and update the same playlist ID instead of deleting and recreating it. An explicitly configured target-name conflict returns a clear error for the user to resolve; do not silently append a number.
7. Complete naming before replacing playlist members so a rename failure does not follow an already replaced membership. Naming, tags, and members have no transaction spanning plugin storage; do not promise atomic sync. Failures preserve retryable mappings and report the actual completed work.

Preview uses the same allocation rules and explains expected names and conflicts without writing. Execution rechecks availability to handle concurrent changes since preview and reports final names.

### 4.3 Existing playlist migration

Existing Navidrome playlists use “connection name · remote name”. Apply a one-time naming migration only to playlists with verified mappings and ownership, preserving IDs, songs, ordering, pins, and history associations.

When no explicit target is configured and the current name still matches the plugin’s old default, try the remote name, append the source on conflict, then a number if needed; persist the result. Keep explicit target settings. Preserve names that differ from the old default or whose ownership cannot be established. Do not globally strip “Navidrome” from arbitrary playlist names.

Use a fixed allocation order, such as connection ID then remote ID, so repeated previews are deterministic. Subsequent syncs reuse saved allocations.

## 5. Display and compatibility

Show the unique name as the main title, with source tags on a second line or in category navigation; support scrolling long titles. Prioritize source and connection tags when space is limited, with remaining tags in details. Hide recognized internal sync markers from the description UI while preserving their stored values for ownership checks.

No name-related schema migration is needed. Name lookups retain their return types, and MIoT does not need a new duplicate-name selection flow. Existing backup name merging, scanner naming, and built-in favorites rules remain; their independent limitations are outside this change.

Shared tags still need portable backup support: extend the format with definitions and song/playlist bindings, then map them to local IDs on import. Never copy cross-instance `tag:12` references directly. Keep compatibility with old backups without inventing missing definitions; do not write unresolved references. Cross-instance plugin ownership and raw IDs remain subject to the existing backup contract; matching display tags do not establish ownership.

## 6. Playlist covers and layout: included in #562 delivery

### 6.1 Library playlist covers

There are two current gaps: when a playlist has no own cover fields, `Playlist.CoverURLPath` returns an empty string, so cards never request fallback; `GetPlaylistCover` only checks local covers in the first 20 member songs and cannot use existing remote Navidrome song covers. Plugin-relative URLs also require existing internal URL resolution and authentication.

- Return a unified cover endpoint for playlists without their own cover but with members eligible for fallback. Avoid per-playlist member reads while assembling lists; select the image on demand at the endpoint.
- Prefer a valid playlist-local cover, then its remote cover, then valid local / remote member-song covers. Respect user-selected playlist covers and reuse existing remote and plugin resource handling.
- Query a bounded number of cover candidates rather than inspecting only the first 20 arbitrary songs. Bound remote fallback by a total request budget; do not scan the entire playlist or make unlimited network requests.
- Resolve plugin covers in both server and Bundle modes. Verify cache refresh after sync, membership changes, and cover replacement. Empty playlists and exhausted fallbacks retain placeholders.
- Library grids, playlist lists, home cards, and details share the same cover contract so the fix reaches every entry point.

### 6.2 Titles, sources, and the detail header

- Reuse existing `ScrollingText` for playlist lists, grid cards, and detail titles, respecting the long-title scrolling preference and reduced-motion settings. Keep static ellipsis when scrolling is disabled.
- Show source tags separately or in category navigation. Hide internal sync markers from displayed descriptions without deleting stored descriptions; long user descriptions remain accessible.
- Reduce the phone detail cover from 160px to approximately 96px and trim surrounding whitespace. Let metadata adapt to its content instead of retaining a fixed 160px height; large fonts must not clip titles, tags, or counts.
- Increase the song count from the current small footnote to a corresponding approximately 15–16px font tier and keep it visible. Wide layouts retain larger artwork and existing action organization.

### 6.3 Library and playlist song rows

Phone favorite and overflow buttons are already adjacent 44px targets with zero individual margins. Review actual container whitespace, focusing on horizontal row padding, the 28px index column and its 4px margin, and the gap between metadata and actions. Keep icons centered and retain non-overlapping 44px touch targets.

Library and playlists share `SongListRow`; apply phone styling consistently. Preserve selection, sorting, three-or-more-digit indices, current playback state, and wide-column alignment. Phone song removal remains in the overflow menu with confirmation; existing long-song-title scrolling and subtitle duration remain effective.

### 6.4 Completion criteria

Besides unit and API regressions, verify at least 360px and 390px phone widths with long Chinese / English playlist and song names, multiple sources and connections, light / dark themes, and large text. Record usable title width, header height, and visible first-screen song count. Exercise favorites, overflow menus, removal confirmation, tag filters, and pagination.

Cover verification includes Navidrome plugin-relative URLs, ordinary remote images, local images, missing images, and failed-resource fallback in both server and Bundle modes. Scrolling and artwork on the reported Honor 30 Pro / HarmonyOS 4 still need device acceptance; browser checks cannot substitute for that result.

## 7. Code changes

These are the implementation areas based on the current paths. The complete design spans five repositories: main, Lynx, Flutter, Navidrome, and the plugin toolchain.

| Area | Main files | Expected changes |
| --- | --- | --- |
| Tag models and repositories | `internal/models/song_tag.go`, `internal/models/models.go`, `internal/database/song_tag_repository.go`, `playlist_repository.go`, `queries/song_tags.sql`, `queries/playlists.sql` | Playlist tag references, filters, counts, bulk cleanup, and generated sqlc output |
| Transactions, services, and APIs | `internal/database/unit_of_work.go`, `sqlite.go`, `internal/services/song_tag_service.go`, `playlist_service.go`, `internal/handlers/song_tag.go`, `playlist.go`, `internal/app/app.go`, `routers.go` | Transactional deletion / binding, dependency injection, playlist-tag endpoints, and list filters |
| Playlist covers | `internal/models/models.go`, `internal/handlers/playlist.go`, `internal/services/playlist_service.go`, relevant member queries / repositories and application wiring | Fallback endpoint exposure, candidate selection, existing remote / internal URL resolution, and refresh handling |
| Backup and plugin bridge | `internal/models/backup.go`, `internal/services/backup_service.go`, `internal/jsplugin/api_bridge.go` | Portable tag definitions and bindings; playlist methods in the tags namespace |
| Lynx tags and categories | `src/models/song-tag.ts`, `src/features/library/api/song-tags-api.ts`, `data/song-tags-query.ts`, `pages/TagSongsPage.tsx`, `widgets/ManageTagsSheet.tsx`, `TagGridView.tsx`, `TagCard.tsx`, and playlist APIs / queries / editing entry points | Shared editing, playlist / song browsing, counts, pagination, and source display |
| Lynx titles and layout | `src/features/playlist/widgets/PlaylistCard.tsx`, `PlaylistsView.tsx`, `pages/PlaylistDetailPage.tsx/.css`, `widgets/PlaylistDescPanel.tsx`, `src/shared/ui/MediaListItem.tsx/.css`, `src/features/library/widgets/SongListRow.tsx`, `SongRow.css` | Title scrolling, separate source text, hidden internal descriptions, compact headers, counts, and song-row whitespace |
| Flutter feature parity | `lib/features/library/data/song_tags_api.dart`, `presentation/providers/song_tag_provider.dart`, `tag_songs_page.dart`, `widgets/tag_grid_view.dart`, `lib/shared/widgets/manage_tags_modal.dart`, and playlist API / editing / list / detail files | Shared tags, categories, sources, and long titles; phone header changes based on Flutter’s actual layout |
| Navidrome plugin | `src/sync.ts`, `model.ts`, `main.ts`, relevant configuration UI, `tests/sync.test.mjs` | Source tags, conflict suffixes, stable naming state, preview, and verified managed-name migration |
| SDK, copy, and generated output | `plugins/toolchain/packages/plugin-sdk/src/global.d.ts`, relevant package versions / lockfiles, client localization resources, `docs/swagger.json`, `swagger.yaml`, `docs.go`, bilingual user documentation | Bridge types, dependency compatibility, copy, API contracts, and verification records |

Add meaningful backend, plugin, and client regressions. Keep the unique-name index, built-in identities, and existing song-tag relationships; add neither `playlist_tag_links` nor a display-name column. This change does not require rewriting MIoT, WebDAV, or TV workflows.

## 8. Implementation order and verification

1. Implement backend shared bindings, filters, counts, transactional cleanup, API documentation, and portable tag backup mapping.
2. Fix the backend playlist-cover contract and remote / plugin fallback for all client entry points.
3. Reuse client tag editing and category navigation for songs and playlists, support long titles, and implement detail-header, count, and song-row spacing improvements.
4. Update Navidrome source / optional connection tags, default name allocation, consistent preview, and verified managed-name migration; update SDK dependencies and plugin documentation.
5. Perform acceptance against the complete #562 scenarios. Covers and layout are required deliverables; completing tags and naming alone does not complete the issue.

| Required scenario | Expected result |
| --- | --- |
| Same-name normal / radio playlists, direct creation or renaming | Preserve global uniqueness and HTTP 409 behavior |
| Built-in favorites plus two remote favorites; names already containing numbers | Allocate available unique names without changing or claiming existing playlists |
| Concurrent occupation and unrelated write failures | Retry only actual conflicts; surface other failures |
| Repeated sync, original or shorter candidates becoming available, tag / connection-name edits | Stable playlist IDs and allocated names |
| Remote rename, explicit target conflict, manual host rename | Respect naming modes without overwrites or lost mappings |
| Old default-name migration, uncertain ownership, occupation after preview | Preserve data safely and explain execution results |
| Concurrent tag deletion/binding and ID reuse | No dangling or incorrect references; preserve system markers |
| Tag pagination / hidden counts, empty playlists, cross-instance backup | Consistent counts and lists; correctly remapped references |
| Library / list / home / detail artwork, plugin URLs, failed-resource fallback | Consistent artwork, own-cover priority, bounded queries and network work |
| Narrow long titles, reduced motion, large text, phone / wide layouts | Correct scrolling or static ellipsis, unclipped headers and counts |
| Index / action spacing, selection / sorting / removal confirmation | More title space while preserving 44px targets and interactions |

See the [#562 verification and delivery record](issue-562-verification.md). Native device acceptance: Not run. Commit, push and Issue reply are authorized; keep the Issue open for device acceptance. A formal host release is separate.
