package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

const (
	orderCreatedAtDesc          = "created_at desc"
	queryCodeOrNameLike         = "LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)"
	queryIDAndIsActive          = "id = ? AND is_active = true"
	queryCodeOrNameLikeIsActive = "(LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)) AND is_active = true"
	errCompanyDeptNotFound      = "Company or Department not found or inactive"
)

// isSelf reports whether targetID refers to the authenticated caller.
func isSelf(c *gin.Context, targetID uint) bool {
	return targetID == currentUserID(c)
}

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
	uid := currentUserID(c)
	if req.Email != "" {
		var existing models.User
		if err := s.DB.Where("email = ? AND id != ?", req.Email, uid).First(&existing).Error; err == nil {
			RespondError(c, http.StatusBadRequest, "email is already registered by another user")
			return
		}
	}
	change := models.ProfileChangeRequest{
		UserID:       uid,
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
			Email:         ch.Email,
			Notes:         ch.Notes,
			ReviewedBy:    ch.ReviewedBy,
			ReviewedAt:    ch.ReviewedAt,
		}
	}
	RespondSuccess(c, http.StatusOK, resp)
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
			RespondError(c, http.StatusNotFound, errUserNotFound)
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
