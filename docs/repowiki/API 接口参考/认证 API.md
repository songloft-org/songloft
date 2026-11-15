# 认证 API

本页覆盖登录、刷新、登出、会话令牌管理和永久访问 Token 管理。默认凭证来自启动配置，不由 dev / release 构建标签决定；已运行实例以启动参数或环境变量为准。

本文档基于以下源文件编写：

- [internal/handlers/auth.go](https://github.com/songloft-org/songloft/blob/main/internal/handlers/auth.go) -- 登录、登出、令牌刷新、令牌管理处理器
- [internal/app/routers.go](https://github.com/songloft-org/songloft/blob/main/internal/app/routers.go) -- 认证路由注册（公开端点 + 需授权端点）
- [internal/models/models.go](https://github.com/songloft-org/songloft/blob/main/internal/models/models.go) -- LoginRequest / LoginResponse / RefreshTokenRequest / RevokeTokenRequest / TokenInfo 结构体
- [internal/middleware/auth.go](https://github.com/songloft-org/songloft/blob/main/internal/middleware/auth.go) -- JWT 认证中间件（Bearer + query param）

## 目录

1. [认证机制概述](#1-认证机制概述)
2. [公开端点](#2-公开端点)
3. [需授权端点](#3-需授权端点)
4. [永久访问 Token](#4-永久访问-token)

---

## 1. 认证机制概述

**章节来源**: `internal/handlers/auth.go`、`internal/middleware/auth.go`

Songloft 使用 JWT 双令牌认证机制：

- **Access Token**: 短期令牌，用于 API 鉴权。客户端通过 `Authorization: Bearer <token>` 请求头或 `?access_token=<token>` 查询参数传递
- **Refresh Token**: 长期令牌，用于在 Access Token 过期后获取新令牌，无需重新登录
- **默认账号**: 未配置凭证时为 `admin` / `admin`，与构建标签无关
- **令牌类型**: `access`（访问令牌）和 `refresh`（刷新令牌）
- **公开端点**: 登录和刷新在认证中间件之外；`/version` 和 `/health` 同样公开
- **受保护业务端点**: 可用有效登录凭证或永久凭证，通过 Bearer 或 `access_token` 传入；插件声明的公开路径按插件规则处理。Refresh Token 用于 `/auth/refresh` 续期，不是正常登录管理凭证

---

## 2. 公开端点

**章节来源**: `internal/handlers/auth.go`、`internal/app/routers.go`

以下端点注册在认证中间件之外，无需携带令牌即可访问。

### POST /api/v1/auth/login

用户登录，获取访问令牌和刷新令牌。handler 自动提取请求的 User-Agent 作为客户端信息存储。

- **认证**: 无需认证
- **请求体** (`application/json`):

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 是 | 用户名 |
| `password` | string | 是 | 密码 |

- **200**:

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 604800,
  "token_type": "Bearer"
}
```

| 字段 | 说明 |
|------|------|
| `access_token` | 访问令牌，用于后续 API 请求鉴权 |
| `refresh_token` | 刷新令牌，用于 Access Token 过期后续期 |
| `expires_in` | Access Token 有效期（秒） |
| `token_type` | 固定值 `Bearer` |

- **400**: JSON 解析失败
- **401**: 用户名或密码错误
- **500**: 服务器错误

### POST /api/v1/auth/refresh

使用刷新令牌获取新的访问令牌，无需重新登录。同样提取 User-Agent 作为客户端信息。

- **认证**: 无需认证
- **请求体** (`application/json`):

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `refresh_token` | string | 是 | 之前登录时获得的刷新令牌 |

- **200**: 返回新的令牌对（`RefreshResponse`，结构同登录响应）
- **400**: 请求数据错误
- **401**: 刷新令牌无效或已过期
- **500**: 服务器错误

---

## 3. 需授权端点

**章节来源**: `internal/handlers/auth.go`、`internal/app/routers.go`

以下登出和 Token 管理端点还使用 `RequireLoginSession`，仅限正常登录的 Access Token。插件、永久及刷新凭证不能管理 Token 或登出（已识别的非登录身份返回 403，无效 / 已撤销凭证返回 401）。

### POST /api/v1/auth/logout

撤销当前访问令牌，使其失效。

- **认证**: Bearer Token
- **实现细节**: 从 `Authorization: Bearer <token>` 头提取当前 Access Token 并调用 `authService.Logout` 撤销。即使提取失败也返回成功（幂等设计）。
- **200**: `{"message": "登出成功"}`
- **401**: 未授权
- **500**: 登出失败

### GET /api/v1/auth/tokens

获取当前用户的所有活跃令牌列表，用于令牌管理界面。按创建时间倒序排列。

- **认证**: Bearer Token
- **查询参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 否 | 令牌类型过滤：`access` / `refresh` |
| `limit` | int | 否 | 每页数量，默认 20 |
| `offset` | int | 否 | 偏移量，默认 0 |

- **200**:

```json
{
  "tokens": [
    {
      "token_id": "abc123",
      "token_type": "access",
      "client_info": "Mozilla/5.0 AppleWebKit/605.1.15",
      "expires_at": "2024-01-08T12:00:00Z",
      "created_at": "2024-01-01T12:00:00Z"
    }
  ],
  "total": 3,
  "limit": 20,
  "offset": 0
}
```

- **401**: 未授权
- **500**: 获取失败

### DELETE /api/v1/auth/tokens/{token_id}

撤销指定的令牌，使其立即失效。可用于远程踢出其他设备的登录会话。

- **认证**: Bearer Token
- **路径参数**: `token_id`（string，要撤销的令牌 ID）
- **请求体** (`application/json`):

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `reason` | string | 否 | 撤销原因（如"用户主动登出"） |

- **200**: `{"message": "令牌已撤销"}`
- **400**: 请求数据错误
- **401**: 未授权
- **500**: 撤销失败

## 4. 永久访问 Token

源码：`internal/handlers/personal_token.go`、`internal/models/personal_token.go`、`internal/services/auth_service.go`、`internal/middleware/auth.go`。

永久凭证用于设备集成，具有管理员业务 API 权限，有效期至主动撤销，退出登录及重启不使其失效；不能管理 Token 或登出。数据库只保存哈希，完整凭证仅创建时返回一次。后续请求读取持久状态，因此撤销影响下一次认证，不中断已开始的请求。

三个管理端点均要求正常登录会话：

| 方法与路径 | 请求 | 成功响应 |
| --- | --- | --- |
| `POST /api/v1/auth/personal-tokens` | `{"name":"Home Assistant"}`，名称去空白后 1–80 个字符，不含控制字符 | 201，元数据及仅此次返回的 `token` |
| `GET /api/v1/auth/personal-tokens` | 可选 `limit`（默认 20）、`offset`（默认 0） | 200，`tokens`、`total`、`limit`、`offset`；含已撤销记录，不返回凭证或哈希 |
| `DELETE /api/v1/auth/personal-tokens/{id}` | 管理 ID，无需请求体 | 204；重复撤销成功，不存在返回 404 |

元数据含 `id`、`name`、`prefix`、`created_at`、`last_used_at`、`revoked_at`。`last_used_at` 最多每分钟更新一次，未使用时为 null。创建与列表返回 `Cache-Control: no-store`。未满足登录鉴权返回 401 / 403，创建请求非法返回 400。
