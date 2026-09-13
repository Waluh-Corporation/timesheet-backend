package repository_test

import (
	"context"
	"testing"
	"time"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestActivityRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewActivityRepository(tx)
	ctx := context.Background()

	// Ensure test user
	user := &models.User{
		Username:     "act_repo_user_unique",
		Email:        "act_repo_user@example.com",
		Name:         "Act Repo User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Ensure test project
	proj := &models.Project{
		Code:        "ACT-PRJ-01",
		Name:        "Test Act Project",
		AppImpacted: "Backend",
		IsActive:    true,
	}
	if err := tx.Create(proj).Error; err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	date, _ := time.Parse("2006-01-02", "2026-09-25")
	act := &models.DailyActivity{
		UserID:       user.ID,
		Date:         date,
		StartTime:    "08:30",
		EndTime:      "17:30",
		Status:       "P",
		Activity:     "Repository test activity",
		ProjectName:  proj.Name,
		ProjectID:    proj.Code,
		ProjectRefID: &proj.ID,
		IsActive:     true,
	}

	// 1. Create
	if err := repo.Create(ctx, act); err != nil {
		t.Fatalf("repo.Create failed: %v", err)
	}
	if act.ID == 0 {
		t.Fatal("expected activity ID to be non-zero")
	}

	// 2. FindActiveByID
	found, err := repo.FindActiveByID(ctx, act.ID)
	if err != nil {
		t.Fatalf("repo.FindActiveByID failed: %v", err)
	}
	if found.Activity != act.Activity {
		t.Errorf("expected activity %q, got %q", act.Activity, found.Activity)
	}

	// 3. FindActiveByUserAndDate
	foundByDate, err := repo.FindActiveByUserAndDate(ctx, user.ID, date)
	if err != nil {
		t.Fatalf("repo.FindActiveByUserAndDate failed: %v", err)
	}
	if foundByDate.ID != act.ID {
		t.Errorf("expected ID %d, got %d", act.ID, foundByDate.ID)
	}

	// 4. Update
	foundByDate.Activity = "Updated repo activity"
	if err := repo.Update(ctx, foundByDate); err != nil {
		t.Fatalf("repo.Update failed: %v", err)
	}

	refreshed, err := repo.FindActiveByID(ctx, act.ID)
	if err != nil || refreshed.Activity != "Updated repo activity" {
		t.Fatalf("repo.Update not reflected, got: %v, err: %v", refreshed, err)
	}

	// 5. ListActiveByUser
	filter := repository.ActivityFilter{
		Page:  1,
		Limit: 10,
	}
	list, total, err := repo.ListActiveByUser(ctx, user.ID, filter)
	if err != nil {
		t.Fatalf("repo.ListActiveByUser failed: %v", err)
	}
	if total < 1 || len(list) < 1 {
		t.Fatalf("expected at least 1 record in list, got total=%d, len=%d", total, len(list))
	}

	// 6. ValidateStatus
	isValid, err := repo.ValidateStatus(ctx, "P")
	if err != nil {
		t.Fatalf("repo.ValidateStatus failed: %v", err)
	}
	if !isValid {
		t.Error("expected status P to be valid")
	}

	// 7. FindActiveProjectByRefID
	foundProj, err := repo.FindActiveProjectByRefID(ctx, proj.ID)
	if err != nil {
		t.Fatalf("repo.FindActiveProjectByRefID failed: %v", err)
	}
	if foundProj.Code != proj.Code {
		t.Errorf("expected project code %s, got %s", proj.Code, foundProj.Code)
	}

	// 8. FindActiveProjectByCodeOrName
	foundByName, err := repo.FindActiveProjectByCodeOrName(ctx, "", proj.Name)
	if err != nil {
		t.Fatalf("repo.FindActiveProjectByCodeOrName failed: %v", err)
	}
	if foundByName.ID != proj.ID {
		t.Errorf("expected project ID %d, got %d", proj.ID, foundByName.ID)
	}
}
