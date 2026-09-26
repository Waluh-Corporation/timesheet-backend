package entity

import "time"

// Role defines role-based access control roles.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// ProfileStatus tracks the review state of user profile modifications.
type ProfileStatus string

const (
	ProfileApproved ProfileStatus = "approved"
	ProfilePending  ProfileStatus = "pending"
	ProfileRejected ProfileStatus = "rejected"
)

// User represents the core user account domain entity.
type User struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Username     string
	Email        string
	PasswordHash string
	Role         Role
	IsActive     bool
	Name         string
	BniID        string
	EmployeeID   string
	Division     string
	DivisionID   *uint
	Department   string
	DepartmentID *uint
	GroupName    string
	Position     string
	Site         string
	SiteID       *uint
	Company      string
	CompanyID    *uint
}

// ProfileChangeRequest represents a domain entity for employee self-service profile update requests.
type ProfileChangeRequest struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	UserID       uint
	Status       ProfileStatus
	Name         string
	BniID        string
	EmployeeID   string
	Division     string
	DivisionID   *uint
	Department   string
	DepartmentID *uint
	GroupName    string
	Position     string
	Site         string
	SiteID       *uint
	CompanyID    *uint
	Email        string
	Notes        string
	ReviewedBy   *uint
	ReviewedAt   *time.Time
}
