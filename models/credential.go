package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/datatypes"
)

// ToLibrary converts a persisted credential back into the shape the webauthn
// library expects during assertion (login) ceremonies.
func (c WebAuthnCredential) ToLibrary() (webauthn.Credential, error) {
	var transports []protocol.AuthenticatorTransport
	if len(c.Transports) > 0 {
		var raw []string
		if err := json.Unmarshal(c.Transports, &raw); err == nil {
			for _, t := range raw {
				transports = append(transports, protocol.AuthenticatorTransport(t))
			}
		}
	}
	return webauthn.Credential{
		ID:              c.CredentialID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Transport:       transports,
		// Restore the backup flags so the library's login-time consistency check
		// (BackupEligible must not change) compares against the recorded value.
		Flags: webauthn.CredentialFlags{
			BackupEligible: c.BackupEligible,
			BackupState:    c.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       c.AAGUID,
			SignCount:    c.SignCount,
			CloneWarning: c.CloneWarning,
		},
	}, nil
}

// AuthenticatorInfo stores authenticator metadata including friendly name and SVG icons.
type AuthenticatorInfo struct {
	Name      string
	IconLight string
	IconDark  string
}

var (
	aaguidMu     sync.RWMutex
	knownAAGUIDs = make(map[string]AuthenticatorInfo)
)

// RegisterAuthenticator dynamically maps an AAGUID canonical string to authenticator info.
func RegisterAuthenticator(aaguid, name, iconLight, iconDark string) {
	aaguidMu.Lock()
	defer aaguidMu.Unlock()
	knownAAGUIDs[strings.ToLower(strings.TrimSpace(aaguid))] = AuthenticatorInfo{
		Name:      strings.TrimSpace(name),
		IconLight: strings.TrimSpace(iconLight),
		IconDark:  strings.TrimSpace(iconDark),
	}
}

// RegisterAAGUID dynamically maps an AAGUID canonical string to an authenticator name.
func RegisterAAGUID(aaguid, name string) {
	RegisterAuthenticator(aaguid, name, "", "")
}

// RegisterAAGUIDs dynamically registers a batch of AAGUID to name mappings.
func RegisterAAGUIDs(mapping map[string]string) {
	for k, v := range mapping {
		RegisterAuthenticator(k, v, "", "")
	}
}

// GetKnownAAGUIDs returns a copy of all currently registered AAGUID name mappings.
func GetKnownAAGUIDs() map[string]string {
	aaguidMu.RLock()
	defer aaguidMu.RUnlock()
	out := make(map[string]string, len(knownAAGUIDs))
	for k, v := range knownAAGUIDs {
		out[k] = v.Name
	}
	return out
}

// FormatAAGUID returns the canonical hyphenated UUID string of a 16-byte AAGUID.
// Returns false if aaguid is invalid (not 16 bytes or all zeroes).
func FormatAAGUID(aaguid []byte) (string, bool) {
	if len(aaguid) != 16 {
		return "", false
	}
	isAllZero := true
	for _, b := range aaguid {
		if b != 0 {
			isAllZero = false
			break
		}
	}
	if isAllZero {
		return "", false
	}
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		aaguid[0], aaguid[1], aaguid[2], aaguid[3],
		aaguid[4], aaguid[5],
		aaguid[6], aaguid[7],
		aaguid[8], aaguid[9],
		aaguid[10], aaguid[11],
		aaguid[12], aaguid[13], aaguid[14], aaguid[15],
	), true
}

// GetAuthenticatorInfo returns full metadata (name, icons) for a 16-byte AAGUID.
func GetAuthenticatorInfo(aaguid []byte) AuthenticatorInfo {
	formatted, ok := FormatAAGUID(aaguid)
	if !ok {
		return AuthenticatorInfo{Name: "Passkey"}
	}

	aaguidMu.RLock()
	defer aaguidMu.RUnlock()
	if info, exists := knownAAGUIDs[strings.ToLower(formatted)]; exists && info.Name != "" {
		return info
	}
	return AuthenticatorInfo{Name: "Passkey"}
}

// AuthenticatorNameFromAAGUID returns the friendly name of the authenticator
// based on its 16-byte AAGUID. If unrecognised or zeroed, defaults to "Passkey".
func AuthenticatorNameFromAAGUID(aaguid []byte) string {
	return GetAuthenticatorInfo(aaguid).Name
}

// NewWebAuthnCredential builds a persistable record from a freshly registered
// library credential. If friendlyName is empty, it automatically detects and
// assigns the default name from the authenticator's AAGUID.
func NewWebAuthnCredential(userID uint, cred *webauthn.Credential, friendlyName string) WebAuthnCredential {
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	raw, _ := json.Marshal(transports)

	info := GetAuthenticatorInfo(cred.Authenticator.AAGUID)
	name := strings.TrimSpace(friendlyName)
	if name == "" {
		name = info.Name
		if name == "" {
			name = "Passkey"
		}
	}

	var authAAGUIDPtr *string
	if formatted, ok := FormatAAGUID(cred.Authenticator.AAGUID); ok {
		lower := strings.ToLower(formatted)
		aaguidMu.RLock()
		_, exists := knownAAGUIDs[lower]
		aaguidMu.RUnlock()
		if exists {
			authAAGUIDPtr = &lower
		}
	}

	return WebAuthnCredential{
		UserID:              userID,
		CredentialID:        cred.ID,
		PublicKey:           cred.PublicKey,
		AttestationType:     cred.AttestationType,
		AAGUID:              cred.Authenticator.AAGUID,
		AuthenticatorAAGUID: authAAGUIDPtr,
		SignCount:           cred.Authenticator.SignCount,
		CloneWarning:        cred.Authenticator.CloneWarning,
		BackupEligible:      cred.Flags.BackupEligible,
		BackupState:         cred.Flags.BackupState,
		Transports:          datatypes.JSON(raw),
		FriendlyName:        name,
		IconLight:           info.IconLight,
		IconDark:            info.IconDark,
	}
}
