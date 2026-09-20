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

	// 8. Test DeleteExpiredRefreshTokens
	olderThan := time.Now().Add(8 * 24 * time.Hour)
	deletedTokens, err := tokenRepo.DeleteExpiredRefreshTokens(ctx, olderThan)
	if err != nil || deletedTokens == 0 {
		t.Errorf("expected DeleteExpiredRefreshTokens to delete tokens, deleted: %d, err: %v", deletedTokens, err)
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

	// Should no longer be valid via FindValidResetTokenByHash
	_, err = tokenRepo.FindValidResetTokenByHash(ctx, hash)
	if err == nil {
		t.Errorf("expected consumed reset token to not be returned by FindValidResetTokenByHash")
	}

	// But should still be found via FindResetTokenByHash to inspect its status
	consumedFound, err := tokenRepo.FindResetTokenByHash(ctx, hash)
	if err != nil || consumedFound == nil {
		t.Fatalf("expected FindResetTokenByHash to return consumed token, err: %v", err)
	}
	if consumedFound.UsedAt == nil {
		t.Errorf("expected UsedAt to be set on consumed token")
	}

	// Non-existent hash should return error
	_, err = tokenRepo.FindResetTokenByHash(ctx, "nonexistent-hash")
	if err == nil {
		t.Errorf("expected error for nonexistent hash")
	}

	// Test InvalidateResetTokensByUserID
	_, hash2, _ := auth.GenerateResetToken()
	_ = tokenRepo.CreateResetToken(ctx, &models.PasswordResetToken{
		UserID:    user.ID,
		TokenType: "password_reset",
		TokenHash: hash2,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err := tokenRepo.InvalidateResetTokensByUserID(ctx, user.ID, time.Now()); err != nil {
		t.Fatalf("failed to invalidate reset tokens by user ID: %v", err)
	}

	// Test CreateResetTokenWithInvalidation and GetLatestResetTokenByUserID
	_, hash3, _ := auth.GenerateResetToken()
	tok3 := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenType: "password_reset",
		TokenHash: hash3,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := tokenRepo.CreateResetTokenWithInvalidation(ctx, tok3, time.Now()); err != nil {
		t.Fatalf("failed to create reset token with invalidation: %v", err)
	}

	latest, err := tokenRepo.GetLatestResetTokenByUserID(ctx, user.ID)
	if err != nil || latest == nil {
		t.Fatalf("failed to get latest reset token: %v", err)
	}
	if latest.TokenHash != hash3 {
		t.Errorf("expected latest token hash to be %s, got %s", hash3, latest.TokenHash)
	}

	// Another token with invalidation should invalidate tok3
	_, hash4, _ := auth.GenerateResetToken()
	tok4 := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenType: "password_reset",
		TokenHash: hash4,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	invalidationTime := time.Now()
	if err := tokenRepo.CreateResetTokenWithInvalidation(ctx, tok4, invalidationTime); err != nil {
		t.Fatalf("failed to create reset token with invalidation: %v", err)
	}

	// tok3 should now be invalidated
	tok3Check, err := tokenRepo.FindResetTokenByHash(ctx, hash3)
	if err != nil || tok3Check.UsedAt == nil {
		t.Errorf("expected tok3 to be invalidated (UsedAt != nil)")
	}

	// tok4 should be valid
	tok4Check, err := tokenRepo.FindValidResetTokenByHash(ctx, hash4)
	if err != nil || tok4Check == nil {
		t.Errorf("expected tok4 to be valid, got err: %v", err)
	}

	// Test DeleteExpiredResetTokens
	deletedResetTokens, err := tokenRepo.DeleteExpiredResetTokens(ctx, time.Now().Add(2*time.Hour), time.Now().Add(2*time.Hour))
	if err != nil || deletedResetTokens == 0 {
		t.Errorf("expected DeleteExpiredResetTokens to delete tokens, deleted: %d, err: %v", deletedResetTokens, err)
	}

	_ = raw
}
