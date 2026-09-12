package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
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
	srv := &Server{
		DB:     tx,
		Cfg:    cfg,
		Auth:   authSvc,
		Mailer: m,
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
		body, _ := json.Marshal(models.LoginRequest{
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
		body, _ := json.Marshal(models.LoginRequest{
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

	t.Run("Login wrong password", func(t *testing.T) {
		body, _ := json.Marshal(models.LoginRequest{
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
		body, _ := json.Marshal(models.LoginRequest{
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
		body, _ := json.Marshal(models.LoginRequest{
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
		body, _ := json.Marshal(models.ForgotRequest{
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
		body, _ := json.Marshal(models.ForgotRequest{
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
		shortPassPayload, _ := json.Marshal(models.ResetRequest{
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
		validPayload, _ := json.Marshal(models.ResetRequest{
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
	})
}
