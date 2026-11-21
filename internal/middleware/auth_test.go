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
	"songloft/internal/services/playactivity"
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
		if got := playactivity.SessionFromContext(r.Context()).ClientID; got == "" {
			t.Error("play activity session client ID should be populated by auth middleware")
		}
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

func TestAuthMiddlewareRestoresSpaceSeparatedPlaybackQuery(t *testing.T) {
	authService := newAuthServiceForTest(t)
	defer authService.Close()
	login, err := authService.Login(context.Background(), "testuser", "testpass", "speaker")
	if err != nil {
		t.Fatal(err)
	}

	handler := AuthMiddleware(authService)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, want := range map[string]string{"format": "mp3", "seek": "12", "speed": "1.25"} {
			if got := r.URL.Query().Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/songs/7/play?access_token="+
		login.AccessToken+"%20format=mp3%20seek=12%20speed=1.25", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
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
