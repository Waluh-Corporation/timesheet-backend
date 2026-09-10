package models

// APIResponse represents the standard enterprise response envelope.
type APIResponse struct {
	Code    int         `json:"code" example:"200"`
	Status  string      `json:"status" example:"success"`
	Message string      `json:"message,omitempty" example:"operation successful"`
	Data    interface{} `json:"data,omitempty"`
}

// MessageResponse represents a generic success response message with status code.
type MessageResponse struct {
	Code    int    `json:"code" example:"200"`
	Status  string `json:"status" example:"success"`
	Message string `json:"message" example:"operation successful"`
}

// ErrorResponse represents an API error response with status code.
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`
	Status  string `json:"status" example:"error"`
	Error   string `json:"error" example:"invalid request or unauthorized"`
	Message string `json:"message,omitempty" example:"invalid request or unauthorized"`
}

// DeleteResponse represents a deletion status response with status code.
type DeleteResponse struct {
	Code    int    `json:"code" example:"200"`
	Status  string `json:"status" example:"success"`
	Deleted bool   `json:"deleted" example:"true"`
}

// LoginResponse represents a successful authentication response containing JWT and user profile.
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  User   `json:"user"`
}

// VAPIDKeyResponse carries the Web Push VAPID public application server key.
type VAPIDKeyResponse struct {
	PublicKey string `json:"public_key" example:"BEl62iUYgUivxIkv...`
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

// CreateUserRequest is the payload for administrator user provisioning.
type CreateUserRequest struct {
	Username     string `json:"username" binding:"required,min=3,max=64" example:"john_doe"`
	Email        string `json:"email" binding:"required,email" example:"john.doe@example.com"`
	Role         Role   `json:"role" binding:"required,oneof=admin user" example:"user"`
	Name         string `json:"name" example:"John Doe"`
	MiiID        string `json:"mii_id" example:"MII-00001"`
	Division     string `json:"division" example:"Application Development Division"`
	Department   string `json:"department" example:"Core Banking"`
	DepartmentID *uint  `json:"department_id" example:"1"`
	Site         string `json:"site" example:"Jakarta"`
	Company      string `json:"company" example:"MII"`
	CompanyID    *uint  `json:"company_id" example:"1"`
	Password     string `json:"password" example:"TempPass123!"`
}

// UpdateUserRequest represents the admin-driven user update payload.
type UpdateUserRequest struct {
	Role         *Role   `json:"role" example:"user"`
	IsActive     *bool   `json:"is_active" example:"true"`
	Name         *string `json:"name" example:"John Doe"`
	MiiID        *string `json:"mii_id" example:"MII-00001"`
	Division     *string `json:"division" example:"Application Development Division"`
	Department   *string `json:"department" example:"Core Banking"`
	DepartmentID *uint   `json:"department_id" example:"1"`
	Site         *string `json:"site" example:"Jakarta"`
	Company      *string `json:"company" example:"MII"`
	CompanyID    *uint   `json:"company_id" example:"1"`
}

// ProfileChangeRequestDTO represents a user's self-service profile change submission.
type ProfileChangeRequestDTO struct {
	Name         string `json:"name" example:"John Doe"`
	MiiID        string `json:"mii_id" example:"MII-00001"`
	Division     string `json:"division" example:"Application Development Division"`
	Department   string `json:"department" example:"Core Banking"`
	DepartmentID *uint  `json:"department_id" example:"1"`
	Site         string `json:"site" example:"Jakarta"`
	CompanyID    *uint  `json:"company_id" example:"1"`
}

// DailyActivityRequest represents a single day's timesheet input.
type DailyActivityRequest struct {
	Date         string `json:"date" binding:"required" example:"2026-09-01"`
	StartTime    string `json:"start_time" example:"08:00"`
	EndTime      string `json:"end_time" example:"17:00"`
	Status       string `json:"status" example:"P"`
	Activity     string `json:"activity" example:"Developing core features and API normalization"`
	ProjectName  string `json:"project_name" example:"BNI Direct Cash"`
	ProjectID    string `json:"project_id" example:"P24015"`
	AppImpacted  string `json:"app_impacted" example:"BNI Mobile"`
	ProjectRefID *uint  `json:"project_ref_id" example:"1"`
}

// GenerateRequest specifies parameters to render and download a timesheet workbook.
type GenerateRequest struct {
	Month int `json:"month" binding:"required,min=1,max=12" example:"9"`
	Year  int `json:"year" binding:"required,min=2000,max=9999" example:"2026"`
}

// PushKeyPayload carries the browser push subscription keys.
type PushKeyPayload struct {
	P256dh string `json:"p256dh" binding:"required" example:"BCVxsG6...`
	Auth   string `json:"auth" binding:"required" example:"5Kpqz...`
}

// SubscribeRequest carries the Web Push subscription parameters.
type SubscribeRequest struct {
	Endpoint string         `json:"endpoint" binding:"required" example:"https://fcm.googleapis.com/fcm/send/..."`
	Keys     PushKeyPayload `json:"keys" binding:"required"`
}

// UnsubscribeRequest carries the endpoint to remove from push notifications.
type UnsubscribeRequest struct {
	Endpoint string `json:"endpoint" example:"https://fcm.googleapis.com/fcm/send/..."`
}

