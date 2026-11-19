// Package contextkeys 定义跨请求中间件与业务服务共用的 context key。
package contextkeys

// Key 是请求上下文 key 的专用类型，避免与其他包的字符串 key 碰撞。
type Key string

// ClientIDKey 保存认证令牌对应的客户端会话 ID。
const ClientIDKey Key = "client_id"
