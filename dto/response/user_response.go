package response

import (
	"time"

	"timesheet-backend/models"
)

// UserResponse represents a user profile payload excluding sensitive credentials.
type UserResponse struct {
	ID           uint        `json:"id" example:"1"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Username     string      `json:"username" example:"john_doe"`
	Email        string      `json:"email" example:"john.doe@example.com"`
	Role         models.Role `json:"role" example:"user"`
	IsActive     bool        `json:"is_active" example:"true"`
	Name         string      `json:"name" example:"John Doe"`
	BniID        string      `json:"bni_id" example:"12345678"`
	EmployeeID   string      `json:"employee_id" example:"EMP-001"`
	Division     string      `json:"division" example:"Application Development Division"`
	Department   string      `json:"department" example:"Core Banking"`
	DepartmentID *uint       `json:"department_id,omitempty" example:"1"`
	GroupName    string      `json:"group_name,omitempty" example:"SDD"`
	Position     string      `json:"position,omitempty" example:"Software Engineer"`
	Site         string      `json:"site" example:"Jakarta"`
	Company      string      `json:"company" example:"MII"`
	CompanyID    *uint       `json:"company_id,omitempty" example:"1"`
}

// ToUserResponse maps a models.User entity to a safe UserResponse DTO.
func ToUserResponse(u *models.User) UserResponse {
	return UserResponse{
		ID:           u.ID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		Username:     u.Username,
		Email:        u.Email,
		Role:         u.Role,
		IsActive:     u.IsActive,
		Name:         u.Name,
		BniID:        u.BniID,
		EmployeeID:   u.EmployeeID,
		Division:     u.Division,
		Department:   u.Department,
		DepartmentID: u.DepartmentID,
		GroupName:    u.GroupName,
		Position:     u.Position,
		Site:         u.Site,
		Company:      u.Company,
		CompanyID:    u.CompanyID,
	}
}

// CreateUserData represents the response data payload when an admin creates a new user.
type CreateUserData struct {
	Message string       `json:"message" example:"user created successfully"`
	User    UserResponse `json:"user"`
}

// CreateUserResponse represents the full API response envelope for user provisioning.
type CreateUserResponse struct {
	Code   int            `json:"code" example:"201"`
	Status string         `json:"status" example:"success"`
	Data   CreateUserData `json:"data"`
}

// ChangePasswordResponse represents the full API response envelope for password modification.
type ChangePasswordResponse struct {
	Code    int    `json:"code" example:"200"`
	Status  string `json:"status" example:"success"`
	Message string `json:"message" example:"password changed successfully"`
}

// UpdateUserResponse represents the full API response envelope when an admin updates a user.
type UpdateUserResponse struct {
	Code    int           `json:"code" example:"200"`
	Status  string        `json:"status" example:"success"`
	Message string        `json:"message" example:"user updated successfully"`
	Data    *UserResponse `json:"data,omitempty"`
}

// SubmitProfileChangeResponse represents the API response envelope for user profile update requests.
type SubmitProfileChangeResponse struct {
	Code    int                    `json:"code" example:"201"`
	Status  string                 `json:"status" example:"success"`
	Message string                 `json:"message" example:"profile change request submitted"`
	Data    *ProfileChangeResponse `json:"data,omitempty"`
}

// UserDetailResponse represents the API response envelope for user details.
type UserDetailResponse struct {
	Code   int          `json:"code" example:"200"`
	Status string       `json:"status" example:"success"`
	Data   UserResponse `json:"data"`
}

// UserListResponse represents the API response envelope for a list of users.
type UserListResponse struct {
	Code   int            `json:"code" example:"200"`
	Status string         `json:"status" example:"success"`
	Data   []UserResponse `json:"data"`
}
