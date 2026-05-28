package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

// newAuthServiceForTest 启动 :memory: SQLite 并构造一份可用的 AuthService。
// JWT secret 直接写入 ConfigRepository，避免依赖隐式 lazy 初始化。
func newAuthServiceForTest(t *testing.T) *services.AuthService {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)

	secret, err := services.GenerateSecret()
	if err != nil {
		t.Fatalf("generate jwt secret: %v", err)
	}
	if err := mdb.ConfigRepository().Set(context.Background(), &models.Config{
		Key:   "jwt_secret",
		Value: secret,
	}); err != nil {
		t.Fatalf("seed jwt_secret: %v", err)
	}

	svc, err := services.NewAuthService(mdb.ConfigRepository(), mdb.TokenRepository(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	return svc
}

// TestAuthMiddlewareSuccess 测试认证成功
func TestAuthMiddlewareSuccess(t *testing.T) {
	authService := newAuthServiceForTest(t)

	loginResp, err := authService.Login(context.Background(), "testuser", "testpass", "test-client")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	mw := AuthMiddleware(authService)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	if rr.Body.String() != "success" {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), "success")
	}
}

// TestAuthMiddlewareMissingToken 测试缺少 token
func TestAuthMiddlewareMissingToken(t *testing.T) {
	authService := newAuthServiceForTest(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when token is missing")
	})

	mw := AuthMiddleware(authService)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusUnauthorized)
	}

	var errResp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if errResp["error"] != "缺少认证信息" {
		t.Errorf("handler returned unexpected error: got %v", errResp["error"])
	}
}
