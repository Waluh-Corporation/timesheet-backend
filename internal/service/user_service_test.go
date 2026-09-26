package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

type mockUserRepo struct {
	users          map[uint]*models.User
	updatedPass    map[uint]string
	updatedAtTimes map[uint]time.Time
	profileChanges map[uint]*models.ProfileChangeRequest
	createErr      error
	updatePassErr  error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:          make(map[uint]*models.User),
		updatedPass:    make(map[uint]string),
		updatedAtTimes: make(map[uint]time.Time),
		profileChanges: make(map[uint]*models.ProfileChangeRequest),
	}
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error) {
	for _, u := range m.users {
		if u.Username == identifier || u.Email == identifier {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	user.ID = uint(len(m.users) + 1)
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *models.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error {
	if m.updatePassErr != nil {
		return m.updatePassErr
	}
	m.updatedPass[id] = passwordHash
	m.updatedAtTimes[id] = updatedAt
	if u, ok := m.users[id]; ok {
		u.PasswordHash = passwordHash
		u.UpdatedAt = updatedAt
	}
	return nil
}

func (m *mockUserRepo) FindByIDWithDetails(ctx context.Context, id uint) (*models.User, error) {
	return m.FindByID(ctx, id)
}

func (m *mockUserRepo) FindByIDWithCredentials(ctx context.Context, id uint) (*models.User, error) {
	return m.FindByID(ctx, id)
}

func (m *mockUserRepo) FindByUsernameOrEmailWithCredentials(ctx context.Context, identifier string) (*models.User, error) {
	return m.FindByUsernameOrEmail(ctx, identifier)
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return m.FindByUsernameOrEmail(ctx, email)
}

func (m *mockUserRepo) CreatePasskeyCredential(ctx context.Context, cred *models.WebAuthnCredential) error {
	return nil
}

func (m *mockUserRepo) UpdatePasskeySignCount(ctx context.Context, credID []byte, signCount uint32, backupState bool) error {
	return nil
}

func (m *mockUserRepo) ListPasskeysByUserID(ctx context.Context, userID uint) ([]models.WebAuthnCredential, error) {
	return nil, nil
}

func (m *mockUserRepo) FindByEmailExcludingUser(ctx context.Context, email string, excludeUserID uint) (*models.User, error) {
	for _, u := range m.users {
		if u.Email == email && u.ID != excludeUserID {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockUserRepo) SoftDelete(ctx context.Context, id uint) error {
	if u, ok := m.users[id]; ok {
		u.IsActive = false
		return nil
	}
	return domain.ErrNotFound
}

func (m *mockUserRepo) ListUsers(ctx context.Context, isActive *bool) ([]models.User, error) {
	var list []models.User
	for _, u := range m.users {
		if isActive == nil || u.IsActive == *isActive {
			list = append(list, *u)
		}
	}
	return list, nil
}

func (m *mockUserRepo) CreateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	change.ID = uint(len(m.profileChanges) + 1)
	m.profileChanges[change.ID] = change
	return nil
}

func (m *mockUserRepo) ListProfileChanges(ctx context.Context, userID *uint, status string) ([]models.ProfileChangeRequest, error) {
	var list []models.ProfileChangeRequest
	for _, p := range m.profileChanges {
		if userID != nil && p.UserID != *userID {
			continue
		}
		if status != "" && string(p.Status) != status {
			continue
		}
		list = append(list, *p)
	}
	return list, nil
}

func (m *mockUserRepo) FindProfileChangeByID(ctx context.Context, id uint) (*models.ProfileChangeRequest, error) {
	p, ok := m.profileChanges[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (m *mockUserRepo) UpdateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	m.profileChanges[change.ID] = change
	return nil
}

func (m *mockUserRepo) DeletePasskey(ctx context.Context, id uint, userID *uint) (bool, error) {
	return true, nil
}

func (m *mockUserRepo) UpdatePasskeyName(ctx context.Context, id uint, userID *uint, name string) (bool, error) {
	return true, nil
}

func TestUserService_ApplyApprovedProfileChange(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, auth.DefaultHasher, nil)

	user := &models.User{
		ID:         10,
		Username:   "bob",
		Email:      "bob@example.com",
		Role:       models.RoleUser,
		Name:       "Old Name",
		EmployeeID: "OLD-1",
	}
	repo.users[10] = user

	change := &models.ProfileChangeRequest{
		UserID:     10,
		Name:       "New Name",
		EmployeeID: "NEW-1",
		Department: "DevOps",
	}

	err := svc.ApplyApprovedProfileChange(context.Background(), change)
	if err != nil {
		t.Fatalf("ApplyApprovedProfileChange failed: %v", err)
	}

	if repo.users[10].Name != "New Name" {
		t.Errorf("expected name 'New Name', got %s", repo.users[10].Name)
	}
	if repo.users[10].EmployeeID != "NEW-1" {
		t.Errorf("expected employeeID 'NEW-1', got %s", repo.users[10].EmployeeID)
	}
	if repo.users[10].Department != "DevOps" {
		t.Errorf("expected department 'DevOps', got %s", repo.users[10].Department)
	}
}

func TestUserService_ApplyApprovedProfileChange_WithEmail(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, auth.DefaultHasher, nil)

	user := &models.User{
		ID:       10,
		Username: "alice",
		Email:    "alice.old@example.com",
		Role:     models.RoleUser,
		Name:     "Alice",
	}
	repo.users[10] = user

	change := &models.ProfileChangeRequest{
		UserID: 10,
		Name:   "Alice",
		Email:  "alice.new@example.com",
		Notes:  "Update email to new domain",
	}

	err := svc.ApplyApprovedProfileChange(context.Background(), change)
	if err != nil {
		t.Fatalf("ApplyApprovedProfileChange failed: %v", err)
	}

	if repo.users[10].Email != "alice.new@example.com" {
		t.Errorf("expected email 'alice.new@example.com', got %s", repo.users[10].Email)
	}
}

func TestUserService_ApplyApprovedProfileChange_EmailConflict(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, auth.DefaultHasher, nil)

	user1 := &models.User{
		ID:       10,
		Username: "alice",
		Email:    "alice@example.com",
		Role:     models.RoleUser,
	}
	user2 := &models.User{
		ID:       20,
		Username: "bob",
		Email:    "bob@example.com",
		Role:     models.RoleUser,
	}
	repo.users[10] = user1
	repo.users[20] = user2

	change := &models.ProfileChangeRequest{
		UserID: 10,
		Email:  "bob@example.com", // existing email of user 20
		Notes:  "Change email",
	}

	err := svc.ApplyApprovedProfileChange(context.Background(), change)
	if err == nil {
		t.Fatalf("expected error on email conflict, got nil")
	}
	if !errors.Is(err, domain.ErrEmailConflict) {
		t.Errorf("expected ErrEmailConflict, got %v", err)
	}
}

func TestUserService_CreateUserByAdmin(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, auth.DefaultHasher, nil)

	user := &models.User{
		Username: "newadminuser",
		Email:    "newadminuser@example.com",
		Role:     models.RoleUser,
	}

	pass, err := svc.CreateUserByAdmin(context.Background(), user, "http://localhost/login")
	if err != nil {
		t.Fatalf("CreateUserByAdmin failed: %v", err)
	}

	if len(pass) != 16 {
		t.Fatalf("expected 16 chars generated password, got %d", len(pass))
	}
	if !strings.HasPrefix(user.PasswordHash, "$argon2id$") {
		t.Errorf("expected Argon2id hash, got %s", user.PasswordHash)
	}
	if !auth.VerifyPassword(user.PasswordHash, pass) {
		t.Errorf("generated password failed verification against stored hash")
	}

	// Duplicate username rejection
	dupUser := &models.User{
		Username: "newadminuser",
		Email:    "different@example.com",
	}
	_, err = svc.CreateUserByAdmin(context.Background(), dupUser, "http://localhost/login")
	if !errors.Is(err, domain.ErrUsernameConflict) {
		t.Errorf("expected ErrUsernameConflict, got %v", err)
	}

	// Duplicate email rejection
	dupEmail := &models.User{
		Username: "different_username",
		Email:    "newadminuser@example.com",
	}
	_, err = svc.CreateUserByAdmin(context.Background(), dupEmail, "http://localhost/login")
	if !errors.Is(err, domain.ErrEmailConflict) {
		t.Errorf("expected ErrEmailConflict, got %v", err)
	}
}

func TestUserService_ChangePassword(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, auth.DefaultHasher, nil)

	oldPass := "InitialPass123!@#"
	oldHash, _ := auth.HashPassword(oldPass)

	activeUser := &models.User{
		ID:           1,
		Username:     "activeuser",
		Email:        "active@example.com",
		Name:         "Active User",
		PasswordHash: oldHash,
		IsActive:     true,
	}
	repo.users[1] = activeUser

	disabledUser := &models.User{
		ID:           2,
		Username:     "disableduser",
		Email:        "disabled@example.com",
		PasswordHash: oldHash,
		IsActive:     false,
	}
	repo.users[2] = disabledUser

	t.Run("User not found", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 999, oldPass, "NewPass123!@#")
		if err == nil {
			t.Fatal("expected error for non-existent user")
		}
	})

	t.Run("User disabled", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 2, oldPass, "NewPass123!@#")
		if err == nil {
			t.Fatal("expected error for disabled user")
		}
	})

	t.Run("Wrong old password", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 1, "IncorrectOldPass1!", "NewPass123!@#")
		if err == nil || !strings.Contains(err.Error(), "Old password does not match") {
			t.Fatalf("expected old password mismatch error, got: %v", err)
		}
	})

	t.Run("Same new password as old password", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 1, oldPass, oldPass)
		if err == nil || !strings.Contains(err.Error(), "New password cannot be the same") {
			t.Fatalf("expected same password error, got: %v", err)
		}
	})

	t.Run("Weak new password rejected", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 1, oldPass, "123")
		if err == nil {
			t.Fatal("expected error for weak new password")
		}
	})

	t.Run("Successful password change", func(t *testing.T) {
		newPass := "SuperSecretNewPassword123!@#"
		err := svc.ChangePassword(context.Background(), 1, oldPass, newPass)
		if err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}

		if !auth.VerifyPassword(repo.updatedPass[1], newPass) {
			t.Fatal("new password failed verification against updated hash")
		}
		if auth.VerifyPassword(repo.updatedPass[1], oldPass) {
			t.Fatal("old password unexpectedly verified against updated hash")
		}
		if repo.updatedAtTimes[1].IsZero() {
			t.Fatal("expected updated_at time to be set")
		}
	})

	t.Run("Repo UpdatePassword error", func(t *testing.T) {
		repo.updatePassErr = errors.New("update password db failure")
		defer func() { repo.updatePassErr = nil }()
		err := svc.ChangePassword(context.Background(), 1, "SuperSecretNewPassword123!@#", "AnotherValidNewPass123!@#")
		if err == nil || !strings.Contains(err.Error(), "failed to update password") {
			t.Fatalf("expected update password failure, got: %v", err)
		}
	})
}

func TestUserService_CreateUserByAdmin_RepoError(t *testing.T) {
	repo := newMockUserRepo()
	repo.createErr = errors.New("failed to persist user")
	svc := NewUserService(repo, nil, nil) // tests nil hasher fallback to DefaultHasher

	user := &models.User{
		Username: "dbfailuser",
		Email:    "dbfail@example.com",
	}
	_, err := svc.CreateUserByAdmin(context.Background(), user, "http://localhost/login")
	if err == nil || !strings.Contains(err.Error(), "failed to persist user") {
		t.Fatalf("expected create error, got: %v", err)
	}
}

type mockMasterRepoForUser struct {
	companies   map[uint]*models.Company
	departments map[uint]*models.Department
	sites       map[uint]*models.Site
	divisions   map[uint]*models.Division
}

func newMockMasterRepoForUser() *mockMasterRepoForUser {
	return &mockMasterRepoForUser{
		companies:   make(map[uint]*models.Company),
		departments: make(map[uint]*models.Department),
		sites:       make(map[uint]*models.Site),
		divisions:   make(map[uint]*models.Division),
	}
}

func (m *mockMasterRepoForUser) ListApprovers(ctx context.Context, roleType string, activeStatus *bool) ([]models.Approver, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindApproverByID(ctx context.Context, id uint) (*models.Approver, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) CreateApprover(ctx context.Context, a *models.Approver) error {
	return nil
}
func (m *mockMasterRepoForUser) UpdateApprover(ctx context.Context, a *models.Approver) error {
	return nil
}
func (m *mockMasterRepoForUser) SoftDeleteApprover(ctx context.Context, id uint) error { return nil }

func (m *mockMasterRepoForUser) ListCompanies(ctx context.Context, activeStatus *bool) ([]models.Company, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindCompanyByID(ctx context.Context, id uint) (*models.Company, error) {
	c, ok := m.companies[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}
func (m *mockMasterRepoForUser) FindCompanyByCode(ctx context.Context, code string) (*models.Company, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindActiveCompanyByCodeOrName(ctx context.Context, identifier string) (*models.Company, error) {
	for _, c := range m.companies {
		if (c.Code == identifier || c.Name == identifier) && c.IsActive {
			return c, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockMasterRepoForUser) CreateCompany(ctx context.Context, c *models.Company) error {
	return nil
}
func (m *mockMasterRepoForUser) UpdateCompany(ctx context.Context, c *models.Company) error {
	return nil
}
func (m *mockMasterRepoForUser) SoftDeleteCompany(ctx context.Context, id uint) error { return nil }

func (m *mockMasterRepoForUser) ListSites(ctx context.Context, activeStatus *bool) ([]models.Site, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindSiteByID(ctx context.Context, id uint) (*models.Site, error) {
	s, ok := m.sites[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}
func (m *mockMasterRepoForUser) FindSiteByCode(ctx context.Context, code string) (*models.Site, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindActiveSiteByCodeOrName(ctx context.Context, identifier string) (*models.Site, error) {
	for _, s := range m.sites {
		if (s.Code == identifier || s.Name == identifier) && s.IsActive {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockMasterRepoForUser) CreateSite(ctx context.Context, s *models.Site) error { return nil }
func (m *mockMasterRepoForUser) UpdateSite(ctx context.Context, s *models.Site) error { return nil }
func (m *mockMasterRepoForUser) SoftDeleteSite(ctx context.Context, id uint) error    { return nil }

func (m *mockMasterRepoForUser) ListDivisions(ctx context.Context, activeStatus *bool) ([]models.Division, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindDivisionByID(ctx context.Context, id uint) (*models.Division, error) {
	d, ok := m.divisions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return d, nil
}
func (m *mockMasterRepoForUser) FindDivisionByCode(ctx context.Context, code string) (*models.Division, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindActiveDivisionByCodeOrName(ctx context.Context, identifier string) (*models.Division, error) {
	for _, d := range m.divisions {
		if (d.Code == identifier || d.Name == identifier) && d.IsActive {
			return d, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockMasterRepoForUser) CreateDivision(ctx context.Context, d *models.Division) error {
	return nil
}
func (m *mockMasterRepoForUser) UpdateDivision(ctx context.Context, d *models.Division) error {
	return nil
}
func (m *mockMasterRepoForUser) SoftDeleteDivision(ctx context.Context, id uint) error { return nil }

func (m *mockMasterRepoForUser) ListDepartments(ctx context.Context, divisionID *uint, divisionName string, activeStatus *bool) ([]models.Department, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindDepartmentByID(ctx context.Context, id uint) (*models.Department, error) {
	d, ok := m.departments[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return d, nil
}
func (m *mockMasterRepoForUser) FindDepartmentByName(ctx context.Context, name string) (*models.Department, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindActiveDepartmentByCodeOrName(ctx context.Context, identifier string) (*models.Department, error) {
	for _, d := range m.departments {
		if (d.Code == identifier || d.Name == identifier) && d.IsActive {
			return d, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockMasterRepoForUser) CreateDepartment(ctx context.Context, d *models.Department) error {
	return nil
}
func (m *mockMasterRepoForUser) UpdateDepartment(ctx context.Context, d *models.Department) error {
	return nil
}
func (m *mockMasterRepoForUser) SoftDeleteDepartment(ctx context.Context, id uint) error { return nil }

func (m *mockMasterRepoForUser) ListProjects(ctx context.Context, activeOnly bool) ([]models.Project, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) FindProjectByID(ctx context.Context, id uint) (*models.Project, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) ListActivityStatuses(ctx context.Context) ([]models.ActivityStatus, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) ListHolidaysByMonth(ctx context.Context, year, month int) ([]models.Holiday, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) ListAllHolidays(ctx context.Context, year *int) ([]models.Holiday, error) {
	return nil, nil
}
func (m *mockMasterRepoForUser) UpsertHolidays(ctx context.Context, holidays []models.Holiday) error {
	return nil
}

func TestUserService_ListUsers(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, nil, nil)
	ctx := context.Background()

	u1 := &models.User{ID: 1, Username: "user1", IsActive: true}
	u2 := &models.User{ID: 2, Username: "user2", IsActive: false}
	repo.users[1] = u1
	repo.users[2] = u2

	// List all
	all, err := svc.ListUsers(ctx, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("expected 2 users, got %d (err: %v)", len(all), err)
	}

	// List active only
	trueVal := true
	active, err := svc.ListUsers(ctx, &trueVal)
	if err != nil || len(active) != 1 {
		t.Fatalf("expected 1 active user, got %d (err: %v)", len(active), err)
	}
}

func TestUserService_AdminCreateUser(t *testing.T) {
	repo := newMockUserRepo()
	master := newMockMasterRepoForUser()
	svc := NewUserService(repo, nil, nil, master)
	ctx := context.Background()

	// Seed master data
	comp := &models.Company{ID: 1, Code: "MII", Name: "PT MII", IsActive: true}
	master.companies[1] = comp

	dept := &models.Department{ID: 10, Code: "ENG", Name: "Engineering", Division: "Tech", DivisionID: uintPtr(20), IsActive: true}
	master.departments[10] = dept

	site := &models.Site{ID: 30, Code: "JKT", Name: "Jakarta", IsActive: true}
	master.sites[30] = site

	div := &models.Division{ID: 20, Code: "TECH", Name: "Tech", IsActive: true}
	master.divisions[20] = div

	t.Run("Create Admin User clears company", func(t *testing.T) {
		req := &request.CreateUserRequest{
			Username:  "admin_svc_test",
			Email:     "admin_svc@example.com",
			Role:      models.RoleAdmin,
			Name:      "Super Admin",
			Company:   "MII",
			CompanyID: uintPtr(1),
		}
		u, pass, err := svc.AdminCreateUser(ctx, req, "http://localhost")
		if err != nil {
			t.Fatalf("AdminCreateUser failed: %v", err)
		}
		if pass == "" || u.Role != models.RoleAdmin || u.Company != "" || u.CompanyID != nil {
			t.Errorf("unexpected admin user attributes: %+v", u)
		}
	})

	t.Run("Create Regular User with ID resolution", func(t *testing.T) {
		req := &request.CreateUserRequest{
			Username:     "regular_svc_test",
			Email:        "regular_svc@example.com",
			Role:         models.RoleUser,
			Name:         "Regular User",
			CompanyID:    uintPtr(1),
			DepartmentID: uintPtr(10),
			SiteID:       uintPtr(30),
			DivisionID:   uintPtr(20),
		}
		u, _, err := svc.AdminCreateUser(ctx, req, "http://localhost")
		if err != nil {
			t.Fatalf("AdminCreateUser failed: %v", err)
		}
		if u.Company != "PT MII" || u.Department != "Engineering" || u.Site != "Jakarta" || u.Division != "Tech" {
			t.Errorf("master data resolution mismatch: %+v", u)
		}
	})

	t.Run("Create Regular User with CodeOrName resolution", func(t *testing.T) {
		req := &request.CreateUserRequest{
			Username:   "regular_name_test",
			Email:      "regular_name@example.com",
			Role:       models.RoleUser,
			Name:       "Regular Name",
			Company:    "MII",
			Department: "ENG",
			Site:       "JKT",
			Division:   "TECH",
		}
		u, _, err := svc.AdminCreateUser(ctx, req, "http://localhost")
		if err != nil {
			t.Fatalf("AdminCreateUser failed: %v", err)
		}
		if u.Company != "PT MII" || u.Department != "Engineering" || u.Site != "Jakarta" || u.Division != "Tech" {
			t.Errorf("master data code resolution mismatch: %+v", u)
		}
	})

	t.Run("Invalid company ID returns error", func(t *testing.T) {
		req := &request.CreateUserRequest{
			Username:  "invalid_comp",
			Email:     "invalid_comp@example.com",
			Role:      models.RoleUser,
			CompanyID: uintPtr(999),
		}
		_, _, err := svc.AdminCreateUser(ctx, req, "http://localhost")
		if err == nil {
			t.Fatal("expected error for invalid company ID")
		}
	})

	t.Run("Invalid department ID returns error", func(t *testing.T) {
		req := &request.CreateUserRequest{
			Username:     "invalid_dept",
			Email:        "invalid_dept@example.com",
			Role:         models.RoleUser,
			DepartmentID: uintPtr(999),
		}
		_, _, err := svc.AdminCreateUser(ctx, req, "http://localhost")
		if err == nil {
			t.Fatal("expected error for invalid department ID")
		}
	})

	t.Run("Invalid site ID returns error", func(t *testing.T) {
		req := &request.CreateUserRequest{
			Username: "invalid_site",
			Email:    "invalid_site@example.com",
			Role:     models.RoleUser,
			SiteID:   uintPtr(999),
		}
		_, _, err := svc.AdminCreateUser(ctx, req, "http://localhost")
		if err == nil {
			t.Fatal("expected error for invalid site ID")
		}
	})

	t.Run("Invalid division ID returns error", func(t *testing.T) {
		req := &request.CreateUserRequest{
			Username:   "invalid_div",
			Email:      "invalid_div@example.com",
			Role:       models.RoleUser,
			DivisionID: uintPtr(999),
		}
		_, _, err := svc.AdminCreateUser(ctx, req, "http://localhost")
		if err == nil {
			t.Fatal("expected error for invalid division ID")
		}
	})
}

func TestUserService_AdminUpdateUser(t *testing.T) {
	repo := newMockUserRepo()
	master := newMockMasterRepoForUser()
	svc := NewUserService(repo, nil, nil, master)
	ctx := context.Background()

	adminUser := &models.User{
		ID:       1,
		Username: "admin_1",
		Email:    "admin_1@example.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	regularUser := &models.User{
		ID:       2,
		Username: "regular_2",
		Email:    "regular_2@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	repo.users[1] = adminUser
	repo.users[2] = regularUser

	master.companies[1] = &models.Company{ID: 1, Code: "MII", Name: "PT MII", IsActive: true}
	master.departments[10] = &models.Department{ID: 10, Code: "ENG", Name: "Engineering", Division: "Tech", IsActive: true}
	master.sites[30] = &models.Site{ID: 30, Code: "JKT", Name: "Jakarta", IsActive: true}
	master.divisions[20] = &models.Division{ID: 20, Code: "TECH", Name: "Tech", IsActive: true}

	t.Run("Self-deactivation blocked", func(t *testing.T) {
		falseVal := false
		err := svc.AdminUpdateUser(ctx, 1, 1, &request.UpdateUserRequest{IsActive: &falseVal})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("Self-demotion blocked", func(t *testing.T) {
		roleUser := models.RoleUser
		err := svc.AdminUpdateUser(ctx, 1, 1, &request.UpdateUserRequest{Role: &roleUser})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("User not found", func(t *testing.T) {
		err := svc.AdminUpdateUser(ctx, 999, 1, &request.UpdateUserRequest{})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Email conflict", func(t *testing.T) {
		dupEmail := "admin_1@example.com"
		err := svc.AdminUpdateUser(ctx, 2, 1, &request.UpdateUserRequest{Email: &dupEmail})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("Successful update with master data fields", func(t *testing.T) {
		newName := "Updated Regular User"
		newEmail := "regular_updated@example.com"
		newBni := "BNI-001"
		newEmp := "EMP-001"
		newDiv := "Tech"
		newSite := "Jakarta"
		roleAdmin := models.RoleAdmin

		req := &request.UpdateUserRequest{
			Name:         &newName,
			Email:        &newEmail,
			BniID:        &newBni,
			EmployeeID:   &newEmp,
			Division:     &newDiv,
			Site:         &newSite,
			CompanyID:    uintPtr(1),
			DepartmentID: uintPtr(10),
			SiteID:       uintPtr(30),
			DivisionID:   uintPtr(20),
			Role:         &roleAdmin,
		}
		err := svc.AdminUpdateUser(ctx, 2, 1, req)
		if err != nil {
			t.Fatalf("AdminUpdateUser failed: %v", err)
		}
		if repo.users[2].Name != newName || repo.users[2].Email != newEmail {
			t.Errorf("user update verification failed: %+v", repo.users[2])
		}
		// Since role was updated to Admin, company must be cleared
		if repo.users[2].Company != "" || repo.users[2].CompanyID != nil {
			t.Errorf("expected admin company to be empty, got %+v", repo.users[2].Company)
		}
	})

	t.Run("Clearing IDs when set to 0", func(t *testing.T) {
		zeroVal := uint(0)
		emptyStr := ""
		req := &request.UpdateUserRequest{
			DepartmentID: &zeroVal,
			CompanyID:    &zeroVal,
			SiteID:       &zeroVal,
			DivisionID:   &zeroVal,
			Company:      &emptyStr,
		}
		err := svc.AdminUpdateUser(ctx, 2, 1, req)
		if err != nil {
			t.Fatalf("AdminUpdateUser zero IDs failed: %v", err)
		}
		if repo.users[2].DepartmentID != nil || repo.users[2].CompanyID != nil || repo.users[2].SiteID != nil || repo.users[2].DivisionID != nil {
			t.Errorf("expected nil IDs after zeroing: %+v", repo.users[2])
		}
	})
}

func TestUserService_AdminDeleteUser(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, nil, nil)
	ctx := context.Background()

	u := &models.User{ID: 10, Username: "to_delete", IsActive: true}
	repo.users[10] = u

	t.Run("Self delete blocked", func(t *testing.T) {
		err := svc.AdminDeleteUser(ctx, 10, 10)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("Delete not found user", func(t *testing.T) {
		err := svc.AdminDeleteUser(ctx, 999, 10)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Successful delete", func(t *testing.T) {
		err := svc.AdminDeleteUser(ctx, 10, 1)
		if err != nil {
			t.Fatalf("AdminDeleteUser failed: %v", err)
		}
		if repo.users[10].IsActive {
			t.Error("expected user to be inactive after delete")
		}
	})
}

func TestUserService_ProfileChanges_Workflow(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, nil, nil)
	ctx := context.Background()

	user1 := &models.User{ID: 1, Username: "user1", Email: "user1@example.com", Name: "User One", IsActive: true}
	user2 := &models.User{ID: 2, Username: "user2", Email: "user2@example.com", Name: "User Two", IsActive: true}
	repo.users[1] = user1
	repo.users[2] = user2

	t.Run("SubmitProfileChange with duplicate email fails", func(t *testing.T) {
		req := &request.ProfileChangeRequestDTO{
			Email: "user2@example.com", // belongs to user 2
			Name:  "User One New",
		}
		err := svc.SubmitProfileChange(ctx, 1, req)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for duplicate email, got %v", err)
		}
	})

	t.Run("SubmitProfileChange success", func(t *testing.T) {
		req := &request.ProfileChangeRequestDTO{
			Email: "user1.new@example.com",
			Name:  "User One New Name",
			Notes: "Domain change",
		}
		err := svc.SubmitProfileChange(ctx, 1, req)
		if err != nil {
			t.Fatalf("SubmitProfileChange failed: %v", err)
		}
		if len(repo.profileChanges) != 1 {
			t.Fatalf("expected 1 profile change, got %d", len(repo.profileChanges))
		}
	})

	t.Run("MyProfileChanges and ListProfileChanges", func(t *testing.T) {
		myChanges, err := svc.MyProfileChanges(ctx, 1)
		if err != nil || len(myChanges) != 1 {
			t.Fatalf("expected 1 my change, got %d (err: %v)", len(myChanges), err)
		}

		allPending, err := svc.ListProfileChanges(ctx, string(models.ProfilePending))
		if err != nil || len(allPending) != 1 {
			t.Fatalf("expected 1 pending change, got %d (err: %v)", len(allPending), err)
		}
	})

	t.Run("ReviewProfileChange invalid action", func(t *testing.T) {
		err := svc.ReviewProfileChange(ctx, 1, 99, "invalid_action")
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("ReviewProfileChange not found", func(t *testing.T) {
		err := svc.ReviewProfileChange(ctx, 999, 99, "approve")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("ReviewProfileChange reject action", func(t *testing.T) {
		// Create another request to test reject
		req := &request.ProfileChangeRequestDTO{Name: "To Reject"}
		_ = svc.SubmitProfileChange(ctx, 2, req)

		err := svc.ReviewProfileChange(ctx, 2, 99, "reject")
		if err != nil {
			t.Fatalf("ReviewProfileChange reject failed: %v", err)
		}
		if repo.profileChanges[2].Status != "rejected" {
			t.Errorf("expected rejected status, got %s", repo.profileChanges[2].Status)
		}

		// Re-review should fail with ErrConflict
		err = svc.ReviewProfileChange(ctx, 2, 99, "approve")
		if !errors.Is(err, domain.ErrConflict) {
			t.Errorf("expected ErrConflict on re-review, got %v", err)
		}
	})

	t.Run("ReviewProfileChange approve action", func(t *testing.T) {
		err := svc.ReviewProfileChange(ctx, 1, 99, "approve")
		if err != nil {
			t.Fatalf("ReviewProfileChange approve failed: %v", err)
		}
		if repo.profileChanges[1].Status != models.ProfileApproved {
			t.Errorf("expected approved status, got %s", repo.profileChanges[1].Status)
		}
		if repo.users[1].Name != "User One New Name" || repo.users[1].Email != "user1.new@example.com" {
			t.Errorf("expected user fields to be updated, got %+v", repo.users[1])
		}
	})
}

func uintPtr(u uint) *uint {
	return &u
}
