package handlers

import (
	"bytes"
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
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("CreateApprover rejects invalid role type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"name": "Alice", "role_type": "invalid_role"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/approvers", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateApprover(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("CreateCompany rejects empty body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/companies", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateCompany(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("CreateCompany rejects too short code", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"code": "m", "name": "Valid Name"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/companies", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateCompany(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("UpdateApprover rejects invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/1", bytes.NewReader([]byte("not-json")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateApprover(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("UpdateApprover rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/"+invalidID, bytes.NewReader([]byte("{}")))
			c.Request.Header.Set("Content-Type", "application/json")

			srv.UpdateApprover(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("UpdateApprover nil DB", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/1", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateApprover(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("DeleteApprover rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approvers/"+invalidID, nil)

			srv.DeleteApprover(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("UpdateCompany rejects invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/1", bytes.NewReader([]byte("not-json")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateCompany(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("UpdateCompany rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/"+invalidID, bytes.NewReader([]byte("{}")))
			c.Request.Header.Set("Content-Type", "application/json")

			srv.UpdateCompany(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("UpdateCompany nil DB", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/1", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateCompany(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("DeleteCompany rejects invalid ID", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invalidID}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/companies/"+invalidID, nil)

			srv.DeleteCompany(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})
}

func TestAdminMasterHandlers_ApproverDBIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{
		DB:  tx,
		Cfg: cfg,
	}

	// 1. Create Approver
	createBody := `{"name": "Master Approver", "role_type": "team_leader", "title": "Lead Engineer", "is_active": true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/approvers", bytes.NewReader([]byte(createBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.CreateApprover(c)
	assertFatalCode(t, w, http.StatusCreated)

	var created models.Approver
	if err := srv.DB.Order("id desc").First(&created).Error; err != nil {
		t.Fatalf("failed to find created approver: %v", err)
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
	assertFatalCode(t, w, http.StatusOK)

	// Update not found
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "99999999"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/approvers/99999999", bytes.NewReader([]byte(updateBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.UpdateApprover(c)
	assertFatalCode(t, w, http.StatusNotFound)

	// 3. Delete Approver
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approvers/"+idStr, nil)

	srv.DeleteApprover(c)
	assertFatalCode(t, w, http.StatusOK)

	// Delete not found
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "99999999"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approvers/99999999", nil)

	srv.DeleteApprover(c)
	assertFatalCode(t, w, http.StatusNotFound)
}

func TestAdminMasterHandlers_CompanyDBIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{
		DB:  tx,
		Cfg: cfg,
	}

	// 1. Create Company
	createBody := `{"code": "tstcorp", "name": "Test Corporation"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/companies", bytes.NewReader([]byte(createBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.CreateCompany(c)
	assertFatalCode(t, w, http.StatusCreated)

	var created models.Company
	if err := srv.DB.Where("code = ?", "tstcorp").First(&created).Error; err != nil {
		t.Fatalf("failed to find created company: %v", err)
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
	assertFatalCode(t, w, http.StatusOK)

	// Update not found
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "99999999"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/companies/99999999", bytes.NewReader([]byte(updateBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.UpdateCompany(c)
	assertFatalCode(t, w, http.StatusNotFound)

	// 3. Delete Company
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/companies/"+idStr, nil)

	srv.DeleteCompany(c)
	assertFatalCode(t, w, http.StatusOK)

	// Delete not found
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "99999999"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/companies/99999999", nil)

	srv.DeleteCompany(c)
	assertFatalCode(t, w, http.StatusNotFound)
}
