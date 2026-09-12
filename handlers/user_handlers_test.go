package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/models"
)

func TestUserHandlers_SelfProtectionAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{}

	t.Run("DeleteUser rejects deleting own account", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "10"}}
		c.Set(ctxUserID, uint(10)) // caller is user 10
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/10", nil)

		srv.DeleteUser(c)
		assertResponseCode(t, w, http.StatusForbidden)
	})

	t.Run("DeleteUser rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+invalidID, nil)

			srv.DeleteUser(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("UpdateUser rejects deactivating own account", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "10"}}
		c.Set(ctxUserID, uint(10)) // caller is user 10
		body := `{"is_active": false}`
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/10", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		assertResponseCode(t, w, http.StatusForbidden)
	})

	t.Run("UpdateUser rejects demoting own account", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "10"}}
		c.Set(ctxUserID, uint(10)) // caller is user 10
		body := `{"role": "user"}`
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/10", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		assertResponseCode(t, w, http.StatusForbidden)
	})

	t.Run("UpdateUser rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+invalidID, bytes.NewReader([]byte("{}")))
			c.Request.Header.Set("Content-Type", "application/json")

			srv.UpdateUser(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("CreateUser rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateUser(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ReviewProfileChange rejects invalid ID and action", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "abc"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/abc/review?action=approve", nil)
		srv.ReviewProfileChange(c)
		assertResponseCode(t, w, http.StatusBadRequest)

		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/1/review?action=invalid", nil)
		srv.ReviewProfileChange(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})
}

func TestUserHandlers_UserCRUDIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Auth: authSvc,
	}

	targetUser := models.User{
		Username: "target_user_test",
		Email:    "target_user@example.com",
		Role:     models.RoleUser,
		Name:     "Target User",
		IsActive: true,
	}
	if err := tx.Create(&targetUser).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}
	targetIDStr := fmt.Sprintf("%d", targetUser.ID)

	adminUser := models.User{
		Username: "admin_caller_test",
		Email:    "admin_caller@example.com",
		Role:     models.RoleAdmin,
		Name:     "Admin Caller",
		IsActive: true,
	}
	if err := tx.Create(&adminUser).Error; err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	t.Run("ListUsers returns users", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)

		srv.ListUsers(c)
		assertFatalCode(t, w, http.StatusOK)
	})

	t.Run("UpdateUser updates target attributes", func(t *testing.T) {
		updateBody := `{"name": "Updated Target Name", "bni_id": "123456", "division": "Technology", "department": "IT", "site": "Jakarta", "role": "user", "is_active": true}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+targetIDStr, bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)

		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/99999999", bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusNotFound)
	})

	t.Run("DeleteUser deactivates user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+targetIDStr, nil)

		srv.DeleteUser(c)
		assertFatalCode(t, w, http.StatusOK)

		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/99999999", nil)

		srv.DeleteUser(c)
		assertFatalCode(t, w, http.StatusNotFound)
	})
}

func TestUserHandlers_ProfileChangeIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Auth: authSvc,
	}

	targetUser := models.User{
		Username: "profile_change_user",
		Email:    "target_profile@example.com",
		Role:     models.RoleUser,
		Name:     "Target Profile User",
		IsActive: true,
	}
	if err := tx.Create(&targetUser).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	adminUser := models.User{
		Username: "profile_admin_caller",
		Email:    "admin_profile@example.com",
		Role:     models.RoleAdmin,
		Name:     "Admin Profile Caller",
		IsActive: true,
	}
	if err := tx.Create(&adminUser).Error; err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	submitBody := `{"name": "Target New Name", "bni_id": "78910", "division": "Fintech"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ctxUserID, targetUser.ID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/change", bytes.NewReader([]byte(submitBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.SubmitProfileChange(c)
	assertFatalCode(t, w, http.StatusCreated)

	var respEnvelope struct {
		Data models.ProfileChangeRequest `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &respEnvelope)
	reqIDStr := fmt.Sprintf("%d", respEnvelope.Data.ID)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set(ctxUserID, targetUser.ID)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/profile/changes", nil)
	srv.MyProfileChanges(c)
	assertFatalCode(t, w, http.StatusOK)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/profile-changes?status=pending", nil)
	srv.ListProfileChanges(c)
	assertFatalCode(t, w, http.StatusOK)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set(ctxUserID, adminUser.ID)
	c.Params = gin.Params{{Key: "id", Value: "99999999"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/99999999/review?action=approve", nil)
	srv.ReviewProfileChange(c)
	assertFatalCode(t, w, http.StatusNotFound)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set(ctxUserID, adminUser.ID)
	c.Params = gin.Params{{Key: "id", Value: reqIDStr}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+reqIDStr+"/review?action=approve", nil)
	srv.ReviewProfileChange(c)
	assertFatalCode(t, w, http.StatusOK)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set(ctxUserID, adminUser.ID)
	c.Params = gin.Params{{Key: "id", Value: reqIDStr}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+reqIDStr+"/review?action=reject", nil)
	srv.ReviewProfileChange(c)
	assertFatalCode(t, w, http.StatusConflict)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set(ctxUserID, targetUser.ID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/change", bytes.NewReader([]byte(submitBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.SubmitProfileChange(c)
	var respEnvelope2 struct {
		Data models.ProfileChangeRequest `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &respEnvelope2)
	req2IDStr := fmt.Sprintf("%d", respEnvelope2.Data.ID)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set(ctxUserID, adminUser.ID)
	c.Params = gin.Params{{Key: "id", Value: req2IDStr}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+req2IDStr+"/review?action=reject", nil)
	srv.ReviewProfileChange(c)
	assertFatalCode(t, w, http.StatusOK)
}
