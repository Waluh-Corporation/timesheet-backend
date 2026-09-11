package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminMasterHandlers_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{}

	t.Run("CreateApprover rejects empty body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/approvers", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateApprover(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty approver body, got %d", w.Code)
		}
	})

	t.Run("CreateApprover rejects invalid role type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"name": "Alice", "role_type": "invalid_role"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/approvers", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateApprover(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid role_type, got %d", w.Code)
		}
	})

	t.Run("CreateCompany rejects empty body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/companies", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateCompany(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty company body, got %d", w.Code)
		}
	})

	t.Run("CreateCompany rejects too short code", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"code": "m", "name": "Valid Name"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/companies", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateCompany(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for 1-char company code, got %d", w.Code)
		}
	})

	t.Run("UpdateApprover rejects invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/1", bytes.NewReader([]byte("not-json")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateApprover(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid json, got %d", w.Code)
		}
	})

	t.Run("UpdateCompany rejects invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/1", bytes.NewReader([]byte("not-json")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateCompany(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid json, got %d", w.Code)
		}
	})
}
