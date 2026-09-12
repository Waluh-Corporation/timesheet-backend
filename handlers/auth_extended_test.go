package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
)

func TestAuthHandlers_FullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	m := mailer.New(cfg)
	srv, err := NewServer(tx, cfg, authSvc, m, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	rawPass := "MyStr0ngPassw0rd!2026"
	hash, err := auth.HashPassword(rawPass)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	testUser := models.User{
		Username:     "authtestuser",
		Email:        "authtest@example.com",
		Name:         "Auth Test User",
		PasswordHash: string(hash),
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := tx.Create(&testUser).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	inactiveUser := models.User{
		Username:     "inactiveuser",
		Email:        "inactive@example.com",
		Name:         "Inactive User",
		PasswordHash: string(hash),
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := tx.Create(&inactiveUser).Error; err != nil {
		t.Fatalf("failed to create inactive user: %v", err)
	}
	_ = tx.Model(&inactiveUser).Update("is_active", false)

	t.Run("Login success with username", func(t *testing.T) {
		body, _ := json.Marshal(request.LoginRequest{
			Identifier: "authtestuser",
			Password:   rawPass,
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp["data"].(map[string]interface{})
		if data["token"] == nil || data["token"] == "" {
			t.Errorf("expected session token in response")
		}
	})

	t.Run("Login success with email", func(t *testing.T) {
		body, _ := json.Marshal(request.LoginRequest{
			Identifier: "authtest@example.com",
			Password:   rawPass,
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("Login success with legacy bcrypt hash triggers rehash", func(t *testing.T) {
		legacyPass := "LegacyPassword123!"
		bHash, err := bcrypt.GenerateFromPassword([]byte(legacyPass), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("bcrypt error: %v", err)
		}
		legacyUser := models.User{
			Username:     "legacyuser",
			Email:        "legacy@example.com",
			Name:         "Legacy User",
			PasswordHash: string(bHash),
			Role:         models.RoleUser,
			IsActive:     true,
		}
		if err := tx.Create(&legacyUser).Error; err != nil {
			t.Fatalf("failed to create legacy user: %v", err)
		}

		body, _ := json.Marshal(request.LoginRequest{
			Identifier: "legacyuser",
			Password:   legacyPass,
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusOK)

		var updated models.User
		_ = tx.Where(queryID, legacyUser.ID).First(&updated)
		if !strings.HasPrefix(updated.PasswordHash, "$argon2id$") {
			t.Errorf("expected upgraded Argon2id hash, got: %s", updated.PasswordHash)
		}
	})

	t.Run("Login wrong password", func(t *testing.T) {
		body, _ := json.Marshal(request.LoginRequest{
			Identifier: "authtestuser",
			Password:   "WrongPass123!",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusUnauthorized)
	})

	t.Run("Login nonexistent identifier", func(t *testing.T) {
		body, _ := json.Marshal(request.LoginRequest{
			Identifier: "nonexistent",
			Password:   rawPass,
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusUnauthorized)
	})

	t.Run("Login inactive user", func(t *testing.T) {
		body, _ := json.Marshal(request.LoginRequest{
			Identifier: "inactiveuser",
			Password:   rawPass,
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusForbidden)
	})

	t.Run("Me returns current user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Set(ctxUserID, testUser.ID)

		srv.Me(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("Me user not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Set(ctxUserID, uint(999999))

		srv.Me(c)
		assertResponseCode(t, w, http.StatusNotFound)
	})

	t.Run("ForgotPassword with valid user", func(t *testing.T) {
		body, _ := json.Marshal(request.ForgotRequest{
			Email: "authtest@example.com",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ForgotPassword(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ForgotPassword with nonexistent user succeeds silently", func(t *testing.T) {
		body, _ := json.Marshal(request.ForgotRequest{
			Email: "ghost@example.com",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ForgotPassword(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ResetPassword full lifecycle", func(t *testing.T) {
		rawToken, tokenHash, err := auth.GenerateResetToken()
		if err != nil {
			t.Fatalf("token gen error: %v", err)
		}
		resetToken := models.PasswordResetToken{
			UserID:    testUser.ID,
			TokenType: "password_reset",
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		if err := tx.Create(&resetToken).Error; err != nil {
			t.Fatalf("create token error: %v", err)
		}

		// Reset with password violating policy (too short)
		shortPassPayload, _ := json.Marshal(request.ResetRequest{
			Token:    rawToken,
			Password: "short",
		})
		wShort := httptest.NewRecorder()
		cShort, _ := gin.CreateTestContext(wShort)
		cShort.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(shortPassPayload))
		cShort.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(cShort)
		assertResponseCode(t, wShort, http.StatusBadRequest)

		// Reset with valid new password
		newPass := "Br4ndNewSecurePass!2026"
		validPayload, _ := json.Marshal(request.ResetRequest{
			Token:    rawToken,
			Password: newPass,
		})
		wValid := httptest.NewRecorder()
		cValid, _ := gin.CreateTestContext(wValid)
		cValid.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(validPayload))
		cValid.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(cValid)
		assertResponseCode(t, wValid, http.StatusOK)

		// Trying to use same token again should fail
		wReused := httptest.NewRecorder()
		cReused, _ := gin.CreateTestContext(wReused)
		cReused.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(validPayload))
		cReused.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(cReused)
		assertResponseCode(t, wReused, http.StatusBadRequest)
	})

	t.Run("Passkey endpoints and decodeUserHandle", func(t *testing.T) {
		// Test decodeUserHandle
		h1 := decodeUserHandle([]byte{42, 0, 0, 0, 0, 0, 0, 0})
		if h1 != 42 {
			t.Errorf("expected 42, got %d", h1)
		}
		hZero := decodeUserHandle(nil)
		if hZero != 0 {
			t.Errorf("expected 0, got %d", hZero)
		}

		// Seed a credential
		cred := models.WebAuthnCredential{
			UserID:       testUser.ID,
			CredentialID: []byte("sample-cred-id-1"),
			PublicKey:    []byte("sample-public-key"),
			FriendlyName: "My Security Key",
		}
		if err := tx.Create(&cred).Error; err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}

		// ListPasskeys
		wList := httptest.NewRecorder()
		cList, _ := gin.CreateTestContext(wList)
		cList.Request = httptest.NewRequest(http.MethodGet, "/api/v1/passkeys", nil)
		cList.Set(ctxUserID, testUser.ID)
		srv.ListPasskeys(cList)
		assertResponseCode(t, wList, http.StatusOK)

		// AdminListPasskeys
		wAdminList := httptest.NewRecorder()
		cAdminList, _ := gin.CreateTestContext(wAdminList)
		cAdminList.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d/passkeys", testUser.ID), nil)
		cAdminList.Params = gin.Params{{Key: "id", Value: fmt.Sprint(testUser.ID)}}
		srv.AdminListPasskeys(cAdminList)
		assertResponseCode(t, wAdminList, http.StatusOK)

		// DeletePasskey (self-service)
		wDel := httptest.NewRecorder()
		cDel, _ := gin.CreateTestContext(wDel)
		cDel.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), nil)
		cDel.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred.ID)}}
		cDel.Set(ctxUserID, testUser.ID)
		srv.DeletePasskey(cDel)
		assertResponseCode(t, wDel, http.StatusOK)

		// DeletePasskey 404
		wDel404 := httptest.NewRecorder()
		cDel404, _ := gin.CreateTestContext(wDel404)
		cDel404.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), nil)
		cDel404.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred.ID)}}
		cDel404.Set(ctxUserID, testUser.ID)
		srv.DeletePasskey(cDel404)
		assertResponseCode(t, wDel404, http.StatusNotFound)

		// AdminDeletePasskey
		cred2 := models.WebAuthnCredential{
			UserID:       testUser.ID,
			CredentialID: []byte("sample-cred-id-2"),
			PublicKey:    []byte("sample-public-key-2"),
			FriendlyName: "Admin Deletable Key",
		}
		if err := tx.Create(&cred2).Error; err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}

		wAdminDel := httptest.NewRecorder()
		cAdminDel, _ := gin.CreateTestContext(wAdminDel)
		cAdminDel.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d/passkeys/%d", testUser.ID, cred2.ID), nil)
		cAdminDel.Params = gin.Params{
			{Key: "id", Value: fmt.Sprint(testUser.ID)},
			{Key: "pid", Value: fmt.Sprint(cred2.ID)},
		}
		srv.AdminDeletePasskey(cAdminDel)
		assertResponseCode(t, wAdminDel, http.StatusOK)

		// AdminDeletePasskey 404
		wAdminDel404 := httptest.NewRecorder()
		cAdminDel404, _ := gin.CreateTestContext(wAdminDel404)
		cAdminDel404.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d/passkeys/%d", testUser.ID, cred2.ID), nil)
		cAdminDel404.Params = gin.Params{
			{Key: "id", Value: fmt.Sprint(testUser.ID)},
			{Key: "pid", Value: fmt.Sprint(cred2.ID)},
		}
		srv.AdminDeletePasskey(cAdminDel404)
		assertResponseCode(t, wAdminDel404, http.StatusNotFound)

		// BeginPasskeyRegistration
		wRegBegin := httptest.NewRecorder()
		cRegBegin, _ := gin.CreateTestContext(wRegBegin)
		cRegBegin.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/begin", nil)
		cRegBegin.Set(ctxUserID, testUser.ID)
		srv.BeginPasskeyRegistration(cRegBegin)
		assertResponseCode(t, wRegBegin, http.StatusOK)

		// BeginPasskeyRegistration 404 user not found
		wRegBegin404 := httptest.NewRecorder()
		cRegBegin404, _ := gin.CreateTestContext(wRegBegin404)
		cRegBegin404.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/begin", nil)
		cRegBegin404.Set(ctxUserID, uint(999999))
		srv.BeginPasskeyRegistration(cRegBegin404)
		assertResponseCode(t, wRegBegin404, http.StatusNotFound)

		// FinishPasskeyRegistration missing/invalid session
		wRegFin400 := httptest.NewRecorder()
		cRegFin400, _ := gin.CreateTestContext(wRegFin400)
		cRegFin400.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=nonexistent", nil)
		srv.FinishPasskeyRegistration(cRegFin400)
		assertResponseCode(t, wRegFin400, http.StatusBadRequest)

		// FinishPasskeyRegistration with session but user not found
		fakeSid := "fake-reg-session-1"
		srv.putSession(fakeSid, &webauthn.SessionData{UserID: []byte{1}})
		wRegFinUser404 := httptest.NewRecorder()
		cRegFinUser404, _ := gin.CreateTestContext(wRegFinUser404)
		cRegFinUser404.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/passkey/register/finish?session_id=%s", fakeSid), nil)
		cRegFinUser404.Set(ctxUserID, uint(999999))
		srv.FinishPasskeyRegistration(cRegFinUser404)
		assertResponseCode(t, wRegFinUser404, http.StatusNotFound)

		// FinishPasskeyRegistration with valid session but invalid webauthn request body
		fakeSid2 := "fake-reg-session-2"
		srv.putSession(fakeSid2, &webauthn.SessionData{UserID: []byte{1}})
		wRegFinBad := httptest.NewRecorder()
		cRegFinBad, _ := gin.CreateTestContext(wRegFinBad)
		cRegFinBad.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/passkey/register/finish?session_id=%s", fakeSid2), strings.NewReader("{}"))
		cRegFinBad.Set(ctxUserID, testUser.ID)
		srv.FinishPasskeyRegistration(cRegFinBad)
		assertResponseCode(t, wRegFinBad, http.StatusBadRequest)

		// BeginPasskeyLogin (discoverable)
		wLoginBegin := httptest.NewRecorder()
		cLoginBegin, _ := gin.CreateTestContext(wLoginBegin)
		cLoginBegin.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader("{}"))
		cLoginBegin.Request.Header.Set("Content-Type", "application/json")
		srv.BeginPasskeyLogin(cLoginBegin)
		assertResponseCode(t, wLoginBegin, http.StatusOK)

		// BeginPasskeyLogin (user-scoped without credentials returns 500)
		wLoginBeginUserNoCreds := httptest.NewRecorder()
		cLoginBeginUserNoCreds, _ := gin.CreateTestContext(wLoginBeginUserNoCreds)
		cLoginBeginUserNoCreds.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{"identifier":"authtest@example.com"}`))
		cLoginBeginUserNoCreds.Request.Header.Set("Content-Type", "application/json")
		srv.BeginPasskeyLogin(cLoginBeginUserNoCreds)
		assertResponseCode(t, wLoginBeginUserNoCreds, http.StatusInternalServerError)

		// Seed a credential for user-scoped login success
		credLogin := models.WebAuthnCredential{
			UserID:       testUser.ID,
			CredentialID: []byte("login-cred-id"),
			PublicKey:    []byte("login-public-key"),
			FriendlyName: "Login Key",
		}
		if err := tx.Create(&credLogin).Error; err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}

		// BeginPasskeyLogin (user-scoped with credentials returns 200)
		wLoginBeginUser := httptest.NewRecorder()
		cLoginBeginUser, _ := gin.CreateTestContext(wLoginBeginUser)
		cLoginBeginUser.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{"identifier":"authtest@example.com"}`))
		cLoginBeginUser.Request.Header.Set("Content-Type", "application/json")
		srv.BeginPasskeyLogin(cLoginBeginUser)
		assertResponseCode(t, wLoginBeginUser, http.StatusOK)

		// BeginPasskeyLogin (user-scoped non-existent)
		wLoginBeginGhost := httptest.NewRecorder()
		cLoginBeginGhost, _ := gin.CreateTestContext(wLoginBeginGhost)
		cLoginBeginGhost.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{"identifier":"ghostuser"}`))
		cLoginBeginGhost.Request.Header.Set("Content-Type", "application/json")
		srv.BeginPasskeyLogin(cLoginBeginGhost)
		assertResponseCode(t, wLoginBeginGhost, http.StatusUnauthorized)

		// FinishPasskeyLogin missing session
		wLoginFin400 := httptest.NewRecorder()
		cLoginFin400, _ := gin.CreateTestContext(wLoginFin400)
		cLoginFin400.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish?session_id=nonexistent", nil)
		srv.FinishPasskeyLogin(cLoginFin400)
		assertResponseCode(t, wLoginFin400, http.StatusBadRequest)

		// FinishPasskeyLogin user not found in session
		fakeLoginSid := "fake-login-session-1"
		srv.putSession(fakeLoginSid, &webauthn.SessionData{UserID: []byte{255, 255, 0, 0, 0, 0, 0, 0}})
		wLoginFinGhost := httptest.NewRecorder()
		cLoginFinGhost, _ := gin.CreateTestContext(wLoginFinGhost)
		cLoginFinGhost.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/auth/passkey/login/finish?session_id=%s", fakeLoginSid), strings.NewReader("{}"))
		srv.FinishPasskeyLogin(cLoginFinGhost)
		assertResponseCode(t, wLoginFinGhost, http.StatusUnauthorized)

		// FinishPasskeyLogin user found but invalid payload
		fakeLoginSid2 := "fake-login-session-2"
		uidBytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			uidBytes[i] = byte(testUser.ID >> (8 * i))
		}
		srv.putSession(fakeLoginSid2, &webauthn.SessionData{UserID: uidBytes})
		wLoginFinBad := httptest.NewRecorder()
		cLoginFinBad, _ := gin.CreateTestContext(wLoginFinBad)
		cLoginFinBad.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/auth/passkey/login/finish?session_id=%s", fakeLoginSid2), strings.NewReader("{}"))
		srv.FinishPasskeyLogin(cLoginFinBad)
		assertResponseCode(t, wLoginFinBad, http.StatusUnauthorized)
	})
}
