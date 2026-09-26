package models_test

import (
	"testing"

	"timesheet-backend/models"
)

func TestTimesheetJob_TableName(t *testing.T) {
	job := models.TimesheetJob{}
	if job.TableName() != "timesheet_jobs" {
		t.Fatalf("expected timesheet_jobs, got %s", job.TableName())
	}
}
