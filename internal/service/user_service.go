package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
)

const (
	errUserNotFound           = "user not found"
	errCannotModifyAnonymized = "cannot modify an anonymized account"
	errCompanyDeptNotFound    = "Company or Department not found or inactive"
	errSiteNotFound           = "Site not found or inactive"
	errDivisionNotFound       = "Division not found or inactive"
)

// UserService defines user account management and password credential operations.
type UserService interface {
	CreateUserByAdmin(ctx context.Context, user *models.User, loginURL string) (initialPassword string, err error)
	ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error
	ApplyApprovedProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error

	// Admin operations
	ListUsers(ctx context.Context, isActive *bool) ([]models.User, error)
	AdminCreateUser(ctx context.Context, req *request.CreateUserRequest, loginBaseURL string) (*models.User, string, error)
	AdminUpdateUser(ctx context.Context, id uint, callerID uint, req *request.UpdateUserRequest) error
	AdminDeleteUser(ctx context.Context, id uint, callerID uint) error
	AdminAnonymizeUser(ctx context.Context, id uint, callerID uint) error

	// Profile Change Requests
	SubmitProfileChange(ctx context.Context, userID uint, req *request.ProfileChangeRequestDTO) error
	MyProfileChanges(ctx context.Context, userID uint) ([]models.ProfileChangeRequest, error)
	ListProfileChanges(ctx context.Context, status string) ([]models.ProfileChangeRequest, error)
	ReviewProfileChange(ctx context.Context, id uint, reviewerID uint, action string) error
}

type userService struct {
	repo   repository.UserRepository
	hasher auth.PasswordHasher
	mailer *mailer.Mailer
	master repository.MasterRepository
}

// NewUserService constructs an instance of UserService.
func NewUserService(repo repository.UserRepository, hasher auth.PasswordHasher, m *mailer.Mailer, master ...repository.MasterRepository) UserService {
	if hasher == nil {
		hasher = auth.DefaultHasher
	}
	var mr repository.MasterRepository
	if len(master) > 0 {
		mr = master[0]
	}
	return &userService{
		repo:   repo,
		hasher: hasher,
		mailer: m,
		master: mr,
	}
}

func (s *userService) ListUsers(ctx context.Context, isActive *bool) ([]models.User, error) {
	return s.repo.ListUsers(ctx, isActive)
}

func (s *userService) resolveCreateCompany(ctx context.Context, req *request.CreateUserRequest, user *models.User) error {
	if user.Role == models.RoleAdmin {
		user.Company = ""
		user.CompanyID = nil
		return nil
	}
	if req.CompanyID != nil && *req.CompanyID != 0 {
		comp, err := s.master.FindCompanyByID(ctx, *req.CompanyID)
		if err != nil || comp == nil || !comp.IsActive {
			return domain.NewUserError(domain.ErrInvalidInput, errCompanyDeptNotFound)
		}
		user.CompanyID = &comp.ID
		user.Company = comp.Name
		return nil
	}
	if req.Company != "" {
		comp, err := s.master.FindActiveCompanyByCodeOrName(ctx, req.Company)
		if err != nil || comp == nil {
			return domain.NewUserError(domain.ErrInvalidInput, errCompanyDeptNotFound)
		}
		user.CompanyID = &comp.ID
		user.Company = comp.Name
	}
	return nil
}

func (s *userService) resolveCreateDepartment(ctx context.Context, req *request.CreateUserRequest, user *models.User) error {
	if req.DepartmentID != nil && *req.DepartmentID != 0 {
		dept, err := s.master.FindDepartmentByID(ctx, *req.DepartmentID)
		if err != nil || dept == nil || !dept.IsActive {
			return domain.NewUserError(domain.ErrInvalidInput, errCompanyDeptNotFound)
		}
		user.DepartmentID = &dept.ID
		user.Department = dept.Name
		if user.Division == "" {
			user.Division = dept.Division
			user.DivisionID = dept.DivisionID
		}
		return nil
	}
	if req.Department != "" {
		dept, err := s.master.FindActiveDepartmentByCodeOrName(ctx, req.Department)
		if err == nil && dept != nil {
			user.DepartmentID = &dept.ID
			user.Department = dept.Name
			if user.Division == "" {
				user.Division = dept.Division
				user.DivisionID = dept.DivisionID
			}
		}
	}
	return nil
}

func (s *userService) resolveCreateSite(ctx context.Context, req *request.CreateUserRequest, user *models.User) error {
	if req.SiteID != nil && *req.SiteID != 0 {
		site, err := s.master.FindSiteByID(ctx, *req.SiteID)
		if err != nil || site == nil || !site.IsActive {
			return domain.NewUserError(domain.ErrInvalidInput, errSiteNotFound)
		}
		user.SiteID = &site.ID
		user.Site = site.Name
		return nil
	}
	if req.Site != "" {
		site, err := s.master.FindActiveSiteByCodeOrName(ctx, req.Site)
		if err == nil && site != nil {
			user.SiteID = &site.ID
			user.Site = site.Name
		}
	}
	return nil
}

func (s *userService) resolveCreateDivision(ctx context.Context, req *request.CreateUserRequest, user *models.User) error {
	if req.DivisionID != nil && *req.DivisionID != 0 {
		div, err := s.master.FindDivisionByID(ctx, *req.DivisionID)
		if err != nil || div == nil || !div.IsActive {
			return domain.NewUserError(domain.ErrInvalidInput, errDivisionNotFound)
		}
		user.DivisionID = &div.ID
		user.Division = div.Name
		return nil
	}
	if req.Division != "" {
		div, err := s.master.FindActiveDivisionByCodeOrName(ctx, req.Division)
		if err == nil && div != nil {
			user.DivisionID = &div.ID
			user.Division = div.Name
		}
	}
	return nil
}

func (s *userService) resolveCreateMasterData(ctx context.Context, req *request.CreateUserRequest, user *models.User) error {
	if s.master == nil {
		if user.Role == models.RoleAdmin {
			user.Company = ""
			user.CompanyID = nil
		}
		return nil
	}
	if err := s.resolveCreateCompany(ctx, req, user); err != nil {
		return err
	}
	if err := s.resolveCreateDepartment(ctx, req, user); err != nil {
		return err
	}
	if err := s.resolveCreateSite(ctx, req, user); err != nil {
		return err
	}
	return s.resolveCreateDivision(ctx, req, user)
}

func (s *userService) AdminCreateUser(ctx context.Context, req *request.CreateUserRequest, loginBaseURL string) (*models.User, string, error) {
	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		Role:         req.Role,
		Name:         req.Name,
		BniID:        req.BniID,
		EmployeeID:   req.EmployeeID,
		Division:     req.Division,
		DivisionID:   req.DivisionID,
		Department:   req.Department,
		DepartmentID: req.DepartmentID,
		Site:         req.Site,
		SiteID:       req.SiteID,
		Company:      req.Company,
		CompanyID:    req.CompanyID,
		IsActive:     true,
	}

	if err := s.resolveCreateMasterData(ctx, req, &user); err != nil {
		return nil, "", err
	}

	plainPass, err := s.CreateUserByAdmin(ctx, &user, loginBaseURL+"/login")
	if err != nil {
		return nil, "", err
	}
	return &user, plainPass, nil
}

func validateSelfUpdate(id, callerID uint, req *request.UpdateUserRequest) error {
	if id != callerID {
		return nil
	}
	if req.IsActive != nil && !*req.IsActive {
		return domain.NewUserError(domain.ErrForbidden, "you cannot deactivate your own account")
	}
	if req.Role != nil && *req.Role != models.RoleAdmin {
		return domain.NewUserError(domain.ErrForbidden, "you cannot remove your own admin role")
	}
	return nil
}

func validateUpdatableUser(user *models.User, err error) error {
	if err != nil || user == nil {
		return domain.NewUserError(domain.ErrNotFound, errUserNotFound)
	}
	if user.IsAnonymized || strings.HasPrefix(user.Username, "deleted_user_") || strings.HasPrefix(user.Name, "ANONYMIZED_USER_") {
		return domain.NewUserError(domain.ErrForbidden, errCannotModifyAnonymized)
	}
	return nil
}

func applyBasicUserUpdates(req *request.UpdateUserRequest, user *models.User) {
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.BniID != nil {
		user.BniID = *req.BniID
	}
	if req.EmployeeID != nil {
		user.EmployeeID = *req.EmployeeID
	}
	if req.Division != nil {
		user.Division = *req.Division
	}
	if req.Site != nil {
		user.Site = *req.Site
	}
}

func (s *userService) applyEmailUpdate(ctx context.Context, newEmail *string, user *models.User) error {
	if newEmail == nil {
		return nil
	}
	email := strings.TrimSpace(*newEmail)
	if email == "" || email == user.Email {
		return nil
	}
	existing, err := s.repo.FindByEmailExcludingUser(ctx, email, user.ID)
	if err == nil && existing != nil {
		return domain.NewUserError(domain.ErrInvalidInput, "email is already registered")
	}
	user.Email = email
	return nil
}

func (s *userService) findDepartmentByIDForUpdate(ctx context.Context, id uint) (*models.Department, error) {
	dept, err := s.master.FindDepartmentByID(ctx, id)
	if err != nil || dept == nil || !dept.IsActive {
		return nil, domain.NewUserError(domain.ErrInvalidInput, errCompanyDeptNotFound)
	}
	return dept, nil
}

func (s *userService) findCompanyByIDForUpdate(ctx context.Context, id uint) (*models.Company, error) {
	comp, err := s.master.FindCompanyByID(ctx, id)
	if err != nil || comp == nil || !comp.IsActive {
		return nil, domain.NewUserError(domain.ErrInvalidInput, errCompanyDeptNotFound)
	}
	return comp, nil
}

func (s *userService) findCompanyByNameForUpdate(ctx context.Context, name string) (*models.Company, error) {
	comp, err := s.master.FindActiveCompanyByCodeOrName(ctx, name)
	if err != nil || comp == nil {
		return nil, domain.NewUserError(domain.ErrInvalidInput, errCompanyDeptNotFound)
	}
	return comp, nil
}

func (s *userService) findSiteByIDForUpdate(ctx context.Context, id uint) (*models.Site, error) {
	site, err := s.master.FindSiteByID(ctx, id)
	if err != nil || site == nil || !site.IsActive {
		return nil, domain.NewUserError(domain.ErrInvalidInput, errSiteNotFound)
	}
	return site, nil
}

func (s *userService) findDivisionByIDForUpdate(ctx context.Context, id uint) (*models.Division, error) {
	div, err := s.master.FindDivisionByID(ctx, id)
	if err != nil || div == nil || !div.IsActive {
		return nil, domain.NewUserError(domain.ErrInvalidInput, errDivisionNotFound)
	}
	return div, nil
}

func (s *userService) applyDepartmentUpdate(ctx context.Context, req *request.UpdateUserRequest, user *models.User) error {
	if req.DepartmentID != nil {
		if *req.DepartmentID != 0 {
			dept, err := s.findDepartmentByIDForUpdate(ctx, *req.DepartmentID)
			if err != nil {
				return err
			}
			user.DepartmentID = &dept.ID
			user.Department = dept.Name
			if user.Division == "" {
				user.Division = dept.Division
			}
		} else {
			user.DepartmentID = nil
		}
	} else if req.Department != nil {
		user.Department = *req.Department
	}
	return nil
}

func (s *userService) applyCompanyUpdate(ctx context.Context, req *request.UpdateUserRequest, user *models.User) error {
	if req.CompanyID != nil {
		if *req.CompanyID == 0 {
			user.CompanyID = nil
			user.Company = ""
			return nil
		}
		comp, err := s.findCompanyByIDForUpdate(ctx, *req.CompanyID)
		if err != nil {
			return err
		}
		user.CompanyID = &comp.ID
		user.Company = comp.Name
		return nil
	}

	if req.Company != nil {
		if *req.Company == "" {
			user.CompanyID = nil
			user.Company = ""
			return nil
		}
		comp, err := s.findCompanyByNameForUpdate(ctx, *req.Company)
		if err != nil {
			return err
		}
		user.CompanyID = &comp.ID
		user.Company = comp.Name
	}
	return nil
}

func (s *userService) applySiteUpdate(ctx context.Context, req *request.UpdateUserRequest, user *models.User) error {
	if req.SiteID != nil {
		if *req.SiteID != 0 {
			site, err := s.findSiteByIDForUpdate(ctx, *req.SiteID)
			if err != nil {
				return err
			}
			user.SiteID = &site.ID
			user.Site = site.Name
		} else {
			user.SiteID = nil
			user.Site = ""
		}
	} else if req.Site != nil && *req.Site != "" {
		site, err := s.master.FindActiveSiteByCodeOrName(ctx, *req.Site)
		if err == nil && site != nil {
			user.SiteID = &site.ID
			user.Site = site.Name
		} else {
			user.SiteID = nil
		}
	}
	return nil
}

func (s *userService) applyDivisionUpdate(ctx context.Context, req *request.UpdateUserRequest, user *models.User) error {
	if req.DivisionID != nil {
		if *req.DivisionID != 0 {
			div, err := s.findDivisionByIDForUpdate(ctx, *req.DivisionID)
			if err != nil {
				return err
			}
			user.DivisionID = &div.ID
			user.Division = div.Name
		} else {
			user.DivisionID = nil
			user.Division = ""
		}
	} else if req.Division != nil && *req.Division != "" {
		div, err := s.master.FindActiveDivisionByCodeOrName(ctx, *req.Division)
		if err == nil && div != nil {
			user.DivisionID = &div.ID
			user.Division = div.Name
		} else {
			user.DivisionID = nil
		}
	}
	return nil
}

func (s *userService) applyMasterDataUpdates(ctx context.Context, req *request.UpdateUserRequest, user *models.User) error {
	if err := s.applyDepartmentUpdate(ctx, req, user); err != nil {
		return err
	}
	if err := s.applyCompanyUpdate(ctx, req, user); err != nil {
		return err
	}
	if err := s.applySiteUpdate(ctx, req, user); err != nil {
		return err
	}
	return s.applyDivisionUpdate(ctx, req, user)
}

func (s *userService) AdminUpdateUser(ctx context.Context, id uint, callerID uint, req *request.UpdateUserRequest) error {
	if err := validateSelfUpdate(id, callerID, req); err != nil {
		return err
	}

	user, err := s.repo.FindByID(ctx, id)
	if err := validateUpdatableUser(user, err); err != nil {
		return err
	}

	applyBasicUserUpdates(req, user)
	if err := s.applyEmailUpdate(ctx, req.Email, user); err != nil {
		return err
	}

	if s.master != nil {
		if err := s.applyMasterDataUpdates(ctx, req, user); err != nil {
			return err
		}
	}

	if user.Role == models.RoleAdmin {
		user.Company = ""
		user.CompanyID = nil
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (s *userService) AdminDeleteUser(ctx context.Context, id uint, callerID uint) error {
	if id == callerID {
		return domain.NewUserError(domain.ErrForbidden, "you cannot deactivate your own account")
	}
	user, err := s.repo.FindByID(ctx, id)
	if err != nil || user == nil {
		return domain.NewUserError(domain.ErrNotFound, errUserNotFound)
	}
	if user.IsAnonymized || strings.HasPrefix(user.Username, "deleted_user_") || strings.HasPrefix(user.Name, "ANONYMIZED_USER_") {
		return domain.NewUserError(domain.ErrForbidden, "cannot deactivate an anonymized account")
	}
	return s.repo.SoftDelete(ctx, id)
}

func (s *userService) AdminAnonymizeUser(ctx context.Context, id uint, callerID uint) error {
	if id == callerID {
		return domain.NewUserError(domain.ErrForbidden, "you cannot anonymize your own account")
	}
	user, err := s.repo.FindByID(ctx, id)
	if err != nil || user == nil {
		return domain.NewUserError(domain.ErrNotFound, errUserNotFound)
	}
	if user.IsAnonymized || strings.HasPrefix(user.Username, "deleted_user_") || strings.HasPrefix(user.Name, "ANONYMIZED_USER_") {
		return domain.NewUserError(domain.ErrForbidden, "user is already anonymized")
	}
	if err := s.repo.Anonymize(ctx, id); err != nil {
		return fmt.Errorf("failed to anonymize user data: %w", err)
	}
	return s.repo.SoftDelete(ctx, id)
}

func (s *userService) SubmitProfileChange(ctx context.Context, userID uint, req *request.ProfileChangeRequestDTO) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return domain.NewUserError(domain.ErrNotFound, errUserNotFound)
	}
	if user.IsAnonymized || strings.HasPrefix(user.Username, "deleted_user_") || strings.HasPrefix(user.Name, "ANONYMIZED_USER_") {
		return domain.NewUserError(domain.ErrForbidden, errCannotModifyAnonymized)
	}

	if req.Email != "" {
		existing, err := s.repo.FindByEmailExcludingUser(ctx, req.Email, userID)
		if err == nil && existing != nil {
			return domain.NewUserError(domain.ErrInvalidInput, "email is already registered by another user")
		}
	}

	change := models.ProfileChangeRequest{
		UserID:       userID,
		Status:       models.ProfilePending,
		Name:         req.Name,
		Email:        req.Email,
		BniID:        req.BniID,
		EmployeeID:   req.EmployeeID,
		Division:     req.Division,
		DivisionID:   req.DivisionID,
		Department:   req.Department,
		DepartmentID: req.DepartmentID,
		Site:         req.Site,
		SiteID:       req.SiteID,
		CompanyID:    req.CompanyID,
		Notes:        req.Notes,
	}

	if err := s.repo.CreateProfileChange(ctx, &change); err != nil {
		return fmt.Errorf("failed to create profile change request: %w", err)
	}
	return nil
}

func (s *userService) MyProfileChanges(ctx context.Context, userID uint) ([]models.ProfileChangeRequest, error) {
	return s.repo.ListProfileChanges(ctx, &userID, "")
}

func (s *userService) ListProfileChanges(ctx context.Context, status string) ([]models.ProfileChangeRequest, error) {
	return s.repo.ListProfileChanges(ctx, nil, status)
}

func (s *userService) ReviewProfileChange(ctx context.Context, id uint, reviewerID uint, action string) error {
	if action != "approve" && action != "reject" {
		return domain.NewUserError(domain.ErrInvalidInput, "invalid action: must be 'approve' or 'reject'")
	}

	change, err := s.repo.FindProfileChangeByID(ctx, id)
	if err != nil || change == nil {
		return domain.NewUserError(domain.ErrNotFound, "request not found")
	}
	if change.Status != models.ProfilePending {
		return domain.NewUserError(domain.ErrConflict, "request already reviewed")
	}

	now := time.Now()
	if action == "approve" {
		if err := s.ApplyApprovedProfileChange(ctx, change); err != nil {
			return err
		}
		change.Status = models.ProfileApproved
	} else {
		change.Status = "rejected"
	}
	change.ReviewedBy = &reviewerID
	change.ReviewedAt = &now

	return s.repo.UpdateProfileChange(ctx, change)
}

// CreateUserByAdmin generates a cryptographically-secure random initial password,
// hashes it with Argon2id, persists the user record, and dispatches a welcome notification.
func (s *userService) CreateUserByAdmin(ctx context.Context, user *models.User, loginURL string) (string, error) {
	if existing, err := s.repo.FindByUsernameOrEmail(ctx, user.Username); err == nil && existing != nil {
		return "", domain.ErrUsernameConflict
	}
	if existing, err := s.repo.FindByUsernameOrEmail(ctx, user.Email); err == nil && existing != nil {
		return "", domain.ErrEmailConflict
	}

	plainPass, err := auth.GenerateSecurePassword(auth.GeneratedPasswordLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure password: %w", err)
	}

	hash, err := s.hasher.Hash(plainPass)
	if err != nil {
		return "", fmt.Errorf("could not hash password: %w", err)
	}
	user.PasswordHash = hash
	user.IsActive = true

	if err := s.repo.Create(ctx, user); err != nil {
		return "", err
	}

	if s.mailer != nil && user.Email != "" {
		go func(toEmail, username, pass, link string) {
			if err := s.mailer.SendAccountWelcomeEmail(toEmail, username, pass, link); err != nil {
				slog.Error("failed to send welcome email", "error", err, "email", toEmail)
			}
		}(user.Email, user.Username, plainPass, loginURL)
	}

	return plainPass, nil
}

// ChangePassword verifies current credentials, detects weak passwords, confirms difference,
// hashes with Argon2id, updates the timestamp, and sends a security notice email.
func (s *userService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return domain.NewUserError(domain.ErrNotFound, "User not found")
	}
	if !user.IsActive {
		return domain.NewUserError(domain.ErrAccountDisabled, "Account is disabled")
	}

	if user.PasswordHash == "" || !s.hasher.Verify(user.PasswordHash, oldPassword) {
		return domain.NewUserError(domain.ErrInvalidInput, "Old password does not match")
	}

	if oldPassword == newPassword || s.hasher.Verify(user.PasswordHash, newPassword) {
		return domain.NewUserError(domain.ErrInvalidInput, "New password cannot be the same as old password")
	}

	if err := auth.ValidatePassword(newPassword, user.Username, user.Email, user.Name); err != nil {
		return err
	}

	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("could not hash password: %w", err)
	}

	now := time.Now()
	if err := s.repo.UpdatePassword(ctx, user.ID, newHash, now); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if s.mailer != nil && user.Email != "" {
		go func(to, username string) {
			_ = s.mailer.SendPasswordChangedEmail(to, username)
		}(user.Email, user.Username)
	}

	return nil
}

func (s *userService) applyChangeEmail(ctx context.Context, newEmail string, user *models.User) error {
	if newEmail == "" || newEmail == user.Email {
		return nil
	}
	existing, err := s.repo.FindByEmail(ctx, newEmail)
	if err == nil && existing != nil && existing.ID != user.ID {
		return domain.NewUserError(domain.ErrEmailConflict, "email already in use")
	}
	user.Email = newEmail
	return nil
}

func applyChangeBasicFields(change *models.ProfileChangeRequest, user *models.User) {
	user.Name = change.Name
	user.BniID = change.BniID
	if change.EmployeeID != "" {
		user.EmployeeID = change.EmployeeID
	}
	user.Division = change.Division
	if change.DivisionID != nil && *change.DivisionID != 0 {
		user.DivisionID = change.DivisionID
	}
	user.Site = change.Site
	if change.SiteID != nil && *change.SiteID != 0 {
		user.SiteID = change.SiteID
	}
}

func (s *userService) applyChangeDepartment(ctx context.Context, change *models.ProfileChangeRequest, user *models.User) {
	if change.DepartmentID != nil && *change.DepartmentID != 0 {
		if s.master != nil {
			dept, err := s.master.FindDepartmentByID(ctx, *change.DepartmentID)
			if err == nil && dept != nil && dept.IsActive {
				user.DepartmentID = &dept.ID
				user.Department = dept.Name
				if user.Division == "" {
					user.Division = dept.Division
				}
			}
		}
	} else if change.Department != "" {
		user.Department = change.Department
	}
}

func (s *userService) applyChangeCompany(ctx context.Context, change *models.ProfileChangeRequest, user *models.User) {
	if user.Role == models.RoleAdmin {
		user.CompanyID = nil
		user.Company = ""
		return
	}
	if change.CompanyID != nil && *change.CompanyID != 0 && s.master != nil {
		comp, err := s.master.FindCompanyByID(ctx, *change.CompanyID)
		if err == nil && comp != nil && comp.IsActive {
			user.CompanyID = &comp.ID
			user.Company = comp.Name
		}
	}
}

// ApplyApprovedProfileChange updates a user entity based on the approved profile change request.
func (s *userService) ApplyApprovedProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	if change == nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Profile change request is required")
	}
	user, err := s.repo.FindByID(ctx, change.UserID)
	if err := validateUpdatableUser(user, err); err != nil {
		return err
	}

	if err := s.applyChangeEmail(ctx, change.Email, user); err != nil {
		return err
	}
	applyChangeBasicFields(change, user)
	s.applyChangeDepartment(ctx, change, user)
	s.applyChangeCompany(ctx, change, user)

	return s.repo.Update(ctx, user)
}
