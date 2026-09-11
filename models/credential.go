package models

import (
	"encoding/json"

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

// NewWebAuthnCredential builds a persistable record from a freshly registered
// library credential.
func NewWebAuthnCredential(userID uint, cred *webauthn.Credential, friendlyName string) WebAuthnCredential {
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	raw, _ := json.Marshal(transports)
	return WebAuthnCredential{
		UserID:          userID,
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		CloneWarning:    cred.Authenticator.CloneWarning,
		BackupEligible:  cred.Flags.BackupEligible,
		BackupState:     cred.Flags.BackupState,
		Transports:      datatypes.JSON(raw),
		FriendlyName:    friendlyName,
	}
}
