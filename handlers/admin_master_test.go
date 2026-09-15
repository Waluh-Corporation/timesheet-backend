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

func TestAdminMasterHandlers_SiteCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{DB: tx, Cfg: cfg}

	// 1. Create Site
	createBody := `{"code": "testsite", "name": "Test Site Jakarta"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/sites", bytes.NewReader([]byte(createBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.CreateSite(c)
	assertFatalCode(t, w, http.StatusCreated)

	var created models.Site
	if err := srv.DB.Where("code = ?", "testsite").First(&created).Error; err != nil {
		t.Fatalf("failed to find created site: %v", err)
	}
	idStr := fmt.Sprintf("%d", created.ID)

	// 2. List Sites
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sites", nil)
	srv.ListSites(c)
	assertFatalCode(t, w, http.StatusOK)

	// 3. Update Site
	updateBody := `{"name": "Test Site Jakarta Updated"}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/sites/"+idStr, bytes.NewReader([]byte(updateBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.UpdateSite(c)
	assertFatalCode(t, w, http.StatusOK)

	// 4. Delete Site
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/sites/"+idStr, nil)

	srv.DeleteSite(c)
	assertFatalCode(t, w, http.StatusOK)
}

func TestAdminMasterHandlers_DivisionAndDepartmentCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{DB: tx, Cfg: cfg}

	// 1. Create Division
	createDivBody := `{"code": "tdiv", "name": "Test Division WDD"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/divisions", bytes.NewReader([]byte(createDivBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.CreateDivision(c)
	assertFatalCode(t, w, http.StatusCreated)

	var createdDiv models.Division
	if err := srv.DB.Where("code = ?", "tdiv").First(&createdDiv).Error; err != nil {
		t.Fatalf("failed to find created division: %v", err)
	}
	divIdStr := fmt.Sprintf("%d", createdDiv.ID)

	// 2. List Divisions
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/divisions", nil)
	srv.ListDivisions(c)
	assertFatalCode(t, w, http.StatusOK)

	// 3. Create Department linked to Division
	createDeptBody := fmt.Sprintf(`{"code": "tdept", "name": "Test Department Core", "division_id": %d}`, createdDiv.ID)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/departments", bytes.NewReader([]byte(createDeptBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.CreateDepartment(c)
	assertFatalCode(t, w, http.StatusCreated)

	var createdDept models.Department
	if err := srv.DB.Where("code = ?", "TDEPT").First(&createdDept).Error; err != nil {
		t.Fatalf("failed to find created department: %v", err)
	}
	if createdDept.Division != "Test Division WDD" {
		t.Fatalf("expected department division to be synced to 'Test Division WDD', got %q", createdDept.Division)
	}

	// 4. List Departments with division filter
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/departments?division_id=%d", createdDiv.ID), nil)
	srv.ListDepartments(c)
	assertFatalCode(t, w, http.StatusOK)

	// 4b. Update Division
	updateDivBody := `{"name": "Test Division WDD Updated", "code": "tdiv2"}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: divIdStr}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/divisions/"+divIdStr, bytes.NewReader([]byte(updateDivBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.UpdateDivision(c)
	assertFatalCode(t, w, http.StatusOK)

	// 5. Update Department
	updateDeptBody := `{"name": "Test Department Core Renamed"}`
	deptIdStr := fmt.Sprintf("%d", createdDept.ID)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: deptIdStr}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/departments/"+deptIdStr, bytes.NewReader([]byte(updateDeptBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.UpdateDepartment(c)
	assertFatalCode(t, w, http.StatusOK)

	// 5b. Update Department with Division name matching existing
	updateDeptDivNameBody := `{"division": "Test Division WDD Updated"}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: deptIdStr}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/departments/"+deptIdStr, bytes.NewReader([]byte(updateDeptDivNameBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.UpdateDepartment(c)
	assertFatalCode(t, w, http.StatusOK)

	// 5c. Update Department with non-existent Division name
	updateDeptNonDivBody := `{"division": "Non Existent Division Name"}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: deptIdStr}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/departments/"+deptIdStr, bytes.NewReader([]byte(updateDeptNonDivBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.UpdateDepartment(c)
	assertFatalCode(t, w, http.StatusOK)

	// 5d. Update Department clearing Division with division_id = 0
	clearDivBody := `{"division_id": 0}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: deptIdStr}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/departments/"+deptIdStr, bytes.NewReader([]byte(clearDivBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.UpdateDepartment(c)
	assertFatalCode(t, w, http.StatusOK)

	// 5e. Update Department clearing Division with division = ""
	clearDivStrBody := `{"division": ""}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: deptIdStr}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/departments/"+deptIdStr, bytes.NewReader([]byte(clearDivStrBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	srv.UpdateDepartment(c)
	assertFatalCode(t, w, http.StatusOK)

	// 6. Delete Department
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: deptIdStr}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/departments/"+deptIdStr, nil)

	srv.DeleteDepartment(c)
	assertFatalCode(t, w, http.StatusOK)

	// 7. Delete Division
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: divIdStr}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/divisions/"+divIdStr, nil)

	srv.DeleteDivision(c)
	assertFatalCode(t, w, http.StatusOK)
}

func TestAdminMasterHandlers_Validation_SitesDivisionsDepartments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{DB: tx, Cfg: cfg}

	// Invalid JSON payload tests
	t.Run("Create handlers reject empty or invalid JSON", func(t *testing.T) {
		handlers := []func(*gin.Context){
			srv.CreateSite,
			srv.CreateDivision,
			srv.CreateDepartment,
		}
		for _, h := range handlers {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{invalid")))
			c.Request.Header.Set("Content-Type", "application/json")
			h(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("Update handlers reject invalid JSON", func(t *testing.T) {
		handlers := []func(*gin.Context){
			srv.UpdateSite,
			srv.UpdateDivision,
			srv.UpdateDepartment,
		}
		for _, h := range handlers {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: "1"}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/1", bytes.NewReader([]byte("{invalid")))
			c.Request.Header.Set("Content-Type", "application/json")
			h(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("Update and Delete reject invalid IDs", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-1"} {
			updateHandlers := []func(*gin.Context){
				srv.UpdateSite,
				srv.UpdateDivision,
				srv.UpdateDepartment,
			}
			for _, h := range updateHandlers {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Params = gin.Params{{Key: "id", Value: invalidID}}
				c.Request = httptest.NewRequest(http.MethodPatch, "/"+invalidID, bytes.NewReader([]byte("{}")))
				c.Request.Header.Set("Content-Type", "application/json")
				h(c)
				assertResponseCode(t, w, http.StatusBadRequest)
			}

			deleteHandlers := []func(*gin.Context){
				srv.DeleteSite,
				srv.DeleteDivision,
				srv.DeleteDepartment,
			}
			for _, h := range deleteHandlers {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Params = gin.Params{{Key: "id", Value: invalidID}}
				c.Request = httptest.NewRequest(http.MethodDelete, "/"+invalidID, nil)
				h(c)
				assertResponseCode(t, w, http.StatusBadRequest)
			}
		}
	})

	t.Run("Update and Delete return 404 for non-existent IDs", func(t *testing.T) {
		updateHandlers := []func(*gin.Context){
			srv.UpdateSite,
			srv.UpdateDivision,
			srv.UpdateDepartment,
		}
		for _, h := range updateHandlers {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: "99999999"}}
			c.Request = httptest.NewRequest(http.MethodPatch, "/99999999", bytes.NewReader([]byte("{}")))
			c.Request.Header.Set("Content-Type", "application/json")
			h(c)
			assertResponseCode(t, w, http.StatusNotFound)
		}

		deleteHandlers := []func(*gin.Context){
			srv.DeleteSite,
			srv.DeleteDivision,
			srv.DeleteDepartment,
		}
		for _, h := range deleteHandlers {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: "99999999"}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/99999999", nil)
			h(c)
			assertResponseCode(t, w, http.StatusNotFound)
		}
	})
}

func TestAdminMasterHandlers_AdminListEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{DB: tx, Cfg: cfg}

	// Seed one of each
	comp := models.Company{Code: "list_comp", Name: "List Comp", IsActive: true}
	_ = tx.Create(&comp)
	appr := models.Approver{Name: "List Appr", RoleType: models.ApproverRoleTeamLeader, IsActive: true}
	_ = tx.Create(&appr)
	site := models.Site{Code: "list_site", Name: "List Site", IsActive: true}
	_ = tx.Create(&site)
	div := models.Division{Code: "list_div", Name: "List Div", IsActive: true}
	_ = tx.Create(&div)
	dept := models.Department{Code: "list_dept", Name: "List Dept", DivisionID: &div.ID, Division: div.Name, IsActive: true}
	_ = tx.Create(&dept)

	endpoints := []struct {
		name    string
		handler func(*gin.Context)
		url     string
	}{
		{"AdminListApprovers", srv.AdminListApprovers, "/api/v1/admin/approvers"},
		{"AdminListCompanies", srv.AdminListCompanies, "/api/v1/admin/companies"},
		{"AdminListSites", srv.AdminListSites, "/api/v1/admin/sites"},
		{"AdminListDivisions", srv.AdminListDivisions, "/api/v1/admin/divisions"},
		{"AdminListDepartments", srv.AdminListDepartments, "/api/v1/admin/departments"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name+" default", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, ep.url, nil)
			ep.handler(c)
			assertFatalCode(t, w, http.StatusOK)
		})

		t.Run(ep.name+" ?is_active=true", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, ep.url+"?is_active=true", nil)
			ep.handler(c)
			assertFatalCode(t, w, http.StatusOK)
		})

		t.Run(ep.name+" ?is_active=false", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, ep.url+"?is_active=false", nil)
			ep.handler(c)
			assertFatalCode(t, w, http.StatusOK)
		})

		t.Run(ep.name+" ?include_inactive=false", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, ep.url+"?include_inactive=false", nil)
			ep.handler(c)
			assertFatalCode(t, w, http.StatusOK)
		})
	}

	// Test AdminListDepartments with division query filter
	t.Run("AdminListDepartments with division filters", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/departments?division_id=%d", div.ID), nil)
		srv.AdminListDepartments(c)
		assertFatalCode(t, w, http.StatusOK)

		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/departments?division=List+Div", nil)
		srv.AdminListDepartments(c)
		assertFatalCode(t, w, http.StatusOK)
	})
}
