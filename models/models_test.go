package models

import (
	"database/sql/driver"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestJSON_ValueAndScan(t *testing.T) {
	// Test nil/empty Value
	var jEmpty JSON
	val, err := jEmpty.Value()
	if err != nil {
		t.Fatalf("unexpected error for empty JSON: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for empty JSON value, got %v", val)
	}

	// Test non-empty Value
	jData := JSON([]byte(`{"key":"value"}`))
	val, err = jData.Value()
	if err != nil {
		t.Fatalf("unexpected error for JSON: %v", err)
	}
	if val != `{"key":"value"}` {
		t.Errorf("expected %s, got %v", `{"key":"value"}`, val)
	}

	// Test Scan nil
	var jScan JSON
	if err := jScan.Scan(nil); err != nil {
		t.Fatalf("failed to scan nil: %v", err)
	}
	if jScan != nil {
		t.Errorf("expected nil after scanning nil, got %v", jScan)
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
		AppImpacted: "Core App",
	}
	if actFallback.GetProjectCode() != "PRJ-01" {
		t.Errorf("expected PRJ-01, got %s", actFallback.GetProjectCode())
	}
	if actFallback.GetProjectName() != "Project Alpha" {
		t.Errorf("expected Project Alpha, got %s", actFallback.GetProjectName())
	}
	if actFallback.GetAppImpacted() != "Core App" {
		t.Errorf("expected Core App, got %s", actFallback.GetAppImpacted())
	}

	// Referenced Project takes precedence
	actRef := DailyActivity{
		ProjectID:   "PRJ-OLD",
		ProjectName: "Name Old",
		AppImpacted: "App Old",
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

func TestDTOs(t *testing.T) {
	// Verify struct instantiations compile and fields are accessible
	resp := APIResponse{Code: 200, Status: "success", Message: "ok", Data: "test"}
	if resp.Code != 200 || resp.Status != "success" {
		t.Errorf("APIResponse mismatch: %+v", resp)
	}

	msg := MessageResponse{Code: 200, Status: "success", Message: "done"}
	if msg.Message != "done" {
		t.Errorf("MessageResponse mismatch: %+v", msg)
	}

	errResp := ErrorResponse{Code: 400, Status: "error", Error: "err", Message: "err"}
	if errResp.Error != "err" {
		t.Errorf("ErrorResponse mismatch: %+v", errResp)
	}

	delResp := DeleteResponse{Code: 200, Status: "success", Deleted: true}
	if !delResp.Deleted {
		t.Errorf("DeleteResponse mismatch: %+v", delResp)
	}

	loginResp := LoginResponse{Token: "jwt-token", User: User{ID: 1}}
	if loginResp.Token != "jwt-token" {
		t.Errorf("LoginResponse mismatch: %+v", loginResp)
	}

	vapidResp := VAPIDKeyResponse{PublicKey: "vapid-key"}
	if vapidResp.PublicKey != "vapid-key" {
		t.Errorf("VAPIDKeyResponse mismatch: %+v", vapidResp)
	}

	originsResp := OriginsResponse{Origins: []string{"https://example.com"}}
	if len(originsResp.Origins) != 1 {
		t.Errorf("OriginsResponse mismatch: %+v", originsResp)
	}

	passkeySess := PasskeySessionResponse{SessionID: "sess-123"}
	if passkeySess.SessionID != "sess-123" {
		t.Errorf("PasskeySessionResponse mismatch: %+v", passkeySess)
	}

	loginReq := LoginRequest{Identifier: "admin", Password: "pwd"}
	if loginReq.Identifier != "admin" {
		t.Errorf("LoginRequest mismatch: %+v", loginReq)
	}

	resetReq := ResetRequest{Token: "tok", Password: "pwd"}
	if resetReq.Token != "tok" {
		t.Errorf("ResetRequest mismatch: %+v", resetReq)
	}

	passkeyReq := BeginPasskeyLoginRequest{Identifier: "user1"}
	if passkeyReq.Identifier != "user1" {
		t.Errorf("BeginPasskeyLoginRequest mismatch: %+v", passkeyReq)
	}

	userReq := CreateUserRequest{Username: "newuser", Email: "new@example.com", Role: RoleUser}
	if userReq.Username != "newuser" {
		t.Errorf("CreateUserRequest mismatch: %+v", userReq)
	}

	updateReq := UpdateUserRequest{}
	if updateReq.Role != nil {
		t.Errorf("UpdateUserRequest mismatch: %+v", updateReq)
	}

	profileReq := ProfileChangeRequestDTO{Name: "Name"}
	if profileReq.Name != "Name" {
		t.Errorf("ProfileChangeRequestDTO mismatch: %+v", profileReq)
	}

	actReq := DailyActivityRequest{Date: "2026-09-01", Status: "P"}
	if actReq.Date != "2026-09-01" {
		t.Errorf("DailyActivityRequest mismatch: %+v", actReq)
	}

	genReq := GenerateRequest{Month: 9, Year: 2026}
	if genReq.Month != 9 {
		t.Errorf("GenerateRequest mismatch: %+v", genReq)
	}

	subReq := SubscribeRequest{Endpoint: "https://push.example.com", Keys: PushKeyPayload{P256dh: "key", Auth: "auth"}}
	if subReq.Endpoint == "" {
		t.Errorf("SubscribeRequest mismatch: %+v", subReq)
	}

	unsubReq := UnsubscribeRequest{Endpoint: "https://push.example.com"}
	if unsubReq.Endpoint == "" {
		t.Errorf("UnsubscribeRequest mismatch: %+v", unsubReq)
	}

	pageResp := PaginatedResponse{
		Code:       200,
		Status:     "success",
		Pagination: PaginationMeta{Page: 1, Limit: 10, TotalRows: 100, TotalPages: 10},
	}
	if pageResp.Pagination.TotalPages != 10 {
		t.Errorf("PaginatedResponse mismatch: %+v", pageResp)
	}

	entry := DailyEntry{Day: 1, Status: "P"}
	if entry.Day != 1 {
		t.Errorf("DailyEntry mismatch: %+v", entry)
	}

	tsReq := TimesheetRequest{Month: 9, Year: 2026, Format: "excel"}
	if tsReq.Month != 9 {
		t.Errorf("TimesheetRequest mismatch: %+v", tsReq)
	}

	holDTO := HolidayDTO{Date: "2026-01-01", Description: "New Year"}
	if holDTO.Date != "2026-01-01" {
		t.Errorf("HolidayDTO mismatch: %+v", holDTO)
	}

	kItem := KemendesaHolidayItem{Date: "2026-01-01", Name: "Tahun Baru"}
	if kItem.Date != "2026-01-01" {
		t.Errorf("KemendesaHolidayItem mismatch: %+v", kItem)
	}

	kResp := KemendesaHolidayResponse{Data: []KemendesaHolidayItem{kItem}}
	if len(kResp.Data) != 1 {
		t.Errorf("KemendesaHolidayResponse mismatch: %+v", kResp)
	}
}
