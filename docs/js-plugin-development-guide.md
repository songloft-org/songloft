# JS 插件开发指南

本文档详细介绍 Songloft JS 插件系统的架构、API 和开发流程。

---

## 1. 概述

Songloft JS 插件系统允许开发者使用 JavaScript 扩展音乐服务器功能，无需编译 Go 代码。

### 设计理念

系统基于 **Skynet Actor 模型**设计：

- 每个插件是一个独立的 **Actor（JSService）**，拥有自己的 JS 虚拟机
- 插件之间通过 **消息** 通信，互不干扰
- 所有消息由 **ServiceScheduler** 统一调度，保证串行处理
- 双层 SHA256 校验确保插件代码完整性

### 核心特性

| 特性 | 说明 |
|------|------|
| 沙箱隔离 | 每个插件运行在独立的 QuickJS 虚拟机中 |
| 权限控制 | 细粒度权限声明，按需授权 |
| 热更新 | 运行时更新插件，无需重启服务 |
| 插件间通信 | send/call 消息机制 |
| 静态资源 | 内置 Web UI 托管 |
| 健康检查 | 自动检测异常插件并处理 |

### 架构示意

```
Manager（管理器）
  ├── PackageManager（包管理：安装/更新/卸载）
  ├── ServiceScheduler（消息调度器）
  │   ├── JSService[plugin-a]（Actor + QuickJS VM）
  │   ├── JSService[plugin-b]（Actor + QuickJS VM）
  │   └── ...
  ├── HotReloader（热更新监控）
  └── HealthChecker（健康检查）
```

---

## 2. 快速开始

推荐使用官方工具链 [songloft-plugin-toolchain](https://github.com/songloft-org/plugin-toolchain)，5 分钟创建、构建并上传你的第一个 JS 插件。

### Step 1: 用脚手架创建项目

```bash
pnpm create songloft-plugin my-plugin
# 或 npm create songloft-plugin my-plugin
cd my-plugin
pnpm install
```

脚手架会交互式询问插件 `name` / `entryPath` / `description` / `author` / `permissions`，并生成如下结构：

```
my-plugin/
├── plugin.json        # 插件清单（entryHash / zipHash 由 builder 生成）
├── package.json       # npm 依赖（@songloft/plugin-sdk / @songloft/plugin-builder）
├── tsconfig.json
├── src/
│   └── main.ts        # TypeScript 源码入口
└── static/            # 静态资源
    └── index.html
```

### Step 2: 编写业务逻辑

`src/main.ts` 使用 `@songloft/plugin-sdk` 提供的全局类型与 helper：

```typescript
/// <reference types="@songloft/plugin-sdk" />
import { jsonResponse, createRouter } from '@songloft/plugin-sdk';

const router = createRouter();

router.get('/hello', (req) => jsonResponse({ message: 'Hello!', query: req.query }));

router.get('/songs', (req) => {
  const songs = songloft.songs.list({ limit: 10 });
  return jsonResponse({ count: songs.length, songs });
});

function onInit(): void { songloft.log.info('my-plugin initialized'); }
function onDeinit(): void { songloft.log.info('my-plugin deinitialized'); }
function onHTTPRequest(req: HTTPRequest): HTTPResponse { return router.handle(req); }

// @ts-expect-error — QuickJS 全局注入
globalThis.onInit = onInit;
// @ts-expect-error
globalThis.onDeinit = onDeinit;
// @ts-expect-error
globalThis.onHTTPRequest = onHTTPRequest;
```

### Step 3: 启动开发模式（推荐）

```bash
pnpm run dev          # 等价于 songloft-plugin dev
```

首次运行会交互式询问 Songloft 实例地址、用户名与密码，之后：

1. 把账号密码写入项目根目录的 `.songloft-dev.json`（builder 会自动把它追加到 `.gitignore`），后续运行直接静默登录；
2. 立即执行一次构建并上传，首次安装时自动启用插件；
3. 监听 `src/`、`static/`、`plugin.json`，源码变更时自动重建上传，已激活的插件会被后端自动热重载。

> Token 不缓存：每次会话用账号密码即时登录，因此无需关心 token 过期 / 刷新。要换帐号或改密码，编辑（或直接删除）`.songloft-dev.json` 即可。

控制台会打印插件的访问入口（例如 `http://localhost:58091/api/v1/jsplugin/<entryPath>/`），按 `Ctrl+C` 退出。

> 开发模式的详细 CLI 选项、环境变量与配置文件字段见下文 [开发模式详解](#开发模式详解-songloft-plugin-dev)。

### Step 4: 构建生产包

发布前生成可分发的 `.jsplugin.zip`：

```bash
pnpm run build        # 等价于 songloft-plugin build
```

builder 会：

1. 用 esbuild 把 `src/main.ts` 打包为 `build/main.js`（`format: iife`, `target: es2020`，禁止引用 Node 内置模块）；
2. 拷贝 `static/` 到 `build/`，并对 JS/CSS/字体/图片注入内容 hash（可在 `plugin.json` 中设置 `"staticHash": false` 关闭）；
3. 若检测到可用的 `jsc` 工具，将 `main.js` 进一步编译为 `main.jsc` 字节码；
4. 计算 `entryHash = sha256(main 文件)` 与 `zipHash`（规范化算法，排除 `plugin.json` 自身），写回 `build/plugin.json`；
5. 打包为 `dist/<entryPath>.jsplugin.zip`，并生成 `dist/<entryPath>.json` 远程更新元数据。

### Step 5: 安装到目标实例

任选其一：

- **开发模式自动上传** —— `pnpm run dev`（见 Step 3），适合本地迭代；
- **设置页面上传** —— 在 Songloft 客户端的插件管理页选择 `dist/<entryPath>.jsplugin.zip`；
- **目录放置** —— 把 zip 放进服务器的 `data/jsplugins/` 目录，下次启动时自动扫描；
- **API 上传** —— `POST /api/v1/jsplugins/upload`，multipart 字段名 `file`（开发模式底层即此接口）。

安装后，插件的 HTTP API 通过 `/api/v1/jsplugin/<entryPath>/` 访问，静态资源通过 `/api/v1/jsplugin/<entryPath>/static/...` 访问。

### 开发模式详解 (songloft-plugin dev)

`songloft-plugin dev` 把"构建 → 上传 → 热重载"压缩成一个常驻命令，适合本地开发与远程实例联调。

#### 默认行为

| 阶段 | 行为 |
|------|------|
| 启动 | 读取 `.songloft-dev.json`，缺失 `username` / `password` 时交互式询问，登录成功后落地保存 |
| 登录策略 | 不缓存 token；每次启动用账号密码即时登录，会话期间出现 `401` 时自动用同一密码重登 |
| 首次上传 | 调用 `POST /api/v1/jsplugins/upload`，新装后自动调用 `enable` |
| 后续上传 | 同一 `entryPath` 复用 upload 接口，由后端识别为覆盖更新；插件处于活跃状态时自动热重载 |
| 文件监听 | 监听 `src/`、`static/`、`plugin.json`，250ms debounce 触发增量构建 |
| 密码失效 | 若服务器拒绝缓存的密码（如已被修改），自动清除 `.songloft-dev.json` 中的 `password` 字段并提示重新运行 |

#### CLI 选项

```text
songloft-plugin dev [options]

--host <url>        Songloft 实例 URL（默认 http://localhost:58091，
                    亦可读 $MIMUSIC_HOST 或 .songloft-dev.json）
--username <name>   登录用户名（或 $MIMUSIC_USER）
--password <pwd>    登录密码（或 $MIMUSIC_PASSWORD；缺省时静默提示输入）
--token <jwt>       直接使用预签发的 access token（或 $MIMUSIC_TOKEN）
--once              构建+上传一次后退出，跳过 watch
--no-enable         首次安装后不自动启用插件
```

#### 环境变量

| 变量 | 等价选项 |
|------|----------|
| `MIMUSIC_HOST` | `--host` |
| `MIMUSIC_USER` | `--username` |
| `MIMUSIC_PASSWORD` | `--password` |
| `MIMUSIC_TOKEN` | `--token` |

#### `.songloft-dev.json` 字段

dev 命令自动在项目根目录维护下面的配置文件（同时把它追加到 `.gitignore`）：

```json
{
  "host": "http://localhost:58091",
  "username": "admin",
  "password": "your-password",
  "pluginId": 12,
  "entryPath": "my-plugin"
}
```

| 字段 | 写入时机 | 说明 |
|------|----------|------|
| `host` | 首次启动 | Songloft 实例 URL |
| `username` / `password` | 首次启动交互输入后写入，亦可手填 | 用于每次会话登录；明文存储，**切勿提交** |
| `pluginId` / `entryPath` | 首次上传后写入 | 仅供参考，dev 命令实际通过 `entryPath` 与后端对账 |

> 不存在 `accessToken` / `refreshToken` 字段：dev 命令不缓存 token。
>
> 不想让密码明文落地？改用 `--token <jwt>` 或 `$MIMUSIC_TOKEN` 提供预签发的 access token；token 模式下不会读写 `.songloft-dev.json` 中的凭据字段。
>
> 删除整个文件等同于重置登录状态。

---

## 3. 插件结构

### ZIP 打包格式

插件以 `.jsplugin.zip` 格式分发，文件名规则：`{entryPath}.jsplugin.zip`

ZIP 内部结构（所有文件在根级别，不含父目录）：

```
plugin.json          # 插件清单（必须）
main.js              # 入口文件（必须，或 main.jsc 字节码）
static/              # 静态资源目录（可选）
  ├── index.html
  ├── style.css
  └── app.js
```

### plugin.json 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 插件名称（2-50 字符） |
| `version` | string | 是 | 语义化版本号（如 `1.0.0`） |
| `description` | string | 否 | 插件描述 |
| `author` | string | 否 | 作者 |
| `homepage` | string | 否 | 主页 URL |
| `license` | string | 否 | 许可证 |
| `entryPath` | string | 是 | 路由前缀（小写字母+数字+连字符，如 `my-plugin`） |
| `main` | string | 是 | 入口文件路径（必须以 `.js` 结尾） |
| `minHostVersion` | string | 否 | 最低宿主版本要求 |
| `permissions` | string[] | 是 | 权限列表（可为空数组 `[]`） |
| `updateUrl` | string | 否 | 远程更新检查 URL |
| `download_url` | string | 否 | 插件下载 URL |
| `entryHash` | string | 是 | `sha256(main.js)` 64 位小写 hex，由 `@songloft/plugin-builder` 自动生成，请勿手动编辑 |
| `zipHash` | string | 是 | zip 内除 `plugin.json` 外所有文件的规范化 sha256 64 位小写 hex，由 `@songloft/plugin-builder` 自动生成，请勿手动编辑 |

> `entryHash` / `zipHash` 为强制校验字段，缺失或与实际内容不匹配时，安装与加载均会被后端拒绝。`zipHash` 计算范围**不含** `plugin.json` 自身，避免 hash 写回 `plugin.json` 引起的循环依赖。

### entryPath 命名规则

- 仅允许小写字母、数字和连字符
- 必须以小写字母开头
- 正则：`^[a-z][a-z0-9-]*$`
- 示例：`example-basic`、`music-sync`、`metadata-helper`

---

## 4. 生命周期

插件有三个核心生命周期回调函数：

### onInit()

插件加载完成后调用。用于初始化资源、设置定时器等。

```javascript
function onInit() {
    songloft.log.info("Plugin initialized");
    songloft.storage.set("start_time", new Date().toISOString());
}
```

**注意**：`onInit()` 失败不会阻止插件运行，插件仍可响应 HTTP 请求。

### onDeinit()

插件卸载前调用。用于清理资源、保存状态。

```javascript
function onDeinit() {
    songloft.log.info("Plugin shutting down, saving state...");
}
```

### onHTTPRequest(req)

收到 HTTP 请求时调用。这是插件对外提供服务的主要入口。

**参数 `req` 结构：**

```javascript
{
    method: "GET",           // HTTP 方法
    path: "/songs",          // 请求路径（相对于插件的 entryPath）
    headers: {},             // 请求头 map
    body: "",                // 请求体（POST/PUT 时）
    query: "limit=10&offset=0"  // URL 查询字符串
}
```

**返回值结构：**

```javascript
{
    statusCode: 200,          // HTTP 状态码
    headers: {                // 响应头
        "Content-Type": "application/json"
    },
    body: "..."               // 响应体（字符串）
}
```

**示例：路由分发**

```javascript
function onHTTPRequest(req) {
    switch (req.path) {
        case "/":
        case "":
            return { statusCode: 200, body: "Hello!", headers: {} };
        case "/api/data":
            if (req.method === "POST") {
                return handlePost(req);
            }
            return handleGet(req);
        default:
            return { statusCode: 404, body: "Not Found", headers: {} };
    }
}
```

---

## 5. API 参考

所有 API 通过全局 `songloft` 对象访问。

### HTTP 请求（全局 fetch）

使用标准全局 `fetch` 函数发起 HTTP 请求（由运行时 polyfill 提供，底层为同步实现并以 Promise 形式暴露）。**无需声明权限**。

```javascript
// GET
const resp = await fetch("https://example.com/api");
const data = await resp.json();

// POST
const resp = await fetch("https://example.com/api", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ hello: "world" })
});
const text = await resp.text();
```

**`Response` 对象字段：**
- `ok` — `status >= 200 && status < 300`
- `status` — HTTP 状态码
- `statusText` — 状态文本
- `headers` — 响应头对象
- `json()` — 返回 `Promise<unknown>`，解析 JSON
- `text()` — 返回 `Promise<string>`，原始文本

> **注意**：`onHTTPRequest` 目前以同步方式调用，若要在其中使用 `fetch` 的响应，建议在 `onInit` 阶段预取并写入 `songloft.storage`，或通过 `songloft.comm` 异步通信。

### 定时器（全局 setTimeout / setInterval）

使用标准全局定时器 API（由运行时 polyfill 提供）。**无需声明权限**，插件卸载时运行时会自动清理未清除的定时器。

```javascript
// 一次性延迟
const t = setTimeout(() => songloft.log.info("tick"), 1000);
clearTimeout(t);

// 周期执行
const i = setInterval(() => songloft.log.info("heartbeat"), 60000);
clearInterval(i);
```

**注意：** 定时器回调在独立的后台 goroutine 中执行（每 500ms 检查一次到期定时器），使用 TryLock 机制确保**不阻塞 HTTP 请求处理**。当 HTTP 请求正在处理时，定时器自动让步等待下一轮。`setInterval` 的最小间隔被限制为 10ms。

### songloft.storage — 持久化存储

需要权限：`storage`

```javascript
// 读取值（返回字符串或 null）
var value = songloft.storage.get("key");

// 写入值
songloft.storage.set("key", "value");

// 删除键
songloft.storage.delete("key");

// 获取所有键名
var keys = songloft.storage.keys();  // ["key1", "key2", ...]
```

**存储限制：**
- 键名为字符串
- 值为字符串（复杂对象需手动 JSON 序列化）
- 每个插件有独立的存储空间

### songloft.songs — 歌曲操作

需要权限：`songs.read`

```javascript
// 获取歌曲列表
var songs = songloft.songs.list({ limit: 20, offset: 0 });

// 根据 ID 获取歌曲
var song = songloft.songs.getById(123);

// 搜索歌曲
var results = songloft.songs.search("关键词");
```

**Song 对象结构：**
```javascript
{
    id: 1,
    type: "local",        // "local" | "remote" | "radio"
    title: "歌曲名",
    artist: "艺术家",
    album: "专辑名",
    duration: 240.5,      // 秒
    file_path: "/path/to/file.mp3",
    url: "",
    cover_path: ""
}
```

### songloft.playlists — 歌单操作

需要权限：`playlists.read`（读取）或 `playlists.write`（修改）；或者通配符糖 `playlists.*`。

```javascript
// 需要 playlists.read
var playlists = songloft.playlists.list();
var playlist = songloft.playlists.getById(1);
var songs = songloft.playlists.getSongs(1, { limit: 50, offset: 0 });
```

### songloft.comm — 插件间通信

需要权限：`inter-plugin`

```javascript
// 异步发送消息（fire-and-forget）
songloft.comm.send("target-plugin", "action-name", { data: "hello" });

// 同步调用（等待响应，超时默认 10s）
var resp = songloft.comm.call("target-plugin", "action-name", { data: "hello" }, 5000);
// resp = { success: true, data: { ... } }

// 注册消息处理器
songloft.comm.onMessage("action-name", function(payload, from) {
    // payload: 发送方传递的数据
    // from: 发送方的 entryPath
    return { result: "processed" };  // 返回值作为 call 的响应
});
```

### songloft.log — 日志

无需权限。

```javascript
songloft.log.info("informational message");
songloft.log.warn("warning message");
songloft.log.error("error message");
```

日志输出到服务器标准日志，带 `[plugin]` 前缀。

### songloft.plugin — 插件信息

无需权限。

```javascript
// 获取插件的 JWT Token（用于访问宿主 API，如音乐文件、封面等需认证的资源）
var token = songloft.plugin.getToken();

// 获取宿主服务的基础 URL（如 http://192.168.1.100:58091）
var hostUrl = songloft.plugin.getHostUrl();
```

**典型用法：构建带认证的资源 URL**

```javascript
function getMusicUrl(songId) {
    var host = songloft.plugin.getHostUrl();
    var token = songloft.plugin.getToken();
    return host + "/music/" + encodedPath + "?access_token=" + token;
}
```

**方法说明：**
- `getToken()` — 返回当前有效的 JWT access_token 字符串，可用于访问宿主的受保护 API
- `getHostUrl()` — 返回宿主服务的基础 URL，用于构建完整的 API 或资源地址

---

## 6. 权限系统

插件必须在 `plugin.json` 的 `permissions` 字段中声明所需权限。运行时调用 API 时会校验权限，未声明的权限将被拒绝。

### 可用权限列表

与后端 `internal/jsplugin/permissions.go` 的 `AllPermissions` 保持一致：

| 权限 | 说明 |
|------|------|
| `storage` | 读写插件私有持久化存储 |
| `songs.read` | 读取歌曲元数据 |
| `songs.write` | 修改/写入歌曲元数据 |
| `songs.*` | 歌曲读写通配符（一把梭糖） |
| `playlists.read` | 读取歌单及歌单中的歌曲 |
| `playlists.write` | 创建/修改/删除歌单及其歌曲 |
| `playlists.*` | 歌单读写通配符（一把梭糖） |
| `inter-plugin` | 插件间通信 |
| `command` | 执行宿主提供的指令 |

> 注意：网络请求 (`fetch`)、定时器 (`setTimeout/setInterval`)、日志等能力**无需权限声明**，是默认宿主能力。

### 通配符糖

以 `.*` 结尾的权限在声明层作为一把梭糖，runner 在检查时用前缀匹配。例如声明 `playlists.*`
既包括 `playlists.read` 也包括 `playlists.write`；而单声明 `playlists.read` 时无法调用写接口。

### 最小权限原则

只声明实际需要的权限，减少安全风险：

```json
{
  "permissions": ["storage", "songs.read"]
}
```

---

## 7. 插件间通信

插件可以通过消息机制相互协作。

### 异步发送（Send）

发送方不等待响应，适合通知类场景：

```javascript
// Plugin A: 通知 Plugin B
songloft.comm.send("plugin-b", "data-updated", { source: "plugin-a" });
```

### 同步调用（Call）

发送方等待接收方处理并返回结果：

```javascript
// Plugin A: 调用 Plugin B 的服务
var response = songloft.comm.call("plugin-b", "get-data", { id: 123 }, 5000);
if (response.success) {
    var data = response.data;
}
```

### 注册处理器（onMessage）

接收方注册处理特定 action 的函数：

```javascript
// Plugin B: 注册 action handler
songloft.comm.onMessage("get-data", function(payload, from) {
    songloft.log.info("Request from: " + from);
    // payload = { id: 123 }
    return { name: "example", value: 42 };
});

songloft.comm.onMessage("data-updated", function(payload, from) {
    songloft.log.info("Got notification from: " + from);
    // 无需返回值（send 场景）
});
```

### 通信权限

通信双方都需要 `inter-plugin` 权限。

---

## 8. 静态资源

插件可以通过 `static/` 目录提供 Web UI。

### 目录结构

```
my-plugin/
├── plugin.json
├── main.js
└── static/
    ├── index.html
    ├── style.css
    └── app.js
```

### 访问路径

安装后，静态文件通过以下路径访问（注意：运行时路由是单数 `jsplugin`，与管理 API `/api/v1/jsplugins`（复数）不同）：

```
GET /api/v1/jsplugin/{entryPath}/                 → static/index.html（自动注入 <base>）
GET /api/v1/jsplugin/{entryPath}/static           → static/index.html
GET /api/v1/jsplugin/{entryPath}/static/<file>    → 任意静态资源
```

例如：`/api/v1/jsplugin/my-plugin/static/style.css`

> 后端在返回 `index.html` 时自动注入 `<base href="/api/v1/jsplugin/{entryPath}/">`，因此 HTML 中可直接用相对路径引用 `static/...` 和插件 API；同时还会注入一段 fetch 拦截脚本，在插件正在热重载（HTTP 503 `plugin_unavailable`）时静默重试。

### 在 HTML 中调用插件 API

```html
<script>
async function callPluginAPI() {
    // 相对路径访问插件的 HTTP API
    const resp = await fetch('../songs');
    const data = await resp.json();
    console.log(data);
}
</script>
```

### 注意事项

- 静态文件在安装时从 ZIP 解压到 `data/jsplugins_data/{entryPath}/static/`
- 更新插件时会重新解压静态文件
- 建议使用相对路径引用 API

---

## 9. 安全机制

### 双层 Hash 校验

插件系统使用两层 SHA256 校验保护代码完整性：

1. **Layer 1 — ZIP Hash**：整个 ZIP 文件的 SHA256
2. **Layer 2 — Entry Hash**：入口文件（main.js）内容的 SHA256

#### 校验流程

```
加载插件时：
1. 计算 ZIP 文件 SHA256 → 与数据库中的 zip_hash 比对
2. 若不匹配：
   - 检查文件 mtime 是否变化
   - mtime 未变 = 文件被篡改 → 拒绝加载
   - mtime 已变 = 合法更新 → 允许并更新 hash
3. 从 ZIP 内存中读取 main.js（不落盘）
4. 计算 main.js SHA256 → 与 entry_hash 比对
5. 若不匹配且 ZIP hash 未变 → 拒绝（内部篡改）
```

### main.js 不落盘

入口文件从 ZIP 直接读入内存，不写入磁盘文件系统，减少被篡改风险。

### 权限隔离

- 每个插件声明权限，运行时严格校验
- 未声明权限的 API 调用会被拒绝
- QuickJS 虚拟机提供运行时隔离

---

## 10. 打包发布

### 打包步骤

```bash
# 1. 确保目录结构正确
my-plugin/
├── plugin.json
├── main.js
└── static/
    └── index.html

# 2. 进入插件目录
cd my-plugin/

# 3. 打包为 ZIP（文件在根级别，不含父目录）
zip -r ../my-plugin.jsplugin.zip plugin.json main.js static/

# 4. 验证 ZIP 结构
unzip -l ../my-plugin.jsplugin.zip
# 应该看到:
#   plugin.json
#   main.js
#   static/index.html
```

### 文件命名

ZIP 文件名格式：`{entryPath}.jsplugin.zip`

系统会从文件名提取 entryPath：`my-plugin.jsplugin.zip` → `my-plugin`

### 安装方式

1. **开发模式（推荐）**：`songloft-plugin dev` 在本地迭代，参见 [§2.6](#26-开发模式详解-songloft-plugin-dev)
2. **UI 上传**：通过 Songloft 客户端的设置页面 → 插件管理上传 ZIP
3. **目录放置**：将 ZIP 放入服务器的 `data/jsplugins/` 目录，服务启动时自动发现
4. **API 上传**：`POST /api/v1/jsplugins/upload`，multipart 字段名 `file`（开发模式底层即此接口）

### 更新已有插件

- 重新上传同 `entryPath` 的新版本 ZIP 即可（`/upload` 端点同时处理新装与覆盖更新，由后端用响应状态码 `201` / `200` 区分）
- 也可显式调用 `PUT /api/v1/jsplugins/{id}` 上传新 ZIP
- 或直接替换 `data/jsplugins/` 目录中的 ZIP 文件

无论哪种方式，原插件若处于 `active` 状态，更新成功后后端会自动触发热重载。

---

## 11. 热更新

插件支持运行时更新，无需重启 Songloft 服务。

### 热更新流程

```
1. 检测到 ZIP 文件变化（mtime 改变）
2. 冻结当前服务（停止接收新消息）
3. 调用 onDeinit() 回调
4. 销毁旧的 QuickJS 虚拟机
5. 从新 ZIP 重新加载代码
6. 创建新的 QuickJS 虚拟机
7. 调用 onInit() 回调
8. 解冻服务，恢复消息处理
```

### 自动检测

系统每 30 秒轮询 `data/jsplugins/` 目录，检测 ZIP 文件 mtime 变化。若检测到变化，自动触发热更新。

### 手动触发

目前未提供独立的 `reload` 端点。重新触发热更新的常用做法：

- **开发期**：保持 `songloft-plugin dev` 运行，保存源码即可；
- **运维**：重新上传同 `entryPath` 的 ZIP（`POST /api/v1/jsplugins/upload`）或调用 `PUT /api/v1/jsplugins/{id}`，后端在更新成功后会自动对处于 `active` 状态的插件触发热重载；
- **远程更新**：调用 `POST /api/v1/jsplugins/{id}/update` 拉取 `updateUrl` 中的新版本，同样会自动热重载。

### 错误回滚

如果新版本加载失败，系统会尝试回滚到旧版本。若回滚也失败，则将插件标记为 `error` 状态。

### 注意事项

- 热更新期间，正在处理的请求会完成后再切换
- 定时器和存储状态在热更新后需要重新初始化
- 建议在 `onInit()` 中恢复必要状态

---

## 12. 最佳实践

### 性能建议

1. **避免长时间阻塞** — `onHTTPRequest` 应快速返回
2. **合理使用定时器** — 定时器回调在独立线程中执行，不阻塞 HTTP 请求。但回调中的 `fetch` 等网络操作仍会占用 VM 锁，建议避免在单次回调中执行多个串行网络请求
3. **缓存计算结果** — 使用 `songloft.storage` 缓存频繁访问的数据
4. **控制响应体大小** — 避免返回过大的 JSON 响应
5. **定时器间隔** — 建议 `setInterval` 间隔不低于 1 秒；系统每 500ms 检查一次到期定时器

### 错误处理

```javascript
function onHTTPRequest(req) {
    try {
        // 业务逻辑
        var data = processRequest(req);
        return {
            statusCode: 200,
            body: JSON.stringify(data),
            headers: { "Content-Type": "application/json" }
        };
    } catch (e) {
        songloft.log.error("Request failed: " + e.message);
        return {
            statusCode: 500,
            body: JSON.stringify({ error: e.message }),
            headers: { "Content-Type": "application/json" }
        };
    }
}
```

### 版本管理

- 遵循语义化版本（SemVer）
- 在 `plugin.json` 中设置 `updateUrl` 支持远程更新检查
- 重大变更时更新主版本号

### 开发调试

1. 查看服务器日志中 `[plugin]` 前缀的输出
2. 使用 `songloft.log.info/warn/error` 输出调试信息
3. 健康检查失败会在日志中记录

### 存储使用模式

```javascript
// 存储复杂对象
function saveConfig(config) {
    songloft.storage.set("config", JSON.stringify(config));
}

function loadConfig() {
    var raw = songloft.storage.get("config");
    return raw ? JSON.parse(raw) : { defaultKey: "defaultValue" };
}
```

### 插件间协作模式

```javascript
// 服务提供者模式
songloft.comm.onMessage("get-service", function(payload, from) {
    switch (payload.method) {
        case "translate":
            return { text: translate(payload.text) };
        case "summarize":
            return { summary: summarize(payload.text) };
        default:
            return { error: "unknown method" };
    }
});

// 服务消费者模式
function useTranslation(text) {
    var resp = songloft.comm.call("translator-plugin", "get-service", {
        method: "translate",
        text: text
    }, 5000);
    if (resp.success && resp.data) {
        return resp.data.text;
    }
    return text; // fallback
}
```

---

## 附录：完整示例

参见 [plugin-toolchain/examples/basic](https://github.com/songloft-org/plugin-toolchain/tree/main/examples/basic) 目录，包含基于官方工具链的完整示例插件代码。
