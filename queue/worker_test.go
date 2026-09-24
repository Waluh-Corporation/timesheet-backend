package queue

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

type mockStorageService struct {
	uploadedKey   string
	uploadedBytes []byte
}

func (m *mockStorageService) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	m.uploadedKey = key
	buf, _ := io.ReadAll(body)
	m.uploadedBytes = buf
	return nil
}

func (m *mockStorageService) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "https://s3.example.com/timesheets/" + key, nil
}

func (m *mockStorageService) Delete(ctx context.Context, key string) error {
	return nil
}

func (m *mockStorageService) FileExists(ctx context.Context, key string) (bool, error) {
	return true, nil
}

type mockJobRepository struct {
	createdJob *models.TimesheetJob
	lastStatus models.TimesheetJobStatus
	lastURL    string
}

func (m *mockJobRepository) Create(ctx context.Context, job *models.TimesheetJob) error {
	m.createdJob = job
	return nil
}

func (m *mockJobRepository) FindByID(ctx context.Context, id string) (*models.TimesheetJob, error) {
	if m.createdJob != nil && m.createdJob.ID == id {
		return m.createdJob, nil
	}
	return nil, nil
}

func (m *mockJobRepository) FindActiveByIDAndUser(ctx context.Context, id string, userID uint) (*models.TimesheetJob, error) {
	if m.createdJob != nil && m.createdJob.ID == id && m.createdJob.UserID == userID {
		return m.createdJob, nil
	}
	return nil, nil
}

func (m *mockJobRepository) ListByUser(ctx context.Context, userID uint, limit, offset int) ([]models.TimesheetJob, int64, error) {
	if m.createdJob != nil && m.createdJob.UserID == userID {
		return []models.TimesheetJob{*m.createdJob}, 1, nil
	}
	return []models.TimesheetJob{}, 0, nil
}

func (m *mockJobRepository) UpdateStatus(ctx context.Context, id string, status models.TimesheetJobStatus, fileKey, downloadURL, errMsg string, expiresAt *time.Time) error {
	m.lastStatus = status
	m.lastURL = downloadURL
	if m.createdJob != nil {
		m.createdJob.Status = status
		m.createdJob.DownloadURL = downloadURL
		m.createdJob.FileKey = fileKey
		m.createdJob.ErrorMessage = errMsg
		m.createdJob.ExpiresAt = expiresAt
	}
	return nil
}

func (m *mockJobRepository) UpdateStatusWithToken(ctx context.Context, id string, status models.TimesheetJobStatus, fileKey, downloadURL, errMsg string, expiresAt *time.Time, downloadToken string, maxDownloads int, tokenExpiresAt *time.Time) error {
	m.lastStatus = status
	m.lastURL = downloadURL
	if m.createdJob != nil {
		m.createdJob.Status = status
		m.createdJob.DownloadURL = downloadURL
		m.createdJob.FileKey = fileKey
		m.createdJob.ErrorMessage = errMsg
		m.createdJob.ExpiresAt = expiresAt
		m.createdJob.DownloadToken = &downloadToken
		m.createdJob.MaxDownloads = maxDownloads
		m.createdJob.TokenExpiresAt = tokenExpiresAt
	}
	return nil
}

func (m *mockJobRepository) FindByDownloadToken(ctx context.Context, token string) (*models.TimesheetJob, error) {
	if m.createdJob != nil && m.createdJob.DownloadToken != nil && *m.createdJob.DownloadToken == token {
		return m.createdJob, nil
	}
	return nil, nil
}

func (m *mockJobRepository) IncrementDownloadCount(ctx context.Context, id string) error {
	if m.createdJob != nil {
		m.createdJob.DownloadCount++
	}
	return nil
}

func (m *mockJobRepository) RenewDownloadToken(ctx context.Context, id string, newToken string, maxDownloads int, expiresAt time.Time) error {
	if m.createdJob != nil {
		m.createdJob.DownloadToken = &newToken
		m.createdJob.DownloadCount = 0
		m.createdJob.MaxDownloads = maxDownloads
		m.createdJob.TokenExpiresAt = &expiresAt
	}
	return nil
}

func (m *mockJobRepository) FindActiveCompletedJob(ctx context.Context, userID uint, month, year int) (*models.TimesheetJob, error) {
	if m.createdJob != nil && m.createdJob.UserID == userID && m.createdJob.Month == month && m.createdJob.Year == year && m.createdJob.Status == models.JobStatusCompleted {
		return m.createdJob, nil
	}
	return nil, nil
}

func (m *mockJobRepository) HasInFlightJob(ctx context.Context, userID uint, month, year int) (bool, error) {
	return false, nil
}

func (m *mockJobRepository) FindExpiredJobs(ctx context.Context, now time.Time, limit int) ([]models.TimesheetJob, error) {
	return nil, nil
}

func (m *mockJobRepository) Delete(ctx context.Context, id string) error {
	return nil
}

type mockTimesheetService struct{}

func (m *mockTimesheetService) GenerateWorkbook(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	return []byte("PK-mock-excel-binary"), "Timesheet_john_07_2026.xlsx", nil
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

type mockUserRepo struct {
	repository.UserRepository
}

func (m *mockUserRepo) FindByIDWithDetails(ctx context.Context, id uint) (*models.User, error) {
	return &models.User{
		ID:       id,
		Username: "john_doe",
		Email:    "john@example.com",
		Company:  "PT Mitra Integrasi Informatika",
	}, nil
}

func TestWorkerServer_ProcessTaskDirect(t *testing.T) {
	cfg := &config.Config{
		S3Bucket:               "test-timesheets",
		S3RetentionDays:        7,
		ExcelMaxConcurrentJobs: 5,
	}

	jobRepo := &mockJobRepository{
		createdJob: &models.TimesheetJob{
			ID:     "job-12345",
			UserID: 1,
			Month:  7,
			Year:   2026,
			Status: models.JobStatusQueued,
		},
	}
	storageSvc := &mockStorageService{}
	tsSvc := &mockTimesheetService{}
	userRepo := &mockUserRepo{}

	worker := NewWorkerServer(cfg, jobRepo, userRepo, tsSvc, storageSvc, nil, nil)
	require.NotNil(t, worker)

	ctx := context.Background()
	payload := GenerateTimesheetPayload{
		JobID:  "job-12345",
		UserID: 1,
		Month:  7,
		Year:   2026,
	}

	err := worker.ProcessTaskDirect(ctx, payload)
	require.NoError(t, err)

	assert.Equal(t, models.JobStatusCompleted, jobRepo.lastStatus)
	assert.Contains(t, jobRepo.lastURL, "/api/v1/timesheet/downloads/")
	assert.NotNil(t, jobRepo.createdJob.DownloadToken)
	assert.Equal(t, 3, jobRepo.createdJob.MaxDownloads)
	assert.Equal(t, []byte("PK-mock-excel-binary"), storageSvc.uploadedBytes)
}
