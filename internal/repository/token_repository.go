package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// TokenRepository defines the database persistence contract for authentication tokens,
// including refresh tokens and password reset tokens.
type TokenRepository interface {
	// Refresh token operations
	CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error
	FindRefreshTokenByHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uint, revokedAt time.Time) error
	RevokeFamily(ctx context.Context, familyID string, revokedAt time.Time) error
	RevokeUserTokens(ctx context.Context, userID uint, revokedAt time.Time) error
	DeleteExpiredRefreshTokens(ctx context.Context, olderThan time.Time) (int64, error)

	// Password reset token operations
	CreateResetToken(ctx context.Context, token *models.PasswordResetToken) error
	FindValidResetTokenByHash(ctx context.Context, hash string) (*models.PasswordResetToken, error)
	FindResetTokenByHash(ctx context.Context, hash string) (*models.PasswordResetToken, error)
	InvalidateResetTokensByUserID(ctx context.Context, userID uint, at time.Time) error
	ConsumeResetToken(ctx context.Context, tokenID uint, usedAt time.Time, usedIP string) error
	DeleteExpiredResetTokens(ctx context.Context, olderThan time.Time, usedOlderThan time.Time) (int64, error)
}

type tokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository constructs an instance of TokenRepository.
func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *tokenRepository) FindRefreshTokenByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *tokenRepository) RevokeRefreshToken(ctx context.Context, id uint, revokedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", revokedAt).Error
}

func (r *tokenRepository) RevokeFamily(ctx context.Context, familyID string, revokedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", revokedAt).Error
}

func (r *tokenRepository) RevokeUserTokens(ctx context.Context, userID uint, revokedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", revokedAt).Error
}

func (r *tokenRepository) DeleteExpiredRefreshTokens(ctx context.Context, olderThan time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("expires_at < ?", olderThan).
		Or("revoked_at IS NOT NULL AND revoked_at < ?", olderThan).
		Delete(&models.RefreshToken{})
	return res.RowsAffected, res.Error
}

func (r *tokenRepository) CreateResetToken(ctx context.Context, token *models.PasswordResetToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *tokenRepository) FindValidResetTokenByHash(ctx context.Context, hash string) (*models.PasswordResetToken, error) {
	var token models.PasswordResetToken
	if err := r.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", hash, time.Now()).
		First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *tokenRepository) FindResetTokenByHash(ctx context.Context, hash string) (*models.PasswordResetToken, error) {
	var token models.PasswordResetToken
	if err := r.db.WithContext(ctx).
		Where("token_hash = ?", hash).
		First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *tokenRepository) InvalidateResetTokensByUserID(ctx context.Context, userID uint, at time.Time) error {
	return r.db.WithContext(ctx).Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", at).Error
}

func (r *tokenRepository) ConsumeResetToken(ctx context.Context, tokenID uint, usedAt time.Time, usedIP string) error {
	return r.db.WithContext(ctx).Model(&models.PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", tokenID).
		Updates(map[string]interface{}{
			"used_at": usedAt,
			"used_ip": usedIP,
		}).Error
}

func (r *tokenRepository) DeleteExpiredResetTokens(ctx context.Context, olderThan time.Time, usedOlderThan time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("expires_at < ?", olderThan).
		Or("used_at IS NOT NULL AND created_at < ?", usedOlderThan).
		Delete(&models.PasswordResetToken{})
	return res.RowsAffected, res.Error
}
