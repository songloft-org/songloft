# Release package names

These are the filenames defined by the release workflows. The dev and stable channels use the same names within their respective tags. Historical Releases keep their original filenames. While signing is unconfigured, dev Bundles are available as artifacts of the corresponding backend Actions run. iOS IPAs require re-signing before installation. Flutter Standard provides Linux DEB, RPM, macOS DMG, and dev AppImage packages when those packaging steps produce them. Stable releases require AppImage.

[Backend Releases](https://github.com/songloft-org/songloft/releases) · [Flutter Releases](https://github.com/songloft-org/songloft-player-flutter/releases) · [Lynx Releases](https://github.com/songloft-org/songloft-player-lynx/releases) · [FFmpeg Releases](https://github.com/hanxi/ffmpeg-builder/releases)

## Flutter Bundle (backend repository)

| Target | Filename |
| --- | --- |
| Android | `songloft-bundled-android.apk` |
| iOS | `songloft-bundled-ios-nosign.ipa` |
| Linux x64 | `songloft-bundled-linux-x64.tar.gz` |
| macOS arm64 | `songloft-bundled-macos.zip` |
| Windows x64 | `songloft-bundled-windows-x64.zip` |

## Lynx Bundle (backend repository)

| Target | Filename |
| --- | --- |
| Android | `songloft-lynx-bundled-android.apk` |
| iOS | `songloft-lynx-bundled-ios-nosign.ipa` |
| Windows x64 | `songloft-lynx-bundled-win32-x64.exe` |
| macOS arm64 | `songloft-lynx-bundled-darwin-arm64.dmg`, `songloft-lynx-bundled-darwin-arm64.zip` |
| macOS x64 | `songloft-lynx-bundled-darwin-x64.dmg`, `songloft-lynx-bundled-darwin-x64.zip` |

## Flutter Standard (Flutter repository)

| Target | Filename |
| --- | --- |
| Android | `songloft-android.apk` |
| iOS | `songloft-ios-nosign.ipa` |
| Linux x64 | `songloft-linux-x64.tar.gz`, `songloft-linux-x64.deb`, `songloft-linux-x64.rpm`, `songloft-linux-x64.AppImage` |
| macOS | `songloft-macos.dmg`, `songloft-macos.zip` |
| Windows x64 | `songloft-windows-x64.zip` |
| Web | `songloft-web-standalone.tar.gz`, `songloft-web-embedded.tar.gz` |

## Lynx Standard (Lynx repository)

| Target | Filename |
| --- | --- |
| Android | `songloft-lynx-android.apk` |
| iOS | `songloft-lynx-ios-nosign.ipa` |
| HarmonyOS | `songloft-lynx-harmony.hap` |
| Web | `songloft-lynx-web-standalone.tar.gz`, `songloft-lynx-web-embedded.tar.gz` |
| Windows x64 | `songloft-lynx-standard-win32-x64.exe` |
| macOS arm64 | `songloft-lynx-standard-darwin-arm64.dmg`, `songloft-lynx-standard-darwin-arm64.zip` |
| macOS x64 | `songloft-lynx-standard-darwin-x64.dmg`, `songloft-lynx-standard-darwin-x64.zip` |

## Backend executables

| Target | Filename |
| --- | --- |
| Linux amd64 | `songloft-linux-amd64`, `songloft-linux-amd64-lite` |
| Linux arm64 | `songloft-linux-arm64`, `songloft-linux-arm64-lite` |
| Linux armv7 | `songloft-linux-armv7`, `songloft-linux-armv7-lite` |
| macOS amd64 | `songloft-darwin-amd64`, `songloft-darwin-amd64-lite` |
| macOS arm64 | `songloft-darwin-arm64`, `songloft-darwin-arm64-lite` |
| Windows amd64 | `songloft-windows-amd64.exe`, `songloft-windows-amd64-lite.exe` |
| Windows arm64 | `songloft-windows-arm64.exe`, `songloft-windows-arm64-lite.exe` |

## Docker archives

| Target | Filename |
| --- | --- |
| linux-amd64 | `songloft-docker-linux-amd64.tar`, `songloft-docker-linux-amd64-lite.tar` |
| linux-arm64 | `songloft-docker-linux-arm64.tar`, `songloft-docker-linux-arm64-lite.tar` |
| linux-arm-v7 | `songloft-docker-linux-arm-v7.tar`, `songloft-docker-linux-arm-v7-lite.tar` |

## FFmpeg 8.0.1 (ffmpeg-builder repository)

| Target | Filename |
| --- | --- |
| linux-x86_64 | `ffmpeg-8.0.1-linux-x86_64.tar.gz` |
| linux-arm64 | `ffmpeg-8.0.1-linux-arm64.tar.gz` |
| linux-armv7 | `ffmpeg-8.0.1-linux-armv7.tar.gz` |
| windows-x86_64 | `ffmpeg-8.0.1-windows-x86_64.zip` |
| macos-arm64 | `ffmpeg-8.0.1-macos-arm64.tar.gz` |
| macos-x86_64 | `ffmpeg-8.0.1-macos-x86_64.tar.gz` |
| android-arm64-v8a | `ffmpeg-8.0.1-android-arm64-v8a.tar.gz` |
| android-armeabi-v7a | `ffmpeg-8.0.1-android-armeabi-v7a.tar.gz` |
| android-x86_64 | `ffmpeg-8.0.1-android-x86_64.tar.gz` |
| ios-arm64 | `ffmpeg-8.0.1-ios-arm64.tar.gz` |

## Update assets

Desktop Ed25519 manifests use `desktop-standard-<target>.json` in the Lynx repository and `desktop-bundled-<target>.json` in the backend repository. A target is `win32-x64`, `darwin-arm64`, or `darwin-x64`. Flutter Android Bundle patch artifacts use `bundle-patch-<abi>-<backend-commit>.zip` and `bundle-libgojni-<abi>-<backend-commit>.so`. The Lynx signed Bundle update asset is `songloft-lynx-main.lynx.bundle`. See [Bundle Mode](bundle-mode.md) for signing and manifest details.
