package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

type mockMasterRepo struct {
	approvers   map[uint]*models.Approver
	companies   map[uint]*models.Company
	sites       map[uint]*models.Site
	divisions   map[uint]*models.Division
	departments map[uint]*models.Department
	projects    []models.Project
	statuses    []models.ActivityStatus
}

func newMockMasterRepo() *mockMasterRepo {
	return &mockMasterRepo{
		approvers:   make(map[uint]*models.Approver),
		companies:   make(map[uint]*models.Company),
		sites:       make(map[uint]*models.Site),
		divisions:   make(map[uint]*models.Division),
		departments: make(map[uint]*models.Department),
	}
}

func (m *mockMasterRepo) ListApprovers(ctx context.Context, roleType string, activeStatus *bool) ([]models.Approver, error) {
	var list []models.Approver
	for _, a := range m.approvers {
		if activeStatus != nil && a.IsActive != *activeStatus {
			continue
		}
		if roleType != "" && string(a.RoleType) != roleType {
			continue
		}
		list = append(list, *a)
	}
	return list, nil
}
func (m *mockMasterRepo) FindApproverByID(ctx context.Context, id uint) (*models.Approver, error) {
	a, ok := m.approvers[id]
	if !ok || !a.IsActive {
		return nil, errors.New("not found")
	}
	return a, nil
}
func (m *mockMasterRepo) CreateApprover(ctx context.Context, a *models.Approver) error {
	a.ID = uint(len(m.approvers) + 1)
	m.approvers[a.ID] = a
	return nil
}
func (m *mockMasterRepo) UpdateApprover(ctx context.Context, a *models.Approver) error {
	m.approvers[a.ID] = a
	return nil
}
func (m *mockMasterRepo) SoftDeleteApprover(ctx context.Context, id uint) error {
	if a, ok := m.approvers[id]; ok {
		a.IsActive = false
	}
	return nil
}

func (m *mockMasterRepo) ListCompanies(ctx context.Context, activeStatus *bool) ([]models.Company, error) {
	var list []models.Company
	for _, c := range m.companies {
		if activeStatus != nil && c.IsActive != *activeStatus {
			continue
		}
		list = append(list, *c)
	}
	return list, nil
}
func (m *mockMasterRepo) FindCompanyByID(ctx context.Context, id uint) (*models.Company, error) {
	c, ok := m.companies[id]
	if !ok || !c.IsActive {
		return nil, errors.New("not found")
	}
	return c, nil
}
func (m *mockMasterRepo) FindCompanyByCode(ctx context.Context, code string) (*models.Company, error) {
	for _, c := range m.companies {
		if strings.EqualFold(c.Code, code) {
			return c, nil
		}
	}
	return nil, nil
}
func (m *mockMasterRepo) CreateCompany(ctx context.Context, c *models.Company) error {
	c.ID = uint(len(m.companies) + 1)
	m.companies[c.ID] = c
	return nil
}
func (m *mockMasterRepo) UpdateCompany(ctx context.Context, c *models.Company) error {
	m.companies[c.ID] = c
	return nil
}
func (m *mockMasterRepo) SoftDeleteCompany(ctx context.Context, id uint) error {
	if c, ok := m.companies[id]; ok {
		c.IsActive = false
	}
	return nil
}

func (m *mockMasterRepo) ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error) {
	var list []models.Site
	for _, s := range m.sites {
		if activeStatus != nil && s.IsActive != *activeStatus {
			continue
		}
		list = append(list, *s)
	}
	return list, nil
}
func (m *mockMasterRepo) FindSiteByID(ctx context.Context, id uint) (*models.Site, error) {
	s, ok := m.sites[id]
	if !ok || !s.IsActive {
		return nil, errors.New("not found")
	}
	return s, nil
}
func (m *mockMasterRepo) FindSiteByCode(ctx context.Context, code string) (*models.Site, error) {
	for _, s := range m.sites {
		if strings.EqualFold(s.Code, code) {
			return s, nil
		}
	}
	return nil, nil
}
func (m *mockMasterRepo) CreateSite(ctx context.Context, s *models.Site) error {
	s.ID = uint(len(m.sites) + 1)
	m.sites[s.ID] = s
	return nil
}
func (m *mockMasterRepo) UpdateSite(ctx context.Context, s *models.Site) error {
	m.sites[s.ID] = s
	return nil
}
func (m *mockMasterRepo) SoftDeleteSite(ctx context.Context, id uint) error {
	if s, ok := m.sites[id]; ok {
		s.IsActive = false
	}
	return nil
}

func (m *mockMasterRepo) ListDivisions(ctx context.Context, activeStatus *bool) ([]models.Division, error) {
	var list []models.Division
	for _, d := range m.divisions {
		if activeStatus != nil && d.IsActive != *activeStatus {
			continue
		}
		list = append(list, *d)
	}
	return list, nil
}
func (m *mockMasterRepo) FindDivisionByID(ctx context.Context, id uint) (*models.Division, error) {
	d, ok := m.divisions[id]
	if !ok || !d.IsActive {
		return nil, errors.New("not found")
	}
	return d, nil
}
func (m *mockMasterRepo) FindDivisionByCode(ctx context.Context, code string) (*models.Division, error) {
	for _, d := range m.divisions {
		if strings.EqualFold(d.Code, code) {
			return d, nil
		}
	}
	return nil, nil
}
func (m *mockMasterRepo) FindActiveDivisionByCodeOrName(ctx context.Context, identifier string) (*models.Division, error) {
	for _, d := range m.divisions {
		if d.IsActive && (strings.EqualFold(d.Code, identifier) || strings.EqualFold(d.Name, identifier)) {
			return d, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockMasterRepo) CreateDivision(ctx context.Context, d *models.Division) error {
	d.ID = uint(len(m.divisions) + 1)
	m.divisions[d.ID] = d
	return nil
}
func (m *mockMasterRepo) UpdateDivision(ctx context.Context, d *models.Division) error {
	m.divisions[d.ID] = d
	return nil
}
func (m *mockMasterRepo) SoftDeleteDivision(ctx context.Context, id uint) error {
	if d, ok := m.divisions[id]; ok {
		d.IsActive = false
	}
	return nil
}

func (m *mockMasterRepo) ListDepartments(ctx context.Context, divisionID *uint, divisionName string, activeStatus *bool) ([]models.Department, error) {
	var list []models.Department
	for _, dept := range m.departments {
		if activeStatus != nil && dept.IsActive != *activeStatus {
			continue
		}
		if divisionID != nil && (dept.DivisionID == nil || *dept.DivisionID != *divisionID) {
			continue
		}
		if divisionName != "" && !strings.EqualFold(dept.Division, divisionName) {
			continue
		}
		list = append(list, *dept)
	}
	return list, nil
}
func (m *mockMasterRepo) FindDepartmentByID(ctx context.Context, id uint) (*models.Department, error) {
	d, ok := m.departments[id]
	if !ok || !d.IsActive {
		return nil, errors.New("not found")
	}
	return d, nil
}
func (m *mockMasterRepo) FindDepartmentByName(ctx context.Context, name string) (*models.Department, error) {
	for _, d := range m.departments {
		if strings.EqualFold(d.Name, name) {
			return d, nil
		}
	}
	return nil, nil
}
func (m *mockMasterRepo) CreateDepartment(ctx context.Context, d *models.Department) error {
	d.ID = uint(len(m.departments) + 1)
	m.departments[d.ID] = d
	return nil
}
func (m *mockMasterRepo) UpdateDepartment(ctx context.Context, d *models.Department) error {
	m.departments[d.ID] = d
	return nil
}
func (m *mockMasterRepo) SoftDeleteDepartment(ctx context.Context, id uint) error {
	if d, ok := m.departments[id]; ok {
		d.IsActive = false
	}
	return nil
}

func (m *mockMasterRepo) ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error) {
	return m.projects, nil
}
func (m *mockMasterRepo) ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error) {
	return m.statuses, nil
}

var _ repository.MasterRepository = (*mockMasterRepo)(nil)

func TestMasterDataService_Approvers(t *testing.T) {
	repo := newMockMasterRepo()
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	// Validation
	_, err := svc.CreateApprover(ctx, "", models.ApproverRoleTeamLeader, "", nil)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// Create
	a, err := svc.CreateApprover(ctx, "John TL", models.ApproverRoleTeamLeader, "Lead", nil)
	if err != nil {
		t.Fatalf("CreateApprover failed: %v", err)
	}
	if a.ID != 1 || a.Name != "John TL" {
		t.Errorf("unexpected approver: %+v", a)
	}

	// Update
	newName := "John Senior TL"
	updated, err := svc.UpdateApprover(ctx, a.ID, &newName, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateApprover failed: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected updated name, got %s", updated.Name)
	}

	// List
	active := true
	list, err := svc.ListApprovers(ctx, "", &active)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 approver, got %d", len(list))
	}

	// Delete
	if err := svc.DeleteApprover(ctx, a.ID); err != nil {
		t.Fatalf("DeleteApprover failed: %v", err)
	}
}

func TestMasterDataService_CompanyAndDept(t *testing.T) {
	repo := newMockMasterRepo()
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	// Company
	comp, err := svc.CreateCompany(ctx, "mii", "PT MII")
	if err != nil {
		t.Fatalf("CreateCompany failed: %v", err)
	}

	// Division
	div, err := svc.CreateDivision(ctx, "WDD", "Wholesale Digital Delivery", nil)
	if err != nil {
		t.Fatalf("CreateDivision failed: %v", err)
	}

	// Department with division
	dept, err := svc.CreateDepartment(ctx, "DEV", "Development Team", "", &div.ID, nil)
	if err != nil {
		t.Fatalf("CreateDepartment failed: %v", err)
	}
	if dept.Division != "Wholesale Digital Delivery" {
		t.Errorf("expected division name set, got: %s", dept.Division)
	}

	// Site
	site, err := svc.CreateSite(ctx, "RDTX", "RDTX Tower", nil)
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if site.ID != 1 {
		t.Errorf("expected site ID 1, got %d", site.ID)
	}

	_ = comp
}
