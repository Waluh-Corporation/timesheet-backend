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

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Code       int                   `json:"code"`
			Status     string                `json:"status"`
			Data       []map[string]string   `json:"data"`
			Pagination models.PaginationMeta `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal JSON response: %v", err)
		}

		if resp.Code != 200 {
			t.Errorf("expected code 200, got %d", resp.Code)
		}
		if resp.Status != "success" {
			t.Errorf("expected status 'success', got %s", resp.Status)
		}
		if len(resp.Data) != 2 {
			t.Errorf("expected 2 items, got %d", len(resp.Data))
		}
		if resp.Pagination.Page != 2 {
			t.Errorf("expected page 2, got %d", resp.Pagination.Page)
		}
		if resp.Pagination.Limit != 10 {
			t.Errorf("expected limit 10, got %d", resp.Pagination.Limit)
		}
		if resp.Pagination.TotalRows != 35 {
			t.Errorf("expected total_rows 35, got %d", resp.Pagination.TotalRows)
		}
		if resp.Pagination.TotalPages != 4 {
			t.Errorf("expected total_pages 4 (ceil(35/10)), got %d", resp.Pagination.TotalPages)
		}
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

			if w.Code != tc.expectCode {
				t.Errorf("expected status %d, got %d (body: %s)", tc.expectCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestActivityHandlers_Integration(t *testing.T) {
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Auth: authSvc,
	}

	// Create test users
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

	// Create test activities for user1
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

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
		}

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

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status 403 for other user, got %d", w.Code)
		}
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

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status 403 for admin viewing another user's activity, got %d", w.Code)
		}
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

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Code       int                    `json:"code"`
			Status     string                 `json:"status"`
			Data       []models.DailyActivity `json:"data"`
			Pagination models.PaginationMeta  `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(resp.Data) != 2 {
			t.Errorf("expected 2 items for limit=2, got %d", len(resp.Data))
		}
		if resp.Pagination.Page != 1 {
			t.Errorf("expected page 1, got %d", resp.Pagination.Page)
		}
		if resp.Pagination.Limit != 2 {
			t.Errorf("expected limit 2, got %d", resp.Pagination.Limit)
		}
		if resp.Pagination.TotalRows != 3 {
			t.Errorf("expected total_rows 3, got %d", resp.Pagination.TotalRows)
		}
		if resp.Pagination.TotalPages != 2 {
			t.Errorf("expected total_pages 2, got %d", resp.Pagination.TotalPages)
		}
	})

	t.Run("UpsertDailyActivity synchronizes project attributes from master Project", func(t *testing.T) {
		// Create a master project
		testProj := models.Project{
			Code:        "P99001",
			Name:        "Master Project Alpha",
			AppImpacted: "Alpha Mobile App",
			IsActive:    true,
		}
		if err := tx.Create(&testProj).Error; err != nil {
			t.Fatalf("failed to create test project: %v", err)
		}

		// Upsert with project_ref_id
		reqBody := `{"date":"2026-09-20","project_ref_id":` + itoa(testProj.ID) + `,"activity":"Developing Alpha feature","status":"P","start_time":"08:30","end_time":"17:30"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/activities", strings.NewReader(reqBody))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, user1.ID)
		c.Set(ctxRole, models.RoleUser)

		srv.UpsertDailyActivity(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
		}

		var resp struct {
			Code int                  `json:"code"`
			Data models.DailyActivity `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		// Verify 3NF relational sync
		if resp.Data.ProjectRefID == nil || *resp.Data.ProjectRefID != testProj.ID {
			t.Errorf("expected ProjectRefID %d, got %v", testProj.ID, resp.Data.ProjectRefID)
		}
		if resp.Data.ProjectID != testProj.Code {
			t.Errorf("expected ProjectID %q, got %q", testProj.Code, resp.Data.ProjectID)
		}
		if resp.Data.ProjectName != testProj.Name {
			t.Errorf("expected ProjectName %q, got %q", testProj.Name, resp.Data.ProjectName)
		}
		if resp.Data.AppImpacted != testProj.AppImpacted {
			t.Errorf("expected AppImpacted %q, got %q", testProj.AppImpacted, resp.Data.AppImpacted)
		}
		if resp.Data.GetProjectCode() != testProj.Code {
			t.Errorf("expected GetProjectCode() %q, got %q", testProj.Code, resp.Data.GetProjectCode())
		}
		if resp.Data.GetProjectName() != testProj.Name {
			t.Errorf("expected GetProjectName() %q, got %q", testProj.Name, resp.Data.GetProjectName())
		}
		if resp.Data.GetAppImpacted() != testProj.AppImpacted {
			t.Errorf("expected GetAppImpacted() %q, got %q", testProj.AppImpacted, resp.Data.GetAppImpacted())
		}
	})
}

func itoa(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}
