package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	_ "timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
)

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
