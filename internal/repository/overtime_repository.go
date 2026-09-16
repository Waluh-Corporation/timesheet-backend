package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// OvertimeRepository defines database operations for overtime entries.
type OvertimeRepository interface {
	FindActiveByID(ctx context.Context, id uint, userID uint) (*models.OvertimeEntry, error)
	FindByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.OvertimeEntry, error)
	FindByUserAndMonth(ctx context.Context, userID uint, start time.Time, end time.Time) ([]models.OvertimeEntry, error)
	Create(ctx context.Context, entry *models.OvertimeEntry) error
	Update(ctx context.Context, entry *models.OvertimeEntry) error
	SoftDelete(ctx context.Context, id uint, userID uint) error
}

type overtimeRepository struct {
	db *gorm.DB
}

// NewOvertimeRepository constructs an instance of OvertimeRepository.
func NewOvertimeRepository(db *gorm.DB) OvertimeRepository {
	return &overtimeRepository{db: db}
}

func (r *overtimeRepository) FindActiveByID(ctx context.Context, id uint, userID uint) (*models.OvertimeEntry, error) {
	var entry models.OvertimeEntry
	err := r.db.WithContext(ctx).Scopes(models.ActiveOnly).
		Preload("TeamLeader", models.ActiveOnly).
		Preload("DepartmentHead", models.ActiveOnly).
		Where("id = ? AND user_id = ?", id, userID).
		First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *overtimeRepository) FindByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.OvertimeEntry, error) {
	var entry models.OvertimeEntry
	err := r.db.WithContext(ctx).Scopes(models.ActiveOnly).
		Where("user_id = ? AND date = ?", userID, date).
		First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *overtimeRepository) FindByUserAndMonth(ctx context.Context, userID uint, start time.Time, end time.Time) ([]models.OvertimeEntry, error) {
	var overtimes []models.OvertimeEntry
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Scopes(models.ActiveOnly).
		Preload("TeamLeader", models.ActiveOnly).
		Preload("DepartmentHead", models.ActiveOnly).
		Order("date asc").
		Find(&overtimes).Error
	if err != nil {
		return nil, err
	}
	return overtimes, nil
}

func (r *overtimeRepository) Create(ctx context.Context, entry *models.OvertimeEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *overtimeRepository) Update(ctx context.Context, entry *models.OvertimeEntry) error {
	return r.db.WithContext(ctx).Save(entry).Error
}

func (r *overtimeRepository) SoftDelete(ctx context.Context, id uint, userID uint) error {
	return r.db.WithContext(ctx).Model(&models.OvertimeEntry{}).
		Where("id = ? AND user_id = ? AND is_active = true", id, userID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now(),
		}).Error
}
