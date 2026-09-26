package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

type mockStorageService struct {
	existsResult bool
	existsErr    error
	downloadURL  string
	downloadErr  error
}

func (m *mockStorageService) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	return nil
}
func (m *mockStorageService) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return m.downloadURL, m.downloadErr
}
func (m *mockStorageService) GetPresignedDownloadURLWithFilename(ctx context.Context, key string, filename string, expiry time.Duration) (string, error) {
	return m.downloadURL, m.downloadErr
}
func (m *mockStorageService) Delete(ctx context.Context, key string) error {
	return nil
}
func (m *mockStorageService) FileExists(ctx context.Context, key string) (bool, error) {
	return m.existsResult, m.existsErr
}

type mockQueueClient struct {
	enqueueErr error
}

func (m *mockQueueClient) EnqueueTimesheetJob(ctx context.Context, jobID string, userID uint, month, year int) error {
	return m.enqueueErr
}
func (m *mockQueueClient) Close() error {
	return nil
}

type mockTimesheetService struct {
	genOut  []byte
	genName string
	genErr  error
}

func (m *mockTimesheetService) GenerateWorkbook(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	return m.genOut, m.genName, m.genErr
}
func (m *mockTimesheetService) UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error {
	return nil
}
func (m *mockTimesheetService) ListMonthlyOvertimes(ctx context.Context, userID uint, month int, year int) ([]response.OvertimeResponse, error) {
	return nil, nil
}
func (m *mockTimesheetService) DeleteOvertime(ctx context.Context, id uint, userID uint) error {
	return nil
}

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

	// 13. DownloadTimesheetByToken - storage error
	validToken := "token-with-storage-error"
	validJob := &models.TimesheetJob{
		ID:             uuid.New().String(),
		UserID:         user.ID,
		Month:          3,
		Year:           2026,
		Status:         models.JobStatusCompleted,
		FileKey:        "timesheets/job/valid.xlsx",
		DownloadToken:  &validToken,
		TokenExpiresAt: &future,
		MaxDownloads:   5,
		DownloadCount:  0,
	}
	_ = jobRepo.Create(context.Background(), validJob)
	srv.Storage = &mockStorageService{downloadErr: errors.New("s3 presign failed")}

	w13 := httptest.NewRecorder()
	c13, _ := gin.CreateTestContext(w13)
	c13.Params = []gin.Param{{Key: "token", Value: validToken}}
	c13.Request = httptest.NewRequest("GET", "/api/v1/timesheet/downloads/"+validToken, nil)
	srv.DownloadTimesheetByToken(c13)
	assertResponseCode(t, w13, http.StatusInternalServerError)

	// 14. DownloadTimesheetByToken - success 302 redirect
	srv.Storage = &mockStorageService{downloadURL: "https://s3.example.com/timesheet.xlsx"}
	w14 := httptest.NewRecorder()
	c14, _ := gin.CreateTestContext(w14)
	c14.Params = []gin.Param{{Key: "token", Value: validToken}}
	c14.Request = httptest.NewRequest("GET", "/api/v1/timesheet/downloads/"+validToken, nil)
	srv.DownloadTimesheetByToken(c14)
	assertResponseCode(t, w14, http.StatusFound)
}

func TestTimesheetHandlers_GenerateTimesheet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)

	// Seed user with company
	userWithCompany := &models.User{
		Username:     "gen_user_with_co",
		Email:        "gen_co@example.com",
		Name:         "Gen User Co",
		Role:         models.RoleUser,
		PasswordHash: "secret",
		Company:      "PT Maju Jaya",
		IsActive:     true,
	}
	// Seed user without company
	userNoCompany := &models.User{
		Username:     "gen_user_no_co",
		Email:        "gen_noco@example.com",
		Name:         "Gen User No Co",
		Role:         models.RoleUser,
		PasswordHash: "secret",
		Company:      "",
		IsActive:     true,
	}
	userRepo := repository.NewUserRepository(tx)
	_ = userRepo.Create(context.Background(), userWithCompany)
	_ = userRepo.Create(context.Background(), userNoCompany)

	// Seed activity for userWithCompany in months 1..6, year 2026
	actRepo := repository.NewActivityRepository(tx)
	for m := 1; m <= 6; m++ {
		act := &models.DailyActivity{
			UserID:    userWithCompany.ID,
			Date:      time.Date(2026, time.Month(m), 15, 0, 0, 0, 0, time.UTC),
			StartTime: "08:00",
			EndTime:   "17:00",
			Status:    "P",
			Activity:  "Daily development",
			IsActive:  true,
		}
		_ = actRepo.Create(context.Background(), act)
	}

	t.Run("Invalid JSON payload returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader([]byte("{invalid-json")))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("Invalid month or year bounds returns 400", func(t *testing.T) {
		bounds := []struct {
			month int
			year  int
		}{
			{0, 2026},
			{13, 2026},
			{6, 1999},
			{6, 2101},
		}
		for _, b := range bounds {
			payload, _ := json.Marshal(request.GenerateRequest{Month: b.month, Year: b.year})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
			c.Request.Header.Set("Content-Type", "application/json")
			srv.GenerateTimesheet(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("UserRepo uninitialized returns 500", func(t *testing.T) {
		emptySrv := &Server{}
		payload, _ := json.Marshal(request.GenerateRequest{Month: 6, Year: 2026})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		emptySrv.GenerateTimesheet(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("User not found returns 404", func(t *testing.T) {
		payload, _ := json.Marshal(request.GenerateRequest{Month: 6, Year: 2026})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("userID", uint(999999))
		c.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c)
		assertResponseCode(t, w, http.StatusNotFound)
	})

	t.Run("User without company returns 400", func(t *testing.T) {
		payload, _ := json.Marshal(request.GenerateRequest{Month: 6, Year: 2026})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("userID", userNoCompany.ID)
		c.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("No activities for period returns 400", func(t *testing.T) {
		payload, _ := json.Marshal(request.GenerateRequest{Month: 7, Year: 2026})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("userID", userWithCompany.ID)
		c.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("Synchronous fallback without queue", func(t *testing.T) {
		srv.QueueClient = nil
		payload, _ := json.Marshal(request.GenerateRequest{Month: 6, Year: 2026})

		// 1. TimesheetSvc nil -> 500
		srv.TimesheetSvc = nil
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Set("userID", userWithCompany.ID)
		c1.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c1.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c1)
		assertResponseCode(t, w1, http.StatusInternalServerError)

		// 2. TimesheetSvc returns ErrNotFound -> 404
		srv.TimesheetSvc = &mockTimesheetService{genErr: domain.ErrNotFound}
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Set("userID", userWithCompany.ID)
		c2.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c2.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c2)
		assertResponseCode(t, w2, http.StatusNotFound)

		// 3. TimesheetSvc returns ErrInvalidInput -> 400
		srv.TimesheetSvc = &mockTimesheetService{genErr: domain.ErrInvalidInput}
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Set("userID", userWithCompany.ID)
		c3.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c3.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c3)
		assertResponseCode(t, w3, http.StatusBadRequest)

		// 4. TimesheetSvc returns generic error -> 500
		srv.TimesheetSvc = &mockTimesheetService{genErr: errors.New("fatal build error")}
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Set("userID", userWithCompany.ID)
		c4.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c4.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c4)
		assertResponseCode(t, w4, http.StatusInternalServerError)

		// 5. TimesheetSvc returns success -> 200 attachment
		srv.TimesheetSvc = &mockTimesheetService{genOut: []byte("excel-content"), genName: "Timesheet.xlsx"}
		w5 := httptest.NewRecorder()
		c5, _ := gin.CreateTestContext(w5)
		c5.Set("userID", userWithCompany.ID)
		c5.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload))
		c5.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c5)
		assertResponseCode(t, w5, http.StatusOK)
		if !strings.Contains(w5.Header().Get("Content-Disposition"), "Timesheet.xlsx") {
			t.Errorf("expected Content-Disposition with Timesheet.xlsx, got %s", w5.Header().Get("Content-Disposition"))
		}
	})

	t.Run("Asynchronous queue workflow", func(t *testing.T) {
		jobRepo := repository.NewJobRepository(tx)
		srv.JobRepo = jobRepo
		mockQ := &mockQueueClient{}
		srv.QueueClient = mockQ

		// 1. In-flight job conflict -> 409 (Month 1)
		payload1, _ := json.Marshal(request.GenerateRequest{Month: 1, Year: 2026})
		inFlightJob := &models.TimesheetJob{
			ID:        uuid.New().String(),
			UserID:    userWithCompany.ID,
			Month:     1,
			Year:      2026,
			Status:    models.JobStatusQueued,
			CreatedAt: time.Now(),
		}
		_ = jobRepo.Create(context.Background(), inFlightJob)

		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Set("userID", userWithCompany.ID)
		c1.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload1))
		c1.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c1)
		assertResponseCode(t, w1, http.StatusConflict)

		// 2. Active completed job cache hit -> 200 OK re-issued (Month 2)
		payload2, _ := json.Marshal(request.GenerateRequest{Month: 2, Year: 2026})
		exp := time.Now().Add(24 * time.Hour)
		completedJob := &models.TimesheetJob{
			ID:             uuid.New().String(),
			UserID:         userWithCompany.ID,
			Month:          2,
			Year:           2026,
			Status:         models.JobStatusCompleted,
			FileKey:        "timesheets/cached.xlsx",
			ExpiresAt:      &exp,
			TokenExpiresAt: &exp,
			CreatedAt:      time.Now().Add(1 * time.Minute), // Created after user and activity
		}
		_ = jobRepo.Create(context.Background(), completedJob)
		srv.Storage = &mockStorageService{existsResult: true}

		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Set("userID", userWithCompany.ID)
		c2.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload2))
		c2.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c2)
		assertResponseCode(t, w2, http.StatusOK)

		// 3. Cache miss: Enqueue failure -> 500 (Month 3)
		payload3, _ := json.Marshal(request.GenerateRequest{Month: 3, Year: 2026})
		mockQ.enqueueErr = errors.New("redis connection refused")
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Set("userID", userWithCompany.ID)
		c3.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload3))
		c3.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c3)
		assertResponseCode(t, w3, http.StatusInternalServerError)

		// 4. Cache miss: Enqueue success -> 202 Accepted (Month 4)
		payload4, _ := json.Marshal(request.GenerateRequest{Month: 4, Year: 2026})
		mockQ.enqueueErr = nil
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Set("userID", userWithCompany.ID)
		c4.Request = httptest.NewRequest("POST", "/api/v1/timesheet/generate", bytes.NewReader(payload4))
		c4.Request.Header.Set("Content-Type", "application/json")
		srv.GenerateTimesheet(c4)
		assertResponseCode(t, w4, http.StatusAccepted)
	})
}
