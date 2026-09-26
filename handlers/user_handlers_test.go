package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
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

	t.Run("CreateUser rejects invalid email", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"username":"testauto","email":"not-an-email","role":"user"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(body)))
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

	srv := newTestServer(t, tx, cfg)

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

	t.Run("ListUsers includes inactive users by default and respects filters", func(t *testing.T) {
		// 1. Default ListUsers should include the deactivated user
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		srv.ListUsers(c)
		assertFatalCode(t, w, http.StatusOK)

		var respAll response.APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &respAll); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		usersAllRaw, _ := json.Marshal(respAll.Data)
		var usersAll []response.AdminUserResponse
		_ = json.Unmarshal(usersAllRaw, &usersAll)

		foundDeactivated := false
		for _, u := range usersAll {
			if u.ID == targetUser.ID {
				foundDeactivated = true
				if u.IsActive {
					t.Errorf("expected deactivated user to have is_active = false, got true")
				}
			}
		}
		if !foundDeactivated {
			t.Errorf("expected deactivated user %d to be included in default ListUsers, but was missing", targetUser.ID)
		}

		// 2. ListUsers with ?is_active=true should NOT include deactivated user
		wActive := httptest.NewRecorder()
		cActive, _ := gin.CreateTestContext(wActive)
		cActive.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?is_active=true", nil)
		srv.ListUsers(cActive)
		assertFatalCode(t, wActive, http.StatusOK)

		var respActive response.APIResponse
		_ = json.Unmarshal(wActive.Body.Bytes(), &respActive)
		usersActiveRaw, _ := json.Marshal(respActive.Data)
		var usersActive []response.AdminUserResponse
		_ = json.Unmarshal(usersActiveRaw, &usersActive)

		for _, u := range usersActive {
			if u.ID == targetUser.ID {
				t.Errorf("expected user %d NOT to appear when is_active=true filter is applied", targetUser.ID)
			}
		}

		// 3. ListUsers with ?is_active=false should include deactivated user
		wInactive := httptest.NewRecorder()
		cInactive, _ := gin.CreateTestContext(wInactive)
		cInactive.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?is_active=false", nil)
		srv.ListUsers(cInactive)
		assertFatalCode(t, wInactive, http.StatusOK)

		var respInactive response.APIResponse
		_ = json.Unmarshal(wInactive.Body.Bytes(), &respInactive)
		usersInactiveRaw, _ := json.Marshal(respInactive.Data)
		var usersInactive []response.AdminUserResponse
		_ = json.Unmarshal(usersInactiveRaw, &usersInactive)

		foundInInactive := false
		for _, u := range usersInactive {
			if u.ID == targetUser.ID {
				foundInInactive = true
			}
			if u.IsActive {
				t.Errorf("user %d should not have is_active=true when is_active=false filter is applied", u.ID)
			}
		}
		if !foundInInactive {
			t.Errorf("expected deactivated user %d to appear when is_active=false filter is applied", targetUser.ID)
		}

		// 4. ListUsers with ?include_inactive=false should exclude deactivated user
		wIncFalse := httptest.NewRecorder()
		cIncFalse, _ := gin.CreateTestContext(wIncFalse)
		cIncFalse.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?include_inactive=false", nil)
		srv.ListUsers(cIncFalse)
		assertFatalCode(t, wIncFalse, http.StatusOK)

		var respIncFalse response.APIResponse
		_ = json.Unmarshal(wIncFalse.Body.Bytes(), &respIncFalse)
		usersIncFalseRaw, _ := json.Marshal(respIncFalse.Data)
		var usersIncFalse []response.AdminUserResponse
		_ = json.Unmarshal(usersIncFalseRaw, &usersIncFalse)

		for _, u := range usersIncFalse {
			if u.ID == targetUser.ID {
				t.Errorf("expected user %d NOT to appear when include_inactive=false", targetUser.ID)
			}
		}
	})

	t.Run("CreateUser with admin role ignores company and enforces no company relation", func(t *testing.T) {
		comp := models.Company{Code: "admin_test_comp", Name: "Admin Test Comp"}
		_ = tx.Create(&comp)

		createAdminBody := fmt.Sprintf(`{"username":"new_admin_nocomp","email":"admin_nocomp@example.com","role":"admin","name":"Admin No Comp","company_id":%d,"company":"Admin Test Comp"}`, comp.ID)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(createAdminBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		var createdAdmin models.User
		if err := tx.Where("username = ?", "new_admin_nocomp").First(&createdAdmin).Error; err != nil {
			t.Fatalf("failed to find created admin: %v", err)
		}
		if createdAdmin.CompanyID != nil {
			t.Errorf("expected admin company_id to be nil, got %v", createdAdmin.CompanyID)
		}
		if createdAdmin.Company != "" {
			t.Errorf("expected admin company to be empty, got %q", createdAdmin.Company)
		}
	})

	t.Run("UpdateUser to admin role clears any existing company relation", func(t *testing.T) {
		comp := models.Company{Code: "upd_admin_comp", Name: "Upd Admin Comp"}
		_ = tx.Create(&comp)
		regularUser := models.User{
			Username:  "regular_to_admin",
			Email:     "reg2admin@example.com",
			Role:      models.RoleUser,
			Name:      "Regular User",
			Company:   comp.Name,
			CompanyID: &comp.ID,
			IsActive:  true,
		}
		if err := tx.Create(&regularUser).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		updateBody := `{"role": "admin"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", regularUser.ID)}}
		c.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", regularUser.ID), bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)

		var reloaded models.User
		if err := tx.Where("id = ?", regularUser.ID).First(&reloaded).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if reloaded.CompanyID != nil {
			t.Errorf("expected promoted admin company_id to be nil, got %v", reloaded.CompanyID)
		}
		if reloaded.Company != "" {
			t.Errorf("expected promoted admin company to be empty, got %q", reloaded.Company)
		}
	})
}

func TestUserHandlers_ProfileChangeIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)

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

	comp := models.Company{Code: "pc_comp", Name: "PC Company"}
	_ = tx.Create(&comp)
	dept := models.Department{Name: "PC Dept", Division: "PC Div", IsActive: true}
	_ = tx.Create(&dept)

	submitBody := fmt.Sprintf(`{"name": "Target New Name", "bni_id": "78910", "employee_id": "EMP-9999", "division": "Fintech", "department": "PC Dept", "department_id": %d, "company_id": %d}`, dept.ID, comp.ID)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ctxUserID, targetUser.ID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/change", bytes.NewReader([]byte(submitBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.SubmitProfileChange(c)
	assertFatalCode(t, w, http.StatusCreated)

	var lastReq models.ProfileChangeRequest
	if err := tx.Where("user_id = ?", targetUser.ID).Order("id desc").First(&lastReq).Error; err != nil {
		t.Fatalf("failed to find created profile change request: %v", err)
	}
	reqIDStr := fmt.Sprintf("%d", lastReq.ID)

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

	var updatedTarget models.User
	tx.First(&updatedTarget, targetUser.ID)
	if updatedTarget.EmployeeID != "EMP-9999" {
		t.Errorf("expected employee_id 'EMP-9999', got %s", updatedTarget.EmployeeID)
	}

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
	var lastReq2 models.ProfileChangeRequest
	if err := tx.Where("user_id = ?", targetUser.ID).Order("id desc").First(&lastReq2).Error; err != nil {
		t.Fatalf("failed to find created profile change request: %v", err)
	}
	req2IDStr := fmt.Sprintf("%d", lastReq2.ID)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set(ctxUserID, adminUser.ID)
	c.Params = gin.Params{{Key: "id", Value: req2IDStr}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+req2IDStr+"/review?action=reject", nil)
	srv.ReviewProfileChange(c)
	assertFatalCode(t, w, http.StatusOK)

	// ReviewProfileChange with invalid action
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: req2IDStr}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+req2IDStr+"/review?action=invalid_action", nil)
	srv.ReviewProfileChange(c)
	assertFatalCode(t, w, http.StatusBadRequest)
}

func TestUserHandlers_CreateUpdateDeleteList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)

	comp := models.Company{Code: "user_test_comp", Name: "User Test Company"}
	_ = tx.Create(&comp)

	dept := models.Department{Name: "Product Engineering", IsActive: true}
	_ = tx.Create(&dept)

	adminUser := models.User{
		Username: "admin_test_flow",
		Email:    "admin_flow@example.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	_ = tx.Create(&adminUser)

	var newUserID uint

	t.Run("CreateUser with company, department, and password", func(t *testing.T) {
		reqBody := `{
			"username": "new_created_user",
			"email": "new_created_user@example.com",
			"name": "New Created User",
			"role": "user",
			"company": "user_test_comp",
			"department": "Product Engineering",
			"initial_password": "StrongPassword123!"
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		var createdUser models.User
		if err := tx.Where("username = ?", "new_created_user").First(&createdUser).Error; err != nil {
			t.Fatalf("failed to query created user: %v", err)
		}
		newUserID = createdUser.ID
		if newUserID == 0 {
			t.Fatal("expected non-zero created user ID")
		}
	})

	t.Run("CreateUser with company_id and department_id", func(t *testing.T) {
		reqBody := fmt.Sprintf(`{
			"username": "user_with_rel_ids",
			"email": "user_with_rel_ids@example.com",
			"name": "User Rel IDs",
			"role": "user",
			"company_id": %d,
			"department_id": %d,
			"initial_password": "StrongPassword123!"
		}`, comp.ID, dept.ID)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)
	})

	t.Run("CreateUser with employee_id saves in database", func(t *testing.T) {
		reqBody := `{
			"username": "user_with_employee_id",
			"email": "user_with_employee_id@example.com",
			"name": "User Employee ID",
			"role": "user",
			"bni_id": "000001",
			"employee_id": "000001"
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		var saved models.User
		if err := tx.Where("username = ?", "user_with_employee_id").First(&saved).Error; err != nil {
			t.Fatalf("failed to query created user: %v", err)
		}
		if saved.EmployeeID != "000001" {
			t.Fatalf("expected EmployeeID '000001', got '%s'", saved.EmployeeID)
		}
		if saved.BniID != "000001" {
			t.Fatalf("expected BniID '000001', got '%s'", saved.BniID)
		}
	})

	t.Run("UpdateUser modifies user attributes", func(t *testing.T) {
		newName := "Updated New User Name"
		reqBody := fmt.Sprintf(`{"name": "%s", "company": "user_test_comp", "department": "Product Engineering"}`, newName)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", newUserID)}}
		c.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", newUserID), bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)
	})

	t.Run("ListUsers returns paginated users", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&limit=10", nil)
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.ListUsers(c)
		assertFatalCode(t, w, http.StatusOK)
	})

	t.Run("CreateUser and UpdateUser with site_id and division_id returns human-readable names and IDs", func(t *testing.T) {
		site1 := models.Site{Code: "sby_test", Name: "Surabaya Hub", IsActive: true}
		_ = tx.Create(&site1)
		site2 := models.Site{Code: "bdg_test", Name: "Bandung Hub", IsActive: true}
		_ = tx.Create(&site2)

		div1 := models.Division{Code: "it_ops", Name: "IT Operations", IsActive: true}
		_ = tx.Create(&div1)
		div2 := models.Division{Code: "fin_tech", Name: "Financial Technology", IsActive: true}
		_ = tx.Create(&div2)

		reqBody := fmt.Sprintf(`{
			"username": "user_site_div_test",
			"email": "user_site_div@example.com",
			"name": "User Site Div",
			"role": "user",
			"company": "user_test_comp",
			"department": "Product Engineering",
			"site_id": %d,
			"division_id": %d,
			"initial_password": "StrongPassword123!"
		}`, site1.ID, div1.ID)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(reqBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		var resp struct {
			Data struct {
				Message string                `json:"message"`
				User    response.UserResponse `json:"user"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Data.User.Site != "Surabaya Hub" {
			t.Errorf("expected site 'Surabaya Hub', got %q", resp.Data.User.Site)
		}
		if resp.Data.User.SiteID == nil || *resp.Data.User.SiteID != site1.ID {
			t.Errorf("expected site_id %d, got %v", site1.ID, resp.Data.User.SiteID)
		}
		if resp.Data.User.Division != "IT Operations" {
			t.Errorf("expected division 'IT Operations', got %q", resp.Data.User.Division)
		}
		if resp.Data.User.DivisionID == nil || *resp.Data.User.DivisionID != div1.ID {
			t.Errorf("expected division_id %d, got %v", div1.ID, resp.Data.User.DivisionID)
		}

		// Update to site2 and div2
		updateBody := fmt.Sprintf(`{"site_id": %d, "division_id": %d}`, site2.ID, div2.ID)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", resp.Data.User.ID)}}
		c.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", resp.Data.User.ID), bytes.NewReader([]byte(updateBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)

		var updatedUser models.User
		if err := tx.Where("id = ?", resp.Data.User.ID).First(&updatedUser).Error; err != nil {
			t.Fatalf("failed to query updated user: %v", err)
		}
		if updatedUser.Site != "Bandung Hub" {
			t.Errorf("expected updated site 'Bandung Hub', got %q", updatedUser.Site)
		}
		if updatedUser.SiteID == nil || *updatedUser.SiteID != site2.ID {
			t.Errorf("expected updated site_id %d, got %v", site2.ID, updatedUser.SiteID)
		}
		if updatedUser.Division != "Financial Technology" {
			t.Errorf("expected updated division 'Financial Technology', got %q", updatedUser.Division)
		}
		if updatedUser.DivisionID == nil || *updatedUser.DivisionID != div2.ID {
			t.Errorf("expected updated division_id %d, got %v", div2.ID, updatedUser.DivisionID)
		}

		// Verify ListUsers returns human-readable fields and IDs
		wList := httptest.NewRecorder()
		cList, _ := gin.CreateTestContext(wList)
		cList.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		cList.Set(ctxUserID, adminUser.ID)
		cList.Set(ctxRole, models.RoleAdmin)

		srv.ListUsers(cList)
		assertFatalCode(t, wList, http.StatusOK)

		var listResp struct {
			Data []response.AdminUserResponse `json:"data"`
		}
		if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("failed to decode list response: %v", err)
		}
		var foundUser *response.AdminUserResponse
		for i := range listResp.Data {
			if listResp.Data[i].ID == resp.Data.User.ID {
				foundUser = &listResp.Data[i]
				break
			}
		}
		if foundUser == nil {
			t.Fatal("user not found in ListUsers")
		}
		if foundUser.Site != "Bandung Hub" {
			t.Errorf("expected site 'Bandung Hub', got %q", foundUser.Site)
		}
		if foundUser.SiteID == nil || *foundUser.SiteID != site2.ID {
			t.Errorf("expected site_id %d, got %v", site2.ID, foundUser.SiteID)
		}
		if foundUser.Division != "Financial Technology" {
			t.Errorf("expected division 'Financial Technology', got %q", foundUser.Division)
		}
		if foundUser.DivisionID == nil || *foundUser.DivisionID != div2.ID {
			t.Errorf("expected division_id %d, got %v", div2.ID, foundUser.DivisionID)
		}
	})

	t.Run("DeleteUser deactivates user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", newUserID)}}
		c.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", newUserID), nil)
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.DeleteUser(c)
		assertFatalCode(t, w, http.StatusOK)
	})
}

func TestHandleCreateUserDBError(t *testing.T) {
	msg, code := handleCreateUserDBError(fmt.Errorf("error 23503 foreign key violation"))
	if code != http.StatusBadRequest || msg != errCompanyDeptNotFound {
		t.Errorf("expected 400 with %s, got %d with %s", errCompanyDeptNotFound, code, msg)
	}

	msg, code = handleCreateUserDBError(fmt.Errorf("duplicate key value violates unique constraint on username"))
	if code != http.StatusConflict || msg != "username already exists" {
		t.Errorf("expected 409 username already exists, got %d with %s", code, msg)
	}

	msg, code = handleCreateUserDBError(fmt.Errorf("duplicate key value on email"))
	if code != http.StatusConflict || msg != "email already exists" {
		t.Errorf("expected 409 email already exists, got %d with %s", code, msg)
	}

	msg, code = handleCreateUserDBError(fmt.Errorf("generic db error"))
	if code != http.StatusConflict || msg != "username or email already exists" {
		t.Errorf("expected 409 username or email already exists, got %d with %s", code, msg)
	}
}

func TestUserHandlers_MasterDataResolutionAndEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)

	// 1. CreateUser error cases
	t.Run("CreateUser rejects invalid site_id or division_id", func(t *testing.T) {
		bodyInvalidSite := `{"username":"usr_bad_site","email":"bads@example.com","role":"user","site_id":999999}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyInvalidSite)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusBadRequest)

		bodyInvalidDiv := `{"username":"usr_bad_div","email":"badd@example.com","role":"user","division_id":999999}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyInvalidDiv)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusBadRequest)

		bodyInvalidDept := `{"username":"usr_bad_dept","email":"baddept@example.com","role":"user","department_id":999999}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyInvalidDept)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusBadRequest)

		bodyInvalidComp := `{"username":"usr_bad_comp","email":"badcomp@example.com","role":"user","company_id":999999}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyInvalidComp)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusBadRequest)
	})

	t.Run("CreateUser resolves site and division by string name", func(t *testing.T) {
		site := models.Site{Code: "sby_hub", Name: "Surabaya Hub", IsActive: true}
		_ = tx.Create(&site)
		div := models.Division{Code: "fintech_div", Name: "Fintech Division", IsActive: true}
		_ = tx.Create(&div)

		// With matching names
		bodyMatch := `{"username":"usr_by_name","email":"byname@example.com","role":"user","site":"Surabaya Hub","division":"Fintech Division"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyMatch)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		// With non-matching names
		bodyNoMatch := `{"username":"usr_nomatch","email":"nomatch@example.com","role":"user","site":"Custom Site","division":"Custom Div"}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyNoMatch)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusCreated)

		// Duplicate username
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyMatch)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusConflict)

		// Duplicate email
		bodyDupEmail := `{"username":"usr_other_name","email":"byname@example.com","role":"user"}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyDupEmail)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.CreateUser(c)
		assertFatalCode(t, w, http.StatusConflict)
	})

	t.Run("UpdateUser master data resolution and clearing", func(t *testing.T) {
		site := models.Site{Code: "site_upd", Name: "Site Upd", IsActive: true}
		_ = tx.Create(&site)
		div := models.Division{Code: "div_upd", Name: "Div Upd", IsActive: true}
		_ = tx.Create(&div)
		dept := models.Department{Code: "dept_upd", Name: "Dept Upd", DivisionID: &div.ID, Division: div.Name, IsActive: true}
		_ = tx.Create(&dept)
		comp := models.Company{Code: "comp_upd", Name: "Comp Upd", IsActive: true}
		_ = tx.Create(&comp)

		target := models.User{Username: "usr_upd_target", Email: "targetupd@example.com", Role: models.RoleUser, IsActive: true}
		_ = tx.Create(&target)
		admin := models.User{Username: "admin_upd", Email: "adminupd@example.com", Role: models.RoleAdmin, IsActive: true}
		_ = tx.Create(&admin)

		targetIDStr := fmt.Sprintf("%d", target.ID)

		// Update with invalid foreign keys -> 400
		for _, badPayload := range []string{
			`{"department_id": 999999}`,
			`{"company_id": 999999}`,
			`{"site_id": 999999}`,
			`{"division_id": 999999}`,
		} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
			c.Set(ctxUserID, admin.ID)
			c.Set(ctxRole, models.RoleAdmin)
			c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+targetIDStr, bytes.NewReader([]byte(badPayload)))
			c.Request.Header.Set("Content-Type", "application/json")
			srv.UpdateUser(c)
			assertFatalCode(t, w, http.StatusBadRequest)
		}

		// Update with string name matching
		goodNamePayload := `{"site": "Site Upd", "division": "Div Upd", "company": "Comp Upd"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+targetIDStr, bytes.NewReader([]byte(goodNamePayload)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)

		// Update with string name non-matching for site and division
		nomatchPayload := `{"site": "Custom Site 2", "division": "Custom Div 2"}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+targetIDStr, bytes.NewReader([]byte(nomatchPayload)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)

		// Update with non-matching company name returns 400
		badCompNamePayload := `{"company": "Custom Comp 2"}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+targetIDStr, bytes.NewReader([]byte(badCompNamePayload)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusBadRequest)

		// Clear foreign keys with 0 or empty strings
		clearPayload := `{"department_id": 0, "company_id": 0, "site_id": 0, "division_id": 0, "site": "", "division": "", "company": ""}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: targetIDStr}}
		c.Set(ctxUserID, admin.ID)
		c.Set(ctxRole, models.RoleAdmin)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+targetIDStr, bytes.NewReader([]byte(clearPayload)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)
	})

	t.Run("applyApprovedProfileChange covers admin and edge cases", func(t *testing.T) {
		adminTarget := models.User{Username: "admin_pc_target", Email: "adminpc@example.com", Role: models.RoleAdmin, IsActive: true}
		_ = tx.Create(&adminTarget)

		comp := models.Company{Code: "pc_comp_edge", Name: "PC Comp Edge", IsActive: true}
		_ = tx.Create(&comp)
		site := models.Site{Code: "pc_site_edge", Name: "PC Site Edge", IsActive: true}
		_ = tx.Create(&site)
		div := models.Division{Code: "pc_div_edge", Name: "PC Div Edge", IsActive: true}
		_ = tx.Create(&div)
		dept := models.Department{Code: "pc_dept_edge", Name: "PC Dept Edge", DivisionID: &div.ID, Division: div.Name, IsActive: true}
		_ = tx.Create(&dept)

		// Approving change for admin user clears company
		changeAdmin := models.ProfileChangeRequest{
			UserID:       adminTarget.ID,
			Name:         "Admin New Name",
			BniID:        "BNI-999",
			EmployeeID:   "EMP-ADMIN",
			SiteID:       &site.ID,
			Site:         site.Name,
			DivisionID:   &div.ID,
			Division:     div.Name,
			DepartmentID: &dept.ID,
			CompanyID:    &comp.ID,
			Status:       models.ProfilePending,
		}
		_ = tx.Create(&changeAdmin)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", changeAdmin.ID)}}
		c.Set(ctxUserID, adminTarget.ID)
		c.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/profile-changes/%d/review?action=approve", changeAdmin.ID), nil)
		srv.ReviewProfileChange(c)
		assertFatalCode(t, w, http.StatusOK)

		var reloadedAdmin models.User
		_ = tx.Where("id = ?", adminTarget.ID).First(&reloadedAdmin)
		if reloadedAdmin.CompanyID != nil || reloadedAdmin.Company != "" {
			t.Errorf("expected admin company to be empty, got %v / %q", reloadedAdmin.CompanyID, reloadedAdmin.Company)
		}

		// Non-admin user with string department (no department_id)
		regUser := models.User{Username: "reg_pc_target", Email: "regpc@example.com", Role: models.RoleUser, IsActive: true}
		_ = tx.Create(&regUser)

		changeReg := models.ProfileChangeRequest{
			UserID:     regUser.ID,
			Name:       "Reg New Name",
			Department: "Custom Dept String",
			CompanyID:  &comp.ID,
			Status:     models.ProfilePending,
		}
		_ = tx.Create(&changeReg)

		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", changeReg.ID)}}
		c.Set(ctxUserID, adminTarget.ID)
		c.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/profile-changes/%d/review?action=approve", changeReg.ID), nil)
		srv.ReviewProfileChange(c)
		assertFatalCode(t, w, http.StatusOK)

		var reloadedReg models.User
		_ = tx.Where("id = ?", regUser.ID).First(&reloadedReg)
		if reloadedReg.Department != "Custom Dept String" {
			t.Errorf("expected department 'Custom Dept String', got %q", reloadedReg.Department)
		}
		if reloadedReg.CompanyID == nil || *reloadedReg.CompanyID != comp.ID {
			t.Errorf("expected company_id %d, got %v", comp.ID, reloadedReg.CompanyID)
		}
	})
}
