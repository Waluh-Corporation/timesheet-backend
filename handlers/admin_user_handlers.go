package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

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
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		if isActive, err := strconv.ParseBool(isActiveStr); err == nil {
			query = query.Where("is_active = ?", isActive)
		}
	} else if c.Query("include_inactive") == "false" {
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
			DivisionID:   u.DivisionID,
			Department:   u.Department,
			DepartmentID: u.DepartmentID,
			Site:         u.Site,
			SiteID:       u.SiteID,
			Company:      u.Company,
			CompanyID:    u.CompanyID,
			IsActive:     u.IsActive,
		}
	}
	RespondSuccess(c, http.StatusOK, resp)
}

// CreateUser godoc
// @Summary Provision a new user (Admin)
// @Description Creates a new user account with a randomly generated secure password and sends a welcome notification.
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

	if user.Role == models.RoleAdmin {
		user.Company = ""
		user.CompanyID = nil
	}

	if errMsg, code := s.resolveUserMasterData(&req, &user); code != 0 {
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

	if errMsg, code := s.checkUserExistence(req.Username, req.Email); code != 0 {
		RespondError(c, code, errMsg)
		return
	}

	if err := s.DB.Create(&user).Error; err != nil {
		errMsg, code := handleCreateUserDBError(err)
		RespondError(c, code, errMsg)
		return
	}

	// Send account creation / welcome notification email completely separate from password reset flow.
	if s.Mailer != nil && user.Email != "" {
		loginLink := s.publicBaseURL(c) + "/login"
		go func(toEmail, username, pass, link string) {
			if err := s.Mailer.SendAccountWelcomeEmail(toEmail, username, pass, link); err != nil {
				slog.Error("failed to send account welcome email", "error", err, "email", toEmail)
			}
		}(user.Email, user.Username, plainPass, loginLink)
	}

	RespondSuccess(c, http.StatusCreated, response.CreateUserData{
		Message: "user created successfully",
		User:    response.ToUserResponse(&user),
	})
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
		RespondError(c, http.StatusNotFound, errUserNotFound)
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
	if err := s.DB.Where(queryIDAndIsActive, id).First(&user).Error; err != nil {
		RespondError(c, http.StatusNotFound, errUserNotFound)
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
