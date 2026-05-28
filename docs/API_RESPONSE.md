# API 响应格式规范

本项目采用 **RESTful 直返风格**，不使用 `{code, data, message}` 统一信封。

---

## 成功响应

### 单个实体 — 直接返回模型

```json
// GET /api/v1/songs/1
{"id":1, "title":"Sample Track", "artist":"Sample Artist", ...}
```

### 分页列表 — 集合名 + 分页元数据

```json
// GET /api/v1/songs?limit=20&offset=0
{"songs":[...], "total":100, "limit":20, "offset":0}
```

### 操作结果 — `{"message": "..."}`

```json
// DELETE /api/v1/songs/1
{"message": "歌曲已删除"}
```

用 `models.SuccessResponse` 或 `map[string]string{"message": "..."}` 均可。

---

## 错误响应

统一通过 `respondError` 返回，格式固定：

```json
{"error": "人类可读的错误信息", "detail": "可选的技术细节"}
```

- `error`：必有，面向用户的简短描述
- `detail`：可选，仅当传入 `err != nil` 时输出，包含底层错误信息

对应结构体：`models.ErrorResponse`。

**中间件层**同样必须返回 JSON 格式错误（使用局部 `respondAuthError`），**禁止** `http.Error()` 返回纯文本。

### 例外：二进制流端点

播放（`/songs/{id}/play`）、代理（`/proxy`）、静态文件等二进制流端点，错误可使用 `http.Error()` / `http.NotFound()`，因为客户端不期望 JSON body。

---

## 禁止事项

| 禁止 | 原因 |
|------|------|
| `{code, data, message}` 信封 | `code` 与 HTTP 状态码语义重复，增加客户端解析负担 |
| 自定义错误字段名 | 必须使用 `error` + `detail`，不得用 `message`、`msg`、`reason` 等替代 |
| API 端点返回纯文本错误 | 前端 `response.json()` 解析会抛异常（二进制流端点除外） |

---

## 实现要点

| 场景 | 使用方式 |
|------|----------|
| 返回数据 | `respondJSON(w, status, data)` — `data` 直接序列化为顶层 JSON |
| 返回错误 | `respondError(w, status, message, err)` — 自动构建 `{error, detail}` |
| 中间件错误 | `respondAuthError(w, status, message, err)` — 与 `respondError` 格式一致 |
| jsplugin 错误 | `writePluginUnavailable` — 同样使用 `{error, detail}` 字段 |
