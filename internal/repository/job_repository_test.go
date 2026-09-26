package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestJobRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	repo := repository.NewJobRepository(tx)
	ctx := context.Background()

	// Seed user for foreign key
	user := &models.User{
		Username:     "job_repo_test_user",
		Email:        "job_repo@example.com",
		Name:         "Job Repo Test",
		Role:         models.RoleUser,
		PasswordHash: "secret123",
		IsActive:     true,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	jobID := uuid.New().String()
	job := &models.TimesheetJob{
		ID:        jobID,
		UserID:    user.ID,
		Month:     7,
		Year:      2026,
		Status:    models.JobStatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 1. Create
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// 2. FindByID
	found, err := repo.FindByID(ctx, jobID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found == nil || found.ID != jobID {
		t.Fatalf("expected job %s, got %+v", jobID, found)
	}

	notFound, err := repo.FindByID(ctx, "non-existent-id")
	if err != nil {
		t.Fatalf("FindByID non-existent returned error: %v", err)
	}
	if notFound != nil {
		t.Fatalf("expected nil for non-existent job, got %+v", notFound)
	}

	// 3. FindActiveByIDAndUser
	foundUserJob, err := repo.FindActiveByIDAndUser(ctx, jobID, user.ID)
	if err != nil {
		t.Fatalf("FindActiveByIDAndUser failed: %v", err)
	}
	if foundUserJob == nil {
		t.Fatalf("expected job to be found for user %d", user.ID)
	}

	notFoundUserJob, err := repo.FindActiveByIDAndUser(ctx, jobID, 999999)
	if err != nil {
		t.Fatalf("FindActiveByIDAndUser unexpected error: %v", err)
	}
	if notFoundUserJob != nil {
		t.Fatalf("expected nil for wrong user")
	}

	// 4. HasInFlightJob
	hasInFlight, err := repo.HasInFlightJob(ctx, user.ID, 7, 2026)
	if err != nil {
		t.Fatalf("HasInFlightJob failed: %v", err)
	}
	if !hasInFlight {
		t.Fatalf("expected in-flight job to be true")
	}

	hasInFlightOther, err := repo.HasInFlightJob(ctx, user.ID, 8, 2026)
	if err != nil {
		t.Fatalf("HasInFlightJob other failed: %v", err)
	}
	if hasInFlightOther {
		t.Fatalf("expected false for month 8")
	}

	// 5. UpdateStatus
	expiresAt := time.Now().Add(24 * time.Hour)
	err = repo.UpdateStatus(ctx, jobID, models.JobStatusCompleted, "file-key-1", "https://example.com/dl", "no error", &expiresAt)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// 6. UpdateStatusWithToken
	tokenExpiresAt := time.Now().Add(2 * time.Hour)
	err = repo.UpdateStatusWithToken(ctx, jobID, models.JobStatusCompleted, "file-key-2", "https://example.com/token-dl", "", &expiresAt, "test-download-token", 5, &tokenExpiresAt)
	if err != nil {
		t.Fatalf("UpdateStatusWithToken failed: %v", err)
	}

	// 7. FindByDownloadToken
	foundByToken, err := repo.FindByDownloadToken(ctx, "test-download-token")
	if err != nil {
		t.Fatalf("FindByDownloadToken failed: %v", err)
	}
	if foundByToken == nil || foundByToken.DownloadToken == nil || *foundByToken.DownloadToken != "test-download-token" {
		t.Fatalf("expected job found by token, got %+v", foundByToken)
	}

	notFoundByToken, err := repo.FindByDownloadToken(ctx, "invalid-token")
	if err != nil {
		t.Fatalf("FindByDownloadToken invalid returned error: %v", err)
	}
	if notFoundByToken != nil {
		t.Fatalf("expected nil for invalid token")
	}

	// 8. IncrementDownloadCount
	if err := repo.IncrementDownloadCount(ctx, jobID); err != nil {
		t.Fatalf("IncrementDownloadCount failed: %v", err)
	}
	refreshed, _ := repo.FindByID(ctx, jobID)
	if refreshed.DownloadCount != 1 {
		t.Fatalf("expected download_count 1, got %d", refreshed.DownloadCount)
	}

	// 9. RenewDownloadToken
	newExpires := time.Now().Add(4 * time.Hour)
	if err := repo.RenewDownloadToken(ctx, jobID, "renewed-token-123", 10, newExpires); err != nil {
		t.Fatalf("RenewDownloadToken failed: %v", err)
	}
	refreshed, _ = repo.FindByID(ctx, jobID)
	if refreshed.DownloadToken == nil || *refreshed.DownloadToken != "renewed-token-123" || refreshed.DownloadCount != 0 || refreshed.MaxDownloads != 10 {
		t.Fatalf("RenewDownloadToken values mismatch: %+v", refreshed)
	}

	// 10. FindActiveCompletedJob
	completedJob, err := repo.FindActiveCompletedJob(ctx, user.ID, 7, 2026)
	if err != nil {
		t.Fatalf("FindActiveCompletedJob failed: %v", err)
	}
	if completedJob == nil {
		t.Fatalf("expected active completed job to be found")
	}

	notCompleted, err := repo.FindActiveCompletedJob(ctx, user.ID, 9, 2026)
	if err != nil {
		t.Fatalf("FindActiveCompletedJob non-existent failed: %v", err)
	}
	if notCompleted != nil {
		t.Fatalf("expected nil for non-existent completed job")
	}

	// 11. ListByUser
	jobs, total, err := repo.ListByUser(ctx, user.ID, -1, -1)
	if err != nil {
		t.Fatalf("ListByUser default pagination failed: %v", err)
	}
	if total < 1 || len(jobs) < 1 {
		t.Fatalf("expected at least 1 job in ListByUser, got total=%d len=%d", total, len(jobs))
	}

	jobsPaged, _, err := repo.ListByUser(ctx, user.ID, 5, 0)
	if err != nil {
		t.Fatalf("ListByUser custom pagination failed: %v", err)
	}
	if len(jobsPaged) == 0 {
		t.Fatalf("expected jobs in paged list")
	}

	// 12. FindExpiredJobs
	futureTime := time.Now().Add(48 * time.Hour)
	expiredJobs, err := repo.FindExpiredJobs(ctx, futureTime, 0)
	if err != nil {
		t.Fatalf("FindExpiredJobs failed: %v", err)
	}
	if len(expiredJobs) == 0 {
		t.Fatalf("expected expired job when queried with future time")
	}

	// 13. Delete
	if err := repo.Delete(ctx, jobID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	deleted, _ := repo.FindByID(ctx, jobID)
	if deleted != nil {
		t.Fatalf("expected job to be deleted, still found %+v", deleted)
	}
}
