# 🎵 MiMusic 快速使用指南

[![GitHub License](https://img.shields.io/github/license/mimusic-org/mimusic)](https://github.com/mimusic-org/mimusic)
[![Docker Image Version](https://img.shields.io/docker/v/hanxi/mimusic?sort=semver&label=docker%20image)](https://hub.docker.com/r/hanxi/mimusic)
[![Docker Pulls](https://img.shields.io/docker/pulls/hanxi/mimusic)](https://hub.docker.com/r/hanxi/mimusic)
[![GitHub Release](https://img.shields.io/github/v/release/mimusic-org/mimusic)](https://github.com/mimusic-org/mimusic/releases)
[![Visitors](https://api.visitorbadge.io/api/daily?path=mimusic-org%2Fmimusic&label=daily%20visitor&countColor=%232ccce4&style=flat)](https://visitorbadge.io/status?path=mimusic-org%2Fmimusic)
[![Visitors](https://api.visitorbadge.io/api/visitors?path=mimusic-org%2Fmimusic&label=total%20visitor&countColor=%232ccce4&style=flat)](https://visitorbadge.io/status?path=mimusic-org%2Fmimusic)

<p align="center">
  <strong>🎵 面向个人用户的自托管音乐服务器 — 仅管理你合法拥有的音乐</strong>
</p>

> 📣 **品牌迁移预告**：本项目计划在 v2.0 更名为 **Songloft**（项目内核与功能保持不变，仅是品牌升级，旧仓库 / Docker 镜像保留至少一年作 redirect）。当前 `MiMusic` 命名继续维护到 v1.x 全部周期结束。新名相关讨论欢迎在 [Issues](https://github.com/mimusic-org/mimusic/issues) 留言。

<p align="center">
  <a href="https://github.com/mimusic-org/mimusic">🏠 GitHub</a> •
  <a href="https://github.com/mimusic-org/mimusic/releases">📥 下载</a> •
  <a href="https://xdocs.hanxi.cc">📖 文档</a> •
  <a href="https://github.com/mimusic-org/mimusic/issues">💬 问题反馈</a> •
  <a href="https://github.com/mimusic-org/mimusic/issues/2">👥 微信群</a> •
  <a href="https://github.com/mimusic-org/mimusic/issues/6">📸 截图</a>
</p>


## ✨ 核心功能

- 🎵 **本地音乐管理** — 扫描本地目录，自动提取 MP3/FLAC/WAV/APE/OGG/M4A 等格式的封面和元数据
- 🧩 **JS 插件体系** — 基于 QuickJS 沙箱运行，支持权限模型、健康检查、热更新，可自由扩展音源 / 元数据 / 设备控制等能力
- 📱 **跨平台客户端** — Flutter 客户端支持 Android、iOS、macOS、Windows、Linux、Web 六端
- 🌐 **Web 界面** — 完整版内置 Web 前端，开箱即用
- 🔑 **JWT 认证** — 双 Token 机制（Access Token + Refresh Token），支持多设备管理
- 📡 **网络歌曲 & 电台** — 支持添加你已合法获取的网络音频 URL 与电台
- 🔁 **歌单转本地** — 把你**已合法持有的**网络音频 URL 离线落盘到本地音乐库，按歌单分目录、可读文件名命名，转换后回写元数据 / 封面 / 歌词。⚠️ 仅限本人合法持有使用权的内容；下载未授权的版权资源属于侵权，由使用者自行承担责任
- 🔌 **完整 REST API** — 内置 Swagger 文档，方便集成和二次开发
- ⚡ **轻量高效** — Go 编写，CGO-free，无外部依赖，适合 NAS / 树莓派等低功耗设备

## ⚖️ 版权与免责声明

MiMusic 是一款**面向个人用户的自托管工具**，定位为帮助用户管理自己合法拥有的音乐文件。在使用本项目前，请务必阅读并理解以下条款：

- 🚫 **不提供任何音乐内容** — MiMusic 本身不内置、不分发、不存储任何受版权保护的音乐资源，仅是一个供你管理本地音乐库的开源软件
- ✅ **请仅管理合法来源的音乐** — 用户应仅使用 MiMusic 管理自己合法获得的音乐文件，包括但不限于：
  - 自行购买并下载的数字音乐
  - 从实体唱片转录的个人备份
  - 自己创作或录制的作品
  - 公有领域（Public Domain）作品
  - 明确以 CC（Creative Commons）等开源协议授权的作品
- 🔌 **第三方插件免责** — JS 插件由第三方社区维护，**主仓库不预置、不分发任何第三方音源插件成品**。插件接入的任何网络音源、元数据、歌词内容版权均归原权利人所有。**使用网络音源、歌单转本地等功能下载或转存内容时，用户须自行承担版权合规责任**，并遵守所在国家 / 地区的法律法规
- 🏠 **仅供个人非商业使用** — 严禁将本项目用于商业用途、对外公开分发版权内容，或搭建面向不特定多数人的公共服务
- ⚠️ **责任自担** — 因不当使用本项目（包括但不限于侵犯第三方版权）所引发的任何法律责任、纠纷或损失，均由使用者本人承担，本项目作者及贡献者不承担任何责任
- ™️ **商标声明** — 本项目及内置插件中提到的所有品牌、协议、产品名称（包括但不限于「MIoT」「Bluetooth」「Android」「iOS」「macOS」「Windows」「Docker」等）均归各自商标权人所有。相关名称的出现仅出于互操作和指示性合理使用目的，**MiMusic 与上述商标持有人无任何关联，也未获得任何形式的授权或背书**。详见 [NOTICE](./NOTICE)
- 🔒 **隐私** — MiMusic 服务端**不内置任何遥测**，所有数据保存在你本地。详见 [PRIVACY.md](./PRIVACY.md)

> 💡 我们尊重并支持知识产权保护。如果你喜欢某位艺术家的作品，请通过正版渠道购买或订阅以支持创作者。

## 📋 版本说明

MiMusic 提供两种版本，满足不同使用场景：

| 版本 | 后缀 | 说明 | 适用场景 |
|------|------|------|----------|
| 🌟 **完整版** | `-full` | 包含 Web 前端，开箱即用 | 推荐初次使用，访问服务地址即可看到 Web 界面 |
| 📦 **精简版** | 无后缀 | 不包含 Web 前端，体积更小 | 配合 Flutter 桌面/移动客户端，或前后端分离部署 |

> 💡 **推荐**：初次使用建议下载 **完整版（-full）**，开箱即用，无需额外配置前端。

## 🖥️ 平台支持

### 📦 二进制文件

#### 🌟 完整版（推荐）

包含 Web 前端，开箱即用：

| 平台 | 架构 | 下载链接 |
|------|------|--------|
| 🐧 Linux | x86_64 | [mimusic-linux-amd64-full](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-linux-amd64-full) |
| 🐧 Linux | ARM64 | [mimusic-linux-arm64-full](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-linux-arm64-full) |
| 🐧 Linux | ARMv7 | [mimusic-linux-armv7-full](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-linux-armv7-full) |
| 🍎 macOS | x86_64 (Intel) | [mimusic-darwin-amd64-full](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-darwin-amd64-full) |
| 🍎 macOS | ARM64 (Apple Silicon) | [mimusic-darwin-arm64-full](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-darwin-arm64-full) |
| 🪟 Windows | x86_64 | [mimusic-windows-amd64-full.exe](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-windows-amd64-full.exe) |
| 🪟 Windows | ARM64 | [mimusic-windows-arm64-full.exe](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-windows-arm64-full.exe) |

#### 📦 精简版（Lite）

不包含 Web 前端，体积更小：

| 平台 | 架构 | 下载链接 |
|------|------|--------|
| 🐧 Linux | x86_64 | [mimusic-linux-amd64](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-linux-amd64) |
| 🐧 Linux | ARM64 | [mimusic-linux-arm64](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-linux-arm64) |
| 🐧 Linux | ARMv7 | [mimusic-linux-armv7](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-linux-armv7) |
| 🍎 macOS | x86_64 (Intel) | [mimusic-darwin-amd64](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-darwin-amd64) |
| 🍎 macOS | ARM64 (Apple Silicon) | [mimusic-darwin-arm64](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-darwin-arm64) |
| 🪟 Windows | x86_64 | [mimusic-windows-amd64.exe](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-windows-amd64.exe) |
| 🪟 Windows | ARM64 | [mimusic-windows-arm64.exe](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-windows-arm64.exe) |

### 🐳 Docker 镜像

#### 🌟 完整版（推荐）

| 平台 | 下载链接 |
|------|--------|
| 🐧 Linux x86_64 | [mimusic-docker-linux-amd64-full.tar](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-docker-linux-amd64-full.tar) |
| 🐧 Linux ARM64 | [mimusic-docker-linux-arm64-full.tar](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-docker-linux-arm64-full.tar) |
| 🐧 Linux ARMv7 | [mimusic-docker-linux-armv7-full.tar](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-docker-linux-armv7-full.tar) |

#### 📦 精简版（Lite）

| 平台 | 下载链接 |
|------|--------|
| 🐧 Linux x86_64 | [mimusic-docker-linux-amd64.tar](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-docker-linux-amd64.tar) |
| 🐧 Linux ARM64 | [mimusic-docker-linux-arm64.tar](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-docker-linux-arm64.tar) |
| 🐧 Linux ARMv7 | [mimusic-docker-linux-armv7.tar](https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-docker-linux-armv7.tar) |

### 📱 Flutter 客户端

除了 Web 界面，MiMusic 还提供功能更强大的跨平台 Flutter 客户端，支持后台播放、本地缓存、媒体控制（耳机/锁屏/通知栏）等服务端 Web 界面无法实现的能力，覆盖 iOS、Android、macOS、Windows、Linux 和 Web 六端。

🔗 **GitHub 仓库**：[mimusic-org/mimusic-player](https://github.com/mimusic-org/mimusic-player)
📥 **下载**：[GitHub Releases](https://github.com/mimusic-org/mimusic-player/releases/latest)

> 💡 使用 **精简版** 服务端时，推荐直接搭配 Flutter 客户端使用（无需额外部署 Web 前端）；如确实需要独立 Web 前端，可参考 [mimusic-player](https://github.com/mimusic-org/mimusic-player) 仓库的 `flutter build web` 流程自行构建并由 Nginx 等反向代理静态托管。

## 🚀 快速开始

> 🔐 **安全提示（必读）**：默认管理员账号是 `admin / admin`，**仅适用于本地测试**。任何对外网暴露或多设备访问的部署，请务必通过环境变量 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 设置强密码后再启动；否则你的音乐库可能被陌生人访问。

### 📦 方式一：直接运行二进制文件

#### 🐧 Linux / 🍎 macOS

```bash
# 1️⃣ 下载对应平台的二进制文件（推荐完整版）
# 例如 Linux x86_64 完整版:
wget https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-linux-amd64-full
mv mimusic-linux-amd64-full mimusic

# 2️⃣ 添加执行权限
chmod +x mimusic

# 3️⃣ 创建必要目录
mkdir -p music data

# 4️⃣ 启动（推荐通过环境变量传入凭证，避免出现在 shell history / 进程列表中）
ADMIN_USERNAME=admin ADMIN_PASSWORD='your_strong_password' ./mimusic
```

> 🍎 **macOS 用户注意**：从 GitHub 下载的二进制带有 Gatekeeper 隔离属性，首次运行会被拦截。运行前请先执行：
> ```bash
> xattr -d com.apple.quarantine ./mimusic
> ```

#### 🪟 Windows

```powershell
# 1️⃣ 下载对应平台的二进制文件（推荐完整版），并重命名为 mimusic.exe
# 例如 Windows x86_64 完整版: mimusic-windows-amd64-full.exe

# 2️⃣ 创建必要目录
mkdir music
mkdir data

# 3️⃣ 设置环境变量并启动（PowerShell）
$env:ADMIN_USERNAME = "admin"
$env:ADMIN_PASSWORD = "your_strong_password"
.\mimusic.exe
```

### 🐳 方式二：Docker 部署

> ⚠️ **镜像标签提醒**：`hanxi/mimusic:latest` 是 **精简版**（不含 Web 前端）。如需开箱即用的 Web 界面，请使用 `hanxi/mimusic:full` 或 `hanxi/mimusic:<version>-full`。

#### 🌐 从 Docker Hub 拉取（推荐）

```bash
# 🌟 完整版（推荐，包含 Web 前端）
docker pull hanxi/mimusic:full
# 或指定版本: docker pull hanxi/mimusic:1.4.1-full

# 📦 精简版（不含 Web 前端，需搭配 Flutter 客户端使用）
docker pull hanxi/mimusic:latest
# 或指定版本: docker pull hanxi/mimusic:1.4.1

# 运行容器（以完整版为例）
docker run -d \
  --name mimusic \
  -p 58091:58091 \
  -v /path/to/music:/app/music \
  -v /path/to/data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD='your_strong_password' \
  hanxi/mimusic:full
```

#### 📥 从 Release 离线导入镜像

适合无法直接访问 Docker Hub 的环境：

```bash
# 1️⃣ 下载对应平台的 Docker 镜像 tar 文件（推荐完整版）
wget https://github.com/mimusic-org/mimusic/releases/latest/download/mimusic-docker-linux-amd64-full.tar

# 2️⃣ 导入镜像
docker load -i mimusic-docker-linux-amd64-full.tar

# 3️⃣ 查看导入的镜像标签
docker images | grep mimusic

# 4️⃣ 使用上一节的 docker run 命令启动即可（注意替换为导入的镜像标签）
```

#### 🐙 Docker Compose 部署（推荐）

使用 Docker Compose 可以更方便地管理容器配置：

```yaml
version: '3.8'

services:
  mimusic:
    image: hanxi/mimusic:full
    container_name: mimusic
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

MiMusic 仅依赖少量启动期配置（凭证、端口、数据库路径）通过环境变量或命令行参数指定，其余业务配置（音乐目录、扫描规则、封面存储等）都保存在数据库 `config` 表中，启动后通过 Web 界面管理。

### 🌍 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ADMIN_USERNAME` | 👤 管理员用户名 | admin |
| `ADMIN_PASSWORD` | 🔐 管理员密码 | admin |
| `LISTEN_PORT` | 🔌 服务端口 | 58091 |
| `DB_PATH` | 💾 数据库文件路径 | data/mimusic.db |

> 📁 Docker 镜像中音乐目录与数据目录固定为 `/app/music` 与 `/app/data`，通过 `-v` 挂载即可，无需额外环境变量。

### 💻 命令行参数

```bash
# 查看帮助
./mimusic -help

# 指定端口
./mimusic -port 8080

# 指定数据库文件路径
./mimusic -db data/mimusic.db

# 指定管理员账号（不推荐，密码会出现在 shell history 和 ps 进程列表中）
./mimusic -username admin -password your_password
```

> ⚙️ **优先级**：命令行参数 **高于** 环境变量。若两者均未提供，则回退到默认值（管理员账号为 `admin/admin`）。
> ⚠️ **参数格式**：MiMusic 使用单横杠参数（如 `-help`），不支持双横杠参数（如 `--help`）。
> 🔐 **密码安全**：推荐通过环境变量 `ADMIN_PASSWORD` 传入密码，避免 `-password` 在进程列表中明文暴露。

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
wget https://github.com/mimusic-org/mimusic/releases/latest/download/checksums.txt

# 验证文件
sha256sum -c checksums.txt
```

## 📌 版本检查

```bash
# 查看版本信息（含 Git Commit / 构建时间 / 构建类型）
./mimusic -version

# 查看完整帮助
./mimusic -help

# 通过 API 检查版本
curl http://localhost:58091/api/v1/version
```

输出示例：

```text
MiMusic Version: 1.4.1
Git Commit: c8f3171
Build Time: 2026-05-28_13:46:11
Build Type: lite
```

## 🔌 插件系统

MiMusic 内置 JS 插件引擎，插件运行在 QuickJS 沙箱中，支持权限模型、健康检查与热更新，可自由扩展音源 / 元数据 / 设备控制等能力。

### 🎯 插件获取

每个插件在自己的 GitHub 仓库下分发：进到对应仓库的 Releases 页下载最新版的 `.jsplugin.zip`，再到 Songloft 客户端的「插件管理」页上传即可启用。可在 [插件合集 Issue](https://xdocs.hanxi.cc/issues/4) 找到当前可用的插件清单。

> 想看更多插件或共建？欢迎在 [插件合集 Issue](https://xdocs.hanxi.cc/issues/4) 留言。

> ⚠️ **版权提示**：第三方插件接入的任何网络音源、歌词、封面等内容，版权均归原权利人所有。请仅将插件用于访问你本人享有合法使用权的内容，下载 / 转存 / 二次分发等行为请遵守所在国家或地区的法律法规。详见上文 [版权与免责声明](#️-版权与免责声明)。

### 🛠️ 插件开发

如需开发自定义插件，请参考以下资源：

- **开发工具链**: [mimusic-org/plugin-toolchain](https://github.com/mimusic-org/plugin-toolchain) — `@mimusic/plugin-sdk` + `@mimusic/plugin-builder` + 脚手架
- **快速开始**: `pnpm create mimusic-plugin <name>`，详见 [JS 插件开发指南](./docs/js-plugin-development-guide.md)

## 📖 API 文档

完整的 API 文档（Swagger/OpenAPI 格式）可在以下地址查看：

- **Swagger API 文档**: [swagger.json](https://github.com/mimusic-org/mimusic/blob/main/docs/swagger.json)
- **Swagger UI 在线查看**: [petstore.swagger.io](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/mimusic-org/mimusic/refs/heads/main/docs/swagger.json)
- **本地查看**: 启动服务后访问 `http://localhost:58091/swagger/index.html`

### 主要接口概览

| 接口组 | 路径 | 说明 |
|--------|------|------|
| 认证 | `/api/v1/auth/*` | 登录、刷新 Token、登出、Token 管理 |
| 歌曲 | `/api/v1/songs/*` | 歌曲 CRUD、封面、播放、歌词 |
| 歌单 | `/api/v1/playlists/*` | 歌单 CRUD、歌单歌曲管理、网络歌曲转本地 |
| JS 插件 | `/api/v1/jsplugins/*` | 插件上传、启用、禁用、删除、检查更新 |
| 扫描 | `/api/v1/scan/*` | 音乐库扫描 |
| 配置 | `/api/v1/configs/*` | 系统配置管理 |
| 版本 | `/api/v1/version` | 版本信息 |

## ❓ 常见问题

遇到问题？请查看 [常见问题与解决方案](https://github.com/mimusic-org/mimusic/issues/1) 💬

## 🛠️ 技术支持

- **GitHub**: [mimusic-org/mimusic](https://github.com/mimusic-org/mimusic)
- **Issues**: [问题与反馈](https://github.com/mimusic-org/mimusic/issues)
- 💬 加入微信群交流：[微信群二维码](https://github.com/mimusic-org/mimusic/issues/2)
- 🐧 QQ群: 979995241

## 📝 更新日志

详细的版本更新记录请查看 [CHANGELOG.md](CHANGELOG.md)。

---

## 📄 许可证

本项目基于 [Apache-2.0 License](LICENSE) 开源。
