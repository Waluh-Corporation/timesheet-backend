package domain

import (
	"context"
	"time"

	"timesheet-backend/models"
	"timesheet-backend/services"
)

// UserRepository defines the database persistence contract for User and Passkey entities.
type UserRepository interface {
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByIDWithDetails(ctx context.Context, id uint) (*models.User, error)
	FindByIDWithCredentials(ctx context.Context, id uint) (*models.User, error)
	FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error)
	FindByUsernameOrEmailWithCredentials(ctx context.Context, identifier string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByEmailExcludingUser(ctx context.Context, email string, excludeUserID uint) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error
	SoftDelete(ctx context.Context, id uint) error
	ListUsers(ctx context.Context, isActive *bool) ([]models.User, error)

	// Profile Change Requests
	CreateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error
	ListProfileChanges(ctx context.Context, userID *uint, status string) ([]models.ProfileChangeRequest, error)
	FindProfileChangeByID(ctx context.Context, id uint) (*models.ProfileChangeRequest, error)
	UpdateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error

	// WebAuthn Passkey operations
	CreatePasskeyCredential(ctx context.Context, cred *models.WebAuthnCredential) error
	UpdatePasskeySignCount(ctx context.Context, credID []byte, signCount uint32, backupState bool) error
	ListPasskeysByUserID(ctx context.Context, userID uint) ([]models.WebAuthnCredential, error)
	DeletePasskey(ctx context.Context, id uint, userID *uint) (bool, error)
	UpdatePasskeyName(ctx context.Context, id uint, userID *uint, name string) (bool, error)
}

// MasterRepository defines persistence operations for master data, org structure, and holidays.
type MasterRepository interface {
	// Approvers
	ListApprovers(ctx context.Context, roleType string, activeStatus *bool) ([]models.Approver, error)
	FindApproverByID(ctx context.Context, id uint) (*models.Approver, error)
	CreateApprover(ctx context.Context, a *models.Approver) error
	UpdateApprover(ctx context.Context, a *models.Approver) error
	SoftDeleteApprover(ctx context.Context, id uint) error

	// Companies
	ListCompanies(ctx context.Context, activeStatus *bool) ([]models.Company, error)
	FindCompanyByID(ctx context.Context, id uint) (*models.Company, error)
	FindCompanyByCode(ctx context.Context, code string) (*models.Company, error)
	FindActiveCompanyByCodeOrName(ctx context.Context, identifier string) (*models.Company, error)
	CreateCompany(ctx context.Context, c *models.Company) error
	UpdateCompany(ctx context.Context, c *models.Company) error
	SoftDeleteCompany(ctx context.Context, id uint) error

	// Sites
	ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error)
	FindSiteByID(ctx context.Context, id uint) (*models.Site, error)
	FindSiteByCode(ctx context.Context, code string) (*models.Site, error)
	FindActiveSiteByCodeOrName(ctx context.Context, identifier string) (*models.Site, error)
	CreateSite(ctx context.Context, s *models.Site) error
	UpdateSite(ctx context.Context, s *models.Site) error
	SoftDeleteSite(ctx context.Context, id uint) error

	// Divisions
	ListDivisions(ctx context.Context, activeStatus *bool) ([]models.Division, error)
	FindDivisionByID(ctx context.Context, id uint) (*models.Division, error)
	FindDivisionByCode(ctx context.Context, code string) (*models.Division, error)
	FindActiveDivisionByCodeOrName(ctx context.Context, identifier string) (*models.Division, error)
	CreateDivision(ctx context.Context, d *models.Division) error
	UpdateDivision(ctx context.Context, d *models.Division) error
	SoftDeleteDivision(ctx context.Context, id uint) error

	// Departments
	ListDepartments(ctx context.Context, divisionID *uint, divisionName string, activeStatus *bool) ([]models.Department, error)
	FindDepartmentByID(ctx context.Context, id uint) (*models.Department, error)
	FindDepartmentByName(ctx context.Context, name string) (*models.Department, error)
	FindActiveDepartmentByCodeOrName(ctx context.Context, identifier string) (*models.Department, error)
	CreateDepartment(ctx context.Context, d *models.Department) error
	UpdateDepartment(ctx context.Context, d *models.Department) error
	SoftDeleteDepartment(ctx context.Context, id uint) error

	// Projects & ActivityStatuses
	ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error)
	FindProjectByID(ctx context.Context, id uint) (*models.Project, error)
	ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error)

	// Holidays
	ListHolidaysByMonth(ctx context.Context, year, month int) ([]models.Holiday, error)
	ListAllHolidays(ctx context.Context, year *int) ([]models.Holiday, error)
	UpsertHolidays(ctx context.Context, holidays []models.Holiday) error
}

// PushRepository defines operations for Web Push subscription persistence.
type PushRepository interface {
	Subscribe(ctx context.Context, sub *models.PushSubscription) error
	Unsubscribe(ctx context.Context, userID uint, endpoint string) error
	ListByUserID(ctx context.Context, userID uint) ([]models.PushSubscription, error)
	DeleteByID(ctx context.Context, id uint) error
}

// AuthenticatorRepository defines operations for AAGUID registry persistence.
type AuthenticatorRepository interface {
	SyncAAGUIDs(ctx context.Context, entries map[string]services.CommunityAAGUIDEntry) (int, error)
	ListAuthenticators(ctx context.Context, search string, offset, limit int) ([]models.AuthenticatorAAGUID, int64, error)
}

// SetupRepository defines operations for initial onboarding system setup persistence.
type SetupRepository interface {
	GetAdminCount(ctx context.Context) (int64, error)
	GetSystemSetting(ctx context.Context, key string) (string, error)
	ExecuteSetup(ctx context.Context, adminUser *models.User, companies []models.Company, depts []models.Department, approvers []models.Approver) error
}
