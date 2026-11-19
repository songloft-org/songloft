# 客户端选择

Songloft 提供 Flutter、Lynx、Electron 桌面与 TV 客户端，均连接同一后端；发布包和能力范围分别维护。

| 项目 | Flutter | Lynx（预览版） | Songloft Desktop | Songloft TV |
| --- | --- | --- | --- | --- |
| 技术 | Flutter / Dart | ReactLynx / TypeScript | Electron | Kotlin / Jetpack Compose for TV |
| 平台 | Android、iOS、macOS、Windows、Linux、Web | Android、iOS、HarmonyOS、Web | Windows、macOS、Linux | Android 5.0+ 电视 / 盒子及横屏大屏设备 |
| Bundle 本地后端 | Flutter 客户端仓库构建 Android/iOS/macOS/Windows/Linux Bundle 包 | Lynx 客户端仓库的 Android/iOS Release 内嵌 Go 后端 | 未实现，需独立服务器 | 未实现，需独立服务器 |
| 桌面客户端 | 支持 | Windows/macOS 候选包待验收 | 专用桌面客户端 | 无桌面版 |
| 客户端更新 | 以 Flutter 客户端指南为准 | 重新下载并安装，尚无客户端内检查更新 | 以 Electron 仓库说明为准 | 设置页检查更新 |
| 核心功能 | 播放、曲库、歌单、歌词、插件、主题等 | 已有对应实现，仍需原生设备回归 | 曲库、歌单、歌词、插件与桌面集成 | 遥控器导航、音乐 / MV、歌词、K 歌与扫码点歌 |
| 许可证 | Apache-2.0 | Apache-2.0 | Apache-2.0 | Apache-2.0 |

## Flutter

- [标准版下载](https://github.com/songloft-org/songloft-player/releases/latest)：连接独立服务器。
- [Bundle 下载](https://github.com/songloft-org/songloft-player/releases/latest)：该仓库 Release 同时发布标准客户端和 `songloft-bundled-*` 包。
- Android 标准版 `songloft-android.apk` 与 Bundle 版 `songloft-bundled-android.apk` 各用一个通用 APK，均支持 ARM64、ARMv7 和 x86_64。通用包从开发版开始提供。[Flutter 开发版](https://github.com/songloft-org/songloft-player/releases/tag/dev)
- [架构](player/architecture.md) · [构建](player/build_guide.md) · [源码](https://github.com/songloft-org/songloft-player)。

后端完整镜像与二进制默认嵌入 Lynx Web。Flutter 原生客户端与独立 Web 保留；旧 Flutter 嵌入版可通过 `make build-frontend-flutter-web-embedded` 显式构建。

## Lynx 预览版

底层技术：[Lynx 官网](https://lynxjs.org/) · [ReactLynx 官方文档](https://lynxjs.org/react/)。

- [开发版下载](https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev)：main 代码推送后构建，五个包全部成功才更新。
- [正式版本](https://github.com/songloft-org/songloft-player-lynx/releases/latest)：版本 tag 触发；首次成功发布前可能没有正式包。
- Lynx 仓库的 Android/iOS Release 包含 Go 后端和本地模式入口；主仓库不构建或发布客户端应用。
- [客户端概览](player-lynx/index.md) · [安装](player-lynx/installation.md) · [构建](player-lynx/build-and-run.md) · [测试](player-lynx/testing.md) · [发版](player-lynx/releasing.md) · [贡献](player-lynx/contributing.md)。

| 平台                     | 产物                                             | 使用边界                                                |
| ------------------------ | ------------------------------------------------ | ------------------------------------------------------- |
| Android 6.0+             | `songloft-lynx-android.apk` | Lynx 包含本地后端；覆盖升级需相同签名与较新构建号 |
| iOS 15+                  | `songloft-lynx-ios-nosign.ipa` | 未签名，安装前需自行重签；Live Activity 需 iOS 16.2+ |
| HarmonyOS NEXT / API 13+ | `songloft-lynx-harmony.hap`                      | 实验性；签名 profile 决定可安装设备范围，全屏视频未实现 |
| Web                      | `songloft-lynx-web-{standalone,embedded}.tar.gz` | 需连接服务器；当前核心流程支持普通 HTTP；无 DLNA 或单曲离线缓存 |

每个 Release 同时提供 `version.json` 与 `checksums.txt`。dev 下载包同样是 Release 构建，不开放 TCP/E2E 测试桥。

Web standalone 显示服务器地址输入；embedded 使用同源服务器并隐藏该输入。两者都需要正确的 MIME；当前异步引擎不强制 SAB 或隔离头。根路径与子路径已验证，旧浏览器、插件和长后台覆盖边界请参阅 [Web 指南](player-lynx/web-deployment.md)。

## Songloft Desktop（Electron 桌面客户端）

[Songloft Desktop](https://github.com/songloft-org/songloft-desktop) 是 Songloft 官方 Electron 桌面客户端，面向 Windows、macOS 和 Linux。需连接 Songloft 服务端，不含 Bundle 本地后端。

- [下载与安装包](https://github.com/songloft-org/songloft-desktop/releases/latest)：各平台安装方式以仓库说明和 Release 产物为准。
- [项目说明与源码](https://github.com/songloft-org/songloft-desktop) · [问题反馈](https://github.com/songloft-org/songloft-desktop/issues)。

## Songloft TV（TV / 大屏客户端）

[Songloft TV](https://github.com/songloft-org/songloft-tv) 使用 Kotlin + Jetpack Compose for TV，支持 Android 5.0（API 21）及以上的电视、盒子及横屏大屏设备。需连接已有的 Songloft 服务端，不含 Bundle 本地后端。

- [正式版 APK 下载](https://github.com/songloft-org/songloft-tv/releases/latest) · [开发版下载](https://github.com/songloft-org/songloft-tv/releases/tag/dev)。
- [构建与安装说明](https://github.com/songloft-org/songloft-tv#构建) · [源码](https://github.com/songloft-org/songloft-tv) · [问题反馈](https://github.com/songloft-org/songloft-tv/issues)。
- 支持遥控器 D-Pad 焦点导航与触屏、手机扫码配置服务器、音乐 / MV 播放、原唱 / 伴奏切换、同步与逐字歌词。
- 提供 K 歌模式和手机扫码点歌、收藏歌曲 / 电台、播放缓存、后台播放及设置页更新检查。
- 播放统计需服务端安装播放统计插件；未安装时自动隐藏对应页面。

设备兼容性与安装要求以 TV 仓库说明为准；车载使用边界参阅该仓库的[声明与免责](https://github.com/songloft-org/songloft-tv#声明与免责)。

## 第三方客户端

更多社区客户端见[快速开始中的第三方客户端](quick-start.md#🌐-第三方客户端)。

## 发布与文档维护

Lynx 在自己的仓库使用 `pnpm run release patch --dry-run` 审核发版计划，实际发版同步版本并原子推送 main 与版本 tag。无需为客户端发版同步修改服务端版本。

本站的 Lynx 公共指南从 `clients/player-lynx` 子模块自动同步，中文在 `/player-lynx/`、英文在 `/en/player-lynx/`。编辑源仓库、提交推送后，在主工程更新子模块指针；不要手改生成的文档站页面。Flutter 与 Lynx 客户端仓库都从打包启动时主仓库 `main` 的最新提交构建 Go 后端并发布自己的客户端 Bundle 包；主仓库 Release 只发布服务端与容器资产，不发布客户端安装包。
