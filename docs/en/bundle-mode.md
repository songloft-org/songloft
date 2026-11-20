# Bundle Local Mode

Bundle mode installs the Go backend with the client, so users do not need to deploy a separate server. Clients switch between local and remote modes from the login and settings screens. Android and desktop users choose a music directory the first time local mode is enabled; iOS uses the app's Documents directory. The host starts the backend, waits for its health check, then establishes a session with local administrator credentials. The app does not show a separate local-or-remote chooser on first launch.

## Clients and package sources

| Client | Bundle packages | Release repository | Backend source |
|---|---|---|---|
| Flutter | `songloft-bundled-android.apk`, `songloft-bundled-ios-nosign.ipa`, `songloft-bundled-linux-x64.tar.gz`, `songloft-bundled-macos.zip`, `songloft-bundled-windows-x64.zip` | [Backend stable](https://github.com/songloft-org/songloft/releases/latest) · [dev](https://github.com/songloft-org/songloft/releases/tag/dev) | Full root and submodule SHAs are fixed during Release preparation |
| Lynx | `songloft-lynx-bundled-android.apk`, `songloft-lynx-bundled-ios-nosign.ipa`, `songloft-lynx-bundled-win32-x64.exe`, `songloft-lynx-bundled-darwin-arm64.dmg`, `songloft-lynx-bundled-darwin-x64.dmg` | [Backend stable](https://github.com/songloft-org/songloft/releases/latest) · [dev](https://github.com/songloft-org/songloft/releases/tag/dev) | Full root and submodule SHAs are fixed during Release preparation |
| Lynx standard | Android/iOS and standard desktop packages | [Lynx stable](https://github.com/songloft-org/songloft-player-lynx/releases/latest) · [dev](https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev) | Fixed by the Lynx Release |
| Lynx HarmonyOS / Web | HAP and standalone/embedded Web | [Lynx stable](https://github.com/songloft-org/songloft-player-lynx/releases/latest) · [dev](https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev) | Non-Bundle outputs |

The backend Release also publishes `bundle-index.json`, `bundle-flutter-version.json`, `bundle-flutter-backend-manifest-<abi>.json`, `bundle-lynx-version.json`, `bundle-lynx-version.json.sig`, and `desktop-bundled-<target>.json`. Each `bundle-index.json` record includes `client`, `edition`, platform, asset name, full client SHA, full backend SHA, version, build time, and SHA-256. The server's own `version.json` remains separate.

Flutter standard reads the Flutter Release. Flutter Bundle reads the backend Release. Lynx standard reads the Lynx Release. Lynx Bundle reads the backend Release. The desktop updater selects the repository and signed manifest by `variant`, and desktop asset names contain no `dev` or `stable` segment.

## Android and iOS local mode

- **Android**: gomobile builds the backend as an `.aar`, called through the `SongloftBundle` native module. The Lynx APK with the embedded backend requires Android 6.0 or later. The user grants read access through the system directory picker.
- **iOS**: gomobile builds `Songloft.xcframework`, called through a Swift module. The music directory lives under the app's Documents directory and can be managed with the Files app; system directory authorization does not apply.
- Both platforms share Lynx mode-switching and session-restore logic. The backend chooses a dynamic port; local login starts only after its health check succeeds. The local backend generates and stores its administrator password in the app data directory.
- When switching to a remote server, the client stops the local session and backend before connecting to the configured server. Choosing remote mode does not delete local library data.

Device directory permissions, scanning and playback, clean installs, and upgrades are verified per platform. Check each client's Releases for the published packages and versions.

## Related documentation

- [Choosing a client](clients.md)
- [Frontend architecture · Deployment modes](architecture_frontend.md#deployment-modes)
- [Flutter client Bundle documentation](/player/architecture)
- [Lynx installation guide](player-lynx/installation.md)
- [Lynx release guide](player-lynx/releasing.md)
