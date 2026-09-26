package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
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
	svc := s.GetUserService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "user service not available")
		return
	}
	if err := svc.SubmitProfileChange(c.Request.Context(), uid, &req); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
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
	svc := s.GetUserService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "user service not available")
		return
	}
	changes, err := svc.MyProfileChanges(c.Request.Context(), currentUserID(c))
	if err != nil {
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

	svc := s.GetUserService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "user service not available")
		return
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
