# Songloft 项目架构说明

Songloft 是一个自托管的本地音乐服务器，采用前后端分离架构。

## 架构文档导航

- **[后端架构](./ARCHITECTURE_BACKEND.md)** - Go 后端 API 服务详细架构
- **[前端架构](./ARCHITECTURE_FRONTEND.md)** - Flutter 跨平台前端详细架构
- **[颜色系统](./COLOR_SYSTEM.md)** - Material 3 颜色体系和主题规范
- **[快速开始](./quick-start.md)** - 快速上手指南（由 README.md 同步生成）

## 整体架构

```
┌──────────────────────────────────────────────────────────────┐
│  Flutter 跨平台前端                                          │
│  /songloft-player (独立仓库: github.com/songloft-org/songloft-player) │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ Android  │ │   iOS    │ │  macOS   │ │ Windows  │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
│  ┌──────────┐ ┌──────────────────────────────────────┐      │
│  │  Linux   │ │  Web (嵌入 Go 二进制 / 独立部署)      │      │
│  └──────────┘ └──────────────────────────────────────┘      │
│  状态: Riverpod  路由: GoRouter  音频: just_audio            │
└──────────────────────────────────────────────────────────────┘
                        │
                   HTTP/REST API
                   JWT Bearer Token
                        │
┌──────────────────────────────────────────────────────────────┐
│                   Go 后端 (Chi v5)                           │
│  ┌──────────┐ ┌──────────┐ ┌────────────────────┐ ┌──────┐ │
│  │ Handlers │→│ Services │→│ Repository/UoW     │→│SQLite│ │
│  └──────────┘ └──────────┘ │  (sqlc + squirrel) │ └──────┘ │
│                            └────────────────────┘          │
│                            goose migrations 启动自动 Up      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                    │
│  │ JSPlugin │ │JS Runtime│ │  Cache   │                    │
│  │ Manager  │ │ (QuickJS)│ │ Service  │                    │
│  └──────────┘ └──────────┘ └──────────┘                    │
│  Static: embed.FS (Flutter Web) + SPA 路由回退              │
│  Monitoring: Tracely (心跳 + 错误上报 + panic 捕获)          │
└──────────────────────────────────────────────────────────────┘
                        │
┌──────────────────────────────────────────────────────────────┐
│              SQLite 数据库 (modernc.org/sqlite)              │
│              纯 Go CGO-free 实现，无需外部依赖               │
└──────────────────────────────────────────────────────────────┘
```

## 技术栈概览

### 后端

| 技术 | 版本 | 说明 |
|------|------|------|
| Go | 1.26+ | 后端语言 |
| Chi | v5.2.4 | HTTP 路由框架 |
| SQLite | modernc.org/sqlite v1.46.1 | 纯 Go 数据库驱动 |
| goose | v3 | SQL schema 迁移（启动时自动 Up） |
| sqlc | - | 固定 SQL 生成类型安全 Go 代码（CLI） |
| squirrel | v1.5 | 动态 SQL 构造（变长 WHERE/SET/ORDER） |
| JWT | golang-jwt/jwt v5 | 双 Token 认证 |
| QuickJS | modernc.org/quickjs | JS 运行时（JS 插件沙盒） |
| hanxi/tag | - | 音频元数据读写 |
| ffprobe | 可选 | 音频技术参数 |
| Tracely | v1.0.19 | 监控和错误上报 |

### Flutter 前端

| 技术 | 版本 | 说明 |
|------|------|------|
| Flutter | 3.29+ | 跨平台 UI 框架 |
| Dart | 3.7+ | 编程语言 |
| Riverpod | ^3.1.0 | 状态管理 |
| GoRouter | ^17.1.0 | 声明式路由 |
| Dio | ^5.7.0 | HTTP 客户端 |
| just_audio | ^0.10.5 | 音频播放引擎 |
| audio_service | ^0.18.17 | 系统通知栏控制 |
| Material 3 | - | UI 设计系统 |

## 项目目录结构

```
songloft/
├── main.go                     # 主程序入口
├── web_embed.go                # 轻量版（空 embed.FS，build tag: !full）
├── web_embed_full.go           # 完整版（嵌入 Flutter Web，build tag: full）
├── Makefile                    # 构建和测试命令
├── go.mod                      # Go 模块定义（Go 1.26）
├── Dockerfile                  # Docker 配置
├── internal/                   # 后端核心代码
│   ├── app/                    # 应用初始化、路由注册、静态文件服务、source 适配
│   ├── config/                 # 配置类型定义
│   ├── handlers/               # HTTP 请求处理器
│   ├── middleware/             # JWT 认证中间件
│   ├── models/                 # 数据模型和常量
│   ├── database/               # SQLite 数据库层（Repository + UnitOfWork + sqlc + goose 迁移 + testutil）
│   ├── services/               # 业务逻辑层（含 source/ 子包：fetcher / resolver / validator / orchestrator / metrics）
│   ├── jsplugin/               # JS 插件管理（生命周期、健康检查、热更新）
│   ├── jsruntime/              # QuickJS JavaScript 运行时
│   └── version/                # 版本信息
├── pkg/                        # 公共包
│   └── tag/                    # 音频元数据读写库
├── songloft-player/             # Flutter 前端（独立子仓库）
│   └── lib/                    # Dart 源码
│       ├── config/             # API 配置、部署模式
│       ├── core/               # 网络、路由、主题、存储、音频
│       ├── features/           # 功能模块（auth / home / library / player / playlist / settings / jsplugin）
│       └── shared/             # 共享布局、模型、组件
├── plugin-toolchain/           # JS 插件开发工具链（SDK + Builder + 脚手架）
├── jsplugins-src/              # JS 插件源码集合（子模块）
├── jsplugins/                  # JS 插件构建产物（子模块）
├── scripts/                    # 构建和发布脚本
└── docs/                       # 项目文档
```

## 构建系统

### 构建标签

| 命令 | 标签 | 说明 |
|------|------|------|
| `make run` | `-tags dev` | 开发模式，含 Swagger，不嵌入前端 |
| `make build-prod` | 无标签 | 生产轻量版，不含 Swagger 和前端 |
| `make build-prod` + `FULL_FLAGS` | `-tags full` | 生产完整版，嵌入 Flutter Web |

### 前端构建

```bash
make build-frontend-web-embedded   # 嵌入模式（隐藏 API 地址 UI）
make build-frontend-web            # 独立部署版
make build-frontend-all            # 当前系统支持的所有平台
```

## 技术亮点

### 后端

1. **纯 Go 实现**：音频元数据提取、SQLite 驱动、QuickJS 运行时均为纯 Go 实现，无需 CGO，部署简单
2. **JS 插件系统**：基于 QuickJS 的脚本插件架构，支持动态扩展音源能力，沙盒隔离 + 权限模型 + 健康检查 + 热更新
3. **JWT 双 Token**：Access Token + Refresh Token，支持令牌撤销和管理
4. **音乐缓存**：按 hash 缓存网络歌曲，支持并发下载去重
5. **歌单转本地**：将歌单内网络歌曲落地到本地音乐库（按歌单分目录、可读文件名、限速防风控），支持手动+自动模式;转换后回写文件 tag (标题/艺术家/专辑/年份/歌词/封面);URL 歌词自动下载并 cache
6. **音频 tag 读写**:pkg/tag 在原 dhowden/tag 基础上扩展 MP3 (ID3v2.3) 和 FLAC (Vorbis Comment + Picture) 写入,纯 Go 无外部依赖
7. **资源代理**：内置 CORS 代理，含 SSRF 防护
8. **数据库驱动配置**：配置存储在 SQLite，支持 JSON 格式和 API 动态更新
9. **Tracely 监控**：心跳包、错误上报、panic 捕获

### 前端

1. **跨平台一致体验**：一套代码适配 6 个平台
2. **四端响应式布局**：Mobile / Tablet / Desktop / TV 自适应
3. **Feature-First 架构**：按功能模块组织，每个模块含 data / domain / presentation
4. **音频播放**：just_audio + audio_service，支持通知栏控制和后台播放
5. **歌词显示**：LRC 歌词解析和同步显示
6. **封面颜色提取**：从封面图片提取主色调用于动态配色
7. **TV 端支持**：焦点导航、D-pad 支持、大尺寸 UI

## 数据库设计

### 表结构

| 表名 | 说明 | 关键字段 |
|------|------|---------|
| **songs** | 歌曲/电台 | type(local/remote/radio), title, artist, album, duration, file_path, url, cover_path, lyric, lyric_source, plugin_entry_path, source_data, dedup_key |
| **playlists** | 歌单 | type(normal/radio), name, labels, cover_path, cover_url |
| **playlist_songs** | 歌单-歌曲关联 | playlist_id, song_id, position |
| **configs** | 系统配置 | key(唯一), value(JSON) |
| **auth_tokens** | 认证令牌 | token_id, token_type(access/refresh), expires_at, revoked_at |
| **js_plugins** | JS 插件信息 | name, version, entry_path, main, permissions, file_path, status(active/inactive/error), zip_hash, entry_hash |

### 索引设计

- 歌曲：类型、标题、艺术家、添加时间;(plugin_entry_path, dedup_key) 部分唯一索引(`WHERE dedup_key != ''`),用于网络歌曲按插件身份去重导入
- 歌单：类型、labels
- 歌单歌曲：playlist_id、position
- 配置：key
- 令牌：token_id、token_type、expires_at、revoked_at
- JS 插件：status、entry_path

### 触发器

所有表均配置 `updated_at` 自动更新触发器。

### 初始化数据

- 内置歌单：收藏（id=1）、电台收藏（id=2），均带 `labels=["built_in"]`
- 默认配置：`music_path`、`cover_storage_path`、`scan_config`、`ffprobe_path`、`jwt_secret`、`source_validation`、`source_fallback`、`source_metrics`
- `music_cache_config` / `auto_convert_remote` 等不在迁移内预置，由对应 service 首次使用时按需写入

## 扩展性设计

### 易于添加新功能
- 新 API：在对应 handler 中添加方法 → 在 `routers.go` 注册路由
- 新模型：在 `models/` 中定义 → 在 `database/` 中实现 CRUD
- 新服务：在 `services/` 中实现 → 通过构造函数注入到 handler
- 新插件：通过 JS 插件系统扩展（脚手架 `pnpm create songloft-plugin`），无需修改宿主代码

### 易于测试
- 数据库层走 `database/testutil.OpenMemoryDB(t)` 起 `:memory:` SQLite + 真实 Repository，避免手写 mock（已统一删除）
- service 注入接口而非具体类型，业务逻辑与 HTTP 处理分离
- 每个模块职责单一
- 完整的单元测试和集成测试

### 易于维护
- `internal/` 目录防止外部依赖内部实现
- 模块间低耦合
- 遵循 Go 社区标准和惯例
- 插件系统与核心功能解耦