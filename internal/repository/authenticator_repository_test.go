package repository_test

import (
	"context"
	"testing"

	"timesheet-backend/internal/repository"
	"timesheet-backend/services"
)

func TestAuthenticatorRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewAuthenticatorRepository(tx)
	ctx := context.Background()

	// 1. SyncAAGUIDs with empty entries
	count, err := repo.SyncAAGUIDs(ctx, nil)
	if err != nil {
		t.Fatalf("SyncAAGUIDs with nil returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}

	// 2. SyncAAGUIDs with items
	entries := map[string]services.CommunityAAGUIDEntry{
		"  ": {Name: "Blank AAGUID"},
		"a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d": {
			Name: "Test Authenticator 1",
			Icon: "https://example.com/icon1.png",
		},
		"b2c3d4e5-f6a7-8b9c-0d1e-2f3a4b5c6d7e": {
			Name: "Test Authenticator 2",
			Icon: "https://example.com/icon2.png",
		},
	}

	count, err = repo.SyncAAGUIDs(ctx, entries)
	if err != nil {
		t.Fatalf("SyncAAGUIDs failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 synced records, got %d", count)
	}

	// Re-sync with update
	entries["a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"] = services.CommunityAAGUIDEntry{
		Name: "Updated Authenticator 1",
		Icon: "https://example.com/icon1_updated.png",
	}
	count, err = repo.SyncAAGUIDs(ctx, entries)
	if err != nil {
		t.Fatalf("SyncAAGUIDs update failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 updated records, got %d", count)
	}

	// 3. ListAuthenticators without search
	list, total, err := repo.ListAuthenticators(ctx, "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuthenticators failed: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected at least 2 authenticators, got total %d", total)
	}
	if len(list) == 0 {
		t.Fatal("expected non-empty list of authenticators")
	}

	// 4. ListAuthenticators with search by name
	filteredList, filteredTotal, err := repo.ListAuthenticators(ctx, "Updated Authenticator 1", 0, 10)
	if err != nil {
		t.Fatalf("ListAuthenticators with search failed: %v", err)
	}
	if filteredTotal != 1 || len(filteredList) != 1 {
		t.Fatalf("expected 1 match for search, got total %d, count %d", filteredTotal, len(filteredList))
	}
	if filteredList[0].Name != "Updated Authenticator 1" {
		t.Fatalf("expected name 'Updated Authenticator 1', got '%s'", filteredList[0].Name)
	}

	// 5. ListAuthenticators with search by AAGUID
	filteredByAAGUID, _, err := repo.ListAuthenticators(ctx, "b2c3d4e5", 0, 10)
	if err != nil {
		t.Fatalf("ListAuthenticators by AAGUID failed: %v", err)
	}
	if len(filteredByAAGUID) != 1 {
		t.Fatalf("expected 1 match by aaguid substring, got %d", len(filteredByAAGUID))
	}

	// 6. ListAuthenticators without limit
	allList, allTotal, err := repo.ListAuthenticators(ctx, "", 0, 0)
	if err != nil {
		t.Fatalf("ListAuthenticators without limit failed: %v", err)
	}
	if int64(len(allList)) != allTotal {
		t.Fatalf("expected allList len %d == allTotal %d", len(allList), allTotal)
	}
}
