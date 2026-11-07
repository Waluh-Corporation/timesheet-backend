package request

import "encoding/json"

// UploadSignatureRequest represents the payload to upload or draw a transparent signature.
type UploadSignatureRequest struct {
	// SignatureBase64 is a Base64-encoded PNG image (can include data:image/png;base64, prefix).
	SignatureBase64 string `json:"signature_base64" binding:"required" example:"data:image/png;base64,iVBORw0KGgo..."`
}

// SignTimesheetRequest represents the payload to sign a monthly timesheet.
type SignTimesheetRequest struct {
	Month int `json:"month" binding:"required,min=1,max=12" example:"10"`
	Year  int `json:"year" binding:"required,min=2000,max=2100" example:"2026"`
}

// RevokeTimesheetRequest represents the payload to revoke a previously signed timesheet.
type RevokeTimesheetRequest struct {
	Month int `json:"month" binding:"required,min=1,max=12" example:"10"`
	Year  int `json:"year" binding:"required,min=2000,max=2100" example:"2026"`
}

// SubmitBeritaAcaraRequest represents the payload to submit a Berita Acara for witness sign-off.
type SubmitBeritaAcaraRequest struct {
	Month     int  `json:"month" binding:"required,min=1,max=12" example:"10"`
	Year      int  `json:"year" binding:"required,min=2000,max=2100" example:"2026"`
	WitnessID uint `json:"witness_id" binding:"omitempty,gt=0" example:"2"`
}

// WebAuthnAssertionPayload carries the cryptographic authenticator assertion from client-side WebAuthn ceremonies.
type WebAuthnAssertionPayload struct {
	CredentialID      string `json:"credential_id"`
	AuthenticatorData string `json:"authenticator_data"`
	ClientDataJSON    string `json:"client_data_json"`
	Signature         string `json:"signature"`
	UserHandle        string `json:"user_handle,omitempty"`
}

// UnmarshalJSON enables backwards and camelCase compatibility for WebAuthnAssertionPayload.
func (p *WebAuthnAssertionPayload) UnmarshalJSON(data []byte) error {
	type Alias WebAuthnAssertionPayload
	aux := &struct {
		AltCredID string `json:"credentialId"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if p.CredentialID == "" && aux.AltCredID != "" {
		p.CredentialID = aux.AltCredID
	}
	return nil
}

// WitnessActionRequest represents the action taken by a witness (approve & sign or reject with notes).
type WitnessActionRequest struct {
	Action    string                    `json:"action" binding:"required,oneof=approve reject" example:"approve"`
	Notes     string                    `json:"notes,omitempty" example:"Mohon perbaiki tanggal kejadian nomor 3"`
	Assertion *WebAuthnAssertionPayload `json:"assertion,omitempty"`
}
