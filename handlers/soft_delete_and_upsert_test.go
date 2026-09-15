package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

func TestSoftDelete_And_Upsert_Suite(t *testing.T) {
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

	// Setup master test data
	comp := models.Company{Code: "ACME", Name: "Acme Corporation", IsActive: true}
	_ = tx.Create(&comp)

	dept := models.Department{Code: "DEV", Name: "Software Development", Division: "Technology", IsActive: true}
	_ = tx.Create(&dept)

	user := models.User{
		Username:     "test_dev_user",
		Email:        "dev@example.com",
		Role:         models.RoleUser,
		CompanyID:    &comp.ID,
		Company:      comp.Name,
		DepartmentID: &dept.ID,
		Department:   dept.Name,
		IsActive:     true,
	}
	_ = tx.Create(&user)

	admin := models.User{
		Username: "admin_tester",
		Email:    "adm@example.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	_ = tx.Create(&admin)

	t.Run("CreateUser pre-check rejects inactive or nonexistent company/department", func(t *testing.T) {
		// Nonexistent company
		bodyNonExistent := `{"username": "bad_comp_user", "email": "badc@example.com", "password": "Password123!", "company_id": 999999}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyNonExistent)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, admin.ID)
		srv.CreateUser(c)
		assertResponseCode(t, w, http.StatusBadRequest)

		// Inactive company
		inactiveComp := models.Company{Code: "INACT", Name: "Inactive Corp", IsActive: false}
		_ = tx.Create(&inactiveComp)
		bodyInactive := fmt.Sprintf(`{"username": "inact_comp_user", "email": "inactc@example.com", "password": "Password123!", "company_id": %d}`, inactiveComp.ID)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyInactive)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, admin.ID)
		srv.CreateUser(c)
		assertResponseCode(t, w, http.StatusBadRequest)

		// Inactive department
		inactiveDept := models.Department{Code: "INACTD", Name: "Inactive Dept", Division: "None", IsActive: false}
		_ = tx.Create(&inactiveDept)
		bodyInactiveDept := fmt.Sprintf(`{"username": "inact_dept_user", "email": "inactd@example.com", "password": "Password123!", "company_id": %d, "department_id": %d}`, comp.ID, inactiveDept.ID)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(bodyInactiveDept)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, admin.ID)
		srv.CreateUser(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("CreateUser synchronizes company and department names correctly", func(t *testing.T) {
		body := fmt.Sprintf(`{"username": "valid_synced_user", "email": "synced@example.com", "password": "Password123!", "role": "user", "company_id": %d, "department_id": %d}`, comp.ID, dept.ID)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, admin.ID)
		srv.CreateUser(c)
		assertResponseCode(t, w, http.StatusCreated)

		var created models.User
		if err := tx.Where("username = ?", "valid_synced_user").First(&created).Error; err != nil {
			t.Fatalf("failed to find created user: %v", err)
		}
		if created.Company != comp.Name {
			t.Errorf("expected company name %q, got %q", comp.Name, created.Company)
		}
		if created.Department != dept.Name {
			t.Errorf("expected department name %q, got %q", dept.Name, created.Department)
		}
	})

	t.Run("Profile changes payload does not contain user key", func(t *testing.T) {
		// Submit change request
		submitBody := `{"name": "New Developer Name", "division": "Technology"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/change", bytes.NewReader([]byte(submitBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.SubmitProfileChange(c)
		assertResponseCode(t, w, http.StatusCreated)

		// Get changes
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/profile/changes", nil)
		srv.MyProfileChanges(c)
		assertResponseCode(t, w, http.StatusOK)

		var rawResp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &rawResp); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}
		data, ok := rawResp["data"].([]interface{})
		if !ok || len(data) == 0 {
			t.Fatalf("expected non-empty data array in response, got %v", rawResp["data"])
		}
		firstItem := data[0].(map[string]interface{})
		if _, hasUser := firstItem["user"]; hasUser {
			t.Errorf("expected response not to contain 'user' key, but found: %v", firstItem["user"])
		}
	})

	t.Run("Overtime upsert, soft delete, and re-entry lifecycle", func(t *testing.T) {
		otDate := "2026-09-18"

		// 1. Initial Insert
		otBody1 := fmt.Sprintf(`{"date": "%s", "start_time": "18:00", "end_time": "20:00", "task_description": "Initial overtime"}`, otDate)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/overtimes", bytes.NewReader([]byte(otBody1)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpsertOvertime(c)
		assertResponseCode(t, w, http.StatusOK)

		var ot1 models.OvertimeEntry
		parsedDate, _ := time.Parse("2006-01-02", otDate)
		if err := tx.Where("user_id = ? AND date = ? AND is_active = true", user.ID, parsedDate).First(&ot1).Error; err != nil {
			t.Fatalf("failed to find inserted overtime: %v", err)
		}
		if ot1.TaskDescription != "Initial overtime" {
			t.Errorf("expected 'Initial overtime', got %q", ot1.TaskDescription)
		}

		// 2. Edit Overtime (Upsert with same date, no ID) -> Updates existing active record
		otBodyUpdate := fmt.Sprintf(`{"date": "%s", "start_time": "18:00", "end_time": "21:00", "task_description": "Updated overtime"}`, otDate)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/overtimes", bytes.NewReader([]byte(otBodyUpdate)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpsertOvertime(c)
		assertResponseCode(t, w, http.StatusOK)

		var otCount int64
		_ = tx.Model(&models.OvertimeEntry{}).Where("user_id = ? AND date = ?", user.ID, parsedDate).Count(&otCount)
		if otCount != 1 {
			t.Errorf("expected exactly 1 overtime record after update, got %d", otCount)
		}

		var otUpdated models.OvertimeEntry
		_ = tx.Where("user_id = ? AND date = ? AND is_active = true", user.ID, parsedDate).First(&otUpdated)
		if otUpdated.ID != ot1.ID || otUpdated.TaskDescription != "Updated overtime" {
			t.Errorf("expected record ID %d with updated description, got ID %d description %q", ot1.ID, otUpdated.ID, otUpdated.TaskDescription)
		}

		// 3. Soft Delete Overtime
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", ot1.ID)}}
		c.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/overtimes/%d", ot1.ID), nil)
		srv.DeleteOvertime(c)
		assertResponseCode(t, w, http.StatusOK)

		// Verify record still exists in DB but is_active = false
		var otDeleted models.OvertimeEntry
		if err := tx.Where("id = ?", ot1.ID).First(&otDeleted).Error; err != nil {
			t.Fatalf("record should not be hard deleted: %v", err)
		}
		if otDeleted.IsActive {
			t.Errorf("expected is_active = false after delete, got true")
		}

		// Verify ListMonthlyOvertimes does NOT return the soft-deleted overtime
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/overtimes?year=2026&month=9", nil)
		srv.ListMonthlyOvertimes(c)
		assertResponseCode(t, w, http.StatusOK)

		var listResp response.APIResponse
		_ = json.Unmarshal(w.Body.Bytes(), &listResp)
		items, _ := json.Marshal(listResp.Data)
		var otList []response.OvertimeResponse
		_ = json.Unmarshal(items, &otList)
		for _, item := range otList {
			if item.ID == ot1.ID {
				t.Errorf("soft-deleted overtime %d should not appear in ListMonthlyOvertimes", ot1.ID)
			}
		}

		// 4. Re-input on soft-deleted date -> MUST create a NEW record (Insert), NOT reactivate old record
		otBodyReinput := fmt.Sprintf(`{"date": "%s", "start_time": "19:00", "end_time": "22:00", "task_description": "New overtime after deletion"}`, otDate)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/overtimes", bytes.NewReader([]byte(otBodyReinput)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpsertOvertime(c)
		assertResponseCode(t, w, http.StatusOK)

		var allRecords []models.OvertimeEntry
		_ = tx.Where("user_id = ? AND date = ?", user.ID, parsedDate).Order("id asc").Find(&allRecords)
		if len(allRecords) != 2 {
			t.Fatalf("expected exactly 2 records in DB (1 inactive, 1 new active), got %d", len(allRecords))
		}

		if allRecords[0].ID != ot1.ID || allRecords[0].IsActive != false {
			t.Errorf("original record should remain inactive: ID=%d, IsActive=%v", allRecords[0].ID, allRecords[0].IsActive)
		}
		if allRecords[1].ID == ot1.ID || allRecords[1].IsActive != true || allRecords[1].TaskDescription != "New overtime after deletion" {
			t.Errorf("second record should be brand new and active: ID=%d, IsActive=%v, Desc=%q", allRecords[1].ID, allRecords[1].IsActive, allRecords[1].TaskDescription)
		}
	})

	t.Run("DailyActivity soft delete and re-entry lifecycle", func(t *testing.T) {
		actDate := "2026-09-19"
		parsedDate, _ := time.Parse("2006-01-02", actDate)

		// 1. Insert activity
		actBody1 := fmt.Sprintf(`{"date": "%s", "start_time": "08:30", "end_time": "17:30", "status": "P", "activity": "First task"}`, actDate)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewReader([]byte(actBody1)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpsertDailyActivity(c)
		assertResponseCode(t, w, http.StatusOK)

		var act1 models.DailyActivity
		_ = tx.Where("user_id = ? AND date = ? AND is_active = true", user.ID, parsedDate).First(&act1)

		// 2. Soft-delete activity
		_ = tx.Model(&act1).Update("is_active", false)

		// Verify ListActivities ignores it
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/activities?start_date=2026-09-19&end_date=2026-09-19", nil)
		srv.ListActivities(c)
		assertResponseCode(t, w, http.StatusOK)

		var pagResp response.PaginatedResponse
		_ = json.Unmarshal(w.Body.Bytes(), &pagResp)
		if pagResp.Pagination.TotalRows != 0 {
			t.Errorf("expected 0 active activities in list, got %d", pagResp.Pagination.TotalRows)
		}

		// 3. Re-input on same date -> creates NEW record
		actBody2 := fmt.Sprintf(`{"date": "%s", "start_time": "09:00", "end_time": "18:00", "status": "P", "activity": "Second task after soft delete"}`, actDate)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Set(ctxUserID, user.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewReader([]byte(actBody2)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpsertDailyActivity(c)
		assertResponseCode(t, w, http.StatusOK)

		var activitiesOnDate []models.DailyActivity
		_ = tx.Where("user_id = ? AND date = ?", user.ID, parsedDate).Order("id asc").Find(&activitiesOnDate)
		if len(activitiesOnDate) != 2 {
			t.Fatalf("expected 2 activities (1 inactive, 1 active), got %d", len(activitiesOnDate))
		}
		if activitiesOnDate[0].ID != act1.ID || activitiesOnDate[0].IsActive != false {
			t.Errorf("first activity should remain inactive")
		}
		if activitiesOnDate[1].ID == act1.ID || activitiesOnDate[1].IsActive != true {
			t.Errorf("second activity should be new and active")
		}
	})

	t.Run("Master Companies and Approvers updates execute UPDATE without inserting new records", func(t *testing.T) {
		// Update Company
		updateCompBody := `{"name": "Acme Global Solutions"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", comp.ID)}}
		c.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/companies/%d", comp.ID), bytes.NewReader([]byte(updateCompBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpdateCompany(c)
		assertResponseCode(t, w, http.StatusOK)

		var compAfter models.Company
		_ = tx.Where("id = ?", comp.ID).First(&compAfter)
		if compAfter.Name != "Acme Global Solutions" {
			t.Errorf("expected updated name, got %q", compAfter.Name)
		}

		// Update Approver
		appr := models.Approver{Name: "Original Lead", RoleType: models.ApproverRoleTeamLeader, Title: "TL", IsActive: true}
		_ = tx.Create(&appr)

		updateApprBody := `{"name": "Promoted Lead", "title": "Senior TL"}`
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", appr.ID)}}
		c.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/approvers/%d", appr.ID), bytes.NewReader([]byte(updateApprBody)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.UpdateApprover(c)
		assertResponseCode(t, w, http.StatusOK)

		var apprAfter models.Approver
		_ = tx.Where("id = ?", appr.ID).First(&apprAfter)
		if apprAfter.Name != "Promoted Lead" || apprAfter.Title != "Senior TL" {
			t.Errorf("expected updated approver name and title, got %q and %q", apprAfter.Name, apprAfter.Title)
		}
	})

	t.Run("ActiveOnly and FilterActiveTable scopes filter out inactive records", func(t *testing.T) {
		pActive := models.Project{Code: "ACT-TEST-01", Name: "Active Scope Proj", IsActive: true}
		pInactive := models.Project{Code: "INACT-TEST-02", Name: "Inactive Scope Proj", IsActive: true}
		_ = tx.Create(&pActive)
		_ = tx.Create(&pInactive)
		_ = tx.Model(&pInactive).Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now(),
		})

		var activeProjects []models.Project
		err := tx.Scopes(models.ActiveOnly).Where("code IN ?", []string{"ACT-TEST-01", "INACT-TEST-02"}).Find(&activeProjects).Error
		if err != nil {
			t.Fatalf("failed to query with ActiveOnly: %v", err)
		}
		if len(activeProjects) != 1 || activeProjects[0].Code != "ACT-TEST-01" {
			t.Errorf("expected only 1 active project ACT-TEST-01, got %v", activeProjects)
		}

		var activeProjectsTable []models.Project
		err = tx.Scopes(models.FilterActiveTable("projects")).Where("projects.code IN ?", []string{"ACT-TEST-01", "INACT-TEST-02"}).Find(&activeProjectsTable).Error
		if err != nil {
			t.Fatalf("failed to query with FilterActiveTable: %v", err)
		}
		if len(activeProjectsTable) != 1 || activeProjectsTable[0].Code != "ACT-TEST-01" {
			t.Errorf("expected only 1 active project with table filter, got %v", activeProjectsTable)
		}
	})
}
