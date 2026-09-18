package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"timesheet-backend/auth"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestTokenRepository_RefreshTokenFlow(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	tokenRepo := repository.NewTokenRepository(tx)
	ctx := context.Background()

	// 1. Create a test user
	user := &models.User{
		Username:     "token_repo_user",
		Email:        "token_repo@example.com",
		Name:         "Token Repo User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	familyID := uuid.NewString()
	raw1, hash1, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	token1 := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash1,
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedIP: "127.0.0.1",
		UserAgent: "TestAgent/1.0",
	}

	// 2. Insert Token 1
	if err := tokenRepo.CreateRefreshToken(ctx, token1); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	// 3. Find by hash
	found, err := tokenRepo.FindRefreshTokenByHash(ctx, hash1)
	if err != nil {
		t.Fatalf("failed to find refresh token by hash: %v", err)
	}
	if found.FamilyID != familyID {
		t.Errorf("expected familyID %s, got %s", familyID, found.FamilyID)
	}
	if found.RevokedAt != nil {
		t.Errorf("expected token not to be revoked")
	}

	// 4. Revoke Token 1
	now := time.Now()
	if err := tokenRepo.RevokeRefreshToken(ctx, found.ID, now); err != nil {
		t.Fatalf("failed to revoke refresh token: %v", err)
	}
	revoked, err := tokenRepo.FindRefreshTokenByHash(ctx, hash1)
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("expected token to be marked revoked, err: %v", err)
	}

	// 5. Create Token 2 under the same family
	_, hash2, _ := auth.GenerateRefreshToken()
	token2 := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash2,
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedIP: "127.0.0.1",
		UserAgent: "TestAgent/1.0",
	}
	if err := tokenRepo.CreateRefreshToken(ctx, token2); err != nil {
		t.Fatalf("failed to create token2: %v", err)
	}

	// 6. Test Family Revocation (simulate reuse detection)
	if err := tokenRepo.RevokeFamily(ctx, familyID, time.Now()); err != nil {
		t.Fatalf("failed to revoke family: %v", err)
	}
	token2Found, err := tokenRepo.FindRefreshTokenByHash(ctx, hash2)
	if err != nil || token2Found.RevokedAt == nil {
		t.Fatalf("expected token2 in family to be revoked, err: %v", err)
	}

	// 7. Test RevokeUserTokens
	_, hash3, _ := auth.GenerateRefreshToken()
	token3 := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash3,
		FamilyID:  uuid.NewString(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	_ = tokenRepo.CreateRefreshToken(ctx, token3)
	if err := tokenRepo.RevokeUserTokens(ctx, user.ID, time.Now()); err != nil {
		t.Fatalf("failed to revoke user tokens: %v", err)
	}
	token3Found, _ := tokenRepo.FindRefreshTokenByHash(ctx, hash3)
	if token3Found == nil || token3Found.RevokedAt == nil {
		t.Errorf("expected token3 to be revoked by RevokeUserTokens")
	}

	_ = raw1
}

func TestTokenRepository_ResetTokenFlow(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	tokenRepo := repository.NewTokenRepository(tx)
	ctx := context.Background()

	user := &models.User{
		Username:     "reset_repo_user",
		Email:        "reset_repo@example.com",
		Name:         "Reset Repo User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	raw, hash, err := auth.GenerateResetToken()
	if err != nil {
		t.Fatalf("failed to generate reset token: %v", err)
	}

	resetToken := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenType: "password_reset",
		TokenHash: hash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedIP: "127.0.0.1",
	}

	if err := tokenRepo.CreateResetToken(ctx, resetToken); err != nil {
		t.Fatalf("failed to create reset token: %v", err)
	}

	found, err := tokenRepo.FindValidResetTokenByHash(ctx, hash)
	if err != nil || found == nil {
		t.Fatalf("expected to find valid reset token: %v", err)
	}

	if err := tokenRepo.ConsumeResetToken(ctx, found.ID, time.Now(), "127.0.0.1"); err != nil {
		t.Fatalf("failed to consume reset token: %v", err)
	}

	// Should no longer be valid
	_, err = tokenRepo.FindValidResetTokenByHash(ctx, hash)
	if err == nil {
		t.Errorf("expected consumed reset token to not be returned by FindValidResetTokenByHash")
	}

	_ = raw
}
