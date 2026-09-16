package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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

// CreateSiteRequest carries fields to add a new site.
type CreateSiteRequest struct {
	Code     string `json:"code" binding:"required,min=2,max=64" example:"jkt"`
	Name     string `json:"name" binding:"required,min=2,max=255" example:"Jakarta"`
	IsActive *bool  `json:"is_active" example:"true"`
}

// UpdateSiteRequest carries fields to update an existing site.
type UpdateSiteRequest struct {
	Code     *string `json:"code" example:"jkt"`
	Name     *string `json:"name" example:"Jakarta"`
	IsActive *bool   `json:"is_active" example:"true"`
}

// CreateDivisionRequest carries fields to add a new division.
type CreateDivisionRequest struct {
	Code     string `json:"code" binding:"required,min=2,max=64" example:"wdd"`
	Name     string `json:"name" binding:"required,min=2,max=255" example:"Wholesale Digital Delivery"`
	IsActive *bool  `json:"is_active" example:"true"`
}

// UpdateDivisionRequest carries fields to update an existing division.
type UpdateDivisionRequest struct {
	Code     *string `json:"code" example:"wdd"`
	Name     *string `json:"name" example:"Wholesale Digital Delivery"`
	IsActive *bool   `json:"is_active" example:"true"`
}

// CreateDepartmentRequest carries fields to add a new department.
type CreateDepartmentRequest struct {
	Code       string `json:"code" binding:"required,min=2,max=64" example:"wdl"`
	Name       string `json:"name" binding:"required,min=2,max=255" example:"Wholesale Channel and Service Delivery"`
	Division   string `json:"division" example:"Wholesale Digital Delivery"`
	DivisionID *uint  `json:"division_id" example:"1"`
	IsActive   *bool  `json:"is_active" example:"true"`
}

// UpdateDepartmentRequest carries fields to update an existing department.
type UpdateDepartmentRequest struct {
	Code       *string `json:"code" example:"wdl"`
	Name       *string `json:"name" example:"Wholesale Channel and Service Delivery"`
	Division   *string `json:"division" example:"Wholesale Digital Delivery"`
	DivisionID *uint   `json:"division_id" example:"1"`
	IsActive   *bool   `json:"is_active" example:"true"`
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

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	_, err := svc.CreateCompany(reqContext(c), req.Code, req.Name)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
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

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	_, err = svc.UpdateCompany(reqContext(c), uint(id), req.Code, req.Name, nil)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "company not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to update company: "+err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "company updated successfully")
}

// DeleteCompany godoc
// @Summary Delete a company (admin only)
// @Description Soft-deactivates a company from master data.
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

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	if err := svc.DeleteCompany(reqContext(c), uint(id)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "company not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to delete company: "+err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}

// CreateSite godoc
// @Summary Create a site (admin only)
// @Description Adds a new office or placement site.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body handlers.CreateSiteRequest true "Site data"
// @Success 201 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/sites [post]
func (s *Server) CreateSite(c *gin.Context) {
	var req CreateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	_, err := svc.CreateSite(reqContext(c), req.Code, req.Name, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to create site: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusCreated, "site created successfully")
}

// UpdateSite godoc
// @Summary Update a site (admin only)
// @Description Updates code, name, or active status of an existing site.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Site ID"
// @Param request body handlers.UpdateSiteRequest true "Site update data"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Site not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/sites/{id} [patch]
func (s *Server) UpdateSite(c *gin.Context) {
	var req UpdateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid site ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	_, err = svc.UpdateSite(reqContext(c), uint(id), req.Code, req.Name, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "site not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to update site: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "site updated successfully")
}

// DeleteSite godoc
// @Summary Delete a site (admin only)
// @Description Soft-deactivates a site from master data.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param id path int true "Site ID"
// @Success 200 {object} response.DeleteResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Site not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/sites/{id} [delete]
func (s *Server) DeleteSite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid site ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	if err := svc.DeleteSite(reqContext(c), uint(id)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "site not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to delete site: "+err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}

// CreateDivision godoc
// @Summary Create a division (admin only)
// @Description Adds a new organizational division.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body handlers.CreateDivisionRequest true "Division data"
// @Success 201 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/divisions [post]
func (s *Server) CreateDivision(c *gin.Context) {
	var req CreateDivisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	_, err := svc.CreateDivision(reqContext(c), req.Code, req.Name, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to create division: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusCreated, "division created successfully")
}

// UpdateDivision godoc
// @Summary Update a division (admin only)
// @Description Updates code, name, or active status of an existing division.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Division ID"
// @Param request body handlers.UpdateDivisionRequest true "Division update data"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Division not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/divisions/{id} [patch]
func (s *Server) UpdateDivision(c *gin.Context) {
	var req UpdateDivisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid division ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	_, err = svc.UpdateDivision(reqContext(c), uint(id), req.Code, req.Name, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "division not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to update division: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "division updated successfully")
}

// DeleteDivision godoc
// @Summary Delete a division (admin only)
// @Description Soft-deactivates a division from master data.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param id path int true "Division ID"
// @Success 200 {object} response.DeleteResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Division not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/divisions/{id} [delete]
func (s *Server) DeleteDivision(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid division ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	if err := svc.DeleteDivision(reqContext(c), uint(id)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "division not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to delete division: "+err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}

// CreateDepartment godoc
// @Summary Create a department (admin only)
// @Description Adds a new organizational department.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body handlers.CreateDepartmentRequest true "Department data"
// @Success 201 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/departments [post]
func (s *Server) CreateDepartment(c *gin.Context) {
	var req CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	_, err := svc.CreateDepartment(reqContext(c), req.Code, req.Name, req.Division, req.DivisionID, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to create department: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusCreated, "department created successfully")
}

// UpdateDepartment godoc
// @Summary Update a department (admin only)
// @Description Updates code, name, division, or active status of a department.
// @Tags Master Data
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Department ID"
// @Param request body handlers.UpdateDepartmentRequest true "Department update data"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Department not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/departments/{id} [patch]
func (s *Server) UpdateDepartment(c *gin.Context) {
	var req UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid department ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	_, err = svc.UpdateDepartment(reqContext(c), uint(id), req.Code, req.Name, req.Division, req.DivisionID, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "department not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to update department: "+err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "department updated successfully")
}

// DeleteDepartment godoc
// @Summary Delete a department (admin only)
// @Description Soft-deactivates a department from master data.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param id path int true "Department ID"
// @Success 200 {object} response.DeleteResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "Department not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/departments/{id} [delete]
func (s *Server) DeleteDepartment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid department ID, expected positive integer")
		return
	}

	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}

	if err := svc.DeleteDepartment(reqContext(c), uint(id)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "department not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "failed to delete department: "+err.Error())
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

// AdminListCompanies godoc
// @Summary List all companies (admin only)
// @Description Returns all companies (both active and inactive).
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Company
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/companies [get]
func (s *Server) AdminListCompanies(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	companies, err := svc.ListCompanies(reqContext(c), nil)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, companies)
}

// AdminListSites godoc
// @Summary List all sites (admin only)
// @Description Returns all registered office/placement sites (both active and inactive).
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Site
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/sites [get]
func (s *Server) AdminListSites(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	sites, err := svc.ListSites(reqContext(c), nil)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, sites)
}

// AdminListDivisions godoc
// @Summary List all divisions (admin only)
// @Description Returns all organizational divisions (both active and inactive).
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Division
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/divisions [get]
func (s *Server) AdminListDivisions(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	divisions, err := svc.ListDivisions(reqContext(c), nil)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, divisions)
}

// AdminListDepartments godoc
// @Summary List all departments (admin only)
// @Description Returns all departments (both active and inactive) with optional division filter.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param division query string false "Filter by division name"
// @Param division_id query int false "Filter by division ID"
// @Success 200 {array} models.Department
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/departments [get]
func (s *Server) AdminListDepartments(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	var divID *uint
	if divIDStr := c.Query("division_id"); divIDStr != "" {
		if id, err := strconv.ParseUint(divIDStr, 10, 64); err == nil {
			uID := uint(id)
			divID = &uID
		}
	}
	divName := strings.TrimSpace(c.Query("division"))
	depts, err := svc.ListDepartments(reqContext(c), divID, divName, nil)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, depts)
}
