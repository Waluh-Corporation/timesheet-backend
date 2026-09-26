package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	_ "timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// CreateApproverRequest carries fields to add a new approver.
type CreateApproverRequest struct {
	Name     string                  `json:"name" binding:"required" example:"Approver Name"`
	RoleType models.ApproverRoleType `json:"role_type" binding:"required,oneof=team_leader department_head" example:"team_leader"`
	Title    string                  `json:"title" example:"Team Leader"`
	IsActive *bool                   `json:"is_active" example:"true"`
}

// UpdateApproverRequest carries fields to update an existing approver.
type UpdateApproverRequest struct {
	Name     *string                  `json:"name" example:"Approver Name Updated"`
	RoleType *models.ApproverRoleType `json:"role_type" example:"department_head"`
	Title    *string                  `json:"title" example:"Department Head"`
	IsActive *bool                    `json:"is_active" example:"true"`
}

// CreateApprover godoc
// @Summary Create a new approver (admin only)
// @Description Adds a new approver (Team Leader or Department Head).
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body handlers.CreateApproverRequest true "Approver data"
// @Success 201 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/approvers [post]
func (s *Server) CreateApprover(c *gin.Context) {
	var req CreateApproverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	_, err := svc.CreateApprover(reqContext(c), req.Name, req.RoleType, req.Title, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to create approver: "+err.Error())
		return
	}

	RespondMessage(c, http.StatusCreated, "approver created successfully")
}

// UpdateApprover godoc
// @Summary Update an existing approver (admin only)
// @Description Updates approver name, role, title, or active status.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Approver ID"
// @Param request body handlers.UpdateApproverRequest true "Approver update data"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Approver not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/approvers/{id} [patch]
func (s *Server) UpdateApprover(c *gin.Context) {
	var req UpdateApproverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid approver ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	_, err = svc.UpdateApprover(reqContext(c), uint(id), req.Name, req.RoleType, req.Title, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "approver not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to update approver: "+err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "approver updated successfully")
}

// DeleteApprover godoc
// @Summary Delete an approver (admin only)
// @Description Soft-deactivates an approver from master data.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param id path int true "Approver ID"
// @Success 200 {object} response.DeleteResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Approver not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/approvers/{id} [delete]
func (s *Server) DeleteApprover(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid approver ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	if err := svc.DeleteApprover(reqContext(c), uint(id)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "approver not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to delete approver: "+err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}

// AdminListApprovers godoc
// @Summary List all approvers (admin only)
// @Description Returns all registered approvers (both active and inactive).
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Approver
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/approvers [get]
func (s *Server) AdminListApprovers(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	approvers, err := svc.ListApprovers(reqContext(c), c.Query("role_type"), nil)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, approvers)
}
