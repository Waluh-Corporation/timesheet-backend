package repository_test

import (
	"context"
	"testing"
	"time"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestOvertimeRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := &models.User{
		Username: "ot_repo_test_user",
		Email:    "ot_repo_test@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	repo := repository.NewOvertimeRepository(tx)
	ctx := context.Background()

	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	entry := &models.OvertimeEntry{
		UserID:          user.ID,
		Date:            now,
		StartTime:       "17:00",
		EndTime:         "20:00",
		TaskDescription: "Deploy release",
		IsActive:        true,
	}

	// Create
	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// FindByUserAndDate
	found, err := repo.FindByUserAndDate(ctx, user.ID, now)
	if err != nil {
		t.Fatalf("FindByUserAndDate failed: %v", err)
	}
	if found.TaskDescription != "Deploy release" {
		t.Errorf("expected 'Deploy release', got %q", found.TaskDescription)
	}

	// Update
	found.TaskDescription = "Deploy release v2"
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// FindActiveByID
	byId, err := repo.FindActiveByID(ctx, found.ID, user.ID)
	if err != nil {
		t.Fatalf("FindActiveByID failed: %v", err)
	}
	if byId.TaskDescription != "Deploy release v2" {
		t.Errorf("expected 'Deploy release v2', got %q", byId.TaskDescription)
	}

	// FindByUserAndMonth
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	list, err := repo.FindByUserAndMonth(ctx, user.ID, start, end)
	if err != nil {
		t.Fatalf("FindByUserAndMonth failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 entry, got %d", len(list))
	}

	// SoftDelete
	if err := repo.SoftDelete(ctx, found.ID, user.ID); err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}

	// Confirm no longer returned by FindActiveByID
	_, err = repo.FindActiveByID(ctx, found.ID, user.ID)
	if err == nil {
		t.Errorf("expected error after soft delete, got nil")
	}
}
