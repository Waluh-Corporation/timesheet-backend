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

// dailyActivityRequest is a single day's entry from the daily modal or grid.
type dailyActivityRequest struct {
	Date        string `json:"date" binding:"required"` // YYYY-MM-DD
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Status      string `json:"status"`
	Activity    string `json:"activity"`
	ProjectName string `json:"project_name"`
	ProjectID   string `json:"project_id"`
	AppImpacted string `json:"app_impacted"`
}

// UpsertDailyActivity creates or updates the current user's entry for one day.
func (s *Server) UpsertDailyActivity(c *gin.Context) {
	var req dailyActivityRequest
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
		UserID:      currentUserID(c),
		Date:        date,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Status:      req.Status,
		Activity:    req.Activity,
		ProjectName: req.ProjectName,
		ProjectID:   req.ProjectID,
		AppImpacted: req.AppImpacted,
	}

	// Associate with normalized Project if matched by code or name
	if req.ProjectID != "" || req.ProjectName != "" {
		var proj models.Project
		if err := s.DB.Where("code = ? OR LOWER(name) = LOWER(?)", req.ProjectID, req.ProjectName).First(&proj).Error; err == nil {
			activity.ProjectRefID = &proj.ID
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
	c.JSON(http.StatusOK, activity)
}

// ListMonthlyActivities returns the current user's entries for a month.
func (s *Server) ListMonthlyActivities(c *gin.Context) {
	year := queryIntDefault(c, "year", time.Now().In(jakarta()).Year())
	month := queryIntDefault(c, "month", int(time.Now().In(jakarta()).Month()))

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, jakarta())
	end := start.AddDate(0, 1, 0)

	var activities []models.DailyActivity
	if err := s.DB.Where("user_id = ? AND date >= ? AND date < ?", currentUserID(c), start, end).
		Order("date asc").Find(&activities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, activities)
}

// generateRequest selects the template, month and year to render.
type generateRequest struct {
	TemplateID uint `json:"template_id"`
	Month      int  `json:"month" binding:"required,min=1,max=12"`
	Year       int  `json:"year" binding:"required,min=2000,max=9999"`
}

// GenerateTimesheet renders the user's month into the mapped template, streams
// the .xlsx back for download, and emails a copy to the user (requirement 5).
func (s *Server) GenerateTimesheet(c *gin.Context) {
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := s.DB.First(&user, currentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Resolve the template: explicit id, or strictly by user's assigned company.
	var tmpl models.Template
	if req.TemplateID != 0 {
		if err := s.DB.Preload("CellMappings").Where("id = ?", req.TemplateID).First(&tmpl).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "template not found"})
			return
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

// GetHolidays returns Indonesian public holidays for a month so the frontend
// grid can gray out and label weekends/holidays.
func (s *Server) GetHolidays(c *gin.Context) {
	now := time.Now().In(jakarta())
	year := queryIntDefault(c, "year", now.Year())
	month := queryIntDefault(c, "month", int(now.Month()))
	holidays, err := services.FetchHolidays(year, month)
	if err != nil {
		// Non-fatal: return an empty set so the grid still renders.
		c.JSON(http.StatusOK, []models.Holiday{})
		return
	}
	c.JSON(http.StatusOK, holidays)
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
	ID              uint   `json:"id"`
	Date            string `json:"date" binding:"required"` // YYYY-MM-DD
	StartTime       string `json:"start_time" binding:"required"`
	EndTime         string `json:"end_time" binding:"required"`
	TaskDescription string `json:"task_description" binding:"required"`
	TeamLeader      string `json:"team_leader"`
	DepartmentHead  string `json:"department_head"`
}

// UpsertOvertime creates or updates an overtime entry for the authenticated user.
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

// ListMonthlyOvertimes returns the current user's overtime records for a month.
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

// DeleteOvertime deletes an overtime entry by ID.
func (s *Server) DeleteOvertime(c *gin.Context) {
	id := c.Param("id")
	if err := s.DB.Where("id = ? AND user_id = ?", id, currentUserID(c)).Delete(&models.OvertimeEntry{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ListProjects returns all active projects, optionally filtered by company_id.
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

// ListCompanies returns all companies and their associated templates and projects.
func (s *Server) ListCompanies(c *gin.Context) {
	var companies []models.Company
	if err := s.DB.Preload("Projects").Preload("Templates").Order("id asc").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, companies)
}


