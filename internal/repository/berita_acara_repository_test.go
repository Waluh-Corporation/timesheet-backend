package repository_test

import (
	"context"
	"testing"
	"time"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestBeritaAcaraRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewBeritaAcaraRepository(tx)
	ctx := context.Background()

	// 1. Create test user
	user := &models.User{
		Username:     "ba_repo_user_unique",
		Email:        "ba_repo_user@example.com",
		Name:         "BA Repo User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// 2. Create test approvers
	tl := &models.Approver{
		Name:     "Lead Tester",
		RoleType: models.ApproverRoleTeamLeader,
		Title:    "TL",
		IsActive: true,
	}
	if err := tx.Create(tl).Error; err != nil {
		t.Fatalf("failed to create test team leader: %v", err)
	}

	dh := &models.Approver{
		Name:     "Head Tester",
		RoleType: models.ApproverRoleDepartmentHead,
		Title:    "DH",
		IsActive: true,
	}
	if err := tx.Create(dh).Error; err != nil {
		t.Fatalf("failed to create test department head: %v", err)
	}

	date, _ := time.Parse("2006-01-02", "2026-09-10")
	item := &models.BeritaAcara{
		UserID:           user.ID,
		Date:             date,
		Day:              "Kamis",
		StartTime:        "08:00",
		EndTime:          "17:00",
		Keterangan:       "Lupa tap in",
		TeamLeaderID:     &tl.ID,
		DepartmentHeadID: &dh.ID,
		IsActive:         true,
	}

	// Test Create
	if err := repo.Create(ctx, item); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if item.ID == 0 {
		t.Fatal("expected assigned ID, got 0")
	}

	// Test FindActiveByID
	found, err := repo.FindActiveByID(ctx, item.ID)
	if err != nil || found == nil {
		t.Fatalf("FindActiveByID failed: %v", err)
	}
	if found.Keterangan != "Lupa tap in" {
		t.Errorf("expected keterangan 'Lupa tap in', got %s", found.Keterangan)
	}
	if found.TeamLeader == nil || found.TeamLeader.Name != "Lead Tester" {
		t.Errorf("expected preloaded TeamLeader 'Lead Tester'")
	}

	// Test FindActiveByUserAndDate
	byDate, err := repo.FindActiveByUserAndDate(ctx, user.ID, date)
	if err != nil || byDate == nil {
		t.Fatalf("FindActiveByUserAndDate failed: %v", err)
	}
	if byDate.ID != item.ID {
		t.Errorf("expected ID %d, got %d", item.ID, byDate.ID)
	}

	// Test Update
	found.Keterangan = "Lupa tap in dan tap out"
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	afterUpdate, _ := repo.FindActiveByID(ctx, item.ID)
	if afterUpdate.Keterangan != "Lupa tap in dan tap out" {
		t.Errorf("expected updated keterangan, got %s", afterUpdate.Keterangan)
	}

	// Test ListActiveByUser
	filter := repository.BeritaAcaraFilter{
		Page:  1,
		Limit: 10,
	}
	items, count, err := repo.ListActiveByUser(ctx, user.ID, filter)
	if err != nil {
		t.Fatalf("ListActiveByUser failed: %v", err)
	}
	if count != 1 || len(items) != 1 {
		t.Errorf("expected 1 item, got count=%d, len=%d", count, len(items))
	}

	// Test FindActiveForMonth
	mItems, err := repo.FindActiveForMonth(ctx, user.ID, 2026, 9)
	if err != nil {
		t.Fatalf("FindActiveForMonth failed: %v", err)
	}
	if len(mItems) != 1 {
		t.Errorf("expected 1 month item, got %d", len(mItems))
	}
}
