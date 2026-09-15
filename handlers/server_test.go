package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

func TestServer_Responses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{}

	// RespondSuccess
	wSuccess := httptest.NewRecorder()
	cSuccess, _ := gin.CreateTestContext(wSuccess)
	srv.RespondSuccess(cSuccess, http.StatusOK, gin.H{"item": "val"})
	if wSuccess.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wSuccess.Code)
	}

	// RespondMessage
	wMsg := httptest.NewRecorder()
	cMsg, _ := gin.CreateTestContext(wMsg)
	srv.RespondMessage(cMsg, http.StatusAccepted, "accepted message")
	if wMsg.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", wMsg.Code)
	}

	// RespondDelete
	wDel := httptest.NewRecorder()
	cDel, _ := gin.CreateTestContext(wDel)
	srv.RespondDelete(cDel, http.StatusOK)
	if wDel.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wDel.Code)
	}

	// RespondError
	wErr := httptest.NewRecorder()
	cErr, _ := gin.CreateTestContext(wErr)
	srv.RespondError(cErr, http.StatusBadRequest, "bad request")
	if wErr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", wErr.Code)
	}

	// RespondAbortError
	wAbort := httptest.NewRecorder()
	cAbort, _ := gin.CreateTestContext(wAbort)
	srv.RespondAbortError(cAbort, http.StatusForbidden, "forbidden abort")
	if wAbort.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", wAbort.Code)
	}
	if !cAbort.IsAborted() {
		t.Errorf("expected context to be aborted")
	}

	// RespondPaginated
	wPage := httptest.NewRecorder()
	cPage, _ := gin.CreateTestContext(wPage)
	srv.RespondPaginated(cPage, http.StatusOK, []string{"a", "b"}, 1, 10, 25)
	if wPage.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wPage.Code)
	}
	var pageResp response.PaginatedResponse
	if err := json.Unmarshal(wPage.Body.Bytes(), &pageResp); err != nil {
		t.Fatalf("failed to decode paginated response: %v", err)
	}
	if pageResp.Pagination.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", pageResp.Pagination.TotalPages)
	}
}

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		RPDisplayName: "Test RP",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000"},
	}
	s, err := NewServer(nil, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s == nil || s.WebAuthn == nil {
		t.Fatal("expected non-nil server and webauthn")
	}
}

func TestServer_PublicBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{
		Cfg: &config.Config{
			FrontendURL: "http://localhost:3000/",
			RPOrigins:   []string{"https://portal.example.com", "http://insecure.example.com"},
		},
	}

	// 1. Fallback to FrontendURL when no proxy headers
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	if u := srv.publicBaseURL(c1); u != "http://localhost:3000" {
		t.Errorf("expected http://localhost:3000, got %s", u)
	}

	// 2. X-Forwarded-Host and X-Forwarded-Proto for trusted domain
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c2.Request.Header.Set("X-Forwarded-Host", "portal.example.com, internal.proxy")
	c2.Request.Header.Set("X-Forwarded-Proto", "https, http")
	if u := srv.publicBaseURL(c2); u != "https://portal.example.com" {
		t.Errorf("expected https://portal.example.com, got %s", u)
	}

	// 3. X-Forwarded-Host without Proto (non-TLS) for trusted domain
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c3.Request.Header.Set("X-Forwarded-Host", "insecure.example.com")
	if u := srv.publicBaseURL(c3); u != "http://insecure.example.com" {
		t.Errorf("expected http://insecure.example.com, got %s", u)
	}

	// 4. Untrusted host header poisoning attempt is rejected and falls back safely
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c4.Request.Header.Set("X-Forwarded-Host", "attacker.evil.com")
	c4.Request.Header.Set("X-Forwarded-Proto", "https")
	if u := srv.publicBaseURL(c4); u != "http://localhost:3000" {
		t.Errorf("expected fallback to http://localhost:3000, got %s", u)
	}
}

func TestServer_Sessions(t *testing.T) {
	srv := &Server{
		webAuthnSessions: make(map[string]*webAuthnSessionEntry),
	}

	data := &webauthn.SessionData{
		Challenge: "test-challenge",
	}

	srv.putSession("session-1", data)

	got, ok := srv.takeSession("session-1")
	if !ok || got == nil || got.Challenge != "test-challenge" {
		t.Fatalf("takeSession failed: got %v, ok %v", got, ok)
	}

	// Subsequent take returns false (one-time use)
	_, ok2 := srv.takeSession("session-1")
	if ok2 {
		t.Errorf("expected session to be consumed on first take")
	}

	// Test expired session (> 5 minutes)
	srv.sessionsMu.Lock()
	srv.webAuthnSessions["expired-session"] = &webAuthnSessionEntry{
		data:      data,
		createdAt: time.Now().Add(-6 * time.Minute),
	}
	srv.sessionsMu.Unlock()

	_, okExpired := srv.takeSession("expired-session")
	if okExpired {
		t.Errorf("expected expired session to be rejected")
	}
}

func TestServer_Middlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 24*time.Hour)
	srv := &Server{
		Auth: authSvc,
	}

	r := gin.New()
	r.Use(CORSMiddleware())
	r.OPTIONS("/cors", func(c *gin.Context) {})

	// Test CORS OPTIONS preflight without Origin
	reqOpts := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	wOpts := httptest.NewRecorder()
	r.ServeHTTP(wOpts, reqOpts)
	if wOpts.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", wOpts.Code)
	}
	if wOpts.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *")
	}

	// Test CORS allowed origin (e.g. localhost:3000 in dev)
	reqAllowed := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqAllowed.Header.Set("Origin", "http://localhost:3000")
	wAllowed := httptest.NewRecorder()
	r.ServeHTTP(wAllowed, reqAllowed)
	if wAllowed.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin to match allowed origin")
	}
	if wAllowed.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true for allowed origin")
	}

	// Test CORS untrusted origin (must not be allowed with credentials)
	reqUntrusted := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqUntrusted.Header.Set("Origin", "http://attacker.evil.com")
	wUntrusted := httptest.NewRecorder()
	r.ServeHTTP(wUntrusted, reqUntrusted)
	if wUntrusted.Header().Get("Access-Control-Allow-Credentials") == "true" {
		t.Errorf("untrusted origin must not receive Access-Control-Allow-Credentials: true")
	}
	if wUntrusted.Header().Get("Access-Control-Allow-Origin") == "http://attacker.evil.com" {
		t.Errorf("untrusted origin must not be reflected in Access-Control-Allow-Origin")
	}

	// Test AuthMiddleware & AdminOnly
	r.GET("/protected", srv.AuthMiddleware(), func(c *gin.Context) {
		uid := currentUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})
	r.GET("/admin-only", srv.AuthMiddleware(), srv.AdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "admin"})
	})

	// Missing token
	reqNoToken := httptest.NewRequest(http.MethodGet, "/protected", nil)
	wNoToken := httptest.NewRecorder()
	r.ServeHTTP(wNoToken, reqNoToken)
	if wNoToken.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on missing token, got %d", wNoToken.Code)
	}

	// Invalid token
	reqBadToken := httptest.NewRequest(http.MethodGet, "/protected", nil)
	reqBadToken.Header.Set("Authorization", "Bearer invalid-token")
	wBadToken := httptest.NewRecorder()
	r.ServeHTTP(wBadToken, reqBadToken)
	if wBadToken.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on invalid token, got %d", wBadToken.Code)
	}

	// Valid user token
	userToken, err := authSvc.GenerateToken(&models.User{ID: 42, Username: "testuser", Role: models.RoleUser})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	reqUser := httptest.NewRequest(http.MethodGet, "/protected", nil)
	reqUser.Header.Set("Authorization", "Bearer "+userToken)
	wUser := httptest.NewRecorder()
	r.ServeHTTP(wUser, reqUser)
	if wUser.Code != http.StatusOK {
		t.Errorf("expected 200 with valid user token, got %d", wUser.Code)
	}

	// Valid user token trying admin-only route -> 403
	reqUserAdmin := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	reqUserAdmin.Header.Set("Authorization", "Bearer "+userToken)
	wUserAdmin := httptest.NewRecorder()
	r.ServeHTTP(wUserAdmin, reqUserAdmin)
	if wUserAdmin.Code != http.StatusForbidden {
		t.Errorf("expected 403 for user accessing admin route, got %d", wUserAdmin.Code)
	}

	// Valid admin token
	adminToken, err := authSvc.GenerateToken(&models.User{ID: 1, Username: "admin", Role: models.RoleAdmin})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	wAdmin := httptest.NewRecorder()
	r.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 for admin accessing admin route, got %d", wAdmin.Code)
	}
}
