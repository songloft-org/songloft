# Release 包名清单

这是公开 Release 的文件名清单。dev 和正式版各用自己的 tag，同一包在两个通道中的文件名一致。iOS IPA 安装前需要重签。Flutter 标准版的 Linux DEB、RPM 和 macOS DMG 取决于对应打包步骤是否产出。

[后端 Releases](https://github.com/songloft-org/songloft/releases) · [Flutter Releases](https://github.com/songloft-org/songloft-player-flutter/releases) · [Lynx Releases](https://github.com/songloft-org/songloft-player-lynx/releases) · [FFmpeg Releases](https://github.com/hanxi/ffmpeg-builder/releases)

## Flutter Bundle（后端仓库）

| 目标 | 文件名 |
| --- | --- |
| Android | `songloft-bundled-android.apk` |
| iOS | `songloft-bundled-ios-nosign.ipa` |
| Linux x64 | `songloft-bundled-linux-x64.tar.gz` |
| macOS arm64 | `songloft-bundled-macos.zip` |
| Windows x64 | `songloft-bundled-windows-x64.zip` |

## Lynx Bundle（后端仓库）

| 目标 | 文件名 |
| --- | --- |
| Android | `songloft-lynx-bundled-android.apk` |
| iOS | `songloft-lynx-bundled-ios-nosign.ipa` |
| Windows x64 | `songloft-lynx-bundled-win32-x64.exe` |
| macOS arm64 | `songloft-lynx-bundled-darwin-arm64.dmg`、`songloft-lynx-bundled-darwin-arm64.zip` |
| macOS x64 | `songloft-lynx-bundled-darwin-x64.dmg`、`songloft-lynx-bundled-darwin-x64.zip` |

## Flutter 标准版（Flutter 仓库）

| 目标 | 文件名 |
| --- | --- |
| Android | `songloft-android.apk` |
| iOS | `songloft-ios-nosign.ipa` |
| Linux x64 | `songloft-linux-x64.tar.gz`、`songloft-linux-x64.deb`、`songloft-linux-x64.rpm`、`songloft-linux-x64.AppImage` |
| macOS | `songloft-macos.dmg`、`songloft-macos.zip` |
| Windows x64 | `songloft-windows-x64.zip` |
| Web | `songloft-web-standalone.tar.gz`、`songloft-web-embedded.tar.gz` |

## Lynx 标准版（Lynx 仓库）

| 目标 | 文件名 |
| --- | --- |
| Android | `songloft-lynx-android.apk` |
| iOS | `songloft-lynx-ios-nosign.ipa` |
| HarmonyOS | `songloft-lynx-harmony.hap` |
| Web | `songloft-lynx-web-standalone.tar.gz`、`songloft-lynx-web-embedded.tar.gz` |
| Windows x64 | `songloft-lynx-standard-win32-x64.exe` |
| macOS arm64 | `songloft-lynx-standard-darwin-arm64.dmg`、`songloft-lynx-standard-darwin-arm64.zip` |
| macOS x64 | `songloft-lynx-standard-darwin-x64.dmg`、`songloft-lynx-standard-darwin-x64.zip` |

## 独立后端

| 目标 | 文件名 |
| --- | --- |
| Linux amd64 | `songloft-linux-amd64`、`songloft-linux-amd64-lite` |
| Linux arm64 | `songloft-linux-arm64`、`songloft-linux-arm64-lite` |
| Linux armv7 | `songloft-linux-armv7`、`songloft-linux-armv7-lite` |
| macOS amd64 | `songloft-darwin-amd64`、`songloft-darwin-amd64-lite` |
| macOS arm64 | `songloft-darwin-arm64`、`songloft-darwin-arm64-lite` |
| Windows amd64 | `songloft-windows-amd64.exe`、`songloft-windows-amd64-lite.exe` |
| Windows arm64 | `songloft-windows-arm64.exe`、`songloft-windows-arm64-lite.exe` |

## Docker 离线包

| 目标 | 文件名 |
| --- | --- |
| linux-amd64 | `songloft-docker-linux-amd64.tar`、`songloft-docker-linux-amd64-lite.tar` |
| linux-arm64 | `songloft-docker-linux-arm64.tar`、`songloft-docker-linux-arm64-lite.tar` |
| linux-arm-v7 | `songloft-docker-linux-arm-v7.tar`、`songloft-docker-linux-arm-v7-lite.tar` |

## FFmpeg 8.0.1（ffmpeg-builder 仓库）

| 目标 | 文件名 |
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

## 更新资产

Lynx 仓库发布桌面标准版的 `desktop-standard-<target>.json`，后端仓库发布桌面 Bundle 的 `desktop-bundled-<target>.json`。其中目标为 `win32-x64`、`darwin-arm64` 或 `darwin-x64`。Flutter Android Bundle 补丁使用 `bundle-patch-<abi>-<backend-commit>.zip` 和 `bundle-libgojni-<abi>-<backend-commit>.so`。Lynx Bundle 的签名热更新资产是 `songloft-lynx-main.lynx.bundle`。签名和清单详见 [Bundle 模式说明](bundle-mode.md)。
