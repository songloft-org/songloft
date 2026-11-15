# Authentication API

This page covers login, refresh, logout, session-token management and permanent access-token management. Startup configuration determines default credentials independently of dev / release build tags; running instances use their startup flags or environment variables.

This document is based on the following source files:

- `internal/handlers/auth.go` -- Login, logout, token refresh, and token management handlers
- `internal/app/routers.go` -- Authentication route registration (public endpoints + authorized endpoints)
- `internal/models/models.go` -- LoginRequest / LoginResponse / RefreshTokenRequest / RevokeTokenRequest / TokenInfo structs
- `internal/middleware/auth.go` -- JWT authentication middleware (Bearer + query param)

## Table of Contents

1. [Authentication Mechanism Overview](#1-authentication-mechanism-overview)
2. [Public Endpoints](#2-public-endpoints)
3. [Authorized Endpoints](#3-authorized-endpoints)
4. [Permanent Access Tokens](#4-permanent-access-tokens)

---

## 1. Authentication Mechanism Overview

**Section source**: `internal/handlers/auth.go`, `internal/middleware/auth.go`

Songloft uses a JWT dual-token authentication mechanism:

- **Access Token**: A short-lived token used for API authorization. The client passes it via the `Authorization: Bearer <token>` request header or the `?access_token=<token>` query parameter
- **Refresh Token**: A long-lived token used to obtain a new token after the Access Token expires, without re-logging in
- **Default account**: `admin` / `admin` when credentials are not configured, independently of build tags
- **Token types**: `access` (access token) and `refresh` (refresh token)
- **Public endpoints**: Login and refresh are outside authentication middleware; `/version` and `/health` are also public
- **Protected business endpoints**: Accept valid login or permanent credentials via Bearer or `access_token`; declared public plugin paths have their own rules. Refresh tokens renew sessions through `/auth/refresh`; they are not normal login management credentials

---

## 2. Public Endpoints

**Section source**: `internal/handlers/auth.go`, `internal/app/routers.go`

The following endpoints are registered outside the authentication middleware and can be accessed without carrying a token.

### POST /api/v1/auth/login

User login, obtains an access token and a refresh token. The handler automatically extracts the request's User-Agent and stores it as client information.

- **Authentication**: None required
- **Request body** (`application/json`):

| Field | Type | Required | Description |
|------|------|------|------|
| `username` | string | Yes | Username |
| `password` | string | Yes | Password |

- **200**:

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 604800,
  "token_type": "Bearer"
}
```

| Field | Description |
|------|------|
| `access_token` | Access token, used for authorization of subsequent API requests |
| `refresh_token` | Refresh token, used to renew the Access Token after it expires |
| `expires_in` | Access Token validity period (seconds) |
| `token_type` | Fixed value `Bearer` |

- **400**: JSON parsing failed
- **401**: Incorrect username or password
- **500**: Server error

### POST /api/v1/auth/refresh

Uses a refresh token to obtain a new access token without re-logging in. Also extracts the User-Agent as client information.

- **Authentication**: None required
- **Request body** (`application/json`):

| Field | Type | Required | Description |
|------|------|------|------|
| `refresh_token` | string | Yes | The refresh token obtained during a previous login |

- **200**: Returns a new token pair (`RefreshResponse`, same structure as the login response)
- **400**: Bad request data
- **401**: Refresh token invalid or expired
- **500**: Server error

---

## 3. Authorized Endpoints

**Section source**: `internal/handlers/auth.go`, `internal/app/routers.go`

All logout and token-management endpoints below also use `RequireLoginSession`: only a normal login Access Token may call them. Plugin, permanent and refresh credentials cannot manage tokens or log out (403 for recognized non-login credentials; malformed/revoked credentials return 401).

### POST /api/v1/auth/logout

Revokes the current access token, invalidating it.

- **Authentication**: Bearer Token
- **Implementation detail**: Extracts the current Access Token from the `Authorization: Bearer <token>` header and calls `authService.Logout` to revoke it. Returns success even if extraction fails (idempotent design).
- **200**: `{"message": "登出成功"}`
- **401**: Unauthorized
- **500**: Logout failed

### GET /api/v1/auth/tokens

Gets the list of all active tokens for the current user, used for the token management interface. Ordered by creation time in descending order.

- **Authentication**: Bearer Token
- **Query parameters**:

| Parameter | Type | Required | Description |
|------|------|------|------|
| `type` | string | No | Token type filter: `access` / `refresh` |
| `limit` | int | No | Items per page, default 20 |
| `offset` | int | No | Offset, default 0 |

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

- **401**: Unauthorized
- **500**: Retrieval failed

### DELETE /api/v1/auth/tokens/{token_id}

Revokes a specified token, invalidating it immediately. Can be used to remotely kick out login sessions on other devices.

- **Authentication**: Bearer Token
- **Path parameter**: `token_id` (string, ID of the token to revoke)
- **Request body** (`application/json`):

| Field | Type | Required | Description |
|------|------|------|------|
| `reason` | string | No | Revocation reason (e.g., "user-initiated logout") |

- **200**: `{"message": "令牌已撤销"}`
- **400**: Bad request data
- **401**: Unauthorized
- **500**: Revocation failed

## 4. Permanent Access Tokens

Sources: `internal/handlers/personal_token.go`, `internal/models/personal_token.go`, `internal/services/auth_service.go`, `internal/middleware/auth.go`.

Permanent tokens provide administrator business API access for integrations, are valid until revoked, and survive logout and restart. They cannot manage tokens or log out. The database stores a hash; the full credential is returned only once when created. Subsequent requests check persistent state, so revocation affects the next authentication without interrupting requests already in progress.

All three management endpoints require a normal login session:

| Method and path | Request | Success |
| --- | --- | --- |
| `POST /api/v1/auth/personal-tokens` | `{"name":"Home Assistant"}`, name 1–80 characters after trimming, no control characters | 201, metadata plus one-time `token` |
| `GET /api/v1/auth/personal-tokens` | Optional `limit` (default 20) and `offset` (default 0) | 200, `tokens`, `total`, `limit`, `offset`; includes revoked records, no credentials/hashes |
| `DELETE /api/v1/auth/personal-tokens/{id}` | Management ID, no body | 204; repeated revocation succeeds, missing ID returns 404 |

Metadata includes `id`, `name`, `prefix`, `created_at`, `last_used_at` and `revoked_at`. `last_used_at` updates at most once per minute and is null before first use. Management reads/creation send `Cache-Control: no-store`. Missing login authorization returns 401/403; invalid creation requests return 400.
