# 🎵 Songloft 快速使用指南

<p align="center">
  <strong>简体中文</strong> • <a href="README.en.md">English</a>
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/songloft-org/songloft/main/docs/public/social-preview.png" alt="Songloft" width="640">
</p>

[![GitHub License](https://img.shields.io/github/license/songloft-org/songloft)](https://github.com/songloft-org/songloft)
[![Docker Image Version](https://img.shields.io/docker/v/songloft/songloft?sort=semver&label=docker%20image)](https://hub.docker.com/r/songloft/songloft)
[![Docker Pulls](https://img.shields.io/docker/pulls/songloft/songloft)](https://hub.docker.com/r/songloft/songloft)
[![GitHub Release](https://img.shields.io/github/v/release/songloft-org/songloft)](https://github.com/songloft-org/songloft/releases)
[![Visitors](https://api.visitorbadge.io/api/daily?path=songloft-org%2Fsongloft&label=daily%20visitor&countColor=%232ccce4&style=flat)](https://visitorbadge.io/status?path=songloft-org%2Fsongloft)
[![Visitors](https://api.visitorbadge.io/api/visitors?path=songloft-org%2Fsongloft&label=total%20visitor&countColor=%232ccce4&style=flat)](https://visitorbadge.io/status?path=songloft-org%2Fsongloft)

<p align="center">
  <strong>🎵 面向个人用户的自托管音乐服务器 — 仅管理你合法拥有的音乐</strong>
</p>

> 📣 **关于改名**：本项目自 v2.0 起由 **MiMusic** 更名为 **Songloft**（内核与功能不变，仅品牌升级）。老的 `mimusic-org` GitHub 组织与 `hanxi/mimusic` Docker 镜像保留至少一年作跳转，但不再更新。详见 [GitHub Releases](https://github.com/songloft-org/songloft/releases)。

<p align="center">
  <a href="https://github.com/songloft-org/songloft">🏠 GitHub</a> •
  <a href="https://github.com/songloft-org/songloft/releases">📥 下载</a> •
  <a href="https://songloft.hanxi.cc">📖 文档</a> •
  <a href="https://github.com/songloft-org/songloft/issues">💬 问题反馈</a> •
  <a href="https://github.com/songloft-org/songloft/issues/2">👥 微信群</a> •
  <a href="#screenshots">📸 截图</a>
</p>

> ### 💚 纯为爱发电 · 谨防诈骗
>
> Songloft 是**完全免费**的开源项目，纯粹出于兴趣与热爱开发，与任何商业利益无关：
>
> - ✅ **永久免费** —— 没有任何收费功能、内购或会员
> - ✅ **无广告** —— 不含任何广告或商业推广
> - ✅ **不接受赞助 / 捐款** —— 官方不设任何收款渠道
> - ⚠️ **谨防诈骗** —— 官方渠道仅有 [GitHub 仓库](https://github.com/songloft-org/songloft)、[文档站](https://songloft.hanxi.cc) 与 [官方微信群](https://github.com/songloft-org/songloft/issues/2)。任何以 Songloft 名义收费、售卖付费版本、代部署收费或索要赞助的行为都与本项目无关；即便在群内，我们也从不向任何人收取任何费用，请勿上当受骗。


## ✨ 核心功能

- 🎵 **本地音乐管理** — 扫描本地目录，自动提取 MP3/FLAC/WAV/APE/OGG/Opus/M4A/M4B/WMA/AIF/AIFF/MKA 等格式的封面和元数据
- 🎬 **视频支持** — 扫描 MP4/MOV/M4V/MKV/WebM/AVI/TS/MPG/MPEG/FLV/WMV/RM/RMVB/3GP 等视频容器并探测真实视频轨，客户端内渲染画面
- 🧩 **JS 插件体系** — 基于 QuickJS 沙箱运行，支持权限模型、健康检查、热更新，可自由扩展音源 / 元数据 / 设备控制等能力
- 📱 **跨平台客户端** — Flutter 客户端支持 Android、iOS、macOS、Windows、Linux、Web 六端
- 📦 **Bundle 本地模式** — 客户端内嵌 Go 后端，无需部署服务器，手机/电脑直接播放本地音乐
- 📺 **Kodi 插件客户端** — 支持 Xbox、Apple TV、树莓派、Android TV 等大屏设备，专为遥控器操作优化，带来流畅的客厅影音体验
- 🌐 **Web 界面** — 完整版内置 Web 前端，开箱即用
- 🔑 **JWT 认证** — 双 Token 机制（Access Token + Refresh Token），支持多设备管理
- 📡 **网络歌曲 & 电台** — 支持添加网络音频 URL 与电台流，播放时透明缓存到服务端
- 🔌 **完整 REST API** — 内置 Swagger 文档，方便集成和二次开发
- ⚡ **轻量高效** — Go 编写，CGO-free，无外部依赖，适合 NAS / 树莓派等低功耗设备

<a id="screenshots"></a>

## 🖼️ 界面截图

<p align="center">
  <img src="https://raw.githubusercontent.com/songloft-org/songloft/main/docs/public/screenshots/home-desktop.png" alt="首页 · 桌面端" width="600">
</p>
<p align="center">
  <img src="https://raw.githubusercontent.com/songloft-org/songloft/main/docs/public/screenshots/player-mobile.png" alt="沉浸式播放器 · 移动端" width="240">
</p>

> 📸 更多界面（歌曲库、歌单、设置等，含桌面 / 移动端与亮 / 暗双主题）见 **[界面截图一览](docs/screenshots.md)**。

## ⚖️ 版权与免责声明

Songloft 是一款**面向个人用户的自托管工具**，定位为帮助用户管理自己合法拥有的音乐文件。在使用本项目前，请务必阅读并理解以下条款：

- 🚫 **不提供任何音乐内容** — Songloft 本身不内置、不分发、不存储任何受版权保护的音乐资源，仅是一个供你管理本地音乐库的开源软件
- ✅ **请仅管理合法来源的音乐** — 用户应仅使用 Songloft 管理自己合法获得的音乐文件，包括但不限于：
  - 自行购买并下载的数字音乐
  - 从实体唱片转录的个人备份
  - 自己创作或录制的作品
  - 公有领域（Public Domain）作品
  - 明确以 CC（Creative Commons）等开源协议授权的作品
- 🔌 **第三方插件免责** — JS 插件由第三方社区维护，**主仓库不预置、不分发任何第三方音源插件成品**。插件接入的任何网络音源、元数据、歌词内容版权均归原权利人所有。**使用网络音源等功能时，用户须自行承担版权合规责任**，并遵守所在国家 / 地区的法律法规
- 🏠 **仅供个人非商业使用** — 严禁将本项目用于商业用途、对外公开分发版权内容，或搭建面向不特定多数人的公共服务
- ⚠️ **责任自担** — 因不当使用本项目（包括但不限于侵犯第三方版权）所引发的任何法律责任、纠纷或损失，均由使用者本人承担，本项目作者及贡献者不承担任何责任
- 📩 **侵权举报** — 如果你是版权持有人，认为本项目代码、文档或官方分发的插件侵犯了你的合法权益，请通过 [GitHub Issues](https://github.com/songloft-org/songloft/issues) 或邮件 (im.hanxi@gmail.com) 联系我们，我们将在核实后及时处理。对于第三方社区插件的侵权问题，请直接联系该插件的维护者
- ™️ **商标声明** — 本项目及内置插件中提到的所有品牌、协议、产品名称（包括但不限于「MIoT」「Bluetooth」「Android」「iOS」「macOS」「Windows」「Docker」等）均归各自商标权人所有。相关名称的出现仅出于互操作和指示性合理使用目的，**Songloft 与上述商标持有人无任何关联，也未获得任何形式的授权或背书**。详见 [NOTICE](./NOTICE)
- 🔒 **隐私** — Songloft 服务端**不内置任何遥测**，所有数据保存在你本地。详见 [PRIVACY.md](./PRIVACY.md)

> 💡 我们尊重并支持知识产权保护。如果你喜欢某位艺术家的作品，请通过正版渠道购买或订阅以支持创作者。

## 📋 版本说明

Songloft 提供三种版本，满足不同使用场景：

| 版本 | 后缀 | 说明 | 适用场景 |
|------|------|------|----------|
| 🌟 **完整版** | 无后缀 | 包含 Web 前端，开箱即用 | 推荐初次使用，访问服务地址即可看到 Web 界面 |
| 📦 **精简版** | `-lite` | 不包含 Web 前端，体积更小 | 配合 Flutter 桌面/移动客户端，或前后端分离部署 |
| 📱 **Bundle 版** | `bundled-` | Flutter 客户端内嵌 Go 后端 | 无需部署服务器，手机/电脑直接播放本地音乐 |

> 💡 **推荐**：初次使用直接下载默认的 **完整版**，开箱即用，无需额外配置前端。
> 想在手机或电脑上独立使用，无需部署服务器？可下载 **Bundle 版**。

## 🖥️ 平台支持

### 📦 二进制文件

#### 🌟 完整版（推荐）

包含 Web 前端，开箱即用：

| 平台 | 架构 | 下载链接 |
|------|------|--------|
| 🐧 Linux | x86_64 | [songloft-linux-amd64](https://github.com/songloft-org/songloft/releases/latest/download/songloft-linux-amd64) |
| 🐧 Linux | ARM64 | [songloft-linux-arm64](https://github.com/songloft-org/songloft/releases/latest/download/songloft-linux-arm64) |
| 🐧 Linux | ARMv7 | [songloft-linux-armv7](https://github.com/songloft-org/songloft/releases/latest/download/songloft-linux-armv7) |
| 🍎 macOS | x86_64 (Intel) | [songloft-darwin-amd64](https://github.com/songloft-org/songloft/releases/latest/download/songloft-darwin-amd64) |
| 🍎 macOS | ARM64 (Apple Silicon) | [songloft-darwin-arm64](https://github.com/songloft-org/songloft/releases/latest/download/songloft-darwin-arm64) |
| 🪟 Windows | x86_64 | [songloft-windows-amd64.exe](https://github.com/songloft-org/songloft/releases/latest/download/songloft-windows-amd64.exe) |
| 🪟 Windows | ARM64 | [songloft-windows-arm64.exe](https://github.com/songloft-org/songloft/releases/latest/download/songloft-windows-arm64.exe) |

#### 📦 精简版（Lite）

不包含 Web 前端，体积更小：

| 平台 | 架构 | 下载链接 |
|------|------|--------|
| 🐧 Linux | x86_64 | [songloft-linux-amd64-lite](https://github.com/songloft-org/songloft/releases/latest/download/songloft-linux-amd64-lite) |
| 🐧 Linux | ARM64 | [songloft-linux-arm64-lite](https://github.com/songloft-org/songloft/releases/latest/download/songloft-linux-arm64-lite) |
| 🐧 Linux | ARMv7 | [songloft-linux-armv7-lite](https://github.com/songloft-org/songloft/releases/latest/download/songloft-linux-armv7-lite) |
| 🍎 macOS | x86_64 (Intel) | [songloft-darwin-amd64-lite](https://github.com/songloft-org/songloft/releases/latest/download/songloft-darwin-amd64-lite) |
| 🍎 macOS | ARM64 (Apple Silicon) | [songloft-darwin-arm64-lite](https://github.com/songloft-org/songloft/releases/latest/download/songloft-darwin-arm64-lite) |
| 🪟 Windows | x86_64 | [songloft-windows-amd64-lite.exe](https://github.com/songloft-org/songloft/releases/latest/download/songloft-windows-amd64-lite.exe) |
| 🪟 Windows | ARM64 | [songloft-windows-arm64-lite.exe](https://github.com/songloft-org/songloft/releases/latest/download/songloft-windows-arm64-lite.exe) |

#### 📱 Bundle 版（内嵌后端，无需服务器）

客户端内嵌 Go 后端，首次启动点击「使用本地模式」选择音乐目录即可使用：

| 平台 | 架构 | 下载链接 |
|------|------|--------|
| 🤖 Android | ARM64 + ARMv7 | [songloft-bundled-android-arm64-v8a.apk](https://github.com/songloft-org/songloft/releases/latest/download/songloft-bundled-android-arm64-v8a.apk) |
| 🐧 Linux | x86_64 | [songloft-bundled-linux-x64.tar.gz](https://github.com/songloft-org/songloft/releases/latest/download/songloft-bundled-linux-x64.tar.gz) |
| 🍎 macOS | ARM64 (Apple Silicon) | [songloft-bundled-macos-arm64.dmg](https://github.com/songloft-org/songloft/releases/latest/download/songloft-bundled-macos-arm64.dmg) |
| 🪟 Windows | x86_64 | [songloft-bundled-windows-x64.zip](https://github.com/songloft-org/songloft/releases/latest/download/songloft-bundled-windows-x64.zip) |
| 🍎 iOS | ARM64 | [songloft-bundled-ios-arm64.ipa](https://github.com/songloft-org/songloft/releases/latest/download/songloft-bundled-ios-arm64.ipa) |

### 🐳 Docker 镜像

#### 🌟 完整版（推荐）

| 平台 | 下载链接 |
|------|--------|
| 🐧 Linux x86_64 | [songloft-docker-linux-amd64.tar](https://github.com/songloft-org/songloft/releases/latest/download/songloft-docker-linux-amd64.tar) |
| 🐧 Linux ARM64 | [songloft-docker-linux-arm64.tar](https://github.com/songloft-org/songloft/releases/latest/download/songloft-docker-linux-arm64.tar) |
| 🐧 Linux ARMv7 | [songloft-docker-linux-arm-v7.tar](https://github.com/songloft-org/songloft/releases/latest/download/songloft-docker-linux-arm-v7.tar) |

#### 📦 精简版（Lite）

| 平台 | 下载链接 |
|------|--------|
| 🐧 Linux x86_64 | [songloft-docker-linux-amd64-lite.tar](https://github.com/songloft-org/songloft/releases/latest/download/songloft-docker-linux-amd64-lite.tar) |
| 🐧 Linux ARM64 | [songloft-docker-linux-arm64-lite.tar](https://github.com/songloft-org/songloft/releases/latest/download/songloft-docker-linux-arm64-lite.tar) |
| 🐧 Linux ARMv7 | [songloft-docker-linux-arm-v7-lite.tar](https://github.com/songloft-org/songloft/releases/latest/download/songloft-docker-linux-arm-v7-lite.tar) |

### 📱 Flutter 客户端

除了 Web 界面，Songloft 还提供功能更强大的跨平台 Flutter 客户端，支持后台播放、本地缓存、媒体控制（耳机/锁屏/通知栏）等服务端 Web 界面无法实现的能力，覆盖 iOS、Android、macOS、Windows、Linux 和 Web 六端。

🔗 **GitHub 仓库**：[songloft-org/songloft-player](https://github.com/songloft-org/songloft-player)

- **标准版**（需连接服务器）：[songloft-player Releases](https://github.com/songloft-org/songloft-player/releases/latest)
- **Bundle 版**（内嵌后端，无需服务器）：[songloft Releases](https://github.com/songloft-org/songloft/releases/latest)（下载 `songloft-bundled-*` 文件）

> 🪟 **Windows 用户**：可通过 [Scoop](https://scoop.sh) 一键安装与更新，该 Bucket 提供三个应用，可按需选择：
> ```powershell
> scoop bucket add songloft https://github.com/songloft-org/songloft-scoop
> scoop install songloft-player    # Flutter 客户端（GUI，需连接服务器）
> scoop install songloft-server    # 服务端（CLI，仅命令行程序）
> scoop install songloft-bundled   # Bundle 版（服务端 + 客户端一体化，无需单独部署服务器）
> # 更新：scoop update songloft-player（其余同理）
> # 卸载：scoop uninstall songloft-player（加 --purge 一并删除配置数据）
> ```
> ⚠️ `songloft-bundled` 与 `songloft-server`、`songloft-player` 存在文件冲突，**不能同时安装**；`songloft-server` 与 `songloft-player` 可以共存。

> 💡 **Bundle 版使用方式**：首次启动在登录页点击「使用本地模式」→ 选择音乐目录 → 自动完成。支持随时在设置页切换本地/远程模式。

> 💡 使用 **精简版（-lite）** 服务端时，推荐直接搭配 Flutter 客户端使用（无需额外部署 Web 前端）；如确实需要独立 Web 前端，可参考 [songloft-player](https://github.com/songloft-org/songloft-player) 仓库的 `flutter build web` 流程自行构建并由 Nginx 等反向代理静态托管。

### 📺 TV 客户端

除 Flutter 客户端外，TV 端推荐使用专门的 **[songloft-tv](https://github.com/boluofan/songloft-tv)** 客户端，专为 Android TV 设计，支持遥控器操作。

### 📺 Kodi 插件

除 Flutter 客户端外，Songloft 还提供官方 **Kodi 插件**，让你在 Kodi 媒体中心中直接播放 Songloft 音乐库。适合 **Xbox**、Apple TV、树莓派、Android TV 等支持 Kodi 的大屏设备，专为遥控器操作优化，带来流畅的客厅影音体验。

🔗 **GitHub 仓库**：[songloft-org/plugin.audio.songloft](https://github.com/songloft-org/plugin.audio.songloft)
📥 **下载**：[GitHub Releases](https://github.com/songloft-org/plugin.audio.songloft/releases/latest)

### 📡 路由器 / 光猫部署

路由器与光猫是家庭中常见的基础网络设备，天然支持 24 小时不间断运行，非常适合用于部署 Songloft。社区项目 **songloft-for-router** 提供了面向 OpenWrt、Entware 与梅林（Merlin，含 SWRTdev / Koolshare 软件中心）三类主流路由器固件生态的插件安装包，帮助你在路由器 / 光猫上轻松管理 Songloft 服务。

🔗 **GitHub 仓库**：[songloft-org/songloft-for-router](https://github.com/songloft-org/songloft-for-router)

## 🚀 快速开始

> 🔐 **安全提示（必读）**：默认管理员账号是 `admin / admin`，**仅适用于本地测试**。任何对外网暴露或多设备访问的部署，请务必通过环境变量 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 设置强密码后再启动；否则你的音乐库可能被陌生人访问。

### 📦 方式一：直接运行二进制文件

#### 🐧 Linux / 🍎 macOS

```bash
# 1️⃣ 下载对应平台的二进制文件（默认即完整版）
# 例如 Linux x86_64:
wget https://github.com/songloft-org/songloft/releases/latest/download/songloft-linux-amd64
mv songloft-linux-amd64 songloft

# 2️⃣ 添加执行权限
chmod +x songloft

# 3️⃣ 创建必要目录
mkdir -p music data

# 4️⃣ 启动（推荐通过环境变量传入凭证，避免出现在 shell history / 进程列表中）
ADMIN_USERNAME=admin ADMIN_PASSWORD='your_strong_password' ./songloft
```

> 🍎 **macOS 用户注意**：从 GitHub 下载的二进制带有 Gatekeeper 隔离属性，首次运行会被拦截。运行前请先执行：
> ```bash
> xattr -d com.apple.quarantine ./songloft
> ```

#### 🪟 Windows

```powershell
# 1️⃣ 下载对应平台的二进制文件（默认即完整版），并重命名为 songloft.exe
# 例如 Windows x86_64: songloft-windows-amd64.exe

# 2️⃣ 创建必要目录
mkdir music
mkdir data

# 3️⃣ 设置环境变量并启动（PowerShell）
$env:ADMIN_USERNAME = "admin"
$env:ADMIN_PASSWORD = "your_strong_password"
.\songloft.exe
```

### 🐳 方式二：Docker 部署

#### 🌐 从 Docker Hub 拉取（推荐）

```bash
# 🌟 完整版（推荐，包含 Web 前端，:latest 即完整版）
docker pull songloft/songloft:latest

# 📦 精简版（不含 Web 前端，需搭配 Flutter 客户端使用）
docker pull songloft/songloft:lite

# 运行容器
docker run -d \
  --name songloft \
  -p 58091:58091 \
  -v /path/to/music:/app/music \
  -v /path/to/data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD='your_strong_password' \
  -e PUID=$(id -u) -e PGID=$(id -g) \
  songloft/songloft:latest
```

> 👤 **非 root 运行（可选）**：加上 `-e PUID=$(id -u) -e PGID=$(id -g)` 后，容器内主程序会以宿主机当前用户身份运行，挂载目录下新产生的文件不再是 root 属主。不设置则保持容器默认的 root 运行方式，不影响现有部署。

#### 📥 从 Release 离线导入镜像

适合无法直接访问 Docker Hub 的环境：

```bash
# 1️⃣ 下载对应平台的 Docker 镜像 tar 文件（默认即完整版）
wget https://github.com/songloft-org/songloft/releases/latest/download/songloft-docker-linux-amd64.tar

# 2️⃣ 导入镜像
docker load -i songloft-docker-linux-amd64.tar

# 3️⃣ 查看导入的镜像标签
docker images | grep songloft

# 4️⃣ 使用上一节的 docker run 命令启动即可（注意替换为导入的镜像标签）
```

#### 🐙 Docker Compose 部署（推荐）

使用 Docker Compose 可以更方便地管理容器配置：

```yaml
version: '3.8'

services:
  songloft:
    image: songloft/songloft:latest
    container_name: songloft
    restart: always
    ports:
      - "58091:58091"
    volumes:
      - /path/to/music:/app/music
      - /path/to/data:/app/data
    environment:
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=your_strong_password
      - LISTEN_PORT=58091
      - PUID=1000 # 可选：非 root 运行，改成宿主机对应用户的 uid
      - PGID=1000 # 可选：非 root 运行，改成宿主机对应用户的 gid
```

将上述内容保存为 `docker-compose.yml` 文件，然后运行：

```bash
# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

### 🏠 方式三：Home Assistant 加载项

如果你在使用 Home Assistant OS（HAOS），可以把 Songloft 作为**加载项（Add-on）**一键安装，无需手动写 `docker run`。

[![添加仓库到你的 Home Assistant。](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fsongloft-org%2Fhome-assistant-addon)

点击上方徽章会直接在你的 HA 里弹出「添加加载项仓库」对话框；或手动操作：

1. 「设置 → 加载项 → 加载项商店」，右上角菜单「仓库」，添加：`https://github.com/songloft-org/home-assistant-addon`
2. 刷新后在商店里找到 **Songloft**，点击安装
3. 到「配置」页填写管理员账号密码、音乐库路径（默认 `/media`），然后启动
4. 点加载项详情页的「打开 Web UI」访问

音乐文件放进 Home Assistant 的媒体目录（`/media`）或共享目录（`/share`）即可被扫描。数据持久化在加载项的 `/data` 目录，卸载重装不丢。

> 🔐 **安全提示**：默认账号 `admin/admin` 仅适用于本地测试，任何对外访问的部署请先在配置页设置强密码再启动。

## 📋 使用流程

### 1️⃣ 启动服务

服务启动后，访问 `http://localhost:58091` 即可打开 Web 界面（仅完整版内置；精简版请使用 [Flutter 客户端](#-flutter-客户端) 连接）。

### 2️⃣ 登录系统

使用配置的管理员账号密码登录。

### 3️⃣ 配置音乐目录

首次登录后，进入「设置」页面配置音乐文件目录（`music_path`）。Docker 部署时通常配置为 `/app/music`。

### 4️⃣ 扫描音乐

在 Web 界面中点击"扫描"按钮，系统会自动扫描音乐目录中的音频文件并提取元数据。

### 5️⃣ 播放音乐

扫描完成后，即可在界面中浏览和播放音乐。

## ⚙️ 配置说明

Songloft 仅依赖少量启动期配置（凭证、端口、数据库路径）通过环境变量或命令行参数指定，其余业务配置（音乐目录、扫描规则、封面存储等）都保存在数据库 `config` 表中，启动后通过 Web 界面管理。

### 🌍 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ADMIN_USERNAME` | 👤 管理员用户名 | admin |
| `ADMIN_PASSWORD` | 🔐 管理员密码 | admin |
| `LISTEN_PORT` | 🔌 服务端口 | 58091 |
| `DB_PATH` | 💾 数据库文件路径 | data/songloft.db |
| `BASE_PATH` | 🔗 URL 基础路径（反向代理子路径部署用，如 `/songloft`） | 空（根路径） |
| `MUSIC_DIR` | 🎵 音乐目录（非空时覆盖数据库中的默认值，等价于 `-music` 参数） | 空 |
| `PUID` | 👤 **（仅 Docker）** 设置后以该 uid 非 root 运行主程序，未设置时保持 root 运行 | 空（root） |
| `PGID` | 👤 **（仅 Docker）** 设置后以该 gid 非 root 运行主程序，未设置时保持 root 运行 | 空（root） |
| `FIX_MUSIC_PERMISSIONS` | 🔧 **（仅 Docker）** 设为 `true` 时递归修复 `/app/music` 下所有文件的属主为 `PUID:PGID`（默认仅修复顶层目录，音乐库较大时递归修复可能耗时较长，一般只在升级为非 root 运行后需要用一次） | false |

> 📁 Docker 镜像中音乐目录与数据目录默认为 `/app/music` 与 `/app/data`，通过 `-v` 挂载即可；如需指向其他路径，可用 `MUSIC_DIR` 指定音乐目录。
> 👤 **非 root 运行**：`PUID`/`PGID` 任一设置即启用，未设置的一方默认补 `1000`；`/app/data` 会自动递归修复属主（解决从 root 运行升级过来的历史文件），`/app/music` 默认仅修复顶层目录，如需连同库内历史文件一起修复请设置 `FIX_MUSIC_PERMISSIONS=true`。

### 💻 命令行参数

```bash
# 查看帮助
./songloft -help

# 指定端口
./songloft -port 8080

# 指定数据库文件路径
./songloft -db data/songloft.db

# 指定管理员账号（不推荐，密码会出现在 shell history 和 ps 进程列表中）
./songloft -username admin -password your_password

# 指定子路径（反向代理部署时使用）
./songloft -base-path /songloft
```

> ⚙️ **优先级**：命令行参数 **高于** 环境变量。若两者均未提供，则回退到默认值（管理员账号为 `admin/admin`）。
> ⚠️ **参数格式**：Songloft 使用单横杠参数（如 `-help`），不支持双横杠参数（如 `--help`）。
> 🔐 **密码安全**：推荐通过环境变量 `ADMIN_PASSWORD` 传入密码，避免 `-password` 在进程列表中明文暴露。

## 🌐 反向代理子路径部署

如果你通过 Nginx 等反向代理将 Songloft 挂在子路径下（如 `https://nas.example.com/songloft/`），需要配置 `BASE_PATH` 环境变量。

### 配置步骤

**1. 启动 Songloft 时指定 BASE_PATH：**

```bash
# 环境变量方式
BASE_PATH=/songloft ./songloft

# 或命令行参数方式
./songloft -base-path /songloft

# Docker 方式
docker run -d \
  --name songloft \
  -p 58091:58091 \
  -v /path/to/music:/app/music \
  -v /path/to/data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD='your_strong_password' \
  -e BASE_PATH=/songloft \
  -e PUID=$(id -u) -e PGID=$(id -g) \
  songloft/songloft:latest
```

**2. 配置 Nginx 反向代理：**

```nginx
location /songloft/ {
    proxy_pass http://127.0.0.1:58091;
    proxy_read_timeout 52w;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

> ⚠️ **注意**：`proxy_pass` 末尾**不要**加斜杠。Nginx 会将完整路径（含 `/songloft/`）转发给后端，由 Songloft 自行处理前缀剥离。

### 客户端连接

| 客户端类型 | 服务器地址填写 |
|-----------|--------------|
| 内置 Web 前端 | 浏览器直接访问 `https://nas.example.com/songloft/`，自动工作 |
| Flutter 桌面/移动客户端 | 填写 `https://nas.example.com/songloft` |

### Docker Compose 示例

```yaml
version: '3.8'

services:
  songloft:
    image: songloft/songloft:latest
    container_name: songloft
    restart: always
    ports:
      - "58091:58091"
    volumes:
      - /path/to/music:/app/music
      - /path/to/data:/app/data
    environment:
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=your_strong_password
      - BASE_PATH=/songloft
      - PUID=1000 # 可选：非 root 运行，改成宿主机对应用户的 uid
      - PGID=1000 # 可选：非 root 运行，改成宿主机对应用户的 gid
```

## 💻 系统要求

| 项目 | 要求 |
|------|------|
| **操作系统** | 🐧 Linux / 🍎 macOS / 🪟 Windows |
| **架构** | x86_64 / ARM64 / ARMv7 |
| **可选依赖** | 🔧 ffprobe（用于获取音频技术参数，不安装也可正常运行） |

## ✅ 校验文件完整性

每个 Release 都包含 `checksums.txt` 文件，用于验证下载文件的完整性：

```bash
# 下载校验和文件
wget https://github.com/songloft-org/songloft/releases/latest/download/checksums.txt

# 验证文件
sha256sum -c checksums.txt
```

## 📌 版本检查

```bash
# 查看版本信息（含 Git Commit / 构建时间 / 构建类型）
./songloft -version

# 查看完整帮助
./songloft -help

# 通过 API 检查版本
curl http://localhost:58091/api/v1/version
```

输出示例：

```text
Songloft Version: x.y.z
Git Commit: abc1234
Build Time: 2026-01-01_00:00:00
Build Type: full
```

## 🔌 插件系统

Songloft 内置 JS 插件引擎，插件运行在 QuickJS 沙箱中，支持权限模型、健康检查与热更新，可自由扩展音源 / 元数据 / 设备控制等能力。

### 🎯 插件获取

每个插件在自己的 GitHub 仓库下分发：进到对应仓库的 Releases 页下载最新版的 `.jsplugin.zip`，再到 Songloft 客户端的「插件管理」页上传即可启用。可在 [插件合集 Issue](https://songloft.hanxi.cc/issues/4) 找到当前可用的插件清单。

> 想看更多插件或共建？欢迎在 [插件合集 Issue](https://songloft.hanxi.cc/issues/4) 留言。

> ⚠️ **版权提示**：第三方插件接入的任何网络音源、歌词、封面等内容，版权均归原权利人所有。请仅将插件用于访问你本人享有合法使用权的内容，下载 / 转存 / 二次分发等行为请遵守所在国家或地区的法律法规。详见上文 [版权与免责声明](#️-版权与免责声明)。

### 🛠️ 插件开发

如需开发自定义插件，请参考以下资源：

- **开发工具链**: [songloft-org/plugin-toolchain](https://github.com/songloft-org/plugin-toolchain) — `@songloft/plugin-sdk` + `@songloft/plugin-builder` + 脚手架
- **快速开始**: `pnpm create songloft-plugin <name>`，详见 [JS 插件开发指南](./docs/js-plugin-development-guide.md)

## 📖 API 文档

完整的 API 文档（Swagger/OpenAPI 格式）可在以下地址查看：

- **Swagger API 文档**: [swagger.json](https://github.com/songloft-org/songloft/blob/main/docs/swagger.json)
- **Swagger UI 在线查看**: [petstore.swagger.io](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/songloft-org/songloft/refs/heads/main/docs/swagger.json)
- **本地查看**: 启动服务后访问 `http://localhost:58091/swagger/index.html`

### 主要接口概览

| 接口组 | 路径 | 说明 |
|--------|------|------|
| 认证 | `/api/v1/auth/*` | 登录、刷新 Token、登出、Token 管理 |
| 歌曲 | `/api/v1/songs/*` | 歌曲 CRUD、封面、播放、歌词 |
| 歌单 | `/api/v1/playlists/*` | 歌单 CRUD、歌单歌曲管理、导入导出 |
| JS 插件 | `/api/v1/jsplugins/*` | 插件上传、启用、禁用、删除、检查更新 |
| 扫描 | `/api/v1/scan/*` | 音乐库扫描 |
| 配置 | `/api/v1/configs/*` | 系统配置管理（通用 KV） |
| 设置 | `/api/v1/settings/*` | 业务功能设置（音乐目录、HLS 代理、HTTP 代理、自动扫描等） |
| 缓存管理 | `/api/v1/cache-manage/*` | 缓存统计、清理、配置 |
| 升级 | `/api/v1/upgrade/*` | 版本检查、升级（仅 Docker） |
| 版本 | `/api/v1/version` | 版本信息 |
| 健康检查 | `/api/v1/health` | 服务健康状态 |
| 资源代理 | `/api/v1/proxy` | 资源代理（解决 CORS 问题） |
| 日志 | `/api/v1/logs/export` | 日志导出 |

## ❓ 常见问题

遇到问题？请查看 [常见问题与解决方案](https://songloft.hanxi.cc/faq) 💬

## 🛠️ 技术支持

- **GitHub**: [songloft-org/songloft](https://github.com/songloft-org/songloft)
- **Issues**: [问题与反馈](https://github.com/songloft-org/songloft/issues)
- 💬 加入微信群交流：[微信群二维码](https://github.com/songloft-org/songloft/issues/2)
- 🐧 QQ群: 979995241 (满了可以搜新群)

## 📝 更新日志

详细的版本更新记录请查看 [CHANGELOG.md](CHANGELOG.md)。

---

## 📄 许可证

本项目**源代码**基于 [Apache-2.0 License](LICENSE) 开源；服务端二进制与 Docker 镜像同样按 Apache-2.0 分发。

> **⚠️ 客户端二进制按 GPL-3.0 分发**
>
> Flutter 客户端的**原生平台构建**（Android / iOS / Windows / macOS / Linux，含本仓库发布的 `songloft-bundled-*` Bundle 版）链接了 [WebF](https://github.com/openwebf/webf) 用于渲染 JS 插件页，而 WebF 是 **GPL-3.0-only 且没有链接例外**。Apache-2.0 单向兼容 GPLv3，所以这种组合是允许的 —— 但**组合后的产物（即我们分发的每个客户端二进制）整体受 GPL-3.0 约束**。
>
> 作为客户端二进制的接收方，你据此获得以下权利：
>
> - 你拿到的那份二进制**按 GPL-3.0 而非 Apache-2.0** 授予你；
> - 你有权获取该组合作品的**完整对应源码** —— [本仓库](https://github.com/songloft-org/songloft)（服务端）、[客户端仓库](https://github.com/songloft-org/songloft-player)、[WebF](https://github.com/openwebf/webf)，每个 release 都随附一份说明其构建所用的确切 tag/commit；
> - 你可以自行修改并按 GPL-3.0 的条件再分发。
>
> GPL-3.0 全文见 [LICENSES/GPL-3.0.txt](https://github.com/songloft-org/songloft/blob/main/LICENSES/GPL-3.0.txt)，并随每个 release 一同发布。许可全文同时**内置在客户端安装包里**，无需联网即可在 App 内「设置 → 关于与更新 → 开源许可」查看 GPL-3.0 全文、第三方组件声明与逐个依赖包许可。
>
> **不受影响的部分**：后端（Go 服务端）源码与二进制、Docker 镜像、Web 端客户端构建都**不含 WebF**，仍然只受 Apache-2.0 约束；把客户端源码去掉 WebF 依赖后自行编译，产物同样只受 Apache-2.0 约束。第三方组件清单见 [客户端 NOTICE](https://github.com/songloft-org/songloft-player/blob/main/NOTICE)。
