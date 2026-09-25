package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
	"timesheet-backend/services"
)

const (
	dateFormatYYYYMMDD               = "2006-01-02"
	queryDateRange                   = "date >= ? AND date < ?"
	queryUserDateRange               = "user_id = ? AND date >= ? AND date < ?"
	orderDateAsc                     = "date asc"
	orderNameAsc                     = "name asc"
	orderIDAsc                       = "id asc"
	queryIsActive                    = "is_active = ?"
	errActivityServiceNotInitialized = "activity service not initialized"
)

// jakarta returns the Asia/Jakarta location, falling back to a fixed +07:00.
func jakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

func (s *Server) findProjectByRefID(refID uint) (*models.Project, error) {
	var proj models.Project
	if err := s.DB.Scopes(models.ActiveOnly).Where(queryID, refID).First(&proj).Error; err != nil || proj.ID == 0 {
		return nil, errors.New("invalid project_ref_id: project does not exist or is inactive")
	}
	return &proj, nil
}

func (s *Server) buildProjectQuery(projectID, projectName string) *gorm.DB {
	query := s.DB.Model(&models.Project{}).Scopes(models.ActiveOnly)
	if idNum, err := strconv.Atoi(projectID); err == nil && idNum > 0 {
		query = query.Where("id = ? OR code = ?", idNum, projectID)
	} else if projectID != "" {
		query = query.Where("code = ?", projectID)
	}
	if projectName != "" {
		if projectID != "" {
			query = s.DB.Model(&models.Project{}).Scopes(models.ActiveOnly).Where("(code = ? OR LOWER(name) = LOWER(?))", projectID, projectName)
		} else {
			query = query.Where("LOWER(name) = LOWER(?)", projectName)
		}
	}
	return query
}

func reqContext(c *gin.Context) context.Context {
	if c != nil && c.Request != nil && c.Request.Context() != nil {
		return c.Request.Context()
	}
	return context.Background()
}

func (s *Server) getActivityService() service.ActivityService {
	if s.ActivitySvc != nil {
		return s.ActivitySvc
	}
	if s.DB != nil {
		return service.NewActivityService(repository.NewActivityRepository(s.DB))
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
// @Param request body request.DailyActivityRequest true "Daily activity payload"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid date format or payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/activities [post]
func (s *Server) UpsertDailyActivity(c *gin.Context) {
	var req request.DailyActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	svc := s.getActivityService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errActivityServiceNotInitialized)
		return
	}

	if err := svc.UpsertDailyActivity(reqContext(c), currentUserID(c), &req); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			msg := strings.TrimPrefix(err.Error(), domain.ErrInvalidInput.Error()+": ")
			RespondError(c, http.StatusBadRequest, msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "activity saved successfully")
}

// GetDailyActivity godoc
// @Summary Get daily activity detail
// @Description Retrieves details of a specific daily activity by ID.
// @Tags Activity
// @Security BearerAuth
// @Produce json
// @Param id path int true "Daily Activity ID"
// @Success 200 {object} response.DailyActivityResponse
// @Failure 400 {object} response.ErrorResponse "Invalid activity ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden: not authorized to access another user's activity"
// @Failure 404 {object} response.ErrorResponse "Activity not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/activities/{id} [get]
func (s *Server) GetDailyActivity(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid activity ID, expected positive integer")
		return
	}

	svc := s.getActivityService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errActivityServiceNotInitialized)
		return
	}

	resp, err := svc.GetDailyActivity(reqContext(c), currentUserID(c), uint(id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "activity not found")
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(c, http.StatusForbidden, "you are not authorized to access another user's activity")
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, *resp)
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

func parseDateQuery(c *gin.Context, param string) *time.Time {
	val := c.Query(param)
	if val == "" {
		return nil
	}
	t, err := time.ParseInLocation(dateFormatYYYYMMDD, val, jakarta())
	if err != nil {
		return nil
	}
	return &t
}

func parsePeriodFilter(c *gin.Context, filter *repository.ActivityFilter) {
	hasMonth := c.Query("month") != ""
	hasYear := c.Query("year") != ""
	if hasMonth || hasYear {
		year := queryIntDefault(c, "year", time.Now().In(jakarta()).Year())
		filter.Year = &year
		if hasMonth {
			month := queryIntDefault(c, "month", int(time.Now().In(jakarta()).Month()))
			filter.Month = &month
		}
		return
	}
	if c.Query("page") == "" && c.Query("limit") == "" && c.Query("start_date") == "" && c.Query("end_date") == "" {
		filter.IsCurrentMonthDefault = true
	}
}

func parsePaginationFilter(c *gin.Context, filter *repository.ActivityFilter) {
	page := queryIntDefault(c, "page", 1)
	if page < 1 {
		page = 1
	}

	limit := queryIntDefault(c, "limit", 10)
	filter.IsAll = c.Query("all") == "true" || limit == -1
	filter.Page = page
	filter.Limit = limit

	if c.Query("page") == "" && c.Query("limit") == "" {
		filter.Limit = -1
		filter.IsAll = true
	}
}

func parseActivityFilter(c *gin.Context) repository.ActivityFilter {
	var filter repository.ActivityFilter
	filter.StartDate = parseDateQuery(c, "start_date")
	filter.EndDate = parseDateQuery(c, "end_date")
	parsePeriodFilter(c, &filter)
	filter.Status = c.Query("status")
	filter.SortOrder = determineActivitySortOrder(c)
	parsePaginationFilter(c, &filter)
	return filter
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
// @Success 200 {object} response.PaginatedResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/activities [get]
func (s *Server) ListActivities(c *gin.Context) {
	uid := currentUserID(c)
	svc := s.getActivityService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errActivityServiceNotInitialized)
		return
	}

	filter := parseActivityFilter(c)

	respItems, meta, err := svc.ListActivities(reqContext(c), uid, filter)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondPaginated(c, http.StatusOK, respItems, meta.Page, meta.Limit, meta.TotalRows)
}

// ListMonthlyActivities delegates to ListActivities for backwards-compatibility.
func (s *Server) ListMonthlyActivities(c *gin.Context) {
	s.ListActivities(c)
}

// GenerateTimesheet godoc
// @Summary Generate timesheet spreadsheet asynchronously
// @Description Evaluates cache for valid active timesheet and re-issues a download token, or enqueues a new generation task. Returns 200 OK on cache hit, or 202 Accepted when queued.
// @Tags Timesheet
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.GenerateRequest true "Generation parameters"
// @Success 200 {object} response.TimesheetJobResponse "Timesheet retrieved from cache and download link re-issued"
// @Success 202 {object} response.TimesheetJobResponse "Generation task accepted"
// @Failure 400 {object} response.ErrorResponse "Template or company mapping missing"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 409 {object} response.ErrorResponse "Generation request in-flight or debounce active"
// @Failure 500 {object} response.ErrorResponse "Generation failed"
// @Router /api/v1/timesheet/generate [post]
func (s *Server) GenerateTimesheet(c *gin.Context) {
	var req request.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Month < 1 || req.Month > 12 || req.Year < 2000 || req.Year > 2100 {
		RespondError(c, http.StatusBadRequest, "Invalid month or year")
		return
	}

	uid := currentUserID(c)
	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "user repository not initialized")
		return
	}

	user, err := userRepo.FindByIDWithDetails(reqContext(c), uid)
	if err != nil || user == nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	hasCompany := false
	if user.CompanyRel != nil && user.CompanyRel.Code != "" {
		hasCompany = true
	} else if user.CompanyID != nil && *user.CompanyID != 0 {
		hasCompany = true
	} else if strings.TrimSpace(user.Company) != "" {
		hasCompany = true
	}
	if !hasCompany {
		RespondError(c, http.StatusBadRequest, "User has no company assigned")
		return
	}

	actRepo := s.getActivityRepository()
	if actRepo != nil {
		m := req.Month
		y := req.Year
		filter := repository.ActivityFilter{
			Month: &m,
			Year:  &y,
			IsAll: true,
		}
		activities, _, aerr := actRepo.ListActiveByUser(reqContext(c), uid, filter)
		if aerr != nil {
			RespondError(c, http.StatusInternalServerError, "failed to check activities: "+aerr.Error())
			return
		}
		if len(activities) == 0 {
			RespondError(c, http.StatusBadRequest, "No activities recorded for this period")
			return
		}
	}

	jobRepo := s.getJobRepo()
	queueClient := s.getQueueClient()

	if queueClient != nil && jobRepo != nil {
		// 1. Distributed debouncing lock via Redis (Edge Case 3)
		rdb := s.getRedisClient()
		lockKey := fmt.Sprintf("timesheet:lock:%d:%d:%02d", uid, req.Year, req.Month)
		if rdb != nil {
			acquired, lerr := rdb.SetNX(reqContext(c), lockKey, "locked", 10*time.Second).Result()
			if lerr == nil && !acquired {
				RespondError(c, http.StatusConflict, "A generation request for this period is currently being processed. Please wait a moment.")
				return
			}
		}

		// 2. Check if there's already an in-flight job (queued or processing)
		if inFlight, err := jobRepo.HasInFlightJob(reqContext(c), uid, req.Month, req.Year); err == nil && inFlight {
			RespondError(c, http.StatusConflict, "A generation request for this period is already queued or processing. Please check your email or wait a moment.")
			return
		}

		// 3. Autonomous Cache Evaluation & Re-issue (Option A)
		activeJob, err := jobRepo.FindActiveCompletedJob(reqContext(c), uid, req.Month, req.Year)
		if err == nil && activeJob != nil && activeJob.FileKey != "" {
			isCacheValid := true

			// Check if activities were modified after job was created
			if actRepo != nil {
				latestActUpdate, aerr := actRepo.GetLatestActivityUpdateTime(reqContext(c), uid, req.Month, req.Year)
				if aerr == nil && !latestActUpdate.IsZero() && latestActUpdate.After(activeJob.CreatedAt) {
					isCacheValid = false
				}
			}

			// Check if user profile was modified after job was created
			if isCacheValid && user.UpdatedAt.After(activeJob.CreatedAt) {
				isCacheValid = false
			}

			// Check if physical file exists in storage
			if isCacheValid {
				storageSvc := s.getStorage()
				if storageSvc != nil {
					exists, serr := storageSvc.FileExists(reqContext(c), activeJob.FileKey)
					if serr != nil || !exists {
						isCacheValid = false
					}
				}
			}

			if isCacheValid {
				// Autonomous Re-issue without regenerating XLSX or duplicating in S3
				newToken := uuid.New().String()
				maxDownloads := 3
				if s.Cfg != nil && s.Cfg.TimesheetDownloadMaxQuota > 0 {
					maxDownloads = s.Cfg.TimesheetDownloadMaxQuota
				}
				retentionDays := 7
				if s.Cfg != nil && s.Cfg.S3RetentionDays > 0 {
					retentionDays = s.Cfg.S3RetentionDays
				}
				tokenExpiry := time.Now().Add(time.Duration(retentionDays) * 24 * time.Hour)

				if rerr := jobRepo.RenewDownloadToken(reqContext(c), activeJob.ID, newToken, maxDownloads, tokenExpiry); rerr == nil {
					// Initialize fast-path counter in Redis
					if rdb != nil {
						tokenKey := fmt.Sprintf("timesheet:token:%s:count", newToken)
						_ = rdb.Set(reqContext(c), tokenKey, 0, time.Duration(retentionDays)*24*time.Hour).Err()
					}

					baseURL := s.publicBaseURL(c)
					if s.Cfg != nil && s.Cfg.AppBaseURL != "" {
						baseURL = s.Cfg.AppBaseURL
					}
					downloadURL := fmt.Sprintf("%s/api/v1/timesheet/downloads/%s", baseURL, newToken)

					// Dispatch email notification asynchronously
					if s.Mailer != nil && user.Email != "" {
						compName := ""
						if user.CompanyRel != nil {
							compName = user.CompanyRel.Name
						} else {
							compName = user.Company
						}
						period := mailer.FormatMonthYearIndonesian(req.Month, req.Year)
						filename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", user.Username, req.Month, req.Year)
						toEmail := user.Email
						toUsername := user.Username
						m := s.Mailer
						go func() {
							if merr := m.SendTimesheetReadyEmail(toEmail, toUsername, compName, period, filename, downloadURL, tokenExpiry); merr != nil {
								slog.Error("failed to send re-issued timesheet ready email", "error", merr, "email", toEmail)
							}
						}()
					}

					// Dispatch Web Push notification
					if s.Push != nil {
						pushTitle := "Timesheet Telah Siap"
						pushBody := fmt.Sprintf("Timesheet periode %02d/%04d Anda telah siap diunduh.", req.Month, req.Year)
						s.Push.SendToUser(uid, push.Payload{
							Title: pushTitle,
							Body:  pushBody,
							URL:   downloadURL,
						})
					}

					activeJob.DownloadURL = downloadURL
					activeJob.DownloadToken = &newToken
					activeJob.DownloadCount = 0
					activeJob.MaxDownloads = maxDownloads
					activeJob.TokenExpiresAt = &tokenExpiry

					c.JSON(http.StatusOK, gin.H{
						"code":    http.StatusOK,
						"status":  "success",
						"message": "Timesheet retrieved from cache and new download link re-issued. Please check your email.",
						"data":    response.ToTimesheetJobResponse(activeJob),
					})
					return
				}
			}
		}

		// 4. Cache Miss or Invalidation: Enqueue new generation job
		jobID := uuid.New().String()
		job := &models.TimesheetJob{
			ID:        jobID,
			UserID:    uid,
			Month:     req.Month,
			Year:      req.Year,
			Status:    models.JobStatusQueued,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := jobRepo.Create(reqContext(c), job); err != nil {
			RespondError(c, http.StatusInternalServerError, "failed to create generation job: "+err.Error())
			return
		}

		if err := queueClient.EnqueueTimesheetJob(reqContext(c), jobID, uid, req.Month, req.Year); err != nil {
			_ = jobRepo.UpdateStatus(reqContext(c), jobID, models.JobStatusFailed, "", "", "failed to enqueue task: "+err.Error(), nil)
			RespondError(c, http.StatusInternalServerError, "failed to enqueue generation task: "+err.Error())
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"code":    http.StatusAccepted,
			"status":  "success",
			"message": "Timesheet generation job queued successfully. The download link will be sent to your email once ready.",
			"data":    response.ToTimesheetJobResponse(job),
		})
		return
	}

	// Fallback when queue is not configured
	tsSvc := s.getTimesheetService()
	if tsSvc == nil {
		RespondError(c, http.StatusInternalServerError, "timesheet service not initialized")
		return
	}

	out, filename, err := tsSvc.GenerateWorkbook(reqContext(c), uid, req.Month, req.Year)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "generation failed: "+err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", out)
}

// GetTimesheetJob godoc
// @Summary Get timesheet generation job status
// @Description Checks status of an asynchronous timesheet generation job.
// @Tags Timesheet
// @Security BearerAuth
// @Produce json
// @Param id path string true "Job ID (UUID)"
// @Success 200 {object} response.TimesheetJobResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Job not found"
// @Router /api/v1/timesheet/jobs/{id} [get]
func (s *Server) GetTimesheetJob(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		RespondError(c, http.StatusBadRequest, "job id is required")
		return
	}

	jobRepo := s.getJobRepo()
	if jobRepo == nil {
		RespondError(c, http.StatusInternalServerError, "job repository not initialized")
		return
	}

	job, err := jobRepo.FindActiveByIDAndUser(reqContext(c), id, currentUserID(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to query job: "+err.Error())
		return
	}
	if job == nil {
		RespondError(c, http.StatusNotFound, "job not found")
		return
	}

	RespondSuccess(c, http.StatusOK, response.ToTimesheetJobResponse(job))
}

// ListTimesheetJobs godoc
// @Summary List user's timesheet generation jobs
// @Description Returns recent timesheet generation jobs for the authenticated user.
// @Tags Timesheet
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Page size (default 10)"
// @Success 200 {object} response.PaginatedResponse
// @Router /api/v1/timesheet/jobs [get]
func (s *Server) ListTimesheetJobs(c *gin.Context) {
	jobRepo := s.getJobRepo()
	if jobRepo == nil {
		RespondError(c, http.StatusInternalServerError, "job repository not initialized")
		return
	}

	page := queryIntDefault(c, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := queryIntDefault(c, "limit", 10)
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	jobs, total, err := jobRepo.ListByUser(reqContext(c), currentUserID(c), limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to list jobs: "+err.Error())
		return
	}

	resps := make([]response.TimesheetJobResponse, 0, len(jobs))
	for i := range jobs {
		resps = append(resps, response.ToTimesheetJobResponse(&jobs[i]))
	}

	RespondPaginated(c, http.StatusOK, resps, page, limit, total)
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
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
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
// @Success 200 {object} response.MessageResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 502 {object} response.ErrorResponse "Bad gateway"
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
		RespondMessage(c, http.StatusOK, fmt.Sprintf("no holidays found for year %d", year))
		return
	}

	syncedCount := saveYearlyHolidays(s.DB, yearlyHolidays)

	RespondMessage(c, http.StatusOK, fmt.Sprintf("successfully synchronized %d holidays for year %d", syncedCount, year))
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

func parseActiveFilter(c *gin.Context) *bool {
	if role, _ := c.Get(ctxRole); role == models.RoleAdmin {
		return nil
	}
	t := true
	return &t
}

// ListProjects godoc
// @Summary List active projects
// @Description Returns all active projects.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Project
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/projects [get]
func (s *Server) ListProjects(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	projects, err := svc.ListProjects(reqContext(c), true)
	if err != nil {
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
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/companies [get]
func (s *Server) ListCompanies(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	companies, err := svc.ListCompanies(reqContext(c), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, companies)
}

// ListSites godoc
// @Summary List all sites
// @Description Returns all registered office/placement sites.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Site
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/sites [get]
func (s *Server) ListSites(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	sites, err := svc.ListSites(reqContext(c), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, sites)
}

// ListDivisions godoc
// @Summary List all divisions
// @Description Returns all organizational divisions.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Division
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/divisions [get]
func (s *Server) ListDivisions(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	divisions, err := svc.ListDivisions(reqContext(c), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, divisions)
}

// ListDepartments godoc
// @Summary List active departments
// @Description Returns departments, optionally filtered by division, division_id, or is_active.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param division query string false "Division name filter"
// @Param division_id query int false "Division ID filter"
// @Success 200 {array} models.Department
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/departments [get]
func (s *Server) ListDepartments(c *gin.Context) {
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
	depts, err := svc.ListDepartments(reqContext(c), divID, divName, parseActiveFilter(c))
	if err != nil {
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
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/activity-statuses [get]
func (s *Server) ListActivityStatuses(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	statuses, err := svc.ListActivityStatuses(reqContext(c))
	if err != nil {
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
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
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
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/approvers [get]
func (s *Server) ListApprovers(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	approvers, err := svc.ListApprovers(reqContext(c), c.Query("role_type"), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, approvers)
}

// DownloadTimesheetByToken godoc
// @Summary Download generated timesheet via secure token
// @Description Validates magic download token, enforces download limits, and redirects directly to S3 storage. Supports HEAD for link verifiers.
// @Tags Timesheet
// @Produce octet-stream
// @Param token path string true "Magic Download Token"
// @Success 200 "Link verified successfully (HEAD request)"
// @Success 302 {string} string "Redirects directly to S3 Presigned URL"
// @Failure 400 {object} response.ErrorResponse "Invalid download token"
// @Failure 404 {object} response.ErrorResponse "Download token not found"
// @Failure 410 {object} response.ErrorResponse "Download token expired or quota exceeded"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/timesheet/downloads/{token} [get]
func (s *Server) DownloadTimesheetByToken(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" || len(token) < 10 {
		RespondError(c, http.StatusBadRequest, "Invalid download token")
		return
	}

	ctx := reqContext(c)
	rdb := s.getRedisClient()

	jobRepo := s.getJobRepo()
	if jobRepo == nil {
		RespondError(c, http.StatusInternalServerError, "Job repository not initialized")
		return
	}

	// Step 1: Query job by download token
	job, err := jobRepo.FindByDownloadToken(ctx, token)
	if err != nil || job == nil {
		RespondError(c, http.StatusNotFound, "Download token not found or invalid")
		return
	}

	// Step 2: Check token expiration
	if job.TokenExpiresAt != nil && job.TokenExpiresAt.Before(time.Now()) {
		RespondError(c, http.StatusGone, "Download token has expired. Please generate a new timesheet to get an updated link.")
		return
	}

	// Step 3: Check download quota (max downloads)
	maxDownloads := job.MaxDownloads
	if maxDownloads <= 0 {
		if s.Cfg != nil && s.Cfg.TimesheetDownloadMaxQuota > 0 {
			maxDownloads = s.Cfg.TimesheetDownloadMaxQuota
		} else {
			maxDownloads = 3
		}
	}
	if job.DownloadCount >= maxDownloads {
		RespondError(c, http.StatusGone, "Download quota exceeded. Please generate a new timesheet to get an updated link.")
		return
	}

	// Step 4: Handle HEAD request from link scanners (SES tracking, email clients)
	// Return 200 OK without incrementing counter or consuming download quota
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Status(http.StatusOK)
		return
	}

	// Step 5: Fast-path atomic counter in Redis
	tokenKey := fmt.Sprintf("timesheet:token:%s:count", token)
	if rdb != nil {
		// Sync with DB count if key doesn't exist
		exists, _ := rdb.Exists(ctx, tokenKey).Result()
		if exists == 0 {
			remainingTTL := 7 * 24 * time.Hour
			if job.TokenExpiresAt != nil && job.TokenExpiresAt.After(time.Now()) {
				remainingTTL = time.Until(*job.TokenExpiresAt)
			}
			_ = rdb.Set(ctx, tokenKey, job.DownloadCount, remainingTTL).Err()
		}

		newCount, err := rdb.Incr(ctx, tokenKey).Result()
		if err == nil && newCount > int64(maxDownloads) {
			RespondError(c, http.StatusGone, "Download quota exceeded. Please generate a new timesheet to get an updated link.")
			return
		}
	}

	// Step 6: Increment download count in PostgreSQL
	if err := jobRepo.IncrementDownloadCount(ctx, job.ID); err != nil {
		slog.Error("failed to increment download count in db", "error", err, "job_id", job.ID)
	}

	// Step 7: Generate temporary Presigned S3 URL valid for 15 minutes with attachment filename
	storageSvc := s.getStorage()
	if storageSvc == nil {
		RespondError(c, http.StatusInternalServerError, "Storage service not initialized")
		return
	}

	filename := path.Base(job.FileKey)
	if idx := strings.Index(filename, "_"); idx != -1 && idx+1 < len(filename) {
		filename = filename[idx+1:]
	}
	if filename == "" || filename == "." {
		filename = "timesheet.xlsx"
	}

	presignedURL, err := storageSvc.GetPresignedDownloadURLWithFilename(ctx, job.FileKey, filename, 15*time.Minute)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to generate download link: "+err.Error())
		return
	}

	// Step 8: 302 Found Redirect to S3
	c.Redirect(http.StatusFound, presignedURL)
}
