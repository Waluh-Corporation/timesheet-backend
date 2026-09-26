package entity

import "time"

// AuthenticatorAAGUID represents a WebAuthn authenticator model in the AAGUID registry.
type AuthenticatorAAGUID struct {
	AAGUID    string
	Name      string
	Icon      string
	UpdatedAt time.Time
}
