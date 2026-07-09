# Songloft

自托管本地音乐服务器 —— 作为 Home Assistant 加载项运行。

Self-hosted local music server, running as a Home Assistant add-on.

---

## 中文

### 安装

1. Home Assistant「设置 → 加载项 → 加载项商店」，右上角菜单「仓库」，添加：
   `https://github.com/songloft-org/songloft`
2. 刷新后在商店里找到 **Songloft**，点击安装。
3. 安装完成后到「配置」页填写选项，然后启动。

### 配置选项

| 选项 | 说明 | 默认 |
|------|------|------|
| `admin_username` | 管理员用户名 | `admin` |
| `admin_password` | 管理员密码（**请务必修改**） | `admin` |
| `music_path` | 音乐库目录，默认指向 HA 媒体目录 `/media`，也可用 `/share` | `/media` |
| `base_path` | 反向代理子路径部署用（如 `/songloft`），根路径部署留空 | 空 |

> 🔐 **安全提示**：默认账号 `admin/admin` 仅适用于本地测试。任何对外暴露的部署，请先在配置页设置强密码再启动。

### 音乐文件放哪

把音乐放进 Home Assistant 的媒体目录（默认映射为 `/media`）或共享目录（`/share`）。启动后进入 Web 界面触发扫描即可。

### 访问

- 启动后点加载项详情页的「打开 Web UI」按钮（新标签页）。
- 数据（数据库、缓存、插件数据）持久化在加载项的 `/data` 目录，卸载重装不丢。

---

## English

### Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**, open the top-right menu **Repositories**, and add:
   `https://github.com/songloft-org/songloft`
2. After refreshing, find **Songloft** in the store and install it.
3. Fill in the options on the **Configuration** tab, then start the add-on.

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `admin_username` | Administrator username | `admin` |
| `admin_password` | Administrator password (**change this**) | `admin` |
| `music_path` | Music library directory. Defaults to the HA media folder `/media`; `/share` also works | `/media` |
| `base_path` | URL base path for reverse-proxy sub-path deployments (e.g. `/songloft`); leave empty for root | empty |

> 🔐 **Security**: the default `admin/admin` credentials are for local testing only. Set a strong password before exposing this instance.

### Where to put music

Place your music in Home Assistant's media folder (mapped to `/media`) or share folder (`/share`), then open the Web UI to trigger a scan.

### Access

- Use the **Open Web UI** button on the add-on page (opens in a new tab).
- Data (database, cache, plugin data) is persisted in the add-on's `/data` directory and survives reinstalls.
