# Bundle 本地模式

Bundle 将 Go 后端与客户端一起安装，用户不必另外部署服务器。客户端在登录页和设置页切换本地/远程模式；Android 与桌面平台首次启用时选择音乐目录，iOS 使用应用 Documents 目录。启动流程在宿主内启动后端、等待健康检查通过，再使用本地管理员凭据建立会话。应用不会在首次启动时额外弹出“本地或远程”选择框。

## 客户端与包体来源

| 客户端 | Bundle 包 | 发布仓库 | 后端源码固定方式 |
|---|---|---|---|
| Flutter | `songloft-bundled-android.apk`、`songloft-bundled-ios-nosign.ipa`、`songloft-bundled-linux-x64.tar.gz`、`songloft-bundled-macos.zip`、`songloft-bundled-windows-x64.zip` | [后端正式版](https://github.com/songloft-org/songloft/releases/latest) · [dev](https://github.com/songloft-org/songloft/releases/tag/dev) | Release 准备阶段固定主仓库与子模块完整 SHA |
| Lynx | `songloft-lynx-bundled-android.apk`、`songloft-lynx-bundled-ios-nosign.ipa`、`songloft-lynx-bundled-win32-x64.exe`、`songloft-lynx-bundled-darwin-arm64.dmg`、`songloft-lynx-bundled-darwin-x64.dmg` / `songloft-lynx-bundled-darwin-arm64.zip` / `songloft-lynx-bundled-darwin-x64.zip` | [后端正式版](https://github.com/songloft-org/songloft/releases/latest) · [dev](https://github.com/songloft-org/songloft/releases/tag/dev) | Release 准备阶段固定主仓库与子模块完整 SHA |
| Lynx 标准版 | Android/iOS、桌面标准包 | [Lynx Release](https://github.com/songloft-org/songloft-player-lynx/releases/latest) · [dev](https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev) | Lynx Release 自己固定后端依赖 |
| Lynx HarmonyOS / Web | HAP 与 Web standalone/embedded | [Lynx Release](https://github.com/songloft-org/songloft-player-lynx/releases/latest) · [dev](https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev) | 非 Bundle 产物 |

[完整包名清单](release-packages.md)。后端仓库配置 `DESKTOP_UPDATE_PUBLIC_KEY` 和 `DESKTOP_UPDATE_PRIVATE_KEY` 后签署桌面更新清单。缺少密钥时，dev 构建仍上传 Bundle 到 Actions，跳过 GitHub Release 发布。Lynx 移动热更新另使用 `LYNX_UPDATE_PUBLIC_KEY` 和 `LYNX_UPDATE_PRIVATE_KEY`。

后端 Release 同时发布 `bundle-index.json`、`bundle-flutter-version.json`、`bundle-flutter-backend-manifest-<abi>.json`、`bundle-lynx-version.json`、`bundle-lynx-version.json.sig` 和 `desktop-bundled-<target>.json`。`bundle-index.json` 的每条记录包含 `client`、`edition`、平台、资产名、完整客户端 SHA、完整后端 SHA、版本、构建时间和 SHA-256。服务端自己的 `version.json` 保留独立名称。

Bundle 构建开始时从 `hanxi/ffmpeg-builder` 的 `dev` 或正式通道解析一个具体 Release，并把 `ffmpeg_release_tag`、`ffmpeg_source_commit` 与 `ffmpeg_manifest_sha256` 写入 Bundle 清单。所有平台复用同一解析记录，下载后再次校验平台、ABI、大小和 SHA-256。

- Android APK 为 arm64-v8a、armeabi-v7a 和 x86_64 携带 FFmpeg 和 FFprobe。Android 将它们作为原生文件安装，后端从应用的 `nativeLibraryDir` 执行它们。Flutter 后端热补丁仍校验 gomobile 接口签名，旧 APK 需要整包升级才能获得内置工具。
- 桌面启动器将包内的 FFmpeg 和 FFprobe 路径传给 Go 后端。Flutter macOS Bundle 包含签名后的工具。
- iOS 将 arm64 FFmpeg 静态链接进 gomobile 后端。元数据、封面和视频帧、文件及流式转码、seek、响度均衡、倍速、电台、HLS 和 Chromaprint 在应用进程内运行。IPA 仍需重签才能安装。构建需要包含静态库、头文件及链接参数的新 FFmpeg Release 资产。

Flutter 标准版继续读取 Flutter Release。Flutter Bundle 读取后端 Release。Lynx 标准版继续读取 Lynx Release。Lynx Bundle 读取后端 Release。桌面更新器按 `variant` 选择对应仓库与签名清单，桌面资产名不包含 `dev` 或 `stable`。

## Android 与 iOS 本地模式

- **Android**：后端通过 gomobile 编译为 `.aar`，由 `SongloftBundle` 原生模块调用。含后端的 Lynx APK 最低需要 Android 6.0。用户通过系统目录选择器选择外部音乐目录并授予读取权限。
- **iOS**：后端通过 gomobile 编译为 `Songloft.xcframework`，由 Swift 模块调用。音乐目录位于应用 Documents 下，供“文件”应用管理；系统目录访问授权不适用。
- 两端共用 Lynx 的模式切换与会话恢复逻辑。后端使用动态端口，端口健康后才建立本地登录会话；管理员密码由本地后端生成并保存在应用数据目录。
- 切换远程服务器时，客户端停止本地会话和后端，再连接用户配置的服务器。选择远程模式不会删除本地曲库数据。

设备目录权限、扫描播放、干净安装和升级按平台统一验收。请通过各客户端 Release 核对已发布的包和对应版本。

## 相关资料

- [客户端选择](clients.md)
- [前端架构 · 部署模式](architecture_frontend.md#deployment-modes)
- [Flutter 客户端 Bundle 说明](player/architecture.md)
- [Lynx 安装指南](player-lynx/installation.md)
- [Lynx 发版指南](player-lynx/releasing.md)
