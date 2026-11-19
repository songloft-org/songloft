# Lynx desktop Bundle host contract

[简体中文](../desktop-host.md)

Windows/macOS implementation lives in the [client subrepository](https://github.com/songloft-org/songloft-player-lynx). The backend retains its HTTP API, supplying lite executables, dynamic ports and optional parent-exit monitoring. Desktop packages are now published by the client's shared workflow. Production signing/notarization and final device acceptance remain open.

The host launches a matching-architecture backend with absolute `-db` and `-music` paths and `-port 0 -parent-watch-stdin`, using its application data directory as cwd. Parse the announced actual port and verify `/api/v1/health` before automatic login. Pass random administrator credentials through `ADMIN_USERNAME/ADMIN_PASSWORD`, never command-line arguments or ordinary logs. Keep music separate from database/plugin/cache directories. The Bundle backend intentionally listens on all interfaces and allows LAN access; the UI displays available LAN addresses, while the password stays in the credential store for local automatic login.

If the selected music folder is unavailable, the client keeps the local mode and existing library and offers retry or another folder on the sign-in page. A replacement folder is saved after the bundled backend passes its health check. A missing drive keeps the user on the sign-in page even with an existing token and does not trigger a database migration recovery prompt. The music folder may be read-only. The database and cache still need their own writable directories.

`-parent-watch-stdin` defaults off, preserving existing server and mobile behavior. When enabled, stdin must be a pipe whose only writer belongs to the host. Explicit exit stops playback/plugins/network before closing it; a crashed parent also produces EOF. EOF/read failure and OS signals share the 15-second graceful shutdown. The desktop grants 20 seconds before forced termination; do not inherit the writer into other children.

Parent-pipe monitoring starts before application initialization. If the host exits while the backend initializes or binds, the backend waits until the HTTP server has registered its listener before graceful shutdown, avoiding a shutdown-before-server race that could leave an orphan.

Desktop CI resolves the latest root `main` commit at startup. All three architectures use that SHA and its `pkg/tag` Go replacement gitlink. Whole-app updates stop the backend and back up SQLite plus remaining WAL/SHM. After a failed new-backend migration, old backends may run only after explicitly restoring the matching backup, retaining failure evidence.

Regression: `go test ./internal/parentwatch ./internal/app ./internal/config`. Client integration additionally checks real scanning, Range, persistence across restart and shutdown. This does not replace clean target-device installation/upgrade acceptance.
