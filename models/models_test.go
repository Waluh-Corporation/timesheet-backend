package models

import (
	"database/sql/driver"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/datatypes"
)

func TestJSON_ValueAndScan(t *testing.T) {
	// Test nil/empty Value
	var jEmpty datatypes.JSON
	val, err := jEmpty.Value()
	if err != nil {
		t.Fatalf("unexpected error for empty JSON: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for empty JSON value, got %v", val)
	}

	// Test non-empty Value
	jData := datatypes.JSON([]byte(`{"key":"value"}`))
	val, err = jData.Value()
	if err != nil {
		t.Fatalf("unexpected error for JSON: %v", err)
	}
	if val != `{"key":"value"}` {
		t.Errorf("expected %s, got %v", `{"key":"value"}`, val)
	}

	// Test Scan nil
	var jScan datatypes.JSON
	if err := jScan.Scan(nil); err != nil {
		t.Fatalf("failed to scan nil: %v", err)
	}
	if string(jScan) != "null" {
		t.Errorf("expected null after scanning nil, got %v", jScan)
	}

	// Test Scan []byte
	inputBytes := []byte(`["usb","nfc"]`)
	if err := jScan.Scan(inputBytes); err != nil {
		t.Fatalf("failed to scan []byte: %v", err)
	}
	if string(jScan) != string(inputBytes) {
		t.Errorf("expected %s, got %s", inputBytes, jScan)
	}

	// Test Scan string
	inputStr := `["ble","internal"]`
	if err := jScan.Scan(inputStr); err != nil {
		t.Fatalf("failed to scan string: %v", err)
	}
	if string(jScan) != inputStr {
		t.Errorf("expected %s, got %s", inputStr, jScan)
	}

	// Test Scan invalid type
	if err := jScan.Scan(12345); err == nil {
		t.Errorf("expected error scanning int, got nil")
	}

	// Driver Valuer interface check
	var _ driver.Valuer = jData
}

func TestWebAuthnCredential_ToLibraryAndNew(t *testing.T) {
	cred := &webauthn.Credential{
		ID:              []byte("cred-id-123"),
		PublicKey:       []byte("public-key-456"),
		AttestationType: "none",
		Transport: []protocol.AuthenticatorTransport{
			protocol.USB,
			protocol.NFC,
		},
		Flags: webauthn.CredentialFlags{
			BackupEligible: true,
			BackupState:    false,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       []byte("aaguid-0000-1111"),
			SignCount:    42,
			CloneWarning: false,
		},
	}

	record := NewWebAuthnCredential(1, cred, "My Security Key")
	if record.UserID != 1 {
		t.Errorf("expected UserID 1, got %d", record.UserID)
	}
	if record.FriendlyName != "My Security Key" {
		t.Errorf("expected FriendlyName 'My Security Key', got %s", record.FriendlyName)
	}
	if !record.BackupEligible {
		t.Errorf("expected BackupEligible to be true")
	}

	libCred, err := record.ToLibrary()
	if err != nil {
		t.Fatalf("ToLibrary returned error: %v", err)
	}
	if string(libCred.ID) != string(cred.ID) {
		t.Errorf("expected ID %s, got %s", cred.ID, libCred.ID)
	}
	if len(libCred.Transport) != 2 {
		t.Fatalf("expected 2 transports, got %d", len(libCred.Transport))
	}
	if libCred.Transport[0] != protocol.USB || libCred.Transport[1] != protocol.NFC {
		t.Errorf("transports mismatch: %v", libCred.Transport)
	}
	if libCred.Flags.BackupEligible != true || libCred.Flags.BackupState != false {
		t.Errorf("backup flags mismatch: %+v", libCred.Flags)
	}
	if libCred.Authenticator.SignCount != 42 {
		t.Errorf("sign count mismatch: %d", libCred.Authenticator.SignCount)
	}

	// Test ToLibrary with empty transports
	emptyRecord := WebAuthnCredential{
		CredentialID: []byte("empty-id"),
	}
	emptyLibCred, err := emptyRecord.ToLibrary()
	if err != nil {
		t.Fatalf("ToLibrary with empty transports failed: %v", err)
	}
	if len(emptyLibCred.Transport) != 0 {
		t.Errorf("expected 0 transports, got %d", len(emptyLibCred.Transport))
	}
}

func TestUser_WebAuthnMethods(t *testing.T) {
	u := User{
		ID:       10,
		Username: "alice",
		Name:     "Alice Wonderland",
		Credentials: []WebAuthnCredential{
			{
				CredentialID: []byte("cred-1"),
				PublicKey:    []byte("key-1"),
			},
		},
	}

	// WebAuthnID
	idBytes := u.WebAuthnID()
	if len(idBytes) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(idBytes))
	}
	if idBytes[0] != 10 {
		t.Errorf("expected first byte 10, got %d", idBytes[0])
	}

	// WebAuthnName
	if u.WebAuthnName() != "alice" {
		t.Errorf("expected WebAuthnName 'alice', got %s", u.WebAuthnName())
	}

	// WebAuthnDisplayName with Name
	if u.WebAuthnDisplayName() != "Alice Wonderland" {
		t.Errorf("expected 'Alice Wonderland', got %s", u.WebAuthnDisplayName())
	}

	// WebAuthnDisplayName fallback to Username
	uNoName := User{Username: "bob"}
	if uNoName.WebAuthnDisplayName() != "bob" {
		t.Errorf("expected 'bob', got %s", uNoName.WebAuthnDisplayName())
	}

	// WebAuthnIcon
	if u.WebAuthnIcon() != "" {
		t.Errorf("expected empty string, got %s", u.WebAuthnIcon())
	}

	// WebAuthnCredentials
	creds := u.WebAuthnCredentials()
	if len(creds) != 1 {
		t.Fatalf("expected 1 cred, got %d", len(creds))
	}
	if string(creds[0].ID) != "cred-1" {
		t.Errorf("expected cred-1, got %s", creds[0].ID)
	}
}

func TestDailyActivity_ProjectGetters(t *testing.T) {
	// Fallback to denormalized fields
	actFallback := DailyActivity{
		ProjectID:   "PRJ-01",
		ProjectName: "Project Alpha",
	}
	if actFallback.GetProjectCode() != "PRJ-01" {
		t.Errorf("expected PRJ-01, got %s", actFallback.GetProjectCode())
	}
	if actFallback.GetProjectName() != "Project Alpha" {
		t.Errorf("expected Project Alpha, got %s", actFallback.GetProjectName())
	}
	if actFallback.GetAppImpacted() != "" {
		t.Errorf("expected empty string without ProjectRef, got %s", actFallback.GetAppImpacted())
	}

	// Referenced Project takes precedence and provides canonical AppImpacted
	actRef := DailyActivity{
		ProjectID:   "PRJ-OLD",
		ProjectName: "Name Old",
		ProjectRef: &Project{
			Code:        "PRJ-NEW",
			Name:        "Name New",
			AppImpacted: "App New",
		},
	}
	if actRef.GetProjectCode() != "PRJ-NEW" {
		t.Errorf("expected PRJ-NEW, got %s", actRef.GetProjectCode())
	}
	if actRef.GetProjectName() != "Name New" {
		t.Errorf("expected Name New, got %s", actRef.GetProjectName())
	}
	if actRef.GetAppImpacted() != "App New" {
		t.Errorf("expected App New, got %s", actRef.GetAppImpacted())
	}
}

func TestHolidayDTOs(t *testing.T) {
	holDTO := HolidayDTO{Date: "2026-01-01", Description: "New Year"}
	if holDTO.Date != "2026-01-01" {
		t.Errorf("HolidayDTO mismatch: %+v", holDTO)
	}

	kItem := KemendesaHolidayItem{Date: "2026-01-01", Name: "Tahun Baru"}
	kResp := KemendesaHolidayResponse{Data: []KemendesaHolidayItem{kItem}}
	if len(kResp.Data) != 1 {
		t.Errorf("KemendesaHolidayResponse mismatch: %+v", kResp)
	}
}
