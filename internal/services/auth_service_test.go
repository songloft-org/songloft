package services

import (
	"context"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

// authTestEnv 把 :memory: SQLite 下 auth 测试需要的两个仓储打包好。
type authTestEnv struct {
	configs *database.ConfigRepository
	tokens  *database.TokenRepository
}

func newAuthTestEnv(t *testing.T) *authTestEnv {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	return &authTestEnv{
		configs: mdb.ConfigRepository(),
		tokens:  mdb.TokenRepository(),
	}
}

func (e *authTestEnv) seedJWTSecret(t *testing.T, secret string) {
	t.Helper()
	if err := e.configs.Set(context.Background(), &models.Config{Key: "jwt_secret", Value: secret}); err != nil {
		t.Fatalf("seed jwt_secret: %v", err)
	}
}

// TestNewAuthService 测试创建认证服务
func TestNewAuthService(t *testing.T) {
	env := newAuthTestEnv(t)
	env.seedJWTSecret(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	service, err := NewAuthService(env.configs, env.tokens, "admin", "password")
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	if service == nil {
		t.Fatal("Auth service should not be nil")
	}
}

// TestGenerateSecret 测试生成密钥
func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	if secret == "" {
		t.Error("Generated secret should not be empty")
	}

	if len(secret) != 64 {
		t.Errorf("Generated secret should be 64 characters long, got %d", len(secret))
	}
}

// TestAuthService_Login 测试登录功能
func TestAuthService_Login(t *testing.T) {
	env := newAuthTestEnv(t)
	secret, _ := GenerateSecret()
	env.seedJWTSecret(t, secret)

	service, err := NewAuthService(env.configs, env.tokens, "admin", "password")
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	resp, err := service.Login(context.Background(), "admin", "password", "test-client")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("Access token should not be empty")
	}
	if resp.RefreshToken == "" {
		t.Error("Refresh token should not be empty")
	}
	if resp.ExpiresIn <= 0 {
		t.Error("ExpiresIn should be positive")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("TokenType should be 'Bearer', got '%s'", resp.TokenType)
	}

	if _, err := service.Login(context.Background(), "admin", "wrong-password", "test-client"); err == nil {
		t.Error("Login should fail with wrong password")
	}
}

// TestAuthService_Logout 测试登出功能
func TestAuthService_Logout(t *testing.T) {
	env := newAuthTestEnv(t)
	secret, _ := GenerateSecret()
	env.seedJWTSecret(t, secret)

	service, err := NewAuthService(env.configs, env.tokens, "admin", "password")
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	loginResp, err := service.Login(context.Background(), "admin", "password", "test-client")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if err := service.Logout(context.Background(), loginResp.AccessToken, "test-client"); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Logout 是按 token 字符串解析出 tokenID 后撤销, 这里走与登出相同的解析路径校验。
	claims, err := service.ValidateToken(context.Background(), loginResp.AccessToken)
	if err == nil {
		t.Errorf("ValidateToken should fail after logout, got claims=%+v", claims)
	}
}

// TestAuthService_ValidateToken 测试token验证功能
func TestAuthService_ValidateToken(t *testing.T) {
	env := newAuthTestEnv(t)
	secret, _ := GenerateSecret()
	env.seedJWTSecret(t, secret)

	service, err := NewAuthService(env.configs, env.tokens, "admin", "password")
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	loginResp, err := service.Login(context.Background(), "admin", "password", "test-client")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	claims, err := service.ValidateToken(context.Background(), loginResp.AccessToken)
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}

	if claims.ClientID == "" {
		t.Error("ClientID should not be empty")
	}
}
