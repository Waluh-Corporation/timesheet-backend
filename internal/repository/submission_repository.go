package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/models"
)

// SubmissionRepository provides data access for timesheet and berita acara signing submissions.
type SubmissionRepository interface {
	// Timesheet submissions
	FindTimesheetSubmission(ctx context.Context, userID uint, month, year int) (*models.TimesheetSubmission, error)
	FindTimesheetByToken(ctx context.Context, token string) (*models.TimesheetSubmission, error)
	UpsertTimesheetSubmission(ctx context.Context, sub *models.TimesheetSubmission) error

	// Berita Acara submissions
	FindBeritaAcaraSubmission(ctx context.Context, userID uint, month, year int) (*models.BeritaAcaraSubmission, error)
	FindBeritaAcaraByID(ctx context.Context, id string) (*models.BeritaAcaraSubmission, error)
	FindBeritaAcaraByToken(ctx context.Context, token string) (*models.BeritaAcaraSubmission, error)
	UpsertBeritaAcaraSubmission(ctx context.Context, sub *models.BeritaAcaraSubmission) error
	ListPendingWitnessRequests(ctx context.Context, witnessID uint) ([]models.BeritaAcaraSubmission, error)

	// Locking checks
	IsTimesheetLocked(ctx context.Context, userID uint, month, year int) (bool, error)
	IsBeritaAcaraLocked(ctx context.Context, userID uint, month, year int) (bool, error)
}

type submissionRepository struct {
	db *gorm.DB
}

// NewSubmissionRepository constructs a SubmissionRepository backed by GORM.
func NewSubmissionRepository(db *gorm.DB) SubmissionRepository {
	return &submissionRepository{db: db}
}

func (r *submissionRepository) FindTimesheetSubmission(ctx context.Context, userID uint, month, year int) (*models.TimesheetSubmission, error) {
	var sub models.TimesheetSubmission
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("user_id = ? AND month = ? AND year = ?", userID, month, year).
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find timesheet submission: %w", err)
	}
	return &sub, nil
}

func (r *submissionRepository) FindTimesheetByToken(ctx context.Context, token string) (*models.TimesheetSubmission, error) {
	var sub models.TimesheetSubmission
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("verification_token = ?", token).
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find timesheet by token: %w", err)
	}
	return &sub, nil
}

func (r *submissionRepository) UpsertTimesheetSubmission(ctx context.Context, sub *models.TimesheetSubmission) error {
	if sub == nil {
		return errors.New("timesheet submission cannot be nil")
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "month"},
			{Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"status",
			"signed_at",
			"verification_token",
			"document_hash",
			"audit_metadata",
			"updated_at",
		}),
	}).Create(sub).Error
	if err != nil {
		return fmt.Errorf("upsert timesheet submission: %w", err)
	}
	return nil
}

func (r *submissionRepository) FindBeritaAcaraSubmission(ctx context.Context, userID uint, month, year int) (*models.BeritaAcaraSubmission, error) {
	var sub models.BeritaAcaraSubmission
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Witness").
		Where("user_id = ? AND month = ? AND year = ?", userID, month, year).
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find berita acara submission: %w", err)
	}
	return &sub, nil
}

func (r *submissionRepository) FindBeritaAcaraByID(ctx context.Context, id string) (*models.BeritaAcaraSubmission, error) {
	var sub models.BeritaAcaraSubmission
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Witness").
		Where("id = ?", id).
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find berita acara by id: %w", err)
	}
	return &sub, nil
}

func (r *submissionRepository) FindBeritaAcaraByToken(ctx context.Context, token string) (*models.BeritaAcaraSubmission, error) {
	var sub models.BeritaAcaraSubmission
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Witness").
		Where("verification_token = ?", token).
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find berita acara by token: %w", err)
	}
	return &sub, nil
}

func (r *submissionRepository) UpsertBeritaAcaraSubmission(ctx context.Context, sub *models.BeritaAcaraSubmission) error {
	if sub == nil {
		return errors.New("berita acara submission cannot be nil")
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "month"},
			{Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"witness_id",
			"status",
			"employee_signed_at",
			"witness_signed_at",
			"witness_rejection_notes",
			"verification_token",
			"document_hash",
			"audit_metadata",
			"updated_at",
		}),
	}).Create(sub).Error
	if err != nil {
		return fmt.Errorf("upsert berita acara submission: %w", err)
	}
	return nil
}

func (r *submissionRepository) ListPendingWitnessRequests(ctx context.Context, witnessID uint) ([]models.BeritaAcaraSubmission, error) {
	var list []models.BeritaAcaraSubmission
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Witness").
		Where("witness_id = ? AND status = ?", witnessID, models.SubmissionStatusPendingWitness).
		Order("created_at desc").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list pending witness requests: %w", err)
	}
	return list, nil
}

func (r *submissionRepository) IsTimesheetLocked(ctx context.Context, userID uint, month, year int) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.TimesheetSubmission{}).
		Where("user_id = ? AND month = ? AND year = ? AND status IN (?, ?)",
			userID, month, year, models.SubmissionStatusSigned, models.SubmissionStatusSubmitted).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check timesheet locked: %w", err)
	}
	return count > 0, nil
}

func (r *submissionRepository) IsBeritaAcaraLocked(ctx context.Context, userID uint, month, year int) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.BeritaAcaraSubmission{}).
		Where("user_id = ? AND month = ? AND year = ? AND status IN (?, ?)",
			userID, month, year, models.SubmissionStatusPendingWitness, models.SubmissionStatusSigned).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check berita acara locked: %w", err)
	}
	return count > 0, nil
}
