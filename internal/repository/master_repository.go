package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// MasterRepository defines database persistence operations for master data entities.
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
	CreateCompany(ctx context.Context, c *models.Company) error
	UpdateCompany(ctx context.Context, c *models.Company) error
	SoftDeleteCompany(ctx context.Context, id uint) error

	// Sites
	ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error)
	FindSiteByID(ctx context.Context, id uint) (*models.Site, error)
	FindSiteByCode(ctx context.Context, code string) (*models.Site, error)
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
	CreateDepartment(ctx context.Context, d *models.Department) error
	UpdateDepartment(ctx context.Context, d *models.Department) error
	SoftDeleteDepartment(ctx context.Context, id uint) error

	// Projects & ActivityStatuses
	ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error)
	ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error)
}

type masterRepository struct {
	db *gorm.DB
}

// NewMasterRepository constructs a MasterRepository implementation.
func NewMasterRepository(db *gorm.DB) MasterRepository {
	return &masterRepository{db: db}
}

// --- Approvers ---
func (r *masterRepository) ListApprovers(ctx context.Context, roleType string, activeStatus *bool) ([]models.Approver, error) {
	var items []models.Approver
	q := r.db.WithContext(ctx)
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	if roleType != "" {
		q = q.Where("role_type = ?", roleType)
	}
	err := q.Order("name asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindApproverByID(ctx context.Context, id uint) (*models.Approver, error) {
	var a models.Approver
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *masterRepository) CreateApprover(ctx context.Context, a *models.Approver) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *masterRepository) UpdateApprover(ctx context.Context, a *models.Approver) error {
	return r.db.WithContext(ctx).Save(a).Error
}

func (r *masterRepository) SoftDeleteApprover(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Approver{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Companies ---
func (r *masterRepository) ListCompanies(ctx context.Context, activeStatus *bool) ([]models.Company, error) {
	var items []models.Company
	q := r.db.WithContext(ctx)
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	err := q.Order("id asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindCompanyByID(ctx context.Context, id uint) (*models.Company, error) {
	var c models.Company
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *masterRepository) FindCompanyByCode(ctx context.Context, code string) (*models.Company, error) {
	var c models.Company
	err := r.db.WithContext(ctx).Where("LOWER(code) = LOWER(?)", code).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *masterRepository) CreateCompany(ctx context.Context, c *models.Company) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *masterRepository) UpdateCompany(ctx context.Context, c *models.Company) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *masterRepository) SoftDeleteCompany(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Company{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Sites ---
func (r *masterRepository) ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error) {
	var items []models.Site
	q := r.db.WithContext(ctx)
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	err := q.Order("id asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindSiteByID(ctx context.Context, id uint) (*models.Site, error) {
	var s models.Site
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *masterRepository) FindSiteByCode(ctx context.Context, code string) (*models.Site, error) {
	var s models.Site
	err := r.db.WithContext(ctx).Where("LOWER(code) = LOWER(?)", code).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *masterRepository) CreateSite(ctx context.Context, s *models.Site) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *masterRepository) UpdateSite(ctx context.Context, s *models.Site) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *masterRepository) SoftDeleteSite(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Site{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Divisions ---
func (r *masterRepository) ListDivisions(ctx context.Context, activeStatus *bool) ([]models.Division, error) {
	var items []models.Division
	q := r.db.WithContext(ctx)
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	err := q.Order("id asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindDivisionByID(ctx context.Context, id uint) (*models.Division, error) {
	var d models.Division
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) FindDivisionByCode(ctx context.Context, code string) (*models.Division, error) {
	var d models.Division
	err := r.db.WithContext(ctx).Where("LOWER(code) = LOWER(?)", code).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) FindActiveDivisionByCodeOrName(ctx context.Context, identifier string) (*models.Division, error) {
	var d models.Division
	err := r.db.WithContext(ctx).Where("(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true", identifier, "%"+identifier+"%").First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) CreateDivision(ctx context.Context, d *models.Division) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *masterRepository) UpdateDivision(ctx context.Context, d *models.Division) error {
	return r.db.WithContext(ctx).Save(d).Error
}

func (r *masterRepository) SoftDeleteDivision(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Division{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Departments ---
func (r *masterRepository) ListDepartments(ctx context.Context, divisionID *uint, divisionName string, activeStatus *bool) ([]models.Department, error) {
	var items []models.Department
	q := r.db.WithContext(ctx).Preload("DivisionRel")
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	if divisionID != nil && *divisionID != 0 {
		q = q.Where("division_id = ?", *divisionID)
	}
	if divisionName != "" {
		q = q.Where("LOWER(division) = LOWER(?)", divisionName)
	}
	err := q.Order("name asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindDepartmentByID(ctx context.Context, id uint) (*models.Department, error) {
	var d models.Department
	err := r.db.WithContext(ctx).Preload("DivisionRel").Where("id = ?", id).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) FindDepartmentByName(ctx context.Context, name string) (*models.Department, error) {
	var d models.Department
	err := r.db.WithContext(ctx).Where("LOWER(name) = LOWER(?)", name).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) CreateDepartment(ctx context.Context, d *models.Department) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *masterRepository) UpdateDepartment(ctx context.Context, d *models.Department) error {
	return r.db.WithContext(ctx).Save(d).Error
}

func (r *masterRepository) SoftDeleteDepartment(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Department{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Projects & ActivityStatuses ---
func (r *masterRepository) ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error) {
	var items []models.Project
	q := r.db.WithContext(ctx)
	if activeOnly {
		q = q.Scopes(models.ActiveOnly)
	}
	err := q.Order("name asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error) {
	var items []models.ActivityStatus
	err := r.db.WithContext(ctx).Order("sort_order asc").Find(&items).Error
	return items, err
}
