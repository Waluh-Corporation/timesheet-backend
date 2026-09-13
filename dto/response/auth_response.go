package response

// LoginResponse represents a successful authentication response containing JWT token.
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// OriginsResponse returns allowed WebAuthn related origins.
type OriginsResponse struct {
	Origins []string `json:"origins" example:"https://timesheet.example.com"`
}

// PasskeySessionResponse returns the WebAuthn session ID and ceremonial options.
type PasskeySessionResponse struct {
	SessionID string      `json:"session_id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	Options   interface{} `json:"options"`
}
