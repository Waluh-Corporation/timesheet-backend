package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// MasterRepository is re-exported from domain.MasterRepository.
type MasterRepository = domain.MasterRepository

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

func (r *masterRepository) FindActiveCompanyByCodeOrName(ctx context.Context, identifier string) (*models.Company, error) {
	var c models.Company
	err := r.db.WithContext(ctx).Where("(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true", identifier, "%"+identifier+"%").First(&c).Error
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

func (r *masterRepository) FindActiveSiteByCodeOrName(ctx context.Context, identifier string) (*models.Site, error) {
	var s models.Site
	err := r.db.WithContext(ctx).Where("(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true", identifier, "%"+identifier+"%").First(&s).Error
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

func (r *masterRepository) FindActiveDepartmentByCodeOrName(ctx context.Context, identifier string) (*models.Department, error) {
	var d models.Department
	err := r.db.WithContext(ctx).Where("(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true", identifier, "%"+identifier+"%").First(&d).Error
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

func (r *masterRepository) FindProjectByID(ctx context.Context, id uint) (*models.Project, error) {
	var p models.Project
	if err := r.db.WithContext(ctx).Scopes(models.ActiveOnly).Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *masterRepository) ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error) {
	var items []models.ActivityStatus
	err := r.db.WithContext(ctx).Order("sort_order asc").Find(&items).Error
	return items, err
}

// --- Holidays ---
func (r *masterRepository) ListHolidaysByMonth(ctx context.Context, year, month int) ([]models.Holiday, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var holidays []models.Holiday
	err := r.db.WithContext(ctx).
		Where("date >= ? AND date < ?", start, end).
		Order("date asc").
		Find(&holidays).Error
	return holidays, err
}

func (r *masterRepository) ListAllHolidays(ctx context.Context, year *int) ([]models.Holiday, error) {
	var holidays []models.Holiday
	q := r.db.WithContext(ctx).Order("date asc")
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
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "is_joint_leave", "is_civic", "is_religious", "updated_at"}),
	}).Create(&holidays).Error
}
