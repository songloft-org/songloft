# 插件源制作指南

本文档介绍如何创建和发布 Songloft 插件源（Plugin Registry），让其他用户通过订阅你的源地址，在应用内的「插件商店」浏览并安装你的插件。

---

## 什么是插件源

插件源是一个 JSON 文件，包含一组 `plugin.json` 的 URL 列表。用户在「设置 → JS 插件管理 → 插件商店 → 管理订阅源」中添加你的 JSON URL 后，即可看到源中的所有插件并一键安装。

商店会自动从每个 `plugin.json` URL 拉取插件的名称、版本、描述等信息，无需在源文件中重复填写。

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

每个 `plugins` 中的 URL 指向插件仓库中的 `plugin.json`，商店会自动拉取并解析以下字段：

- `name` — 插件名称
- `entryPath` — 插件唯一标识符
- `version` — 版本号
- `description` — 描述
- `author` — 作者
- `homepage` — 项目主页
- `download_url` — ZIP 包下载地址
- `updateUrl` — 更新检查 URL
- `minHostVersion` — 最低宿主版本

如果 `plugin.json` 中没有 `download_url`（大多数插件是这种情况），商店会自动从 `updateUrl` 指向的 `manifest.json` 中获取。

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
    "https://raw.githubusercontent.com/songloft-org/songloft-plugin-subsonic/main/plugin.json"
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

去重的判定单位是「**插件身份**」，而不是单独的 `entryPath`：

- **身份 = `entryPath` + 作者**。作者取 `plugin.json` 的 `author` 字段，规范化后比较（忽略大小写、剥掉 `<邮箱>` 与 `(备注)`），所以同一作者写成 `hanxi` 或 `Hanxi <a@b.com>` 视为同一人。
- `author` 为空时，退化为用 `updateUrl` 指向的 GitHub 仓库（`owner/repo`）作为身份；非 GitHub 的自托管地址无法可靠推断仓库归属，故不推断。
- 两者都得不出身份时，退化为仅按 `entryPath` 判定。

由此：

| 情形 | 结果 |
|------|------|
| 同一插件出现在多个源（含嵌套） | 合并为一条，保留**版本号更高**的（版本按 `.` 分隔后逐段数值比较） |
| `entryPath` 相同但**作者不同**的两个插件 | **各自成一条**，都能在商店里看到并安装 |

> **`entryPath` 相同的两个插件无法在本地共存。** `entryPath` 同时是插件的安装目录名、路由前缀和数据归属键，因此本地同一个 `entryPath` 只能装一个插件。若你要安装的插件与已装插件撞了 `entryPath`，商店会把该条目标记为冲突、并说明是谁占用了它；点击「覆盖安装」并确认后，原插件会被替换，且**新插件会继承原插件保存的数据**。为避免给用户造成这种两难，发布插件时请选一个足够独特的 `entryPath`。

---

## 客户端渐进加载

Flutter 和 Lynx 的商店由客户端请求注册表、递归 `includes` 与插件清单，解析出一个插件就更新列表，不等待其他慢请求。每次最多并发 8 个请求；首批加载完成后，每页显示 20 项，搜索与翻页复用 5 分钟的会话缓存。主动刷新会重新读取源配置并绕过缓存，切换源或搜索会取消旧列表更新。

刷新时发现当前源已删除或停用，会回退到所有启用源；有效空清单会替换旧缓存，后续搜索不会恢复已删除条目。首条安装后若后续解析出更高版本，仍会显示更新入口。

源配置和已安装插件仍从 Songloft 服务器读取，安装与更新仍由服务器执行。Web 遇到跨域或连接失败时回退现有服务器读取接口；原生客户端保留已成功的条目并显示失败警告。首条结果通常更早可见，实际总耗时仍取决于客户端网络、源响应和 Web 是否需要回退。

## GitHub 发现图标

在默认分支根目录的 `plugin.json` 中声明 `icon`，图标就会在 GitHub 发现列表和详情中显示。单文件名（如 `icon.svg`）从 `static/` 读取，带目录的路径（如 `frontend/public/icon.svg`）从仓库根目录读取。支持 SVG 和客户端可解码的位图。缺省或加载失败时显示插件名首字。

## GitHub 发现 Token

Flutter 和 Lynx 的「GitHub 发现」页顶部提供 GitHub Token 配置入口，支持保存、更换和清除。Token 保存在当前连接的 Songloft 服务器，客户端进入页面或主动刷新时重新读取；发现请求仍由客户端发起。

- 配置接口：已登录客户端通过 `GET/PUT /api/v1/settings/github-discovery-token` 读写 `{ "token": "..." }`；未配置时为空，PUT 空字符串清除。响应禁止缓存。
- 配置 Token 后，搜索和 Release 查询直连 `https://api.github.com`，通过 Authorization 请求头认证。GitHub 加速代理和公开插件清单请求不携带此 Token，认证请求失败也不会改走加速代理。
- 未配置 Token 时，发现请求沿用现有 GitHub 加速代理及失败直连机制。保存或清除后刷新发现结果；无效或过期的 Token 会显示提示。

此配置与下方的私有订阅源 Token 独立，**插件源自己的 Token 优先**。商店客户端访问官方 GitHub API 时，只有该请求没有源 Token 才使用共享 Token；共享 Token 不用于 raw 清单、第三方源、服务器回退或安装下载。配置后客户端必须能访问 GitHub 官方 API。

## 私有源认证

插件源支持 **Bearer Token 认证**，用于分发未开源的插件或访问 GitHub 私有仓库中的插件。

### 配置方式

在「管理订阅源」对话框中添加或编辑源时，填入 Token 字段即可。配置了 Token 的源会在列表中显示锁图标。

Token 以 `Authorization: Bearer <token>` 头发送。作用域规则：

- **registry JSON 本身**、**plugins 数组中的所有 URL**、**ZIP 下载** — 始终带 token
- **includes 引用的子源** — 客户端仅在**同 origin**（协议、主机、端口均相同）时继承 token；服务器兼容读取按同 host 判断。跨域 includes 不继承 token

源 Token 优先于共享 GitHub Token。带 Token 的清单请求、服务器兼容读取和安装下载均绕过 GitHub 加速代理；不会把两类 Token 发送到前缀代理。插件清单中的后续 `updateUrl` 在客户端仅同初始清单或配置源的 origin 时继承源 Token。

因此，私有源可以安全地 include 公开的官方源或第三方源，token 不会被发给它们。同时，同服务器上的私有子源（同 host 的 includes）仍能正常认证。

### GitHub 私有仓库

1. 在 GitHub 创建 [Fine-grained Personal Access Token](https://github.com/settings/tokens?type=beta)
2. 权限：只需目标仓库的 **Contents: Read-only**
3. 源 URL 使用 `raw.githubusercontent.com` 格式，与公开仓库相同：
   ```
   https://raw.githubusercontent.com/{用户名}/{私有仓库}/main/registry.json
   ```
4. 在「管理订阅源」中将 PAT 填入 Token 字段

> GitHub PAT 同时适用于 `raw.githubusercontent.com`（registry/plugin.json）和 `github.com`（Release 下载）。Go HTTP 客户端在跨域重定向时会自动剥离 Authorization 头，Release 下载重定向到的 S3 签名 URL 不需要 Token，因此行为正确。

### 自托管私有源

在你的服务器上校验 `Authorization: Bearer <token>` 头即可。以 nginx 为例：

```nginx
location /registry/ {
    if ($http_authorization != "Bearer your-secret-token") {
        return 401;
    }
    root /var/www/plugins;
}
```

### 安全注意事项

- Token 以明文存储在服务端 config 表中（自托管应用，在用户控制范围内）
- 客户端 `includes` 仅同 origin 继承 Token；服务器兼容读取仅同 host 继承，跨域 includes 不继承
- Token 通过 `Authorization` 请求头发送，不会出现在 URL 中，不会泄露到日志或 Referrer

---

## 注意事项

| 限制 | 值 | 说明 |
|------|-----|------|
| 单源插件数上限 | 500 个 | 超出部分截断，记入警告 |
| 客户端注册表数上限 | 每源 512 个 | 超出部分跳过，记入警告 |
| 递归深度上限 | 20 层 | 超过 20 层的 `includes` 会被跳过 |
| 单个 JSON 大小上限 | 2 MB | 超过则拒绝解析 |
| 单 URL 请求超时 | 15 秒 | 超时后跳过该 URL（记入警告） |
| 循环引用 | 自动检测 | A → B → A 不会死循环，已访问的 URL 自动跳过 |
| 结果缓存 | 5 分钟 | 客户端缓存完整清单；服务器回退也有缓存，翻页与搜索不会重复拉取远端 |

- `plugins` 中的 URL 必须指向有效的 `plugin.json` 文件
- **调试插件源时记得点「刷新」**：商店的分页与搜索都在缓存的完整列表上做，改了源文件后
  普通翻页看不到变化，点刷新按钮才会绕过缓存重新拉取（缓存也会在 5 分钟后自然过期）。
  插件的安装状态不受缓存影响，始终实时计算
- 匿名 GitHub 请求可使用已配置的 GitHub 镜像加速；带 Token 的请求直接访问源站。客户端请求使用本机网络，服务器「HTTP 代理」只影响服务器回退读取和安装下载，不会改变客户端的请求路径
- 某个 `includes` 或 `plugins` 条目拉取失败不会影响其他源和插件的加载
