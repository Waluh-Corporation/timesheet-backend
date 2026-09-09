package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"timesheet-backend/models"
	"timesheet-backend/services"
)

// jakarta returns the Asia/Jakarta location, falling back to a fixed +07:00.
func jakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

// UpsertDailyActivity godoc
// @Summary Upsert daily timesheet activity
// @Description Creates or updates a daily activity record for the authenticated user on a given date.
// @Tags Activity
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.DailyActivityRequest true "Daily activity payload"
// @Success 200 {object} models.DailyActivity
// @Failure 400 {object} models.ErrorResponse "Invalid date format or payload"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/activities [post]
func (s *Server) UpsertDailyActivity(c *gin.Context) {
	var req models.DailyActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	date, err := time.ParseInLocation("2006-01-02", req.Date, jakarta())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, expected YYYY-MM-DD"})
		return
	}

	activity := models.DailyActivity{
		UserID:       currentUserID(c),
		Date:         date,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		Status:       req.Status,
		Activity:     req.Activity,
		ProjectName:  req.ProjectName,
		ProjectID:    req.ProjectID,
		AppImpacted:  req.AppImpacted,
		ProjectRefID: req.ProjectRefID,
	}

	// Associate with normalized Project if matched by ID, code, or name
	if req.ProjectRefID != nil && *req.ProjectRefID != 0 {
		var proj models.Project
		if err := s.DB.First(&proj, *req.ProjectRefID).Error; err == nil {
			activity.ProjectRefID = &proj.ID
			if activity.ProjectName == "" {
				activity.ProjectName = proj.Name
			}
			if activity.ProjectID == "" {
				activity.ProjectID = proj.Code
			}
			if activity.AppImpacted == "" {
				activity.AppImpacted = proj.AppImpacted
			}
		}
	} else if req.ProjectID != "" || req.ProjectName != "" {
		var proj models.Project
		if err := s.DB.Where("code = ? OR LOWER(name) = LOWER(?)", req.ProjectID, req.ProjectName).First(&proj).Error; err == nil {
			activity.ProjectRefID = &proj.ID
			if activity.AppImpacted == "" && proj.AppImpacted != "" {
				activity.AppImpacted = proj.AppImpacted
			}
		}
	}

	// Upsert on the (user_id, date) unique index.
	err = s.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"start_time", "end_time", "status", "activity",
			"project_name", "project_id", "app_impacted", "project_ref_id", "updated_at",
		}),
	}).Create(&activity).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Preload associations before returning
	_ = s.DB.Preload("ProjectRef").Preload("StatusRef").First(&activity, activity.ID)
	c.JSON(http.StatusOK, activity)
}

// ListMonthlyActivities godoc
// @Summary List monthly activities
// @Description Retrieves all daily activities for the authenticated user for a specific month and year.
// @Tags Activity
// @Security BearerAuth
// @Produce json
// @Param year query int false "Year (defaults to current year)"
// @Param month query int false "Month 1-12 (defaults to current month)"
// @Success 200 {array} models.DailyActivity
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/activities [get]
func (s *Server) ListMonthlyActivities(c *gin.Context) {
	year := queryIntDefault(c, "year", time.Now().In(jakarta()).Year())
	month := queryIntDefault(c, "month", int(time.Now().In(jakarta()).Month()))

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, jakarta())
	end := start.AddDate(0, 1, 0)

	var activities []models.DailyActivity
	if err := s.DB.Preload("ProjectRef").Preload("StatusRef").
		Where("user_id = ? AND date >= ? AND date < ?", currentUserID(c), start, end).
		Order("date asc").Find(&activities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, activities)
}

// GenerateTimesheet godoc
// @Summary Generate timesheet spreadsheet
// @Description Renders monthly activities and overtimes into an Excel (.xlsx) workbook, initiates download, and dispatches an email copy.
// @Tags Timesheet
// @Security BearerAuth
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param request body models.GenerateRequest true "Generation parameters"
// @Success 200 {file} binary "Generated Excel workbook (.xlsx)"
// @Failure 400 {object} models.ErrorResponse "Template or company mapping missing"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 500 {object} models.ErrorResponse "Generation failed"
// @Router /api/v1/timesheet/generate [post]
func (s *Server) GenerateTimesheet(c *gin.Context) {
	var req models.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := s.DB.First(&user, currentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Resolve the template: explicit id, or strictly by user's assigned company (relational first, then string fallback).
	var tmpl models.Template
	if req.TemplateID != 0 {
		if err := s.DB.Preload("CellMappings").Where("id = ?", req.TemplateID).First(&tmpl).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "template not found"})
			return
		}
	} else if user.CompanyID != nil && *user.CompanyID != 0 {
		if err := s.DB.Preload("CellMappings").Where("company_id = ?", *user.CompanyID).First(&tmpl).Error; err != nil {
			// Fallback to name/string matching if company_id template is not flagged
			if err2 := s.DB.Preload("CellMappings").Where("LOWER(company) = LOWER(?) OR LOWER(name) LIKE LOWER(?) OR LOWER(builtin) = LOWER(?)", user.Company, "%"+user.Company+"%", user.Company).First(&tmpl).Error; err2 != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no template available for company: " + user.Company + ". Please ask an admin to upload a template for " + user.Company})
				return
			}
		}
	} else if user.Company != "" {
		if err := s.DB.Preload("CellMappings").Where("LOWER(company) = LOWER(?) OR LOWER(name) LIKE LOWER(?) OR LOWER(builtin) = LOWER(?)", user.Company, "%"+user.Company+"%", user.Company).First(&tmpl).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no template available for company: " + user.Company + ". Please ask an admin to upload a template for " + user.Company})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user has no company assigned. Ask an admin to assign a company (MII, SDD, NTT, or Adidata) to your account."})
		return
	}

	start := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, jakarta())
	end := start.AddDate(0, 1, 0)
	var activities []models.DailyActivity
	s.DB.Where("user_id = ? AND date >= ? AND date < ?", user.ID, start, end).Find(&activities)

	var overtimes []models.OvertimeEntry
	s.DB.Where("user_id = ? AND date >= ? AND date < ?", user.ID, start, end).Order("date asc").Find(&overtimes)

	// Fetch public holidays for the month so weekends/holidays are reflected in
	// the generated sheet (best-effort; generation still proceeds on failure).
	holidays := map[int]string{}
	if hs, herr := services.FetchHolidays(req.Year, req.Month); herr == nil {
		for _, h := range hs {
			var y, m, d int
			if _, e := fmt.Sscanf(h.Date, "%d-%d-%d", &y, &m, &d); e == nil {
				holidays[d] = h.Description
			}
		}
	}

	out, err := services.GenerateFromTemplate(services.GenerationInput{
		Template:   &tmpl,
		Mappings:   tmpl.CellMappings,
		User:       &user,
		Month:      req.Month,
		Year:       req.Year,
		Activities: activities,
		Overtimes:  overtimes,
		Holidays:   holidays,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generation failed: " + err.Error()})
		return
	}

	filename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", sanitize(user.Username), req.Month, req.Year)

	companyName := tmpl.Company
	if companyName == "" {
		companyName = user.Company
	}

	// Email a copy asynchronously so the download isn't blocked on SMTP.
	go func(to, comp, fn string, data []byte) {
		_ = s.Mailer.SendTimesheetEmail(to, comp, fn, data)
	}(user.Email, companyName, filename, out)

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", out)
}

// GetHolidays godoc
// @Summary Get monthly Indonesian public holidays
// @Description Returns public holidays for the specified month and year with database cache fallback.
// @Tags Holiday
// @Security BearerAuth
// @Produce json
// @Param year query int false "Year (defaults to current year)"
// @Param month query int false "Month 1-12 (defaults to current month)"
// @Success 200 {array} models.HolidayDTO
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /api/v1/holidays [get]
func (s *Server) GetHolidays(c *gin.Context) {
	now := time.Now().In(jakarta())
	year := queryIntDefault(c, "year", now.Year())
	month := queryIntDefault(c, "month", int(now.Month()))
	holidays, err := services.FetchHolidays(year, month)
	if err == nil && len(holidays) > 0 {
		// Cache fetched holidays into database for resilience
		for _, h := range holidays {
			if t, parseErr := time.Parse("2006-01-02", h.Date); parseErr == nil {
				_ = s.DB.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "date"}},
					DoUpdates: clause.AssignmentColumns([]string{"description", "updated_at"}),
				}).Create(&models.Holiday{
					Date:        t,
					Description: h.Description,
				}).Error
			}
		}
		c.JSON(http.StatusOK, holidays)
		return
	}

	// Fallback to relational database if external API is unreachable
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var dbHolidays []models.Holiday
	if err := s.DB.Where("date >= ? AND date < ?", start, end).Order("date asc").Find(&dbHolidays).Error; err == nil && len(dbHolidays) > 0 {
		resp := make([]map[string]string, 0, len(dbHolidays))
		for _, dh := range dbHolidays {
			resp = append(resp, map[string]string{
				"date":        dh.Date.Format("2006-01-02"),
				"description": dh.Description,
			})
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	c.JSON(http.StatusOK, []models.HolidayDTO{})
}

func queryIntDefault(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		}
	}
	return string(out)
}

// OvertimeRequest carries data to create/update an overtime entry.
type OvertimeRequest struct {
	ID              uint   `json:"id" example:"1"`
	Date            string `json:"date" binding:"required" example:"2026-09-01"` // YYYY-MM-DD
	StartTime       string `json:"start_time" binding:"required" example:"17:00"`
	EndTime         string `json:"end_time" binding:"required" example:"21:00"`
	TaskDescription string `json:"task_description" binding:"required" example:"Production bug fixing and system deployment"`
	TeamLeader      string `json:"team_leader" example:"Team Lead Name"`
	DepartmentHead  string `json:"department_head" example:"Dept Head Name"`
}

// UpsertOvertime godoc
// @Summary Create or update overtime record
// @Description Upserts an overtime entry for the authenticated user for SPL reporting.
// @Tags Overtime
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body handlers.OvertimeRequest true "Overtime entry data"
// @Success 200 {object} models.OvertimeEntry
// @Failure 400 {object} models.ErrorResponse "Invalid payload or date format"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/overtimes [post]
func (s *Server) UpsertOvertime(c *gin.Context) {
	var req OvertimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	date, err := time.ParseInLocation("2006-01-02", req.Date, jakarta())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, expected YYYY-MM-DD"})
		return
	}

	entry := models.OvertimeEntry{
		ID:              req.ID,
		UserID:          currentUserID(c),
		Date:            date,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		TaskDescription: req.TaskDescription,
		TeamLeader:      req.TeamLeader,
		DepartmentHead:  req.DepartmentHead,
	}

	if entry.ID != 0 {
		if err := s.DB.Where("id = ? AND user_id = ?", entry.ID, entry.UserID).Updates(&entry).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := s.DB.Create(&entry).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, entry)
}

// ListMonthlyOvertimes godoc
// @Summary List monthly overtime records
// @Description Retrieves all overtime records for the authenticated user for a specific month and year.
// @Tags Overtime
// @Security BearerAuth
// @Produce json
// @Param year query int false "Year (defaults to current year)"
// @Param month query int false "Month 1-12 (defaults to current month)"
// @Success 200 {array} models.OvertimeEntry
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/overtimes [get]
func (s *Server) ListMonthlyOvertimes(c *gin.Context) {
	year := queryIntDefault(c, "year", time.Now().In(jakarta()).Year())
	month := queryIntDefault(c, "month", int(time.Now().In(jakarta()).Month()))

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, jakarta())
	end := start.AddDate(0, 1, 0)

	var overtimes []models.OvertimeEntry
	if err := s.DB.Where("user_id = ? AND date >= ? AND date < ?", currentUserID(c), start, end).
		Order("date asc").Find(&overtimes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, overtimes)
}

// DeleteOvertime godoc
// @Summary Delete overtime record
// @Description Removes a specific overtime entry belonging to the authenticated user.
// @Tags Overtime
// @Security BearerAuth
// @Produce json
// @Param id path int true "Overtime Entry ID"
// @Success 200 {object} models.DeleteResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/overtimes/{id} [delete]
func (s *Server) DeleteOvertime(c *gin.Context) {
	id := c.Param("id")
	if err := s.DB.Where("id = ? AND user_id = ?", id, currentUserID(c)).Delete(&models.OvertimeEntry{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ListProjects godoc
// @Summary List active projects
// @Description Returns all active projects, optionally filtered by company_id.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param company_id query int false "Company ID filter"
// @Success 200 {array} models.Project
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/projects [get]
func (s *Server) ListProjects(c *gin.Context) {
	var projects []models.Project
	query := s.DB.Where("is_active = ?", true)
	if compID := c.Query("company_id"); compID != "" {
		query = query.Where("company_id = ?", compID)
	}
	if err := query.Preload("Company").Order("name asc").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

// ListCompanies godoc
// @Summary List all companies
// @Description Returns all companies with their associated projects, templates, and departments.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Company
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/companies [get]
func (s *Server) ListCompanies(c *gin.Context) {
	var companies []models.Company
	if err := s.DB.Preload("Projects").Preload("Templates").Preload("Departments").Order("id asc").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, companies)
}

// ListDepartments godoc
// @Summary List active departments
// @Description Returns active departments, optionally filtered by company_id.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param company_id query int false "Company ID filter"
// @Success 200 {array} models.Department
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/departments [get]
func (s *Server) ListDepartments(c *gin.Context) {
	var depts []models.Department
	query := s.DB.Where("is_active = ?", true)
	if compID := c.Query("company_id"); compID != "" {
		query = query.Where("company_id = ?", compID)
	}
	if err := query.Preload("Company").Order("name asc").Find(&depts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, depts)
}

// ListActivityStatuses godoc
// @Summary List activity status options
// @Description Returns all normalized activity status options (e.g. Present, Sick, Vacation).
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.ActivityStatus
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/activity-statuses [get]
func (s *Server) ListActivityStatuses(c *gin.Context) {
	var statuses []models.ActivityStatus
	if err := s.DB.Order("sort_order asc").Find(&statuses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, statuses)
}

// ListHolidays godoc
// @Summary List holidays
// @Description Returns holidays, optionally filtered by year or company_id.
// @Tags Holiday
// @Security BearerAuth
// @Produce json
// @Param company_id query int false "Company ID filter"
// @Param year query int false "Year filter"
// @Success 200 {array} models.Holiday
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/holidays/all [get]
func (s *Server) ListHolidays(c *gin.Context) {
	var holidays []models.Holiday
	query := s.DB.Order("date asc")
	if compID := c.Query("company_id"); compID != "" {
		query = query.Where("company_id = ? OR company_id IS NULL", compID)
	}
	if yearStr := c.Query("year"); yearStr != "" {
		if y, err := time.Parse("2006", yearStr); err == nil {
			start := time.Date(y.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(1, 0, 0)
			query = query.Where("date >= ? AND date < ?", start, end)
		}
	}
	if err := query.Find(&holidays).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, holidays)
}


