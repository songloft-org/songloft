# Playlist resume and audiobooks

Every normal playlist has independent resume state. Playback history remains a list of recently played songs; its timestamps do not represent track positions.

## Behavior

- The Audiobook editor switch sets the system label `audiobook`. It does not change the normal/radio type or depend on user-defined tag names.
- An audiobook with no saved mode defaults to sequential playback with a terminal end. Other normal playlists initially use the current player mode. A saved playlist mode takes precedence; toggling Audiobook preserves mode and position.
- Continue restores the song, millisecond position and playlist mode. Explicit taps in lists, search results or history start the selected track from zero.
- Search selects a starting point within the entire playlist, using its persisted sort. Locate last listened clears the search.
- Removed members invalidate the position. Deleting the playlist cascades its state. A natural terminal end shows Finished.

## Server contract

Authenticated `GET/PUT/DELETE /api/v1/playlists/{id}/playback-progress` supports normal playlists and follows the existing instance-shared data model.

GET returns nullable song, position_ms, play_mode, completed, invalid, version and updated_at. An absent record has version=0.

PUT accepts version and optional song_id/position_ms/play_mode/completed. Omitted fields are retained; mode can be saved independently. The song must belong to the playlist. Modes are order, loop, single, random and singlePlay. A stale version returns 409 and must not be blindly retried. DELETE clears the position, retains the mode and increments the version so stale writes cannot restore a cleared position.

Playlist PUT accepts an optional audiobook boolean. It changes only that system label and preserves other labels. Existing built-in playlist editing protection still applies.

## Persistence and compatibility

Playback saves every five seconds, supplemented by pause, seek, track/context changes and background events. Failed initial loads do not create a new listening position. Writes are serialized. Failed network snapshots are kept locally and replayed before a later playback request only if the server version still matches. Conflicts stop automatic writes for that session; the next explicit playback request reads fresh state.

Older servers without the endpoint retain basic playback. Old history does not fabricate seconds. Process termination and browser closure cannot guarantee a final upload; periodic persistence is the fallback. Native background behavior requires device acceptance on each platform.
