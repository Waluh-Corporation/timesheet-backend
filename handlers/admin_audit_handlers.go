package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/config"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
)

// ListAuditLogsQuery encapsulates query parameters for listing audit logs.
type ListAuditLogsQuery struct {
	Action        string `form:"action"`
	ResourceType  string `form:"resource_type"`
	ActorUsername string `form:"actor_username"`
	StartDate     string `form:"start_date"`
	EndDate       string `form:"end_date"`
	Page          int    `form:"page"`
	Limit         int    `form:"limit"`
}

func parseAuditDate(raw string, isEnd bool) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "\"'")
	if s == "" {
		return nil, nil
	}
	if t, ok := parseRFC3339Variants(s); ok {
		return t, nil
	}
	loc := config.AppLocation()
	if t, ok := parseDateTimeInLocation(s, loc); ok {
		return t, nil
	}
	if t, ok := parseMinuteOrDayInLocation(s, loc, isEnd); ok {
		return t, nil
	}
	return nil, errors.New("invalid date format, expected YYYY-MM-DD or RFC3339")
}

func parseRFC3339Variants(s string) (*time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return &t, true
	}
	var candidates []string
	if len(s) > 10 && s[10] == ' ' {
		normalized := s[:10] + "T" + s[11:]
		candidates = append(candidates, normalized)
		if idx := strings.LastIndex(normalized, " "); idx > 10 {
			candidates = append(candidates, normalized[:idx]+"+"+normalized[idx+1:])
		}
	}
	if idx := strings.LastIndex(s, " "); idx > 10 {
		candidates = append(candidates, s[:idx]+"+"+s[idx+1:])
	}
	for _, c := range candidates {
		if t, err := time.Parse(time.RFC3339Nano, c); err == nil {
			return &t, true
		}
	}
	return nil, false
}

func parseDateTimeInLocation(s string, loc *time.Location) (*time.Time, bool) {
	formats := []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			return &t, true
		}
	}
	return nil, false
}

func parseMinuteOrDayInLocation(s string, loc *time.Location, isEnd bool) (*time.Time, bool) {
	minuteFormats := []string{"2006-01-02T15:04", "2006-01-02 15:04"}
	for _, f := range minuteFormats {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			sec, nsec := 0, 0
			if isEnd {
				sec, nsec = 59, 999999999
			}
			res := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), sec, nsec, loc)
			return &res, true
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		hour, min, sec, nsec := 0, 0, 0, 0
		if isEnd {
			hour, min, sec, nsec = 23, 59, 59, 999999999
		}
		res := time.Date(t.Year(), t.Month(), t.Day(), hour, min, sec, nsec, loc)
		return &res, true
	}
	return nil, false
}

// ListAuditLogs godoc
// @Summary List audit logs (Admin)
// @Description Retrieves paginated audit logs with optional filtering by action, resource type, actor username, and date range.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param action query string false "Filter by action"
// @Param resource_type query string false "Filter by resource type"
// @Param actor_username query string false "Filter by actor username"
// @Param start_date query string false "Filter by start date (YYYY-MM-DD or RFC3339)"
// @Param end_date query string false "Filter by end date (YYYY-MM-DD or RFC3339)"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Page size limit (default 50, max 100)"
// @Success 200 {object} response.PaginatedResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/audit-logs [get]
func (s *Server) ListAuditLogs(c *gin.Context) {
	repo := s.getAuditRepository()
	if repo == nil {
		RespondError(c, http.StatusInternalServerError, "audit repository unavailable")
		return
	}

	var query ListAuditLogsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	startDate, err := parseAuditDate(query.StartDate, false)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid start_date format, expected YYYY-MM-DD or RFC3339")
		return
	}

	endDate, err := parseAuditDate(query.EndDate, true)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid end_date format, expected YYYY-MM-DD or RFC3339")
		return
	}

	if startDate != nil && endDate != nil && startDate.After(*endDate) {
		RespondError(c, http.StatusBadRequest, "start_date cannot be after end_date")
		return
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	const maxPage = 10_000_000
	if page > maxPage {
		page = maxPage
	}

	limit := query.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	filter := domain.AuditLogFilter{
		Action:        strings.Trim(strings.TrimSpace(query.Action), "\"'"),
		ResourceType:  strings.Trim(strings.TrimSpace(query.ResourceType), "\"'"),
		ActorUsername: strings.Trim(strings.TrimSpace(query.ActorUsername), "\"'"),
		StartDate:     startDate,
		EndDate:       endDate,
	}

	entries, total, err := repo.List(c.Request.Context(), filter, limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]response.AuditLogResponse, 0, len(entries))
	for _, entry := range entries {
		var details json.RawMessage
		trimmed := bytes.TrimSpace(entry.Details)
		if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) && json.Valid(trimmed) {
			details = json.RawMessage(trimmed)
		}
		resp = append(resp, response.AuditLogResponse{
			ID:            entry.ID,
			CreatedAt:     entry.CreatedAt,
			ActorID:       entry.ActorID,
			ActorUsername: entry.ActorUsername,
			Action:        entry.Action,
			ResourceType:  entry.ResourceType,
			ResourceID:    entry.ResourceID,
			IPAddress:     entry.IPAddress,
			UserAgent:     entry.UserAgent,
			Details:       details,
		})
	}

	RespondPaginated(c, http.StatusOK, resp, page, limit, total)
}
