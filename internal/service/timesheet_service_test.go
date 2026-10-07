package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	masterRepo := repository.NewMasterRepository(tx)
	svc := service.NewTimesheetService(userRepo, actRepo, otRepo, masterRepo, nil)
	if tsImpl, ok := svc.(interface{ SetNowFunc(func() time.Time) }); ok {
		tsImpl.SetNowFunc(func() time.Time {
			return time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
		})
	}
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

	t.Run("Overtime_Validation", func(t *testing.T) {
		testOvertimeValidation(t, svc, ctx, user.ID)
	})
	t.Run("Overtime_Lifecycle", func(t *testing.T) {
		testOvertimeLifecycle(t, svc, ctx, user.ID)
	})
	t.Run("Workbook_Validation", func(t *testing.T) {
		testWorkbookValidation(t, svc, ctx, user.ID)
	})
	t.Run("Workbook_Generation", func(t *testing.T) {
		testWorkbookGenerationSuccess(t, svc, tx, ctx, user.ID)
	})
}

func testOvertimeValidation(t *testing.T, svc service.TimesheetService, ctx context.Context, userID uint) {
	t.Helper()
	if err := svc.UpsertOvertime(ctx, userID, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil req, got %v", err)
	}
	invalidDateReq := &request.OvertimeRequest{Date: "invalid-date"}
	if err := svc.UpsertOvertime(ctx, userID, invalidDateReq); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid date, got %v", err)
	}
	otInvalidTime := &request.OvertimeRequest{
		Date:            "2026-06-16",
		StartTime:       "20:00",
		EndTime:         "18:00",
		TaskDescription: "Invalid overtime hours",
	}
	if err := svc.UpsertOvertime(ctx, userID, otInvalidTime); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for start > end, got %v", err)
	}
}

func testOvertimeLifecycle(t *testing.T, svc service.TimesheetService, ctx context.Context, userID uint) {
	t.Helper()
	validReq := &request.OvertimeRequest{
		Date:            "2026-06-15",
		StartTime:       "17:00",
		EndTime:         "21:00",
		TaskDescription: "Overtime Task 1",
	}
	if err := svc.UpsertOvertime(ctx, userID, validReq); err != nil {
		t.Fatalf("UpsertOvertime create failed: %v", err)
	}

	validReq.TaskDescription = "Overtime Task 1 Updated"
	if err := svc.UpsertOvertime(ctx, userID, validReq); err != nil {
		t.Fatalf("UpsertOvertime update failed: %v", err)
	}

	ots, err := svc.ListMonthlyOvertimes(ctx, userID, 6, 2026)
	if err != nil {
		t.Fatalf("ListMonthlyOvertimes failed: %v", err)
	}
	if len(ots) != 1 || ots[0].TaskDescription != "Overtime Task 1 Updated" {
		t.Fatalf("unexpected overtime list: %+v", ots)
	}

	if _, err := svc.ListMonthlyOvertimes(ctx, userID, 0, 0); err != nil {
		t.Fatalf("ListMonthlyOvertimes with defaults failed: %v", err)
	}

	otID := ots[0].ID
	if err := svc.DeleteOvertime(ctx, otID, userID); err != nil {
		t.Fatalf("DeleteOvertime failed: %v", err)
	}
	if err := svc.DeleteOvertime(ctx, 999999, userID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func testWorkbookValidation(t *testing.T, svc service.TimesheetService, ctx context.Context, userID uint) {
	t.Helper()
	if _, _, err := svc.GenerateWorkbook(ctx, 999999, 6, 2026); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent user, got %v", err)
	}
	if _, _, err := svc.GenerateWorkbook(ctx, userID, 13, 2026); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for month 13, got %v", err)
	}
	if _, _, err := svc.GenerateWorkbook(ctx, userID, 6, 1999); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for year 1999, got %v", err)
	}
	if _, _, err := svc.GenerateWorkbook(ctx, userID, 6, 2026); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when activities are empty, got %v", err)
	}
}

func testWorkbookGenerationSuccess(t *testing.T, svc service.TimesheetService, tx *gorm.DB, ctx context.Context, userID uint) {
	t.Helper()
	act := &models.DailyActivity{
		UserID:    userID,
		Date:      time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "Feature development",
		IsActive:  true,
	}
	if err := tx.Create(act).Error; err != nil {
		t.Fatalf("failed to create activity: %v", err)
	}

	wb, filename, err := svc.GenerateWorkbook(ctx, userID, 6, 2026)
	if err != nil {
		t.Fatalf("GenerateWorkbook failed: %v", err)
	}
	if len(wb) == 0 || filename == "" {
		t.Fatalf("expected non-empty workbook and filename, got len %d, filename %s", len(wb), filename)
	}
}
