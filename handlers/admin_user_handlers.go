package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
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
	svc := s.GetUserService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "user service unavailable")
		return
	}

	var isActive *bool
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		if b, err := strconv.ParseBool(isActiveStr); err == nil {
			isActive = &b
		}
	} else if c.Query("include_inactive") == "false" {
		t := true
		isActive = &t
	}
	users, err := svc.ListUsers(c.Request.Context(), isActive)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]response.AdminUserResponse, len(users))
	for i, u := range users {
		resp[i] = response.AdminUserResponse{
			ID:           u.ID,
			Username:     u.Username,
			Email:        u.Email,
			Role:         models.Role(u.Role),
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

	svc := s.GetUserService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "user service unavailable")
		return
	}

	user, _, err := svc.AdminCreateUser(c.Request.Context(), &req, s.publicBaseURL(c))
	if err != nil {
		if errors.Is(err, domain.ErrUsernameConflict) || errors.Is(err, domain.ErrEmailConflict) {
			RespondError(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusCreated, response.CreateUserData{
		Message: "user created successfully",
		User:    response.ToUserResponse(user),
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

	svc := s.GetUserService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "user service unavailable")
		return
	}

	callerID := currentUserID(c)
	if err := svc.AdminUpdateUser(c.Request.Context(), uint(id), callerID, &req); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, errUserNotFound)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
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
	if isSelf(c, uint(id)) {
		RespondError(c, http.StatusForbidden, "you cannot deactivate your own account")
		return
	}
	svc := s.GetUserService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "user service unavailable")
		return
	}

	callerID := currentUserID(c)
	if err := svc.AdminDeleteUser(c.Request.Context(), uint(id), callerID); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, errUserNotFound)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "user deactivated successfully")
}
