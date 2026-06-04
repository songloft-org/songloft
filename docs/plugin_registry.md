# 插件源制作指南

本文档介绍如何创建和发布 Songloft 插件源（Plugin Registry），让其他用户通过订阅你的源地址，在应用内的「插件商店」浏览并安装你的插件。

---

## 什么是插件源

插件源是一个 JSON 文件，包含一组 `plugin.json` 的 URL 列表。用户在「设置 → JS 插件管理 → 插件商店 → 管理订阅源」中添加你的 JSON URL 后，即可看到源中的所有插件并一键安装。

后端会自动从每个 `plugin.json` URL 拉取插件的名称、版本、描述等信息，无需在源文件中重复填写。

插件源支持**嵌套引用**（`includes`），可以组合多个独立源，实现去中心化的插件分发。

---

## JSON 格式规范

```json
{
  "name": "我的插件源",
  "includes": [
    "https://example.com/other-registry.json"
  ],
  "plugins": [
    "https://raw.githubusercontent.com/you/example-plugin/main/plugin.json",
    "https://raw.githubusercontent.com/you/another-plugin/main/plugin.json"
  ]
}
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | 源名称，用于在 UI 中显示 |
| `includes` | string[] | 否 | 嵌套引用的其他源 URL 数组 |
| `plugins` | string[] | 是 | 各插件 `plugin.json` 的 URL 数组 |

### 自动解析机制

每个 `plugins` 中的 URL 指向插件仓库中的 `plugin.json`，后端会自动拉取并解析以下字段：

- `name` — 插件名称
- `entryPath` — 插件唯一标识符
- `version` — 版本号
- `description` — 描述
- `author` — 作者
- `homepage` — 项目主页
- `download_url` — ZIP 包下载地址
- `updateUrl` — 更新检查 URL
- `minHostVersion` — 最低宿主版本

如果 `plugin.json` 中没有 `download_url`（大多数插件是这种情况），后端会自动从 `updateUrl` 指向的 `manifest.json` 中获取。

---

## 完整示例

### 最小示例

```json
{
  "plugins": [
    "https://raw.githubusercontent.com/you/my-plugin/main/plugin.json"
  ]
}
```

### 含嵌套的完整示例

```json
{
  "name": "Songloft 社区聚合源",
  "includes": [
    "https://raw.githubusercontent.com/alice/songloft-plugins/main/registry.json",
    "https://raw.githubusercontent.com/bob/my-plugin-registry/main/registry.json"
  ],
  "plugins": [
    "https://raw.githubusercontent.com/songloft-org/songloft-plugin-miot/main/plugin.json",
    "https://raw.githubusercontent.com/songloft-org/songloft-plugin-lxmusic/main/plugin.json"
  ]
}
```

---

## 发布到 GitHub

### 方式一：仓库内 Raw URL（推荐）

1. 在你的 GitHub 仓库根目录创建 `registry.json`
2. 推送到 `main` 分支
3. 源地址为：
   ```
   https://raw.githubusercontent.com/{用户名}/{仓库名}/main/registry.json
   ```

**插件 ZIP 托管**：推荐使用 GitHub Releases：
1. 在插件仓库创建 Release
2. 上传 `.jsplugin.zip` 作为 Release Asset
3. 在 `plugin.json` 中通过 `updateUrl` 指向 `manifest.json`，`manifest.json` 中填写 `download_url`：
   ```
   https://github.com/{用户名}/{仓库名}/releases/download/v{版本号}/{entry_path}.jsplugin.zip
   ```

### 方式二：GitHub Pages

1. 启用仓库的 GitHub Pages
2. 将 `registry.json` 放在 Pages 根目录
3. 源地址为：
   ```
   https://{用户名}.github.io/{仓库名}/registry.json
   ```

### 示例仓库结构

```
my-plugin-registry/
├── registry.json          ← 插件源 JSON（只需列出 plugin.json URL）
└── README.md
```

插件本身在各自的仓库中维护，例如：

```
my-songloft-plugin/
├── plugin.json            ← 插件元数据（name, version, entryPath 等）
├── manifest.json          ← 更新清单（version + download_url）
├── main.js                ← 插件入口
└── ...
```

---

## 发布到 JS CDN

### jsDelivr（通过 npm）

1. 创建 npm 包，包含 `registry.json`
2. 发布到 npm：
   ```bash
   npm publish
   ```
3. 源地址为：
   ```
   https://cdn.jsdelivr.net/npm/{包名}@latest/registry.json
   ```

### unpkg（通过 npm）

与 jsDelivr 类似，只需替换域名：
```
https://unpkg.com/{包名}@latest/registry.json
```

### jsDelivr（通过 GitHub）

不发布 npm 包也可以直接用 jsDelivr 加速 GitHub 文件：
```
https://cdn.jsdelivr.net/gh/{用户名}/{仓库名}@{分支}/registry.json
```

---

## 嵌套源的使用场景

`includes` 字段允许一个源引用其他源，递归下载并合并所有插件。

### 典型用法

**聚合源**：收集多个独立作者的源，方便用户一次性订阅：
```json
{
  "name": "社区聚合",
  "includes": [
    "https://raw.githubusercontent.com/alice/plugins/main/registry.json",
    "https://raw.githubusercontent.com/bob/plugins/main/registry.json",
    "https://raw.githubusercontent.com/charlie/plugins/main/registry.json"
  ],
  "plugins": []
}
```

**官方 + 社区**：官方源包含核心插件，同时引入社区贡献：
```json
{
  "name": "官方源",
  "includes": [
    "https://community.example.com/registry.json"
  ],
  "plugins": [
    "https://raw.githubusercontent.com/songloft-org/songloft-plugin-miot/main/plugin.json"
  ]
}
```

### 去重规则

当同一个 `entryPath` 出现在多个源（含嵌套）中时，保留**版本号更高**的条目。版本按 `.` 分隔后逐段数值比较。

---

## 注意事项

| 限制 | 值 | 说明 |
|------|-----|------|
| 插件总数上限 | 500 个 | 超出部分截断，记入警告 |
| 递归深度上限 | 20 层 | 超过 20 层的 `includes` 会被跳过 |
| 单个 JSON 大小上限 | 2 MB | 超过则拒绝解析 |
| 单 URL 请求超时 | 15 秒 | 超时后跳过该 URL（记入警告） |
| 循环引用 | 自动检测 | A → B → A 不会死循环，已访问的 URL 自动跳过 |

- `plugins` 中的 URL 必须指向有效的 `plugin.json` 文件
- 如果源文件托管在 GitHub，中国大陆用户可在插件商店中选择 GitHub 镜像加速（预设或自定义），也可在「设置 → 系统 → HTTP 代理」中配置通用代理（如 `http://192.168.1.1:7890`）以加速访问
- 某个 `includes` 或 `plugins` 条目拉取失败不会影响其他源和插件的加载
