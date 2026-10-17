package models_test

import (
	"testing"

	"timesheet-backend/models"
)

func TestAuditLog_TableName(t *testing.T) {
	log := models.AuditLog{}
	if log.TableName() != "audit_logs" {
		t.Fatalf("expected audit_logs, got %s", log.TableName())
	}
}
