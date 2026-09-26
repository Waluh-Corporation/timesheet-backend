package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// PushRepository is re-exported from domain.PushRepository.
type PushRepository = domain.PushRepository

type pushRepository struct {
	db *gorm.DB
}

// NewPushRepository constructs a PushRepository.
func NewPushRepository(db *gorm.DB) PushRepository {
	return &pushRepository{db: db}
}

func (r *pushRepository) Subscribe(ctx context.Context, sub *models.PushSubscription) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "p256dh", "auth"}),
	}).Create(sub).Error
}

func (r *pushRepository) Unsubscribe(ctx context.Context, userID uint, endpoint string) error {
	q := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if endpoint != "" {
		q = q.Where("endpoint = ?", endpoint)
	}
	return q.Delete(&models.PushSubscription{}).Error
}

func (r *pushRepository) ListByUserID(ctx context.Context, userID uint) ([]models.PushSubscription, error) {
	var subs []models.PushSubscription
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&subs).Error
	return subs, err
}

func (r *pushRepository) DeleteByID(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.PushSubscription{}).Error
}
