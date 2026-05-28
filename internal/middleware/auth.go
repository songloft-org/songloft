package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"songloft/internal/services"
)

func respondAuthError(w http.ResponseWriter, status int, message string, err error) {
	response := map[string]string{"error": message}
	if err != nil {
		response["detail"] = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// AuthMiddleware 认证中间件
func AuthMiddleware(authService *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenString string

			// 优先从 Authorization 头获取 token
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				extracted := strings.TrimPrefix(authHeader, "Bearer ")
				if extracted != authHeader {
					tokenString = extracted
				}
			}

			// 回退：从 URL query parameter 获取 token
			// 用于图片/音频等无法自定义 Header 的场景（如 <img> 标签、CachedNetworkImage）
			if tokenString == "" {
				tokenString = r.URL.Query().Get("access_token")
			}

			if tokenString == "" {
				respondAuthError(w, http.StatusUnauthorized, "缺少认证信息", nil)
				return
			}

			// 验证 JWT token
			claims, err := authService.ValidateToken(r.Context(), tokenString)
			if err != nil {
				respondAuthError(w, http.StatusUnauthorized, "无效的 token", err)
				return
			}

			// 将 claims 信息添加到请求上下文
			ctx := context.WithValue(r.Context(), "client_id", claims.ClientID)

			// 认证成功，继续处理请求
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
