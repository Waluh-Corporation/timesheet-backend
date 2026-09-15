package response

import (
	"time"

	"timesheet-backend/models"
)

// ProfileChangeResponse represents a clean profile change request response without the user entity.
type ProfileChangeResponse struct {
	ID            uint                 `json:"id"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	UserID        uint                 `json:"user_id"`
	Status        models.ProfileStatus `json:"status"`
	Name          string               `json:"name"`
	BniID         string               `json:"bni_id"`
	EmployeeID    string               `json:"employee_id"`
	Division      string               `json:"division"`
	Department    string               `json:"department"`
	DepartmentID  *uint                `json:"department_id,omitempty"`
	DepartmentRel *models.Department   `json:"department_rel,omitempty"`
	GroupName     string               `json:"group_name,omitempty"`
	Position      string               `json:"position,omitempty"`
	Site          string               `json:"site,omitempty"`
	CompanyID     *uint                `json:"company_id,omitempty"`
	CompanyRel    *models.Company      `json:"company_rel,omitempty"`
	ReviewedBy    *uint                `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time           `json:"reviewed_at,omitempty"`
}
