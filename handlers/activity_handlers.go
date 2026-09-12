package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/models"
	"timesheet-backend/services"
)

const (
	dateFormatYYYYMMDD = "2006-01-02"
	queryDateRange     = "date >= ? AND date < ?"
	queryUserDateRange = "user_id = ? AND date >= ? AND date < ?"
	orderDateAsc       = "date asc"
	orderNameAsc       = "name asc"
	queryIsActive      = "is_active = ?"
)

// jakarta returns the Asia/Jakarta location, falling back to a fixed +07:00.
func jakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

func (s *Server) validateActivityStatus(status string) bool {
	var statusCount int64
	return s.DB.Model(&models.ActivityStatus{}).Where("code = ?", status).Count(&statusCount).Error == nil && statusCount > 0
}

func (s *Server) findProjectByRefID(refID uint) (*models.Project, error) {
	var proj models.Project
	if err := s.DB.Where(queryID, refID).First(&proj).Error; err != nil || proj.ID == 0 {
		return nil, errors.New("invalid project_ref_id: project does not exist")
	}
	return &proj, nil
}

func (s *Server) buildProjectQuery(projectID, projectName string) *gorm.DB {
	query := s.DB.Model(&models.Project{})
	if idNum, err := strconv.Atoi(projectID); err == nil && idNum > 0 {
		query = query.Where("id = ? OR code = ?", idNum, projectID)
	} else if projectID != "" {
		query = query.Where("code = ?", projectID)
	}
	if projectName != "" {
		if projectID != "" {
			query = s.DB.Model(&models.Project{}).Where("(code = ? OR LOWER(name) = LOWER(?))", projectID, projectName)
		} else {
			query = query.Where("LOWER(name) = LOWER(?)", projectName)
		}
	}
	return query
}

func (s *Server) resolveDailyActivityProject(req *models.DailyActivityRequest, activity *models.DailyActivity) error {
	if req.ProjectRefID != nil && *req.ProjectRefID != 0 {
		proj, err := s.findProjectByRefID(*req.ProjectRefID)
		if err != nil {
			return err
		}
		activity.ProjectRefID = &proj.ID
		activity.ProjectID = proj.Code
		activity.ProjectName = proj.Name
		return nil
	}

	if req.ProjectID == "" && req.ProjectName == "" {
		return nil
	}

	var proj models.Project
	query := s.buildProjectQuery(req.ProjectID, req.ProjectName)
	if err := query.Limit(1).Find(&proj).Error; err == nil && proj.ID != 0 {
		activity.ProjectRefID = &proj.ID
		activity.ProjectID = proj.Code
		activity.ProjectName = proj.Name
	}
	return nil
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
	date, err := time.ParseInLocation(dateFormatYYYYMMDD, req.Date, jakarta())
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	// Default status to 'P' if not provided
	if req.Status == "" {
		req.Status = "P"
	}

	// Validate status against activity_statuses to return 400 instead of foreign key constraint 500
	if !s.validateActivityStatus(req.Status) {
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
		ProjectRefID: req.ProjectRefID,
	}

	if err := s.resolveDailyActivityProject(&req, &activity); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Upsert on the (user_id, date) unique index.
	err = s.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"start_time", "end_time", "status", "activity",
			"project_name", "project_id", "project_ref_id", "updated_at",
		}),
	}).Create(&activity).Error
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Preload associations before returning
	_ = s.DB.Preload("ProjectRef").Preload("StatusRef").Where(queryID, activity.ID).First(&activity)
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
	if err := s.DB.Preload("ProjectRef").Preload("StatusRef").Where(queryID, id).First(&activity).Error; err != nil {
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
func applyActivityDateFilters(c *gin.Context, query *gorm.DB) *gorm.DB {
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if startDate, err := time.ParseInLocation(dateFormatYYYYMMDD, startDateStr, jakarta()); err == nil {
			query = query.Where("date >= ?", startDate)
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if endDate, err := time.ParseInLocation(dateFormatYYYYMMDD, endDateStr, jakarta()); err == nil {
			query = query.Where("date <= ?", endDate)
		}
	}

	hasMonth := c.Query("month") != ""
	hasYear := c.Query("year") != ""
	if hasMonth || hasYear {
		year := queryIntDefault(c, "year", time.Now().In(jakarta()).Year())
		if hasMonth {
			month := queryIntDefault(c, "month", int(time.Now().In(jakarta()).Month()))
			start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, jakarta())
			end := start.AddDate(0, 1, 0)
			query = query.Where(queryDateRange, start, end)
		} else {
			start := time.Date(year, 1, 1, 0, 0, 0, 0, jakarta())
			end := start.AddDate(1, 0, 0)
			query = query.Where(queryDateRange, start, end)
		}
	} else if c.Query("page") == "" && c.Query("limit") == "" && c.Query("start_date") == "" && c.Query("end_date") == "" {
		year := time.Now().In(jakarta()).Year()
		month := int(time.Now().In(jakarta()).Month())
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, jakarta())
		end := start.AddDate(0, 1, 0)
		query = query.Where(queryDateRange, start, end)
	}

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	return query
}

func determineActivitySortOrder(c *gin.Context) string {
	sortOrder := strings.ToLower(c.Query("sort"))
	if sortOrder == "asc" || sortOrder == "desc" {
		return sortOrder
	}
	if c.Query("month") != "" || (c.Query("page") == "" && c.Query("limit") == "") {
		return "asc"
	}
	return "desc"
}

func calculateActivityPagination(c *gin.Context, totalRows int64) (int, int, bool) {
	page := queryIntDefault(c, "page", 1)
	if page < 1 {
		page = 1
	}

	limit := queryIntDefault(c, "limit", 10)
	isAll := c.Query("all") == "true" || limit == -1

	if c.Query("page") == "" && c.Query("limit") == "" {
		limit = int(totalRows)
		if limit == 0 {
			limit = 10
		}
	}
	return page, limit, isAll
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
	query = applyActivityDateFilters(c, query)

	sortOrder := determineActivitySortOrder(c)

	var totalRows int64
	if err := query.Count(&totalRows).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	page, limit, isAll := calculateActivityPagination(c, totalRows)

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
	if err := s.DB.Where(queryID, currentUserID(c)).First(&user).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	// Resolve the company: relational first (CompanyID), then string fallback.
	var companyCode string
	var companyName string
	if user.CompanyID != nil && *user.CompanyID != 0 {
		var comp models.Company
		if err := s.DB.Where(queryID, *user.CompanyID).First(&comp).Error; err == nil {
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
	s.DB.Where(queryUserDateRange, user.ID, start, end).
		Preload("ProjectRef").Preload("StatusRef").Find(&activities)

	var overtimes []models.OvertimeEntry
	s.DB.Where(queryUserDateRange, user.ID, start, end).
		Preload("TeamLeader").Preload("DepartmentHead").
		Order(orderDateAsc).Find(&overtimes)

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
		if s.Mailer != nil {
			_ = s.Mailer.SendTimesheetEmail(to, comp, fn, data)
		}
	}(user.Email, companyName, filename, out)

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", out)
}

func saveYearlyHolidays(db *gorm.DB, yearlyHolidays []models.HolidayDTO) int {
	syncedCount := 0
	for _, h := range yearlyHolidays {
		if t, parseErr := time.Parse(dateFormatYYYYMMDD, h.Date); parseErr == nil {
			err := db.Clauses(clause.OnConflict{
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
	return syncedCount
}

func filterHolidaysByMonth(yearlyHolidays []models.HolidayDTO, year, month int) []models.HolidayDTO {
	prefix := fmt.Sprintf("%04d-%02d-", year, month)
	var monthHolidays []models.HolidayDTO
	for _, h := range yearlyHolidays {
		if strings.HasPrefix(h.Date, prefix) {
			monthHolidays = append(monthHolidays, h)
		}
	}
	return monthHolidays
}

func fetchDBHolidays(db *gorm.DB, year, month int) []models.HolidayDTO {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var dbHolidays []models.Holiday
	if err := db.Where(queryDateRange, start, end).Order(orderDateAsc).Find(&dbHolidays).Error; err == nil && len(dbHolidays) > 0 {
		resp := make([]models.HolidayDTO, 0, len(dbHolidays))
		for _, dh := range dbHolidays {
			resp = append(resp, models.HolidayDTO{
				Date:          dh.Date.Format(dateFormatYYYYMMDD),
				Description:   dh.Description,
				IsJointLeave:  dh.IsJointLeave,
				IsCutiBersama: dh.IsJointLeave,
				IsCivic:       dh.IsCivic,
				IsReligious:   dh.IsReligious,
			})
		}
		return resp
	}
	return nil
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
		saveYearlyHolidays(s.DB, yearlyHolidays)
		monthHolidays := filterHolidaysByMonth(yearlyHolidays, year, month)
		RespondSuccess(c, http.StatusOK, monthHolidays)
		return
	}

	// Fallback to relational database if external API is unreachable
	if dbHolidays := fetchDBHolidays(s.DB, year, month); len(dbHolidays) > 0 {
		RespondSuccess(c, http.StatusOK, dbHolidays)
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

	syncedCount := saveYearlyHolidays(s.DB, yearlyHolidays)

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
	date, err := time.ParseInLocation(dateFormatYYYYMMDD, req.Date, jakarta())
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	uid := currentUserID(c)
	var entry models.OvertimeEntry

	if req.ID != 0 {
		if err := s.DB.Where("id = ? AND user_id = ?", req.ID, uid).First(&entry).Error; err != nil {
			RespondError(c, http.StatusNotFound, "overtime entry not found")
			return
		}
		entry.Date = date
		entry.StartTime = req.StartTime
		entry.EndTime = req.EndTime
		entry.TaskDescription = req.TaskDescription
		entry.TeamLeaderID = req.TeamLeaderID
		entry.DepartmentHeadID = req.DepartmentHeadID
		if err := s.DB.Save(&entry).Error; err != nil {
			RespondError(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		entry = models.OvertimeEntry{
			UserID:           uid,
			Date:             date,
			StartTime:        req.StartTime,
			EndTime:          req.EndTime,
			TaskDescription:  req.TaskDescription,
			TeamLeaderID:     req.TeamLeaderID,
			DepartmentHeadID: req.DepartmentHeadID,
		}
		if err := s.DB.Create(&entry).Error; err != nil {
			RespondError(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	_ = s.DB.Preload("TeamLeader").Preload("DepartmentHead").Where(queryID, entry.ID).First(&entry)
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
	if err := s.DB.Where(queryUserDateRange, currentUserID(c), start, end).
		Preload("TeamLeader").Preload("DepartmentHead").
		Order(orderDateAsc).Find(&overtimes).Error; err != nil {
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
	query := s.DB.Where(queryIsActive, true)
	if err := query.Order(orderNameAsc).Find(&projects).Error; err != nil {
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
	query := s.DB.Where(queryIsActive, true)
	if compID := c.Query("company_id"); compID != "" {
		query = query.Where("company_id = ?", compID)
	}
	if err := query.Preload("Company").Order(orderNameAsc).Find(&depts).Error; err != nil {
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
	query := s.DB.Order(orderDateAsc)
	if yearStr := c.Query("year"); yearStr != "" {
		if y, err := time.Parse("2006", yearStr); err == nil {
			start := time.Date(y.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(1, 0, 0)
			query = query.Where(queryDateRange, start, end)
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
	q := s.DB.Where(queryIsActive, true)
	if roleType := c.Query("role_type"); roleType != "" {
		q = q.Where("role_type = ?", roleType)
	}
	if err := q.Order(orderNameAsc).Find(&approvers).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, approvers)
}
