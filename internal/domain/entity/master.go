package entity

import "time"

// ApproverRoleType defines the role types for approvers.
type ApproverRoleType string

const (
	RoleTeamLeader     ApproverRoleType = "team_leader"
	RoleDepartmentHead ApproverRoleType = "department_head"
)

// Approver represents an organizational supervisor authorized to approve timesheets.
type Approver struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	RoleType  ApproverRoleType
	Title     string
	IsActive  bool
}

// Company represents a partner, client, or vendor organization.
type Company struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Code      string
	Name      string
	IsActive  bool
}

// Site represents a company branch, office, or placement location.
type Site struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Code      string
	Name      string
	IsActive  bool
}

// Division represents an organizational business or technology division.
type Division struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Code      string
	Name      string
	IsActive  bool
}

// Department represents an operational unit under a division.
type Department struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Code       string
	Name       string
	Division   string
	DivisionID *uint
	IsActive   bool
}

// Project represents a billable or trackable project entity.
type Project struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Code        string
	Name        string
	AppImpacted string
	IsActive    bool
}

// ActivityStatus represents an attendance or work category status (e.g., Present, WFH, Leave).
type ActivityStatus struct {
	Code         string
	Name         string
	Description  string
	IsWorkingDay bool
	SortOrder    int
}

// Holiday represents a public holiday or collective leave day.
type Holiday struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Date         time.Time
	Description  string
	IsJointLeave bool
	IsCivic      bool
	IsReligious  bool
}
