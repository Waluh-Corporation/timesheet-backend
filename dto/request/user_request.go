package request

import "timesheet-backend/models"

// CreateUserRequest is the payload for administrator user provisioning.
type CreateUserRequest struct {
	Username     string      `json:"username" binding:"required,min=3,max=64" example:"john_doe"`
	Email        string      `json:"email" binding:"required,email" example:"john.doe@example.com"`
	Role         models.Role `json:"role" binding:"required,oneof=admin user" example:"user"`
	Name         string      `json:"name" example:"John Doe"`
	BniID        string      `json:"bni_id" example:"12345678"`
	Division     string      `json:"division" example:"Application Development Division"`
	Department   string      `json:"department" example:"Core Banking"`
	DepartmentID *uint       `json:"department_id" example:"1"`
	Site         string      `json:"site" example:"Jakarta"`
	Company      string      `json:"company" example:"MII"`
	CompanyID    *uint       `json:"company_id" example:"1"`
}

// ChangePasswordRequest represents the payload for changing the user's password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" example:"CurrentPass123!"`
	NewPassword string `json:"new_password" binding:"required,min=8" example:"NewStrongPass123!"`
}

// UpdateUserRequest represents the admin-driven user update payload.
type UpdateUserRequest struct {
	Role         *models.Role `json:"role" example:"user"`
	IsActive     *bool        `json:"is_active" example:"true"`
	Name         *string      `json:"name" example:"John Doe"`
	BniID        *string      `json:"bni_id" example:"12345678"`
	Division     *string      `json:"division" example:"Application Development Division"`
	Department   *string      `json:"department" example:"Core Banking"`
	DepartmentID *uint        `json:"department_id" example:"1"`
	Site         *string      `json:"site" example:"Jakarta"`
	Company      *string      `json:"company" example:"MII"`
	CompanyID    *uint        `json:"company_id" example:"1"`
}

// ProfileChangeRequestDTO represents a user's self-service profile change submission.
type ProfileChangeRequestDTO struct {
	Name         string `json:"name" example:"John Doe"`
	BniID        string `json:"bni_id" example:"12345678"`
	EmployeeID   string `json:"employee_id" example:"MII-12345"`
	Division     string `json:"division" example:"Application Development Division"`
	Department   string `json:"department" example:"Core Banking"`
	DepartmentID *uint  `json:"department_id" example:"1"`
	Site         string `json:"site" example:"Jakarta"`
	CompanyID    *uint  `json:"company_id" example:"1"`
}
