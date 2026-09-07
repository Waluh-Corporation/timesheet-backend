package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"timesheet-backend/models"
)

// ListCompanies returns all active companies with their active divisions and sites.
// Accessible by authenticated users to populate cascading dropdowns.
func (s *Server) ListCompanies(c *gin.Context) {
	var companies []models.Company
	if err := s.DB.Where("is_active = ?", true).
		Preload("Divisions", "is_active = ?", true).
		Preload("Sites", "is_active = ?", true).
		Order("name asc").
		Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, companies)
}

// ListDivisionsByCompany returns active divisions under a specific company.
func (s *Server) ListDivisionsByCompany(c *gin.Context) {
	companyID := c.Param("id")
	var divisions []models.Division
	if err := s.DB.Where("company_id = ? AND is_active = ?", companyID, true).
		Order("name asc").
		Find(&divisions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, divisions)
}

// ListSitesByCompany returns active sites under a specific company.
func (s *Server) ListSitesByCompany(c *gin.Context) {
	companyID := c.Param("id")
	var sites []models.Site
	if err := s.DB.Where("company_id = ? AND is_active = ?", companyID, true).
		Order("name asc").
		Find(&sites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sites)
}

// --- Admin Master Data Endpoints ---

type createCompanyRequest struct {
	Code     string `json:"code" binding:"required,min=2,max=32"`
	Name     string `json:"name" binding:"required,min=2,max=255"`
	IsActive *bool  `json:"is_active"`
}

type updateCompanyRequest struct {
	Code     *string `json:"code"`
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

// AdminCreateCompany adds a new company (admin only).
func (s *Server) AdminCreateCompany(c *gin.Context) {
	var req createCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	company := models.Company{
		Code:     req.Code,
		Name:     req.Name,
		IsActive: active,
	}

	if err := s.DB.Create(&company).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "company code already exists or invalid"})
		return
	}
	c.JSON(http.StatusCreated, company)
}

// AdminUpdateCompany modifies a company (admin only).
func (s *Server) AdminUpdateCompany(c *gin.Context) {
	id := c.Param("id")
	var company models.Company
	if err := s.DB.First(&company, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "company not found"})
		return
	}

	var req updateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Code != nil {
		updates["code"] = *req.Code
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := s.DB.Model(&company).Updates(updates).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	s.DB.First(&company, id)
	c.JSON(http.StatusOK, company)
}

// AdminDeleteCompany soft-deletes / deactivates a company (admin only).
func (s *Server) AdminDeleteCompany(c *gin.Context) {
	id := c.Param("id")
	var company models.Company
	if err := s.DB.First(&company, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "company not found"})
		return
	}
	if err := s.DB.Model(&company).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "company deactivated"})
}

type createDivisionRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name" binding:"required,min=2,max=255"`
	IsActive *bool  `json:"is_active"`
}

// AdminCreateDivision creates a division under a specific company (admin only).
func (s *Server) AdminCreateDivision(c *gin.Context) {
	companyID := c.Param("id")
	var company models.Company
	if err := s.DB.First(&company, companyID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "company not found"})
		return
	}

	var req createDivisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	division := models.Division{
		CompanyID: company.ID,
		Code:      req.Code,
		Name:      req.Name,
		IsActive:  active,
	}

	if err := s.DB.Create(&division).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, division)
}

// AdminDeleteDivision deactivates a division (admin only).
func (s *Server) AdminDeleteDivision(c *gin.Context) {
	divID := c.Param("divId")
	var division models.Division
	if err := s.DB.First(&division, divID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "division not found"})
		return
	}
	if err := s.DB.Model(&division).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "division deactivated"})
}

type createSiteRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=128"`
	IsActive *bool  `json:"is_active"`
}

// AdminCreateSite creates a site under a specific company (admin only).
func (s *Server) AdminCreateSite(c *gin.Context) {
	companyID := c.Param("id")
	var company models.Company
	if err := s.DB.First(&company, companyID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "company not found"})
		return
	}

	var req createSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	site := models.Site{
		CompanyID: company.ID,
		Name:      req.Name,
		IsActive:  active,
	}

	if err := s.DB.Create(&site).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, site)
}

// AdminDeleteSite deactivates a site (admin only).
func (s *Server) AdminDeleteSite(c *gin.Context) {
	siteID := c.Param("siteId")
	var site models.Site
	if err := s.DB.First(&site, siteID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}
	if err := s.DB.Model(&site).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "site deactivated"})
}
