package request

// RegisterRequest represents the payload for user self-registration.
type RegisterRequest struct {
	Username     string `json:"username" binding:"required,min=3,max=64" example:"john_doe"`
	Email        string `json:"email" binding:"required,email" example:"john.doe@example.com"`
	Password     string `json:"password" binding:"required,min=8,max=128" example:"SecurePass123!"`
	Name         string `json:"name" binding:"required" example:"John Doe"`
	BniID        string `json:"bni_id" example:"12345678"`
	EmployeeID   string `json:"employee_id" example:"EMP-001"`
	Division     string `json:"division" example:"Wholesale Digital Delivery"`
	DivisionID   *uint  `json:"division_id" example:"1"`
	Department   string `json:"department" example:"Wholesale Channel and Service Delivery"`
	DepartmentID *uint  `json:"department_id" example:"1"`
	Site         string `json:"site" example:"Citicon"`
	SiteID       *uint  `json:"site_id" example:"1"`
	Company      string `json:"company" example:"MII"`
	CompanyID    *uint  `json:"company_id" example:"1"`
	Position     string `json:"position" example:"Software Engineer"`
	GroupName    string `json:"group_name" example:"SDD"`
}

// ReviewRegistrationRequest represents the payload for administrator approval or rejection.
type ReviewRegistrationRequest struct {
	Action     string `json:"action" binding:"required,oneof=approve reject" example:"approve"`
	AdminNotes string `json:"admin_notes" example:"Approved after HR validation"`
}
