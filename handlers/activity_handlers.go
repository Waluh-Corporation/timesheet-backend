package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
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

// appLocation returns the configured application location from config, defaulting to UTC.
func appLocation() *time.Location {
	return config.AppLocation()
}

// jakarta returns the configured application location, defaulting to UTC.
func jakarta() *time.Location {
	return appLocation()
}

func (s *Server) findProjectByRefID(refID uint) (*models.Project, error) {
	if s.MasterSvc == nil {
		return nil, errors.New("invalid project_ref_id: project does not exist or is inactive")
	}
	proj, err := s.MasterSvc.FindProjectByID(context.Background(), refID)
	if err != nil || proj == nil || !proj.IsActive {
		return nil, errors.New("invalid project_ref_id: project does not exist or is inactive")
	}
	return proj, nil
}

func reqContext(c *gin.Context) context.Context {
	if c != nil && c.Request != nil && c.Request.Context() != nil {
		ctx := c.Request.Context()
		if role := currentUserRole(c); role != "" {
			ctx = domain.WithUserRole(ctx, string(role))
		}
		return ctx
	}
	return context.Background()
}

func (s *Server) getActivityService() service.ActivityService {
	return s.ActivitySvc
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

// GetPreviousDailyActivity godoc
// @Summary Get previous day activity
// @Description Retrieves the most recent active daily activity strictly before the specified reference date (default: today in WIB) to allow copying/autofilling activity descriptions.
// @Tags Activity
// @Security BearerAuth
// @Produce json
// @Param date query string false "Reference date (YYYY-MM-DD, default: today)"
// @Success 200 {object} response.DailyActivityResponse
// @Failure 400 {object} response.ErrorResponse "Invalid date format"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "No previous activity found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/activities/previous [get]
func (s *Server) GetPreviousDailyActivity(c *gin.Context) {
	svc := s.getActivityService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errActivityServiceNotInitialized)
		return
	}

	dateParam := c.Query("date")
	resp, err := svc.GetPreviousDailyActivity(reqContext(c), currentUserID(c), dateParam)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "no previous activity found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			msg := strings.TrimPrefix(err.Error(), domain.ErrInvalidInput.Error()+": ")
			RespondError(c, http.StatusBadRequest, msg)
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

	var monthlyWitness *response.WitnessResponse
	if filter.Month != nil && filter.Year != nil {
		monthlyWitness, _ = svc.GetMonthlyWitness(reqContext(c), uid, *filter.Month, *filter.Year)
	}

	extra := make(map[string]interface{})
	if monthlyWitness != nil {
		extra["monthly_witness"] = monthlyWitness
	}

	RespondPaginatedWithMeta(c, http.StatusOK, respItems, meta.Page, meta.Limit, meta.TotalRows, extra)
}

// ListMonthlyActivities delegates to ListActivities for backwards-compatibility.
func (s *Server) ListMonthlyActivities(c *gin.Context) {
	s.ListActivities(c)
}

// ListWitnesses godoc
// @Summary List eligible witnesses for Berita Acara
// @Description Returns active users with role 'user' eligible to be chosen as a witness (excluding current user)
// @Tags Activity
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse{data=[]response.WitnessResponse}
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/users/witnesses [get]
func (s *Server) ListWitnesses(c *gin.Context) {
	uid := currentUserID(c)
	svc := s.getActivityService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errActivityServiceNotInitialized)
		return
	}

	witnesses, err := svc.GetEligibleWitnesses(reqContext(c), uid)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, witnesses)
}

// UpdateMonthlyWitness godoc
// @Summary Update witness for all Berita Acara entries in a given month
// @Description Updates the witness ID for all Berita Acara daily activities in the specified month and year
// @Tags Activity
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body request.UpdateMonthlyWitnessRequest true "Update witness payload"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/activities/witness [put]
func (s *Server) UpdateMonthlyWitness(c *gin.Context) {
	uid := currentUserID(c)
	svc := s.getActivityService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errActivityServiceNotInitialized)
		return
	}

	var req request.UpdateMonthlyWitnessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	count, witness, err := svc.UpdateMonthlyWitness(reqContext(c), uid, req.Month, req.Year, req.WitnessID)
	if err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			status := domain.MapErrorToHTTPStatus(userErr.Err)
			RespondError(c, status, userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var witResp *response.WitnessResponse
	if witness != nil {
		witResp = &response.WitnessResponse{
			ID:   witness.ID,
			Name: witness.Name,
		}
	}

	RespondSuccess(c, http.StatusOK, gin.H{
		"message":       "Monthly witness updated successfully for all Berita Acara records in this period",
		"updated_count": count,
		"witness":       witResp,
	})
}
