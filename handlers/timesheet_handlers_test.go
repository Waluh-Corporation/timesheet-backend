package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestTimesheetHandlers_Jobs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)

	// Seed user
	user := &models.User{
		Username:     "ts_job_tester",
		Email:        "ts_job@example.com",
		Name:         "TS Job Tester",
		Role:         models.RoleUser,
		PasswordHash: "secret",
		IsActive:     true,
	}
	userRepo := repository.NewUserRepository(tx)
	_ = userRepo.Create(context.Background(), user)

	jobRepo := repository.NewJobRepository(tx)
	jobID := uuid.New().String()
	token := "valid-token-1234567890"
	expiresAt := time.Now().Add(24 * time.Hour)
	job := &models.TimesheetJob{
		ID:            jobID,
		UserID:        user.ID,
		Month:         6,
		Year:          2026,
		Status:        models.JobStatusCompleted,
		FileKey:       "timesheets/1/Timesheet_2026_06.xlsx",
		DownloadToken: &token,
		MaxDownloads:  3,
		DownloadCount: 0,
		ExpiresAt:     &expiresAt,
	}
	_ = jobRepo.Create(context.Background(), job)

	// 1. GetTimesheetJob - empty ID
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest("GET", "/api/v1/timesheet/jobs/", nil)
	srv.GetTimesheetJob(c1)
	assertResponseCode(t, w1, http.StatusBadRequest)

	// 2. GetTimesheetJob - uninitialized jobRepo
	emptySrv := &Server{}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = []gin.Param{{Key: "id", Value: jobID}}
	c2.Request = httptest.NewRequest("GET", "/api/v1/timesheet/jobs/"+jobID, nil)
	emptySrv.GetTimesheetJob(c2)
	assertResponseCode(t, w2, http.StatusInternalServerError)

	// 3. GetTimesheetJob - not found for user
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Params = []gin.Param{{Key: "id", Value: "non-existent-job-id"}}
	c3.Set("userID", user.ID)
	c3.Request = httptest.NewRequest("GET", "/api/v1/timesheet/jobs/non-existent-job-id", nil)
	srv.GetTimesheetJob(c3)
	assertResponseCode(t, w3, http.StatusNotFound)

	// 4. GetTimesheetJob - success
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Params = []gin.Param{{Key: "id", Value: jobID}}
	c4.Set("userID", user.ID)
	c4.Request = httptest.NewRequest("GET", "/api/v1/timesheet/jobs/"+jobID, nil)
	srv.GetTimesheetJob(c4)
	assertResponseCode(t, w4, http.StatusOK)

	// 5. ListTimesheetJobs - uninitialized jobRepo
	w5 := httptest.NewRecorder()
	c5, _ := gin.CreateTestContext(w5)
	c5.Request = httptest.NewRequest("GET", "/api/v1/timesheet/jobs", nil)
	emptySrv.ListTimesheetJobs(c5)
	assertResponseCode(t, w5, http.StatusInternalServerError)

	// 6. ListTimesheetJobs - success
	w6 := httptest.NewRecorder()
	c6, _ := gin.CreateTestContext(w6)
	c6.Set("userID", user.ID)
	c6.Request = httptest.NewRequest("GET", "/api/v1/timesheet/jobs?page=1&limit=5", nil)
	srv.ListTimesheetJobs(c6)
	assertResponseCode(t, w6, http.StatusOK)

	// 7. DownloadTimesheetByToken - invalid short token
	w7 := httptest.NewRecorder()
	c7, _ := gin.CreateTestContext(w7)
	c7.Params = []gin.Param{{Key: "token", Value: "short"}}
	c7.Request = httptest.NewRequest("GET", "/api/v1/timesheet/downloads/short", nil)
	srv.DownloadTimesheetByToken(c7)
	assertResponseCode(t, w7, http.StatusBadRequest)

	// 8. DownloadTimesheetByToken - uninitialized jobRepo
	w8 := httptest.NewRecorder()
	c8, _ := gin.CreateTestContext(w8)
	c8.Params = []gin.Param{{Key: "token", Value: token}}
	c8.Request = httptest.NewRequest("GET", "/api/v1/timesheet/downloads/"+token, nil)
	emptySrv.DownloadTimesheetByToken(c8)
	assertResponseCode(t, w8, http.StatusInternalServerError)

	// 9. DownloadTimesheetByToken - token not found
	w9 := httptest.NewRecorder()
	c9, _ := gin.CreateTestContext(w9)
	c9.Params = []gin.Param{{Key: "token", Value: "unknown-token-1234567"}}
	c9.Request = httptest.NewRequest("GET", "/api/v1/timesheet/downloads/unknown-token-1234567", nil)
	srv.DownloadTimesheetByToken(c9)
	assertResponseCode(t, w9, http.StatusNotFound)

	// 10. DownloadTimesheetByToken - HEAD request
	w10 := httptest.NewRecorder()
	c10, _ := gin.CreateTestContext(w10)
	c10.Params = []gin.Param{{Key: "token", Value: token}}
	c10.Request = httptest.NewRequest("HEAD", "/api/v1/timesheet/downloads/"+token, nil)
	srv.DownloadTimesheetByToken(c10)
	assertResponseCode(t, w10, http.StatusOK)

	// 11. DownloadTimesheetByToken - expired token
	past := time.Now().Add(-1 * time.Hour)
	expiredJobID := uuid.New().String()
	expiredToken := "expired-token-1234567"
	expiredJob := &models.TimesheetJob{
		ID:             expiredJobID,
		UserID:         user.ID,
		Month:          5,
		Year:           2026,
		Status:         models.JobStatusCompleted,
		DownloadToken:  &expiredToken,
		TokenExpiresAt: &past,
		MaxDownloads:   3,
	}
	_ = jobRepo.Create(context.Background(), expiredJob)

	w11 := httptest.NewRecorder()
	c11, _ := gin.CreateTestContext(w11)
	c11.Params = []gin.Param{{Key: "token", Value: expiredToken}}
	c11.Request = httptest.NewRequest("GET", "/api/v1/timesheet/downloads/"+expiredToken, nil)
	srv.DownloadTimesheetByToken(c11)
	assertResponseCode(t, w11, http.StatusGone)

	// 12. DownloadTimesheetByToken - quota exceeded
	quotaJobID := uuid.New().String()
	quotaToken := "quota-token-12345678"
	future := time.Now().Add(10 * time.Hour)
	quotaJob := &models.TimesheetJob{
		ID:             quotaJobID,
		UserID:         user.ID,
		Month:          4,
		Year:           2026,
		Status:         models.JobStatusCompleted,
		DownloadToken:  &quotaToken,
		TokenExpiresAt: &future,
		MaxDownloads:   2,
		DownloadCount:  2,
	}
	_ = jobRepo.Create(context.Background(), quotaJob)

	w12 := httptest.NewRecorder()
	c12, _ := gin.CreateTestContext(w12)
	c12.Params = []gin.Param{{Key: "token", Value: quotaToken}}
	c12.Request = httptest.NewRequest("GET", "/api/v1/timesheet/downloads/"+quotaToken, nil)
	srv.DownloadTimesheetByToken(c12)
	assertResponseCode(t, w12, http.StatusGone)
}
