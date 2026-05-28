package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mimusic/internal/database/testutil"
	"mimusic/internal/models"
	"mimusic/internal/services"
)

// newAuthHandlerForTest 启动 :memory: SQLite + 真实 AuthService,
// 返回构造好的 handler。AuthService 是 stateful 的(token 缓存),所以每个测试独占一份。
func newAuthHandlerForTest(t *testing.T) *AuthHandler {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)

	secret, _ := services.GenerateSecret()
	if err := mdb.ConfigRepository().Set(context.Background(), &models.Config{
		Key:   "jwt_secret",
		Value: secret,
	}); err != nil {
		t.Fatalf("seed jwt_secret: %v", err)
	}

	svc, err := services.NewAuthService(mdb.ConfigRepository(), mdb.TokenRepository(), "admin", "password")
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	return NewAuthHandler(svc)
}

// TestAuthHandler_Login 测试登录处理器
func TestAuthHandler_Login(t *testing.T) {
	handler := newAuthHandlerForTest(t)

	loginReq := models.LoginRequest{
		Username: "admin",
		Password: "password",
	}

	jsonData, _ := json.Marshal(loginReq)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	var resp models.LoginResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if resp.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}
	if resp.ExpiresIn <= 0 {
		t.Error("ExpiresIn should be positive")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("Expected TokenType Bearer, got %s", resp.TokenType)
	}
}

// TestAuthHandler_Login_InvalidRequest 测试登录处理器 - 无效请求
func TestAuthHandler_Login_InvalidRequest(t *testing.T) {
	handler := newAuthHandlerForTest(t)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer([]byte("{invalid-json}")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestAuthHandler_Login_ServiceError 测试登录处理器 - 服务错误（密码错误）
func TestAuthHandler_Login_ServiceError(t *testing.T) {
	handler := newAuthHandlerForTest(t)

	loginReq := models.LoginRequest{
		Username: "admin",
		Password: "wrong-password",
	}

	jsonData, _ := json.Marshal(loginReq)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// TestAuthHandler_RefreshToken 测试刷新token处理器
func TestAuthHandler_RefreshToken(t *testing.T) {
	handler := newAuthHandlerForTest(t)

	// 先登录获取真实的 refresh token
	loginReq := models.LoginRequest{
		Username: "admin",
		Password: "password",
	}
	loginData, _ := json.Marshal(loginReq)
	loginRequest := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(loginData))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	handler.Login(loginRecorder, loginRequest)

	var loginResp models.LoginResponse
	json.Unmarshal(loginRecorder.Body.Bytes(), &loginResp)

	refreshReq := models.RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	}

	jsonData, _ := json.Marshal(refreshReq)
	req := httptest.NewRequest("POST", "/auth/refresh", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	var resp services.RefreshResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if resp.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}
}

// TestAuthHandler_RefreshToken_InvalidRequest 测试刷新token处理器 - 无效请求
func TestAuthHandler_RefreshToken_InvalidRequest(t *testing.T) {
	handler := newAuthHandlerForTest(t)

	req := httptest.NewRequest("POST", "/auth/refresh", bytes.NewBuffer([]byte("{invalid-json}")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestAuthHandler_Logout 测试登出处理器
func TestAuthHandler_Logout(t *testing.T) {
	handler := newAuthHandlerForTest(t)

	loginReq := models.LoginRequest{
		Username: "admin",
		Password: "password",
	}
	loginData, _ := json.Marshal(loginReq)
	loginRequest := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(loginData))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	handler.Login(loginRecorder, loginRequest)

	var loginResp models.LoginResponse
	json.Unmarshal(loginRecorder.Body.Bytes(), &loginResp)

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	req.Header.Set("X-Client-ID", "test-client")

	rr := httptest.NewRecorder()
	handler.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}
}

// TestAuthHandler_ListTokens 测试列出token处理器
func TestAuthHandler_ListTokens(t *testing.T) {
	handler := newAuthHandlerForTest(t)

	req := httptest.NewRequest("GET", "/auth/tokens", nil)
	req.Header.Set("Authorization", "Bearer test-access-token")

	rr := httptest.NewRecorder()
	handler.ListTokens(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["limit"] != float64(20) {
		t.Errorf("Expected limit 20, got %v", resp["limit"])
	}
}
