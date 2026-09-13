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

	"timesheet-backend/auth"
	"timesheet-backend/models"
)

func TestUserHandlers_ChangePassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{
		DB:     tx,
		Cfg:    cfg,
		Hasher: auth.DefaultHasher,
	}

	oldPassword := "OldValidSecretPass123!"
	oldHash, err := auth.HashPassword(oldPassword)
	if err != nil {
		t.Fatalf("failed to hash old password: %v", err)
	}

	user := models.User{
		Username:     "test_cp_user",
		Email:        "test_cp@example.com",
		Name:         "Change Pass User",
		Role:         models.RoleUser,
		PasswordHash: oldHash,
		IsActive:     true,
	}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	t.Run("ChangePassword unauthorized when unauthenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"old_password":"` + oldPassword + `","new_password":"NewValidSecretPass123!"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/change-password", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ChangePassword(c)
		assertResponseCode(t, w, http.StatusUnauthorized)
	})

	t.Run("ChangePassword rejects invalid old password", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		body := `{"old_password":"WrongOldPassword123!","new_password":"NewValidSecretPass123!"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/change-password", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ChangePassword(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ChangePassword rejects identical new password", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		body := `{"old_password":"` + oldPassword + `","new_password":"` + oldPassword + `"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/change-password", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ChangePassword(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ChangePassword rejects weak new password", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		body := `{"old_password":"` + oldPassword + `","new_password":"123"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/change-password", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ChangePassword(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ChangePassword success updates hash and updated_at", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		newPassword := "NewValidSecretPass456!"
		body := fmt.Sprintf(`{"old_password":"%s","new_password":"%s"}`, oldPassword, newPassword)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/change-password", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		initialUpdatedAt := user.UpdatedAt

		// Small delay to ensure timestamp difference
		time.Sleep(10 * time.Millisecond)

		srv.ChangePassword(c)
		assertResponseCode(t, w, http.StatusOK)

		var updated models.User
		if err := tx.Where("id = ?", user.ID).First(&updated).Error; err != nil {
			t.Fatalf("failed to query updated user: %v", err)
		}

		if !strings.HasPrefix(updated.PasswordHash, "$argon2id$") {
			t.Errorf("expected Argon2id hash prefix, got %s", updated.PasswordHash)
		}
		if !auth.CheckPassword(updated.PasswordHash, newPassword) {
			t.Error("new password fails verification against updated hash")
		}
		if auth.CheckPassword(updated.PasswordHash, oldPassword) {
			t.Error("old password still verifies against updated hash")
		}
		if !updated.UpdatedAt.After(initialUpdatedAt) {
			t.Errorf("expected updated_at (%v) to be after initial (%v)", updated.UpdatedAt, initialUpdatedAt)
		}
	})
}

func TestUserHandlers_CreateUser_NoResetTokenFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{
		DB:     tx,
		Cfg:    cfg,
		Hasher: auth.DefaultHasher,
	}

	admin := models.User{
		Username: "admin_create_test",
		Email:    "admin_create@example.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	if err := tx.Create(&admin).Error; err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	t.Run("CreateUser auto-generates password when omitted and never creates reset token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)

		reqBody := `{
			"username": "auto_pass_user",
			"email": "autopass@example.com",
			"name": "Auto Password User",
			"role": "user"
		}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		var resp struct {
			Code   int    `json:"code"`
			Status string `json:"status"`
			Data   struct {
				Message         string      `json:"message"`
				InitialPassword string      `json:"initial_password,omitempty"`
				User            models.User `json:"user"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Data.InitialPassword != "" {
			t.Fatalf("initial_password must NOT be returned in admin response, got %q", resp.Data.InitialPassword)
		}

		var rawMap map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &rawMap)
		if dataMap, ok := rawMap["data"].(map[string]interface{}); ok {
			if _, exists := dataMap["initial_password"]; exists {
				t.Errorf("initial_password key must not be present in admin response data")
			}
		}

		if resp.Data.User.Username != "auto_pass_user" {
			t.Errorf("expected username auto_pass_user, got %q", resp.Data.User.Username)
		}

		// Verify user in DB
		var created models.User
		if err := tx.Where("username = ?", "auto_pass_user").First(&created).Error; err != nil {
			t.Fatalf("failed to find created user in DB: %v", err)
		}
		if !strings.HasPrefix(created.PasswordHash, "$argon2id$") {
			t.Errorf("expected Argon2id hash, got: %s", created.PasswordHash)
		}

		// CRITICAL: Ensure NO PasswordResetToken was generated for this new user
		var resetTokenCount int64
		tx.Model(&models.PasswordResetToken{}).Where("user_id = ?", created.ID).Count(&resetTokenCount)
		if resetTokenCount != 0 {
			t.Errorf("expected 0 password reset tokens created, got %d", resetTokenCount)
		}
	})

	t.Run("CreateUser always auto-generates secure password and skips reset token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)

		reqBody := `{
			"username": "second_auto_user",
			"email": "second_auto@example.com",
			"name": "Second Auto User",
			"role": "user"
		}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		var rawMap map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &rawMap)
		if dataMap, ok := rawMap["data"].(map[string]interface{}); ok {
			if _, exists := dataMap["initial_password"]; exists {
				t.Errorf("initial_password key must not be present in admin response data")
			}
		}

		var created models.User
		if err := tx.Where("username = ?", "second_auto_user").First(&created).Error; err != nil {
			t.Fatalf("failed to find created user in DB: %v", err)
		}
		if !strings.HasPrefix(created.PasswordHash, "$argon2id$") {
			t.Errorf("expected Argon2id hash, got %s", created.PasswordHash)
		}

		// CRITICAL: Ensure NO PasswordResetToken was generated
		var resetTokenCount int64
		tx.Model(&models.PasswordResetToken{}).Where("user_id = ?", created.ID).Count(&resetTokenCount)
		if resetTokenCount != 0 {
			t.Errorf("expected 0 password reset tokens created, got %d", resetTokenCount)
		}
	})

	t.Run("CreateUser returns 409 Conflict when username is duplicate", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)

		reqBody := `{
			"username": "auto_pass_user",
			"email": "brand_new_email@example.com",
			"name": "Duplicate Username User",
			"role": "user"
		}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateUser(c)
		assertResponseCode(t, w, http.StatusConflict)

		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		if !strings.Contains(strings.ToLower(errResp.Error), "username already exists") {
			t.Errorf("expected conflict message about username, got: %s", errResp.Error)
		}
	})

	t.Run("CreateUser returns 409 Conflict when email is duplicate", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)

		reqBody := `{
			"username": "brand_new_username",
			"email": "autopass@example.com",
			"name": "Duplicate Email User",
			"role": "user"
		}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateUser(c)
		assertResponseCode(t, w, http.StatusConflict)

		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		if !strings.Contains(strings.ToLower(errResp.Error), "email already exists") {
			t.Errorf("expected conflict message about email, got: %s", errResp.Error)
		}
	})
}
