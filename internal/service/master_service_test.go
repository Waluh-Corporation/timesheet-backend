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
	if !ok {
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
	if !ok {
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
	if !ok {
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
	if !ok {
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
	if !ok {
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

func (m *mockMasterRepo) ListHolidaysByMonth(ctx context.Context, year, month int) ([]models.Holiday, error) {
	return nil, nil
}

func (m *mockMasterRepo) UpsertHolidays(ctx context.Context, holidays []models.Holiday) error {
	return nil
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

	// Update empty name error
	emptyName := "   "
	if _, err := svc.UpdateApprover(ctx, a.ID, &emptyName, nil, nil, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty name, got %v", err)
	}

	// Update not found
	if _, err := svc.UpdateApprover(ctx, 999, &newName, nil, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
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
	if err := svc.DeleteApprover(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on Delete, got %v", err)
	}
}

func TestMasterDataService_Company(t *testing.T) {
	repo := newMockMasterRepo()
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	// Validation
	if _, err := svc.CreateCompany(ctx, "", "Company"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// Create
	comp, err := svc.CreateCompany(ctx, "mii", "PT MII")
	if err != nil {
		t.Fatalf("CreateCompany failed: %v", err)
	}

	// List
	comps, err := svc.ListCompanies(ctx, nil)
	if err != nil || len(comps) != 1 {
		t.Fatalf("ListCompanies failed: %v", err)
	}

	// Update
	newCode := "mii2"
	newName := "PT MII New"
	isActive := true
	updated, err := svc.UpdateCompany(ctx, comp.ID, &newCode, &newName, &isActive)
	if err != nil {
		t.Fatalf("UpdateCompany failed: %v", err)
	}
	if updated.Code != "mii2" || updated.Name != "PT MII New" {
		t.Errorf("unexpected updated company: %+v", updated)
	}

	// Update not found
	if _, err := svc.UpdateCompany(ctx, 999, &newCode, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Delete
	if err := svc.DeleteCompany(ctx, comp.ID); err != nil {
		t.Fatalf("DeleteCompany failed: %v", err)
	}
	if err := svc.DeleteCompany(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on delete, got %v", err)
	}
}

func TestMasterDataService_Site(t *testing.T) {
	repo := newMockMasterRepo()
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	// Validation
	if _, err := svc.CreateSite(ctx, "", "Site", nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// Create
	site, err := svc.CreateSite(ctx, "RDTX", "RDTX Tower", nil)
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}

	// List
	sites, err := svc.ListSites(ctx, nil)
	if err != nil || len(sites) != 1 {
		t.Fatalf("ListSites failed: %v", err)
	}

	// Update
	newCode := "rdtx2"
	newName := "RDTX Tower 2"
	isActive := true
	updated, err := svc.UpdateSite(ctx, site.ID, &newCode, &newName, &isActive)
	if err != nil {
		t.Fatalf("UpdateSite failed: %v", err)
	}
	if updated.Code != "rdtx2" {
		t.Errorf("expected rdtx2, got %s", updated.Code)
	}

	// Update not found
	if _, err := svc.UpdateSite(ctx, 999, &newCode, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Delete
	if err := svc.DeleteSite(ctx, site.ID); err != nil {
		t.Fatalf("DeleteSite failed: %v", err)
	}
	if err := svc.DeleteSite(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on delete, got %v", err)
	}
}

func TestMasterDataService_Division(t *testing.T) {
	repo := newMockMasterRepo()
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	// Validation
	if _, err := svc.CreateDivision(ctx, "", "Division", nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// Create
	div, err := svc.CreateDivision(ctx, "WDD", "Wholesale Digital Delivery", nil)
	if err != nil {
		t.Fatalf("CreateDivision failed: %v", err)
	}

	// List
	divs, err := svc.ListDivisions(ctx, nil)
	if err != nil || len(divs) != 1 {
		t.Fatalf("ListDivisions failed: %v", err)
	}

	// Update
	newCode := "wdd2"
	newName := "Wholesale Digital Delivery 2"
	isActive := true
	updated, err := svc.UpdateDivision(ctx, div.ID, &newCode, &newName, &isActive)
	if err != nil {
		t.Fatalf("UpdateDivision failed: %v", err)
	}
	if updated.Code != "wdd2" {
		t.Errorf("expected wdd2, got %s", updated.Code)
	}

	// Update not found
	if _, err := svc.UpdateDivision(ctx, 999, &newCode, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Delete
	if err := svc.DeleteDivision(ctx, div.ID); err != nil {
		t.Fatalf("DeleteDivision failed: %v", err)
	}
	if err := svc.DeleteDivision(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on delete, got %v", err)
	}
}

func TestMasterDataService_Department(t *testing.T) {
	repo := newMockMasterRepo()
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	// Create division first
	div, _ := svc.CreateDivision(ctx, "WDD", "Wholesale Digital Delivery", nil)

	// Validation
	if _, err := svc.CreateDepartment(ctx, "DEV", "", "", nil, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// Create with division ID
	dept, err := svc.CreateDepartment(ctx, "DEV", "Development Team", "", &div.ID, nil)
	if err != nil {
		t.Fatalf("CreateDepartment failed: %v", err)
	}
	if dept.Division != "Wholesale Digital Delivery" {
		t.Errorf("expected division name set, got %s", dept.Division)
	}

	// Create with division name string
	dept2, err := svc.CreateDepartment(ctx, "QA", "QA Team", "Wholesale Digital Delivery", nil, nil)
	if err != nil {
		t.Fatalf("CreateDepartment with division name failed: %v", err)
	}
	if dept2.DivisionID == nil || *dept2.DivisionID != div.ID {
		t.Errorf("expected division ID resolved, got %v", dept2.DivisionID)
	}

	// List
	depts, err := svc.ListDepartments(ctx, &div.ID, "", nil)
	if err != nil || len(depts) != 2 {
		t.Fatalf("ListDepartments failed: %v", err)
	}

	// Update
	newName := "Dev Team Renamed"
	newCode := "DEVR"
	newDivName := "Wholesale Digital Delivery"
	updated, err := svc.UpdateDepartment(ctx, dept.ID, &newCode, &newName, &newDivName, nil, nil)
	if err != nil {
		t.Fatalf("UpdateDepartment failed: %v", err)
	}
	if updated.Code != "DEVR" || updated.Name != newName {
		t.Errorf("unexpected updated department: %+v", updated)
	}

	// Update clearing division with divisionID = 0
	zero := uint(0)
	cleared, err := svc.UpdateDepartment(ctx, dept.ID, nil, nil, nil, &zero, nil)
	if err != nil || cleared.DivisionID != nil || cleared.Division != "" {
		t.Errorf("expected division cleared, got: %+v", cleared)
	}

	// Update clearing division with empty string
	emptyDiv := ""
	cleared2, err := svc.UpdateDepartment(ctx, dept.ID, nil, nil, &emptyDiv, nil, nil)
	if err != nil || cleared2.DivisionID != nil || cleared2.Division != "" {
		t.Errorf("expected division cleared, got: %+v", cleared2)
	}

	// Update not found
	if _, err := svc.UpdateDepartment(ctx, 999, &newCode, nil, nil, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Delete
	if err := svc.DeleteDepartment(ctx, dept.ID); err != nil {
		t.Fatalf("DeleteDepartment failed: %v", err)
	}
	if err := svc.DeleteDepartment(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on delete, got %v", err)
	}
}

func TestMasterDataService_ProjectsAndStatuses(t *testing.T) {
	repo := newMockMasterRepo()
	repo.projects = []models.Project{{ID: 1, Name: "Proj1", IsActive: true}}
	repo.statuses = []models.ActivityStatus{{Code: "P", Name: "Present"}}
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	projs, err := svc.ListProjects(ctx, true)
	if err != nil || len(projs) != 1 {
		t.Fatalf("ListProjects failed: %v", err)
	}

	statuses, err := svc.ListActivityStatuses(ctx)
	if err != nil || len(statuses) != 1 {
		t.Fatalf("ListActivityStatuses failed: %v", err)
	}
}

func TestMasterDataService_Reactivation(t *testing.T) {
	repo := newMockMasterRepo()
	svc := service.NewMasterDataService(repo)
	ctx := context.Background()

	// 1. Approver
	appr, _ := svc.CreateApprover(ctx, "Test Approver", models.ApproverRoleTeamLeader, "Lead", nil)
	_ = svc.DeleteApprover(ctx, appr.ID)
	activeTrue := true
	reactivatedAppr, err := svc.UpdateApprover(ctx, appr.ID, nil, nil, nil, &activeTrue)
	if err != nil || !reactivatedAppr.IsActive {
		t.Fatalf("failed to reactivate approver: %v", err)
	}

	// 2. Company
	comp, _ := svc.CreateCompany(ctx, "COMP", "Company Corp")
	_ = svc.DeleteCompany(ctx, comp.ID)
	reactivatedComp, err := svc.UpdateCompany(ctx, comp.ID, nil, nil, &activeTrue)
	if err != nil || !reactivatedComp.IsActive {
		t.Fatalf("failed to reactivate company: %v", err)
	}

	// 3. Site
	site, _ := svc.CreateSite(ctx, "SITE", "Site Location", nil)
	_ = svc.DeleteSite(ctx, site.ID)
	reactivatedSite, err := svc.UpdateSite(ctx, site.ID, nil, nil, &activeTrue)
	if err != nil || !reactivatedSite.IsActive {
		t.Fatalf("failed to reactivate site: %v", err)
	}

	// 4. Division
	div, _ := svc.CreateDivision(ctx, "DIV", "Division Name", nil)
	_ = svc.DeleteDivision(ctx, div.ID)
	reactivatedDiv, err := svc.UpdateDivision(ctx, div.ID, nil, nil, &activeTrue)
	if err != nil || !reactivatedDiv.IsActive {
		t.Fatalf("failed to reactivate division: %v", err)
	}

	// 5. Department
	dept, _ := svc.CreateDepartment(ctx, "DEPT", "Dept Name", "", nil, nil)
	_ = svc.DeleteDepartment(ctx, dept.ID)
	reactivatedDept, err := svc.UpdateDepartment(ctx, dept.ID, nil, nil, nil, nil, &activeTrue)
	if err != nil || !reactivatedDept.IsActive {
		t.Fatalf("failed to reactivate department: %v", err)
	}
}
