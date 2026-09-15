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
	FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error
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

func (r *userRepository) FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("username = ? OR email = ?", identifier, identifier).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password_hash": passwordHash,
		"updated_at":    updatedAt,
	}).Error
}
