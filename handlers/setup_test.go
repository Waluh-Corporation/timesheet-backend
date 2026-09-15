package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/database"
	"timesheet-backend/models"
)

func setupTestDB(t *testing.T) (*gorm.DB, *config.Config) {
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}
	return db, cfg
}

func assertResponseCode(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("expected status %d, got %d (body: %s)", expected, w.Code, w.Body.String())
	}
}

func assertFatalCode(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Fatalf("expected status %d, got %d (body: %s)", expected, w.Code, w.Body.String())
	}
}

func TestSetupHandlers_InitWeakPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()
	_ = tx.Exec("DELETE FROM users WHERE role = 'admin'").Error
	_ = tx.Save(&models.SystemSetting{Key: "is_new", Value: "Y"}).Error

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Auth: authSvc,
	}

	payload := InitSetupRequest{
		Admin: InitSetupAdminRequest{
			Username: "newadmin",
			Email:    "newadmin@example.com",
			Name:     "Admin Name",
			Password: "weak", // < 8 characters
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/setup/init", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.InitSetup(c)
	assertResponseCode(t, w, http.StatusBadRequest)
}

func TestSetupHandlers_InitSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()
	_ = tx.Exec("DELETE FROM users WHERE role = 'admin'").Error
	_ = tx.Save(&models.SystemSetting{Key: "is_new", Value: "Y"}).Error

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Auth: authSvc,
	}

	payload := InitSetupRequest{
		Admin: InitSetupAdminRequest{
			Username: "superadmin",
			Email:    "superadmin@example.com",
			Name:     "Super Admin",
			Password: "SuperSecurePassword123!",
		},
		Companies: []InitSetupCompanyRequest{
			{Code: "testcorp", Name: "Test Corporation"},
		},
		Departments: []InitSetupDepartmentRequest{
			{
				Code:        "ENG",
				Name:        "Engineering",
				Division:    "Technology",
				CompanyCode: "testcorp",
			},
		},
		Approvers: []InitSetupApproverRequest{
			{
				Name:        "Test Approver TL",
				RoleType:    models.ApproverRoleTeamLeader,
				Title:       "Team Leader",
				CompanyCode: "testcorp",
			},
			{
				Name:        "Test Approver DH",
				RoleType:    models.ApproverRoleDepartmentHead,
				Title:       "Department Head",
				CompanyCode: "testcorp",
			},
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/setup/init", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.InitSetup(c)
	assertFatalCode(t, w, http.StatusOK)

	// Second attempt should fail with StatusForbidden (system already initialized)
	wReinit := httptest.NewRecorder()
	cReinit, _ := gin.CreateTestContext(wReinit)
	cReinit.Request = httptest.NewRequest("POST", "/api/v1/setup/init", bytes.NewReader(body))
	cReinit.Request.Header.Set("Content-Type", "application/json")
	srv.InitSetup(cReinit)
	assertResponseCode(t, wReinit, http.StatusForbidden)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Token   string      `json:"token"`
			Message string      `json:"message"`
			User    models.User `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.Token == "" {
		t.Error("expected non-empty auth token on successful setup")
	}
	if resp.Data.User.Role != models.RoleAdmin {
		t.Errorf("expected role 'admin', got %s", resp.Data.User.Role)
	}

	var setting models.SystemSetting
	if err := tx.Where("key = ?", "is_new").First(&setting).Error; err != nil {
		t.Errorf("expected is_new setting in system_settings: %v", err)
	} else if setting.Value != "N" {
		t.Errorf("expected is_new = 'N' after setup, got '%s'", setting.Value)
	}
}

func TestSetupHandlers_GetSetupStatusIsNew(t *testing.T) {
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

	// Set is_new = Y
	_ = tx.Save(&models.SystemSetting{Key: "is_new", Value: "Y"}).Error
	_ = tx.Exec("DELETE FROM users WHERE role = 'admin'").Error

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	srv.GetSetupStatus(c)

	var resp struct {
		Data SetupStatusResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.IsNew != "Y" || !resp.Data.RequiresSetup || resp.Data.IsInitialized {
		t.Errorf("expected IsNew=Y, RequiresSetup=true, IsInitialized=false; got %+v", resp.Data)
	}

	// Set is_new = N
	_ = tx.Save(&models.SystemSetting{Key: "is_new", Value: "N"}).Error
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	srv.GetSetupStatus(c2)

	var resp2 struct {
		Data SetupStatusResponse `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Data.IsNew != "N" || resp2.Data.RequiresSetup || !resp2.Data.IsInitialized {
		t.Errorf("expected IsNew=N, RequiresSetup=false, IsInitialized=true; got %+v", resp2.Data)
	}
}

func TestSetupHandlers_GetSetupStatusAdminExistence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   db,
		Cfg:  cfg,
		Auth: authSvc,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/setup/status", nil)

	srv.GetSetupStatus(c)
	assertFatalCode(t, w, http.StatusOK)

	var resp struct {
		Code int                 `json:"code"`
		Data SetupStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	var adminCount int64
	_ = db.Model(&models.User{}).Where("role = ? AND deleted_at IS NULL", models.RoleAdmin).Count(&adminCount).Error

	if resp.Data.IsInitialized != (adminCount > 0) {
		t.Errorf("expected IsInitialized = %v, got %v", adminCount > 0, resp.Data.IsInitialized)
	}
	if resp.Data.RequiresSetup != (adminCount == 0) {
		t.Errorf("expected RequiresSetup = %v, got %v", adminCount == 0, resp.Data.RequiresSetup)
	}
}

func TestSetupHandlers_InitAdminAlreadyExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	var adminCount int64
	_ = db.Model(&models.User{}).Where("role = ? AND deleted_at IS NULL", models.RoleAdmin).Count(&adminCount).Error
	if adminCount == 0 {
		t.Skip("skipping already-initialized test because no admin currently in test DB")
	}

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   db,
		Cfg:  cfg,
		Auth: authSvc,
	}

	payload := InitSetupRequest{
		Admin: InitSetupAdminRequest{
			Username: "anotheradmin",
			Email:    "another@example.com",
			Name:     "Another Admin",
			Password: "SuperSecurePass2026!",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/setup/init", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	srv.InitSetup(c)
	assertResponseCode(t, w, http.StatusForbidden)
}

func TestSetupHandlers_SeedStatuses(t *testing.T) {
	db, _ := setupTestDB(t)
	if err := database.SeedActivityStatuses(db); err != nil {
		t.Fatalf("SeedActivityStatuses failed: %v", err)
	}

	for _, code := range []string{"P", "BT", "S", "PM", "V", "X"} {
		var cnt int64
		_ = db.Model(&models.ActivityStatus{}).Where("code = ?", code).Count(&cnt).Error
		if cnt == 0 {
			t.Errorf("activity status %s was not seeded", code)
		}
	}
}

func TestSetupHandlers_EdgeCases(t *testing.T) {
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

	// 1. Delete is_new setting but insert an admin user to trigger fallback in GetSetupStatus & isSystemAlreadyInitialized
	_ = tx.Exec("DELETE FROM system_settings WHERE key = 'is_new'").Error
	adminUser := models.User{
		Username:     "tempadmin_edge",
		Email:        "tempadmin_edge@example.com",
		Role:         models.RoleAdmin,
		IsActive:     true,
		PasswordHash: "dummyhash",
	}
	_ = tx.Create(&adminUser).Error

	wStatus := httptest.NewRecorder()
	cStatus, _ := gin.CreateTestContext(wStatus)
	cStatus.Request = httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	srv.GetSetupStatus(cStatus)
	assertResponseCode(t, wStatus, http.StatusOK)

	// Init setup should be rejected because system is initialized via admin fallback
	wInitReject := httptest.NewRecorder()
	cInitReject, _ := gin.CreateTestContext(wInitReject)
	cInitReject.Request = httptest.NewRequest("POST", "/api/v1/setup/init", bytes.NewReader([]byte("{}")))
	cInitReject.Request.Header.Set("Content-Type", "application/json")
	srv.InitSetup(cInitReject)
	assertResponseCode(t, wInitReject, http.StatusForbidden)

	// 2. Clear admin users and set is_new to Y, test invalid JSON
	_ = tx.Exec("DELETE FROM users WHERE role = 'admin'").Error
	_ = tx.Save(&models.SystemSetting{Key: "is_new", Value: "Y"}).Error

	wBadJSON := httptest.NewRecorder()
	cBadJSON, _ := gin.CreateTestContext(wBadJSON)
	cBadJSON.Request = httptest.NewRequest("POST", "/api/v1/setup/init", bytes.NewReader([]byte("invalid json")))
	cBadJSON.Request.Header.Set("Content-Type", "application/json")
	srv.InitSetup(cBadJSON)
	assertResponseCode(t, wBadJSON, http.StatusBadRequest)

	// 3. Test empty company code, empty department code, empty approver name
	payload := InitSetupRequest{
		Admin: InitSetupAdminRequest{
			Username: "superadmin_empty_items",
			Email:    "superadmin_empty_items@example.com",
			Name:     "Super Admin",
			Password: "SuperSecurePassword123!",
		},
		Companies: []InitSetupCompanyRequest{
			{Code: "", Name: "No Code Corp"},
			{Code: "valid_corp", Name: "Valid Corp"},
		},
		Departments: []InitSetupDepartmentRequest{
			{Code: "", Name: "No Code Dept"},
			{Code: "DEV", Name: "Development", CompanyCode: "valid_corp"},
		},
		Approvers: []InitSetupApproverRequest{
			{Name: "", RoleType: models.ApproverRoleTeamLeader},
			{Name: "Valid Approver", RoleType: models.ApproverRoleDepartmentHead},
		},
	}
	body, _ := json.Marshal(payload)
	wEmptyItems := httptest.NewRecorder()
	cEmptyItems, _ := gin.CreateTestContext(wEmptyItems)
	cEmptyItems.Request = httptest.NewRequest("POST", "/api/v1/setup/init", bytes.NewReader(body))
	cEmptyItems.Request.Header.Set("Content-Type", "application/json")
	srv.InitSetup(cEmptyItems)
	assertResponseCode(t, wEmptyItems, http.StatusOK)
}
