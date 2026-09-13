package repository_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}
	return db
}

func TestUserRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewUserRepository(tx)
	ctx := context.Background()

	// 1. Create user
	user := &models.User{
		Username:     "repo_test_user_unique",
		Email:        "repo_test_unique@example.com",
		Name:         "Repo Test User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash123",
		IsActive:     true,
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("repo.Create failed: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected user ID to be assigned, got 0")
	}

	// 2. FindByID - Found
	found, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.FindByID failed: %v", err)
	}
	if found.Username != user.Username {
		t.Fatalf("expected username %q, got %q", user.Username, found.Username)
	}
	if found.Email != user.Email {
		t.Fatalf("expected email %q, got %q", user.Email, found.Email)
	}

	// 3. FindByID - Not Found
	notFound, err := repo.FindByID(ctx, 99999999)
	if err == nil || notFound != nil {
		t.Fatalf("expected error and nil user for non-existent ID, got user=%v, err=%v", notFound, err)
	}

	// 4. FindByUsernameOrEmail - by Username
	byUser, err := repo.FindByUsernameOrEmail(ctx, "repo_test_user_unique")
	if err != nil {
		t.Fatalf("repo.FindByUsernameOrEmail(username) failed: %v", err)
	}
	if byUser.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, byUser.ID)
	}

	// 5. FindByUsernameOrEmail - by Email
	byEmail, err := repo.FindByUsernameOrEmail(ctx, "repo_test_unique@example.com")
	if err != nil {
		t.Fatalf("repo.FindByUsernameOrEmail(email) failed: %v", err)
	}
	if byEmail.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, byEmail.ID)
	}

	// 6. FindByUsernameOrEmail - Not Found
	notByIdent, err := repo.FindByUsernameOrEmail(ctx, "nonexistent_ident@example.com")
	if err == nil || notByIdent != nil {
		t.Fatalf("expected error and nil user for non-existent identifier, got user=%v, err=%v", notByIdent, err)
	}

	// 7. UpdatePassword
	newHash := "updatedhash123"
	updatedAt := time.Now().Truncate(time.Second)
	err = repo.UpdatePassword(ctx, user.ID, newHash, updatedAt)
	if err != nil {
		t.Fatalf("repo.UpdatePassword failed: %v", err)
	}

	// Verify updated password
	refreshed, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.FindByID failed after password update: %v", err)
	}
	if refreshed.PasswordHash != newHash {
		t.Fatalf("expected password hash %q, got %q", newHash, refreshed.PasswordHash)
	}
}
