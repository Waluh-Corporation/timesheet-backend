package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
)

func TestForgotPassword_CooldownAndInvalidationLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	m := mailer.New(cfg)
	srv, err := NewServer(tx, cfg, authSvc, m, nil)
	require.NoError(t, err)

	// Configure a short cooldown for testing (80ms)
	srv.Cfg.ResetPasswordCooldown = 80 * time.Millisecond
	srv.Cfg.ResetTokenTTL = 15 * time.Minute

	// Setup spy email sender to monitor email dispatches
	var emailCount atomic.Int32
	var lastLinkMu sync.Mutex
	var lastLink string

	srv.sendResetEmailFunc = func(toEmail, username, resetLink string) error {
		emailCount.Add(1)
		lastLinkMu.Lock()
		lastLink = resetLink
		lastLinkMu.Unlock()
		return nil
	}

	// Create test user
	rawPass := "InitialPassw0rd!2026"
	hash, err := auth.HashPassword(rawPass)
	require.NoError(t, err)

	testUser := models.User{
		Username:     "cooldown_user",
		Email:        "cooldown_user@example.com",
		Name:         "Cooldown Test User",
		PasswordHash: string(hash),
		Role:         models.RoleUser,
		IsActive:     true,
	}
	require.NoError(t, tx.Create(&testUser).Error)

	// =========================================================================
	// 1. Request 1: Initial Request -> Should generate Token 1 and dispatch email
	// =========================================================================
	reqBody1, _ := json.Marshal(request.ForgotRequest{Email: testUser.Email})
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(reqBody1))
	c1.Request.Header.Set("Content-Type", "application/json")
	c1.Request.RemoteAddr = "192.0.2.100:12345"

	srv.ForgotPassword(c1)
	require.Equal(t, http.StatusOK, w1.Code)

	var resp1 response.MessageResponse
	err = json.Unmarshal(w1.Body.Bytes(), &resp1)
	require.NoError(t, err)
	assert.Contains(t, resp1.Message, "if the email exists")

	// Wait briefly for asynchronous email dispatch
	require.Eventually(t, func() bool {
		return emailCount.Load() == 1
	}, 500*time.Millisecond, 10*time.Millisecond, "expected 1 email dispatched")

	lastLinkMu.Lock()
	link1 := lastLink
	lastLinkMu.Unlock()
	require.NotEmpty(t, link1)

	u1, err := url.Parse(link1)
	require.NoError(t, err)
	tokenRaw1 := u1.Query().Get("token")
	require.NotEmpty(t, tokenRaw1)

	// Check DB: exactly 1 token should exist, and used_at must be NULL
	var tokens1 []models.PasswordResetToken
	require.NoError(t, tx.Where("user_id = ?", testUser.ID).Find(&tokens1).Error)
	require.Len(t, tokens1, 1)
	assert.Nil(t, tokens1[0].UsedAt, "Token 1 should be active")
	tokenID1 := tokens1[0].ID

	// =========================================================================
	// 2. Request 2: Consecutive request within cooldown -> Should trigger cooldown
	// =========================================================================
	reqBody2, _ := json.Marshal(request.ForgotRequest{Email: testUser.Email})
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(reqBody2))
	c2.Request.Header.Set("Content-Type", "application/json")
	c2.Request.RemoteAddr = "192.0.2.100:12345"

	srv.ForgotPassword(c2)
	require.Equal(t, http.StatusOK, w2.Code, "Response must remain 200 OK (anti-enumeration)")

	var resp2 response.MessageResponse
	err = json.Unmarshal(w2.Body.Bytes(), &resp2)
	require.NoError(t, err)
	assert.Equal(t, resp1.Message, resp2.Message, "Response message must be identical to avoid enumeration")

	// Verify no new email was dispatched
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, int32(1), emailCount.Load(), "Email count must remain 1 during cooldown")

	// Check DB: token count in DB must still be 1 (no new token generated)
	var tokens2 []models.PasswordResetToken
	require.NoError(t, tx.Where("user_id = ?", testUser.ID).Find(&tokens2).Error)
	require.Len(t, tokens2, 1, "No new token should be created during cooldown")
	assert.Nil(t, tokens2[0].UsedAt, "Token 1 should still be active")

	// =========================================================================
	// 3. Request 3: After cooldown expires -> Should generate Token 2 and invalidate Token 1
	// =========================================================================
	time.Sleep(100 * time.Millisecond) // Wait past the 80ms cooldown

	reqBody3, _ := json.Marshal(request.ForgotRequest{Email: testUser.Email})
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(reqBody3))
	c3.Request.Header.Set("Content-Type", "application/json")
	c3.Request.RemoteAddr = "192.0.2.100:12345"

	srv.ForgotPassword(c3)
	require.Equal(t, http.StatusOK, w3.Code)

	// Wait for second email dispatch
	require.Eventually(t, func() bool {
		return emailCount.Load() == 2
	}, 500*time.Millisecond, 10*time.Millisecond, "expected second email to be dispatched")

	lastLinkMu.Lock()
	link2 := lastLink
	lastLinkMu.Unlock()
	require.NotEmpty(t, link2)
	assert.NotEqual(t, link1, link2, "Link 2 must be different from Link 1")

	u2, err := url.Parse(link2)
	require.NoError(t, err)
	tokenRaw2 := u2.Query().Get("token")
	require.NotEmpty(t, tokenRaw2)
	assert.NotEqual(t, tokenRaw1, tokenRaw2)

	// Check DB: should now have 2 tokens for this user
	var tokens3 []models.PasswordResetToken
	require.NoError(t, tx.Where("user_id = ?", testUser.ID).Order("id ASC").Find(&tokens3).Error)
	require.Len(t, tokens3, 2)

	// Token 1 MUST now be marked as used/invalidated!
	var tok1Check models.PasswordResetToken
	require.NoError(t, tx.First(&tok1Check, tokenID1).Error)
	assert.NotNil(t, tok1Check.UsedAt, "Token 1 must be marked used_at after new token generation")

	// Token 2 MUST be active
	tok2Check := tokens3[1]
	assert.Nil(t, tok2Check.UsedAt, "Token 2 must be active")

	// =========================================================================
	// 4. Invalidation Verification: Token 1 fails, Token 2 succeeds
	// =========================================================================

	// Test 4a: Verify Token 1 on /reset-password/verify -> must return invalid/already_used
	wVerify1 := httptest.NewRecorder()
	cVerify1, _ := gin.CreateTestContext(wVerify1)
	cVerify1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify?token="+tokenRaw1, nil)
	srv.VerifyResetPasswordToken(cVerify1)
	assert.Equal(t, http.StatusBadRequest, wVerify1.Code)

	var verifyResp1 response.VerifyResetTokenResponse
	_ = json.Unmarshal(wVerify1.Body.Bytes(), &verifyResp1)
	assert.False(t, verifyResp1.Valid)
	assert.Equal(t, "already_used", verifyResp1.Status)

	// Test 4b: Attempt ResetPassword with Token 1 -> must be rejected
	resetBody1, _ := json.Marshal(request.ResetRequest{
		Token:    tokenRaw1,
		Password: "BrandNewPassw0rd!2026",
	})
	wReset1 := httptest.NewRecorder()
	cReset1, _ := gin.CreateTestContext(wReset1)
	cReset1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(resetBody1))
	cReset1.Request.Header.Set("Content-Type", "application/json")
	srv.ResetPassword(cReset1)
	assert.Equal(t, http.StatusBadRequest, wReset1.Code)

	// Test 4c: Verify Token 2 on /reset-password/verify -> must be valid
	wVerify2 := httptest.NewRecorder()
	cVerify2, _ := gin.CreateTestContext(wVerify2)
	cVerify2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/reset-password/verify?token="+tokenRaw2, nil)
	srv.VerifyResetPasswordToken(cVerify2)
	assert.Equal(t, http.StatusOK, wVerify2.Code)

	var verifyResp2 response.VerifyResetTokenResponse
	_ = json.Unmarshal(wVerify2.Body.Bytes(), &verifyResp2)
	assert.True(t, verifyResp2.Valid)
	assert.Equal(t, "valid", verifyResp2.Status)

	// Test 4d: ResetPassword with Token 2 -> must succeed
	resetBody2, _ := json.Marshal(request.ResetRequest{
		Token:    tokenRaw2,
		Password: "BrandNewPassw0rd!2026",
	})
	wReset2 := httptest.NewRecorder()
	cReset2, _ := gin.CreateTestContext(wReset2)
	cReset2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(resetBody2))
	cReset2.Request.Header.Set("Content-Type", "application/json")
	srv.ResetPassword(cReset2)
	assert.Equal(t, http.StatusOK, wReset2.Code)
}

func TestForgotPassword_AntiEnumeration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	m := mailer.New(cfg)
	srv, err := NewServer(tx, cfg, authSvc, m, nil)
	require.NoError(t, err)

	var emailSent bool
	srv.sendResetEmailFunc = func(toEmail, username, resetLink string) error {
		emailSent = true
		return nil
	}

	// Non-existent email request
	reqBody, _ := json.Marshal(request.ForgotRequest{Email: "nonexistent_email_12345@example.com"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "192.0.2.101:12345"

	srv.ForgotPassword(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, emailSent)

	var resp response.MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "if the email exists, a reset link has been sent", resp.Message)
}

func TestForgotPassword_ConcurrencyRaceFree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	m := mailer.New(cfg)
	srv, err := NewServer(tx, cfg, authSvc, m, nil)
	require.NoError(t, err)

	srv.Cfg.ResetPasswordCooldown = 2 * time.Second

	var emailCount atomic.Int32
	srv.sendResetEmailFunc = func(toEmail, username, resetLink string) error {
		emailCount.Add(1)
		return nil
	}

	testUser := models.User{
		Username:     "concurrency_user",
		Email:        "concurrency_user@example.com",
		Name:         "Concurrency User",
		PasswordHash: "somehash",
		Role:         models.RoleUser,
		IsActive:     true,
	}
	require.NoError(t, tx.Create(&testUser).Error)

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			body, _ := json.Marshal(request.ForgotRequest{Email: testUser.Email})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.RemoteAddr = "192.0.2.102:12345"

			srv.ForgotPassword(c)
			assert.Equal(t, http.StatusOK, w.Code)
		}(i)
	}

	wg.Wait()

	// Exactly 1 email should have been sent out of the 10 concurrent requests
	assert.Equal(t, int32(1), emailCount.Load(), "Only 1 email should be sent due to cooldown and thread-safety")

	// Check DB: only 1 active token should exist
	var activeTokens []models.PasswordResetToken
	require.NoError(t, tx.Where("user_id = ? AND used_at IS NULL", testUser.ID).Find(&activeTokens).Error)
	assert.Equal(t, 1, len(activeTokens), "Exactly 1 active token must exist in the database")
}
