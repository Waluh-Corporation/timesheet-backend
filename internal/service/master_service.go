package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

// MasterDataService defines business logic for master data operations.
type MasterDataService interface {
	// Approver
	ListApprovers(ctx context.Context, roleType string, activeStatus *bool) ([]models.Approver, error)
	CreateApprover(ctx context.Context, name string, roleType models.ApproverRoleType, title string, isActive *bool) (*models.Approver, error)
	UpdateApprover(ctx context.Context, id uint, name *string, roleType *models.ApproverRoleType, title *string, isActive *bool) (*models.Approver, error)
	DeleteApprover(ctx context.Context, id uint) error

	// Company
	ListCompanies(ctx context.Context, activeStatus *bool) ([]models.Company, error)
	CreateCompany(ctx context.Context, code string, name string) (*models.Company, error)
	UpdateCompany(ctx context.Context, id uint, code *string, name *string, isActive *bool) (*models.Company, error)
	DeleteCompany(ctx context.Context, id uint) error

	// Site
	ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error)
	CreateSite(ctx context.Context, code string, name string, isActive *bool) (*models.Site, error)
	UpdateSite(ctx context.Context, id uint, code *string, name *string, isActive *bool) (*models.Site, error)
	DeleteSite(ctx context.Context, id uint) error

	// Division
	ListDivisions(ctx context.Context, activeStatus *bool) ([]models.Division, error)
	CreateDivision(ctx context.Context, code string, name string, isActive *bool) (*models.Division, error)
	UpdateDivision(ctx context.Context, id uint, code *string, name *string, isActive *bool) (*models.Division, error)
	DeleteDivision(ctx context.Context, id uint) error

	// Department
	ListDepartments(ctx context.Context, divisionID *uint, divisionName string, activeStatus *bool) ([]models.Department, error)
	CreateDepartment(ctx context.Context, code string, name string, division string, divisionID *uint, isActive *bool) (*models.Department, error)
	UpdateDepartment(ctx context.Context, id uint, code *string, name *string, division *string, divisionID *uint, isActive *bool) (*models.Department, error)
	DeleteDepartment(ctx context.Context, id uint) error

	// Projects & ActivityStatuses
	ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error)
	FindProjectByID(ctx context.Context, id uint) (*models.Project, error)
	ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error)

	// Holidays
	GetHolidays(ctx context.Context, year, month int) ([]models.HolidayDTO, error)
	SyncHolidays(ctx context.Context, year int) (int, error)
	ListAllHolidays(ctx context.Context, year *int) ([]models.Holiday, error)
}

type masterDataService struct {
	repo repository.MasterRepository
}

// NewMasterDataService constructs a MasterDataService.
func NewMasterDataService(repo repository.MasterRepository) MasterDataService {
	return &masterDataService{repo: repo}
}

// --- Approvers ---
func (s *masterDataService) ListApprovers(ctx context.Context, roleType string, activeStatus *bool) ([]models.Approver, error) {
	return s.repo.ListApprovers(ctx, roleType, activeStatus)
}

func (s *masterDataService) CreateApprover(ctx context.Context, name string, roleType models.ApproverRoleType, title string, isActive *bool) (*models.Approver, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Name is required")
	}
	active := true
	if isActive != nil {
		active = *isActive
	}
	a := &models.Approver{
		Name:     name,
		RoleType: roleType,
		Title:    strings.TrimSpace(title),
		IsActive: active,
	}
	if err := s.repo.CreateApprover(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *masterDataService) UpdateApprover(ctx context.Context, id uint, name *string, roleType *models.ApproverRoleType, title *string, isActive *bool) (*models.Approver, error) {
	a, err := s.repo.FindApproverByID(ctx, id)
	if err != nil || a == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Approver not found")
	}
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return nil, domain.NewUserError(domain.ErrInvalidInput, "Name cannot be empty")
		}
		a.Name = trimmed
	}
	if roleType != nil {
		a.RoleType = *roleType
	}
	if title != nil {
		a.Title = strings.TrimSpace(*title)
	}
	if isActive != nil {
		a.IsActive = *isActive
	}
	if err := s.repo.UpdateApprover(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *masterDataService) DeleteApprover(ctx context.Context, id uint) error {
	a, err := s.repo.FindApproverByID(ctx, id)
	if err != nil || a == nil {
		return domain.NewUserError(domain.ErrNotFound, "Approver not found")
	}
	return s.repo.SoftDeleteApprover(ctx, id)
}

// --- Companies ---
func (s *masterDataService) ListCompanies(ctx context.Context, activeStatus *bool) ([]models.Company, error) {
	return s.repo.ListCompanies(ctx, activeStatus)
}

func (s *masterDataService) CreateCompany(ctx context.Context, code string, name string) (*models.Company, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Code and name are required")
	}
	c := &models.Company{
		Code:     code,
		Name:     name,
		IsActive: true,
	}
	if err := s.repo.CreateCompany(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *masterDataService) UpdateCompany(ctx context.Context, id uint, code *string, name *string, isActive *bool) (*models.Company, error) {
	c, err := s.repo.FindCompanyByID(ctx, id)
	if err != nil || c == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Company not found")
	}
	if code != nil {
		c.Code = strings.ToLower(strings.TrimSpace(*code))
	}
	if name != nil {
		c.Name = strings.TrimSpace(*name)
	}
	if isActive != nil {
		c.IsActive = *isActive
	}
	if err := s.repo.UpdateCompany(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *masterDataService) DeleteCompany(ctx context.Context, id uint) error {
	c, err := s.repo.FindCompanyByID(ctx, id)
	if err != nil || c == nil {
		return domain.NewUserError(domain.ErrNotFound, "Company not found")
	}
	return s.repo.SoftDeleteCompany(ctx, id)
}

// --- Sites ---
func (s *masterDataService) ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error) {
	return s.repo.ListSites(ctx, activeStatus)
}

func (s *masterDataService) CreateSite(ctx context.Context, code string, name string, isActive *bool) (*models.Site, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Code and name are required")
	}
	active := true
	if isActive != nil {
		active = *isActive
	}
	site := &models.Site{
		Code:     code,
		Name:     name,
		IsActive: active,
	}
	if err := s.repo.CreateSite(ctx, site); err != nil {
		return nil, err
	}
	return site, nil
}

func (s *masterDataService) UpdateSite(ctx context.Context, id uint, code *string, name *string, isActive *bool) (*models.Site, error) {
	site, err := s.repo.FindSiteByID(ctx, id)
	if err != nil || site == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Site not found")
	}
	if code != nil {
		site.Code = strings.ToLower(strings.TrimSpace(*code))
	}
	if name != nil {
		site.Name = strings.TrimSpace(*name)
	}
	if isActive != nil {
		site.IsActive = *isActive
	}
	if err := s.repo.UpdateSite(ctx, site); err != nil {
		return nil, err
	}
	return site, nil
}

func (s *masterDataService) DeleteSite(ctx context.Context, id uint) error {
	site, err := s.repo.FindSiteByID(ctx, id)
	if err != nil || site == nil {
		return domain.NewUserError(domain.ErrNotFound, "Site not found")
	}
	return s.repo.SoftDeleteSite(ctx, id)
}

// --- Divisions ---
func (s *masterDataService) ListDivisions(ctx context.Context, activeStatus *bool) ([]models.Division, error) {
	return s.repo.ListDivisions(ctx, activeStatus)
}

func (s *masterDataService) CreateDivision(ctx context.Context, code string, name string, isActive *bool) (*models.Division, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Code and name are required")
	}
	active := true
	if isActive != nil {
		active = *isActive
	}
	div := &models.Division{
		Code:     code,
		Name:     name,
		IsActive: active,
	}
	if err := s.repo.CreateDivision(ctx, div); err != nil {
		return nil, err
	}
	return div, nil
}

func (s *masterDataService) UpdateDivision(ctx context.Context, id uint, code *string, name *string, isActive *bool) (*models.Division, error) {
	div, err := s.repo.FindDivisionByID(ctx, id)
	if err != nil || div == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Division not found")
	}
	if code != nil {
		div.Code = strings.ToLower(strings.TrimSpace(*code))
	}
	if name != nil {
		div.Name = strings.TrimSpace(*name)
	}
	if isActive != nil {
		div.IsActive = *isActive
	}
	if err := s.repo.UpdateDivision(ctx, div); err != nil {
		return nil, err
	}
	return div, nil
}

func (s *masterDataService) DeleteDivision(ctx context.Context, id uint) error {
	div, err := s.repo.FindDivisionByID(ctx, id)
	if err != nil || div == nil {
		return domain.NewUserError(domain.ErrNotFound, "Division not found")
	}
	return s.repo.SoftDeleteDivision(ctx, id)
}

// --- Departments ---
func (s *masterDataService) ListDepartments(ctx context.Context, divisionID *uint, divisionName string, activeStatus *bool) ([]models.Department, error) {
	return s.repo.ListDepartments(ctx, divisionID, divisionName, activeStatus)
}

func (s *masterDataService) CreateDepartment(ctx context.Context, code string, name string, division string, divisionID *uint, isActive *bool) (*models.Department, error) {
	name = strings.TrimSpace(name)
	code = strings.ToUpper(strings.TrimSpace(code))
	if name == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Name is required")
	}
	active := true
	if isActive != nil {
		active = *isActive
	}

	divName := strings.TrimSpace(division)
	resolvedDivID := divisionID
	if divisionID != nil && *divisionID != 0 {
		div, err := s.repo.FindDivisionByID(ctx, *divisionID)
		if err == nil && div != nil {
			divName = div.Name
			resolvedDivID = &div.ID
		}
	} else if divName != "" {
		div, err := s.repo.FindActiveDivisionByCodeOrName(ctx, divName)
		if err == nil && div != nil {
			divName = div.Name
			resolvedDivID = &div.ID
		}
	}

	dept := &models.Department{
		Code:       code,
		Name:       name,
		Division:   divName,
		DivisionID: resolvedDivID,
		IsActive:   active,
	}
	if err := s.repo.CreateDepartment(ctx, dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *masterDataService) UpdateDepartment(ctx context.Context, id uint, code *string, name *string, division *string, divisionID *uint, isActive *bool) (*models.Department, error) {
	dept, err := s.repo.FindDepartmentByID(ctx, id)
	if err != nil || dept == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Department not found")
	}
	if name != nil {
		dept.Name = strings.TrimSpace(*name)
	}
	if code != nil {
		dept.Code = strings.ToUpper(strings.TrimSpace(*code))
	}
	if divisionID != nil {
		if *divisionID != 0 {
			div, err := s.repo.FindDivisionByID(ctx, *divisionID)
			if err == nil && div != nil {
				dept.DivisionID = &div.ID
				dept.Division = div.Name
			}
		} else {
			dept.DivisionID = nil
			dept.Division = ""
		}
	} else if division != nil {
		trimmed := strings.TrimSpace(*division)
		if trimmed != "" {
			div, err := s.repo.FindActiveDivisionByCodeOrName(ctx, trimmed)
			if err == nil && div != nil {
				dept.DivisionID = &div.ID
				dept.Division = div.Name
			} else {
				dept.DivisionID = nil
				dept.Division = ""
			}
		} else {
			dept.DivisionID = nil
			dept.Division = ""
		}
	}
	if isActive != nil {
		dept.IsActive = *isActive
	}
	if err := s.repo.UpdateDepartment(ctx, dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *masterDataService) DeleteDepartment(ctx context.Context, id uint) error {
	dept, err := s.repo.FindDepartmentByID(ctx, id)
	if err != nil || dept == nil {
		return domain.NewUserError(domain.ErrNotFound, "Department not found")
	}
	return s.repo.SoftDeleteDepartment(ctx, id)
}

// --- Projects & Statuses ---
func (s *masterDataService) ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error) {
	return s.repo.ListProjects(ctx, activeOnly)
}

func (s *masterDataService) FindProjectByID(ctx context.Context, id uint) (*models.Project, error) {
	return s.repo.FindProjectByID(ctx, id)
}

func (s *masterDataService) ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error) {
	return s.repo.ListActivityStatuses(ctx)
}

// --- Holidays ---
func (s *masterDataService) GetHolidays(ctx context.Context, year, month int) ([]models.HolidayDTO, error) {
	if yearlyHolidays, err := services.FetchHolidaysByYear(year); err == nil && len(yearlyHolidays) > 0 {
		_ = s.saveYearlyHolidays(ctx, yearlyHolidays)
		prefix := fmt.Sprintf("%04d-%02d-", year, month)
		monthHolidays := make([]models.HolidayDTO, 0, len(yearlyHolidays))
		for _, h := range yearlyHolidays {
			if strings.HasPrefix(h.Date, prefix) {
				monthHolidays = append(monthHolidays, h)
			}
		}
		return monthHolidays, nil
	}

	dbHolidays, err := s.repo.ListHolidaysByMonth(ctx, year, month)
	if err == nil && len(dbHolidays) > 0 {
		resp := make([]models.HolidayDTO, 0, len(dbHolidays))
		for _, dh := range dbHolidays {
			resp = append(resp, models.HolidayDTO{
				Date:          dh.Date.Format("2006-01-02"),
				Description:   dh.Description,
				IsJointLeave:  dh.IsJointLeave,
				IsCutiBersama: dh.IsJointLeave,
				IsCivic:       dh.IsCivic,
				IsReligious:   dh.IsReligious,
			})
		}
		return resp, nil
	}
	return []models.HolidayDTO{}, nil
}

func (s *masterDataService) SyncHolidays(ctx context.Context, year int) (int, error) {
	yearlyHolidays, err := services.FetchHolidaysByYear(year)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch holidays from Kemendesa API: %w", err)
	}
	if len(yearlyHolidays) == 0 {
		return 0, nil
	}
	syncedCount := s.saveYearlyHolidays(ctx, yearlyHolidays)
	return syncedCount, nil
}

func (s *masterDataService) ListAllHolidays(ctx context.Context, year *int) ([]models.Holiday, error) {
	return s.repo.ListAllHolidays(ctx, year)
}

func (s *masterDataService) saveYearlyHolidays(ctx context.Context, yearlyHolidays []models.HolidayDTO) int {
	modelsHolidays := make([]models.Holiday, 0, len(yearlyHolidays))
	for _, h := range yearlyHolidays {
		if t, parseErr := time.Parse("2006-01-02", h.Date); parseErr == nil {
			modelsHolidays = append(modelsHolidays, models.Holiday{
				Date:         t,
				Description:  h.Description,
				IsJointLeave: h.IsJointLeave,
				IsCivic:      h.IsCivic,
				IsReligious:  h.IsReligious,
			})
		}
	}
	if len(modelsHolidays) > 0 {
		if err := s.repo.UpsertHolidays(ctx, modelsHolidays); err == nil {
			return len(modelsHolidays)
		}
	}
	return 0
}
