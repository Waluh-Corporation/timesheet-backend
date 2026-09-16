package service_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

func setupTestDBForService(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}
	return db
}

func TestTimesheetService_OvertimeAndWorkbook(t *testing.T) {
	db := setupTestDBForService(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	actRepo := repository.NewActivityRepository(tx)
	otRepo := repository.NewOvertimeRepository(tx)
	svc := service.NewTimesheetService(tx, userRepo, actRepo, otRepo, nil)
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Username:   "ts_svc_user",
		Email:      "ts_svc@example.com",
		Role:       models.RoleUser,
		Name:       "Timesheet User",
		Company:    "sdd",
		IsActive:   true,
		EmployeeID: "EMP-TS-1",
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. UpsertOvertime validation
	if err := svc.UpsertOvertime(ctx, user.ID, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil req, got %v", err)
	}
	invalidDateReq := &request.OvertimeRequest{Date: "invalid-date"}
	if err := svc.UpsertOvertime(ctx, user.ID, invalidDateReq); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid date, got %v", err)
	}

	// 2. UpsertOvertime create
	nowStr := "2026-06-15"
	validReq := &request.OvertimeRequest{
		Date:            nowStr,
		StartTime:       "17:00",
		EndTime:         "21:00",
		TaskDescription: "Overtime Task 1",
	}
	if err := svc.UpsertOvertime(ctx, user.ID, validReq); err != nil {
		t.Fatalf("UpsertOvertime create failed: %v", err)
	}

	// 3. UpsertOvertime update
	validReq.TaskDescription = "Overtime Task 1 Updated"
	if err := svc.UpsertOvertime(ctx, user.ID, validReq); err != nil {
		t.Fatalf("UpsertOvertime update failed: %v", err)
	}

	// 4. ListMonthlyOvertimes
	ots, err := svc.ListMonthlyOvertimes(ctx, user.ID, 6, 2026)
	if err != nil {
		t.Fatalf("ListMonthlyOvertimes failed: %v", err)
	}
	if len(ots) != 1 || ots[0].TaskDescription != "Overtime Task 1 Updated" {
		t.Fatalf("unexpected overtime list: %+v", ots)
	}

	// ListMonthlyOvertimes with 0 month/year (defaults)
	_, err = svc.ListMonthlyOvertimes(ctx, user.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListMonthlyOvertimes with defaults failed: %v", err)
	}

	// 5. DeleteOvertime
	otID := ots[0].ID
	if err := svc.DeleteOvertime(ctx, otID, user.ID); err != nil {
		t.Fatalf("DeleteOvertime failed: %v", err)
	}
	if err := svc.DeleteOvertime(ctx, 999999, user.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// 6. GenerateWorkbook
	// User not found
	if _, _, err := svc.GenerateWorkbook(ctx, 999999, 6, 2026); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent user, got %v", err)
	}

	// Invalid month / year
	if _, _, err := svc.GenerateWorkbook(ctx, user.ID, 13, 2026); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for month 13, got %v", err)
	}
	if _, _, err := svc.GenerateWorkbook(ctx, user.ID, 6, 1999); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for year 1999, got %v", err)
	}

	// Valid workbook generation for SDD company
	wb, filename, err := svc.GenerateWorkbook(ctx, user.ID, 6, 2026)
	if err != nil {
		t.Fatalf("GenerateWorkbook failed: %v", err)
	}
	if len(wb) == 0 || filename == "" {
		t.Fatalf("expected non-empty workbook and filename, got len %d, filename %s", len(wb), filename)
	}
}
