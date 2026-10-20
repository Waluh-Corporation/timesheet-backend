package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestSubmissionRepository_Timesheet(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	repo := repository.NewSubmissionRepository(tx)
	ctx := context.Background()

	user := &models.User{
		Username:     "ts_sub_user_" + uuid.New().String()[:8],
		Email:        "ts_sub_" + uuid.New().String()[:8] + "@example.com",
		Name:         "TS Submission User",
		Role:         models.RoleUser,
		PasswordHash: "hashed",
		IsActive:     true,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 1. Initial lock check -> should not be locked
	locked, err := repo.IsTimesheetLocked(ctx, user.ID, 10, 2026)
	if err != nil {
		t.Fatalf("IsTimesheetLocked failed: %v", err)
	}
	if locked {
		t.Fatal("expected timesheet not to be locked initially")
	}

	// 2. Upsert Timesheet Submission
	token := uuid.New().String()
	now := time.Now()
	sub := &models.TimesheetSubmission{
		ID:                uuid.New().String(),
		UserID:            user.ID,
		Month:             10,
		Year:              2026,
		Status:            models.SubmissionStatusSigned,
		SignedAt:          &now,
		VerificationToken: token,
		DocumentHash:      "hash-12345",
		AuditMetadata:     datatypes.JSON([]byte(`{"ip":"127.0.0.1"}`)),
	}

	if err := repo.UpsertTimesheetSubmission(ctx, sub); err != nil {
		t.Fatalf("UpsertTimesheetSubmission failed: %v", err)
	}

	// 3. FindByToken
	found, err := repo.FindTimesheetByToken(ctx, token)
	if err != nil {
		t.Fatalf("FindTimesheetByToken failed: %v", err)
	}
	if found == nil || found.VerificationToken != token {
		t.Fatalf("expected token %s, got %+v", token, found)
	}

	// 4. FindTimesheetSubmission
	foundSub, err := repo.FindTimesheetSubmission(ctx, user.ID, 10, 2026)
	if err != nil {
		t.Fatalf("FindTimesheetSubmission failed: %v", err)
	}
	if foundSub == nil || foundSub.Status != models.SubmissionStatusSigned {
		t.Fatalf("expected status signed, got %+v", foundSub)
	}

	// 5. Check lock -> should now be locked
	locked, err = repo.IsTimesheetLocked(ctx, user.ID, 10, 2026)
	if err != nil {
		t.Fatalf("IsTimesheetLocked failed: %v", err)
	}
	if !locked {
		t.Fatal("expected timesheet to be locked after signing")
	}
}

func TestSubmissionRepository_BeritaAcara(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	repo := repository.NewSubmissionRepository(tx)
	ctx := context.Background()

	employee := &models.User{
		Username:     "ba_emp_" + uuid.New().String()[:8],
		Email:        "ba_emp_" + uuid.New().String()[:8] + "@example.com",
		Name:         "BA Employee",
		Role:         models.RoleUser,
		PasswordHash: "hashed",
		IsActive:     true,
	}
	witness := &models.User{
		Username:     "ba_wit_" + uuid.New().String()[:8],
		Email:        "ba_wit_" + uuid.New().String()[:8] + "@example.com",
		Name:         "BA Witness",
		Role:         models.RoleUser,
		PasswordHash: "hashed",
		IsActive:     true,
	}
	if err := userRepo.Create(ctx, employee); err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if err := userRepo.Create(ctx, witness); err != nil {
		t.Fatalf("create witness: %v", err)
	}

	// 1. Initial lock check
	locked, err := repo.IsBeritaAcaraLocked(ctx, employee.ID, 10, 2026)
	if err != nil {
		t.Fatalf("IsBeritaAcaraLocked: %v", err)
	}
	if locked {
		t.Fatal("expected BA not locked initially")
	}

	// 2. Submit BA
	baID := uuid.New().String()
	token := uuid.New().String()
	empSignTime := time.Now()
	sub := &models.BeritaAcaraSubmission{
		ID:                baID,
		UserID:            employee.ID,
		WitnessID:         witness.ID,
		Month:             10,
		Year:              2026,
		Status:            models.SubmissionStatusPendingWitness,
		EmployeeSignedAt:  &empSignTime,
		VerificationToken: token,
	}

	if err := repo.UpsertBeritaAcaraSubmission(ctx, sub); err != nil {
		t.Fatalf("UpsertBeritaAcaraSubmission failed: %v", err)
	}

	// 3. ListPendingWitnessRequests
	pending, err := repo.ListPendingWitnessRequests(ctx, witness.ID)
	if err != nil {
		t.Fatalf("ListPendingWitnessRequests failed: %v", err)
	}
	if len(pending) == 0 {
		t.Fatal("expected at least 1 pending request for witness")
	}
	if pending[0].ID != baID {
		t.Errorf("expected pending request ID %s, got %s", baID, pending[0].ID)
	}

	// 4. FindBeritaAcaraByID
	found, err := repo.FindBeritaAcaraByID(ctx, baID)
	if err != nil {
		t.Fatalf("FindBeritaAcaraByID: %v", err)
	}
	if found == nil || found.ID != baID {
		t.Fatalf("expected BA %s, got %+v", baID, found)
	}

	// 5. FindBeritaAcaraByToken
	foundByToken, err := repo.FindBeritaAcaraByToken(ctx, token)
	if err != nil {
		t.Fatalf("FindBeritaAcaraByToken: %v", err)
	}
	if foundByToken == nil || foundByToken.VerificationToken != token {
		t.Fatalf("expected BA with token %s, got %+v", token, foundByToken)
	}

	// 6. Check locked -> should be locked in pending_witness status
	locked, err = repo.IsBeritaAcaraLocked(ctx, employee.ID, 10, 2026)
	if err != nil {
		t.Fatalf("IsBeritaAcaraLocked: %v", err)
	}
	if !locked {
		t.Fatal("expected BA locked during pending_witness")
	}
}
