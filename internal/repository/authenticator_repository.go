package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

// AuthenticatorRepository is re-exported from domain.AuthenticatorRepository.
type AuthenticatorRepository = domain.AuthenticatorRepository

type authenticatorRepository struct {
	db *gorm.DB
}

// NewAuthenticatorRepository constructs an AuthenticatorRepository.
func NewAuthenticatorRepository(db *gorm.DB) AuthenticatorRepository {
	return &authenticatorRepository{db: db}
}

func (r *authenticatorRepository) SyncAAGUIDs(ctx context.Context, entries map[string]services.CommunityAAGUIDEntry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	records := make([]models.AuthenticatorAAGUID, 0, len(entries))
	for aaguid, item := range entries {
		normAAGUID := strings.ToLower(strings.TrimSpace(aaguid))
		if normAAGUID == "" {
			continue
		}
		records = append(records, models.AuthenticatorAAGUID{
			AAGUID:    normAAGUID,
			Name:      item.Name,
			Icon:      item.Icon,
			UpdatedAt: now,
		})
	}

	const batchSize = 100
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]
		err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "aaguid"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "icon", "updated_at"}),
		}).Create(&batch).Error
		if err != nil {
			return 0, err
		}
	}

	return len(records), nil
}

func (r *authenticatorRepository) ListAuthenticators(ctx context.Context, search string, offset, limit int) ([]models.AuthenticatorAAGUID, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.AuthenticatorAAGUID{})
	search = strings.TrimSpace(search)
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(aaguid) LIKE LOWER(?)", pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []models.AuthenticatorAAGUID
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	if err := query.Order("name ASC").Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}
