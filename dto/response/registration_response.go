package response

import (
	"time"

	"timesheet-backend/models"
)

// UserRegistrationResponse represents a sanitized registration record for API responses.
type UserRegistrationResponse struct {
	ID            uint                      `json:"id" example:"1"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
	UserID        uint                      `json:"user_id" example:"10"`
	Username      string                    `json:"username" example:"john_doe"`
	Email         string                    `json:"email" example:"john.doe@example.com"`
	Status        models.RegistrationStatus `json:"status" example:"pending"`
	Name          string                    `json:"name" example:"John Doe"`
	BniID         string                    `json:"bni_id" example:"12345678"`
	EmployeeID    string                    `json:"employee_id" example:"EMP-001"`
	Division      string                    `json:"division" example:"Wholesale Digital Delivery"`
	DivisionID    *uint                     `json:"division_id,omitempty" example:"1"`
	DivisionRel   *models.Division          `json:"division_rel,omitempty"`
	Department    string                    `json:"department" example:"Wholesale Channel and Service Delivery"`
	DepartmentID  *uint                     `json:"department_id,omitempty" example:"1"`
	DepartmentRel *models.Department        `json:"department_rel,omitempty"`
	GroupName     string                    `json:"group_name,omitempty" example:"SDD"`
	Position      string                    `json:"position,omitempty" example:"Software Engineer"`
	Site          string                    `json:"site,omitempty" example:"Citicon"`
	SiteID        *uint                     `json:"site_id,omitempty" example:"1"`
	SiteRel       *models.Site              `json:"site_rel,omitempty"`
	Company       string                    `json:"company,omitempty" example:"MII"`
	CompanyID     *uint                     `json:"company_id,omitempty" example:"1"`
	CompanyRel    *models.Company           `json:"company_rel,omitempty"`
	AdminNotes    string                    `json:"admin_notes,omitempty" example:"Verified"`
	ReviewedBy    *uint                     `json:"reviewed_by,omitempty" example:"1"`
	ReviewerName  string                    `json:"reviewer_name,omitempty" example:"Admin"`
	ReviewedAt    *time.Time                `json:"reviewed_at,omitempty"`
}

// ToUserRegistrationResponse transforms a models.UserRegistration into a UserRegistrationResponse DTO.
func ToUserRegistrationResponse(r *models.UserRegistration) UserRegistrationResponse {
	resp := UserRegistrationResponse{
		ID:            r.ID,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
		UserID:        r.UserID,
		Status:        r.Status,
		Name:          r.Name,
		BniID:         r.BniID,
		EmployeeID:    r.EmployeeID,
		Division:      r.Division,
		DivisionID:    r.DivisionID,
		DivisionRel:   r.DivisionRel,
		Department:    r.Department,
		DepartmentID:  r.DepartmentID,
		DepartmentRel: r.DepartmentRel,
		GroupName:     r.GroupName,
		Position:      r.Position,
		Site:          r.Site,
		SiteID:        r.SiteID,
		SiteRel:       r.SiteRel,
		Company:       r.Company,
		CompanyID:     r.CompanyID,
		CompanyRel:    r.CompanyRel,
		AdminNotes:    r.AdminNotes,
		ReviewedBy:    r.ReviewedBy,
		ReviewedAt:    r.ReviewedAt,
	}
	if r.User.ID != 0 {
		resp.Username = r.User.Username
		resp.Email = r.User.Email
	}
	if r.Reviewer != nil && r.Reviewer.ID != 0 {
		if r.Reviewer.Name != "" {
			resp.ReviewerName = r.Reviewer.Name
		} else {
			resp.ReviewerName = r.Reviewer.Username
		}
	}
	return resp
}

// RegisterResultData is the data payload returned upon successful self-registration submission.
type RegisterResultData struct {
	RegistrationID uint                      `json:"registration_id" example:"1"`
	UserID         uint                      `json:"user_id" example:"10"`
	Username       string                    `json:"username" example:"john_doe"`
	Email          string                    `json:"email" example:"john.doe@example.com"`
	Status         models.RegistrationStatus `json:"status" example:"pending"`
	Message        string                    `json:"message" example:"Registration submitted successfully. Your account is pending administrator approval."`
}

// RegisterResponse is the API response envelope for user self-registration.
type RegisterResponse struct {
	Code    int                `json:"code" example:"201"`
	Status  string             `json:"status" example:"success"`
	Message string             `json:"message" example:"Registration submitted successfully"`
	Data    RegisterResultData `json:"data"`
}

// RegistrationDetailResponse is the API response envelope for a single registration record.
type RegistrationDetailResponse struct {
	Code   int                      `json:"code" example:"200"`
	Status string                   `json:"status" example:"success"`
	Data   UserRegistrationResponse `json:"data"`
}

// RegistrationListResponse is the API response envelope for a list of registrations.
type RegistrationListResponse struct {
	Code   int                        `json:"code" example:"200"`
	Status string                     `json:"status" example:"success"`
	Data   []UserRegistrationResponse `json:"data"`
}
