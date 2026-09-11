package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/models"
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

	t.Run("UpdateApprover rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/"+invalidID, bytes.NewReader([]byte("{}")))
			c.Request.Header.Set("Content-Type", "application/json")

			srv.UpdateApprover(c)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for ID %q, got %d", invalidID, w.Code)
			}
		}
	})

	t.Run("UpdateApprover nil DB", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/1", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateApprover(c)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", w.Code)
		}
	})

	t.Run("DeleteApprover rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approvers/"+invalidID, nil)

			srv.DeleteApprover(c)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for ID %q, got %d", invalidID, w.Code)
			}
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

	t.Run("UpdateCompany rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/"+invalidID, bytes.NewReader([]byte("{}")))
			c.Request.Header.Set("Content-Type", "application/json")

			srv.UpdateCompany(c)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for ID %q, got %d", invalidID, w.Code)
			}
		}
	})

	t.Run("UpdateCompany nil DB", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/1", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateCompany(c)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", w.Code)
		}
	})

	t.Run("DeleteCompany rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/companies/"+invalidID, nil)

			srv.DeleteCompany(c)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for ID %q, got %d", invalidID, w.Code)
			}
		}
	})
}

func TestAdminMasterHandlers_DBIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{
		DB:  tx,
		Cfg: cfg,
	}

	// Approver CRUD lifecycle
	t.Run("Approver CRUD lifecycle", func(t *testing.T) {
		// 1. Create Approver
		createBody := `{"name": "Master Approver", "role_type": "team_leader", "title": "Lead Engineer", "is_active": true}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/approvers", bytes.NewReader([]byte(createBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateApprover(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d (body: %s)", w.Code, w.Body.String())
		}

		var created models.Approver
		var respEnvelope struct {
			Data models.Approver `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &respEnvelope)
		created = respEnvelope.Data
		if created.ID == 0 {
			t.Fatalf("expected non-zero ID for created approver")
		}

		idStr := fmt.Sprintf("%d", created.ID)

		// 2. Update Approver
		updateBody := `{"name": "Updated Lead", "title": "Senior Lead"}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: idStr}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/"+idStr, bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateApprover(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// Update not found
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/99999999", bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateApprover(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}

		// 3. Delete Approver
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: idStr}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approvers/"+idStr, nil)

		srv.DeleteApprover(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// Delete not found
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approvers/99999999", nil)

		srv.DeleteApprover(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// Company CRUD lifecycle
	t.Run("Company CRUD lifecycle", func(t *testing.T) {
		// 1. Create Company
		createBody := `{"code": "tstcorp", "name": "Test Corporation"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/companies", bytes.NewReader([]byte(createBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateCompany(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d (body: %s)", w.Code, w.Body.String())
		}

		var created models.Company
		var respEnvelope struct {
			Data models.Company `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &respEnvelope)
		created = respEnvelope.Data
		if created.ID == 0 {
			t.Fatalf("expected non-zero ID for created company")
		}

		idStr := fmt.Sprintf("%d", created.ID)

		// 2. Update Company
		updateBody := `{"code": "tstcorp2", "name": "Test Corporation Updated"}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: idStr}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/"+idStr, bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateCompany(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// Update not found
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/99999999", bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateCompany(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}

		// 3. Delete Company
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: idStr}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/companies/"+idStr, nil)

		srv.DeleteCompany(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// Delete not found
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "99999999"}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/companies/99999999", nil)

		srv.DeleteCompany(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})
}
