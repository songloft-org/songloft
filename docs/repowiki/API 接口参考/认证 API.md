# 认证 API

本文档基于以下源文件编写：

- `internal/handlers/auth.go` -- 登录、登出、令牌刷新、令牌管理处理器
- `internal/app/routers.go` -- 认证路由注册（公开端点 + 需授权端点）
- `internal/models/models.go` -- LoginRequest / LoginResponse / RefreshTokenRequest / RevokeTokenRequest / TokenInfo 结构体
- `internal/middleware/auth.go` -- JWT 认证中间件（Bearer + query param）

## 目录

1. [认证机制概述](#1-认证机制概述)
2. [公开端点](#2-公开端点)
3. [需授权端点](#3-需授权端点)

---

## 1. 认证机制概述

**章节来源**: `internal/handlers/auth.go`、`internal/middleware/auth.go`

Songloft 使用 JWT 双令牌认证机制：

- **Access Token**: 短期令牌，用于 API 鉴权。客户端通过 `Authorization: Bearer <token>` 请求头或 `?access_token=<token>` 查询参数传递
- **Refresh Token**: 长期令牌，用于在 Access Token 过期后获取新令牌，无需重新登录
- **默认账号**: `admin` / `admin`（开发模式）
- **令牌类型**: `access`（访问令牌）和 `refresh`（刷新令牌）
- **公开端点**: 登录和刷新不需要认证，注册在认证中间件之外
- **鉴权端点**: 其他所有 `/api/v1/*` 端点均需要有效的 Access Token

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

以下端点注册在认证中间件保护的路由组内，必须携带有效的 Access Token。

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

### GET /api/v1/auth/tokens/{token_id}

获取指定令牌的详细信息。

- **认证**: Bearer Token
- **路径参数**: `token_id`（string，令牌 ID）
- **200**: 返回 `TokenInfo` 对象：

```json
{
  "token_id": "abc123",
  "token_type": "access",
  "client_info": "Mozilla/5.0...",
  "expires_at": "2024-01-08T12:00:00Z",
  "created_at": "2024-01-01T12:00:00Z",
  "revoked_at": null,
  "revoked_by": "",
  "revoked_reason": ""
}
```

- **401**: 未授权
- **404**: 令牌不存在

> **注意**: 当前实现返回 `501 Not Implemented`，功能待后续完善。

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
