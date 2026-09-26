package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// JobRepository provides data access methods for asynchronous timesheet jobs.
type JobRepository interface {
	Create(ctx context.Context, job *models.TimesheetJob) error
	FindByID(ctx context.Context, id string) (*models.TimesheetJob, error)
	FindActiveByIDAndUser(ctx context.Context, id string, userID uint) (*models.TimesheetJob, error)
	FindByDownloadToken(ctx context.Context, token string) (*models.TimesheetJob, error)
	IncrementDownloadCount(ctx context.Context, id string) error
	RenewDownloadToken(ctx context.Context, id string, newToken string, maxDownloads int, expiresAt time.Time) error
	FindActiveCompletedJob(ctx context.Context, userID uint, month, year int) (*models.TimesheetJob, error)
	HasInFlightJob(ctx context.Context, userID uint, month, year int) (bool, error)
	ListByUser(ctx context.Context, userID uint, limit, offset int) ([]models.TimesheetJob, int64, error)
	UpdateStatus(ctx context.Context, id string, status models.TimesheetJobStatus, fileKey, downloadURL, errMsg string, expiresAt *time.Time) error
	UpdateStatusWithToken(ctx context.Context, id string, status models.TimesheetJobStatus, fileKey, downloadURL, errMsg string, expiresAt *time.Time, downloadToken string, maxDownloads int, tokenExpiresAt *time.Time) error
	FindExpiredJobs(ctx context.Context, now time.Time, limit int) ([]models.TimesheetJob, error)
	Delete(ctx context.Context, id string) error
}

type jobRepository struct {
	db *gorm.DB
}

// NewJobRepository constructs a JobRepository backed by GORM.
func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository) Create(ctx context.Context, job *models.TimesheetJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *jobRepository) FindByID(ctx context.Context, id string) (*models.TimesheetJob, error) {
	var job models.TimesheetJob
	err := r.db.WithContext(ctx).Preload("User").Where("id = ?", id).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *jobRepository) FindActiveByIDAndUser(ctx context.Context, id string, userID uint) (*models.TimesheetJob, error) {
	var job models.TimesheetJob
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *jobRepository) FindByDownloadToken(ctx context.Context, token string) (*models.TimesheetJob, error) {
	var job models.TimesheetJob
	err := r.db.WithContext(ctx).Preload("User").Where("download_token = ?", token).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *jobRepository) IncrementDownloadCount(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&models.TimesheetJob{}).
		Where("id = ?", id).
		Update("download_count", gorm.Expr("download_count + ?", 1)).Error
}

func (r *jobRepository) RenewDownloadToken(ctx context.Context, id string, newToken string, maxDownloads int, expiresAt time.Time) error {
	updates := map[string]interface{}{
		"download_token":   newToken,
		"download_count":   0,
		"max_downloads":    maxDownloads,
		"token_expires_at": expiresAt,
		"updated_at":       time.Now(),
	}
	return r.db.WithContext(ctx).Model(&models.TimesheetJob{}).Where("id = ?", id).Updates(updates).Error
}

func (r *jobRepository) FindActiveCompletedJob(ctx context.Context, userID uint, month, year int) (*models.TimesheetJob, error) {
	var job models.TimesheetJob
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND month = ? AND year = ? AND status = ? AND expires_at IS NOT NULL AND expires_at > ? AND file_key IS NOT NULL AND file_key != ''",
			userID, month, year, models.JobStatusCompleted, now).
		Order("created_at desc").
		First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *jobRepository) HasInFlightJob(ctx context.Context, userID uint, month, year int) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.TimesheetJob{}).
		Where("user_id = ? AND month = ? AND year = ? AND status IN (?)",
			userID, month, year, []models.TimesheetJobStatus{models.JobStatusQueued, models.JobStatusProcessing}).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *jobRepository) ListByUser(ctx context.Context, userID uint, limit, offset int) ([]models.TimesheetJob, int64, error) {
	var jobs []models.TimesheetJob
	var total int64

	q := r.db.WithContext(ctx).Model(&models.TimesheetJob{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&jobs).Error
	return jobs, total, err
}

func (r *jobRepository) UpdateStatus(ctx context.Context, id string, status models.TimesheetJobStatus, fileKey, downloadURL, errMsg string, expiresAt *time.Time) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if fileKey != "" {
		updates["file_key"] = fileKey
	}
	if downloadURL != "" {
		updates["download_url"] = downloadURL
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	if expiresAt != nil {
		updates["expires_at"] = expiresAt
	}

	return r.db.WithContext(ctx).Model(&models.TimesheetJob{}).Where("id = ?", id).Updates(updates).Error
}

func (r *jobRepository) UpdateStatusWithToken(ctx context.Context, id string, status models.TimesheetJobStatus, fileKey, downloadURL, errMsg string, expiresAt *time.Time, downloadToken string, maxDownloads int, tokenExpiresAt *time.Time) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if fileKey != "" {
		updates["file_key"] = fileKey
	}
	if downloadURL != "" {
		updates["download_url"] = downloadURL
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	if expiresAt != nil {
		updates["expires_at"] = expiresAt
	}
	if downloadToken != "" {
		updates["download_token"] = downloadToken
	}
	if maxDownloads > 0 {
		updates["max_downloads"] = maxDownloads
	}
	if tokenExpiresAt != nil {
		updates["token_expires_at"] = tokenExpiresAt
	}

	return r.db.WithContext(ctx).Model(&models.TimesheetJob{}).Where("id = ?", id).Updates(updates).Error
}

func (r *jobRepository) FindExpiredJobs(ctx context.Context, now time.Time, limit int) ([]models.TimesheetJob, error) {
	var jobs []models.TimesheetJob
	if limit <= 0 {
		limit = 100
	}
	err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ? AND file_key IS NOT NULL AND file_key != ''", now).
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

func (r *jobRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.TimesheetJob{}).Error
}
