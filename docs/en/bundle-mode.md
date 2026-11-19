# Bundle Local Mode

Bundle mode installs the Go backend with the client, so users do not need to deploy a separate server. Clients switch between local and remote modes from the login and settings screens. Android and desktop users choose a music directory the first time local mode is enabled; iOS uses the app's Documents directory. The host starts the backend, waits for its health check, then establishes a session with local administrator credentials. The app does not show a separate local-or-remote chooser on first launch.

## Clients and package sources

| Client | Bundle packages | Release repository | Backend source |
|---|---|---|---|
| Flutter | Universal Android APK `songloft-bundled-android.apk` (ARM64, ARMv7, x86_64), iOS `songloft-bundled-ios-nosign.ipa`, Linux `songloft-bundled-linux-x64.tar.gz`, Windows `songloft-bundled-windows-x64.zip`, macOS `songloft-bundled-macos.zip` | [Stable](https://github.com/songloft-org/songloft-player/releases/latest) · [Dev](https://github.com/songloft-org/songloft-player/releases/tag/dev) | Latest root `main` commit at workflow startup |
| Lynx | Android `songloft-lynx-android.apk`, iOS `songloft-lynx-ios-nosign.ipa` | [songloft-player-lynx Releases](https://github.com/songloft-org/songloft-player-lynx/releases/latest) | Latest root `main` commit at workflow startup |
| Lynx desktop | Candidate Windows/macOS packages | Separate desktop workflow in the Lynx repository, stable downloads pending acceptance | Latest root `main` commit at workflow startup |

The Flutter and Lynx client repositories build and publish their own Bundles. Each workflow resolves the latest commit and version from the public root repository’s `main` at startup. All platforms and architectures in that run use the same SHA. The backend or package metadata records the version and commit used. Root Releases publish server, container, and server tool assets.

The root repository and its `pkg/tag` dependency are public, so client GitHub Actions can check them out by fixed SHA without a personal access token or root-repository write permission. Client Release write permission is used only to publish to each client's own repository.

Flutter Bundle uses distinct asset names. Update checks and Android Bundle hot updates use the Flutter repository's own Release; standard and Bundle clients share a version release, with different manifest prefixes. Lynx Android/iOS packages embed the backend directly and are updated through Lynx Releases. Windows/macOS Lynx desktop candidates remain pending consolidated acceptance.

Older Flutter Bundle installations downloaded from the root repository still check the old location. Once the Flutter repository publishes the Bundle assets listed above, download the full package for your platform from that repository to migrate. Lynx mobile packages keep their filenames, so check the corresponding release to confirm it supports local mode.

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
