package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// UserRepository defines the database persistence contract for User entities.
type UserRepository interface {
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByIDWithDetails(ctx context.Context, id uint) (*models.User, error)
	FindByIDWithCredentials(ctx context.Context, id uint) (*models.User, error)
	FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error)
	FindByUsernameOrEmailWithCredentials(ctx context.Context, identifier string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error

	// WebAuthn Passkey operations
	CreatePasskeyCredential(ctx context.Context, cred *models.WebAuthnCredential) error
	UpdatePasskeySignCount(ctx context.Context, credID []byte, signCount uint32, backupState bool) error
	ListPasskeysByUserID(ctx context.Context, userID uint) ([]models.WebAuthnCredential, error)
	DeletePasskey(ctx context.Context, id uint, userID *uint) (bool, error)
	UpdatePasskeyName(ctx context.Context, id uint, userID *uint, name string) (bool, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository constructs a GORM implementation of UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Scopes(models.ActiveOnly).
		Preload("CompanyRel").
		Preload("SiteRel").
		Preload("DepartmentRel").
		Preload("DivisionRel").
		Where("id = ?", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByIDWithCredentials(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Preload("Credentials").Where("id = ?", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("username = ? OR email = ?", identifier, identifier).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByUsernameOrEmailWithCredentials(ctx context.Context, identifier string) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Preload("Credentials").
		Where("username = ? OR email = ?", identifier, identifier).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password_hash": passwordHash,
		"updated_at":    updatedAt,
	}).Error
}

func (r *userRepository) CreatePasskeyCredential(ctx context.Context, cred *models.WebAuthnCredential) error {
	return r.db.WithContext(ctx).Create(cred).Error
}

func (r *userRepository) UpdatePasskeySignCount(ctx context.Context, credID []byte, signCount uint32, backupState bool) error {
	return r.db.WithContext(ctx).Model(&models.WebAuthnCredential{}).
		Where("credential_id = ?", credID).
		Updates(map[string]interface{}{
			"sign_count":   signCount,
			"backup_state": backupState,
		}).Error
}

func (r *userRepository) ListPasskeysByUserID(ctx context.Context, userID uint) ([]models.WebAuthnCredential, error) {
	var creds []models.WebAuthnCredential
	if err := r.db.WithContext(ctx).Preload("Authenticator").Where("user_id = ?", userID).
		Order("created_at desc").Find(&creds).Error; err != nil {
		return nil, err
	}
	for i := range creds {
		if creds[i].Authenticator != nil {
			creds[i].Icon = creds[i].Authenticator.Icon
		} else if info := models.GetAuthenticatorInfo(creds[i].AAGUID); info.Icon != "" {
			creds[i].Icon = info.Icon
		}
	}
	return creds, nil
}

func (r *userRepository) DeletePasskey(ctx context.Context, id uint, userID *uint) (bool, error) {
	query := r.db.WithContext(ctx).Where("id = ?", id)
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	res := query.Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *userRepository) UpdatePasskeyName(ctx context.Context, id uint, userID *uint, name string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&models.WebAuthnCredential{}).Where("id = ?", id)
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	res := query.Update("friendly_name", name)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
