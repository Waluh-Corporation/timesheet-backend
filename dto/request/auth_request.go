package request

// LoginRequest is the username/email and password authentication payload.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required" example:"john_doe"`
	Password   string `json:"password" binding:"required" example:"MySecretPass123!"`
}

// ForgotRequest triggers a password reset email.
type ForgotRequest struct {
	Email string `json:"email" binding:"required,email" example:"john.doe@example.com"`
}

// ResetRequest completes a password reset.
type ResetRequest struct {
	Token    string `json:"token" binding:"required" example:"a8f9c0e2b1d3..."`
	Password string `json:"password" binding:"required,min=8" example:"NewStrongPass123!"`
}

// BeginPasskeyLoginRequest optionally carries a username/email for targeted passkey login.
type BeginPasskeyLoginRequest struct {
	Identifier string `json:"identifier" example:"john_doe"`
}

// RefreshRequest carries a refresh token to obtain a new access token and rotated refresh token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"8f12c3d4..."`
}

// LogoutRequest optionally carries a refresh token to invalidate on logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" example:"8f12c3d4..."`
}
