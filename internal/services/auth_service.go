package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"songloft/internal/database"
	"songloft/internal/models"
)

// TokenRepository 认证令牌仓储接口（AuthService 依赖）。
type TokenRepository interface {
	Create(ctx context.Context, token *models.AuthToken) error
	GetByID(ctx context.Context, tokenID string) (*models.AuthToken, error)
	Revoke(ctx context.Context, tokenID, revokedBy, reason string) error
	ListActive(ctx context.Context, filter *database.TokenFilter) ([]*models.AuthToken, error)
	CleanExpired(ctx context.Context) (int64, error)
	IsRevoked(ctx context.Context, tokenID string) (bool, error)
}

// TokenCacheEntry Token 缓存条目
type TokenCacheEntry struct {
	Claims    *Claims
	ExpiresAt time.Time
	Revoked   bool
}

// AuthService 认证服务
type AuthService struct {
	tokens   TokenRepository
	secret   []byte
	username string
	password string
	// Token 内存缓存，key 为 token 字符串，value 为缓存条目
	tokenCache sync.Map // map[string]*TokenCacheEntry
	done       chan struct{}
	closeOnce  sync.Once
}

// Claims JWT声明结构
type Claims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

// RefreshResponse 刷新Token响应
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// NewAuthService 创建认证服务
func NewAuthService(configs ConfigRepository, tokens TokenRepository, username, password string) (*AuthService, error) {
	// 从数据库获取 JWT 密钥
	config, err := configs.Get(context.Background(), "jwt_secret")
	if err != nil {
		return nil, fmt.Errorf("failed to get jwt secret: %w", err)
	}

	// 解码密钥
	secret, err := hex.DecodeString(config.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode jwt secret: %w", err)
	}

	s := &AuthService{
		tokens:   tokens,
		secret:   secret,
		username: username,
		password: password,
		done:     make(chan struct{}),
	}

	// 启动缓存清理协程
	go s.startCacheCleanup()

	return s, nil
}

// GenerateSecret 生成新的 JWT 密钥
func GenerateSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getAdminUsername 获取管理员用户名
func (s *AuthService) getAdminUsername(ctx context.Context) (string, error) {
	return s.username, nil
}

// getAdminPassword 获取管理员密码
func (s *AuthService) getAdminPassword(ctx context.Context) (string, error) {
	return s.password, nil
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, username, password, clientInfo string) (*models.LoginResponse, error) {
	// 从数据库获取管理员账户信息
	adminUsername, err := s.getAdminUsername(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin username: %w", err)
	}

	adminPassword, err := s.getAdminPassword(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin password: %w", err)
	}

	if username != adminUsername || password != adminPassword {
		return nil, fmt.Errorf("invalid credentials")
	}

	// 生成客户端 ID
	clientID, err := generateClientID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate client id: %w", err)
	}

	// 生成 Access Token (7 天过期)
	accessToken, accessExp, err := s.generateToken(clientID, "access", 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// 生成 Refresh Token (30 天过期)
	refreshToken, refreshExp, err := s.generateToken(clientID, "refresh", 30*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 保存 Token 到数据库
	now := time.Now()
	accessRecord := &models.AuthToken{
		TokenID:    accessToken,
		TokenType:  "access",
		ClientInfo: clientInfo,
		ExpiresAt:  accessExp,
		CreatedAt:  now,
	}

	refreshRecord := &models.AuthToken{
		TokenID:    refreshToken,
		TokenType:  "refresh",
		ClientInfo: clientInfo,
		ExpiresAt:  refreshExp,
		CreatedAt:  now,
	}

	if err := s.tokens.Create(ctx, accessRecord); err != nil {
		return nil, fmt.Errorf("failed to save access token: %w", err)
	}

	if err := s.tokens.Create(ctx, refreshRecord); err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}

	// 清理过期 Token
	_, _ = s.tokens.CleanExpired(ctx)

	return &models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessExp.Sub(now).Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// getCachedToken 从缓存获取 Token 信息
func (s *AuthService) getCachedToken(tokenString string) (*TokenCacheEntry, bool) {
	if entry, ok := s.tokenCache.Load(tokenString); ok {
		cacheEntry := entry.(*TokenCacheEntry)
		// 检查缓存是否过期
		if time.Now().Before(cacheEntry.ExpiresAt) && !cacheEntry.Revoked {
			return cacheEntry, true
		}
		// 缓存已过期，删除它
		s.tokenCache.Delete(tokenString)
	}
	return nil, false
}

// setTokenCache 设置 Token 缓存
func (s *AuthService) setTokenCache(tokenString string, claims *Claims, expiresAt time.Time, revoked bool) {
	s.tokenCache.Store(tokenString, &TokenCacheEntry{
		Claims:    claims,
		ExpiresAt: expiresAt,
		Revoked:   revoked,
	})
}

// deleteTokenCache 删除 Token 缓存
func (s *AuthService) deleteTokenCache(tokenString string) {
	s.tokenCache.Delete(tokenString)
}

// Close 关闭 AuthService，停止缓存清理协程
func (s *AuthService) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

// startCacheCleanup 启动缓存清理协程，每分钟清理一次过期缓存
func (s *AuthService) startCacheCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.tokenCache.Range(func(key, value interface{}) bool {
				entry := value.(*TokenCacheEntry)
				// 如果缓存已过期或 token 已被撤销，删除它
				if now.After(entry.ExpiresAt) || entry.Revoked {
					s.tokenCache.Delete(key)
				}
				return true
			})
		case <-s.done:
			return
		}
	}
}

// Logout 用户登出
func (s *AuthService) Logout(ctx context.Context, accessToken, clientID string) error {
	// 撤销 Access Token
	if err := s.tokens.Revoke(ctx, accessToken, clientID, "logout"); err != nil {
		return fmt.Errorf("failed to revoke access token: %w", err)
	}
	// 清除缓存
	s.deleteTokenCache(accessToken)

	// 查找并撤销对应的 Refresh Token
	filter := &database.TokenFilter{
		TokenType: "refresh",
	}

	tokens, err := s.tokens.ListActive(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list refresh tokens: %w", err)
	}

	for _, token := range tokens {
		// 检查是否属于同一客户端
		if token.ClientInfo == clientID {
			if err := s.tokens.Revoke(ctx, token.TokenID, clientID, "logout"); err != nil {
				return fmt.Errorf("failed to revoke refresh token: %w", err)
			}
			// 清除缓存
			s.deleteTokenCache(token.TokenID)
		}
	}

	return nil
}

// RefreshToken 刷新Token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken, clientInfo string) (*RefreshResponse, error) {
	// 验证Refresh Token
	isRevoked, err := s.tokens.IsRevoked(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to check token status: %w", err)
	}

	if isRevoked {
		return nil, fmt.Errorf("refresh token has been revoked")
	}

	// 获取Token详情
	token, err := s.tokens.GetByID(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// 检查Token类型
	if token.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type")
	}

	// 检查是否过期
	if token.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("refresh token has expired")
	}

	// 撤销旧的 Token 对
	if err := s.tokens.Revoke(ctx, refreshToken, "system", "token refreshed"); err != nil {
		return nil, fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	// 清除旧 Token 的缓存
	s.deleteTokenCache(refreshToken)

	// 生成新的 Access Token (7 天过期)
	newAccessToken, accessExp, err := s.generateToken(token.ClientInfo, "access", 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new access token: %w", err)
	}

	// 生成新的 Refresh Token (30 天过期)
	newRefreshToken, refreshExp, err := s.generateToken(token.ClientInfo, "refresh", 30*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new refresh token: %w", err)
	}

	// 保存新 Token 到数据库
	now := time.Now()
	accessRecord := &models.AuthToken{
		TokenID:    newAccessToken,
		TokenType:  "access",
		ClientInfo: clientInfo,
		ExpiresAt:  accessExp,
		CreatedAt:  now,
	}

	refreshRecord := &models.AuthToken{
		TokenID:    newRefreshToken,
		TokenType:  "refresh",
		ClientInfo: clientInfo,
		ExpiresAt:  refreshExp,
		CreatedAt:  now,
	}

	if err := s.tokens.Create(ctx, accessRecord); err != nil {
		return nil, fmt.Errorf("failed to save new access token: %w", err)
	}

	if err := s.tokens.Create(ctx, refreshRecord); err != nil {
		return nil, fmt.Errorf("failed to save new refresh token: %w", err)
	}

	return &RefreshResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(accessExp.Sub(now).Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// ValidateToken 验证 Token
func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	// 先尝试从缓存获取
	if cacheEntry, found := s.getCachedToken(tokenString); found {
		return cacheEntry.Claims, nil
	}

	// 缓存未命中，解析 Token 获取 claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// 如果是插件系统的 Token，跳过数据库撤销检查
	// 因为插件 Token 不保存在数据库中
	if claims.ClientID == "plugin-system" {
		// 插件 Token 也缓存起来，使用 token 的过期时间
		s.setTokenCache(tokenString, claims, claims.ExpiresAt.Time, false)
		return claims, nil
	}

	// 对于普通用户 Token，检查是否被撤销
	isRevoked, err := s.tokens.IsRevoked(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to check token status: %w", err)
	}

	if isRevoked {
		// 缓存撤销状态，使用 token 的过期时间
		s.setTokenCache(tokenString, claims, claims.ExpiresAt.Time, true)
		return nil, fmt.Errorf("token has been revoked")
	}

	// 缓存有效 Token，使用 token 的过期时间
	s.setTokenCache(tokenString, claims, claims.ExpiresAt.Time, false)

	return claims, nil
}

// ListActiveTokens 列出活跃Token
func (s *AuthService) ListActiveTokens(ctx context.Context, filter *database.TokenFilter) ([]*models.AuthToken, error) {
	return s.tokens.ListActive(ctx, filter)
}

// RevokeToken 撤销 Token
func (s *AuthService) RevokeToken(ctx context.Context, tokenID, revokedBy, reason string) error {
	err := s.tokens.Revoke(ctx, tokenID, revokedBy, reason)
	if err == nil {
		// 清除缓存
		s.deleteTokenCache(tokenID)
	}
	return err
}

// GeneratePluginToken 生成插件专用的永久 JWT Token
// 这个 Token 不会过期，专门用于插件内部调用主程序 API
// 注意：此 Token 不保存到数据库，仅在内存中使用，程序重启后会重新生成
func (s *AuthService) GeneratePluginToken(ctx context.Context) (string, error) {
	clientID := "plugin-system"

	// 生成一个 100 年后过期的 Token（实际上相当于永久）
	expirationTime := time.Now().Add(100 * 365 * 24 * time.Hour)

	claims := &Claims{
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        generateRandomString(32), // Token ID
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("failed to generate plugin token: %w", err)
	}

	// 注意：插件 Token 不保存到数据库，因为：
	// 1. 数据库的 token_type 约束只允许 'access' 和 'refresh'
	// 2. 插件 Token 是内部使用，不需要持久化
	// 3. 程序重启后会自动重新生成
	//
	// 这样做的好处：
	// - 不需要修改数据库 schema
	// - 不需要数据库迁移
	// - Token 验证仍然通过 JWT 签名保证安全性

	return tokenString, nil
}

// generateToken 生成JWT Token
func (s *AuthService) generateToken(clientID, tokenType string, expiresIn time.Duration) (string, time.Time, error) {
	expirationTime := time.Now().Add(expiresIn)

	claims := &Claims{
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        generateRandomString(32), // Token ID
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// generateClientID 生成客户端ID
func generateClientID() (string, error) {
	return generateRandomString(16), nil
}

// generateRandomString 生成随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// fallback to time-based random string
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)[:length]
}
