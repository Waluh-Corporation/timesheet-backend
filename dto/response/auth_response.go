package response

import "timesheet-backend/models"

// LoginResponse represents a successful authentication response containing JWT access token, refresh token, and user profile.
type LoginResponse struct {
	Token        string      `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string      `json:"refresh_token,omitempty" example:"8f12c3d4..."`
	User         models.User `json:"user"`
}

// RefreshResponse represents a successful refresh token exchange.
type RefreshResponse struct {
	Token        string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refresh_token" example:"a4b3c2d1..."`
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

// VerifyResetTokenResponse represents the validity check result for a password reset token.
type VerifyResetTokenResponse struct {
	Valid    bool   `json:"valid" example:"true"`
	Status   string `json:"status" example:"valid"`
	Message  string `json:"message" example:"token valid"`
	Email    string `json:"email,omitempty" example:"j***@example.com"`
	Username string `json:"username,omitempty" example:"john_doe"`
}
