package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	_ "timesheet-backend/dto/response"
	"timesheet-backend/models"
)

const queryID = "id = ?"

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

// CreateCompanyRequest carries fields to add a new company.
type CreateCompanyRequest struct {
	Code string `json:"code" binding:"required,min=2,max=32" example:"mii"`
	Name string `json:"name" binding:"required,min=2,max=255" example:"PT Mitra Integrasi Informatika"`
}

// UpdateCompanyRequest carries fields to update a company.
type UpdateCompanyRequest struct {
	Code *string `json:"code" example:"mii"`
	Name *string `json:"name" example:"PT Mitra Integrasi Informatika"`
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

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	appr := models.Approver{
		Name:     strings.TrimSpace(req.Name),
		RoleType: req.RoleType,
		Title:    strings.TrimSpace(req.Title),
		IsActive: isActive,
	}

	if err := s.DB.Create(&appr).Error; err != nil {
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

	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	var appr models.Approver
	if err := s.DB.WithContext(c.Request.Context()).Where(queryID, id).First(&appr).Error; err != nil {
		RespondError(c, http.StatusNotFound, "approver not found")
		return
	}

	if req.Name != nil {
		appr.Name = strings.TrimSpace(*req.Name)
	}
	if req.RoleType != nil {
		appr.RoleType = *req.RoleType
	}
	if req.Title != nil {
		appr.Title = strings.TrimSpace(*req.Title)
	}
	if req.IsActive != nil {
		appr.IsActive = *req.IsActive
	}

	if err := s.DB.Save(&appr).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to update approver: "+err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "approver updated successfully")
}

// DeleteApprover godoc
// @Summary Delete an approver (admin only)
// @Description Permanently removes an approver from master data.
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

	var appr models.Approver
	if err := s.DB.Where(queryID, id).First(&appr).Error; err != nil {
		RespondError(c, http.StatusNotFound, "approver not found")
		return
	}

	if err := s.DB.Delete(&appr).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to delete approver: "+err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}

// CreateCompany godoc
// @Summary Create a company (admin only)
// @Description Adds a new client/vendor company.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body handlers.CreateCompanyRequest true "Company data"
// @Success 201 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/companies [post]
func (s *Server) CreateCompany(c *gin.Context) {
	var req CreateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	comp := models.Company{
		Code: strings.ToLower(strings.TrimSpace(req.Code)),
		Name: strings.TrimSpace(req.Name),
	}

	if err := s.DB.Create(&comp).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to create company: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusCreated, "company created successfully")
}

// UpdateCompany godoc
// @Summary Update a company (admin only)
// @Description Updates code or name of an existing company.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Company ID"
// @Param request body handlers.UpdateCompanyRequest true "Company update data"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Company not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/companies/{id} [patch]
func (s *Server) UpdateCompany(c *gin.Context) {
	var req UpdateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid company ID, expected positive integer")
		return
	}

	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	var comp models.Company
	if err := s.DB.WithContext(c.Request.Context()).Where(queryID, id).First(&comp).Error; err != nil {
		RespondError(c, http.StatusNotFound, "company not found")
		return
	}

	if req.Code != nil {
		comp.Code = strings.ToLower(strings.TrimSpace(*req.Code))
	}
	if req.Name != nil {
		comp.Name = strings.TrimSpace(*req.Name)
	}

	if err := s.DB.Save(&comp).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to update company: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "company updated successfully")
}

// DeleteCompany godoc
// @Summary Delete a company (admin only)
// @Description Permanently removes a company from master data.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param id path int true "Company ID"
// @Success 200 {object} response.DeleteResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Company not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/companies/{id} [delete]
func (s *Server) DeleteCompany(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid company ID, expected positive integer")
		return
	}

	var comp models.Company
	if err := s.DB.Where(queryID, id).First(&comp).Error; err != nil {
		RespondError(c, http.StatusNotFound, "company not found")
		return
	}

	if err := s.DB.Delete(&comp).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to delete company: "+err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}
