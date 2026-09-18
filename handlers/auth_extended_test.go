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
	"github.com/google/uuid"

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

	t.Run("Login success with weaker Argon2id hash triggers rehash", func(t *testing.T) {
		weakerPass := "WeakerPassword123!"
		weakHasher := auth.NewArgon2idHasher(auth.Argon2idParams{
			Memory:      32 * 1024,
			Iterations:  2,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
		})
		wHash, err := weakHasher.Hash(weakerPass)
		if err != nil {
			t.Fatalf("weakHasher error: %v", err)
		}
		weakerUser := models.User{
			Username:     "weakeruser",
			Email:        "weaker@example.com",
			Name:         "Weaker User",
			PasswordHash: wHash,
			Role:         models.RoleUser,
			IsActive:     true,
		}
		if err := tx.Create(&weakerUser).Error; err != nil {
			t.Fatalf("failed to create weaker user: %v", err)
		}

		body, _ := json.Marshal(request.LoginRequest{
			Identifier: "weakeruser",
			Password:   weakerPass,
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusOK)

		var updated models.User
		_ = tx.Where(queryID, weakerUser.ID).First(&updated)
		if !strings.HasPrefix(updated.PasswordHash, "$argon2id$") {
			t.Errorf("expected upgraded Argon2id hash, got: %s", updated.PasswordHash)
		}
		if auth.NeedsRehash(updated.PasswordHash) {
			t.Errorf("upgraded hash should not need rehash anymore: %s", updated.PasswordHash)
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

		// Reset with token for deleted/inactive user
		ghostUser := models.User{
			Username: "ghostresetuser",
			Email:    "ghostreset@example.com",
			IsActive: false,
		}
		_ = tx.Create(&ghostUser)
		_ = tx.Delete(&ghostUser) // soft delete

		rawGhost := "ghost-raw-token"
		tokGhost := models.PasswordResetToken{
			UserID:    ghostUser.ID,
			TokenType: "password_reset",
			TokenHash: auth.HashToken(rawGhost),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		_ = tx.Create(&tokGhost)

		ghostPayload, _ := json.Marshal(request.ResetRequest{
			Token:    rawGhost,
			Password: "ValidPassword123!",
		})
		wGhost := httptest.NewRecorder()
		cGhost, _ := gin.CreateTestContext(wGhost)
		cGhost.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(ghostPayload))
		cGhost.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(cGhost)
		assertResponseCode(t, wGhost, http.StatusBadRequest)
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

func TestRefreshToken_RotationAndReuseDetection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 15*time.Minute)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	rawPass := "MyStr0ngPassw0rd!2026"
	hash, _ := auth.HashPassword(rawPass)
	testUser := models.User{
		Username:     "rotatetestuser",
		Email:        "rotate@example.com",
		Name:         "Rotate Test User",
		PasswordHash: hash,
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := tx.Create(&testUser).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Step 1: Login to acquire access token and refresh token
	loginBody, _ := json.Marshal(request.LoginRequest{
		Identifier: "rotatetestuser",
		Password:   rawPass,
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.Login(c)
	assertResponseCode(t, w, http.StatusOK)

	var loginResp struct {
		Data struct {
			Token        string `json:"token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	initialRefreshToken := loginResp.Data.RefreshToken
	if initialRefreshToken == "" {
		t.Fatalf("expected refresh token in login response, got empty")
	}

	// Step 2: Rotate refresh token
	refreshBody, _ := json.Marshal(request.RefreshRequest{
		RefreshToken: initialRefreshToken,
	})
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(refreshBody))
	c2.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(c2)
	assertResponseCode(t, w2, http.StatusOK)

	var refreshResp struct {
		Data struct {
			Token        string `json:"token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &refreshResp)
	secondRefreshToken := refreshResp.Data.RefreshToken
	if secondRefreshToken == "" || secondRefreshToken == initialRefreshToken {
		t.Fatalf("expected new distinct refresh token on rotation, got: %s", secondRefreshToken)
	}

	// Step 3: Reuse Detection! Attempting to reuse initialRefreshToken (which was already revoked upon rotation)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(refreshBody))
	c3.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(c3)
	assertResponseCode(t, w3, http.StatusUnauthorized)

	// Step 4: Verify family revocation! Since reuse was detected, the secondRefreshToken (in the same family) should also be revoked!
	reuseCheckBody, _ := json.Marshal(request.RefreshRequest{
		RefreshToken: secondRefreshToken,
	})
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(reuseCheckBody))
	c4.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(c4)
	assertResponseCode(t, w4, http.StatusUnauthorized)
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 15*time.Minute)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	rawPass := "MyStr0ngPassw0rd!2026"
	hash, _ := auth.HashPassword(rawPass)
	testUser := models.User{
		Username:     "logouttestuser",
		Email:        "logout@example.com",
		Name:         "Logout Test User",
		PasswordHash: hash,
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := tx.Create(&testUser).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// 1. Login
	loginBody, _ := json.Marshal(request.LoginRequest{
		Identifier: "logouttestuser",
		Password:   rawPass,
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.Login(c)
	assertResponseCode(t, w, http.StatusOK)

	var loginResp struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	rfToken := loginResp.Data.RefreshToken

	// 2. Logout with refresh token
	logoutBody, _ := json.Marshal(request.LogoutRequest{
		RefreshToken: rfToken,
	})
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(logoutBody))
	c2.Request.Header.Set("Content-Type", "application/json")
	srv.Logout(c2)
	assertResponseCode(t, w2, http.StatusOK)

	// 3. Trying to refresh should fail
	refreshBody, _ := json.Marshal(request.RefreshRequest{
		RefreshToken: rfToken,
	})
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(refreshBody))
	c3.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(c3)
	assertResponseCode(t, w3, http.StatusUnauthorized)
}

func TestAuthMiddleware_ImmediateRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 15*time.Minute)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	user := models.User{
		Username: "midrevoketest",
		Email:    "midrevoke@example.com",
		Name:     "Mid Revoke User",
		Role:     models.RoleUser,
		IsActive: true,
	}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Issue token
	token, err := authSvc.GenerateToken(&user)
	if err != nil {
		t.Fatalf("token error: %v", err)
	}

	router := gin.New()
	router.Use(srv.AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Request while active: 200 OK
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assertResponseCode(t, w, http.StatusOK)

	// Deactivate user in database
	if err := tx.Model(&user).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate user error: %v", err)
	}

	// Immediate Revocation: Same token, but user is now inactive: 401 Unauthorized
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w2, req2)
	assertResponseCode(t, w2, http.StatusUnauthorized)
}

func TestRefreshToken_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 15*time.Minute)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	user := models.User{
		Username:     "rfedgeuser",
		Email:        "rfedge@example.com",
		Name:         "RF Edge User",
		PasswordHash: "dummyhash",
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Non-existent refresh token
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"nonexistenttoken"}`))
	c1.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(c1)
	assertResponseCode(t, w1, http.StatusUnauthorized)

	// 2. Expired refresh token
	_, expHash, _ := auth.GenerateRefreshToken()
	expToken := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: expHash,
		FamilyID:  uuid.New().String(),
		ExpiresAt: time.Now().Add(-2 * time.Hour),
	}
	_ = srv.getTokenRepository().CreateRefreshToken(c1.Request.Context(), &expToken)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"`+expHash+`"}`))
	c2.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(c2)
	// Because hash lookup will find it by hash, let's look up using the hash directly:
	// wait, auth.HashToken(raw) is computed on input. So if input raw was expHash, the stored hash must match auth.HashToken(raw).
	rawExpToken, hashedExpToken, _ := auth.GenerateRefreshToken()
	expToken2 := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashedExpToken,
		FamilyID:  uuid.New().String(),
		ExpiresAt: time.Now().Add(-2 * time.Hour),
	}
	_ = srv.getTokenRepository().CreateRefreshToken(c2.Request.Context(), &expToken2)
	wExp := httptest.NewRecorder()
	cExp, _ := gin.CreateTestContext(wExp)
	cExp.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"`+rawExpToken+`"}`))
	cExp.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(cExp)
	assertResponseCode(t, wExp, http.StatusUnauthorized)

	// 3. Refresh token for deactivated user
	_ = tx.Model(&user).Update("is_active", false)
	rawValid, hashValid, _ := auth.GenerateRefreshToken()
	validToken := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashValid,
		FamilyID:  uuid.New().String(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	_ = srv.getTokenRepository().CreateRefreshToken(c2.Request.Context(), &validToken)
	wDeact := httptest.NewRecorder()
	cDeact, _ := gin.CreateTestContext(wDeact)
	cDeact.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"`+rawValid+`"}`))
	cDeact.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(cDeact)
	assertResponseCode(t, wDeact, http.StatusUnauthorized)

	// 4. Successful refresh token rotation on active user
	_ = tx.Model(&user).Update("is_active", true)
	rawGood, hashGood, _ := auth.GenerateRefreshToken()
	goodToken := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashGood,
		FamilyID:  uuid.New().String(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	_ = srv.getTokenRepository().CreateRefreshToken(c2.Request.Context(), &goodToken)
	wGood := httptest.NewRecorder()
	cGood, _ := gin.CreateTestContext(wGood)
	cGood.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"refresh_token":"`+rawGood+`"}`))
	cGood.Request.Header.Set("Content-Type", "application/json")
	srv.RefreshToken(cGood)
	assertResponseCode(t, wGood, http.StatusOK)
}

func TestPasskeyManagement_FullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 15*time.Minute)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	user := models.User{
		Username: "pkflowuser",
		Email:    "pkflow@example.com",
		Name:     "PK Flow User",
		Role:     models.RoleUser,
		IsActive: true,
	}
	_ = tx.Create(&user)

	// 1. ListPasskeys initially empty
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Set("userID", user.ID)
	c1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/passkeys", nil)
	srv.ListPasskeys(c1)
	assertResponseCode(t, w1, http.StatusOK)

	// 2. Add a passkey to DB
	cred := models.WebAuthnCredential{
		UserID:          user.ID,
		CredentialID:    []byte("cred-test-id-999"),
		PublicKey:       []byte("public-key-bytes"),
		AttestationType: "none",
		AAGUID:          []byte("00000000-0000-0000-0000-000000000000"),
		SignCount:       1,
		FriendlyName:    "Office Key",
	}
	_ = tx.Create(&cred)

	// 3. ListPasskeys now returns 1
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Set("userID", user.ID)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/passkeys", nil)
	srv.ListPasskeys(c2)
	assertResponseCode(t, w2, http.StatusOK)

	// 4. AdminListPasskeys
	wAdmin := httptest.NewRecorder()
	cAdmin, _ := gin.CreateTestContext(wAdmin)
	cAdmin.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", user.ID)}}
	cAdmin.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d/passkeys", user.ID), nil)
	srv.AdminListPasskeys(cAdmin)
	assertResponseCode(t, wAdmin, http.StatusOK)

	// 5. DeletePasskey not found
	wDelNotFound := httptest.NewRecorder()
	cDelNotFound, _ := gin.CreateTestContext(wDelNotFound)
	cDelNotFound.Set("userID", user.ID)
	cDelNotFound.Params = gin.Params{{Key: "id", Value: "999999"}}
	cDelNotFound.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/passkeys/999999", nil)
	srv.DeletePasskey(cDelNotFound)
	assertResponseCode(t, wDelNotFound, http.StatusNotFound)

	// 6. DeletePasskey success
	wDelSuccess := httptest.NewRecorder()
	cDelSuccess, _ := gin.CreateTestContext(wDelSuccess)
	cDelSuccess.Set("userID", user.ID)
	cDelSuccess.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", cred.ID)}}
	cDelSuccess.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), nil)
	srv.DeletePasskey(cDelSuccess)
	assertResponseCode(t, wDelSuccess, http.StatusOK)

	// 7. AdminDeletePasskey
	credAdmin := models.WebAuthnCredential{
		UserID:          user.ID,
		CredentialID:    []byte("cred-test-id-admin"),
		PublicKey:       []byte("public-key-bytes-2"),
		AttestationType: "none",
		AAGUID:          []byte("00000000-0000-0000-0000-000000000000"),
		SignCount:       1,
		FriendlyName:    "Admin Test Key",
	}
	_ = tx.Create(&credAdmin)

	wAdminDel := httptest.NewRecorder()
	cAdminDel, _ := gin.CreateTestContext(wAdminDel)
	cAdminDel.Params = gin.Params{
		{Key: "id", Value: fmt.Sprintf("%d", user.ID)},
		{Key: "pid", Value: fmt.Sprintf("%d", credAdmin.ID)},
	}
	cAdminDel.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d/passkeys/%d", user.ID, credAdmin.ID), nil)
	srv.AdminDeletePasskey(cAdminDel)
	assertResponseCode(t, wAdminDel, http.StatusOK)

	// AdminDeletePasskey not found
	wAdminDel404 := httptest.NewRecorder()
	cAdminDel404, _ := gin.CreateTestContext(wAdminDel404)
	cAdminDel404.Params = gin.Params{
		{Key: "id", Value: fmt.Sprintf("%d", user.ID)},
		{Key: "pid", Value: "999999"},
	}
	cAdminDel404.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d/passkeys/999999", user.ID), nil)
	srv.AdminDeletePasskey(cAdminDel404)
	assertResponseCode(t, wAdminDel404, http.StatusNotFound)

	// 8. BeginPasskeyRegistration for authenticated user
	wReg := httptest.NewRecorder()
	cReg, _ := gin.CreateTestContext(wReg)
	cReg.Set(ctxUserID, user.ID)
	cReg.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/begin", nil)
	srv.BeginPasskeyRegistration(cReg)
	assertResponseCode(t, wReg, http.StatusOK)

	// 9. BeginPasskeyLogin (discoverable)
	wDisc := httptest.NewRecorder()
	cDisc, _ := gin.CreateTestContext(wDisc)
	cDisc.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{}`))
	cDisc.Request.Header.Set("Content-Type", "application/json")
	srv.BeginPasskeyLogin(cDisc)
	assertResponseCode(t, wDisc, http.StatusOK)

	// 10. BeginPasskeyLogin (with valid identifier)
	credLogin := models.WebAuthnCredential{
		UserID:          user.ID,
		CredentialID:    []byte("cred-test-id-for-login"),
		PublicKey:       []byte("public-key-bytes-login"),
		AttestationType: "none",
		AAGUID:          []byte("00000000-0000-0000-0000-000000000000"),
		SignCount:       1,
		FriendlyName:    "Login Test Key",
	}
	_ = tx.Create(&credLogin)

	wUserLogin := httptest.NewRecorder()
	cUserLogin, _ := gin.CreateTestContext(wUserLogin)
	cUserLogin.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{"identifier":"`+user.Username+`"}`))
	cUserLogin.Request.Header.Set("Content-Type", "application/json")
	srv.BeginPasskeyLogin(cUserLogin)
	assertResponseCode(t, wUserLogin, http.StatusOK)

	// 10b. BeginPasskeyLogin (with unknown identifier)
	wUserUnknown := httptest.NewRecorder()
	cUserUnknown, _ := gin.CreateTestContext(wUserUnknown)
	cUserUnknown.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{"identifier":"unknown-user"}`))
	cUserUnknown.Request.Header.Set("Content-Type", "application/json")
	srv.BeginPasskeyLogin(cUserUnknown)
	assertResponseCode(t, wUserUnknown, http.StatusUnauthorized)

	// 11. WebAuthnRelatedOrigins
	wRel := httptest.NewRecorder()
	cRel, _ := gin.CreateTestContext(wRel)
	cRel.Request = httptest.NewRequest(http.MethodGet, "/.well-known/webauthn", nil)
	srv.WebAuthnRelatedOrigins(cRel)
	assertResponseCode(t, wRel, http.StatusOK)
}

func TestMe_Endpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 15*time.Minute)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	user := models.User{
		Username: "meuser",
		Email:    "meuser@example.com",
		Name:     "Me User",
		Role:     models.RoleUser,
		IsActive: true,
	}
	_ = tx.Create(&user)

	// Me success
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", user.ID)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	srv.Me(c)
	assertResponseCode(t, w, http.StatusOK)

	// Me not found
	w404 := httptest.NewRecorder()
	c404, _ := gin.CreateTestContext(w404)
	c404.Set("userID", uint(999999))
	c404.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	srv.Me(c404)
	assertResponseCode(t, w404, http.StatusNotFound)
}
