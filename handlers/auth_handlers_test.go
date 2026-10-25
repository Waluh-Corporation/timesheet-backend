package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

func TestAuthHandlers_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RPOrigins: []string{"http://localhost:3000", "https://timesheet.example.com"},
	}
	srv := &Server{
		Cfg:         cfg,
		RedisClient: newTestRedisClient(t),
	}

	t.Run("WebAuthnRelatedOrigins returns configured origins", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/.well-known/webauthn", nil)

		srv.WebAuthnRelatedOrigins(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp["data"].(map[string]interface{})
		origins, _ := data["origins"].([]interface{})
		if len(origins) != 2 {
			t.Errorf("expected 2 origins, got %v", data["origins"])
		}
	})

	t.Run("Login rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ForgotPassword rejects empty payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ForgotPassword(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ResetPassword rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ResetPassword(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("BeginPasskeyLogin without webauthn returns 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.BeginPasskeyLogin(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("FinishPasskeyLogin rejects empty payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish", bytes.NewReader([]byte("{}")))
		srv.FinishPasskeyLogin(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("RefreshToken rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.RefreshToken(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("Auth endpoints with nil repositories return 500", func(t *testing.T) {
		// Login nil repo
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(`{"identifier":"user","password":"pass"}`)))
		c1.Request.Header.Set("Content-Type", "application/json")
		srv.Login(c1)
		assertResponseCode(t, w1, http.StatusInternalServerError)

		// RefreshToken nil repo
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{"refresh_token":"abc"}`)))
		c2.Request.Header.Set("Content-Type", "application/json")
		srv.RefreshToken(c2)
		assertResponseCode(t, w2, http.StatusInternalServerError)

		// Me nil repo
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		srv.Me(c3)
		assertResponseCode(t, w3, http.StatusInternalServerError)

		// ResetPassword nil repo
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader([]byte(`{"token":"abc","password":"Password123!"}`)))
		c4.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(c4)
		assertResponseCode(t, w4, http.StatusInternalServerError)

		// BeginPasskeyRegistration nil repo
		w5 := httptest.NewRecorder()
		c5, _ := gin.CreateTestContext(w5)
		c5.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/begin", nil)
		srv.BeginPasskeyRegistration(c5)
		assertResponseCode(t, w5, http.StatusInternalServerError)

		// ListPasskeys nil repo
		w6 := httptest.NewRecorder()
		c6, _ := gin.CreateTestContext(w6)
		c6.Request = httptest.NewRequest(http.MethodGet, "/api/v1/passkeys", nil)
		srv.ListPasskeys(c6)
		assertResponseCode(t, w6, http.StatusInternalServerError)

		// DeletePasskey nil repo
		w7 := httptest.NewRecorder()
		c7, _ := gin.CreateTestContext(w7)
		c7.Params = gin.Params{{Key: "id", Value: "1"}}
		c7.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/passkeys/1", nil)
		srv.DeletePasskey(c7)
		assertResponseCode(t, w7, http.StatusInternalServerError)

		// AdminListPasskeys nil repo
		w8 := httptest.NewRecorder()
		c8, _ := gin.CreateTestContext(w8)
		c8.Params = gin.Params{{Key: "id", Value: "1"}}
		c8.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1/passkeys", nil)
		srv.AdminListPasskeys(c8)
		assertResponseCode(t, w8, http.StatusInternalServerError)

		// AdminDeletePasskey is forbidden by policy
		w9 := httptest.NewRecorder()
		c9, _ := gin.CreateTestContext(w9)
		c9.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "pid", Value: "1"}}
		c9.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1/passkeys/1", nil)
		srv.AdminDeletePasskey(c9)
		assertResponseCode(t, w9, http.StatusForbidden)
	})

	t.Run("Passkey endpoints reject invalid id parameters", func(t *testing.T) {
		// DeletePasskey invalid id
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Params = gin.Params{{Key: "id", Value: "not-an-id"}}
		c1.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/passkeys/not-an-id", nil)
		srv.DeletePasskey(c1)
		assertResponseCode(t, w1, http.StatusBadRequest)

		// AdminListPasskeys invalid user id
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Params = gin.Params{{Key: "id", Value: "not-an-id"}}
		c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/not-an-id/passkeys", nil)
		srv.AdminListPasskeys(c2)
		assertResponseCode(t, w2, http.StatusBadRequest)

		// AdminDeletePasskey is forbidden by policy regardless of pid
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "pid", Value: "not-an-id"}}
		c3.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1/passkeys/not-an-id", nil)
		srv.AdminDeletePasskey(c3)
		assertResponseCode(t, w3, http.StatusForbidden)

		// FinishPasskeyRegistration unknown session
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=unknown-sid", nil)
		srv.FinishPasskeyRegistration(c4)
		assertResponseCode(t, w4, http.StatusBadRequest)

		// FinishPasskeyLogin unknown session
		w5 := httptest.NewRecorder()
		c5, _ := gin.CreateTestContext(w5)
		c5.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish?session_id=unknown-sid", nil)
		srv.FinishPasskeyLogin(c5)
		assertResponseCode(t, w5, http.StatusBadRequest)

		// Put dummy session and test nil repo for finish endpoints
		srv.putSession("test-dummy-sid", &webauthn.SessionData{})
		w6 := httptest.NewRecorder()
		c6, _ := gin.CreateTestContext(w6)
		c6.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=test-dummy-sid", nil)
		srv.FinishPasskeyRegistration(c6)
		assertResponseCode(t, w6, http.StatusInternalServerError)

		srv.putSession("test-dummy-sid-2", &webauthn.SessionData{})
		w7 := httptest.NewRecorder()
		c7, _ := gin.CreateTestContext(w7)
		c7.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish?session_id=test-dummy-sid-2", nil)
		srv.FinishPasskeyLogin(c7)
		assertResponseCode(t, w7, http.StatusInternalServerError)

		// Me nil repo
		wMe := httptest.NewRecorder()
		cMe, _ := gin.CreateTestContext(wMe)
		cMe.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		srv.Me(cMe)
		assertResponseCode(t, wMe, http.StatusInternalServerError)

		// BeginPasskeyRegistration nil repo
		wBPR := httptest.NewRecorder()
		cBPR, _ := gin.CreateTestContext(wBPR)
		cBPR.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/begin", nil)
		srv.BeginPasskeyRegistration(cBPR)
		assertResponseCode(t, wBPR, http.StatusInternalServerError)

		// BeginPasskeyLogin nil WebAuthn
		wBPL := httptest.NewRecorder()
		cBPL, _ := gin.CreateTestContext(wBPL)
		cBPL.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", nil)
		srv.BeginPasskeyLogin(cBPL)
		assertResponseCode(t, wBPL, http.StatusInternalServerError)

		// BeginPasskeyLogin with identifier and nil repo
		wa, _ := webauthn.New(&webauthn.Config{RPDisplayName: "Test", RPID: "localhost"})
		srvWithWA := &Server{WebAuthn: wa}
		wBPL2 := httptest.NewRecorder()
		cBPL2, _ := gin.CreateTestContext(wBPL2)
		cBPL2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{"identifier":"user123"}`))
		cBPL2.Request.Header.Set("Content-Type", "application/json")
		srvWithWA.BeginPasskeyLogin(cBPL2)
		assertResponseCode(t, wBPL2, http.StatusInternalServerError)

		// Logout with empty token
		wLogout := httptest.NewRecorder()
		cLogout, _ := gin.CreateTestContext(wLogout)
		cLogout.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{}`))
		cLogout.Request.Header.Set("Content-Type", "application/json")
		srv.Logout(cLogout)
		assertResponseCode(t, wLogout, http.StatusOK)
	})
}

type mockAuthServiceForHandlers struct {
	loginResult   *service.LoginResult
	loginErr      error
	refreshResult *service.RefreshTokenResult
	refreshErr    error
	revokeErr     error
	revokedToken  string
	profileUser   *models.User
	profileErr    error
}

func (m *mockAuthServiceForHandlers) Login(ctx context.Context, dto service.LoginDTO) (*service.LoginResult, error) {
	if m.loginErr != nil {
		return nil, m.loginErr
	}
	return m.loginResult, nil
}

func (m *mockAuthServiceForHandlers) RefreshToken(ctx context.Context, dto service.RefreshTokenDTO) (*service.RefreshTokenResult, error) {
	if m.refreshErr != nil {
		return nil, m.refreshErr
	}
	return m.refreshResult, nil
}

func (m *mockAuthServiceForHandlers) RevokeRefreshToken(ctx context.Context, refreshToken, clientIP string) error {
	m.revokedToken = refreshToken
	return m.revokeErr
}

func (m *mockAuthServiceForHandlers) GetProfile(ctx context.Context, userID uint) (*models.User, error) {
	if m.profileErr != nil {
		return nil, m.profileErr
	}
	return m.profileUser, nil
}

func TestAuthHandlers_ThinControllerDelegation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Login success maps to LoginResponse", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			loginResult: &service.LoginResult{
				AccessToken:  "access-123",
				RefreshToken: "refresh-123",
				User: &models.User{
					ID:       1,
					Username: "john",
					Email:    "john@example.com",
					Role:     models.RoleUser,
				},
			},
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"john","password":"secret"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp["data"].(map[string]interface{})
		if data["token"] != "access-123" || data["refresh_token"] != "refresh-123" {
			t.Fatalf("unexpected login response: %v", data)
		}
	})

	t.Run("Login unauthorized mapped to 401", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			loginErr: domain.NewUserError(domain.ErrUnauthorized, "invalid credentials"),
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"john","password":"wrong"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusUnauthorized)
	})

	t.Run("Login forbidden mapped to 403", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			loginErr: domain.NewUserError(domain.ErrForbidden, "account is disabled"),
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"john","password":"pass"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusForbidden)
	})

	t.Run("RefreshToken success maps to RefreshResponse", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			refreshResult: &service.RefreshTokenResult{
				AccessToken:  "new-access",
				RefreshToken: "new-refresh",
			},
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"old-token"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.RefreshToken(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp["data"].(map[string]interface{})
		if data["token"] != "new-access" || data["refresh_token"] != "new-refresh" {
			t.Fatalf("unexpected refresh response: %v", data)
		}
	})

	t.Run("RefreshToken error mapped to status code", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			refreshErr: domain.NewUserError(domain.ErrUnauthorized, "token reuse detected, please sign in again"),
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"reused"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.RefreshToken(c)
		assertResponseCode(t, w, http.StatusUnauthorized)
	})

	t.Run("Logout delegates refresh token revocation to AuthService", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{"refresh_token":"token-to-revoke"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Logout(c)
		assertResponseCode(t, w, http.StatusOK)
		if mockSvc.revokedToken != "token-to-revoke" {
			t.Errorf("expected revoked token to be 'token-to-revoke', got '%s'", mockSvc.revokedToken)
		}
	})

	t.Run("Me success maps to UserResponse", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			profileUser: &models.User{
				ID:       1,
				Username: "profileuser",
				Email:    "profile@example.com",
				Role:     models.RoleUser,
				IsActive: true,
			},
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Set("userID", uint(1))

		srv.Me(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp["data"].(map[string]interface{})
		if data["username"] != "profileuser" {
			t.Errorf("expected username profileuser, got %v", data["username"])
		}
	})

	t.Run("Me user not found returns 404", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			profileErr: domain.NewUserError(domain.ErrNotFound, "user not found"),
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Set("userID", uint(999))

		srv.Me(c)
		assertResponseCode(t, w, http.StatusNotFound)
	})

	t.Run("Logout revocation error still returns 200", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{
			revokeErr: errors.New("db error revoking token"),
		}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{"refresh_token":"tok-fail"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Logout(c)
		assertResponseCode(t, w, http.StatusOK)
		if mockSvc.revokedToken != "tok-fail" {
			t.Errorf("expected revoked token 'tok-fail', got '%s'", mockSvc.revokedToken)
		}
	})

	t.Run("getAuthService handles dynamic dependency mutations", func(t *testing.T) {
		initialAuthSvc := service.NewAuthService(nil, nil, nil, nil, 0)
		srv := &Server{AuthSvc: initialAuthSvc}

		customMock := &mockAuthServiceForHandlers{}
		srvMock := &Server{AuthSvc: customMock}
		if srvMock.getAuthService() != customMock {
			t.Fatal("expected custom mock to be returned directly")
		}

		if srv.getAuthService() == nil {
			t.Fatal("expected non-nil AuthService")
		}

		type dummyTokenRepo struct{ repository.TokenRepository }
		mockTokRepo := &dummyTokenRepo{}
		srv.TokenRepo = mockTokRepo
		updated := srv.getAuthService()
		if checker, ok := updated.(authServiceRepoChecker); !ok || checker.GetTokenRepo() != mockTokRepo {
			t.Fatal("expected getAuthService to reflect mutated TokenRepo")
		}

		type dummyUserRepo struct{ repository.UserRepository }
		mockURepo := &dummyUserRepo{}
		srv.UserRepo = mockURepo
		updated2 := srv.getAuthService()
		if checker, ok := updated2.(authServiceRepoChecker); !ok || checker.GetUserRepo() != mockURepo {
			t.Fatal("expected getAuthService to reflect mutated UserRepo")
		}

		type dummyHasher struct{ auth.PasswordHasher }
		mockHasher := &dummyHasher{}
		srv.Hasher = mockHasher
		updated3 := srv.getAuthService()
		if checker, ok := updated3.(authServiceRepoChecker); !ok || checker.GetHasher() != mockHasher {
			t.Fatal("expected getAuthService to reflect mutated Hasher")
		}
	})

	t.Run("Login with nil result from service returns 500", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{loginResult: nil}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"john","password":"secret"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("RefreshToken with nil result from service returns 500", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{refreshResult: nil}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"any"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.RefreshToken(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("Me with nil user result from service returns 404", func(t *testing.T) {
		mockSvc := &mockAuthServiceForHandlers{profileUser: nil}
		srv := &Server{AuthSvc: mockSvc}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Set("userID", uint(1))

		srv.Me(c)
		assertResponseCode(t, w, http.StatusNotFound)
	})

	t.Run("getAuthService preserves unmutated dependencies from existing checker", func(t *testing.T) {
		type dummyUserRepo struct{ repository.UserRepository }
		type dummyTokenRepo struct{ repository.TokenRepository }
		type dummyAuditRepo struct{ repository.AuditRepository }
		type dummyHasher struct{ auth.PasswordHasher }

		origUserRepo := &dummyUserRepo{}
		origTokenRepo := &dummyTokenRepo{}
		origAuditRepo := &dummyAuditRepo{}
		origJWT := auth.NewService("test-secret-at-least-32-chars-long!", 3600)
		origHasher := &dummyHasher{}

		baseAuthSvc := service.NewAuthService(origUserRepo, origTokenRepo, origJWT, origAuditRepo, 3600, origHasher)
		srv := &Server{AuthSvc: baseAuthSvc}

		newHasher := &dummyHasher{}
		srv.Hasher = newHasher

		updated := srv.getAuthService()
		checker, ok := updated.(authServiceRepoChecker)
		if !ok {
			t.Fatal("expected updated service to implement authServiceRepoChecker")
		}
		if checker.GetHasher() != newHasher {
			t.Fatal("expected hasher to be updated")
		}
		if checker.GetUserRepo() != origUserRepo {
			t.Fatal("expected userRepo to be preserved from checker")
		}
		if checker.GetTokenRepo() != origTokenRepo {
			t.Fatal("expected tokenRepo to be preserved from checker")
		}
		if checker.GetAuditRepo() != origAuditRepo {
			t.Fatal("expected auditRepo to be preserved from checker")
		}
		if checker.GetJWTService() != origJWT {
			t.Fatal("expected JWT service to be preserved from checker")
		}
	})
}
