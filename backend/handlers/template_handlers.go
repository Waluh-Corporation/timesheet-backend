package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timesheet-backend/models"
	"timesheet-backend/services"
)

// ListTemplates godoc
// @Summary List templates
// @Description Retrieves all uploaded timesheet templates with their cell mappings.
// @Tags Template
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Template
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/templates [get]
func (s *Server) ListTemplates(c *gin.Context) {
	var templates []models.Template
	if err := s.DB.Preload("CellMappings").Preload("CompanyRel").Preload("Creator").Order("created_at desc").Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, templates)
}

// UploadTemplate godoc
// @Summary Upload template workbook (Admin)
// @Description Ingests an admin-uploaded .xlsx workbook template for timesheet generation.
// @Tags Admin
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Excel .xlsx template file"
// @Param name formData string false "Template display name"
// @Param description formData string false "Template description"
// @Param company formData string false "Associated company code or name"
// @Param sheet_name formData string false "Target worksheet name"
// @Param is_default formData string false "Set as default template (true/false)"
// @Success 201 {object} models.Template
// @Failure 400 {object} models.ErrorResponse "Invalid file or parameters"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/templates [post]
func (s *Server) UploadTemplate(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read upload"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read upload"})
		return
	}

	name := c.PostForm("name")
	if name == "" {
		name = fileHeader.Filename
	}

	// Parse to determine the default sheet and validate the file is a workbook.
	_, sheet, err := services.ParseXLSXGrid(data, c.PostForm("sheet_name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid xlsx: " + err.Error()})
		return
	}

	createdBy := currentUserID(c)
	tmpl := models.Template{
		Name:        name,
		Description: c.PostForm("description"),
		Company:     c.PostForm("company"),
		SheetName:   sheet,
		FileData:    data,
		IsDefault:   c.PostForm("is_default") == "true",
		CreatedBy:   &createdBy,
	}

	if tmpl.Company != "" {
		var comp models.Company
		if err := s.DB.Where("LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)", tmpl.Company, "%"+tmpl.Company+"%").First(&comp).Error; err == nil {
			tmpl.CompanyID = &comp.ID
		}
	}

	if tmpl.IsDefault {
		s.DB.Model(&models.Template{}).Where("is_default = ?", true).Update("is_default", false)
	}
	if err := s.DB.Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = s.DB.Preload("CompanyRel").Preload("Creator").First(&tmpl, tmpl.ID)
	c.JSON(http.StatusCreated, tmpl)
}

// GetTemplateGrid godoc
// @Summary Get template grid layout
// @Description Returns the parsed 2-D cell grid and merged cells for Handsontable rendering.
// @Tags Template
// @Security BearerAuth
// @Produce json
// @Param id path int true "Template ID"
// @Success 200 {object} services.SheetLayout
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Template not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/templates/{id}/grid [get]
func (s *Server) GetTemplateGrid(c *gin.Context) {
	id := c.Param("id")
	var tmpl models.Template
	if err := s.DB.First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	layout, err := services.ParseXLSXLayout(tmpl.FileData, tmpl.SheetName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, layout)
}

// SaveTemplateMappings godoc
// @Summary Save template cell mappings (Admin)
// @Description Replaces all cell mappings for a template with the admin-specified configuration.
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Template ID"
// @Param request body models.SaveMappingsRequest true "Cell mappings list"
// @Success 200 {object} models.SaveMappingsResponse
// @Failure 400 {object} models.ErrorResponse "Invalid template ID or payload"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 404 {object} models.ErrorResponse "Template not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/templates/{id}/mappings [post]
func (s *Server) SaveTemplateMappings(c *gin.Context) {
	id := c.Param("id")
	tid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	var tmpl models.Template
	if err := s.DB.First(&tmpl, tid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	var req models.SaveMappingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", tid).Delete(&models.CellMapping{}).Error; err != nil {
			return err
		}
		for i := range req.Mappings {
			req.Mappings[i].ID = 0
			req.Mappings[i].TemplateID = uint(tid)
			if err := tx.Create(&req.Mappings[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var saved []models.CellMapping
	s.DB.Where("template_id = ?", tid).Find(&saved)
	c.JSON(http.StatusOK, gin.H{"mappings": saved})
}

// SetDefaultTemplate godoc
// @Summary Set default template (Admin)
// @Description Marks a template as the default used for generation when a user doesn't pick one (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "Template ID"
// @Success 200 {object} models.MessageResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 404 {object} models.ErrorResponse "Template not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/templates/{id}/default [post]
func (s *Server) SetDefaultTemplate(c *gin.Context) {
	id := c.Param("id")
	var tmpl models.Template
	if err := s.DB.First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	s.DB.Model(&models.Template{}).Where("is_default = ?", true).Update("is_default", false)
	if err := s.DB.Model(&tmpl).Update("is_default", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "default template set"})
}

// DeleteTemplate godoc
// @Summary Delete template (Admin)
// @Description Removes a template and its mappings (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "Template ID"
// @Success 200 {object} models.MessageResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/templates/{id} [delete]
func (s *Server) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := s.DB.Delete(&models.Template{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "template deleted"})
}
