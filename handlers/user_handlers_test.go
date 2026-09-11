package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

	t.Run("ReviewProfileChange rejects invalid status", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		body := `{"action": "unknown_action"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/1/review", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ReviewProfileChange(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid review action, got %d", w.Code)
		}
	})
}
