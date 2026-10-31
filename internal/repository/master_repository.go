package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// ApproverRepository is re-exported from domain.ApproverRepository.
type ApproverRepository = domain.ApproverRepository

// CompanyRepository is re-exported from domain.CompanyRepository.
type CompanyRepository = domain.CompanyRepository

// SiteRepository is re-exported from domain.SiteRepository.
type SiteRepository = domain.SiteRepository

// PlacementRepository is re-exported from domain.PlacementRepository.
type PlacementRepository = domain.PlacementRepository

// DivisionRepository is re-exported from domain.DivisionRepository.
type DivisionRepository = domain.DivisionRepository

// DepartmentRepository is re-exported from domain.DepartmentRepository.
type DepartmentRepository = domain.DepartmentRepository

// ProjectRepository is re-exported from domain.ProjectRepository.
type ProjectRepository = domain.ProjectRepository

// HolidayRepository is re-exported from domain.HolidayRepository.
type HolidayRepository = domain.HolidayRepository

// MasterRepository is re-exported from domain.MasterRepository.
type MasterRepository = domain.MasterRepository

const (
	queryIDEquals         = "id = ?"
	queryLowerCodeEquals  = "LOWER(code) = LOWER(?)"
	queryActiveCodeOrName = "(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true"
)

type masterRepository struct {
	db *gorm.DB
}

// NewMasterRepository constructs a MasterRepository implementation.
func NewMasterRepository(db *gorm.DB) MasterRepository {
	return &masterRepository{db: db}
}

// NewApproverRepository constructs an ApproverRepository implementation.
func NewApproverRepository(db *gorm.DB) ApproverRepository { return NewMasterRepository(db) }

// NewCompanyRepository constructs a CompanyRepository implementation.
func NewCompanyRepository(db *gorm.DB) CompanyRepository { return NewMasterRepository(db) }

// NewSiteRepository constructs a SiteRepository implementation.
func NewSiteRepository(db *gorm.DB) SiteRepository { return NewMasterRepository(db) }

// NewPlacementRepository constructs a PlacementRepository implementation.
func NewPlacementRepository(db *gorm.DB) PlacementRepository { return NewMasterRepository(db) }

// NewDivisionRepository constructs a DivisionRepository implementation.
func NewDivisionRepository(db *gorm.DB) DivisionRepository { return NewMasterRepository(db) }

// NewDepartmentRepository constructs a DepartmentRepository implementation.
func NewDepartmentRepository(db *gorm.DB) DepartmentRepository { return NewMasterRepository(db) }

// NewProjectRepository constructs a ProjectRepository implementation.
func NewProjectRepository(db *gorm.DB) ProjectRepository { return NewMasterRepository(db) }

// NewHolidayRepository constructs a HolidayRepository implementation.
func NewHolidayRepository(db *gorm.DB) HolidayRepository { return NewMasterRepository(db) }

func (r *masterRepository) getDB(ctx context.Context) *gorm.DB {
	if r == nil {
		return ExtractTx(ctx, nil)
	}
	return ExtractTx(ctx, r.db)
}

// WithinTransaction executes fn within an ACID database transaction using the repository's underlying database.
func (r *masterRepository) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if r == nil || r.db == nil {
		return errors.New("master repository database connection is nil")
	}
	return NewGormTransactor(r.db).WithinTransaction(ctx, fn)
}

// --- Approvers ---
func (r *masterRepository) ListApprovers(ctx context.Context, roleType string, activeStatus *bool) ([]models.Approver, error) {
	var items []models.Approver
	q := r.getDB(ctx)
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
	err := r.getDB(ctx).Where(queryIDEquals, id).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *masterRepository) CreateApprover(ctx context.Context, a *models.Approver) error {
	return r.getDB(ctx).Create(a).Error
}

func (r *masterRepository) UpdateApprover(ctx context.Context, a *models.Approver) error {
	return r.getDB(ctx).Save(a).Error
}

func (r *masterRepository) SoftDeleteApprover(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&models.Approver{}).Where(queryIDEquals, id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Companies ---
func (r *masterRepository) ListCompanies(ctx context.Context, activeStatus *bool) ([]models.Company, error) {
	var items []models.Company
	q := r.getDB(ctx)
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	err := q.Order("id asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindCompanyByID(ctx context.Context, id uint) (*models.Company, error) {
	var c models.Company
	err := r.getDB(ctx).Where(queryIDEquals, id).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *masterRepository) FindCompanyByCode(ctx context.Context, code string) (*models.Company, error) {
	var c models.Company
	err := r.getDB(ctx).Where(queryLowerCodeEquals, code).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *masterRepository) FindActiveCompanyByCodeOrName(ctx context.Context, identifier string) (*models.Company, error) {
	var c models.Company
	err := r.getDB(ctx).Where(queryActiveCodeOrName, identifier, "%"+identifier+"%").First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *masterRepository) CreateCompany(ctx context.Context, c *models.Company) error {
	return r.getDB(ctx).Create(c).Error
}

func (r *masterRepository) UpdateCompany(ctx context.Context, c *models.Company) error {
	return r.getDB(ctx).Save(c).Error
}

func (r *masterRepository) SoftDeleteCompany(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&models.Company{}).Where(queryIDEquals, id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Sites ---
func (r *masterRepository) ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error) {
	var items []models.Site
	q := r.getDB(ctx)
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	err := q.Order("id asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindSiteByID(ctx context.Context, id uint) (*models.Site, error) {
	var s models.Site
	err := r.getDB(ctx).Where(queryIDEquals, id).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *masterRepository) FindSiteByCode(ctx context.Context, code string) (*models.Site, error) {
	var s models.Site
	err := r.getDB(ctx).Where(queryLowerCodeEquals, code).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *masterRepository) FindActiveSiteByCodeOrName(ctx context.Context, identifier string) (*models.Site, error) {
	var s models.Site
	err := r.getDB(ctx).Where(queryActiveCodeOrName, identifier, "%"+identifier+"%").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *masterRepository) CreateSite(ctx context.Context, s *models.Site) error {
	return r.getDB(ctx).Create(s).Error
}

func (r *masterRepository) UpdateSite(ctx context.Context, s *models.Site) error {
	return r.getDB(ctx).Save(s).Error
}

func (r *masterRepository) SoftDeleteSite(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&models.Site{}).Where(queryIDEquals, id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Divisions ---
func (r *masterRepository) ListDivisions(ctx context.Context, activeStatus *bool) ([]models.Division, error) {
	var items []models.Division
	q := r.getDB(ctx)
	if activeStatus != nil {
		q = q.Where("is_active = ?", *activeStatus)
	}
	err := q.Order("id asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindDivisionByID(ctx context.Context, id uint) (*models.Division, error) {
	var d models.Division
	err := r.getDB(ctx).Where(queryIDEquals, id).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) FindDivisionByCode(ctx context.Context, code string) (*models.Division, error) {
	var d models.Division
	err := r.getDB(ctx).Where(queryLowerCodeEquals, code).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) FindActiveDivisionByCodeOrName(ctx context.Context, identifier string) (*models.Division, error) {
	var d models.Division
	err := r.getDB(ctx).Where(queryActiveCodeOrName, identifier, "%"+identifier+"%").First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) CreateDivision(ctx context.Context, d *models.Division) error {
	return r.getDB(ctx).Create(d).Error
}

func (r *masterRepository) UpdateDivision(ctx context.Context, d *models.Division) error {
	return r.getDB(ctx).Save(d).Error
}

func (r *masterRepository) SoftDeleteDivision(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&models.Division{}).Where(queryIDEquals, id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Departments ---
func (r *masterRepository) ListDepartments(ctx context.Context, divisionID *uint, divisionName string, activeStatus *bool) ([]models.Department, error) {
	var items []models.Department
	q := r.getDB(ctx).Preload("DivisionRel")
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
	err := r.getDB(ctx).Preload("DivisionRel").Where(queryIDEquals, id).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) FindDepartmentByName(ctx context.Context, name string) (*models.Department, error) {
	var d models.Department
	err := r.getDB(ctx).Where("LOWER(name) = LOWER(?)", name).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) FindActiveDepartmentByCodeOrName(ctx context.Context, identifier string) (*models.Department, error) {
	var d models.Department
	err := r.getDB(ctx).Where(queryActiveCodeOrName, identifier, "%"+identifier+"%").First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *masterRepository) CreateDepartment(ctx context.Context, d *models.Department) error {
	return r.getDB(ctx).Create(d).Error
}

func (r *masterRepository) UpdateDepartment(ctx context.Context, d *models.Department) error {
	return r.getDB(ctx).Save(d).Error
}

func (r *masterRepository) SoftDeleteDepartment(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&models.Department{}).Where(queryIDEquals, id).
		Updates(map[string]interface{}{"is_active": false, "updated_at": time.Now()}).Error
}

// --- Projects & ActivityStatuses ---
func (r *masterRepository) ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error) {
	var items []models.Project
	q := r.getDB(ctx)
	if activeOnly {
		q = q.Scopes(models.ActiveOnly)
	}
	err := q.Order("name asc").Find(&items).Error
	return items, err
}

func (r *masterRepository) FindProjectByID(ctx context.Context, id uint) (*models.Project, error) {
	var p models.Project
	if err := r.getDB(ctx).Scopes(models.ActiveOnly).Where(queryIDEquals, id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *masterRepository) ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error) {
	var items []models.ActivityStatus
	err := r.getDB(ctx).Order("sort_order asc").Find(&items).Error
	return items, err
}

// --- Holidays ---
func (r *masterRepository) ListHolidaysByMonth(ctx context.Context, year, month int) ([]models.Holiday, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var holidays []models.Holiday
	err := r.getDB(ctx).
		Where("date >= ? AND date < ?", start, end).
		Order("date asc").
		Find(&holidays).Error
	return holidays, err
}

func (r *masterRepository) ListAllHolidays(ctx context.Context, year *int) ([]models.Holiday, error) {
	var holidays []models.Holiday
	q := r.getDB(ctx).Order("date asc")
	if year != nil {
		start := time.Date(*year, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(1, 0, 0)
		q = q.Where("date >= ? AND date < ?", start, end)
	}
	err := q.Find(&holidays).Error
	return holidays, err
}

func (r *masterRepository) UpsertHolidays(ctx context.Context, holidays []models.Holiday) error {
	if len(holidays) == 0 {
		return nil
	}
	return r.getDB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "is_joint_leave", "is_civic", "is_religious", "updated_at"}),
	}).Create(&holidays).Error
}
