package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
)

// ListProfileChanges godoc
// @Summary List profile change requests (Admin)
// @Description Returns all submitted profile change requests with optional status filter (admin only).
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
	changes, err := s.GetUserService().ListProfileChanges(c.Request.Context(), c.Query("status"))
	if err != nil {
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
			DivisionID:   ch.DivisionID,
			Department:   ch.Department,
			DepartmentID: ch.DepartmentID,
			Site:         ch.Site,
			SiteID:       ch.SiteID,
			CompanyID:    ch.CompanyID,
			Email:        ch.Email,
			Notes:        ch.Notes,
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

	reviewer := currentUserID(c)
	if err := s.GetUserService().ReviewProfileChange(c.Request.Context(), uint(id), reviewer, action); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "request not found")
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			RespondError(c, http.StatusConflict, "request already reviewed")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to process profile change review: "+err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "profile change request "+action+"d")
}
