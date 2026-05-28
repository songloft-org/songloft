# JS Runtime 包说明

## 概述

`jsruntime` 包提供了基于 QuickJS 的 JavaScript 运行时环境，允许在 Go 应用中执行 JavaScript 代码。

## 目录结构

```
internal/jsruntime/
├── runtime.go      # 核心运行时管理器和主要功能
├── pendingjob.go   # 底层 JS_ExecutePendingJob 调用（处理 Promise 微任务）
└── polyfill.go     # JavaScript polyfill 代码
```

## 主要类型

### JSEnvManager

JS 运行时环境管理器，负责创建、管理和销毁多个 JS 运行环境。

```go
mgr := jsruntime.NewJSEnvManager()
defer mgr.Close()
```

### JSEnv

单个 JS 运行时环境，包含一个独立的 QuickJS VM 实例。

```go
type JSEnv struct {
    vm       *quickjs.VM
    envID    string
    pluginID int64
    created  time.Time
    mu       sync.Mutex
    events   chan JSEventResult
}
```

### ExecuteResult

JS 执行结果封装。

```go
type ExecuteResult struct {
    Result string         // 执行结果字符串
    Events []JSEventResult // 执行期间产生的事件
}
```

### JSEventResult

JS 事件结果封装。

```go
type JSEventResult struct {
    EnvID string // 环境 ID
    Name  string // 事件名称
    Data  string // 事件数据
}
```

## 主要方法

### 创建环境

```go
err := mgr.CreateEnv(envID, initCode, pluginID)
```

- `envID`: 环境唯一标识符
- `initCode`: 初始化时执行的 JS 代码（可选）
- `pluginID`: 创建此环境的插件 ID

### 执行 JS 代码

```go
result, err := mgr.ExecuteJS(envID, code, timeoutMs)
```

- `envID`: 目标环境 ID
- `code`: 要执行的 JS 代码
- `timeoutMs`: 超时时间（毫秒），0 表示使用默认超时（30 秒）

### 执行 JS 并等待事件

```go
result, err := mgr.ExecuteJSAndWaitEvents(envID, code, timeoutMs, waitEventNames)
```

- `waitEventNames`: 要等待的事件名称列表

### 销毁环境

```go
// 销毁单个环境
err := mgr.DestroyEnv(envID)

// 销毁插件创建的所有环境
err := mgr.DestroyPluginEnvs(pluginID)
```

## 内置 Polyfill

JS 运行时提供了丰富的 polyfill，使 JS 代码能够在 QuickJS 环境中运行：

### Console

```javascript
console.log('message')
console.error('error')
console.warn('warning')
console.info('info')
console.debug('debug')
console.trace('trace')
```

### Fetch (同步 HTTP)

```javascript
fetch(url, {
    method: 'GET',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(data)
}).then(response => response.json())
```

### Timer

```javascript
setTimeout(() => {
    console.log('timer fired')
}, 1000)

clearTimeout(timerId)
```

### Buffer

```javascript
const buf = Buffer.from('hello', 'utf8')
console.log(buf.toString('base64'))
```

### Crypto

```javascript
// MD5
const hash = crypto.md5('data')

// AES 加密
const encrypted = crypto.aesEncrypt(buffer, 'cbc', key, iv)

// RSA 加密
const encrypted = crypto.rsaEncrypt(buffer, publicKeyPEM)

// 随机字节
const randomBytes = crypto.randomBytes(32)
```

### Zlib

```javascript
const compressed = zlib.deflate(buffer)
const decompressed = zlib.inflate(compressed)
```

### URL / URLSearchParams

```javascript
const url = new URL('https://example.com/path?query=value')
console.log(url.hostname)
console.log(url.searchParams.get('query'))

const params = new URLSearchParams('key=value&foo=bar')
console.log(params.toString())
```

### TextEncoder / TextDecoder

```javascript
const encoder = new TextEncoder()
const uint8array = encoder.encode('hello')

const decoder = new TextDecoder('utf-8')
const str = decoder.decode(uint8array)
```

## Go 桥接函数

JS 运行时通过以下 Go 桥接函数提供系统级功能：

- `__go_send(name, data)`: 发送事件到 Go
- `__go_console(level, msg)`: 控制台日志输出
- `__go_fetch_async(url, method, headers, body) -> id`: 真异步 HTTP 请求；
  返回 id，结果通过 asyncResults 通道回投，由事件循环 resolve 对应 Promise
  （插件代码统一通过 `globalThis.fetch()` 调用，封装好 Promise 包装）
- `__go_bridge(action, data) -> id`: 真异步桥接调用（storage/songs/playlists/comm/jsenv）
- `__go_pop_async_result() -> json|""`: 主事件循环排空异步结果队列的非阻塞接口
- `__go_now_ms()`: 当前时间戳（毫秒）
- `__go_buffer_from(data, encoding)`: 创建 Buffer
- `__go_buffer_to_string(hex, encoding)`: Buffer 转字符串
- `__go_crypto_md5(str)`: MD5 哈希
- `__go_crypto_random_bytes(size)`: 生成随机字节
- `__go_crypto_aes_encrypt(data, mode, key, iv)`: AES 加密
- `__go_crypto_rsa_encrypt(data, keyPEM)`: RSA 加密
- `__go_zlib_inflate(data)`: zlib 解压
- `__go_zlib_deflate(data)`: zlib 压缩

## 使用示例

### 基本使用

```go
package main

import (
    "log"
    "mimusic/internal/jsruntime"
)

func main() {
    // 创建管理器
    mgr := jsruntime.NewJSEnvManager()
    defer mgr.Close()

    // 创建环境
    err := mgr.CreateEnv("test-env", "", 0)
    if err != nil {
        log.Fatal(err)
    }

    // 执行 JS 代码
    result, err := mgr.ExecuteJS("test-env", `
        console.log('Hello from JS!')
        return 42
    `, 0)
    
    if err != nil {
        log.Printf("执行错误：%v", err)
    }
    
    log.Printf("结果：%s", result.Result)
    log.Printf("事件数：%d", len(result.Events))

    // 销毁环境
    mgr.DestroyEnv("test-env")
}
```

### 等待事件

```go
result, err := mgr.ExecuteJSAndWaitEvents("env-id", `
    setTimeout(() => {
        __go_send('ready', 'data')
    }, 1000)
`, 5000, []string{"ready"})

if err != nil {
    log.Printf("执行错误：%v", err)
}

for _, evt := range result.Events {
    log.Printf("事件：%s - %s", evt.Name, evt.Data)
}
```

### 异步操作

```go
result, err := mgr.ExecuteJS("env-id", `
    fetch('https://api.example.com/data')
        .then(res => res.json())
        .then(data => {
            console.log('获取数据:', data)
            __go_send('data-received', JSON.stringify(data))
        })
`, 10000)

// 处理返回的事件
for _, evt := range result.Events {
    if evt.Name == 'data-received' {
        var data map[string]interface{}
        json.Unmarshal([]byte(evt.Data), &data)
        // 处理数据...
    }
}
```

## 注意事项

1. **线程安全**: `JSEnvManager` 是线程安全的，但每个 `JSEnv` 内部的 VM 不是线程安全的。管理器会自动串行化对同一环境的访问。

2. **资源管理**: 
   - 使用 `Close()` 关闭管理器会销毁所有环境
   - 及时调用 `DestroyEnv()` 或 `DestroyPluginEnvs()` 释放不需要的环境
   - 避免内存泄漏

3. **超时控制**: 
   - 默认超时为 30 秒
   - 可以通过 `timeoutMs` 参数自定义超时时间
   - 长时间运行的 JS 代码应该设置合适的超时

4. **事件处理**: 
   - 事件通道缓冲大小为 64
   - 如果通道已满，新事件会被丢弃
   - 及时收集和处理事件

5. **错误处理**: 
   - JS 执行错误会返回 error
   - 可以通过 `result.Events` 获取执行期间的事件
   - 使用 `ExecuteJSAndWaitEvents` 可以等待特定事件

## 与 jsplugin 包的集成

`jsruntime` 提供底层 VM 能力，`internal/jsplugin` 在其上封装插件生命周期、权限、热更新等业务逻辑：

```go
import "mimusic/internal/jsruntime"

type Manager struct {
    jsRuntime *jsruntime.JSEnvManager
    // ...
}

func NewManager(...) *Manager {
    m := &Manager{
        jsRuntime: jsruntime.NewJSEnvManager(),
        // ...
    }
    return m
}
```

这样可以将 JS 运行时逻辑与插件管理逻辑分离，提高代码的可维护性和可测试性。
