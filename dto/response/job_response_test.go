package response_test

import (
	"testing"
	"time"

	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

func TestToTimesheetJobResponse(t *testing.T) {
	// Test nil job
	nilResp := response.ToTimesheetJobResponse(nil)
	if nilResp.ID != "" {
		t.Fatalf("expected empty ID for nil job, got %s", nilResp.ID)
	}

	// Test valid job
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	job := &models.TimesheetJob{
		ID:           "test-job-uuid",
		UserID:       42,
		Month:        6,
		Year:         2026,
		Status:       models.JobStatusCompleted,
		ExpiresAt:    &expiresAt,
		ErrorMessage: "sample error",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	resp := response.ToTimesheetJobResponse(job)
	if resp.ID != job.ID || resp.UserID != job.UserID || resp.Month != job.Month || resp.Year != job.Year ||
		resp.Status != job.Status || resp.ExpiresAt != job.ExpiresAt || resp.ErrorMessage != job.ErrorMessage {
		t.Fatalf("mismatched conversion: %+v vs %+v", resp, job)
	}
}
