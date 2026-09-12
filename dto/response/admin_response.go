package response

import (
	"time"

	"timesheet-backend/models"
)

// AdminUserResponse represents a concise user record for administrative listings.
type AdminUserResponse struct {
	ID           uint        `json:"id"`
	Username     string      `json:"username"`
	Email        string      `json:"email"`
	Role         models.Role `json:"role"`
	Name         string      `json:"name"`
	BniID        string      `json:"bni_id"`
	EmployeeID   string      `json:"employee_id"`
	Division     string      `json:"division"`
	Department   string      `json:"department"`
	DepartmentID *uint       `json:"department_id,omitempty"`
	Site         string      `json:"site"`
	Company      string      `json:"company"`
	CompanyID    *uint       `json:"company_id,omitempty"`
	IsActive     bool        `json:"is_active"`
}

// AdminProfileChangeResponse represents a concise profile change request for review by administrators.
type AdminProfileChangeResponse struct {
	ID           uint                 `json:"id"`
	UserID       uint                 `json:"user_id"`
	UserName     string               `json:"user_name"`
	UserEmail    string               `json:"user_email"`
	Status       models.ProfileStatus `json:"status"`
	Name         string               `json:"name"`
	BniID        string               `json:"bni_id"`
	EmployeeID   string               `json:"employee_id"`
	Division     string               `json:"division"`
	Department   string               `json:"department"`
	DepartmentID *uint                `json:"department_id,omitempty"`
	Site         string               `json:"site"`
	CompanyID    *uint                `json:"company_id,omitempty"`
	ReviewedBy   *uint                `json:"reviewed_by,omitempty"`
	ReviewerName string               `json:"reviewer_name,omitempty"`
	ReviewedAt   *time.Time           `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
}

// AdminPasskeyResponse represents a concise passkey summary without raw cryptographic byte arrays.
type AdminPasskeyResponse struct {
	ID           uint      `json:"id"`
	FriendlyName string    `json:"friendly_name"`
	CreatedAt    time.Time `json:"created_at"`
}
