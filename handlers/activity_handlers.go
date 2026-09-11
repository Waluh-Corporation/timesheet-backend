package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	date, err := time.ParseInLocation("2006-01-02", req.Date, jakarta())
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	// Default status to 'P' if not provided
	if req.Status == "" {
		req.Status = "P"
	}

	// Validate status against activity_statuses to return 400 instead of foreign key constraint 500
	var statusCount int64
	if err := s.DB.Model(&models.ActivityStatus{}).Where("code = ?", req.Status).Count(&statusCount).Error; err != nil || statusCount == 0 {
		RespondError(c, http.StatusBadRequest, "invalid status code: must be a valid activity status (e.g. P, BT, S, PM, V, X)")
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

	// Pure Relational 3NF: Associate with master Project by ID, code, or name
	// Always synchronize ProjectID (code), ProjectName, and AppImpacted from the canonical Project
	if req.ProjectRefID != nil && *req.ProjectRefID != 0 {
		var proj models.Project
		if err := s.DB.First(&proj, *req.ProjectRefID).Error; err != nil || proj.ID == 0 {
			RespondError(c, http.StatusBadRequest, "invalid project_ref_id: project does not exist")
			return
		}
		activity.ProjectRefID = &proj.ID
		activity.ProjectID = proj.Code
		activity.ProjectName = proj.Name
		activity.AppImpacted = proj.AppImpacted
	} else if req.ProjectID != "" || req.ProjectName != "" {
		var proj models.Project
		query := s.DB.Model(&models.Project{})
		if idNum, err := strconv.Atoi(req.ProjectID); err == nil && idNum > 0 {
			query = query.Where("id = ? OR code = ?", idNum, req.ProjectID)
		} else if req.ProjectID != "" {
			query = query.Where("code = ?", req.ProjectID)
		}
		if req.ProjectName != "" {
			if req.ProjectID != "" {
				query = s.DB.Model(&models.Project{}).Where("(code = ? OR LOWER(name) = LOWER(?))", req.ProjectID, req.ProjectName)
			} else {
				query = query.Where("LOWER(name) = LOWER(?)", req.ProjectName)
			}
		}
		if err := query.Limit(1).Find(&proj).Error; err == nil && proj.ID != 0 {
			activity.ProjectRefID = &proj.ID
			activity.ProjectID = proj.Code
			activity.ProjectName = proj.Name
			activity.AppImpacted = proj.AppImpacted
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
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Preload associations before returning
	_ = s.DB.Preload("ProjectRef").Preload("StatusRef").First(&activity, activity.ID)
	RespondSuccess(c, http.StatusOK, activity)
}

// GetDailyActivity godoc
// @Summary Get daily activity detail
// @Description Retrieves full details of a specific daily activity by ID, including its associated Project and Status.
// @Tags Activity
// @Security BearerAuth
// @Produce json
// @Param id path int true "Daily Activity ID"
// @Success 200 {object} models.DailyActivityDetailResponse
// @Failure 400 {object} models.ErrorResponse "Invalid activity ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Forbidden: not authorized to access another user's activity"
// @Failure 404 {object} models.ErrorResponse "Activity not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/activities/{id} [get]
func (s *Server) GetDailyActivity(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid activity ID, expected positive integer")
		return
	}

	var activity models.DailyActivity
	if err := s.DB.Preload("ProjectRef").Preload("StatusRef").First(&activity, id).Error; err != nil {
		RespondError(c, http.StatusNotFound, "activity not found")
		return
	}

	if activity.UserID != currentUserID(c) {
		RespondError(c, http.StatusForbidden, "you are not authorized to access another user's activity")
		return
	}

	resp := models.DailyActivityDetailResponse{
		ID:          activity.ID,
		CreatedAt:   activity.CreatedAt,
		UpdatedAt:   activity.UpdatedAt,
		UserID:      activity.UserID,
		Date:        activity.Date,
		StartTime:   activity.StartTime,
		EndTime:     activity.EndTime,
		Activity:    activity.Activity,
		ProjectName: activity.ProjectName,
		ProjectID:   activity.ProjectID,
		ProjectRef:  activity.ProjectRef,
		StatusRef:   activity.StatusRef,
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// ListActivities godoc
// @Summary List daily activities with pagination
// @Description Retrieves daily activities for the authenticated user with pagination and optional filtering by year, month, date range, or status.
// @Tags Activity
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 10, max: 100). Use -1 or all=true for all records"
// @Param year query int false "Year filter (e.g. 2026)"
// @Param month query int false "Month filter (1-12)"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Param sort query string false "Sort order: asc or desc (default: desc, or asc when filtering by month)"
// @Success 200 {object} models.PaginatedResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/activities [get]
func (s *Server) ListActivities(c *gin.Context) {
	uid := currentUserID(c)
	query := s.DB.Model(&models.DailyActivity{}).Where("user_id = ?", uid)

	// Filter by exact date range if provided
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if startDate, err := time.ParseInLocation("2006-01-02", startDateStr, jakarta()); err == nil {
			query = query.Where("date >= ?", startDate)
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if endDate, err := time.ParseInLocation("2006-01-02", endDateStr, jakarta()); err == nil {
			query = query.Where("date <= ?", endDate)
		}
	}

	// Filter by year/month if provided
	hasMonth := c.Query("month") != ""
	hasYear := c.Query("year") != ""
	if hasMonth || hasYear {
		year := queryIntDefault(c, "year", time.Now().In(jakarta()).Year())
		if hasMonth {
			month := queryIntDefault(c, "month", int(time.Now().In(jakarta()).Month()))
			start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, jakarta())
			end := start.AddDate(0, 1, 0)
			query = query.Where("date >= ? AND date < ?", start, end)
		} else {
			start := time.Date(year, 1, 1, 0, 0, 0, 0, jakarta())
			end := start.AddDate(1, 0, 0)
			query = query.Where("date >= ? AND date < ?", start, end)
		}
	} else if c.Query("page") == "" && c.Query("limit") == "" && c.Query("start_date") == "" && c.Query("end_date") == "" {
		// Backwards-compatibility: if no filter or pagination params at all, default to current month
		year := time.Now().In(jakarta()).Year()
		month := int(time.Now().In(jakarta()).Month())
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, jakarta())
		end := start.AddDate(0, 1, 0)
		query = query.Where("date >= ? AND date < ?", start, end)
	}

	// Status filter
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// Sorting
	sortOrder := strings.ToLower(c.Query("sort"))
	if sortOrder != "asc" && sortOrder != "desc" {
		if hasMonth || (c.Query("page") == "" && c.Query("limit") == "") {
			sortOrder = "asc"
		} else {
			sortOrder = "desc"
		}
	}

	var totalRows int64
	if err := query.Count(&totalRows).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	page := queryIntDefault(c, "page", 1)
	if page < 1 {
		page = 1
	}

	limit := queryIntDefault(c, "limit", 10)
	isAll := c.Query("all") == "true" || limit == -1

	// If no pagination params given and requesting month view without page/limit, return all for that month
	if c.Query("page") == "" && c.Query("limit") == "" {
		limit = int(totalRows)
		if limit == 0 {
			limit = 10
		}
	}

	activities := make([]models.DailyActivity, 0)
	dataQuery := query.Preload("ProjectRef").Preload("StatusRef").Order("date " + sortOrder)

	if !isAll && limit > 0 {
		if limit > 100 {
			limit = 100
		}
		offset := (page - 1) * limit
		dataQuery = dataQuery.Limit(limit).Offset(offset)
	} else {
		limit = int(totalRows)
		if limit == 0 {
			limit = 1
		}
	}

	if err := dataQuery.Find(&activities).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondPaginated(c, http.StatusOK, activities, page, limit, totalRows)
}

// ListMonthlyActivities delegates to ListActivities for backwards-compatibility.
func (s *Server) ListMonthlyActivities(c *gin.Context) {
	s.ListActivities(c)
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
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := s.DB.First(&user, currentUserID(c)).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	// Resolve the company: relational first (CompanyID), then string fallback.
	var companyCode string
	var companyName string
	if user.CompanyID != nil && *user.CompanyID != 0 {
		var comp models.Company
		if err := s.DB.First(&comp, *user.CompanyID).Error; err == nil {
			companyCode = strings.ToLower(comp.Code)
			companyName = comp.Name
		}
	}
	if companyCode == "" && user.Company != "" {
		companyCode = strings.ToLower(user.Company)
		companyName = user.Company
	}
	if companyCode == "" {
		RespondError(c, http.StatusBadRequest, "user has no company assigned. Ask an admin to assign a company (MII, SDD, NTT, or Adidata) to your account.")
		return
	}

	start := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, jakarta())
	end := start.AddDate(0, 1, 0)
	var activities []models.DailyActivity
	s.DB.Where("user_id = ? AND date >= ? AND date < ?", user.ID, start, end).
		Preload("ProjectRef").Preload("StatusRef").Find(&activities)

	var overtimes []models.OvertimeEntry
	s.DB.Where("user_id = ? AND date >= ? AND date < ?", user.ID, start, end).
		Preload("TeamLeader").Preload("DepartmentHead").
		Order("date asc").Find(&overtimes)

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
		CompanyCode: companyCode,
		User:        &user,
		Month:       req.Month,
		Year:        req.Year,
		Activities:  activities,
		Overtimes:   overtimes,
		Holidays:    holidays,
	})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "generation failed: "+err.Error())
		return
	}

	filename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", sanitize(user.Username), req.Month, req.Year)

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

	// Attempt to fetch and cache entire year from Kemendesa API
	if yearlyHolidays, err := services.FetchHolidaysByYear(year); err == nil && len(yearlyHolidays) > 0 {
		for _, h := range yearlyHolidays {
			if t, parseErr := time.Parse("2006-01-02", h.Date); parseErr == nil {
				_ = s.DB.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "date"}},
					DoUpdates: clause.AssignmentColumns([]string{"description", "is_joint_leave", "is_civic", "is_religious", "updated_at"}),
				}).Create(&models.Holiday{
					Date:         t,
					Description:  h.Description,
					IsJointLeave: h.IsJointLeave,
					IsCivic:      h.IsCivic,
					IsReligious:  h.IsReligious,
				}).Error
			}
		}

		// Filter for the requested month
		prefix := fmt.Sprintf("%04d-%02d-", year, month)
		var monthHolidays []models.HolidayDTO
		for _, h := range yearlyHolidays {
			if strings.HasPrefix(h.Date, prefix) {
				monthHolidays = append(monthHolidays, h)
			}
		}
		RespondSuccess(c, http.StatusOK, monthHolidays)
		return
	}

	// Fallback to relational database if external API is unreachable
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var dbHolidays []models.Holiday
	if err := s.DB.Where("date >= ? AND date < ?", start, end).Order("date asc").Find(&dbHolidays).Error; err == nil && len(dbHolidays) > 0 {
		resp := make([]models.HolidayDTO, 0, len(dbHolidays))
		for _, dh := range dbHolidays {
			resp = append(resp, models.HolidayDTO{
				Date:          dh.Date.Format("2006-01-02"),
				Description:   dh.Description,
				IsJointLeave:  dh.IsJointLeave,
				IsCutiBersama: dh.IsJointLeave,
				IsCivic:       dh.IsCivic,
				IsReligious:   dh.IsReligious,
			})
		}
		RespondSuccess(c, http.StatusOK, resp)
		return
	}

	RespondSuccess(c, http.StatusOK, []models.HolidayDTO{})
}

// SyncHolidays godoc
// @Summary Synchronize holidays from external Kemendesa API to database
// @Description Fetches holidays for a specific year from Kemendesa and upserts into local database.
// @Tags Holiday
// @Security BearerAuth
// @Produce json
// @Param year query int false "Year to sync (defaults to current year)"
// @Success 200 {object} map[string]any
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 502 {object} models.ErrorResponse "Bad gateway"
// @Router /api/v1/holidays/sync [post]
func (s *Server) SyncHolidays(c *gin.Context) {
	now := time.Now().In(jakarta())
	year := queryIntDefault(c, "year", now.Year())

	yearlyHolidays, err := services.FetchHolidaysByYear(year)
	if err != nil {
		RespondError(c, http.StatusBadGateway, fmt.Sprintf("failed to fetch holidays from Kemendesa API: %v", err))
		return
	}
	if len(yearlyHolidays) == 0 {
		RespondSuccess(c, http.StatusOK, gin.H{
			"message": fmt.Sprintf("no holidays found for year %d", year),
			"synced":  0,
			"year":    year,
		})
		return
	}

	syncedCount := 0
	for _, h := range yearlyHolidays {
		if t, parseErr := time.Parse("2006-01-02", h.Date); parseErr == nil {
			err := s.DB.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "date"}},
				DoUpdates: clause.AssignmentColumns([]string{"description", "is_joint_leave", "is_civic", "is_religious", "updated_at"}),
			}).Create(&models.Holiday{
				Date:         t,
				Description:  h.Description,
				IsJointLeave: h.IsJointLeave,
				IsCivic:      h.IsCivic,
				IsReligious:  h.IsReligious,
			}).Error
			if err == nil {
				syncedCount++
			}
		}
	}

	RespondSuccess(c, http.StatusOK, gin.H{
		"message": fmt.Sprintf("successfully synchronized %d holidays for year %d", syncedCount, year),
		"synced":  syncedCount,
		"year":    year,
		"data":    yearlyHolidays,
	})
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
	ID               uint   `json:"id" example:"1"`
	Date             string `json:"date" binding:"required" example:"2026-09-01"` // YYYY-MM-DD
	StartTime        string `json:"start_time" binding:"required" example:"17:00"`
	EndTime          string `json:"end_time" binding:"required" example:"21:00"`
	TaskDescription  string `json:"task_description" binding:"required" example:"Production bug fixing and system deployment"`
	TeamLeaderID     *uint  `json:"team_leader_id" example:"2"`
	DepartmentHeadID *uint  `json:"department_head_id" example:"3"`
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
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	date, err := time.ParseInLocation("2006-01-02", req.Date, jakarta())
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	entry := models.OvertimeEntry{
		ID:               req.ID,
		UserID:           currentUserID(c),
		Date:             date,
		StartTime:        req.StartTime,
		EndTime:          req.EndTime,
		TaskDescription:  req.TaskDescription,
		TeamLeaderID:     req.TeamLeaderID,
		DepartmentHeadID: req.DepartmentHeadID,
	}

	if entry.ID != 0 {
		if err := s.DB.Where("id = ? AND user_id = ?", entry.ID, entry.UserID).Updates(&entry).Error; err != nil {
			RespondError(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		if err := s.DB.Create(&entry).Error; err != nil {
			RespondError(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	_ = s.DB.Preload("TeamLeader").Preload("DepartmentHead").First(&entry, entry.ID)
	RespondSuccess(c, http.StatusOK, entry)
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
		Preload("TeamLeader").Preload("DepartmentHead").
		Order("date asc").Find(&overtimes).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, overtimes)
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
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}

// ListProjects godoc
// @Summary List active projects
// @Description Returns all active projects.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Project
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/projects [get]
func (s *Server) ListProjects(c *gin.Context) {
	var projects []models.Project
	query := s.DB.Where("is_active = ?", true)
	if err := query.Order("name asc").Find(&projects).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, projects)
}

// ListCompanies godoc
// @Summary List all companies
// @Description Returns all companies with their associated departments.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Company
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/companies [get]
func (s *Server) ListCompanies(c *gin.Context) {
	var companies []models.Company
	if err := s.DB.Preload("Departments").Order("id asc").Find(&companies).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, companies)
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
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, depts)
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
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, statuses)
}

// ListHolidays godoc
// @Summary List holidays
// @Description Returns holidays, optionally filtered by year.
// @Tags Holiday
// @Security BearerAuth
// @Produce json
// @Param year query int false "Year filter"
// @Success 200 {array} models.Holiday
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/holidays/all [get]
func (s *Server) ListHolidays(c *gin.Context) {
	var holidays []models.Holiday
	query := s.DB.Order("date asc")
	if yearStr := c.Query("year"); yearStr != "" {
		if y, err := time.Parse("2006", yearStr); err == nil {
			start := time.Date(y.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(1, 0, 0)
			query = query.Where("date >= ? AND date < ?", start, end)
		}
	}
	if err := query.Find(&holidays).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, holidays)
}

// ListApprovers godoc
// @Summary List active approvers
// @Description Retrieves active approvers (Team Leaders, Department Heads), optionally filtered by role_type.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param role_type query string false "Role type filter (team_leader, department_head)"
// @Success 200 {array} models.Approver
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/approvers [get]
func (s *Server) ListApprovers(c *gin.Context) {
	var approvers []models.Approver
	q := s.DB.Where("is_active = ?", true)
	if roleType := c.Query("role_type"); roleType != "" {
		q = q.Where("role_type = ?", roleType)
	}
	if err := q.Order("name asc").Find(&approvers).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, approvers)
}
