package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"songloft/internal/database"
	"songloft/internal/models"
	"songloft/internal/services"

	"github.com/go-chi/chi/v5"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService *services.AuthService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取访问令牌
// @Tags 认证管理
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "登录请求"
// @Success 200 {object} models.LoginResponse "登录成功"
// @Failure 400 {object} models.ErrorResponse "请求数据错误"
// @Failure 401 {object} models.ErrorResponse "用户名或密码错误"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 获取客户端信息
	clientInfo := r.UserAgent()
	if clientInfo == "" {
		clientInfo = r.RemoteAddr
	}

	// 执行登录
	resp, err := h.authService.Login(ctx, req.Username, req.Password, clientInfo)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "用户名或密码错误", err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// Logout 用户登出
// @Summary 用户登出
// @Description 用户登出，撤销当前访问令牌
// @Tags 认证管理
// @Accept json
// @Produce json
// @Success 200 {object} models.SuccessResponse "登出成功"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 从请求上下文中获取当前用户信息
	// 这里假设中间件已经设置了用户信息
	clientID := r.Header.Get("X-Client-ID") // 这将在中间件中设置

	// 获取当前访问令牌
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		accessToken := authHeader[7:]

		// 执行登出
		if err := h.authService.Logout(ctx, accessToken, clientID); err != nil {
			respondError(w, http.StatusInternalServerError, "登出失败", err)
			return
		}
	}

	respondJSON(w, http.StatusOK, models.SuccessResponse{
		Message: "登出成功",
	})
}

// RefreshToken 刷新令牌
// @Summary 刷新令牌
// @Description 使用刷新令牌获取新的访问令牌
// @Tags 认证管理
// @Accept json
// @Produce json
// @Param request body models.RefreshTokenRequest true "刷新令牌请求"
// @Success 200 {object} services.RefreshResponse "刷新成功"
// @Failure 400 {object} models.ErrorResponse "请求数据错误"
// @Failure 401 {object} models.ErrorResponse "刷新令牌无效"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 获取客户端信息
	clientInfo := r.UserAgent()
	if clientInfo == "" {
		clientInfo = r.RemoteAddr
	}

	// 执行刷新令牌
	resp, err := h.authService.RefreshToken(ctx, req.RefreshToken, clientInfo)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "刷新令牌无效", err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// ListTokens 列出活跃令牌
// @Summary 列出活跃令牌
// @Description 获取当前用户的所有活跃令牌列表
// @Tags 认证管理
// @Accept json
// @Produce json
// @Param type query string false "令牌类型" Enums(access, refresh)
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "令牌列表"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /auth/tokens [get]
func (h *AuthHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析查询参数
	tokenType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := models.DefaultPaginationLimit
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	// 构建过滤条件
	filter := &database.TokenFilter{
		TokenType: tokenType,
		Limit:     limit,
		Offset:    offset,
		OrderBy:   "created_at",
		Order:     "DESC",
	}

	// 获取令牌列表
	tokens, err := h.authService.ListActiveTokens(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取令牌列表失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tokens": tokens,
		"total":  len(tokens),
		"limit":  limit,
		"offset": offset,
	})
}

// RevokeToken 撤销令牌
// @Summary 撤销令牌
// @Description 撤销指定的令牌
// @Tags 认证管理
// @Accept json
// @Produce json
// @Param token_id path string true "令牌ID"
// @Param request body models.RevokeTokenRequest true "撤销令牌请求"
// @Success 200 {object} models.SuccessResponse "撤销成功"
// @Failure 400 {object} models.ErrorResponse "请求数据错误"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /auth/tokens/{token_id} [delete]
func (h *AuthHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tokenID := chi.URLParam(r, "token_id")

	var req models.RevokeTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 获取客户端信息作为撤销者
	revokedBy := r.Header.Get("X-Client-ID")
	if revokedBy == "" {
		revokedBy = "unknown"
	}

	// 执行撤销令牌
	if err := h.authService.RevokeToken(ctx, tokenID, revokedBy, req.Reason); err != nil {
		respondError(w, http.StatusInternalServerError, "撤销令牌失败", err)
		return
	}

	respondJSON(w, http.StatusOK, models.SuccessResponse{
		Message: "令牌已撤销",
	})
}

// GetTokenInfo 获取令牌信息
// @Summary 获取令牌信息
// @Description 获取指定令牌的详细信息
// @Tags 认证管理
// @Accept json
// @Produce json
// @Param token_id path string true "令牌ID"
// @Success 200 {object} models.TokenInfo "令牌信息"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "令牌不存在"
// @Security BearerAuth
// @Router /auth/tokens/{token_id} [get]
func (h *AuthHandler) GetTokenInfo(w http.ResponseWriter, r *http.Request) {
	// 这个接口将在后续实现中添加
	respondError(w, http.StatusNotImplemented, "功能未实现", nil)
}
