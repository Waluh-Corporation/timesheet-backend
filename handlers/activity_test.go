package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/models"
)

func assertPaginationResponse(t *testing.T, code int, status string, dataLen int, actual models.PaginationMeta, expected models.PaginationMeta) {
	t.Helper()
	if code != 200 {
		t.Errorf("expected code 200, got %d", code)
	}
	if status != "success" {
		t.Errorf("expected status 'success', got %s", status)
	}
	if dataLen != 2 {
		t.Errorf("expected 2 items, got %d", dataLen)
	}
	if actual.Page != expected.Page {
		t.Errorf("expected page %d, got %d", expected.Page, actual.Page)
	}
	if actual.Limit != expected.Limit {
		t.Errorf("expected limit %d, got %d", expected.Limit, actual.Limit)
	}
	if actual.TotalRows != expected.TotalRows {
		t.Errorf("expected total_rows %d, got %d", expected.TotalRows, actual.TotalRows)
	}
	if actual.TotalPages != expected.TotalPages {
		t.Errorf("expected total_pages %d, got %d", expected.TotalPages, actual.TotalPages)
	}
}

func TestRespondPaginatedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RespondPaginated returns valid envelope with metadata", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		items := []map[string]string{
			{"title": "Task 1"},
			{"title": "Task 2"},
		}

		RespondPaginated(c, http.StatusOK, items, 2, 10, 35)
		assertFatalCode(t, w, http.StatusOK)

		var resp struct {
			Code       int                   `json:"code"`
			Status     string                `json:"status"`
			Data       []map[string]string   `json:"data"`
			Pagination models.PaginationMeta `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal JSON response: %v", err)
		}

		assertPaginationResponse(t, resp.Code, resp.Status, len(resp.Data), resp.Pagination, models.PaginationMeta{
			Page:       2,
			Limit:      10,
			TotalRows:  35,
			TotalPages: 4,
		})
	})

	t.Run("RespondPaginated handles zero items", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		items := []models.DailyActivity{}
		RespondPaginated(c, http.StatusOK, items, 1, 10, 0)

		var resp struct {
			Pagination models.PaginationMeta `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}
		if resp.Pagination.TotalPages != 0 {
			t.Errorf("expected total_pages 0 for 0 items, got %d", resp.Pagination.TotalPages)
		}
	})
}

func TestGetDailyActivity_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		idParam    string
		expectCode int
	}{
		{"invalid string ID", "abc", http.StatusBadRequest},
		{"zero ID", "0", http.StatusBadRequest},
		{"negative ID", "-5", http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: tc.idParam}}

			srv := &Server{}
			srv.GetDailyActivity(c)
			assertResponseCode(t, w, tc.expectCode)
		})
	}
}

func assertListActivitiesPagination(t *testing.T, dataLen int, meta models.PaginationMeta) {
	t.Helper()
	if dataLen != 2 {
		t.Errorf("expected 2 items for limit=2, got %d", dataLen)
	}
	if meta.Page != 1 {
		t.Errorf("expected page 1, got %d", meta.Page)
	}
	if meta.Limit != 2 {
		t.Errorf("expected limit 2, got %d", meta.Limit)
	}
	if meta.TotalRows != 3 {
		t.Errorf("expected total_rows 3, got %d", meta.TotalRows)
	}
	if meta.TotalPages != 2 {
		t.Errorf("expected total_pages 2, got %d", meta.TotalPages)
	}
}

func TestActivityHandlers_GetAndListIntegration(t *testing.T) {
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Auth: authSvc,
	}

	user1 := models.User{
		Username: "activity_user1",
		Email:    "user1@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	user2 := models.User{
		Username: "activity_user2",
		Email:    "user2@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	adminUser := models.User{
		Username: "activity_admin",
		Email:    "admin@example.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	_ = tx.Create(&user1)
	_ = tx.Create(&user2)
	_ = tx.Create(&adminUser)

	loc, _ := time.LoadLocation("Asia/Jakarta")
	date1 := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	date2 := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)
	date3 := time.Date(2026, 9, 3, 0, 0, 0, 0, loc)

	act1 := models.DailyActivity{
		UserID:    user1.ID,
		Date:      date1,
		Status:    "P",
		Activity:  "Working on feature 1",
		StartTime: "08:00",
		EndTime:   "17:00",
	}
	act2 := models.DailyActivity{
		UserID:    user1.ID,
		Date:      date2,
		Status:    "P",
		Activity:  "Working on feature 2",
		StartTime: "08:00",
		EndTime:   "17:00",
	}
	act3 := models.DailyActivity{
		UserID:    user1.ID,
		Date:      date3,
		Status:    "BT",
		Activity:  "Business trip",
		StartTime: "09:00",
		EndTime:   "18:00",
	}
	_ = tx.Create(&act1)
	_ = tx.Create(&act2)
	_ = tx.Create(&act3)

	t.Run("GetDailyActivity returns activity for owner", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: itoa(act1.ID)}}
		c.Set(ctxUserID, user1.ID)
		c.Set(ctxRole, models.RoleUser)

		srv.GetDailyActivity(c)
		assertFatalCode(t, w, http.StatusOK)

		var resp struct {
			Code int                  `json:"code"`
			Data models.DailyActivity `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp.Data.ID != act1.ID {
			t.Errorf("expected activity ID %d, got %d", act1.ID, resp.Data.ID)
		}
		if resp.Data.Activity != "Working on feature 1" {
			t.Errorf("unexpected activity text: %s", resp.Data.Activity)
		}
	})

	t.Run("GetDailyActivity forbids other user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: itoa(act1.ID)}}
		c.Set(ctxUserID, user2.ID)
		c.Set(ctxRole, models.RoleUser)

		srv.GetDailyActivity(c)
		assertFatalCode(t, w, http.StatusForbidden)
		if !strings.Contains(w.Body.String(), "you are not authorized to access another user's activity") {
			t.Fatalf("expected forbidden message, got %s", w.Body.String())
		}
	})

	t.Run("GetDailyActivity forbids admin from viewing another user's activity", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: itoa(act1.ID)}}
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.GetDailyActivity(c)
		assertFatalCode(t, w, http.StatusForbidden)
		if !strings.Contains(w.Body.String(), "you are not authorized to access another user's activity") {
			t.Fatalf("expected forbidden message for admin, got %s", w.Body.String())
		}
	})

	t.Run("ListActivities with pagination", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/activities?page=1&limit=2", nil)
		c.Set(ctxUserID, user1.ID)
		c.Set(ctxRole, models.RoleUser)

		srv.ListActivities(c)
		assertFatalCode(t, w, http.StatusOK)

		var resp struct {
			Code       int                    `json:"code"`
			Status     string                 `json:"status"`
			Data       []models.DailyActivity `json:"data"`
			Pagination models.PaginationMeta  `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		assertListActivitiesPagination(t, len(resp.Data), resp.Pagination)
	})
}

func assertProjectSyncFields(t *testing.T, act models.DailyActivity, proj models.Project) {
	t.Helper()
	if act.ProjectRefID == nil || *act.ProjectRefID != proj.ID {
		t.Errorf("expected ProjectRefID %d, got %v", proj.ID, act.ProjectRefID)
	}
	if act.ProjectID != proj.Code {
		t.Errorf("expected ProjectID %q, got %q", proj.Code, act.ProjectID)
	}
	if act.ProjectName != proj.Name {
		t.Errorf("expected ProjectName %q, got %q", proj.Name, act.ProjectName)
	}
	if act.GetProjectCode() != proj.Code {
		t.Errorf("expected GetProjectCode() %q, got %q", proj.Code, act.GetProjectCode())
	}
	if act.GetProjectName() != proj.Name {
		t.Errorf("expected GetProjectName() %q, got %q", proj.Name, act.GetProjectName())
	}
	if act.GetAppImpacted() != proj.AppImpacted {
		t.Errorf("expected GetAppImpacted() %q, got %q", proj.AppImpacted, act.GetAppImpacted())
	}
}

func TestActivityHandlers_UpsertSyncIntegration(t *testing.T) {
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Auth: authSvc,
	}

	user1 := models.User{
		Username: "activity_sync_user",
		Email:    "sync_user@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	_ = tx.Create(&user1)

	testProj := models.Project{
		Code:        "P99001",
		Name:        "Master Project Alpha",
		AppImpacted: "Alpha Mobile App",
		IsActive:    true,
	}
	if err := tx.Create(&testProj).Error; err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	reqBody := `{"date":"2026-09-20","project_ref_id":` + itoa(testProj.ID) + `,"activity":"Developing Alpha feature","status":"P","start_time":"08:30","end_time":"17:30"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/activities", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxUserID, user1.ID)
	c.Set(ctxRole, models.RoleUser)

	srv.UpsertDailyActivity(c)
	assertFatalCode(t, w, http.StatusOK)

	var resp struct {
		Code int                  `json:"code"`
		Data models.DailyActivity `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	assertProjectSyncFields(t, resp.Data, testProj)
}

func itoa(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}

func TestActivityHandlers_MasterData(t *testing.T) {
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{
		DB:  tx,
		Cfg: cfg,
	}

	comp := models.Company{Code: "test_comp", Name: "Test Company Inc"}
	_ = tx.Create(&comp)

	dept := models.Department{CompanyID: &comp.ID, Name: "Engineering", IsActive: true}
	_ = tx.Create(&dept)

	proj := models.Project{Code: "P12345", Name: "Project Apollo", AppImpacted: "Apollo Core", IsActive: true}
	_ = tx.Create(&proj)

	approver := models.Approver{Name: "Leader John", RoleType: models.ApproverRoleTeamLeader, Title: "Team Lead", IsActive: true}
	_ = tx.Create(&approver)

	holidayDate, _ := time.Parse("2006-01-02", "2099-12-31")
	holiday := models.Holiday{Date: holidayDate, Description: "Future Holiday", IsCivic: true}
	_ = tx.Where("date = ?", holidayDate).FirstOrCreate(&holiday)

	t.Run("ListProjects returns active projects", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		srv.ListProjects(c)
		assertFatalCode(t, w, http.StatusOK)
	})

	t.Run("ListCompanies returns active companies", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
		srv.ListCompanies(c)
		assertFatalCode(t, w, http.StatusOK)
	})

	t.Run("ListDepartments with and without company_id filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/departments?company_id=%d", comp.ID), nil)
		srv.ListDepartments(c)
		assertFatalCode(t, w, http.StatusOK)

		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/departments", nil)
		srv.ListDepartments(c2)
		assertFatalCode(t, w2, http.StatusOK)
	})

	t.Run("ListActivityStatuses returns status codes", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/activity-statuses", nil)
		srv.ListActivityStatuses(c)
		assertFatalCode(t, w, http.StatusOK)
	})

	t.Run("ListApprovers with and without role filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/approvers?role_type=team_leader", nil)
		srv.ListApprovers(c)
		assertFatalCode(t, w, http.StatusOK)

		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/approvers", nil)
		srv.ListApprovers(c2)
		assertFatalCode(t, w2, http.StatusOK)
	})

	t.Run("ListHolidays with and without year filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/holidays/all?year=2099", nil)
		srv.ListHolidays(c)
		assertFatalCode(t, w, http.StatusOK)

		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/holidays/all", nil)
		srv.ListHolidays(c2)
		assertFatalCode(t, w2, http.StatusOK)
	})
}

func TestActivityHandlers_OvertimeAndHelpers(t *testing.T) {
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := &Server{
		DB:  tx,
		Cfg: cfg,
	}

	user := models.User{
		Username: "overtime_user_test",
		Email:    "overtime_test@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	_ = tx.Create(&user)

	t.Run("UpsertOvertime creates and updates entry", func(t *testing.T) {
		body := `{"date":"2026-09-15","start_time":"18:00","end_time":"21:00","task_description":"Deploying hotfix"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/overtimes", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, user.ID)

		srv.UpsertOvertime(c)
		assertFatalCode(t, w, http.StatusOK)

		var resp struct {
			Data models.OvertimeEntry `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		otID := resp.Data.ID

		// Update existing overtime
		updateBody := fmt.Sprintf(`{"id":%d,"date":"2026-09-15","start_time":"19:00","end_time":"22:00","task_description":"Updated overtime task"}`, otID)
		wUp := httptest.NewRecorder()
		cUp, _ := gin.CreateTestContext(wUp)
		cUp.Request = httptest.NewRequest(http.MethodPost, "/api/v1/overtimes", strings.NewReader(updateBody))
		cUp.Request.Header.Set("Content-Type", "application/json")
		cUp.Set(ctxUserID, user.ID)
		srv.UpsertOvertime(cUp)
		assertFatalCode(t, wUp, http.StatusOK)

		// List monthly overtimes
		wList := httptest.NewRecorder()
		cList, _ := gin.CreateTestContext(wList)
		cList.Request = httptest.NewRequest(http.MethodGet, "/api/v1/overtimes?year=2026&month=9", nil)
		cList.Set(ctxUserID, user.ID)
		srv.ListMonthlyOvertimes(cList)
		assertFatalCode(t, wList, http.StatusOK)

		// Delete overtime
		wDel := httptest.NewRecorder()
		cDel, _ := gin.CreateTestContext(wDel)
		cDel.Params = gin.Params{{Key: "id", Value: itoa(otID)}}
		cDel.Set(ctxUserID, user.ID)
		srv.DeleteOvertime(cDel)
		assertFatalCode(t, wDel, http.StatusOK)
	})

	t.Run("findProjectByRefID error on non-existent", func(t *testing.T) {
		_, err := srv.findProjectByRefID(999999)
		if err == nil {
			t.Error("expected error for non-existent project_ref_id, got nil")
		}
	})

	t.Run("buildProjectQuery coverage", func(t *testing.T) {
		q1 := srv.buildProjectQuery("10", "Project A")
		if q1 == nil {
			t.Error("expected valid query")
		}
		q2 := srv.buildProjectQuery("PRJ", "Project B")
		if q2 == nil {
			t.Error("expected valid query")
		}
		q3 := srv.buildProjectQuery("", "Project C")
		if q3 == nil {
			t.Error("expected valid query")
		}
		q4 := srv.buildProjectQuery("PRJ", "")
		if q4 == nil {
			t.Error("expected valid query")
		}
	})

	t.Run("Holiday helpers", func(t *testing.T) {
		dtos := []models.HolidayDTO{
			{Date: "2026-08-17", Description: "Independence Day", IsCivic: true},
			{Date: "2026-01-01", Description: "New Year", IsCivic: true},
		}
		filtered := filterHolidaysByMonth(dtos, 2026, 8)
		if len(filtered) != 1 {
			t.Errorf("expected 1 holiday for August, got %d", len(filtered))
		}

		cnt := saveYearlyHolidays(tx, dtos)
		if cnt != 2 {
			t.Errorf("expected 2 holidays saved, got %d", cnt)
		}

		dbFetched := fetchDBHolidays(tx, 2026, 8)
		if len(dbFetched) == 0 {
			t.Error("expected fetched holidays from DB, got 0")
		}
	})

	t.Run("GetHolidays and SyncHolidays endpoints", func(t *testing.T) {
		wGet := httptest.NewRecorder()
		cGet, _ := gin.CreateTestContext(wGet)
		cGet.Request = httptest.NewRequest(http.MethodGet, "/api/v1/holidays?year=2099&month=12", nil)
		srv.GetHolidays(cGet)
		assertFatalCode(t, wGet, http.StatusOK)

		wSync := httptest.NewRecorder()
		cSync, _ := gin.CreateTestContext(wSync)
		cSync.Request = httptest.NewRequest(http.MethodPost, "/api/v1/holidays/sync?year=2026", nil)
		srv.SyncHolidays(cSync)
		if wSync.Code != http.StatusOK && wSync.Code != http.StatusBadGateway {
			t.Errorf("expected 200 or 502 for SyncHolidays, got %d", wSync.Code)
		}
	})

	t.Run("ListMonthlyActivities returns month list", func(t *testing.T) {
		wList := httptest.NewRecorder()
		cList, _ := gin.CreateTestContext(wList)
		cList.Request = httptest.NewRequest(http.MethodGet, "/api/v1/activities/monthly?year=2026&month=9", nil)
		cList.Set(ctxUserID, user.ID)
		srv.ListMonthlyActivities(cList)
		assertFatalCode(t, wList, http.StatusOK)
	})

	t.Run("ListActivities filters and pagination matrix", func(t *testing.T) {
		urls := []string{
			"/api/v1/activities?all=true",
			"/api/v1/activities?limit=-1",
			"/api/v1/activities?limit=150",
			"/api/v1/activities?sort=desc",
			"/api/v1/activities?sort=asc",
			"/api/v1/activities?year=2026",
			"/api/v1/activities?start_date=2026-09-01&end_date=2026-09-30",
			"/api/v1/activities?status=Present",
		}
		for _, u := range urls {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, u, nil)
			c.Set(ctxUserID, user.ID)
			srv.ListActivities(c)
			assertFatalCode(t, w, http.StatusOK)
		}
	})

	t.Run("GenerateTimesheet validation and success", func(t *testing.T) {
		// 1. Invalid JSON body
		wBadJSON := httptest.NewRecorder()
		cBadJSON, _ := gin.CreateTestContext(wBadJSON)
		cBadJSON.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", strings.NewReader("invalid-json"))
		cBadJSON.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(cBadJSON)
		assertFatalCode(t, wBadJSON, http.StatusBadRequest)

		// 2. User not found
		validBody := `{"year":2026,"month":9}`
		wNotFound := httptest.NewRecorder()
		cNotFound, _ := gin.CreateTestContext(wNotFound)
		cNotFound.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", strings.NewReader(validBody))
		cNotFound.Request.Header.Set("Content-Type", "application/json")
		cNotFound.Set(ctxUserID, uint(999999))
		srv.GenerateTimesheet(cNotFound)
		assertFatalCode(t, wNotFound, http.StatusNotFound)

		// 3. User with no company assigned
		wNoComp := httptest.NewRecorder()
		cNoComp, _ := gin.CreateTestContext(wNoComp)
		cNoComp.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", strings.NewReader(validBody))
		cNoComp.Request.Header.Set("Content-Type", "application/json")
		cNoComp.Set(ctxUserID, user.ID)
		srv.GenerateTimesheet(cNoComp)
		assertFatalCode(t, wNoComp, http.StatusBadRequest)

		// 4. User with assigned company string "mii"
		_ = tx.Model(&user).Update("company", "mii")
		wOk := httptest.NewRecorder()
		cOk, _ := gin.CreateTestContext(wOk)
		cOk.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", strings.NewReader(validBody))
		cOk.Request.Header.Set("Content-Type", "application/json")
		cOk.Set(ctxUserID, user.ID)
		srv.GenerateTimesheet(cOk)
		assertFatalCode(t, wOk, http.StatusOK)
		if wOk.Body.Len() == 0 {
			t.Error("expected non-empty xlsx payload in response")
		}

		// 5. User with assigned CompanyID
		testComp := models.Company{Code: "MII", Name: "Mitra Integrasi Informatika"}
		_ = tx.Create(&testComp)
		_ = tx.Model(&user).Update("company_id", testComp.ID)
		wCompID := httptest.NewRecorder()
		cCompID, _ := gin.CreateTestContext(wCompID)
		cCompID.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", strings.NewReader(validBody))
		cCompID.Request.Header.Set("Content-Type", "application/json")
		cCompID.Set(ctxUserID, user.ID)
		srv.GenerateTimesheet(cCompID)
		assertFatalCode(t, wCompID, http.StatusOK)
	})

	t.Run("sanitize helper", func(t *testing.T) {
		s := sanitize("Hello\r\nWorld\t!")
		if s != "HelloWorld" {
			t.Errorf("unexpected sanitized string: %q", s)
		}
	})
}
