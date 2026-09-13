package models

import (
	"encoding/binary"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Role enumerates the RBAC roles supported by the portal.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// ProfileStatus tracks the approval state of a user's profile changes.
type ProfileStatus string

const (
	ProfileApproved ProfileStatus = "approved"
	ProfilePending  ProfileStatus = "pending"
)

// User is the core account entity. Registration is strictly admin-driven;
// there is no public sign-up path anywhere in the API.
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Username string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Email    string `gorm:"uniqueIndex;size:255;not null" json:"email"`
	// PasswordHash is an Argon2id PHC-encoded hash adhering to OWASP recommendations.
	// It may be empty for passwordless (passkey-only) accounts that have not yet set a password.
	PasswordHash string `gorm:"size:255" json:"-"`
	Role         Role   `gorm:"size:16;not null;default:user;index:idx_users_role_active,priority:1;check:role IN ('admin', 'user')" json:"role"`
	IsActive     bool   `gorm:"column:is_active;type:boolean;default:true;not null;index:idx_users_role_active,priority:2" json:"is_active"`

	// Profile fields (the "approved" / live values).
	Name          string      `gorm:"size:255" json:"name"`
	BniID         string      `gorm:"size:64;comment:NPP BNI" json:"bni_id"` // NPP BNI
	EmployeeID    string      `gorm:"size:64" json:"employee_id"`            // NPP or Vendor ID
	Division      string      `gorm:"size:255" json:"division"`
	Department    string      `gorm:"size:255" json:"department"`
	DepartmentID  *uint       `gorm:"index" json:"department_id"`
	DepartmentRel *Department `gorm:"foreignKey:DepartmentID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"department_rel,omitempty"`
	GroupName     string      `gorm:"size:255" json:"group_name"` // Kelompok (SDD)
	Position      string      `gorm:"size:128" json:"position"`
	Site          string      `gorm:"size:128" json:"site"`
	Company       string      `gorm:"size:64" json:"company"`
	CompanyID     *uint       `gorm:"index" json:"company_id"`
	CompanyRel    *Company    `gorm:"foreignKey:CompanyID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"company_rel,omitempty"`

	// Credential relations.
	Credentials       []WebAuthnCredential   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	PushSubscriptions []PushSubscription     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ProfileRequests   []ProfileChangeRequest `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Overtimes         []OvertimeEntry        `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	DailyActivities   []DailyActivity        `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

// Company represents a vendor/organization (e.g. MII, SDD, Adidata).
type Company struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Code      string    `gorm:"unique;size:32;not null" json:"code"` // "mii", "sdd", "adidata", "ntt"
	Name      string    `gorm:"size:255;not null" json:"name"`
	IsActive  bool      `gorm:"column:is_active;type:boolean;default:true;not null;index:idx_companies_active" json:"is_active"`
}

// Department represents an organizational department or unit within the company portal.
type Department struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Code      string    `gorm:"size:64;index" json:"code"`                                     // e.g. "WCSD", "DEV-01"
	Name      string    `gorm:"size:255;not null;uniqueIndex:uq_departments_name" json:"name"` // e.g. "Wholesale Channel and Service Delivery"
	Division  string    `gorm:"size:255" json:"division"`                                      // e.g. "Wholesale Digital Delivery"
	IsActive  bool      `gorm:"column:is_active;type:boolean;default:true;not null;index:idx_departments_active" json:"is_active"`
}

// ActivityStatus represents a normalized status option for daily timesheet activity.
type ActivityStatus struct {
	Code         string `gorm:"primaryKey;size:8" json:"code"` // "P", "S", "PM", "V", "BT", "X"
	Name         string `gorm:"size:64;not null" json:"name"`  // "Present", "Sick", etc.
	Description  string `gorm:"size:255" json:"description"`
	IsWorkingDay bool   `gorm:"not null;default:true" json:"is_working_day"`
	SortOrder    int    `gorm:"not null;default:0" json:"sort_order"`
}

// Holiday represents a national or regional holiday or joint leave.
type Holiday struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Date         time.Time `gorm:"type:date;uniqueIndex;not null" json:"date"`
	Description  string    `gorm:"size:255;not null" json:"description"`
	IsJointLeave bool      `gorm:"not null;default:false;index:idx_holidays_joint_leave" json:"is_joint_leave"`
	IsCivic      bool      `gorm:"not null;default:false" json:"is_civic"`
	IsReligious  bool      `gorm:"not null;default:false" json:"is_religious"`
}

// Project represents a billable project or initiative (e.g. BNI Direct, Core Banking).
type Project struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Code        string    `gorm:"size:64;not null;index;uniqueIndex:uq_projects_code_name,priority:1" json:"code"` // e.g. "P24015"
	Name        string    `gorm:"size:255;not null;uniqueIndex:uq_projects_code_name,priority:2" json:"name"`      // e.g. "BNI Direct"
	AppImpacted string    `gorm:"size:255" json:"app_impacted"`                                                    // e.g. "BNI Direct Cash"
	IsActive    bool      `gorm:"column:is_active;type:boolean;default:true;not null;index:idx_projects_active" json:"is_active"`
}

// ApproverRoleType enumerates the functional role of an approver.
type ApproverRoleType string

const (
	ApproverRoleTeamLeader     ApproverRoleType = "team_leader"
	ApproverRoleDepartmentHead ApproverRoleType = "department_head"
)

// Approver represents an authorized manager/supervisor who approves timesheet and overtime reports.
type Approver struct {
	ID        uint             `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Name      string           `gorm:"size:255;not null" json:"name"`
	RoleType  ApproverRoleType `gorm:"size:32;not null;index;check:role_type IN ('team_leader', 'department_head')" json:"role_type"`
	Title     string           `gorm:"size:128" json:"title"`
	IsActive  bool             `gorm:"column:is_active;type:boolean;default:true;not null;index:idx_approvers_active" json:"is_active"`
}

// OvertimeEntry records overtime activities for SPL sheet generation.
type OvertimeEntry struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	UserID           uint      `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user_id"`
	DailyActivityID  *uint     `gorm:"index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"daily_activity_id"`
	Date             time.Time `gorm:"type:date;not null;index:idx_overtime_entries_date" json:"date"`
	StartTime        string    `gorm:"size:8" json:"start_time"` // "17:00"
	EndTime          string    `gorm:"size:8" json:"end_time"`   // "21:00"
	TaskDescription  string    `gorm:"type:text;not null" json:"task_description"`
	TeamLeaderID     *uint     `gorm:"index" json:"team_leader_id"`
	DepartmentHeadID *uint     `gorm:"index" json:"department_head_id"`
	IsActive         bool      `gorm:"column:is_active;type:boolean;default:true;not null;index:idx_overtime_entries_active" json:"is_active"`

	User           User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	DailyActivity  *DailyActivity `gorm:"foreignKey:DailyActivityID" json:"daily_activity,omitempty"`
	TeamLeader     *Approver      `gorm:"foreignKey:TeamLeaderID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"team_leader,omitempty"`
	DepartmentHead *Approver      `gorm:"foreignKey:DepartmentHeadID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"department_head,omitempty"`
}

// WebAuthnID implements webauthn.User.
func (u User) WebAuthnID() []byte {
	// Encode the primary key as a stable little-endian byte slice.
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(u.ID))
	return b
}

// WebAuthnName implements webauthn.User.
func (u User) WebAuthnName() string { return u.Username }

// WebAuthnDisplayName implements webauthn.User.
func (u User) WebAuthnDisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}

// WebAuthnIcon implements webauthn.User (deprecated but part of the interface).
func (u User) WebAuthnIcon() string { return "" }

// WebAuthnCredentials implements webauthn.User by decoding stored credentials.
func (u User) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, 0, len(u.Credentials))
	for _, c := range u.Credentials {
		if cred, err := c.ToLibrary(); err == nil {
			creds = append(creds, cred)
		}
	}
	return creds
}

// WebAuthnCredential persists a single passkey credential for a user.
type WebAuthnCredential struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user_id"`

	CredentialID    []byte `gorm:"uniqueIndex;not null" json:"-"`
	PublicKey       []byte `gorm:"not null" json:"-"`
	AttestationType string `gorm:"size:64" json:"-"`
	AAGUID          []byte `gorm:"column:aaguid" json:"-"`
	SignCount       uint32 `json:"-"`
	CloneWarning    bool   `json:"-"`
	// BackupEligible (BE) records whether the authenticator can back up / sync
	// the credential. Per the WebAuthn spec this is immutable for the credential
	// and MUST be persisted so it can be validated for consistency at login.
	// BackupState (BS) records whether it currently is backed up; it may change.
	BackupEligible bool `json:"-"`
	BackupState    bool `json:"-"`
	// Transports is stored as a JSON array of transport strings.
	Transports   datatypes.JSON `json:"-"`
	FriendlyName string         `gorm:"size:128" json:"friendly_name"`
}

// DailyActivity stores one user's timesheet entry for a single calendar day.
type DailyActivity struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID uint `gorm:"not null;index:idx_daily_activities_user_date" json:"user_id"`
	// Date is normalised to midnight in Asia/Jakarta.
	Date time.Time `gorm:"index:idx_daily_activities_user_date;index:idx_daily_activities_date;not null;type:date" json:"date"`

	StartTime   string `gorm:"size:8" json:"start_time"`
	EndTime     string `gorm:"size:8" json:"end_time"`
	Status      string `gorm:"size:8" json:"status"`
	Activity    string `gorm:"type:text" json:"activity"`
	ProjectName string `gorm:"size:255" json:"project_name"`
	ProjectID   string `gorm:"size:64" json:"project_id"`
	IsActive    bool   `gorm:"column:is_active;type:boolean;default:true;not null;index:idx_daily_activities_active" json:"is_active"`

	ProjectRefID *uint           `gorm:"index" json:"project_ref_id"`
	ProjectRef   *Project        `gorm:"foreignKey:ProjectRefID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"project_ref,omitempty"`
	StatusRef    *ActivityStatus `gorm:"foreignKey:Status;references:Code;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"status_ref,omitempty"`
	User         *User           `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
}

// GetProjectCode returns the canonical project code from the referenced Project,
// falling back to the denormalized ProjectID if ProjectRef is not preloaded.
func (d DailyActivity) GetProjectCode() string {
	if d.ProjectRef != nil && d.ProjectRef.Code != "" {
		return d.ProjectRef.Code
	}
	return d.ProjectID
}

// GetProjectName returns the canonical project name from the referenced Project,
// falling back to the denormalized ProjectName if ProjectRef is not preloaded.
func (d DailyActivity) GetProjectName() string {
	if d.ProjectRef != nil && d.ProjectRef.Name != "" {
		return d.ProjectRef.Name
	}
	return d.ProjectName
}

// GetAppImpacted returns the canonical app impacted from the referenced Project,
// or empty string if ProjectRef is not preloaded or empty.
func (d DailyActivity) GetAppImpacted() string {
	if d.ProjectRef != nil {
		return d.ProjectRef.AppImpacted
	}
	return ""
}

// PushSubscription persists a browser Web Push subscription for a user.
type PushSubscription struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user_id"`

	Endpoint string `gorm:"uniqueIndex;size:512;not null" json:"endpoint"`
	P256dh   string `gorm:"size:255;not null" json:"p256dh"`
	Auth     string `gorm:"size:255;not null" json:"auth"`
}

// ProfileChangeRequest captures a pending edit to a user's profile that must be
// approved by an admin before it is applied to the live User record.
type ProfileChangeRequest struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	UserID    uint          `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user_id"`
	Status    ProfileStatus `gorm:"size:16;not null;default:pending;check:status IN ('pending', 'approved', 'rejected')" json:"status"`

	Name          string      `gorm:"size:255" json:"name"`
	BniID         string      `gorm:"size:64;comment:NPP BNI" json:"bni_id"` // NPP BNI
	EmployeeID    string      `gorm:"size:64" json:"employee_id"`
	Division      string      `gorm:"size:255" json:"division"`
	Department    string      `gorm:"size:255" json:"department"`
	DepartmentID  *uint       `gorm:"index" json:"department_id"`
	DepartmentRel *Department `gorm:"foreignKey:DepartmentID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"department_rel,omitempty"`
	GroupName     string      `gorm:"size:255" json:"group_name"`
	Position      string      `gorm:"size:128" json:"position"`
	Site          string      `gorm:"size:128" json:"site"`
	CompanyID     *uint       `gorm:"index" json:"company_id"`
	CompanyRel    *Company    `gorm:"foreignKey:CompanyID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"company_rel,omitempty"`

	ReviewedBy *uint      `gorm:"index:idx_profile_change_requests_reviewed_by" json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	Reviewer   *User      `gorm:"foreignKey:ReviewedBy;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"reviewer,omitempty"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

// PasswordResetToken backs the forgot/reset-password and account setup email flows.
type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey" json:"-"`
	CreatedAt time.Time  `json:"-"`
	UserID    uint       `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	TokenType string     `gorm:"size:32;not null;default:'password_reset'" json:"-"`
	TokenHash string     `gorm:"uniqueIndex;size:64;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"-"`
	UsedAt    *time.Time `json:"-"`
	CreatedIP string     `gorm:"size:45" json:"-"`
	UsedIP    string     `gorm:"size:45" json:"-"`
}

// SystemSetting stores system-wide key-value configuration flags (e.g. is_new = Y/N).
type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
