package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/mailer"
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

	t.Run("CreateUser rejects weak password", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"username":"testweak","email":"testweak@example.com","name":"Weak","password":"123","role":"user"}`
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
	if err := srv.DB.Where("user_id = ?", targetUser.ID).Order("id desc").First(&lastReq).Error; err != nil {
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
	srv.DB.First(&updatedTarget, targetUser.ID)
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
	if err := srv.DB.Where("user_id = ?", targetUser.ID).Order("id desc").First(&lastReq2).Error; err != nil {
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

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:     tx,
		Cfg:    cfg,
		Auth:   authSvc,
		Mailer: mailer.New(cfg),
	}

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
		if err := srv.DB.Where("username = ?", "new_created_user").First(&createdUser).Error; err != nil {
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
