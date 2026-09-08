package models

import (
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
	// PasswordHash is an Argon2id PHC-encoded hash (legacy bcrypt hashes are
	// still verified and upgraded on next login). It may be empty for
	// passwordless (passkey-only) accounts that have not yet set a password.
	PasswordHash string `gorm:"size:255" json:"-"`
	Role         Role   `gorm:"size:16;not null;default:user" json:"role"`
	IsActive     bool   `gorm:"not null;default:true" json:"is_active"`

	// Profile fields (the "approved" / live values).
	Name       string   `gorm:"size:255" json:"name"`
	MiiID      string   `gorm:"size:64" json:"mii_id"`
	EmployeeID string   `gorm:"size:64" json:"employee_id"` // NPP or MII ID
	Division   string   `gorm:"size:255" json:"division"`
	Department string   `gorm:"size:255" json:"department"`
	GroupName  string   `gorm:"size:255" json:"group_name"` // Kelompok (SDD)
	Position   string   `gorm:"size:128" json:"position"`
	Site       string   `gorm:"size:128" json:"site"`
	Company    string   `gorm:"size:64" json:"company"`
	CompanyID  *uint    `gorm:"index" json:"company_id"`
	CompanyRel *Company `gorm:"foreignKey:CompanyID" json:"company_rel,omitempty"`

	// Credential relations.
	Credentials       []WebAuthnCredential   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	PushSubscriptions []PushSubscription     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ProfileRequests   []ProfileChangeRequest `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Overtimes         []OvertimeEntry        `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

// Company represents a vendor/organization (e.g. MII, SDD, Adidata).
type Company struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Code      string     `gorm:"uniqueIndex;size:32;not null" json:"code"` // "mii", "sdd", "adidata"
	Name      string     `gorm:"size:255;not null" json:"name"`
	Templates []Template `gorm:"foreignKey:CompanyID" json:"templates,omitempty"`
}

// OvertimeEntry records overtime activities for SPL sheet generation.
type OvertimeEntry struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	UserID          uint           `gorm:"index;not null" json:"user_id"`
	DailyActivityID *uint          `gorm:"index" json:"daily_activity_id"`
	Date            time.Time      `gorm:"type:date;not null" json:"date"`
	StartTime       string         `gorm:"size:8" json:"start_time"` // "17:00"
	EndTime         string         `gorm:"size:8" json:"end_time"`   // "21:00"
	TaskDescription string         `gorm:"type:text;not null" json:"task_description"`
	TeamLeader      string         `gorm:"size:128" json:"team_leader"`
	DepartmentHead  string         `gorm:"size:128" json:"department_head"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}


// WebAuthnID implements webauthn.User.
func (u User) WebAuthnID() []byte {
	// Encode the primary key as a stable little-endian byte slice.
	b := make([]byte, 8)
	id := u.ID
	for i := 0; i < 8; i++ {
		b[i] = byte(id >> (8 * i))
	}
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
	UserID    uint      `gorm:"index;not null" json:"user_id"`

	CredentialID    []byte `gorm:"uniqueIndex;not null" json:"-"`
	PublicKey       []byte `gorm:"not null" json:"-"`
	AttestationType string `gorm:"size:64" json:"-"`
	AAGUID          []byte `json:"-"`
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

// Template is an admin-uploaded .xlsx timesheet template. Multiple client
// templates are supported; exactly one may be flagged as the default.
type Template struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"size:512" json:"description"`
	Company     string `gorm:"size:64" json:"company"`
	CompanyID   *uint  `gorm:"index" json:"company_id"`
	CompanyRel  *Company `gorm:"foreignKey:CompanyID" json:"company_rel,omitempty"`
	// SheetName is the worksheet the mapping applies to.
	SheetName string `gorm:"size:128;not null;default:Sheet1" json:"sheet_name"`
	// FileData holds the raw .xlsx bytes so generation is self-contained.
	FileData  []byte `gorm:"type:bytea" json:"-"`
	IsDefault bool   `gorm:"not null;default:false" json:"is_default"`
	CreatedBy uint   `json:"created_by"`
	// Builtin, when set (e.g. "bni_dev"), marks a first-class bundled template
	// whose fixed layout is rendered by a dedicated strict-typed generator rather
	// than the generic cell-mapping engine.
	Builtin string `gorm:"size:32" json:"builtin"`

	CellMappings []CellMapping `gorm:"constraint:OnDelete:CASCADE" json:"cell_mappings"`
}

// MappingFieldType enumerates the semantic purpose an admin can assign to a
// cell or column region when building a template mapping.
type MappingFieldType string

const (
	FieldDate        MappingFieldType = "date"
	FieldTimeIn      MappingFieldType = "time_in"
	FieldTimeOut     MappingFieldType = "time_out"
	FieldStatus      MappingFieldType = "status"
	FieldActivity    MappingFieldType = "activity"
	FieldProjectName MappingFieldType = "project_name"
	FieldProjectID   MappingFieldType = "project_id"
	FieldAppImpacted MappingFieldType = "app_impacted"
	// Additional per-day columns used by the MII layout.
	FieldTotalHour  MappingFieldType = "total_hour" // End - Start (computed)
	FieldDivision   MappingFieldType = "division"   // per-day divisi column
	FieldDepartment MappingFieldType = "department" // per-day departement column
	FieldSubDept    MappingFieldType = "sub_department"
	FieldAIPFitur   MappingFieldType = "aip_fitur"
	// Static header/metadata single cells.
	FieldMetaName     MappingFieldType = "meta_name"
	FieldMetaMiiID    MappingFieldType = "meta_mii_id"
	FieldMetaDivision MappingFieldType = "meta_division"
	FieldMetaSite     MappingFieldType = "meta_site"
	FieldMetaMonth    MappingFieldType = "meta_month"
	FieldMetaYear     MappingFieldType = "meta_year"
)

// MappingScope describes whether a mapping addresses a single fixed cell or a
// repeating column whose row grows one-per-day of the month.
type MappingScope string

const (
	// ScopeCell addresses a single absolute cell (e.g. header metadata).
	ScopeCell MappingScope = "cell"
	// ScopeDailyColumn addresses a column whose rows repeat per calendar day,
	// anchored at StartRow (day 1) and incrementing downward.
	ScopeDailyColumn MappingScope = "daily_column"
)

// CellMapping links a semantic field to a physical location in the template.
type CellMapping struct {
	ID         uint             `gorm:"primaryKey" json:"id"`
	TemplateID uint             `gorm:"index;not null" json:"template_id"`
	Field      MappingFieldType `gorm:"size:32;not null" json:"field"`
	Scope      MappingScope     `gorm:"size:16;not null;default:cell" json:"scope"`

	// For ScopeCell: absolute address, e.g. "C4".
	CellRef string `gorm:"size:16" json:"cell_ref"`
	// For ScopeDailyColumn: the column letter (e.g. "K") and the row where the
	// first day of the month is written.
	Column   string `gorm:"size:4" json:"column"`
	StartRow int    `json:"start_row"`
	// Fillable marks whether users may edit this field in the monthly grid.
	Fillable bool `gorm:"not null;default:true" json:"fillable"`
}

// DailyActivity stores one user's timesheet entry for a single calendar day.
type DailyActivity struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID uint `gorm:"uniqueIndex:idx_user_date;not null" json:"user_id"`
	// Date is normalised to midnight in Asia/Jakarta.
	Date time.Time `gorm:"uniqueIndex:idx_user_date;not null;type:date" json:"date"`

	StartTime   string `gorm:"size:8" json:"start_time"`
	EndTime     string `gorm:"size:8" json:"end_time"`
	Status      string `gorm:"size:8" json:"status"`
	Activity    string `gorm:"type:text" json:"activity"`
	ProjectName string `gorm:"size:255" json:"project_name"`
	ProjectID   string `gorm:"size:64" json:"project_id"`
	AppImpacted string `gorm:"size:255" json:"app_impacted"`
}

// PushSubscription persists a browser Web Push subscription for a user.
type PushSubscription struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`

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
	UserID    uint          `gorm:"index;not null" json:"user_id"`
	Status    ProfileStatus `gorm:"size:16;not null;default:pending" json:"status"`

	Name       string `gorm:"size:255" json:"name"`
	MiiID      string `gorm:"size:64" json:"mii_id"`
	EmployeeID string `gorm:"size:64" json:"employee_id"`
	Division   string `gorm:"size:255" json:"division"`
	Department string `gorm:"size:255" json:"department"`
	GroupName  string `gorm:"size:255" json:"group_name"`
	Position   string `gorm:"size:128" json:"position"`
	Site       string `gorm:"size:128" json:"site"`

	ReviewedBy *uint      `json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// PasswordResetToken backs the forgot/reset-password email flow.
type PasswordResetToken struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	CreatedAt time.Time `json:"-"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	TokenHash string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	ExpiresAt time.Time `gorm:"not null" json:"-"`
	Used      bool      `gorm:"not null;default:false" json:"-"`
}
