package request

import "timesheet-backend/models"

// InitSetupAdminRequest carries administrator account details for setup.
type InitSetupAdminRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64" example:"admin"`
	Email    string `json:"email" binding:"required,email" example:"admin@example.com"`
	Name     string `json:"name" binding:"required" example:"Super Administrator"`
	Password string `json:"password" binding:"required,min=8" example:"SuperSecretPass2026!"`
}

// InitSetupCompanyRequest carries initial company details.
type InitSetupCompanyRequest struct {
	Code string `json:"code" binding:"required" example:"mii"`
	Name string `json:"name" binding:"required" example:"PT Mitra Integrasi Informatika"`
}

// InitSetupApproverRequest carries initial approver supervisor details.
type InitSetupApproverRequest struct {
	Name        string                  `json:"name" binding:"required" example:"Supervisor Name"`
	RoleType    models.ApproverRoleType `json:"role_type" binding:"required,oneof=team_leader department_head" example:"team_leader"`
	Title       string                  `json:"title" example:"Team Leader"`
	CompanyCode string                  `json:"company_code" example:"mii"`
}

// InitSetupDepartmentRequest carries optional initial department details.
type InitSetupDepartmentRequest struct {
	Code        string `json:"code" binding:"required" example:"WCSD"`
	Name        string `json:"name" binding:"required" example:"Wholesale Channel and Service Delivery"`
	Division    string `json:"division" example:"Wholesale Digital Delivery"`
	CompanyCode string `json:"company_code" example:"mii"`
}

// InitSetupRequest is the full payload for the one-time system initialization wizard.
type InitSetupRequest struct {
	Admin       InitSetupAdminRequest        `json:"admin" binding:"required"`
	Companies   []InitSetupCompanyRequest    `json:"companies"`
	Approvers   []InitSetupApproverRequest   `json:"approvers"`
	Departments []InitSetupDepartmentRequest `json:"departments"`
}
