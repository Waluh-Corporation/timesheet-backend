package repository_test

import (
	"context"
	"testing"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestSetupRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewSetupRepository(tx)
	ctx := context.Background()

	// 1. GetAdminCount initial
	initialAdminCount, err := repo.GetAdminCount(ctx)
	if err != nil {
		t.Fatalf("GetAdminCount failed: %v", err)
	}

	// 2. ExecuteSetup
	admin := &models.User{
		Username:     "setup_admin_repo_test",
		Email:        "setup_admin_repo@example.com",
		Name:         "Setup Admin",
		Role:         models.RoleAdmin,
		PasswordHash: "hashedpass",
		IsActive:     true,
	}
	companies := []models.Company{
		{Code: "  "}, // empty code skipped
		{Code: "COMP_SETUP_1", Name: "Company Setup One"},
		{Code: "COMP_SETUP_2", Name: "Company Setup Two"},
	}
	depts := []models.Department{
		{Code: "  "}, // empty code skipped
		{Code: "DEPT_SETUP_1", Name: "Dept Setup One", Division: "Div A"},
	}
	approvers := []models.Approver{
		{Name: "  "}, // empty name skipped
		{Name: "Approver One", RoleType: models.ApproverRoleTeamLeader, Title: "TL"},
	}

	if err := repo.ExecuteSetup(ctx, admin, companies, depts, approvers); err != nil {
		t.Fatalf("ExecuteSetup failed: %v", err)
	}

	// Verify Admin count increased
	newAdminCount, err := repo.GetAdminCount(ctx)
	if err != nil {
		t.Fatalf("GetAdminCount after setup failed: %v", err)
	}
	if newAdminCount != initialAdminCount+1 {
		t.Fatalf("expected admin count %d, got %d", initialAdminCount+1, newAdminCount)
	}

	// 3. GetSystemSetting
	val, err := repo.GetSystemSetting(ctx, "is_new")
	if err != nil {
		t.Fatalf("GetSystemSetting('is_new') failed: %v", err)
	}
	if val != "N" {
		t.Fatalf("expected is_new='N', got '%s'", val)
	}

	// GetSystemSetting non-existent
	_, err = repo.GetSystemSetting(ctx, "non_existent_key_12345")
	if err == nil {
		t.Fatal("expected error for non-existent system setting key")
	}

	// Re-run ExecuteSetup with duplicate company code to test branch (already exists)
	admin2 := &models.User{
		Username:     "setup_admin_repo_test_2",
		Email:        "setup_admin_repo_2@example.com",
		Name:         "Setup Admin 2",
		Role:         models.RoleAdmin,
		PasswordHash: "hashedpass",
		IsActive:     true,
	}
	duplicateComps := []models.Company{
		{Code: "COMP_SETUP_1", Name: "Company Setup One"}, // already exists
	}
	if err := repo.ExecuteSetup(ctx, admin2, duplicateComps, nil, nil); err != nil {
		t.Fatalf("ExecuteSetup with duplicate company should succeed: %v", err)
	}
}
