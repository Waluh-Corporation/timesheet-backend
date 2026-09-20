package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/repository"
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

		// Reset with valid token but weak password (< 12 chars, violates NIST policy)
		rawWeak := "weak-token-xyz"
		tokWeak := models.PasswordResetToken{
			UserID:    testUser.ID,
			TokenType: "password_reset",
			TokenHash: auth.HashToken(rawWeak),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		_ = tx.Create(&tokWeak)

		weakPayload, _ := json.Marshal(request.ResetRequest{
			Token:    rawWeak,
			Password: "short",
		})
		wWeak := httptest.NewRecorder()
		cWeak, _ := gin.CreateTestContext(wWeak)
		cWeak.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(weakPayload))
		cWeak.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(cWeak)
		assertResponseCode(t, wWeak, http.StatusBadRequest)
	})

	t.Run("VerifyResetPasswordToken full lifecycle", func(t *testing.T) {
		// 1. Missing token
		wMissing := httptest.NewRecorder()
		cMissing, _ := gin.CreateTestContext(wMissing)
		cMissing.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify", nil)
		srv.VerifyResetPasswordToken(cMissing)
		assertResponseCode(t, wMissing, http.StatusBadRequest)

		// 2. Invalid / nonexistent token
		wInvalid := httptest.NewRecorder()
		cInvalid, _ := gin.CreateTestContext(wInvalid)
		cInvalid.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify?token=nonexistent-token-12345", nil)
		srv.VerifyResetPasswordToken(cInvalid)
		assertResponseCode(t, wInvalid, http.StatusBadRequest)

		var respInvalid response.VerifyResetTokenResponse
		_ = json.Unmarshal(wInvalid.Body.Bytes(), &respInvalid)
		if respInvalid.Valid || respInvalid.Status != "invalid" {
			t.Errorf("expected invalid status, got %+v", respInvalid)
		}

		// 3. Valid token (GET)
		rawTok, hashTok, _ := auth.GenerateResetToken()
		validTok := models.PasswordResetToken{
			UserID:    testUser.ID,
			TokenType: "password_reset",
			TokenHash: hashTok,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		if err := tx.Create(&validTok).Error; err != nil {
			t.Fatalf("failed to create reset token: %v", err)
		}

		wGetValid := httptest.NewRecorder()
		cGetValid, _ := gin.CreateTestContext(wGetValid)
		cGetValid.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify?token="+rawTok, nil)
		srv.VerifyResetPasswordToken(cGetValid)
		assertResponseCode(t, wGetValid, http.StatusOK)

		var respGetValid response.VerifyResetTokenResponse
		_ = json.Unmarshal(wGetValid.Body.Bytes(), &respGetValid)
		if !respGetValid.Valid || respGetValid.Status != "valid" || respGetValid.Username != testUser.Username {
			t.Errorf("expected valid token response, got %+v", respGetValid)
		}

		// 4. Valid token (POST body)
		postPayload, _ := json.Marshal(request.VerifyResetTokenRequest{Token: rawTok})
		wPostValid := httptest.NewRecorder()
		cPostValid, _ := gin.CreateTestContext(wPostValid)
		cPostValid.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password/verify", bytes.NewReader(postPayload))
		cPostValid.Request.Header.Set("Content-Type", "application/json")
		srv.VerifyResetPasswordToken(cPostValid)
		assertResponseCode(t, wPostValid, http.StatusOK)

		// 5. Consumed / Already used token
		now := time.Now()
		validTok.UsedAt = &now
		validTok.UsedIP = "127.0.0.1"
		_ = tx.Save(&validTok)

		wUsed := httptest.NewRecorder()
		cUsed, _ := gin.CreateTestContext(wUsed)
		cUsed.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify?token="+rawTok, nil)
		srv.VerifyResetPasswordToken(cUsed)
		assertResponseCode(t, wUsed, http.StatusBadRequest)

		var respUsed response.VerifyResetTokenResponse
		_ = json.Unmarshal(wUsed.Body.Bytes(), &respUsed)
		if respUsed.Valid || respUsed.Status != "already_used" {
			t.Errorf("expected already_used status, got %+v", respUsed)
		}

		// 6. Expired token
		rawExp, hashExp, _ := auth.GenerateResetToken()
		expTok := models.PasswordResetToken{
			UserID:    testUser.ID,
			TokenType: "password_reset",
			TokenHash: hashExp,
			ExpiresAt: time.Now().Add(-10 * time.Minute),
		}
		_ = tx.Create(&expTok)

		wExp := httptest.NewRecorder()
		cExp, _ := gin.CreateTestContext(wExp)
		cExp.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify?token="+rawExp, nil)
		srv.VerifyResetPasswordToken(cExp)
		assertResponseCode(t, wExp, http.StatusBadRequest)

		var respExp response.VerifyResetTokenResponse
		_ = json.Unmarshal(wExp.Body.Bytes(), &respExp)
		if respExp.Valid || respExp.Status != "expired" {
			t.Errorf("expected expired status, got %+v", respExp)
		}

		// 7. Token for inactive user
		rawInactive, hashInactive, _ := auth.GenerateResetToken()
		disUser := models.User{
			Username: fmt.Sprintf("inact_%d", time.Now().UnixNano()),
			Email:    fmt.Sprintf("inact_%d@example.com", time.Now().UnixNano()),
			IsActive: false,
		}
		_ = tx.Create(&disUser)
		_ = tx.Model(&disUser).Update("is_active", false)
		tokInactive := models.PasswordResetToken{
			UserID:    disUser.ID,
			TokenType: "password_reset",
			TokenHash: hashInactive,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		_ = tx.Create(&tokInactive)

		wInactive := httptest.NewRecorder()
		cInactive, _ := gin.CreateTestContext(wInactive)
		cInactive.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify?token="+rawInactive, nil)
		srv.VerifyResetPasswordToken(cInactive)
		assertResponseCode(t, wInactive, http.StatusBadRequest)
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

		// FinishPasskeyLogin with non-existent user handle
		var nonexistentUIDBytes [8]byte
		binary.LittleEndian.PutUint64(nonexistentUIDBytes[:], 99999999)
		srv.putSession("test-nonexistent-user-sid", &webauthn.SessionData{UserID: nonexistentUIDBytes[:]})
		wNonexistent := httptest.NewRecorder()
		cNonexistent, _ := gin.CreateTestContext(wNonexistent)
		cNonexistent.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish?session_id=test-nonexistent-user-sid", nil)
		srv.FinishPasskeyLogin(cNonexistent)
		assertResponseCode(t, wNonexistent, http.StatusUnauthorized)

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

		// FinishPasskeyRegistration error branches
		wFinishNoSess := httptest.NewRecorder()
		cFinishNoSess, _ := gin.CreateTestContext(wFinishNoSess)
		cFinishNoSess.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish", nil)
		cFinishNoSess.Set(ctxUserID, testUser.ID)
		srv.FinishPasskeyRegistration(cFinishNoSess)
		assertResponseCode(t, wFinishNoSess, http.StatusBadRequest)

		wFinishExp := httptest.NewRecorder()
		cFinishExp, _ := gin.CreateTestContext(wFinishExp)
		cFinishExp.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=expired-id", nil)
		cFinishExp.Set(ctxUserID, testUser.ID)
		srv.FinishPasskeyRegistration(cFinishExp)
		assertResponseCode(t, wFinishExp, http.StatusBadRequest)

		srv.putSession("test-session-user-missing", &webauthn.SessionData{})
		wFinishNoUser := httptest.NewRecorder()
		cFinishNoUser, _ := gin.CreateTestContext(wFinishNoUser)
		cFinishNoUser.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=test-session-user-missing", nil)
		cFinishNoUser.Set(ctxUserID, uint(9999999))
		srv.FinishPasskeyRegistration(cFinishNoUser)
		assertResponseCode(t, wFinishNoUser, http.StatusNotFound)

		srv.putSession("test-session-bad-body", &webauthn.SessionData{})
		wFinishBadBody := httptest.NewRecorder()
		cFinishBadBody, _ := gin.CreateTestContext(wFinishBadBody)
		cFinishBadBody.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=test-session-bad-body", strings.NewReader(`{}`))
		cFinishBadBody.Set(ctxUserID, testUser.ID)
		srv.FinishPasskeyRegistration(cFinishBadBody)
		assertResponseCode(t, wFinishBadBody, http.StatusBadRequest)

		// UpdatePasskey (invalid ID)
		wUpdBad := httptest.NewRecorder()
		cUpdBad, _ := gin.CreateTestContext(wUpdBad)
		cUpdBad.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/passkeys/not-an-id", strings.NewReader(`{"name":"test"}`))
		cUpdBad.Params = gin.Params{{Key: "id", Value: "not-an-id"}}
		cUpdBad.Set(ctxUserID, testUser.ID)
		srv.UpdatePasskey(cUpdBad)
		assertResponseCode(t, wUpdBad, http.StatusBadRequest)

		// UpdatePasskey (invalid JSON body)
		wUpdBadJSON := httptest.NewRecorder()
		cUpdBadJSON, _ := gin.CreateTestContext(wUpdBadJSON)
		cUpdBadJSON.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), strings.NewReader(`{invalid`))
		cUpdBadJSON.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred.ID)}}
		cUpdBadJSON.Set(ctxUserID, testUser.ID)
		srv.UpdatePasskey(cUpdBadJSON)
		assertResponseCode(t, wUpdBadJSON, http.StatusBadRequest)

		// UpdatePasskey (nil repo)
		nilRepoSrv := &Server{}
		wUpdNilRepo := httptest.NewRecorder()
		cUpdNilRepo, _ := gin.CreateTestContext(wUpdNilRepo)
		cUpdNilRepo.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), strings.NewReader(`{"name":"key"}`))
		cUpdNilRepo.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred.ID)}}
		cUpdNilRepo.Set(ctxUserID, testUser.ID)
		nilRepoSrv.UpdatePasskey(cUpdNilRepo)
		assertResponseCode(t, wUpdNilRepo, http.StatusInternalServerError)

		// UpdatePasskey (empty name)
		wUpdEmpty := httptest.NewRecorder()
		cUpdEmpty, _ := gin.CreateTestContext(wUpdEmpty)
		cUpdEmpty.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), strings.NewReader(`{"name":"   "}`))
		cUpdEmpty.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred.ID)}}
		cUpdEmpty.Set(ctxUserID, testUser.ID)
		srv.UpdatePasskey(cUpdEmpty)
		assertResponseCode(t, wUpdEmpty, http.StatusBadRequest)

		// UpdatePasskey (success)
		wUpd := httptest.NewRecorder()
		cUpd, _ := gin.CreateTestContext(wUpd)
		cUpd.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), strings.NewReader(`{"name":"Updated Self Key"}`))
		cUpd.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred.ID)}}
		cUpd.Set(ctxUserID, testUser.ID)
		srv.UpdatePasskey(cUpd)
		assertResponseCode(t, wUpd, http.StatusOK)

		// UpdatePasskey with other user (IDOR protection -> 404)
		wUpdIDOR := httptest.NewRecorder()
		cUpdIDOR, _ := gin.CreateTestContext(wUpdIDOR)
		cUpdIDOR.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/passkeys/%d", cred.ID), strings.NewReader(`{"name":"Hacked Key"}`))
		cUpdIDOR.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred.ID)}}
		cUpdIDOR.Set(ctxUserID, testUser.ID+999)
		srv.UpdatePasskey(cUpdIDOR)
		assertResponseCode(t, wUpdIDOR, http.StatusNotFound)

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

		// AdminUpdatePasskey (strictly forbidden: passkeys are user-managed)
		wAdminUpd := httptest.NewRecorder()
		cAdminUpd, _ := gin.CreateTestContext(wAdminUpd)
		cAdminUpd.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/passkeys/%d", testUser.ID, cred2.ID), strings.NewReader(`{"name":"Admin Renamed Key"}`))
		cAdminUpd.Params = gin.Params{{Key: "id", Value: fmt.Sprint(testUser.ID)}, {Key: "pid", Value: fmt.Sprint(cred2.ID)}}
		srv.AdminUpdatePasskey(cAdminUpd)
		assertResponseCode(t, wAdminUpd, http.StatusForbidden)

		// AdminDeletePasskey (strictly forbidden: passkeys are user-managed)
		wAdminDel := httptest.NewRecorder()
		cAdminDel, _ := gin.CreateTestContext(wAdminDel)
		cAdminDel.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d/passkeys/%d", testUser.ID, cred2.ID), nil)
		cAdminDel.Params = gin.Params{
			{Key: "id", Value: fmt.Sprint(testUser.ID)},
			{Key: "pid", Value: fmt.Sprint(cred2.ID)},
		}
		srv.AdminDeletePasskey(cAdminDel)
		assertResponseCode(t, wAdminDel, http.StatusForbidden)

		// Admin user self-service update their OWN passkey via /passkeys/:id -> OK
		adminUser := models.User{
			Username:     "admin_self",
			Email:        "admin_self@example.com",
			Role:         models.RoleAdmin,
			IsActive:     true,
			PasswordHash: "dummy",
		}
		if err := tx.Create(&adminUser).Error; err != nil {
			t.Fatalf("failed to create adminUser: %v", err)
		}
		adminCred := models.WebAuthnCredential{
			UserID:       adminUser.ID,
			CredentialID: []byte("admin-cred-id"),
			PublicKey:    []byte("admin-public-key"),
			FriendlyName: "Admin Key",
		}
		if err := tx.Create(&adminCred).Error; err != nil {
			t.Fatalf("failed to create adminCred: %v", err)
		}

		wAdminSelfUpd := httptest.NewRecorder()
		cAdminSelfUpd, _ := gin.CreateTestContext(wAdminSelfUpd)
		cAdminSelfUpd.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/passkeys/%d", adminCred.ID), strings.NewReader(`{"name":"Admin Renamed Self Key"}`))
		cAdminSelfUpd.Params = gin.Params{{Key: "id", Value: fmt.Sprint(adminCred.ID)}}
		cAdminSelfUpd.Set(ctxUserID, adminUser.ID)
		cAdminSelfUpd.Set(ctxRole, models.RoleAdmin)
		srv.UpdatePasskey(cAdminSelfUpd)
		assertResponseCode(t, wAdminSelfUpd, http.StatusOK)

		// Admin user trying to update a regular user's passkey via /passkeys/:id -> 404 (IDOR protected)
		wAdminTamperUpd := httptest.NewRecorder()
		cAdminTamperUpd, _ := gin.CreateTestContext(wAdminTamperUpd)
		cAdminTamperUpd.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/passkeys/%d", cred2.ID), strings.NewReader(`{"name":"Admin Tampered Key"}`))
		cAdminTamperUpd.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred2.ID)}}
		cAdminTamperUpd.Set(ctxUserID, adminUser.ID)
		cAdminTamperUpd.Set(ctxRole, models.RoleAdmin)
		srv.UpdatePasskey(cAdminTamperUpd)
		assertResponseCode(t, wAdminTamperUpd, http.StatusNotFound)

		// Admin user trying to delete a regular user's passkey via /passkeys/:id -> 404 (IDOR protected)
		wAdminTamperDel := httptest.NewRecorder()
		cAdminTamperDel, _ := gin.CreateTestContext(wAdminTamperDel)
		cAdminTamperDel.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/passkeys/%d", cred2.ID), nil)
		cAdminTamperDel.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred2.ID)}}
		cAdminTamperDel.Set(ctxUserID, adminUser.ID)
		cAdminTamperDel.Set(ctxRole, models.RoleAdmin)
		srv.DeletePasskey(cAdminTamperDel)
		assertResponseCode(t, wAdminTamperDel, http.StatusNotFound)

		// Delete cred2 using self-service DeletePasskey by owner (testUser) -> OK
		wDelCred2 := httptest.NewRecorder()
		cDelCred2, _ := gin.CreateTestContext(wDelCred2)
		cDelCred2.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/passkeys/%d", cred2.ID), nil)
		cDelCred2.Params = gin.Params{{Key: "id", Value: fmt.Sprint(cred2.ID)}}
		cDelCred2.Set(ctxUserID, testUser.ID)
		srv.DeletePasskey(cDelCred2)
		assertResponseCode(t, wDelCred2, http.StatusOK)

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

	// Admin cannot delete user passkeys by policy (403 Forbidden)
	wAdminDel := httptest.NewRecorder()
	cAdminDel, _ := gin.CreateTestContext(wAdminDel)
	cAdminDel.Params = gin.Params{
		{Key: "id", Value: fmt.Sprintf("%d", user.ID)},
		{Key: "pid", Value: fmt.Sprintf("%d", credAdmin.ID)},
	}
	cAdminDel.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d/passkeys/%d", user.ID, credAdmin.ID), nil)
	srv.AdminDeletePasskey(cAdminDel)
	assertResponseCode(t, wAdminDel, http.StatusForbidden)

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

func TestAuthHandlers_RepoErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	testUser := models.User{
		Username: "erruser",
		Email:    "erruser@example.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	_ = tx.Create(&testUser)

	t.Run("Passkey repository error branches", func(t *testing.T) {
		errUserRepo := &testErrUserRepo{
			UserRepository: srv.UserRepo,
			err:            errors.New("db error"),
		}
		savedUserRepo := srv.UserRepo
		srv.UserRepo = errUserRepo
		defer func() { srv.UserRepo = savedUserRepo }()

		// ListPasskeys error
		wL := httptest.NewRecorder()
		cL, _ := gin.CreateTestContext(wL)
		cL.Set("user_id", testUser.ID)
		cL.Request = httptest.NewRequest(http.MethodGet, "/api/v1/passkeys", nil)
		srv.ListPasskeys(cL)
		assertResponseCode(t, wL, http.StatusInternalServerError)

		// DeletePasskey error
		wD := httptest.NewRecorder()
		cD, _ := gin.CreateTestContext(wD)
		cD.Set("user_id", testUser.ID)
		cD.Params = []gin.Param{{Key: "id", Value: "123"}}
		cD.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/passkeys/123", nil)
		srv.DeletePasskey(cD)
		assertResponseCode(t, wD, http.StatusInternalServerError)

		// AdminListPasskeys error
		wAL := httptest.NewRecorder()
		cAL, _ := gin.CreateTestContext(wAL)
		cAL.Params = []gin.Param{{Key: "id", Value: "123"}}
		cAL.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/123/passkeys", nil)
		srv.AdminListPasskeys(cAL)
		assertResponseCode(t, wAL, http.StatusInternalServerError)

		// AdminDeletePasskey is rejected by policy before repo is called (403 Forbidden)
		wAD := httptest.NewRecorder()
		cAD, _ := gin.CreateTestContext(wAD)
		cAD.Params = []gin.Param{{Key: "id", Value: "123"}, {Key: "pid", Value: "456"}}
		cAD.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/123/passkeys/456", nil)
		srv.AdminDeletePasskey(cAD)
		assertResponseCode(t, wAD, http.StatusForbidden)
	})

	t.Run("Token repository error branches", func(t *testing.T) {
		errTokRepo := &testErrTokenRepo{
			TokenRepository: srv.TokenRepo,
			err:             errors.New("db error"),
		}
		savedTokRepo := srv.TokenRepo
		srv.TokenRepo = errTokRepo
		defer func() { srv.TokenRepo = savedTokRepo }()

		// Refresh token error revoking
		rawTok := "seed-refresh-for-err"
		seedTok := models.RefreshToken{
			UserID:    testUser.ID,
			TokenHash: auth.HashToken(rawTok),
			FamilyID:  uuid.NewString(),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		_ = tx.Create(&seedTok)

		refreshPayload, _ := json.Marshal(request.RefreshRequest{
			RefreshToken: rawTok,
		})
		wRef := httptest.NewRecorder()
		cRef, _ := gin.CreateTestContext(wRef)
		cRef.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh-token", bytes.NewReader(refreshPayload))
		cRef.Request.Header.Set("Content-Type", "application/json")
		srv.RefreshToken(cRef)
		assertResponseCode(t, wRef, http.StatusInternalServerError)

		// Reset password error on update password
		rawReset := "reset-token-for-err"
		seedReset := models.PasswordResetToken{
			UserID:    testUser.ID,
			TokenType: "password_reset",
			TokenHash: auth.HashToken(rawReset),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		_ = tx.Create(&seedReset)

		errUserRepo := &testErrUserRepo{
			UserRepository: srv.UserRepo,
			err:            errors.New("db update password error"),
		}
		savedUserRepo := srv.UserRepo
		srv.UserRepo = errUserRepo
		defer func() { srv.UserRepo = savedUserRepo }()

		resetPayload, _ := json.Marshal(request.ResetRequest{
			Token:    rawReset,
			Password: "ValidPassword123!",
		})
		wReset := httptest.NewRecorder()
		cReset, _ := gin.CreateTestContext(wReset)
		cReset.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(resetPayload))
		cReset.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(cReset)
		assertResponseCode(t, wReset, http.StatusInternalServerError)
	})
}

type testErrUserRepo struct {
	repository.UserRepository
	err error
}

func (r *testErrUserRepo) ListPasskeysByUserID(ctx context.Context, userID uint) ([]models.WebAuthnCredential, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.UserRepository.ListPasskeysByUserID(ctx, userID)
}

func (r *testErrUserRepo) CreatePasskeyCredential(ctx context.Context, cred *models.WebAuthnCredential) error {
	if r.err != nil {
		return r.err
	}
	return r.UserRepository.CreatePasskeyCredential(ctx, cred)
}

func (r *testErrUserRepo) DeletePasskey(ctx context.Context, id uint, userID *uint) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	return r.UserRepository.DeletePasskey(ctx, id, userID)
}

func (r *testErrUserRepo) UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error {
	if r.err != nil {
		return r.err
	}
	return r.UserRepository.UpdatePassword(ctx, id, passwordHash, updatedAt)
}

type testErrTokenRepo struct {
	repository.TokenRepository
	err error
}

func (r *testErrTokenRepo) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if r.err != nil {
		return r.err
	}
	return r.TokenRepository.CreateRefreshToken(ctx, token)
}

func (r *testErrTokenRepo) RevokeRefreshToken(ctx context.Context, id uint, revokedAt time.Time) error {
	if r.err != nil {
		return r.err
	}
	return r.TokenRepository.RevokeRefreshToken(ctx, id, revokedAt)
}

func (r *testErrTokenRepo) ConsumeResetToken(ctx context.Context, tokenID uint, usedAt time.Time, usedIP string) error {
	if r.err != nil {
		return r.err
	}
	return r.TokenRepository.ConsumeResetToken(ctx, tokenID, usedAt, usedIP)
}

func TestFinishPasskeyRegistration_FullCoverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	testUser := &models.User{
		Username:     "test-passkey-user",
		Email:        "passkey-user@example.com",
		PasswordHash: "dummy-hash",
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := tx.Create(testUser).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	srv := &Server{
		DB:       tx,
		UserRepo: userRepo,
	}

	// 1. Success with response.transports
	srv.putSession("session-1", &webauthn.SessionData{})
	srv.finishRegistrationFunc = func(user models.User, session webauthn.SessionData, r *http.Request) (*webauthn.Credential, error) {
		return &webauthn.Credential{
			ID:        []byte("cred-id-1"),
			PublicKey: []byte("pubkey-1"),
			Authenticator: webauthn.Authenticator{
				AAGUID: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			},
		}, nil
	}

	body1 := `{"id":"cred-id-1","response":{"transports":["usb","nfc"]}}`
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=session-1&name=My%20Key", strings.NewReader(body1))
	c1.Set(ctxUserID, testUser.ID)
	srv.FinishPasskeyRegistration(c1)
	assertResponseCode(t, w1, http.StatusOK)

	// 2. Success with root transports & X-Passkey-Name header
	srv.putSession("session-2", &webauthn.SessionData{})
	srv.finishRegistrationFunc = func(user models.User, session webauthn.SessionData, r *http.Request) (*webauthn.Credential, error) {
		return &webauthn.Credential{
			ID:        []byte("cred-id-2"),
			PublicKey: []byte("pubkey-2"),
		}, nil
	}
	body2 := `{"id":"cred-id-2","transports":["internal"],"authenticatorAttachment":"platform","response":{}}`
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=session-2", strings.NewReader(body2))
	c2.Request.Header.Set("X-Passkey-Name", "My Header Name")
	c2.Set(ctxUserID, testUser.ID)
	srv.FinishPasskeyRegistration(c2)
	assertResponseCode(t, w2, http.StatusOK)

	// 3. Success with platform attachment fallback
	srv.putSession("session-3", &webauthn.SessionData{})
	srv.finishRegistrationFunc = func(user models.User, session webauthn.SessionData, r *http.Request) (*webauthn.Credential, error) {
		return &webauthn.Credential{
			ID:        []byte("cred-id-3"),
			PublicKey: []byte("pubkey-3"),
			Authenticator: webauthn.Authenticator{
				Attachment: protocol.Platform,
			},
		}, nil
	}
	body3 := `{"id":"cred-id-3","response":{}}`
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=session-3", strings.NewReader(body3))
	c3.Set(ctxUserID, testUser.ID)
	srv.FinishPasskeyRegistration(c3)
	assertResponseCode(t, w3, http.StatusOK)

	// 4. Failure saving credential (DB error)
	errRepo := &testErrUserRepo{
		UserRepository: userRepo,
		err:            errors.New("db save error"),
	}
	srvErr := &Server{
		DB:       tx,
		UserRepo: errRepo,
		finishRegistrationFunc: func(user models.User, session webauthn.SessionData, r *http.Request) (*webauthn.Credential, error) {
			return &webauthn.Credential{
				ID:        []byte("cred-id-4"),
				PublicKey: []byte("pubkey-4"),
			}, nil
		},
	}
	srvErr.putSession("session-4", &webauthn.SessionData{})
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=session-4", strings.NewReader(`{}`))
	c4.Set(ctxUserID, testUser.ID)
	srvErr.FinishPasskeyRegistration(c4)
	assertResponseCode(t, w4, http.StatusInternalServerError)
}

func TestResolvePasskeyTransports_EdgeCases(t *testing.T) {
	// nil credential
	resolvePasskeyTransports(nil, []byte(`{"transports":["usb"]}`))

	// empty body
	cred := &webauthn.Credential{}
	resolvePasskeyTransports(cred, nil)
	if len(cred.Transport) != 0 {
		t.Errorf("expected 0 transports, got %d", len(cred.Transport))
	}

	// invalid JSON
	resolvePasskeyTransports(cred, []byte(`{invalid-json`))
	if len(cred.Transport) != 0 {
		t.Errorf("expected 0 transports, got %d", len(cred.Transport))
	}

	// already populated transports
	credWithTransports := &webauthn.Credential{
		Transport: []protocol.AuthenticatorTransport{protocol.USB},
	}
	resolvePasskeyTransports(credWithTransports, []byte(`{"transports":["nfc"]}`))
	if len(credWithTransports.Transport) != 1 || credWithTransports.Transport[0] != protocol.USB {
		t.Errorf("expected transports to remain unchanged, got %v", credWithTransports.Transport)
	}
}
