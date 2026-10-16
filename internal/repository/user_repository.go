package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// UserRepository is re-exported from domain.UserRepository for clean architecture consistency.
type UserRepository = domain.UserRepository

const (
	queryWhereID        = "id = ?"
	queryWhereUserID    = "user_id = ?"
	queryOrderCreatedAt = "created_at desc"
)

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository constructs a GORM implementation of UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where(queryWhereID, id).First(&u).Error; err != nil {
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
		Where(queryWhereID, id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByIDWithCredentials(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Preload("Credentials").Where(queryWhereID, id).First(&u).Error; err != nil {
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

func (r *userRepository) FindByEmailExcludingUser(ctx context.Context, email string, excludeUserID uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("email = ? AND id != ?", email, excludeUserID).First(&u).Error; err != nil {
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
	return r.db.WithContext(ctx).Model(&models.User{}).Where(queryWhereID, id).Updates(map[string]interface{}{
		"password_hash": passwordHash,
		"updated_at":    updatedAt,
	}).Error
}

func (r *userRepository) SoftDelete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ? AND is_active = true", id).Updates(map[string]interface{}{
		"is_active":  false,
		"updated_at": time.Now(),
	}).Error
}

func (r *userRepository) Anonymize(ctx context.Context, id uint) error {
	redactedName := fmt.Sprintf("ANONYMIZED_USER_%d", id)
	redactedEmail := fmt.Sprintf("deleted_%d@timesheet.local", id)
	redactedUsername := fmt.Sprintf("deleted_user_%d", id)

	updates := map[string]interface{}{
		"name":          redactedName,
		"username":      redactedUsername,
		"email":         redactedEmail,
		"bni_id":        "",
		"employee_id":   "",
		"password_hash": "",
		"is_active":     false,
		"updated_at":    time.Now(),
	}

	return r.db.WithContext(ctx).Model(&models.User{}).Where(queryWhereID, id).Updates(updates).Error
}

func (r *userRepository) ListUsers(ctx context.Context, isActive *bool) ([]models.User, error) {
	var users []models.User
	query := r.db.WithContext(ctx).Order(queryOrderCreatedAt)
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	err := query.Find(&users).Error
	return users, err
}

func (r *userRepository) CreateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	return r.db.WithContext(ctx).Create(change).Error
}

func (r *userRepository) ListProfileChanges(ctx context.Context, userID *uint, status string) ([]models.ProfileChangeRequest, error) {
	var changes []models.ProfileChangeRequest
	q := r.db.WithContext(ctx).Preload("User").Preload("Reviewer").Preload("CompanyRel", models.ActiveOnly).Preload("DepartmentRel", models.ActiveOnly).Order(queryOrderCreatedAt)
	if userID != nil {
		q = q.Where(queryWhereUserID, *userID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&changes).Error
	return changes, err
}

func (r *userRepository) FindProfileChangeByID(ctx context.Context, id uint) (*models.ProfileChangeRequest, error) {
	var change models.ProfileChangeRequest
	if err := r.db.WithContext(ctx).Where(queryWhereID, id).First(&change).Error; err != nil {
		return nil, err
	}
	return &change, nil
}

func (r *userRepository) UpdateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	return r.db.WithContext(ctx).Save(change).Error
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
	if err := r.db.WithContext(ctx).Preload("Authenticator").Where(queryWhereUserID, userID).
		Order(queryOrderCreatedAt).Find(&creds).Error; err != nil {
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
	query := r.db.WithContext(ctx).Where(queryWhereID, id)
	if userID != nil {
		query = query.Where(queryWhereUserID, *userID)
	}
	res := query.Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *userRepository) UpdatePasskeyName(ctx context.Context, id uint, userID *uint, name string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&models.WebAuthnCredential{}).Where(queryWhereID, id)
	if userID != nil {
		query = query.Where(queryWhereUserID, *userID)
	}
	res := query.Update("friendly_name", name)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *userRepository) ListWitnesses(ctx context.Context, excludeUserID uint, search string) ([]models.User, error) {
	var users []models.User
	query := r.db.WithContext(ctx).Scopes(models.ActiveOnly).
		Where("role = ? AND id != ?", models.RoleUser, excludeUserID)
	if search = strings.TrimSpace(search); search != "" {
		pattern := "%" + search + "%"
		query = query.Where("name ILIKE ? OR username ILIKE ? OR employee_id ILIKE ? OR bni_id ILIKE ?",
			pattern, pattern, pattern, pattern)
	}
	if err := query.Order("name asc, username asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
