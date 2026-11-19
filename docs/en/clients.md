# Choosing a client

Songloft offers Flutter, Lynx, Electron desktop, and TV clients connecting to the same backend. Their packages and capability scopes are maintained separately.

| Item | Flutter | Lynx | Songloft Desktop | Songloft TV |
| --- | --- | --- | --- | --- |
| Technology | Flutter / Dart | ReactLynx / TypeScript | Electron | Kotlin / Jetpack Compose for TV |
| Platforms | Android, iOS, macOS, Windows, Linux, Web | Android, iOS, HarmonyOS, Web | Windows, macOS, Linux | Android 5.0+ TVs / boxes and landscape large-screen devices |
| Bundled local backend | Flutter repository builds Android/iOS/macOS/Windows/Linux Bundle packages | Lynx Android/iOS/Windows/macOS Releases embed the Go backend | Not implemented; requires a separate server | Not implemented; requires a separate server |
| Desktop client | Supported | Windows/macOS packages use the shared release workflow. Final device acceptance is in progress | Dedicated desktop client | No desktop edition |
| Client updates | See Flutter's client guides | Mobile packages use Releases. Desktop packages support in-app updates through per-architecture manifests | See the Electron repository instructions | Update checks in settings |
| Core features | Playback, library, playlists, lyrics, plugins, themes | Implemented, with native device regression still required | Library, playlists, lyrics, plugins, desktop integration | Remote navigation, music / MV, lyrics, karaoke, QR song requests |
| License | Apache-2.0 | Apache-2.0 | Apache-2.0 | Apache-2.0 |

## Flutter

- [Standalone downloads](https://github.com/songloft-org/songloft-player/releases/latest): connect to a separate server.
- [Bundle downloads](https://github.com/songloft-org/songloft-player/releases/latest): the repository releases both standard clients and `songloft-bundled-*` packages.
- The standard `songloft-android.apk` and Bundle `songloft-bundled-android.apk` each support ARM64, ARMv7, and x86_64 in one APK. Universal packages appear first in the dev release. [Flutter dev release](https://github.com/songloft-org/songloft-player/releases/tag/dev)
- [Architecture (Chinese)](../player/architecture.md) · [Build guide (Chinese)](../player/build_guide.md) · [Source](https://github.com/songloft-org/songloft-player).

Full backend images/binaries embed Lynx Web by default. Native Flutter and standalone Web remain available; explicitly build legacy embedded Flutter Web with `make build-frontend-flutter-web-embedded`.

## Lynx client

Underlying technology: [Lynx official website](https://lynxjs.org/) · [ReactLynx documentation](https://lynxjs.org/react/).

- [Development downloads](https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev): the shared workflow builds Windows x64, macOS arm64/x64, Android, iOS, HarmonyOS, and Web, and updates downloads only after all targets pass.
- [Stable releases](https://github.com/songloft-org/songloft-player-lynx/releases/latest): version tags trigger publication; packages may not exist before the first successful release.
- Lynx Releases include the Go backend and local-mode entry points, plus Windows NSIS, macOS DMG/ZIP, and per-architecture update manifests. The root repository does not build or publish client apps.
- [Overview](player-lynx/index.md) · [Installation](player-lynx/installation.md) · [Build](player-lynx/build-and-run.md) · [Testing](player-lynx/testing.md) · [Releasing](player-lynx/releasing.md) · [Contributing](player-lynx/contributing.md).

| Platform                 | Package                                          | Limits                                                                      |
| ------------------------ | ------------------------------------------------ | --------------------------------------------------------------------------- |
| Android 6.0+             | `songloft-lynx-android.apk` | Includes the local backend; upgrades require the same key and a newer build number |
| iOS 15+                  | `songloft-lynx-ios-nosign.ipa` | Unsigned; re-sign before installation. Live Activity needs iOS 16.2+ |
| HarmonyOS NEXT / API 13+ | `songloft-lynx-harmony.hap`                      | Experimental; profile controls eligible devices. No fullscreen video        |
| Web                      | `songloft-lynx-web-{standalone,embedded}.tar.gz` | Requires a server; core flows support ordinary HTTP; no DLNA or single-song offline cache |

Releases include `version.json` and `checksums.txt`. Downloadable dev packages are also Release builds with no TCP/E2E bridge.

Standalone Web shows a server-address field; embedded uses the same-origin server and hides that field. Both require suitable MIME types; the current asynchronous engine does not require SAB or isolation headers. Root and subpath deployments are verified; see [Web deployment](player-lynx/web-deployment.md) for browser, plugin and background-operation limits.

## Songloft Desktop (Electron desktop client)

[Songloft Desktop](https://github.com/songloft-org/songloft-desktop) is Songloft's official Electron desktop client for Windows, macOS, and Linux. It requires a Songloft server and does not include a bundled local backend.

- [Downloads and installers](https://github.com/songloft-org/songloft-desktop/releases/latest): see the repository instructions and Release assets for installation on each platform.
- [Project documentation and source](https://github.com/songloft-org/songloft-desktop) · [Report issues](https://github.com/songloft-org/songloft-desktop/issues).

## Songloft TV (TV / large-screen client)

[Songloft TV](https://github.com/songloft-org/songloft-tv) uses Kotlin + Jetpack Compose for TV on Android 5.0 (API 21) or later TVs, boxes, and landscape large-screen devices. It requires an existing Songloft server and does not include a bundled local backend.

- [Stable APK downloads](https://github.com/songloft-org/songloft-tv/releases/latest) · [Development downloads](https://github.com/songloft-org/songloft-tv/releases/tag/dev).
- [Build and installation (Chinese)](https://github.com/songloft-org/songloft-tv#构建) · [Source](https://github.com/songloft-org/songloft-tv) · [Report issues](https://github.com/songloft-org/songloft-tv/issues).
- Supports remote D-Pad navigation and touch, QR server configuration, music / MV playback, original / backing track switching, and synchronized / word-timed lyrics.
- Provides karaoke and QR song requests, favorite songs / radio, playback cache, background playback, and update checks in settings.
- Playback statistics require the server-side statistics plugin; the corresponding page is hidden when it is absent.

See the TV repository for device compatibility and installation requirements, and its [disclaimer (Chinese)](https://github.com/songloft-org/songloft-tv#声明与免责) for in-car usage limits.

## Third-party clients

See [Third-party Clients in the quick start](quick-start.md#🌐-third-party-clients) for more community clients.

## Releases and documentation upkeep

Preview a Lynx release in its own repository with `pnpm run release patch --dry-run`. Actual release synchronizes versions and atomically pushes main and its version tag. A client release does not require changing the server version.

Public guides are synced from the `clients/player-lynx` submodule to `/player-lynx/` and `/en/player-lynx/`. Edit, commit, and push the source repository, then update its submodule pointer in the parent. Do not edit generated site pages. The Flutter and Lynx client repositories each build Go from the latest root `main` commit at the start of the workflow and publish their own Bundle clients. Root Releases publish server/container assets only, not client installers.
