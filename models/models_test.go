package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
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

	// Test NewWebAuthnCredential with Platform attachment and empty transport fallback
	platformCred := &webauthn.Credential{
		ID:        []byte("platform-cred-id"),
		PublicKey: []byte("platform-pub-key"),
		Authenticator: webauthn.Authenticator{
			Attachment: protocol.Platform,
		},
	}
	platformRecord := NewWebAuthnCredential(1, platformCred, "Touch ID")
	libPlatform, err := platformRecord.ToLibrary()
	if err != nil {
		t.Fatalf("ToLibrary on platform record failed: %v", err)
	}
	if len(libPlatform.Transport) != 1 || libPlatform.Transport[0] != protocol.Internal {
		t.Errorf("expected [internal] transport for platform attachment, got %v", libPlatform.Transport)
	}
}

func parseUUIDBytes(s string) []byte {
	var clean string
	for _, c := range s {
		if c != '-' {
			clean += string(c)
		}
	}
	var out []byte
	for i := 0; i+1 < len(clean); i += 2 {
		var b byte
		for j := 0; j < 2; j++ {
			c := clean[i+j]
			b <<= 4
			switch {
			case c >= '0' && c <= '9':
				b |= c - '0'
			case c >= 'a' && c <= 'f':
				b |= c - 'a' + 10
			case c >= 'A' && c <= 'F':
				b |= c - 'A' + 10
			}
		}
		out = append(out, b)
	}
	return out
}

func TestAuthenticatorNameFromAAGUID(t *testing.T) {
	RegisterAAGUIDs(map[string]string{
		"d548826e-79b4-db40-a3d8-11116f7e8349": "Bitwarden",               // gitleaks:allow
		"b87b7a24-9407-4e38-9cfd-d5588cf3b1b6": "1Password",               // gitleaks:allow
		"ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4": "Google Password Manager", // gitleaks:allow
		"fbfc3007-154e-4ecc-8c0b-6e020557d7bd": "iCloud Keychain",         // gitleaks:allow
		"08987058-cadc-4b81-b6e1-30de50dcbe96": "Windows Hello",           // gitleaks:allow
	})

	tests := []struct {
		name     string
		aaguid   []byte
		expected string
	}{
		{
			name:     "Bitwarden canonical",
			aaguid:   parseUUIDBytes("d548826e-79b4-db40-a3d8-11116f7e8349"),
			expected: "Bitwarden",
		},
		{
			name:     "1Password",
			aaguid:   parseUUIDBytes("b87b7a24-9407-4e38-9cfd-d5588cf3b1b6"),
			expected: "1Password",
		},
		{
			name:     "Google Password Manager",
			aaguid:   parseUUIDBytes("ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4"),
			expected: "Google Password Manager",
		},
		{
			name:     "iCloud Keychain",
			aaguid:   parseUUIDBytes("fbfc3007-154e-4ecc-8c0b-6e020557d7bd"),
			expected: "iCloud Keychain",
		},
		{
			name:     "Windows Hello",
			aaguid:   parseUUIDBytes("08987058-cadc-4b81-b6e1-30de50dcbe96"),
			expected: "Windows Hello",
		},
		{
			name:     "All zeros AAGUID",
			aaguid:   make([]byte, 16),
			expected: "Passkey",
		},
		{
			name:     "Invalid length AAGUID",
			aaguid:   []byte("too-short"),
			expected: "Passkey",
		},
		{
			name:     "Unknown AAGUID",
			aaguid:   parseUUIDBytes("12345678-1234-1234-1234-123456789abc"),
			expected: "Passkey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuthenticatorNameFromAAGUID(tt.aaguid)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestNewWebAuthnCredential_DefaultNameFallback(t *testing.T) {
	RegisterAAGUID("d548826e-79b4-db40-a3d8-11116f7e8349", "Bitwarden")

	bwAAGUID := parseUUIDBytes("d548826e-79b4-db40-a3d8-11116f7e8349")
	cred := &webauthn.Credential{
		ID:        []byte("cred-bw"),
		PublicKey: []byte("key-bw"),
		Authenticator: webauthn.Authenticator{
			AAGUID: bwAAGUID,
		},
	}

	// Case 1: Empty friendlyName -> should fallback to "Bitwarden"
	recDefault := NewWebAuthnCredential(1, cred, "")
	if recDefault.FriendlyName != "Bitwarden" {
		t.Errorf("expected 'Bitwarden', got %q", recDefault.FriendlyName)
	}

	// Case 2: Whitespace friendlyName -> should fallback to "Bitwarden"
	recWhitespace := NewWebAuthnCredential(1, cred, "   ")
	if recWhitespace.FriendlyName != "Bitwarden" {
		t.Errorf("expected 'Bitwarden', got %q", recWhitespace.FriendlyName)
	}

	// Case 3: User custom friendlyName -> should retain user set name
	recCustom := NewWebAuthnCredential(1, cred, "My Work Bitwarden")
	if recCustom.FriendlyName != "My Work Bitwarden" {
		t.Errorf("expected 'My Work Bitwarden', got %q", recCustom.FriendlyName)
	}
}

func TestDynamicAAGUIDRegistry(t *testing.T) {
	customAAGUIDStr := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	customAAGUID := parseUUIDBytes(customAAGUIDStr)

	// Register single
	RegisterAAGUID(customAAGUIDStr, "Custom Security Key")
	if name := AuthenticatorNameFromAAGUID(customAAGUID); name != "Custom Security Key" {
		t.Errorf("expected 'Custom Security Key', got %q", name)
	}

	// Batch register
	RegisterAAGUIDs(map[string]string{
		"11111111-2222-3333-4444-555555555555": "Titan Key",
	})
	titanAAGUID := parseUUIDBytes("11111111-2222-3333-4444-555555555555")
	if name := AuthenticatorNameFromAAGUID(titanAAGUID); name != "Titan Key" {
		t.Errorf("expected 'Titan Key', got %q", name)
	}

	// GetKnownAAGUIDs
	known := GetKnownAAGUIDs()
	if known["aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"] != "Custom Security Key" {
		t.Errorf("expected known AAGUID to contain custom key")
	}
	if known["11111111-2222-3333-4444-555555555555"] != "Titan Key" {
		t.Errorf("expected known AAGUID to contain titan key")
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
	// Without ProjectRef, canonical getters return empty strings
	actEmpty := DailyActivity{}
	if actEmpty.GetProjectCode() != "" {
		t.Errorf("expected empty string, got %s", actEmpty.GetProjectCode())
	}
	if actEmpty.GetProjectName() != "" {
		t.Errorf("expected empty string, got %s", actEmpty.GetProjectName())
	}
	if actEmpty.GetAppImpacted() != "" {
		t.Errorf("expected empty string without ProjectRef, got %s", actEmpty.GetAppImpacted())
	}

	// Referenced Project provides canonical attributes
	actRef := DailyActivity{
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

func TestWebAuthnCredential_AuthenticatorRelation(t *testing.T) {
	// Register a known authenticator
	knownUUID := "12345678-1234-1234-1234-123456789abc"
	RegisterAuthenticator(knownUUID, "Relational Test Key", "data:image/svg+xml;icon")

	// 1. Valid known AAGUID
	knownBytes := parseUUIDBytes(knownUUID)
	credKnown := webauthn.Credential{
		ID:        []byte("id-known"),
		PublicKey: []byte("pk-known"),
		Authenticator: webauthn.Authenticator{
			AAGUID: knownBytes,
		},
	}
	modelKnown := NewWebAuthnCredential(1, &credKnown, "")
	if modelKnown.AuthenticatorAAGUID == nil {
		t.Fatalf("expected AuthenticatorAAGUID to be populated, got nil")
	}
	if *modelKnown.AuthenticatorAAGUID != knownUUID {
		t.Errorf("expected AuthenticatorAAGUID %s, got %s", knownUUID, *modelKnown.AuthenticatorAAGUID)
	}
	if modelKnown.FriendlyName != "Relational Test Key" {
		t.Errorf("expected FriendlyName 'Relational Test Key', got %s", modelKnown.FriendlyName)
	}
	if modelKnown.Icon != "data:image/svg+xml;icon" {
		t.Errorf("expected Icon 'data:image/svg+xml;icon', got %s", modelKnown.Icon)
	}

	// 2. Unknown AAGUID
	unknownUUID := "99999999-9999-9999-9999-999999999999"
	unknownBytes := parseUUIDBytes(unknownUUID)
	credUnknown := webauthn.Credential{
		ID:        []byte("id-unknown"),
		PublicKey: []byte("pk-unknown"),
		Authenticator: webauthn.Authenticator{
			AAGUID: unknownBytes,
		},
	}
	modelUnknown := NewWebAuthnCredential(1, &credUnknown, "My Custom Key")
	if modelUnknown.AuthenticatorAAGUID != nil {
		t.Errorf("expected AuthenticatorAAGUID to be nil for unknown authenticator, got %v", *modelUnknown.AuthenticatorAAGUID)
	}
	if modelUnknown.FriendlyName != "My Custom Key" {
		t.Errorf("expected FriendlyName 'My Custom Key', got %s", modelUnknown.FriendlyName)
	}

	// 3. FormatAAGUID edge cases
	if _, ok := FormatAAGUID([]byte{1, 2, 3}); ok {
		t.Error("expected FormatAAGUID to return false for non-16-byte slice")
	}
	if _, ok := FormatAAGUID(make([]byte, 16)); ok {
		t.Error("expected FormatAAGUID to return false for all-zero slice")
	}

	// 4. JSON Serialization omits redundant user_id and nested authenticator object
	jsonBytes, err := json.Marshal(modelKnown)
	if err != nil {
		t.Fatalf("failed to marshal WebAuthnCredential: %v", err)
	}
	jsonStr := string(jsonBytes)
	if strings.Contains(jsonStr, `"user_id"`) {
		t.Errorf("expected json to not contain redundant 'user_id', got: %s", jsonStr)
	}
	if strings.Contains(jsonStr, `"authenticator":`) {
		t.Errorf("expected json to not contain redundant nested 'authenticator' object, got: %s", jsonStr)
	}
}
