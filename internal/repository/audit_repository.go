package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// AuditRepository defines the interface for persisting and querying audit log entries.
type AuditRepository interface {
	Create(ctx context.Context, entry *models.AuditLog) error
	List(ctx context.Context, limit, offset int) ([]models.AuditLog, int64, error)
}

type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository instantiates a new AuditRepository.
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

// Create persists a new audit log entry into the database.
func (r *auditRepository) Create(ctx context.Context, entry *models.AuditLog) error {
	if entry == nil {
		return nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(entry).Error
}

// List retrieves paginated audit log entries ordered by creation date descending.
func (r *auditRepository) List(ctx context.Context, limit, offset int) ([]models.AuditLog, int64, error) {
	var entries []models.AuditLog
	var total int64

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	q := r.db.WithContext(ctx).Model(&models.AuditLog{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}
