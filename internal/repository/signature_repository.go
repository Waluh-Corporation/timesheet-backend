package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// SignatureRepository provides data access methods for user signature specimens.
type SignatureRepository interface {
	FindActiveByUserID(ctx context.Context, userID uint) (*models.UserSignature, error)
	FindLatestPendingByUserID(ctx context.Context, userID uint) (*models.UserSignature, error)
	FindBySubmitterID(ctx context.Context, submitterID int64) (*models.UserSignature, error)
	FindBySlugAndUserID(ctx context.Context, slug string, userID uint) (*models.UserSignature, error)
	Create(ctx context.Context, sig *models.UserSignature) error
	Save(ctx context.Context, sig *models.UserSignature) error
	ActivateSpecimen(ctx context.Context, sig *models.UserSignature) error
	RevokeActiveByUserID(ctx context.Context, userID uint) (*models.UserSignature, error)
}

type signatureRepository struct {
	db *gorm.DB
}

// NewSignatureRepository constructs a SignatureRepository backed by GORM.
func NewSignatureRepository(db *gorm.DB) SignatureRepository {
	return &signatureRepository{db: db}
}

func (r *signatureRepository) FindActiveByUserID(ctx context.Context, userID uint) (*models.UserSignature, error) {
	var sig models.UserSignature
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = ? AND status = ?", userID, true, models.SignatureStatusCompleted).
		Order("updated_at desc").
		First(&sig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sig, nil
}

func (r *signatureRepository) FindLatestPendingByUserID(ctx context.Context, userID uint) (*models.UserSignature, error) {
	var sig models.UserSignature
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, models.SignatureStatusPending).
		Order("created_at desc").
		First(&sig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sig, nil
}

func (r *signatureRepository) FindBySubmitterID(ctx context.Context, submitterID int64) (*models.UserSignature, error) {
	var sig models.UserSignature
	err := r.db.WithContext(ctx).
		Where("docuseal_submitter_id = ?", submitterID).
		Order("created_at desc").
		First(&sig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sig, nil
}

func (r *signatureRepository) FindBySlugAndUserID(ctx context.Context, slug string, userID uint) (*models.UserSignature, error) {
	var sig models.UserSignature
	err := r.db.WithContext(ctx).
		Where("docuseal_slug = ? AND user_id = ?", slug, userID).
		Order("created_at desc").
		First(&sig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sig, nil
}

func (r *signatureRepository) Create(ctx context.Context, sig *models.UserSignature) error {
	return r.db.WithContext(ctx).Create(sig).Error
}

func (r *signatureRepository) Save(ctx context.Context, sig *models.UserSignature) error {
	return r.db.WithContext(ctx).Save(sig).Error
}

// ActivateSpecimen atomically deactivates any previous active specimen for the user
// and activates the given specimen record.
func (r *signatureRepository) ActivateSpecimen(ctx context.Context, sig *models.UserSignature) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&models.UserSignature{}).
			Where("user_id = ? AND is_active = ? AND id != ?", sig.UserID, true, sig.ID).
			Updates(map[string]interface{}{
				"is_active":  false,
				"status":     models.SignatureStatusRevoked,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		sig.IsActive = true
		sig.Status = models.SignatureStatusCompleted
		sig.UpdatedAt = now
		if sig.ID == 0 {
			return tx.Create(sig).Error
		}
		return tx.Save(sig).Error
	})
}

// RevokeActiveByUserID deactivates the currently active signature specimen for a user and returns it.
func (r *signatureRepository) RevokeActiveByUserID(ctx context.Context, userID uint) (*models.UserSignature, error) {
	active, err := r.FindActiveByUserID(ctx, userID)
	if err != nil || active == nil {
		return nil, err
	}
	now := time.Now()
	err = r.db.WithContext(ctx).Model(&models.UserSignature{}).
		Where("id = ? AND user_id = ?", active.ID, userID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"status":     models.SignatureStatusRevoked,
			"updated_at": now,
		}).Error
	if err != nil {
		return nil, err
	}
	active.IsActive = false
	active.Status = models.SignatureStatusRevoked
	active.UpdatedAt = now
	return active, nil
}
