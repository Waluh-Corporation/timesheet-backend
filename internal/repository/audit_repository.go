package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// AuditLogFilter is re-exported from domain.AuditLogFilter.
type AuditLogFilter = domain.AuditLogFilter

// AuditRepository is re-exported from domain.AuditRepository.
type AuditRepository = domain.AuditRepository

type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository instantiates a new AuditRepository.
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) getDB(ctx context.Context) *gorm.DB {
	if r == nil {
		return ExtractTx(ctx, nil)
	}
	return ExtractTx(ctx, r.db)
}

// Create persists a new audit log entry into the database.
func (r *auditRepository) Create(ctx context.Context, entry *models.AuditLog) error {
	if entry == nil {
		return nil
	}
	db := r.getDB(ctx)
	if db == nil {
		return errors.New("database connection unavailable")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	return db.Create(entry).Error
}

// List retrieves paginated audit log entries with optional filtering ordered by creation date descending.
func (r *auditRepository) List(ctx context.Context, filter AuditLogFilter, limit, offset int) ([]models.AuditLog, int64, error) {
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

	if filter.StartDate != nil && filter.EndDate != nil && filter.StartDate.After(*filter.EndDate) {
		return []models.AuditLog{}, 0, nil
	}

	db := r.getDB(ctx)
	if db == nil {
		return nil, 0, errors.New("database connection unavailable")
	}

	q := db.Model(&models.AuditLog{})

	action := strings.TrimSpace(filter.Action)
	if action != "" {
		q = q.Where("action = ?", action)
	}
	resourceType := strings.TrimSpace(filter.ResourceType)
	if resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	actorUsername := strings.TrimSpace(filter.ActorUsername)
	if actorUsername != "" {
		q = q.Where("actor_username = ?", actorUsername)
	}
	if filter.StartDate != nil {
		q = q.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		q = q.Where("created_at <= ?", *filter.EndDate)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := q.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	if entries == nil {
		entries = []models.AuditLog{}
	}

	return entries, total, nil
}
