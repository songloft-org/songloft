# 从 MiMusic 迁移到 Songloft (v2.0)

> 本文档面向 **v1.x 老用户** 与 **JS 插件作者**。一句话总结：v2.0 是品牌升级到 Songloft，**API 路径 / 数据库结构 / 配置语义全部不变**，但**包名 / 镜像名 / JS 全局变量名 / 客户端 Bundle ID** 全部更名。

---

## 改名概览

| 类型 | v1.x（MiMusic） | v2.0（Songloft） |
|------|----------------|----------------|
| Go module | `mimusic` | `songloft` |
| Docker 镜像 | `hanxi/mimusic` | `songloft/songloft` |
| GitHub 组织 | `mimusic-org` | `songloft-org` |
| 主仓库 | `mimusic-org/mimusic` | `songloft-org/songloft` |
| Flutter 仓库 | `mimusic-org/mimusic-player` | `songloft-org/songloft-player` |
| npm scope | `@mimusic/*` | `@songloft/*` |
| 脚手架命令 | `create-mimusic-plugin` | `create-songloft-plugin` |
| 插件构建 CLI | `mimusic-plugin` | `songloft-plugin` |
| 配置文件 | `.mimusic-dev.json` | `.songloft-dev.json` |
| 环境变量 | `MIMUSIC_*` | `SONGLOFT_*` |
| JS 全局 | `mimusic.songs / log / storage / ...` | `songloft.songs / log / storage / ...` |
| Android 包 ID | `com.mimusic.mimusic_flutter` | `com.songloft.songloft_flutter` |
| iOS Bundle ID | `com.mimusic.mimusicFlutter` | `com.songloft.songloftFlutter` |
| 二进制 / 数据库 | `mimusic` / `data/mimusic.db` | `songloft` / `data/songloft.db` |

---

## 1. 老用户（后端 / 二进制 / Docker）

### 1.1 数据库

v2.0 启动时**自动**检测：若 `data/songloft.db` 不存在且 `data/mimusic.db` 存在 → 自动 rename，无需任何操作。

⚠️ 这是**唯一**的兼容点。如果你想自己迁移，命令是：

```bash
mv data/mimusic.db data/songloft.db
```

### 1.2 二进制升级

老二进制文件名是 `mimusic`，新的是 `songloft`。下载新二进制后：

```bash
# Linux / macOS
chmod +x songloft-linux-amd64-full
mv songloft-linux-amd64-full songloft
./songloft
```

老的 `mimusic` 二进制和 `data/mimusic.db` 不会被自动删除，可放心保留作为回滚备份。

### 1.3 Docker

```bash
# 旧
docker pull hanxi/mimusic:full

# 新
docker pull songloft/songloft:full
```

挂载老 mimusic 用户的 `data/` 卷即可，启动时 db 文件会被自动 rename。

```bash
docker run -d \
  --name songloft \
  -p 58091:58091 \
  -v /path/to/music:/app/music \
  -v /path/to/data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD='your_password' \
  songloft/songloft:full
```

> `hanxi/mimusic` 镜像保留但 **从 v2.0 起不再推新 tag**。

### 1.4 环境变量

后端环境变量名称（`ADMIN_USERNAME` / `ADMIN_PASSWORD` / `LISTEN_PORT` / `DB_PATH`）**没变**，沿用即可。

---

## 2. 客户端（Flutter 桌面 / 移动端）

⚠️ **Songloft 客户端是全新应用**，Android applicationId 和 iOS Bundle ID 都改了，操作系统视之为另一个应用。**老 MiMusic app 的本地数据（账号、设置、缓存）无法迁移到新 Songloft app**。

### 推荐流程

1. 在老 MiMusic app 里记下你的 **API 地址**（在「设置」页能看到）
2. 安装新 Songloft app
3. 首次启动时填回 API 地址 + 用户名 + 密码
4. 确认能正常播放后，再卸载老 MiMusic app

老 app 不会被自动卸载，你可以**新老并存**直到确认数据无误。

---

## 3. JS 插件作者

### 3.1 SDK 包名变更

```diff
- "@mimusic/plugin-sdk": "^0.9.0"
+ "@songloft/plugin-sdk": "^1.0.0"

- "@mimusic/plugin-builder": "^0.9.0"
+ "@songloft/plugin-builder": "^1.0.0"

- "@mimusic/musicsdk": "..."
+ "@songloft/musicsdk": "..."
```

老的 `@mimusic/*` 系列已发布 npm `deprecated` 提示，**不会再有更新**。

### 3.2 脚手架

```bash
# 旧
pnpm create mimusic-plugin my-plugin

# 新
pnpm create songloft-plugin my-plugin
```

### 3.3 JS 全局 ABI（核心 breaking change）

所有 `mimusic.*` 调用必须改为 `songloft.*`：

```diff
- mimusic.log.info('hello');
+ songloft.log.info('hello');

- const songs = await mimusic.songs.list();
+ const songs = await songloft.songs.list();

- await mimusic.storage.set('key', 'value');
+ await songloft.storage.set('key', 'value');
```

涉及的命名空间：
- `mimusic.log` → `songloft.log`
- `mimusic.songs` → `songloft.songs`
- `mimusic.playlists` → `songloft.playlists`
- `mimusic.storage` → `songloft.storage`
- `mimusic.plugin` → `songloft.plugin`
- `mimusic.jsenv` → `songloft.jsenv`
- `mimusic.comm` → `songloft.comm`

⚠️ Songloft v2.0 后端**完全不识别** `mimusic.*` 全局，老插件装上会启动报错。

### 3.4 配置文件 + 环境变量

```bash
# 旧
mv .mimusic-dev.json .songloft-dev.json

# 环境变量
export SONGLOFT_HOST=...
export SONGLOFT_USER=...
export SONGLOFT_PASSWORD=...
export SONGLOFT_TOKEN=...
```

`MIMUSIC_*` 环境变量和 `.mimusic-dev.json` **不再被读取**。

### 3.5 插件分发模式变更

v2.0 起取消"插件聚合仓库"模式（旧的 `mimusic-org/jsplugins` 已下线）。每个插件改为在自己的 GitHub 仓库下分发：

- 构建产物 `.jsplugin.zip` 放到本仓库的 GitHub Release
- `manifest.json` 同时 commit 到本仓库 main 分支供 `updateUrl` 拉取

```diff
- "updateUrl": "https://raw.githubusercontent.com/mimusic-org/jsplugins/main/myplugin.json",
+ "updateUrl": "https://raw.githubusercontent.com/<your-org>/<your-plugin-repo>/main/manifest.json",
```

`homepage` 中如指向 `mimusic-org/*` 仓库的链接也需改为 `songloft-org/*`。

---

## 4. 时间表

| 阶段 | 时间 | 内容 |
|------|------|------|
| v2.0.0-alpha.1 | ~T+2 周 | 后端 + Web 端；插件作者开始适配 |
| v2.0.0-beta.1 | ~T+6 周 | 全平台 Flutter 端就绪 |
| v2.0.0-rc.1 | ~T+10 周 | 文档定稿，最后稳定性验证 |
| v2.0.0 | ~T+12 周 | 正式 release；GitHub 仓库 transfer 到 songloft-org；Docker 镜像切换 |
| v1.x EOL | v2.0 发布后 6 个月 | `hanxi/mimusic` Docker 镜像停止接收 PR 修复 |

---

## 5. 反馈与求助

- GitHub Issues: https://github.com/songloft-org/songloft/issues
- 老仓库 issue 暂仍可使用：https://github.com/mimusic-org/mimusic/issues
- 微信群 / QQ 群链接见 [README](README.md#-技术支持)
