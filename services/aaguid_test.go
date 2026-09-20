package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/models"
)

func TestCommunityAAGUID_FetchAndSync(t *testing.T) {
	// 1. Setup mock HTTP server for community registry
	mockData := map[string]CommunityAAGUIDEntry{
		"ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4": {
			Name:      "Google Password Manager",
			IconLight: "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=",
			IconDark:  "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=",
		},
		"42a048a9-4b68-45a8-aa5a-cfb3d4a462ec": {
			Name:      "Bitwarden",
			IconLight: "data:image/svg+xml;base64,Yml0d2FyZGVu",
			IconDark:  "data:image/svg+xml;base64,Yml0d2FyZGVu",
		},
		"invalid-uuid": {
			Name: "Should Be Skipped",
		},
		"11111111-2222-3333-4444-555555555555": {
			Name: "", // empty name should be skipped
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockData)
	}))
	defer srv.Close()

	// Override URL to mock server
	origURL := CommunityAAGUIDURL
	CommunityAAGUIDURL = srv.URL
	defer func() { CommunityAAGUIDURL = origURL }()

	// 2. Test FetchCommunityAAGUIDs
	ctx := context.Background()
	results, err := FetchCommunityAAGUIDs(ctx)
	if err != nil {
		t.Fatalf("expected no error fetching aaguids, got %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 valid aaguids, got %d", len(results))
	}
	if results["42a048a9-4b68-45a8-aa5a-cfb3d4a462ec"].Name != "Bitwarden" {
		t.Errorf("expected Bitwarden, got %s", results["42a048a9-4b68-45a8-aa5a-cfb3d4a462ec"].Name)
	}

	// 3. Test SyncCommunityAAGUIDsToDB with DB transaction rollback
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}

	tx := db.Begin()
	defer tx.Rollback()

	totalSynced, err := SyncCommunityAAGUIDsToDB(ctx, tx)
	if err != nil {
		t.Fatalf("expected no error syncing to db, got %v", err)
	}
	if totalSynced != 2 {
		t.Errorf("expected 2 synced rows, got %d", totalSynced)
	}

	// Verify in-memory registry was updated
	parsedUUID, pErr := uuid.Parse("42a048a9-4b68-45a8-aa5a-cfb3d4a462ec")
	if pErr != nil {
		t.Fatalf("failed to parse uuid: %v", pErr)
	}
	info := models.GetAuthenticatorInfo(parsedUUID[:])
	if info.Name != "Bitwarden" {
		t.Errorf("expected in-memory registry to contain Bitwarden, got %+v", info)
	}
}

func TestCommunityAAGUID_ErrorHandling(t *testing.T) {
	// 1. Nil DB
	_, err := SyncCommunityAAGUIDsToDB(context.Background(), nil)
	if err == nil {
		t.Error("expected error with nil db")
	}

	// 2. HTTP Server 500 error
	srv500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv500.Close()

	origURL := CommunityAAGUIDURL
	CommunityAAGUIDURL = srv500.URL
	defer func() { CommunityAAGUIDURL = origURL }()

	_, err = FetchCommunityAAGUIDs(context.Background())
	if err == nil {
		t.Error("expected error when server returns 500")
	}

	// 3. Malformed JSON
	srvBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer srvBadJSON.Close()

	CommunityAAGUIDURL = srvBadJSON.URL
	_, err = FetchCommunityAAGUIDs(context.Background())
	if err == nil {
		t.Error("expected error when server returns malformed JSON")
	}
}
