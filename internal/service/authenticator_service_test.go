package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"timesheet-backend/internal/service"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

type mockAuthenticatorRepo struct {
	syncedEntries map[string]services.CommunityAAGUIDEntry
	records       []models.AuthenticatorAAGUID
	syncErr       error
	listErr       error
}

func (m *mockAuthenticatorRepo) SyncAAGUIDs(ctx context.Context, entries map[string]services.CommunityAAGUIDEntry) (int, error) {
	if m.syncErr != nil {
		return 0, m.syncErr
	}
	m.syncedEntries = entries
	return len(entries), nil
}

func (m *mockAuthenticatorRepo) ListAuthenticators(ctx context.Context, search string, offset, limit int) ([]models.AuthenticatorAAGUID, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.records, int64(len(m.records)), nil
}

func TestAuthenticatorService_ListAuthenticators(t *testing.T) {
	now := time.Now()
	repo := &mockAuthenticatorRepo{
		records: []models.AuthenticatorAAGUID{
			{AAGUID: "abc", Name: "Key 1", Icon: "icon1", UpdatedAt: now},
			{AAGUID: "def", Name: "Key 2", Icon: "icon2", UpdatedAt: now},
		},
	}
	svc := service.NewAuthenticatorService(repo)

	items, total, err := svc.ListAuthenticators(context.Background(), "", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 items, got total=%d len=%d", total, len(items))
	}
	if items[0].Name != "Key 1" || items[1].AAGUID != "def" {
		t.Fatalf("unexpected item content: %+v", items)
	}

	// Error case
	repo.listErr = errors.New("query failed")
	_, _, err = svc.ListAuthenticators(context.Background(), "", 1, 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
