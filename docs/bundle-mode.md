# Bundle 本地模式

Bundle 将 Go 后端与客户端一起安装，用户不必另外部署服务器。客户端在登录页和设置页切换本地/远程模式；Android 与桌面平台首次启用时选择音乐目录，iOS 使用应用 Documents 目录。启动流程在宿主内启动后端、等待健康检查通过，再使用本地管理员凭据建立会话。应用不会在首次启动时额外弹出“本地或远程”选择框。

## 客户端与包体来源

| 客户端 | Bundle 包 | 发布仓库 | 后端源码固定方式 |
|---|---|---|---|
| Flutter | Android 通用 APK `songloft-bundled-android.apk`（ARM64、ARMv7、x86_64）、iOS `songloft-bundled-ios-nosign.ipa`、Linux `songloft-bundled-linux-x64.tar.gz`、Windows `songloft-bundled-windows-x64.zip`、macOS `songloft-bundled-macos.zip` | [正式版](https://github.com/songloft-org/songloft-player/releases/latest) · [开发版](https://github.com/songloft-org/songloft-player/releases/tag/dev) | 打包启动时主仓库 `main` 的最新提交 |
| Lynx | Android `songloft-lynx-android.apk`、iOS `songloft-lynx-ios-nosign.ipa` | [songloft-player-lynx Releases](https://github.com/songloft-org/songloft-player-lynx/releases/latest) | 打包启动时主仓库 `main` 的最新提交 |
| Lynx 桌面 | Windows `*.exe`、macOS arm64/x64 `*.dmg` 与更新 `*.zip` | [正式版](https://github.com/songloft-org/songloft-player-lynx/releases/latest) · [开发版](https://github.com/songloft-org/songloft-player-lynx/releases/tag/dev)，与移动端和 Web 同一工作流发布 | 打包启动时主仓库 `main` 的最新提交 |

Flutter 与 Lynx 各自在客户端仓库编译并发布 Bundle。每次流水线启动时解析一次公开主仓库 `main` 的最新 SHA 和版本。该次运行的所有平台和架构使用同一 SHA，并在后端或安装包元数据中记录实际版本及提交。主仓库 Release 只发布服务端、容器和服务端工具。

主仓库和 `pkg/tag` 依赖仓库目前均为公开仓库，客户端 GitHub Actions 可按固定 SHA 直接检出，不需要个人访问令牌或主仓库写权限。客户端 Release 的写权限只用于发布到各自客户端仓库。

Flutter Bundle 使用独立资产名，更新检查与 Android Bundle 热更读取 Flutter 仓库自己的 Release；标准版和 Bundle 版在同一版本 Release，补丁 manifest 用不同文件名前缀隔离。Lynx Android/iOS/桌面客户端包直接内嵌后端，桌面更新由 Lynx 仓库发布的按架构清单负责。桌面包使用无平台证书构建，macOS 采用 ad hoc 签名。

从主仓库下载安装的旧 Flutter Bundle 仍会检查旧地址。Flutter 仓库首次发布上述 Bundle 资产后，请从该仓库手动下载匹配平台的整包进行迁移。Lynx 移动端沿用原文件名，请查阅对应 Release，确认该版本提供本地模式。

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
