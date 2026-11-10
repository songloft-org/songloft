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
npx create-songloft-plugin@latest
# 或 pnpm create songloft-plugin
cd <你的插件目录>
npm install   # 或 pnpm install / yarn install
```

脚手架会交互式引导你完成以下配置：

1. **基本信息** — 目录名、插件显示名称、entryPath、简介、作者
2. **权限选择**（多选） — `storage`、`persistent-storage`、`songs.read`、`songs.write`、`playlists.read`、`playlists.write`、`tags.read`、`tags.write`、`inter-plugin`、`command`、`jsenv`、`fs`、`fs:music`、`fs:external`、`websocket`、`net`、`net:insecure-tls`（通配符糖 `songs.*`、`playlists.*`、`tags.*`、`fs.*` 可在 manifest 中使用）
3. **附加功能模板**（多选，可跳过） — 静态页面 (`static/`)、可执行文件管理 (`bin/`)、Lynx 原生渲染 (`renderEngine: "lynx"`，ReactLynx + 跨平台原生 UI)
4. **包管理器** — npm / pnpm / yarn

生成的项目结构（选择全部附加功能时）：

```
my-plugin/
├── plugin.json        # 插件清单（entryHash / zipHash 由 builder 生成）
├── package.json       # npm 依赖（@songloft/plugin-sdk / @songloft/plugin-builder）
├── tsconfig.json
├── src/
│   └── main.ts        # TypeScript 源码入口
├── static/            # [附加功能] 静态资源（HTML + 插件自定义 JS）
│   ├── index.html
│   └── js/
│       └── app.js
└── bin/               # [附加功能] 可执行文件管理（打包/下载/运行外部程序）
```

模板采用叠加层设计：始终包含基础模板，选中的附加功能会额外合并对应文件。

### Step 2: 编写业务逻辑

`src/main.ts` 使用 `@songloft/plugin-sdk` 提供的全局类型与 helper：

```typescript
/// <reference types="@songloft/plugin-sdk" />
import { jsonResponse, createRouter } from '@songloft/plugin-sdk';

const router = createRouter();

router.get('/hello', (req) => jsonResponse({ message: 'Hello!', query: req.query }));

router.get('/songs', async (req) => {
  const songs = await songloft.songs.list({ limit: 10 });
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
  └── js/
      └── app.js
```

> 公共资源（CSS 变量/reset/MD3 组件样式、字体、API 工具库）由主程序自动注入，插件无需打包。详见 [§8. 静态资源](#8-静态资源)。

### plugin.json 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 插件名称（2-50 字符） |
| `version` | string | 是 | 插件版本（推荐 `YYYY.M.D.N`，如 `2026.11.6.0`；兼容已有 `1.0.0`） |
| `description` | string | 否 | 插件描述 |
| `author` | string | 否 | 作者 |
| `homepage` | string | 否 | 主页 URL |
| `license` | string | 否 | 许可证 |
| `entryPath` | string | 是 | 路由前缀（小写字母+数字+连字符，如 `my-plugin`） |
| `main` | string | 是 | 入口文件路径（必须以 `.js` 结尾） |
| `minHostVersion` | string | 否 | 最低宿主版本要求 |
| `permissions` | string[] | 是 | 权限列表（可为空数组 `[]`） |
| `renderEngine` | string | 否 | 客户端渲染插件 UI 所用的引擎，`webview` / `lynx`，缺失或空串等同 `webview`。详见 [renderEngine 渲染引擎声明](#renderengine-渲染引擎声明) |
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

### renderEngine 渲染引擎声明

原生客户端渲染插件 UI 有两条路径：系统 WebView，以及 [Lynx](https://lynxjs.org/)（跨平台原生 UI，由 ReactLynx 编译产物驱动）。用哪条**由插件自己在 `plugin.json` 里声明**——宿主侧**没有**全局引擎开关，插件之间互不影响。

```json
{
  "entryPath": "my-plugin",
  "renderEngine": "lynx"
}
```

| 取值 | 含义 |
|------|------|
| 字段缺失 / `""` | 等同 `webview`，即宿主默认 |
| `"webview"` | 系统 WebView 渲染（默认） |
| `"lynx"` | Lynx 原生渲染（ReactLynx 编译为 `.lynx.bundle`，宿主通过 `<frame>` 加载；同时自动生成 `index.html` + `.web.bundle` 供不支持 Lynx 的客户端回退到 WebView） |

- **其它取值一律非法**：后端 `ValidateManifest` 阶段直接报错，插件**装不上**（不会静默回退到 `webview`）
- 插件列表 API 以 snake_case 的 `render_engine` 字段返回该值
- 该字段可随版本改：不想再用 Lynx 就发一个把它改回 `webview`（或删掉）的新版本

#### 什么时候该声明 `lynx`

**只有使用 ReactLynx 开发并经过实测的插件才声明。** Lynx 是完全不同于 WebView 的渲染路径——插件 UI 使用 ReactLynx 编写，编译产物为 `.lynx.bundle`，宿主通过 Lynx `<frame>` 元素原生加载。

声明 `lynx` 的前提：

- 使用 `pnpm create @songloft/songloft-plugin` 时选择 **"Lynx 原生渲染"** 模板（或手动搭建 rspeedy + ReactLynx 环境）
- 安装 `@songloft/lynx-plugin-sdk` 作为宿主通信 SDK（替代 WebView 下的 `@songloft/client-sdk`）
- `plugin.json` 中设置 `"staticHash": false`（防止 bundle 文件被重命名）

构建产物结构：

```
static/
├── main.lynx.bundle   # Lynx 原生 bundle（Lynx 客户端加载此文件）
├── main.web.bundle    # Web 回退 bundle（非 Lynx 客户端加载此文件）
└── index.html         # 自动生成的 WebView 回退页（引导 web-core 加载 .web.bundle）
```

**与宿主通信**：Lynx 插件不使用 `window.SongloftPlugin` / `@songloft/client-sdk`（那些是 WebView 专用），而是通过 `@songloft/lynx-plugin-sdk` 提供的 `invokeHost` / `onPlayerState` / `onThemeChange` 等 API，底层走 `NativeModules.SongloftPluginBridge` 原生桥。

**回退机制**：不支持 Lynx 的客户端（如 Flutter 版本）会打开 `index.html`，由 web-core 加载 `.web.bundle` 在 WebView 中渲染——功能等价，只是失去原生性能优势。

---

## 4. 生命周期

插件有三个核心生命周期回调函数：

### onInit()

插件加载完成后调用。用于初始化资源、设置定时器等。

```javascript
async function onInit() {
    songloft.log.info("Plugin initialized");
    await songloft.storage.set("start_time", new Date().toISOString());
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
    query: "limit=10&offset=0", // URL 查询字符串
    remoteAddr: "192.168.1.20:54321" // TCP 对端地址，不采用 X-Forwarded-For 等代理头；旧宿主/内部调用可能省略
}
```

宿主支持 UPnP 事件订阅的 `SUBSCRIBE` / `UNSUBSCRIBE` 扩展 HTTP 方法。插件可在 `onHTTPRequest` 中按 `req.method` 分发，并仅将协议端点列入 `publicPaths`；免 JWT 的端点仍须自行校验访问范围和事件回调地址。OpenAPI 2 不支持这两种方法，因此 Swagger 只在 catch-all 的描述中说明。

需要监听 SSDP 多播时，可使用 `songloft.net.udpBind({address: "0.0.0.0:1900", reuseAddress: true})` 后调用 `udpJoinMulticast(socketId, "239.255.255.250")`。`reuseAddress` 默认关闭，只设置 `SO_REUSEADDR`；同端口其他 socket 也须允许复用，否则仍会失败。旧宿主会忽略该选项。声明 `net` 权限，停用时关闭 socket；活跃 UDP socket 会阻止插件被空闲驱逐。

出站 TCP 连接使用 `songloft.net.tcpConnect(host, port, options?)`，返回带 `send()/onData()/onClose()/close()` 的 socket 句柄，同样需要 `net` 权限。数据接收复用 UDP 的 host event 推送模型；**`data` 是 base64 编码的原始字节**（send 侧 `btoa`、onData 侧 `atob`）：TCP 是字节流，一次读取可能截断在多字节 UTF-8 字符中间，raw string 会被 `json.Marshal` 替换为 U+FFFD 而永久损坏，插件需跨 chunk 累积字节后再解码。仅允许连接私有 / 回环 / 链路本地地址（防 SSRF）；与 UDP 各自独立计每插件 8 个 socket 配额，有活跃 TCP 连接的插件不会被空闲驱逐。典型用途：控制本机 MPD（6600 端口 idle 事件推送）。

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

### onWebSocket(req, socket)

客户端连接 `/api/v1/jsplugin/{entryPath}/...` 并发起 WebSocket upgrade 时调用。插件必须声明 `websocket` 权限。`onWebSocket` 应注册消息/关闭/错误回调后返回，连接生命周期由宿主托管。

**参数 `req` 结构：**

```javascript
{
    method: "GET",
    path: "/api/inbound",
    headers: {},
    query: "access_token=...",
    remoteAddr: "127.0.0.1:12345"
}
```

**`socket` 常用方法：**

- `socket.send(string | Uint8Array | ArrayBuffer)`：发送文本或二进制消息
- `socket.close(code?, reason?)`：关闭连接
- `socket.onMessage(fn)` / `socket.onClose(fn)` / `socket.onError(fn)`：注册事件回调
- `socket.onmessage = fn` / `socket.addEventListener(...)`：兼容浏览器 WebSocket 风格

**示例：Echo 服务**

```javascript
globalThis.onWebSocket = async function(req, socket) {
    socket.onMessage(async function(event) {
        await socket.send(event.data);
    });
};
```

---

## 5. API 参考

所有 API 通过全局 `songloft` 对象访问。

> **重要：所有 `songloft.*` 方法均为异步、返回 Promise，必须在 `async` 函数中 `await`。** 这与 `fetch` 等 Web 标准 API 行为一致。下文示例均置于 `async` 函数上下文中。**例外：** `songloft.log.*`（同步本地日志）和 `songloft.comm.onMessage(...)`（同步注册回调）无需 `await`。

### HTTP 请求（全局 fetch）

使用标准全局 `fetch` 函数发起 HTTP 请求（由运行时 polyfill 提供，返回 Promise）。**无需声明权限**。

```javascript
// GET
const resp = await fetch("https://example.com/api");
const data = await resp.json();

// POST
const postResp = await fetch("https://example.com/api", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ hello: "world" })
});
const text = await postResp.text();
```

请求头里可以使用三个运行时内部控制头，它们只影响运行时行为，都不会转发给目标服务器：

| 控制头 | 作用 |
|--------|------|
| `X-Fetch-No-Redirect` | 禁止自动跟随重定向，由 JS 侧手动处理重定向链（收集中间跳的 `Set-Cookie` 必需——Go 的 net/http 会吞掉它们） |
| `X-Fetch-Timeout-Ms` | 设置单次请求超时（100-30000ms） |
| `X-Fetch-Insecure` | 跳过 TLS 证书校验。**需要 `net:insecure-tls` 权限**，未声明时该头被静默忽略并保持完整校验（宿主会打一条 warn 日志） |

> `X-Fetch-Insecure` 的用途是自建 NAS 类设备：飞牛 fnOS（5667 端口）、群晖等默认自签证书，且插件通常按**裸 IP** 访问它们——即便设备装了 CA 签发的正式证书，证书主体也是域名，按 IP 连必然 hostname mismatch，没有「让用户换个正规证书」的出路。除此之外不要用它，跳过校验意味着放弃 MITM 防护。

**`Response` 对象字段：**
- `ok` — `status >= 200 && status < 300`
- `status` — HTTP 状态码
- `statusText` — 状态文本
- `headers` — 响应头对象，见下
- `json()` — 返回 `Promise<unknown>`，解析 JSON
- `text()` — 返回 `Promise<string>`，原始文本

**`headers` 的读取方式**

`headers` 既可以按属性名直接取值，也可以用标准 `Headers` 的读取方法：

```javascript
resp.headers['Content-Type']          // 属性式：key 是 Go canonical 形式（Set-Cookie / Content-Type）
resp.headers.get('content-type')      // 方法式：大小写不敏感，多值合并为 ", " 串，缺失返回 null
resp.headers.has('set-cookie')        // 是否存在该头
resp.headers.getSetCookie()           // 所有 Set-Cookie 的原始数组（无 Set-Cookie 时为空数组）
resp.headers.forEach(function(value, name) { /* ... */ });
```

> **多条 Cookie 必须用 `getSetCookie()`，不要用 `get('set-cookie')` 再自己按 `", "` 切。**
> 多值合并是不可逆的：cookie 的 `Expires=Wed, 21 Oct 2026 07:28:00 GMT` 属性本身就含 `", "`，
> 切分结果无法与条目分隔符区分。`getSetCookie()` 直接返回逐条完整的原始数组。

这些方法**不可枚举**，因此 `Object.keys(resp.headers)`、`for...in`、`JSON.stringify(resp.headers)`
的结果只含响应头本身，不会出现方法名。

> **TypeScript 下优先用 `.get()` 而非属性式取值。** SDK 把 `fetch` 声明为返回标准 DOM `Response`，
> 其 `headers` 类型是 `Headers`，没有索引签名——`resp.headers['Content-Type']` 虽然运行时可用，
> 但类型检查不过，只能靠 `as unknown as Record<string, string>` 绕过。`.get()` 两边都合法。

`onHTTPRequest`、`onWebSocket` 和事件回调都可以是 `async function`，框架会等待 Promise settle。

### songloft.net.httpStream — 可取消的分块 HTTP 请求

声明 `net` 权限。`httpStream` 同步返回句柄，`response` 等待响应头，`read()` 每次最多返回 16 KiB 原始字节，`null` 表示结束；一次只能有一个读取在等待。`cancel()` 幂等，可取消连接建立及正在等待的读取。宿主最多保留每插件 8 个请求、64 MiB 响应、4 MiB 字符串请求体；`timeoutMs` 是连接与整个响应体的总期限（100–30000 ms，默认 30000）。

请求复用 `fetch` 的 TLS、重定向和内部控制头规则，`timeoutMs` 覆盖内部超时头。响应头是保留多值的 `Record<string, string[]>`。EOF、错误、超时、卸载和休眠会回收请求；活跃请求阻止空闲休眠。读取由消费者拉取，不把响应块无限推入 JS 事件队列。旧宿主没有该方法，调用前检测能力。它不解析 SSE，也不自动重连；跨块 UTF-8 解码与协议解析由调用方完成。

```typescript
if (typeof songloft.net.httpStream === 'function') {
  const request = songloft.net.httpStream('https://example.com/events', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ stream: true }), timeoutMs: 30000,
  });
  try {
    const { status, headers } = await request.response;
    if (status < 200 || status >= 300) throw new Error(`HTTP ${status}`);
    for (;;) {
      const chunk = await request.read();
      if (chunk === null) break;
      // Feed bytes to an incremental protocol parser.
    }
  } finally {
    await request.cancel();
  }
}
```

### Crypto（全局 crypto）

运行时提供轻量 `crypto` 工具对象。**无需声明权限**。

```javascript
const md5 = crypto.md5("data");
const sha256 = crypto.sha256Bytes(Buffer.from("data", "utf8")).toString("hex");

const key = Buffer.from("1234567890abcdef", "utf8");
const iv = Buffer.from("abcdef1234567890", "utf8");
const encrypted = crypto.aesEncrypt("hello", "cbc", key, iv).toString("base64");
const decrypted = crypto.aesDecrypt(encrypted, "cbc", key, iv).toString("utf8");
```

常用方法：`md5(str)`、`sha1(str)`、`sha256Bytes(buffer)`、`rc4(key, data)`、`aesEncrypt(buffer, "cbc" | "ecb", key, iv?)`、`aesDecrypt(buffer, "cbc" | "ecb", key, iv?)`、`rsaEncrypt(buffer, publicKeyPEM)`、`randomBytes(size)`。AES 使用 PKCS7 padding；`aesDecrypt` 的字符串密文默认按 base64 解析，传入 `Buffer` 时按原始字节解析。

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

**注意：** 后台 goroutine 每 500ms 尝试用 TryLock 取得 VM 锁，处理到期定时器；锁忙时跳过这一轮。回调与 HTTP 中的 JS 共用同一 VM，长时间同步计算仍会占锁。`setInterval` 的最小间隔被限制为 10ms。

### songloft.storage — 持久化存储

需要权限：`storage`

```javascript
async function storageExample() {
    // 读取值（异步返回原类型值或 null）
    var value = await songloft.storage.get("key");

    // 写入值（值经 JSON 自动序列化，可直接存对象/数组）
    await songloft.storage.set("config", { volume: 80, list: [1, 2, 3] });

    // 删除键
    await songloft.storage.delete("key");

    // 获取所有键名
    var keys = await songloft.storage.keys();  // ["key1", "key2", ...]
}
```

**存储限制：**
- 键名为字符串
- 值经 JSON 自动序列化，可直接存对象/数组/数字；`get` 异步返回原类型值或 null
- 每个插件有独立的存储空间

### songloft.songs — 歌曲操作

需要权限：`songs.read`

```javascript
async function songsExample() {
    // 获取歌曲列表
    var songs = await songloft.songs.list({ limit: 20, offset: 0 });

    // 根据 ID 获取歌曲
    var song = await songloft.songs.getById(123);

    // 搜索歌曲
    var results = await songloft.songs.search("关键词");
}
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
    cover_url: "",        // 封面 URL（CoverPath 内部字段不会序列化输出）
    is_video: false       // 是否为视频容器
}
```

### songloft.playlists — 歌单操作

需要权限：`playlists.read`（读取）或 `playlists.write`（修改）；或者通配符糖 `playlists.*`。

```javascript
async function playlistsExample() {
    // 需要 playlists.read
    var playlists = await songloft.playlists.list();
    var playlist = await songloft.playlists.getById(1);
    var songs = await songloft.playlists.getSongs(1, { limit: 50, offset: 0 });
}
```


### 歌单原子同步与延迟元数据探测

包含 #558 支持的宿主提供 `await songloft.playlists.replaceSongs(playlistId, songIds)`（`playlists.write`）。仅普通歌单可用；成员及顺序在一个 UnitOfWork 事务中替换，空数组清空，缺失/重复/类型不匹配的 ID 拒绝，任一写入失败均回滚。内置歌单禁止使用此整表替换接口，原有增删及排序接口不受影响。

目录同步可使用 `await songloft.songs.create(inputs, { probeMetadata: false })`（`songs.write`），仅导入歌曲信息，不触发后台音频探测。省略选项时保持原有自动补齐元数据行为。该选项不下载文件，也不改变播放时的地址解析。旧宿主缺少原子接口时，应停止同步并提示升级，不要退回「先清空、后添加」。

### songloft.tags — 自定义标签

需要权限：`tags.read`（读取）或 `tags.write`（修改）；或者通配符糖 `tags.*`。

包含 #562 支持的宿主允许歌曲与歌单复用同一标签定义。`song_count` 为歌曲数，`playlist_count` 为可见歌单数；歌单引用存于现有 `labels` 的 `tag:<id>`，保留系统标记。绑定关系走标签接口，普通元数据编辑保留当前绑定；删除标签在同一事务中清理两类关联。

```javascript
async function tagsExample() {
    // —— 读取操作（需要 tags.read）——

    // 获取标签列表（支持筛选、排序、分页）
    var tags = await songloft.tags.list({
        keyword: "",     // 可选，按名称模糊搜索
        orderBy: "",     // 可选，排序字段
        order: "",       // 可选，"asc" | "desc"
        limit: 100,      // 可选，默认 100
        offset: 0        // 可选，默认 0
    });

    // 根据 ID 获取单个标签
    var tag = await songloft.tags.getById(1);

    // 获取指定歌曲关联的所有标签
    var songTags = await songloft.tags.getSongTags(42);

    // —— 写入操作（需要 tags.write）——

    // 创建标签
    var newTag = await songloft.tags.create({ name: "我的标签", color: "#ff6600" });

    // 更新标签
    await songloft.tags.update(1, { name: "新名称", color: "#00cc00" });

    // 删除标签
    await songloft.tags.delete(1);

    // 批量绑定歌曲到标签（返回 { bound: <实际绑定数> }）
    var result = await songloft.tags.bindSongs(1, [10, 20, 30]);

    // 批量解绑歌曲（返回 { unbound: <实际解绑数> }）
    var result2 = await songloft.tags.unbindSongs(1, [10, 20]);
}
```

**方法参考：**

| 方法 | 权限 | 参数 | 返回 |
|------|------|------|------|
| `list(options?)` | `tags.read` | `{ keyword?, orderBy?, order?, limit?, offset? }` | Tag[] |
| `getById(id)` | `tags.read` | `id: number` | Tag |
| `getSongTags(songId)` | `tags.read` | `songId: number` | Tag[] |
| `getPlaylistTags(playlistId)` | `tags.read` | `playlistId: number` | Tag[] |
| `bindPlaylists(tagId, playlistIds)` | `tags.write` | `number, number[]` | `{ bound: number }` |
| `unbindPlaylists(tagId, playlistIds)` | `tags.write` | `number, number[]` | `{ unbound: number }` |
| `create(options)` | `tags.write` | `{ name: string, color: string }` | Tag |
| `update(id, fields)` | `tags.write` | `number, { name?: string, color?: string }` | void |
| `delete(id)` | `tags.write` | `id: number` | void |
| `bindSongs(tagId, songIds)` | `tags.write` | `number, number[]` | `{ bound: number }` |
| `unbindSongs(tagId, songIds)` | `tags.write` | `number, number[]` | `{ unbound: number }` |

### songloft.comm — 插件间通信

需要权限：`inter-plugin`

```javascript
async function commExample() {
    // 发送消息（fire-and-forget）
    await songloft.comm.send("target-plugin", "action-name", { data: "hello" });

    // 请求-响应调用（等待响应，超时默认 10s）
    var resp = await songloft.comm.call("target-plugin", "action-name", { data: "hello" }, 5000);
    // resp = { success: true, data: { ... } }
}

// 注册消息处理器（同步注册，无需 await）
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
async function pluginInfoExample() {
    // 获取插件的 JWT Token（用于访问宿主 API，如音乐文件、封面等需认证的资源）
    var token = await songloft.plugin.getToken();

    // 获取宿主服务的基础 URL（如 http://192.168.1.100:58091）
    var hostUrl = await songloft.plugin.getHostUrl();
}
```

**典型用法：构建带认证的资源 URL**

```javascript
async function getMusicUrl(songId) {
    var host = await songloft.plugin.getHostUrl();
    var token = await songloft.plugin.getToken();
    return host + "/music/" + encodedPath + "?access_token=" + token;
}
```

**方法说明：**
- `getToken()` — 返回当前有效的 JWT access_token 字符串，可用于访问宿主的受保护 API
- `getHostUrl()` — 返回宿主服务的基础 URL，用于构建完整的 API 或资源地址

### songloft.lyrics — 歌词提供者

无需权限。

插件可以注册为歌词提供者，在歌曲没有歌词时由宿主自动调用。

#### 注册与取消

```javascript
// 注册为歌词提供者
songloft.lyrics.registerProvider();

// 取消注册
songloft.lyrics.unregisterProvider();
```

#### 实现搜索端点

注册后，宿主会通过 `InvokeHTTP` 调用插件的 `/lyric-search` 端点搜索歌词。插件需自行实现该路由。

**请求参数（Query String）：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `title` | string | 歌曲标题 |
| `artist` | string | 艺术家 |
| `album` | string | 专辑名 |
| `duration` | number | 时长（秒） |
| `fingerprint` | string | 音频指纹（Chromaprint，可选，有值时才传） |
| `isrc` | string | ISRC 国际标准录音编码（可选，有值时才传） |

**响应格式（HTTP 200 + JSON）：**

```json
{
  "lyric": "[00:01.00]歌词第一行\n[00:05.00]歌词第二行",
  "tlyric": "[00:01.00]翻译第一行",
  "rlyric": "[00:01.00]罗马音第一行",
  "lxlyric": "[00:01.00]逐字歌词"
}
```

- `lyric`（必填）：主歌词，LRC 格式
- `tlyric`（可选）：翻译歌词
- `rlyric`（可选）：罗马音歌词
- `lxlyric`（可选）：逐字歌词

无结果时返回 HTTP 404 或空 body。

#### 完整示例

```typescript
/// <reference types="@songloft/plugin-sdk" />
import { createRouter, jsonResponse, parseQuery } from '@songloft/plugin-sdk';

const router = createRouter();
let registered = false;

router.get('/lyric-search', async (req: HTTPRequest) => {
  const q = parseQuery(req.query);
  const result = await searchFromMySource(
    q.title, q.artist, q.album,
    parseFloat(q.duration) || 0,
    q.fingerprint,  // 可选，用于精确匹配
    q.isrc           // 可选，用于精确匹配
  );
  if (!result) return jsonResponse(null, 404);
  return jsonResponse(result);  // { lyric, tlyric?, rlyric?, lxlyric? }
});

globalThis.onInit = async () => {
  songloft.lyrics.registerProvider();
  registered = true;
};

globalThis.onDeinit = async () => {
  if (registered) songloft.lyrics.unregisterProvider();
};

globalThis.onHTTPRequest = (req: HTTPRequest) => router.handle(req);
```

#### 工作流程

1. 用户播放无歌词的歌曲，客户端请求 `GET /api/v1/songs/{id}/lyric`
2. 宿主发现歌词为空，遍历所有已注册的歌词提供者插件
3. 对每个插件调用 `GET /lyric-search?title=...&artist=...`（15 秒超时）
4. 第一个返回 HTTP 200 + 非空歌词的插件胜出，停止遍历
5. 搜到的歌词异步写入数据库缓存（`lyric_source=scraped`），后续请求直接返回缓存
6. 本地歌曲还会将歌词嵌入音频文件标签

### songloft.covers — 封面提供者

无需权限。

插件可以注册为封面提供者，在歌曲没有封面时由宿主自动调用。

#### 注册与取消

```javascript
// 注册为封面提供者
songloft.covers.registerProvider();

// 取消注册
songloft.covers.unregisterProvider();
```

#### 实现搜索端点

注册后，宿主会通过 `InvokeHTTP` 调用插件的 `/cover-search` 端点搜索封面。插件需自行实现该路由。

**请求参数（Query String）：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `title` | string | 歌曲标题 |
| `artist` | string | 艺术家 |
| `album` | string | 专辑名 |
| `fingerprint` | string | 音频指纹（Chromaprint，可选，有值时才传） |
| `isrc` | string | ISRC 国际标准录音编码（可选，有值时才传） |

**响应格式（HTTP 200 + JSON）：**

```json
{
  "cover_url": "https://example.com/covers/album.jpg"
}
```

- `cover_url`（必填）：封面图片的完整 URL

无结果时返回 HTTP 404 或空 body。

#### 完整示例

```typescript
/// <reference types="@songloft/plugin-sdk" />
import { createRouter, jsonResponse, parseQuery } from '@songloft/plugin-sdk';

const router = createRouter();
let registered = false;

router.get('/cover-search', async (req: HTTPRequest) => {
  const q = parseQuery(req.query);
  const coverUrl = await searchCoverFromMySource(
    q.title, q.artist, q.album,
    q.fingerprint,  // 可选，用于精确匹配
    q.isrc           // 可选，用于精确匹配
  );
  if (!coverUrl) return jsonResponse(null, 404);
  return jsonResponse({ cover_url: coverUrl });
});

globalThis.onInit = async () => {
  songloft.covers.registerProvider();
  registered = true;
};

globalThis.onDeinit = async () => {
  if (registered) songloft.covers.unregisterProvider();
};

globalThis.onHTTPRequest = (req: HTTPRequest) => router.handle(req);
```

#### 工作流程

1. 用户播放无封面的歌曲，客户端请求 `GET /api/v1/songs/{id}/cover`
2. 宿主发现封面为空，遍历所有已注册的封面提供者插件
3. 对每个插件调用 `GET /cover-search?title=...&artist=...`（15 秒超时）
4. 第一个返回 HTTP 200 + 非空 `cover_url` 的插件胜出，停止遍历
5. 搜到的封面异步持久化：
   - **本地歌曲**：下载封面图片 → 保存到本地 `cover_path` → 嵌入音频文件标签
   - **远程歌曲**：存储 `cover_url` 到数据库
6. 后续请求直接返回缓存，不再调用插件

### 提供者机制通用说明

歌词和封面提供者共享相同的架构：

- **多插件支持**：多个插件可同时注册为同一类型的提供者，宿主按 first-match-wins 策略依次尝试
- **空闲驱逐安全**：插件被空闲驱逐（内存回收）后，提供者注册不会丢失；下次搜索时宿主会自动重新加载插件
- **惰性清理**：已禁用或已删除的插件会在搜索遍历时被自动从提供者集合中移除
- **指纹与 ISRC**：建议插件优先使用 `fingerprint` 和 `isrc` 进行精确匹配（如果有值），再 fallback 到 title/artist 模糊搜索

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
| `tags.read` | 读取自定义标签及歌曲-标签关联 |
| `tags.write` | 创建/修改/删除标签、绑定/解绑歌曲 |
| `tags.*` | 标签读写通配符（一把梭糖） |
| `inter-plugin` | 插件间通信 |
| `command` | 执行外部命令/管理可执行文件 |
| `jsenv` | 创建/执行子 JS 沙箱环境 |
| `fs` | 读写插件数据目录内文件 |
| `fs:music` | 访问 music_path 音乐目录 |
| `fs:external` | 访问管理员配置的外部目录 |
| `websocket` | 使用 `new WebSocket(...)` 主动连接外部服务，或处理入站 `onWebSocket` upgrade |
| `persistent-storage` | 读写卸载插件后仍保留的持久化存储 |
| `net` | 使用网络 socket（UDP / 出站 TCP）及可取消的分块 HTTP 请求 |
| `net:insecure-tls` | 允许 `fetch` 带 `X-Fetch-Insecure` 跳过 TLS 证书校验（自签 / 裸 IP 访问的自建设备）。**不被 `net` 覆盖，必须单独声明** |
| `fs.*` | 文件系统通配符（覆盖 `fs`、`fs:music`、`fs:external`） |

> 注意：网络请求 (`fetch`)、定时器 (`setTimeout/setInterval`)、日志等能力**无需权限声明**，是默认宿主能力。

### 通配符糖

以 `.*` 结尾的权限在声明层作为一把梭糖，runner 在检查时用前缀匹配。例如声明 `playlists.*`
既包括 `playlists.read` 也包括 `playlists.write`；而单声明 `playlists.read` 时无法调用写接口。
当前可用的通配符糖：`songs.*`、`playlists.*`、`tags.*`、`fs.*`。

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
async function notifyB() {
    await songloft.comm.send("plugin-b", "data-updated", { source: "plugin-a" });
}
```

### 同步调用（Call）

发送方等待接收方处理并返回结果：

```javascript
// Plugin A: 调用 Plugin B 的服务
async function fetchFromB() {
    var response = await songloft.comm.call("plugin-b", "get-data", { id: 123 }, 5000);
    if (response.success) {
        var data = response.data;
    }
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
    └── js/
        └── app.js       # 插件自定义逻辑
```

> 公共资源（设计令牌/reset/MD3 组件样式、字体文件、API 工具库 `common.js`）由主程序自动注入，**无需**在插件中打包。

### 主程序自动注入

后端在返回插件 HTML 页面时，会在 `<head>` 顶部自动注入以下内容（按顺序）：

1. **`<base>`** — 设置相对路径基准，HTML 中可直接用相对路径引用 `static/...` 和插件 API
2. **Auth bridge 脚本** — 从 URL `?access_token=` 写入 localStorage、fetch 503 自动重试
3. **`theme.css`** — MD3 颜色令牌（含亮/暗双主题）、字体声明、CSS reset、`body` 基样、安全区默认值
4. **`components.css`** — 与客户端 Flutter 组件对齐的共享组件库（`.card`/`.btn*`/`.switch`/`.tab-bar` 等）
5. **`common.js`** — embed 检测、主题桥接、`window.SongloftPlugin` 全局 API

因此插件 HTML **不需要**：
- `<link>` 引用 fonts.css 或 style.css（主程序提供）
- embed 检测脚本（主程序提供）
- 打包字体文件（主程序通过 `/api/v1/jsplugin-assets/fonts/` 提供）

### window.SongloftPlugin — 浏览器端全局 API

主程序注入的 `common.js` 暴露 `window.SongloftPlugin` 全局对象，提供以下方法：

```javascript
// API 请求
SongloftPlugin.getAuthToken()        // 从 localStorage 读取 JWT token
SongloftPlugin.apiGet(path)          // GET 请求，返回 Promise<JSON>
SongloftPlugin.apiPost(path, body)   // POST 请求
SongloftPlugin.apiPut(path, body)    // PUT 请求
SongloftPlugin.apiDelete(path)       // DELETE 请求

// 主题
SongloftPlugin.getTheme()            // 返回 'light' | 'dark'
SongloftPlugin.onThemeChange(cb)     // 监听主题变化，cb(theme: 'light' | 'dark')
SongloftPlugin.getColorScheme()      // 宿主真实色板 {primary:'#415F91', ...}；未下推时 null
```

插件 JS 可直接使用：

```javascript
const { apiGet, getTheme, onThemeChange } = SongloftPlugin;

const data = await apiGet('/api/hello');
console.log('当前主题:', getTheme());
onThemeChange(theme => console.log('主题切换到:', theme));
```

如果插件有多个 JS 文件，每个文件顶部直接从全局解构即可：

```javascript
const { apiGet, apiPost } = SongloftPlugin;
```

### 客户端 SDK —— 调用宿主播放器（webview 页面专用）

在 Songloft 客户端中打开的插件页面，可通过 `window.SongloftPlugin.host` / `.player` 调用宿主客户端能力——最常见的是改写宿主的「正在播放队列」。

> - 生效范围：**native 客户端**（Android/iOS/macOS/Windows）的 webview 插件页；**Web 端插件页**（Tab 内嵌页与首页/全屏页均在宿主 iframe 内打开，走 postMessage 桥接）。**Linux 桌面当前没有 WebView 渲染引擎**：客户端不挂载插件页、改展示降级视图，用户需走「在浏览器中打开」（songloft-org/songloft-player#47）。
> - 不生效：仅当用户通过「在浏览器中打开」把插件页在独立新浏览器标签打开时（无宿主父窗口）——此时 `host.isAvailable()` 返回 `false`，调用会抛错，务必先 feature-detect。
> - 能力由宿主客户端注入，跟随客户端版本。请在 `plugin.json` 设置合适的 `minHostVersion`，并用 `host.getInfo().capabilities` 做能力协商。

```javascript
const { host, player } = SongloftPlugin;

if (host && host.isAvailable()) {
  // 能力协商
  const info = await host.getInfo();   // { version, platform, capabilities: ['player'] }

  // 用歌曲 id 替换正在播放队列并从第 0 首开始播（id 通常来自你自己的搜索结果，
  // 先经服务端 songs.create 持久化拿到 id）
  await player.setQueue([101, 102, 103], { startIndex: 0 });

  // 追加到队列末尾（不打断当前播放）
  await player.addToQueue([104]);

  // 读取状态 / 订阅状态变更
  const state = await player.getState();     // { queue, current_index, is_playing, ... }
  const off = player.onStateChange(s => console.log('当前第', s.current_index, '首'));
}
```

`player` 命名空间方法：`getState` / `setQueue` / `addToQueue` / `insertToQueue` / `removeFromQueue` / `reorderQueue` / `clearQueue` / `play(id?)` / `pause` / `togglePlay` / `next` / `prev` / `seek(seconds)` / `setVolume(0-100)` / `setPlayMode('order'|'loop'|'single'|'random'|'singlePlay')` / `playPlaylistById(id)` / `onStateChange(cb)`。

用 TypeScript / 构建工具（如 Vue 模板）开发时，安装 [`@songloft/client-sdk`](https://github.com/songloft-org/plugin-toolchain/tree/main/packages/client-sdk) 获得完整类型与便捷封装：

```ts
import { player, host, isClient } from '@songloft/client-sdk';

if (isClient()) {
  await player.setQueue([101, 102], { startIndex: 0 });
}
```

免构建的 vanilla 静态页面无需安装，直接用注入的 `window.SongloftPlugin.player` 即可（仅少了类型提示）。

### 收藏状态同步 —— 改完收藏必须通知宿主

插件自己改收藏（如直接 POST `/playlists/1/songs`）时，服务端数据是对的，但 Flutter 侧曲库的红心读的是 `FavoriteNotifier` 的**内存缓存**，不会自动跟着变。改完必须调一次 `favorite.refresh`：

```js
const res = await SongloftPlugin.apiPost('/playlists/1/songs', { song_id: id });
await SongloftPlugin.favorite.refresh(id, res.is_favorited);
```

**能带参就带参**：带 `(songId, isFavorited)` 是增量更新，宿主只改这一首的归属；不带参是全量重载，曲库上千首时是一次完整的往返。

### 通用宿主调用 `invokeHost`

上面的 `player` / `host` / `favorite` / `getCookies` 都是 `invokeHost(ns, method, params?)` 的 typed wrapper。宿主的分发表在**客户端**侧，可能比服务端这份 `common.js` 更新，所以 `invokeHost` 也公开出来，让插件能触达尚未被 wrapper 覆盖的 namespace：

```js
await SongloftPlugin.invokeHost('favorite', 'refresh', { songId: 42, isFavorited: true });
```

> ⚠️ 它没有 wrapper 那层类型约束，`ns` / `method` 拼错只会在运行时 reject。**有对应 wrapper 时优先用 wrapper。**
>
请始终使用公开的 `invokeHost`。

### Cookie 读取桥 —— 获取第三方站点登录态（native 专用）

插件若需要第三方站点的会话 Cookie（如 FN Connect 网关的 `os-access-code`、自建 NAS 的登录态等），可使用 `getCookies` 桥接——由宿主原生层从 WebView Cookie Store 读取，不受浏览器同源策略和 HttpOnly 限制。

```javascript
// 前提：用户已在应用内 WebView 中打开目标站点并完成登录
const cookies = await SongloftPlugin.getCookies('https://pcyear.5ddd.com');
// cookies: { 'os-access-code': 'xxx', 'music-token': 'yyy', ... }
```

**参数说明：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `origin` | `string` | 目标站点 origin，必须含协议+主机（+端口），如 `https://example.com`。路径忽略 |

**返回值：** `Promise<Record<string, string>>` — Cookie 名→值映射。该 origin 无 Cookie 时返回空对象 `{}`。

**平台限制：**

| 平台 | 支持 | 说明 |
|------|------|------|
| Android / iOS / macOS / Windows | ✅ | 原生 `CookieManager` 读取 WebView Cookie Store |
| Linux | ❌ | 无 WebView 渲染引擎，调用会 reject |
| Web | ❌ | 浏览器同源策略硬限制，调用会 reject |

> ⚠️ 使用前建议检测平台：
> ```javascript
> const info = await SongloftPlugin.host.getInfo();
> if (info.platform !== 'web') {
>   const cookies = await SongloftPlugin.getCookies(origin);
> }
> ```

**典型使用流程（以 FN Connect 为例）：**

1. 用户添加飞牛音乐音源，插件拼出 origin（如 `https://pcyear.5ddd.com`）
2. 插件引导用户在应用内 WebView 中打开目标站点并登录
3. 登录完成后，插件调用 `getCookies(origin)` 获取会话 Cookie
4. 将 Cookie 存入插件后端配置（通过 `apiPost` 等），后续请求携带该会话

**TypeScript 用法：**

```ts
import { getCookies, host } from '@songloft/client-sdk';

const info = await host.getInfo();
if (info.platform !== 'web') {
  const cookies = await getCookies('https://pcyear.5ddd.com');
  // 传给插件后端保存
  await SongloftPlugin.apiPost('/config/cookies', cookies);
}
```

### 主题适配

主程序的 `theme.css` 在 `:root` 下定义了 `--md-*` CSS 变量（亮色），并在 `html[data-theme="dark"]` 下覆盖为暗色值。插件页面使用这些变量即可自动适配主题：

```css
/* 插件自定义样式 — 引用 --md-* 变量自动跟随主题 */
.my-card {
    background: var(--md-surface-container);
    color: var(--md-on-surface);
    border: 1px solid var(--md-outline-variant);
}
```

主题变化时（用户在主程序设置中切换），`common.js` 会：
1. 更新 `<html>` 的 `data-theme` 属性和 `theme-light`/`theme-dark` CSS class
2. 用宿主推来的真实色板改写 `--md-*`（见下）
3. 派发 `songloft-theme-change` CustomEvent
4. 写入 `localStorage['songloft-theme']`

插件 JS 可通过 `SongloftPlugin.onThemeChange(callback)` 监听主题变化做额外处理。

#### 玻璃外观与辅助功能

宿主消息的可选 `appearance.reduceTransparency` / `appearance.increaseContrast` 只接受布尔值，`common.js` 将它们映射为 `<html>` 的 `data-reduce-transparency` / `data-increase-contrast`（`"true"` 或 `"false"`），并派发 `songloft-theme-appearance-change`。字段缺失或格式无效时移除属性，表示宿主未提供信息；这些偏好不写入 localStorage，也不从上次宿主会话恢复。插件应将宿主偏好与浏览器 `prefers-reduced-transparency` / `prefers-contrast` 作 OR，并在减少透明度、增强对比度或不支持模糊时提供实心背景。

Lynx 客户端发送有效材质及这两个标志，并在材质档位、主题包和辅助功能变化时同步整份主题消息。Flutter 客户端也发送有效偏好：减少透明度按设备保存，增强对比度取本机开关与 Flutter 系统高对比度信号的 OR；原生 WebView 与 Web iframe 均随主题重建推送，旧版 Flutter 消息仍兼容上述缺省规则。`--sl-theme-glass-fill` 是宿主材质色，不保证插件文字在任意背景上的对比度；浮动播放器应保护自身填充和文字对比度，并把模糊放在独立装饰层，避免改变 fixed 弹层的包含块。

公共桥接回归测试：`node --test scripts/tests/plugin-theme-appearance.test.mjs`。

#### 变量清单

`--md-*` 与 Flutter 的 `ColorScheme` 字段**逐一对应**（camelCase → kebab-case），方便两侧对照：

| 分组 | 变量 |
|---|---|
| 主色 | `--md-primary` `--md-on-primary` `--md-primary-container` `--md-on-primary-container` |
| 次色 | `--md-secondary` `--md-on-secondary` `--md-secondary-container` `--md-on-secondary-container` |
| 第三色 | `--md-tertiary` `--md-on-tertiary` `--md-tertiary-container` `--md-on-tertiary-container` |
| 错误 | `--md-error` `--md-on-error` `--md-error-container` `--md-on-error-container` |
| 表面 | `--md-surface` `--md-on-surface` `--md-on-surface-variant` `--md-surface-dim` `--md-surface-bright` |
| 表面阶梯 | `--md-surface-container-lowest` `--md-surface-container-low` `--md-surface-container` `--md-surface-container-high` `--md-surface-container-highest` |
| 描边 / 反色 | `--md-outline` `--md-outline-variant` `--md-inverse-surface` `--md-on-inverse-surface` `--md-inverse-primary` |
| 本项目自有（M3 无此角色，**不参与下推**） | `--md-success` `--md-success-container` `--md-warning` `--md-warning-container` |
| 派生别名（组件在用：switch 轨道 / progress 底） | `--md-surface-variant`→`surfaceContainerHighest` |
| 圆角刻度（对齐 Flutter `AppRadius`） | `--md-radius-sm` 8 · `-md` 12 · `-lg` 16 · `-xl` 24 · `-xxl` 28 · `-full` 50px |
| 阴影 | `--md-shadow-1` `--md-shadow-2` `--md-shadow-3` |

> 旧的 `--md-surface-1` / `--md-surface-2` 别名已移除，请直接写 `--md-surface-container` / `--md-surface-container-high`。

**`surface` 与 `surface-container` 的关系别搞反**：`--md-surface` 是**页面底色**，卡片 / 输入框 / hover 用 `container` 阶梯依次加深。公共 `.card` 已是 `SectionCard` 形制（描边无阴影 + 16 圆角），组标题用卡外的 `.section-title`（大写小字）——直接套用即与客户端一致，通常无需自定义：

```css
/* 仅当要自绘卡片时才需要，公共 .card 已经是这个形制 */
.my-section-card {
    background: var(--md-surface-container);
    border: 1px solid var(--md-outline-variant);
    border-radius: var(--md-radius-lg);
}
```

#### 宿主实时下推真实色板

`theme.css` 里的静态值只是**首帧兜底**（由默认 seed `#415F91` 导出）。页面就绪后宿主会把**真实的** `ColorScheme` 随 `songloft-theme` 消息推来——含用户自定义 ThemePack——写成 `documentElement` 的**内联**自定义属性。内联优先级最高，连插件自己在 `:root` 里重定义的同名变量也会被压住。

于是纯 CSS 的插件**什么都不用改**就与主程序同色。要在 JS 里读色则必须用 `getColorScheme()`：

```javascript
// 宿主还没推到时返回 null（此时页面用的是静态兜底色）
const cs = SongloftPlugin.getColorScheme();
const primary = (cs && cs.primary) || '#415F91';

// 到达 / 变更时收通知。注意事件派发在 document 上且**不冒泡**，监听 window 收不到。
document.addEventListener('songloft-color-scheme-change', e => {
    console.log('新主色:', e.detail.colors.primary);
});
```

> ⚠️ 这是插件用 JS 读色的可靠途径——`getComputedStyle` 对 CSS 自定义属性的返回值没有跨环境契约，不要依赖它拿色值。
>
> 色板保证在 `songloft-theme-change` 派发**之前**就已落地，所以在 `onThemeChange` 回调里直接调 `getColorScheme()` 拿到的一定是新值。切换主题包时亮暗可能没变、只有色值变——那种情况只有 `songloft-color-scheme-change` 会派发，两个事件都监听才完整。

### 访问路径

安装后，静态文件通过以下路径访问（注意：运行时路由是单数 `jsplugin`，与管理 API `/api/v1/jsplugins`（复数）不同）：

```
GET /api/v1/jsplugin/{entryPath}/                 → static/index.html（自动注入）
GET /api/v1/jsplugin/{entryPath}/static           → static/index.html
GET /api/v1/jsplugin/{entryPath}/static/<file>    → 任意静态资源
GET /api/v1/jsplugin-assets/*                     → 主程序公共资源（CSS/JS/字体）
```

### 注意事项

- 静态文件在安装时从 ZIP 解压到 `data/jsplugins_data/{entryPath}/static/`
- 更新插件时会重新解压静态文件
- 建议使用相对路径引用插件 API
- 公共资源由主程序提供，插件不需要也不应该打包自己的 CSS 变量/字体/API 工具库

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

### 推荐版本与自动发布规则

推荐插件采用 **北京时间 `YYYY.M.D.N`**：如 `2026.11.6.0`。月份和日期不补零，`N` 是当日从 `0` 开始的递增序号；同日 `.9` 后是 `.10`，换日从 `.0` 开始。Git tag 使用 `v2026.11.6.0`，`plugin.json`、`package.json` 和锁文件中的版本不带 `v`。这是插件专用的四段日期版本规则，不用于主程序版本，也不称为 SemVer；已有 SemVer 插件可以保留原有版本体系。

推荐的 CI 行为：

1. `main` 的每个非文档业务提交单独出包；批量推送或构建期间收到新提交，按主线顺序逐个处理，不只发布最后一笔。仅修改 `.md` / `.mdx` 或 `docs/` 的提交不出包、不占序号；自动生成的版本提交不递归触发。工作流触发过滤与补发队列均须应用文档规则，手动续跑也不补发纯文档提交。
2. 从已有 tag 的数值最大序号分配新版本，不覆盖既有 tag 或 Release、不复用已分配序号；失败留下序号间隔可以接受。首次发布须处理无 tag 的根提交；已有插件可用上次发布的源码或 tag 作为起点，不虚构基线。
3. 对每笔源码建立隔离构建快照，完成适用的类型检查、测试、前端构建、打包和产物清单/哈希校验，再发包。tag 指向该源码加版本元数据的快照，不能把旧产物挂到较新的 main 提交上。
4. Release 说明使用 `Source commit: <完整 SHA>`，清单的 `releaseSource` 保存相同 SHA；主线同步版本、实际下载地址、工具链生成的 `entryHash` / `zipHash`，并保持包清单、根清单和锁文件一致。`zipHash` 不是 ZIP 文件本身的 SHA-256。
5. 同一仓库的发布任务串行，检查失败不推进队列；重跑从未完成的源码续发。若 Release 成功但主线元数据推送失败，按 `Source commit` 接回已有 Release，不重复发布同一源码。保留手动触发入口用于恢复。

从三段日期版本迁移时，同一天的 `2026.11.6` 与 `2026.11.6.0` 数值相等，首个四段版本应从 `.1` 开始；若旧清单使用未来日期，真实日期追上之前沿用旧日期前缀，避免被判断为降级。首次使用新格式前应核对目标宿主及客户端的安装、源列表和 GitHub 发现升级比较；当前代码支持四段版本不代表历史客户端也支持。

在 `plugin.json` 配置稳定的 `updateUrl`，`download_url` 指向实际发布的 `{entryPath}.jsplugin.zip`。新增宿主能力时，`minHostVersion` 填写真正包含该能力的宿主版本，并在运行时检测能力；最低版本字段不能代替能力检查。

可参考 [Navidrome 发布脚本](https://github.com/songloft-org/songloft-plugin-navidrome/blob/main/scripts/release-plugin.mjs)、[工作流](https://github.com/songloft-org/songloft-plugin-navidrome/blob/main/.github/workflows/release.yml)和[发布回归](https://github.com/songloft-org/songloft-plugin-navidrome/blob/main/tests/release_pipeline.test.mjs)。移植时替换仓库名、入口与构建检查，不要携带 MIoT 等插件特有的前端或生成数据步骤。发布脚本测试应使用本地临时 Git 远端及模拟 Release，覆盖首次发布、换日、序号、文档跳过、批量提交、失败续跑和 tag/源码对应关系，不能向真实仓库发测试包。

### 加入 GitHub 社区插件发现

**发布插件后，请给 GitHub 仓库添加 `songloft-plugin` topic。** 在仓库首页的 **About → 设置齿轮 → Topics** 中填写并保存；这是仓库的 topic，不是 Release tag。有此 topic 的仓库可被 Flutter 和 Lynx 客户端的「插件商店 → 选源菜单 → GitHub 发现」自动发现，无需等待人工收录。

仅添加 topic 不保证展示，发布时还需确认：

- 仓库公开、未归档且不是 fork，默认分支根目录存在符合规范的 `plugin.json`。
- 发布稳定的 GitHub Release，上传实际存在、非空的 `{entryPath}.jsplugin.zip`，清单的 `download_url` 指向该资产，允许使用其他 GitHub 仓库的发布包；客户端会验证目标仓库的实际 Release 和资产。如通过 `updateUrl` 获取下载地址，也允许其他 GitHub 仓库的公开更新清单，但必须保持版本和插件入口一致。
- 根清单的 `entryHash`、`zipHash` 可以省略或留空，不影响 GitHub 发现；填写时必须符合 64 位小写十六进制格式。实际安装包中的哈希仍由服务器校验。建议同步根清单的 `main` 和版本，保持与发布包一致。`zipHash` 是工具链生成的 canonical 内容哈希，不是 ZIP 文件本身的 SHA-256。

Release tag 不必与清单版本相同，例如 `v0.17` 可配合 `version: "0.17.0"`；下载 URL 必须指向实际存在的 Release 资产。设置 topic 后可主动刷新发现页检查结果，网络失败会显示为「暂未验证」。社区插件会标注「未经 Songloft 审核」，安装前仍需用户确认。

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
2. **合理使用定时器** — 回调与 HTTP JS 串行访问 VM，避免长时间同步计算；`fetch` 的 HTTP 工作在 Go goroutine 中异步执行，Promise 恢复仍需事件循环推进
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

- 新插件推荐使用[北京时间日期与每日序号](#推荐版本与自动发布规则)，已有 SemVer 插件可保留原体系。
- 在 `plugin.json` 中设置 `updateUrl` 支持远程更新检查，发布说明写清行为变化、兼容性及迁移步骤。
- 版本分配、CI 队列、失败续跑与源码追溯遵循上文推荐规则。

### 开发调试

1. 查看服务器日志中 `[plugin]` 前缀的输出
2. 使用 `songloft.log.info/warn/error` 输出调试信息
3. 健康检查失败会在日志中记录

### 存储使用模式

```javascript
// 存储复杂对象（storage 自动 JSON 序列化，直接存对象即可）
async function saveConfig(config) {
    await songloft.storage.set("config", config);
}

async function loadConfig() {
    var config = await songloft.storage.get("config");
    return config || { defaultKey: "defaultValue" };
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
async function useTranslation(text) {
    var resp = await songloft.comm.call("translator-plugin", "get-service", {
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

## 13. 宿主实现约束（维护者）

以下约束面向修改宿主 `internal/jsplugin` 的维护者，插件作者无需了解。

### 新增权限的同步清单

新增权限常量必须同步三处，否则插件根本无法使用：

1. 宿主 `internal/jsplugin/permissions.go` 的 `AllPermissions`
2. `plugins/toolchain/packages/plugin-builder/src/manifest.ts` 的 `VALID_PERMISSIONS`——它在 build 时硬失败（`unknown permission: xxx`），漏了这处插件连产物都出不来
3. `plugins/toolchain/packages/create-songloft-plugin/src/index.ts` 的 `AVAILABLE_PERMISSIONS`（脚手架勾选列表）

`ValidatePermissions` 对未知权限**只 warn 不拒绝**：它挂在安装/更新的必经路径上，拒绝会让声明新权限的插件在旧宿主上直接装不上。`minHostVersion` 是纯信息字段，宿主从不强制；新权限仍需等宿主版本铺开后再发插件。

### `net:insecure-tls` 的运行时投影

`jsruntime` 层没有权限概念，`fetch` 本身也不需要 `net`。该权限由 `service.go` 在建环境后调用 `jsManager.SetAllowInsecureTLS(envID, ...)` 投影成 `JSEnv.allowInsecureTLS`（默认 false），是权限在运行时层的唯一体现。

- `CheckPermission` **没有冒号前缀匹配，`net` 不覆盖 `net:insecure-tls`**：否则所有已声明 `net` 的存量插件升级后会静默获得跳过校验的能力
- 使用独立的 `insecureTransport`，不能复用 `sharedTransport` 后改单次请求的 `TLSClientConfig`，否则会污染所有插件的连接复用
- 子环境（`songloft.jsenv` 创建的）必须继承父插件策略
- 无权限却带该头时打 `slog.Warn`，不要静默丢弃

### 后台 pump 必须事件驱动

脱离 `ExecuteJS` 的异步链（`onInit` 起的后台循环、`setTimeout(fn, 0)` 甩到后台的任务）全靠 `JSService.runTimerProcessor` 推进：由 `asyncSignal` 即时唤醒、按 `GetNextTimerDeadline` 休眠，500ms 只是兜底。改回固定 ticker 会让每个 await 续作多等半拍，十几跳的链路平白累积数秒。`executeInEnv` 返回时必须 `signalAsync` 一次，否则本次调用新注册的定时器对正在休眠的 pump 协程不可见。排查插件延迟时先对齐宿主侧 `doHTTPRequest` 时间戳与 JS 侧观测时刻，差值落在固定网格上就是 pump 周期。

### 公共资源版本化

`jsplugin-assets/*` 使用 `Cache-Control: immutable` 长缓存，注入 URL 必须带内容哈希（`?v=<hash>`）；新增公共资源必须登记进 `internal/jsplugin/routes.go` 的 `assetVersions`，否则任何后续修改对老用户都不可达。详见[前端踩坑 · 缓存层](frontend_gotchas.md)。

---

## 附录：完整示例

参见 [plugin-toolchain/examples/basic](https://github.com/songloft-org/plugin-toolchain/tree/main/examples/basic) 目录，包含基于官方工具链的完整示例插件代码。
