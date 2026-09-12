package handlers

import (
	"encoding/json"
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
