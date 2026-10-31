package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// UserAccountRepository is re-exported from domain.UserAccountRepository.
type UserAccountRepository = domain.UserAccountRepository

// ProfileChangeRepository is re-exported from domain.ProfileChangeRepository.
type ProfileChangeRepository = domain.ProfileChangeRepository

// PasskeyRepository is re-exported from domain.PasskeyRepository.
type PasskeyRepository = domain.PasskeyRepository

// UserRepository is re-exported from domain.UserRepository for clean architecture consistency.
type UserRepository = domain.UserRepository

const (
	queryID        = "id = ?"
	queryUserID    = "user_id = ?"
	orderCreatedAt = "created_at desc"
)

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository constructs a GORM implementation of UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// NewUserAccountRepository constructs a GORM implementation of UserAccountRepository.
func NewUserAccountRepository(db *gorm.DB) UserAccountRepository {
	return NewUserRepository(db)
}

// NewProfileChangeRepository constructs a GORM implementation of ProfileChangeRepository.
func NewProfileChangeRepository(db *gorm.DB) ProfileChangeRepository {
	return NewUserRepository(db)
}

// NewPasskeyRepository constructs a GORM implementation of PasskeyRepository.
func NewPasskeyRepository(db *gorm.DB) PasskeyRepository {
	return NewUserRepository(db)
}

func (r *userRepository) getDB(ctx context.Context) *gorm.DB {
	if r == nil {
		return ExtractTx(ctx, nil)
	}
	return ExtractTx(ctx, r.db)
}

// WithinTransaction executes fn within an ACID database transaction using the repository's underlying database.
func (r *userRepository) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if r == nil || r.db == nil {
		return errors.New("user repository database connection is nil")
	}
	return NewGormTransactor(r.db).WithinTransaction(ctx, fn)
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.getDB(ctx).Where(queryID, id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.getDB(ctx).Scopes(models.ActiveOnly).
		Preload("CompanyRel").
		Preload("SiteRel").
		Preload("DepartmentRel").
		Preload("DivisionRel").
		Where(queryID, id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByIDWithCredentials(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.getDB(ctx).Preload("Credentials").Where(queryID, id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error) {
	var u models.User
	if err := r.getDB(ctx).Where("username = ? OR email = ?", identifier, identifier).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByUsernameOrEmailWithCredentials(ctx context.Context, identifier string) (*models.User, error) {
	var u models.User
	if err := r.getDB(ctx).Preload("Credentials").
		Where("username = ? OR email = ?", identifier, identifier).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	if err := r.getDB(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByEmailExcludingUser(ctx context.Context, email string, excludeUserID uint) (*models.User, error) {
	var u models.User
	if err := r.getDB(ctx).Where("email = ? AND id != ?", email, excludeUserID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return r.getDB(ctx).Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return r.getDB(ctx).Save(user).Error
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error {
	return r.getDB(ctx).Model(&models.User{}).Where(queryID, id).Updates(map[string]interface{}{
		"password_hash": passwordHash,
		"updated_at":    updatedAt,
	}).Error
}

func (r *userRepository) UpdateSignatureFileKey(ctx context.Context, id uint, fileKey string) error {
	return r.getDB(ctx).Model(&models.User{}).Where(queryID, id).Updates(map[string]interface{}{
		"signature_file_key": fileKey,
		"updated_at":         time.Now(),
	}).Error
}

func (r *userRepository) SoftDelete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&models.User{}).Where("id = ? AND is_active = true", id).Updates(map[string]interface{}{
		"is_active":  false,
		"updated_at": time.Now(),
	}).Error
}

func (r *userRepository) Anonymize(ctx context.Context, id uint) error {
	redactedName := fmt.Sprintf("ANONYMIZED_USER_%d", id)
	redactedEmail := fmt.Sprintf("deleted_%d@timesheet.local", id)
	redactedUsername := fmt.Sprintf("deleted_user_%d", id)

	return r.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"name":          redactedName,
			"username":      redactedUsername,
			"email":         redactedEmail,
			"bni_id":        "",
			"employee_id":   "",
			"password_hash": "",
			"is_active":     false,
			"is_anonymized": true,
			"updated_at":    time.Now(),
		}

		if err := tx.Model(&models.User{}).Where(queryID, id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where(queryUserID, id).Delete(&models.WebAuthnCredential{}).Error; err != nil {
			return err
		}
		if err := tx.Where(queryUserID, id).Delete(&models.PushSubscription{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ProfileChangeRequest{}).
			Where("user_id = ? AND status = ?", id, models.ProfilePending).
			Updates(map[string]interface{}{
				"status": models.ProfileRejected,
				"notes":  "account anonymized",
			}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *userRepository) ListUsers(ctx context.Context, isActive *bool) ([]models.User, error) {
	var users []models.User
	query := r.getDB(ctx).Order(orderCreatedAt)
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	err := query.Find(&users).Error
	return users, err
}

func (r *userRepository) CreateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	return r.getDB(ctx).Create(change).Error
}

func (r *userRepository) ListProfileChanges(ctx context.Context, userID *uint, status string) ([]models.ProfileChangeRequest, error) {
	var changes []models.ProfileChangeRequest
	q := r.getDB(ctx).Preload("User").Preload("Reviewer").Preload("CompanyRel", models.ActiveOnly).Preload("DepartmentRel", models.ActiveOnly).Order(orderCreatedAt)
	if userID != nil {
		q = q.Where(queryUserID, *userID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&changes).Error
	return changes, err
}

func (r *userRepository) FindProfileChangeByID(ctx context.Context, id uint) (*models.ProfileChangeRequest, error) {
	var change models.ProfileChangeRequest
	q := r.getDB(ctx).Where(queryID, id)
	if HasTx(ctx) {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&change).Error; err != nil {
		return nil, err
	}
	return &change, nil
}

func (r *userRepository) UpdateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	return r.getDB(ctx).Save(change).Error
}

func (r *userRepository) CreatePasskeyCredential(ctx context.Context, cred *models.WebAuthnCredential) error {
	return r.getDB(ctx).Create(cred).Error
}

func (r *userRepository) UpdatePasskeySignCount(ctx context.Context, credID []byte, signCount uint32, backupState bool) error {
	return r.getDB(ctx).Model(&models.WebAuthnCredential{}).
		Where("credential_id = ?", credID).
		Updates(map[string]interface{}{
			"sign_count":   signCount,
			"backup_state": backupState,
		}).Error
}

func (r *userRepository) ListPasskeysByUserID(ctx context.Context, userID uint) ([]models.WebAuthnCredential, error) {
	var creds []models.WebAuthnCredential
	if err := r.getDB(ctx).Preload("Authenticator").Where(queryUserID, userID).
		Order(orderCreatedAt).Find(&creds).Error; err != nil {
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
	query := r.getDB(ctx).Where(queryID, id)
	if userID != nil {
		query = query.Where(queryUserID, *userID)
	}
	res := query.Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *userRepository) UpdatePasskeyName(ctx context.Context, id uint, userID *uint, name string) (bool, error) {
	query := r.getDB(ctx).Model(&models.WebAuthnCredential{}).Where(queryID, id)
	if userID != nil {
		query = query.Where(queryUserID, *userID)
	}
	res := query.Update("friendly_name", name)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
