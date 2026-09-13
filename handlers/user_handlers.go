package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

const (
	orderCreatedAtDesc  = "created_at desc"
	queryCodeOrNameLike = "LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)"
)

// isSelf reports whether targetID refers to the authenticated caller.
func isSelf(c *gin.Context, targetID uint) bool {
	return targetID == currentUserID(c)
}

// ListUsers godoc
// @Summary List all users (Admin)
// @Description Retrieves all registered user accounts (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {array} response.AdminUserResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users [get]
func (s *Server) ListUsers(c *gin.Context) {
	var users []models.User
	query := s.DB.Order(orderCreatedAtDesc)
	if c.Query("include_inactive") != "true" {
		query = query.Where("is_active = ?", true)
	}
	if err := query.Find(&users).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]response.AdminUserResponse, len(users))
	for i, u := range users {
		resp[i] = response.AdminUserResponse{
			ID:           u.ID,
			Username:     u.Username,
			Email:        u.Email,
			Role:         u.Role,
			Name:         u.Name,
			BniID:        u.BniID,
			EmployeeID:   u.EmployeeID,
			Division:     u.Division,
			Department:   u.Department,
			DepartmentID: u.DepartmentID,
			Site:         u.Site,
			Company:      u.Company,
			CompanyID:    u.CompanyID,
			IsActive:     u.IsActive,
		}
	}
	RespondSuccess(c, http.StatusOK, resp)
}

func (s *Server) resolveUserCompany(req *request.CreateUserRequest, user *models.User) (string, int) {
	if user.Role == models.RoleAdmin {
		user.CompanyID = nil
		user.Company = ""
		return "", 0
	}
	if req.CompanyID != nil && *req.CompanyID != 0 {
		var comp models.Company
		if err := s.DB.Where("id = ? AND is_active = true", *req.CompanyID).First(&comp).Error; err != nil {
			return "Company or Department not found or inactive", http.StatusBadRequest
		}
		user.CompanyID = &comp.ID
		user.Company = comp.Name
	} else if req.Company != "" {
		var comp models.Company
		if err := s.DB.Where("(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true", req.Company, "%"+req.Company+"%").First(&comp).Error; err == nil {
			user.CompanyID = &comp.ID
			user.Company = comp.Name
		} else {
			return "Company or Department not found or inactive", http.StatusBadRequest
		}
	} else {
		user.CompanyID = nil
		user.Company = ""
	}
	return "", 0
}

func (s *Server) resolveUserDepartment(req *request.CreateUserRequest, user *models.User) (string, int) {
	if req.DepartmentID != nil && *req.DepartmentID != 0 {
		var dept models.Department
		if err := s.DB.Where("id = ? AND is_active = true", *req.DepartmentID).First(&dept).Error; err != nil {
			return "Company or Department not found or inactive", http.StatusBadRequest
		}
		user.DepartmentID = &dept.ID
		user.Department = dept.Name
		if user.Division == "" {
			user.Division = dept.Division
		}
	} else if req.Department != "" {
		var dept models.Department
		if err := s.DB.Where("(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true", req.Department, "%"+req.Department+"%").First(&dept).Error; err == nil {
			user.DepartmentID = &dept.ID
			user.Department = dept.Name
			if user.Division == "" {
				user.Division = dept.Division
			}
		}
	}
	return "", 0
}

func (s *Server) handleInitialPassword(user *models.User) (string, string, int) {
	plain, err := auth.GenerateSecurePassword(auth.GeneratedPasswordLength)
	if err != nil {
		return "", "failed to generate initial password", http.StatusInternalServerError
	}

	hasher := s.Hasher
	if hasher == nil {
		hasher = auth.DefaultHasher
	}
	hash, err := hasher.Hash(plain)
	if err != nil {
		return "", "could not hash password", http.StatusInternalServerError
	}
	user.PasswordHash = hash
	return plain, "", 0
}

// CreateUser godoc
// @Summary Create user (Admin)
// @Description Creates a new user account with role, departmental assignment, and initial password.
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.CreateUserRequest true "User provisioning payload"
// @Success 201 {object} response.CreateUserResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload or policy failure"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 409 {object} response.ErrorResponse "Username or email already exists"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users [post]
func (s *Server) CreateUser(c *gin.Context) {
	var req request.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		Role:         req.Role,
		Name:         req.Name,
		BniID:        req.BniID,
		Division:     req.Division,
		Department:   req.Department,
		DepartmentID: req.DepartmentID,
		Site:         req.Site,
		Company:      req.Company,
		CompanyID:    req.CompanyID,
		IsActive:     true,
	}

	if user.Role == models.RoleAdmin {
		user.Company = ""
		user.CompanyID = nil
	}

	if errMsg, code := s.resolveUserCompany(&req, &user); code != 0 {
		RespondError(c, code, errMsg)
		return
	}
	if errMsg, code := s.resolveUserDepartment(&req, &user); code != 0 {
		RespondError(c, code, errMsg)
		return
	}

	plainPass, errMsg, code := s.handleInitialPassword(&user)
	if code != 0 {
		RespondError(c, code, errMsg)
		return
	}

	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database connection unavailable")
		return
	}

	var existingByUsername models.User
	if err := s.DB.Where("LOWER(username) = LOWER(?)", req.Username).First(&existingByUsername).Error; err == nil {
		RespondError(c, http.StatusConflict, "username already exists")
		return
	}
	var existingByEmail models.User
	if err := s.DB.Where("LOWER(email) = LOWER(?)", req.Email).First(&existingByEmail).Error; err == nil {
		RespondError(c, http.StatusConflict, "email already exists")
		return
	}

	if err := s.DB.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "foreign key") {
			RespondError(c, http.StatusBadRequest, "Company or Department not found or inactive")
			return
		}
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "username") {
			RespondError(c, http.StatusConflict, "username already exists")
			return
		}
		if strings.Contains(errLower, "email") {
			RespondError(c, http.StatusConflict, "email already exists")
			return
		}
		RespondError(c, http.StatusConflict, "username or email already exists")
		return
	}

	// Send account creation / welcome notification email completely separate from password reset flow.
	if s.Mailer != nil && user.Email != "" {
		loginLink := s.publicBaseURL(c) + "/login"
		_ = s.Mailer.SendAccountWelcomeEmail(user.Email, user.Username, plainPass, loginLink)
	}

	RespondSuccess(c, http.StatusCreated, response.CreateUserData{
		Message: "user created successfully",
		User:    response.ToUserResponse(&user),
	})
}

func validateSelfUpdate(c *gin.Context, id uint, req *request.UpdateUserRequest) (string, int) {
	if !isSelf(c, id) {
		return "", 0
	}
	if req.IsActive != nil && !*req.IsActive {
		return "you cannot deactivate your own account", http.StatusForbidden
	}
	if req.Role != nil && *req.Role != models.RoleAdmin {
		return "you cannot remove your own admin role", http.StatusForbidden
	}
	return "", 0
}

func applyUserUpdates(db *gorm.DB, user *models.User, req *request.UpdateUserRequest) (string, int) {
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
	if req.Division != nil {
		user.Division = *req.Division
	}
	if req.Department != nil {
		user.Department = *req.Department
	}
	if req.DepartmentID != nil {
		if *req.DepartmentID != 0 {
			var dept models.Department
			if err := db.Where("id = ? AND is_active = true", *req.DepartmentID).First(&dept).Error; err != nil {
				return "Company or Department not found or inactive", http.StatusBadRequest
			}
			user.DepartmentID = &dept.ID
			user.Department = dept.Name
			if user.Division == "" {
				user.Division = dept.Division
			}
		} else {
			user.DepartmentID = nil
		}
	}
	if req.Site != nil {
		user.Site = *req.Site
	}
	if req.CompanyID != nil {
		if *req.CompanyID != 0 {
			var comp models.Company
			if err := db.Where("id = ? AND is_active = true", *req.CompanyID).First(&comp).Error; err != nil {
				return "Company or Department not found or inactive", http.StatusBadRequest
			}
			user.CompanyID = &comp.ID
			user.Company = comp.Name
		} else {
			user.CompanyID = nil
			user.Company = ""
		}
	} else if req.Company != nil {
		if *req.Company != "" {
			var comp models.Company
			if err := db.Where("(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true", *req.Company, "%"+*req.Company+"%").First(&comp).Error; err == nil {
				user.CompanyID = &comp.ID
				user.Company = comp.Name
			} else {
				return "Company or Department not found or inactive", http.StatusBadRequest
			}
		} else {
			user.CompanyID = nil
			user.Company = ""
		}
	}
	if user.Role == models.RoleAdmin {
		user.Company = ""
		user.CompanyID = nil
	}
	return "", 0
}

// UpdateUser godoc
// @Summary Update user (Admin)
// @Description Updates user attributes (role, active status, name, company, department). Self-deactivation and self-demotion are blocked.
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param request body request.UpdateUserRequest true "Update payload"
// @Success 200 {object} response.UpdateUserResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only or self-demotion forbidden"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Router /api/v1/admin/users/{id} [patch]
func (s *Server) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid user ID, expected positive integer")
		return
	}

	var req request.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if errMsg, code := validateSelfUpdate(c, uint(id), &req); code != 0 {
		RespondError(c, code, errMsg)
		return
	}

	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	var user models.User
	if err := s.DB.WithContext(c.Request.Context()).Where(queryID, id).First(&user).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	if errMsg, code := applyUserUpdates(s.DB, &user, &req); code != 0 {
		RespondError(c, code, errMsg)
		return
	}
	if err := s.DB.Save(&user).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to update user: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "user updated successfully")
}

// DeleteUser godoc
// @Summary Soft deactivate user (Admin)
// @Description Deactivates a user account (admin only). The account is not hard-deleted to preserve timesheet audit logs.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} response.MessageResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only or self-deactivation forbidden"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users/{id} [delete]
func (s *Server) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid user ID, expected positive integer")
		return
	}
	// An admin may never deactivate/delete their own account — doing so could
	// lock the last administrator out of the portal.
	if isSelf(c, uint(id)) {
		RespondError(c, http.StatusForbidden, "you cannot deactivate your own account")
		return
	}
	var user models.User
	if err := s.DB.Where("id = ? AND is_active = true", id).First(&user).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}
	if err := s.DB.Model(&user).Updates(map[string]interface{}{
		"is_active":  false,
		"updated_at": time.Now(),
	}).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "user deactivated successfully")
}

// --- Profile approval flow ---

// SubmitProfileChange godoc
// @Summary Submit self-service profile update request
// @Description Submits a user's profile change request for administrator review and approval.
// @Tags User
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.ProfileChangeRequestDTO true "Profile update fields"
// @Success 201 {object} response.SubmitProfileChangeResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/profile/change [post]
func (s *Server) SubmitProfileChange(c *gin.Context) {
	var req request.ProfileChangeRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	change := models.ProfileChangeRequest{
		UserID:       currentUserID(c),
		Status:       models.ProfilePending,
		Name:         req.Name,
		BniID:        req.BniID,
		EmployeeID:   req.EmployeeID,
		Division:     req.Division,
		Department:   req.Department,
		DepartmentID: req.DepartmentID,
		Site:         req.Site,
		CompanyID:    req.CompanyID,
	}
	if err := s.DB.Create(&change).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondMessage(c, http.StatusCreated, "profile change request submitted")
}

// MyProfileChanges godoc
// @Summary List current user's profile change requests
// @Description Returns the profile change requests submitted by the currently authenticated user.
// @Tags User
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.ProfileChangeRequest
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/profile/changes [get]
func (s *Server) MyProfileChanges(c *gin.Context) {
	var changes []models.ProfileChangeRequest
	if err := s.DB.Preload("CompanyRel", models.ActiveOnly).Preload("DepartmentRel", models.ActiveOnly).
		Where("user_id = ?", currentUserID(c)).
		Order(orderCreatedAtDesc).Find(&changes).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]response.ProfileChangeResponse, len(changes))
	for i, ch := range changes {
		resp[i] = response.ProfileChangeResponse{
			ID:            ch.ID,
			CreatedAt:     ch.CreatedAt,
			UpdatedAt:     ch.UpdatedAt,
			UserID:        ch.UserID,
			Status:        ch.Status,
			Name:          ch.Name,
			BniID:         ch.BniID,
			EmployeeID:    ch.EmployeeID,
			Division:      ch.Division,
			Department:    ch.Department,
			DepartmentID:  ch.DepartmentID,
			DepartmentRel: ch.DepartmentRel,
			GroupName:     ch.GroupName,
			Position:      ch.Position,
			Site:          ch.Site,
			CompanyID:     ch.CompanyID,
			CompanyRel:    ch.CompanyRel,
			ReviewedBy:    ch.ReviewedBy,
			ReviewedAt:    ch.ReviewedAt,
		}
	}
	RespondSuccess(c, http.StatusOK, resp)
}

// ListProfileChanges godoc
// @Summary List all profile change requests (Admin)
// @Description Retrieves submitted profile change requests with optional status filtering (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param status query string false "Filter by review status (pending, approved, rejected)"
// @Success 200 {array} response.AdminProfileChangeResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/profile-changes [get]
func (s *Server) ListProfileChanges(c *gin.Context) {
	var changes []models.ProfileChangeRequest
	q := s.DB.Preload("User").Preload("Reviewer").Order(orderCreatedAtDesc)
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&changes).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]response.AdminProfileChangeResponse, len(changes))
	for i, ch := range changes {
		var uName, uEmail, revName string
		if ch.User.ID != 0 {
			uName = ch.User.Name
			uEmail = ch.User.Email
		}
		if ch.Reviewer != nil {
			revName = ch.Reviewer.Name
		}
		resp[i] = response.AdminProfileChangeResponse{
			ID:           ch.ID,
			UserID:       ch.UserID,
			UserName:     uName,
			UserEmail:    uEmail,
			Status:       ch.Status,
			Name:         ch.Name,
			BniID:        ch.BniID,
			EmployeeID:   ch.EmployeeID,
			Division:     ch.Division,
			Department:   ch.Department,
			DepartmentID: ch.DepartmentID,
			Site:         ch.Site,
			CompanyID:    ch.CompanyID,
			ReviewedBy:   ch.ReviewedBy,
			ReviewerName: revName,
			ReviewedAt:   ch.ReviewedAt,
			CreatedAt:    ch.CreatedAt,
		}
	}
	RespondSuccess(c, http.StatusOK, resp)
}

// ReviewProfileChange godoc
// @Summary Review profile change request (Admin)
// @Description Approves or rejects a submitted profile change request (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "Request ID"
// @Param action query string true "Review action" Enums(approve, reject)
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid action"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 404 {object} response.ErrorResponse "Request not found"
// @Failure 409 {object} response.ErrorResponse "Request already reviewed"
// @Router /api/v1/admin/profile-changes/{id}/review [post]
func (s *Server) ReviewProfileChange(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid profile change request ID, expected positive integer")
		return
	}
	action := c.Query("action") // "approve" or "reject"

	if action != "approve" && action != "reject" {
		RespondError(c, http.StatusBadRequest, "invalid action: must be 'approve' or 'reject'")
		return
	}

	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	var change models.ProfileChangeRequest
	if err := s.DB.WithContext(c.Request.Context()).Where(queryID, id).First(&change).Error; err != nil {
		RespondError(c, http.StatusNotFound, "request not found")
		return
	}
	if change.Status != models.ProfilePending {
		RespondError(c, http.StatusConflict, "request already reviewed")
		return
	}

	reviewer := currentUserID(c)
	now := time.Now()

	if action == "approve" {
		s.applyApprovedProfileChange(&change)
		change.Status = models.ProfileApproved
	} else {
		change.Status = "rejected"
	}
	change.ReviewedBy = &reviewer
	change.ReviewedAt = &now
	s.DB.Save(&change)

	RespondMessage(c, http.StatusOK, "profile change request "+action+"d")
}

func (s *Server) applyApprovedProfileChange(change *models.ProfileChangeRequest) {
	var targetUser models.User
	if err := s.DB.Where(queryID, change.UserID).First(&targetUser).Error; err != nil {
		return
	}
	targetUser.Name = change.Name
	targetUser.BniID = change.BniID
	if change.EmployeeID != "" {
		targetUser.EmployeeID = change.EmployeeID
	}
	targetUser.Division = change.Division
	targetUser.Site = change.Site
	if change.DepartmentID != nil && *change.DepartmentID != 0 {
		var dept models.Department
		if err := s.DB.Where("id = ? AND is_active = true", *change.DepartmentID).First(&dept).Error; err == nil {
			targetUser.DepartmentID = &dept.ID
			targetUser.Department = dept.Name
			if targetUser.Division == "" {
				targetUser.Division = dept.Division
			}
		}
	} else if change.Department != "" {
		targetUser.Department = change.Department
	}

	if targetUser.Role != models.RoleAdmin {
		if change.CompanyID != nil && *change.CompanyID != 0 {
			var comp models.Company
			if err := s.DB.Where("id = ? AND is_active = true", *change.CompanyID).First(&comp).Error; err == nil {
				targetUser.CompanyID = &comp.ID
				targetUser.Company = comp.Name
			}
		}
	} else {
		targetUser.CompanyID = nil
		targetUser.Company = ""
	}
	_ = s.DB.Save(&targetUser).Error
}

// ChangePassword godoc
// @Summary Change password
// @Description Changes the password of the currently authenticated user or admin.
// @Tags User
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.ChangePasswordRequest true "Change password payload"
// @Success 200 {object} response.ChangePasswordResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload, old password mismatch, or policy violation"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Account is disabled"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/users/change-password [post]
func (s *Server) ChangePassword(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	svc := s.UserSvc
	if svc == nil {
		svc = service.NewUserService(repository.NewUserRepository(s.DB), s.Hasher, s.Mailer)
	}

	if err := svc.ChangePassword(c.Request.Context(), uid, req.OldPassword, req.NewPassword); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, domain.ErrAccountDisabled) {
			RespondError(c, http.StatusForbidden, "account is disabled")
			return
		}
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "password changed successfully")
}
