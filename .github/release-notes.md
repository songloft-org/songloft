## 安装说明（中文）

在本页 **Assets** 中按用途、操作系统和 CPU 架构选择文件：`amd64` / `x64` 为 Intel/AMD 64 位，`arm64` 为 ARM 64 位，`armv7` / `arm-v7` 为 ARM 32 位。

| 用途 | 文件 | 使用方式 |
| --- | --- | --- |
| 服务器 Full | `songloft-<系统>-<架构>`（Windows 带 `.exe`） | 包含 Flutter Web 界面。下载后启动，浏览器访问服务器的 `58091` 端口。 |
| 服务器 Lite | `songloft-<系统>-<架构>-lite`（Windows 带 `.exe`） | 不包含 Web 界面，启动后使用独立客户端连接，或另行部署 Web 前端。 |
| Docker 离线镜像 | `songloft-docker-linux-<架构>.tar` / `*-lite.tar` | 执行 `docker load -i <文件名>`，使用命令输出的镜像名称创建容器；配置 `58091:58091` 端口及 `/app/data`、`/app/music` 挂载。 |
| Bundle 客户端 | `songloft-bundled-*` | Flutter 客户端内嵌后端，无需另行部署服务器。安装后选择「使用本地模式」并配置音乐目录。 |

**直接运行服务器：** Linux/macOS 为文件添加执行权限后运行，例如 `chmod +x songloft-linux-amd64`、`./songloft-linux-amd64`；Windows 在 PowerShell 中运行对应的 `.exe`。首次启动可通过 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 设置管理员账号与密码；默认端口为 `58091`，未设置凭证时初始账号为 `admin/admin`。数据保存在运行目录的 `data/`，升级时保留数据目录。

**Docker 在线安装：** 正式 Full 使用 `songloft/songloft:latest`，正式 Lite 使用 `songloft/songloft:lite`；开发版分别使用 `:dev`、`:dev-lite`。完整启动配置见[中文安装指南](https://github.com/songloft-org/songloft/blob/main/README.md)。

**Bundle 安装包：** Android 选择对应架构的 `songloft-bundled-android-*.apk`；Linux 解压 `songloft-bundled-linux-x64.tar.gz`，Windows 解压 `songloft-bundled-windows-x64.zip`，保留完整目录后运行应用。macOS 解压 `songloft-bundled-macos.zip` 并将应用移动到「应用程序」。iOS 的 `songloft-bundled-ios-nosign.ipa` 未签名，需要自己的开发者证书及设备 provisioning profile 重签后安装。附件以本页实际提供的文件为准。

**下载校验：** 将下载文件与 `checksums.txt` 放在同一目录，Linux 可运行 `sha256sum -c checksums.txt --ignore-missing`；macOS 使用 `shasum -a 256 <文件名>`，Windows 使用 PowerShell `Get-FileHash <文件名> -Algorithm SHA256`，对照清单中的 SHA-256。`version.json`、更新 manifest 和补丁文件供更新器使用，无需作为安装包手动运行。

## Installation (English)

Choose files under **Assets** by purpose, operating system, and CPU architecture: `amd64` / `x64` means Intel/AMD 64-bit, `arm64` means ARM 64-bit, and `armv7` / `arm-v7` means ARM 32-bit.

| Purpose | File | Usage |
| --- | --- | --- |
| Full server | `songloft-<os>-<arch>` (`.exe` on Windows) | Includes the Flutter Web interface. Start the server and open port `58091` in your browser. |
| Lite server | `songloft-<os>-<arch>-lite` (`.exe` on Windows) | Does not include a Web interface. Connect with a separate client or deploy a Web frontend. |
| Offline Docker image | `songloft-docker-linux-<arch>.tar` / `*-lite.tar` | Run `docker load -i <filename>` and create a container using the image name printed by that command. Map port `58091:58091` and mount `/app/data` and `/app/music`. |
| Bundled client | `songloft-bundled-*` | Flutter client with an embedded backend; no separate server required. After installation, choose local mode and configure your music directory. |

**Run the server directly:** On Linux/macOS, make the file executable and run it, for example `chmod +x songloft-linux-amd64` followed by `./songloft-linux-amd64`. On Windows, run the matching `.exe` from PowerShell. Set `ADMIN_USERNAME` / `ADMIN_PASSWORD` for the initial administrator credentials. The default port is `58091`; without custom credentials, a fresh installation uses `admin/admin`. Data is stored in `data/` under the working directory; preserve it when upgrading.

**Online Docker installation:** Use `songloft/songloft:latest` for stable Full and `songloft/songloft:lite` for stable Lite. Development images use `:dev` and `:dev-lite`, respectively. See the [English installation guide](https://github.com/songloft-org/songloft/blob/main/README.en.md) for complete startup configuration.

**Bundled packages:** On Android, choose the matching `songloft-bundled-android-*.apk`. Extract `songloft-bundled-linux-x64.tar.gz` on Linux or `songloft-bundled-windows-x64.zip` on Windows, and launch the app with its complete directory. On macOS, extract `songloft-bundled-macos.zip` and move the app into Applications. The iOS `songloft-bundled-ios-nosign.ipa` is unsigned: re-sign with your developer certificate and a provisioning profile for your device before installing. Check the files actually attached to this release.

**Verify downloads:** Keep downloaded files and `checksums.txt` together. On Linux, run `sha256sum -c checksums.txt --ignore-missing`. On macOS, use `shasum -a 256 <filename>`; on Windows, use PowerShell `Get-FileHash <filename> -Algorithm SHA256`. Compare the result with the listed SHA-256. `version.json`, update manifests, and patch files are used by the updater and are not standalone installers.
