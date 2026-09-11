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
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 when deleting self, got %d", w.Code)
		}
	})

	t.Run("DeleteUser rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+invalidID, nil)

			srv.DeleteUser(c)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for invalid ID %q, got %d", invalidID, w.Code)
			}
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
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 when deactivating self, got %d", w.Code)
		}
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
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 when demoting self, got %d", w.Code)
		}
	})

	t.Run("UpdateUser rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+invalidID, bytes.NewReader([]byte("{}")))
			c.Request.Header.Set("Content-Type", "application/json")

			srv.UpdateUser(c)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for invalid ID %q, got %d", invalidID, w.Code)
			}
		}
	})

	t.Run("CreateUser rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateUser(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty create user payload, got %d", w.Code)
		}
	})

	t.Run("ReviewProfileChange rejects invalid ID and action", func(t *testing.T) {
		// Invalid ID
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "abc"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/abc/review?action=approve", nil)
		srv.ReviewProfileChange(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid ID, got %d", w.Code)
		}

		// Invalid Action
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/1/review?action=invalid", nil)
		srv.ReviewProfileChange(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid action, got %d", w.Code)
		}
	})
}

func TestUserHandlers_DBIntegration(t *testing.T) {
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

	// 1. Create a user to operate on
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

	// Admin caller ID
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
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("UpdateUser updates target attributes", func(t *testing.T) {
		// Valid update
		updateBody := `{"name": "Updated Target Name", "bni_id": "123456", "division": "Technology", "department": "IT", "site": "Jakarta", "role": "user", "is_active": true}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+targetIDStr, bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d (body: %s)", w.Code, w.Body.String())
		}

		// Update not found
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/99999999", bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("DeleteUser deactivates user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+targetIDStr, nil)

		srv.DeleteUser(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// Delete not found
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/99999999", nil)

		srv.DeleteUser(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("Profile Change Flow (Submit, List, Review)", func(t *testing.T) {
		// 1. Submit Profile Change
		submitBody := `{"name": "Target New Name", "bni_id": "78910", "division": "Fintech"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, targetUser.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/change", bytes.NewReader([]byte(submitBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.SubmitProfileChange(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", w.Code)
		}

		var created models.ProfileChangeRequest
		var respEnvelope struct {
			Data models.ProfileChangeRequest `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &respEnvelope)
		created = respEnvelope.Data
		reqIDStr := fmt.Sprintf("%d", created.ID)

		// 2. MyProfileChanges
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, targetUser.ID)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/profile/changes", nil)

		srv.MyProfileChanges(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// 3. ListProfileChanges
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/profile-changes?status=pending", nil)

		srv.ListProfileChanges(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// 4. ReviewProfileChange - Not Found
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/99999999/review?action=approve", nil)

		srv.ReviewProfileChange(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}

		// 5. ReviewProfileChange - Approve
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: reqIDStr}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+reqIDStr+"/review?action=approve", nil)

		srv.ReviewProfileChange(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// 6. ReviewProfileChange - Already Reviewed conflict
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: reqIDStr}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+reqIDStr+"/review?action=reject", nil)

		srv.ReviewProfileChange(c)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d", w.Code)
		}

		// 7. Submit second request and reject it
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
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on reject, got %d", w.Code)
		}
	})
}
