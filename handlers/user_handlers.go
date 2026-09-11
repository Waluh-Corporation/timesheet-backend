package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/models"
)

// isSelf reports whether targetID refers to the authenticated caller.
func isSelf(c *gin.Context, targetID uint) bool {
	return targetID == currentUserID(c)
}

// ListUsers godoc
// @Summary List all users (Admin)
// @Description Retrieves all registered user accounts with company and department associations (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.User
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users [get]
func (s *Server) ListUsers(c *gin.Context) {
	var users []models.User
	if err := s.DB.Preload("CompanyRel").Preload("DepartmentRel").Order("created_at desc").Find(&users).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, users)
}

// CreateUser godoc
// @Summary Provision new user account (Admin)
// @Description Creates a new user account and emails an account setup invitation link (admin only).
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.CreateUserRequest true "User provisioning payload"
// @Success 201 {object} models.User
// @Failure 400 {object} models.ErrorResponse "Invalid payload or password policy failure"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 409 {object} models.ErrorResponse "Username or email already exists"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users [post]
func (s *Server) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
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

	// Resolve Company
	if req.CompanyID != nil && *req.CompanyID != 0 {
		var comp models.Company
		if err := s.DB.First(&comp, *req.CompanyID).Error; err == nil {
			user.CompanyID = &comp.ID
			if user.Company == "" {
				user.Company = comp.Name
			}
		}
	} else if req.Company != "" {
		var comp models.Company
		if err := s.DB.Where("LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)", req.Company, "%"+req.Company+"%").First(&comp).Error; err == nil {
			user.CompanyID = &comp.ID
		}
	}

	// Resolve Department
	if req.DepartmentID != nil && *req.DepartmentID != 0 {
		var dept models.Department
		if err := s.DB.First(&dept, *req.DepartmentID).Error; err == nil {
			user.DepartmentID = &dept.ID
			if user.Department == "" {
				user.Department = dept.Name
			}
			if user.Division == "" {
				user.Division = dept.Division
			}
		}
	} else if req.Department != "" {
		var dept models.Department
		if err := s.DB.Where("LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)", req.Department, "%"+req.Department+"%").First(&dept).Error; err == nil {
			user.DepartmentID = &dept.ID
			if user.Division == "" && dept.Division != "" {
				user.Division = dept.Division
			}
		}
	}

	if req.Password != "" {
		// Enforce the NIST SP 800-63B policy on any admin-supplied initial
		// password (length + blocklist + context-specific terms).
		if err := auth.ValidatePassword(req.Password, req.Username, req.Email); err != nil {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			RespondError(c, http.StatusInternalServerError, "could not hash password")
			return
		}
		user.PasswordHash = hash
	}

	if err := s.DB.Create(&user).Error; err != nil {
		RespondError(c, http.StatusConflict, "username or email already exists")
		return
	}

	// Issue a setup token so the user can set their own password / passkey.
	raw, hash, err := auth.GenerateResetToken()
	if err == nil {
		s.DB.Create(&models.PasswordResetToken{
			UserID:    user.ID,
			TokenType: "account_setup",
			TokenHash: hash,
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			CreatedIP: c.ClientIP(),
		})
		link := s.publicBaseURL(c) + "/reset-password?token=" + raw
		_ = s.Mailer.SendSetupEmail(user.Email, user.Username, link)
	}

	_ = s.DB.Preload("CompanyRel").Preload("DepartmentRel").First(&user, user.ID)
	RespondSuccess(c, http.StatusCreated, user)
}

// UpdateUser godoc
// @Summary Update user attributes (Admin)
// @Description Updates user profile information, role, or active status directly (admin only).
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param request body models.UpdateUserRequest true "Update payload"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse "Invalid payload"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only or self-demotion forbidden"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Router /api/v1/admin/users/{id} [patch]
func (s *Server) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid user ID, expected positive integer")
		return
	}

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// An admin may never deactivate or demote their own account through the
	// update path either — both are self-lockout vectors.
	if isSelf(c, uint(id)) {
		if req.IsActive != nil && !*req.IsActive {
			RespondError(c, http.StatusForbidden, "you cannot deactivate your own account")
			return
		}
		if req.Role != nil && *req.Role != models.RoleAdmin {
			RespondError(c, http.StatusForbidden, "you cannot remove your own admin role")
			return
		}
	}

	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	var user models.User
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ?", id).First(&user).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.BniID != nil {
		updates["bni_id"] = *req.BniID
	}
	if req.Division != nil {
		updates["division"] = *req.Division
	}
	if req.Department != nil {
		updates["department"] = *req.Department
	}
	if req.DepartmentID != nil {
		updates["department_id"] = *req.DepartmentID
	}
	if req.Site != nil {
		updates["site"] = *req.Site
	}
	if req.CompanyID != nil {
		updates["company_id"] = *req.CompanyID
	}
	if req.Company != nil {
		updates["company"] = *req.Company
		var comp models.Company
		if err := s.DB.Where("LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)", *req.Company, "%"+*req.Company+"%").First(&comp).Error; err == nil {
			updates["company_id"] = comp.ID
		} else {
			updates["company_id"] = nil
		}
	}
	if len(updates) > 0 {
		s.DB.Model(&user).Updates(updates)
	}
	s.DB.Preload("CompanyRel").Preload("DepartmentRel").First(&user, id)
	RespondSuccess(c, http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Soft deactivate user (Admin)
// @Description Deactivates a user account (admin only). The account is not hard-deleted to preserve timesheet audit logs.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} models.User
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only or self-deactivation forbidden"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
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
	if err := s.DB.Where("id = ?", id).First(&user).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}
	if err := s.DB.Model(&user).Update("is_active", false).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.DB.Where("id = ?", id).First(&user)
	RespondSuccess(c, http.StatusOK, user)
}

// --- Profile approval flow ---

// SubmitProfileChange godoc
// @Summary Submit self-service profile update request
// @Description Submits a user's profile change request for administrator review and approval.
// @Tags User
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.ProfileChangeRequestDTO true "Profile update fields"
// @Success 201 {object} models.ProfileChangeRequest
// @Failure 400 {object} models.ErrorResponse "Invalid payload"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/profile/change [post]
func (s *Server) SubmitProfileChange(c *gin.Context) {
	var req models.ProfileChangeRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	change := models.ProfileChangeRequest{
		UserID:       currentUserID(c),
		Status:       models.ProfilePending,
		Name:         req.Name,
		BniID:        req.BniID,
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
	RespondSuccess(c, http.StatusCreated, change)
}

// MyProfileChanges godoc
// @Summary List current user's profile change requests
// @Description Returns the profile change requests submitted by the currently authenticated user.
// @Tags User
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.ProfileChangeRequest
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/profile/changes [get]
func (s *Server) MyProfileChanges(c *gin.Context) {
	var changes []models.ProfileChangeRequest
	if err := s.DB.Preload("CompanyRel").Preload("DepartmentRel").
		Where("user_id = ?", currentUserID(c)).
		Order("created_at desc").Find(&changes).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, changes)
}

// ListProfileChanges godoc
// @Summary List all profile change requests (Admin)
// @Description Retrieves submitted profile change requests with optional status filtering (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param status query string false "Filter by review status (pending, approved, rejected)"
// @Success 200 {array} models.ProfileChangeRequest
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/profile-changes [get]
func (s *Server) ListProfileChanges(c *gin.Context) {
	var changes []models.ProfileChangeRequest
	q := s.DB.Preload("User").Preload("Reviewer").Preload("CompanyRel").Preload("DepartmentRel").Order("created_at desc")
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&changes).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, changes)
}

// ReviewProfileChange godoc
// @Summary Review profile change request (Admin)
// @Description Approves or rejects a submitted profile change request (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "Request ID"
// @Param action query string true "Review action" Enums(approve, reject)
// @Success 200 {object} models.ProfileChangeRequest
// @Failure 400 {object} models.ErrorResponse "Invalid action"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 404 {object} models.ErrorResponse "Request not found"
// @Failure 409 {object} models.ErrorResponse "Request already reviewed"
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
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ?", id).First(&change).Error; err != nil {
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
		updates := map[string]interface{}{
			"name":     change.Name,
			"bni_id":   change.BniID,
			"division": change.Division,
			"site":     change.Site,
		}
		if change.Department != "" {
			updates["department"] = change.Department
		}
		if change.DepartmentID != nil {
			updates["department_id"] = change.DepartmentID
		}
		if change.CompanyID != nil {
			updates["company_id"] = change.CompanyID
		}
		s.DB.Model(&models.User{}).Where("id = ?", change.UserID).Updates(updates)
		change.Status = models.ProfileApproved
	} else {
		change.Status = "rejected"
	}
	change.ReviewedBy = &reviewer
	change.ReviewedAt = &now
	s.DB.Save(&change)

	_ = s.DB.Preload("User").Preload("Reviewer").Preload("CompanyRel").Preload("DepartmentRel").First(&change, change.ID)
	RespondSuccess(c, http.StatusOK, change)
}
