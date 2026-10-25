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

	// Try RFC3339 / RFC3339Nano first (accepts both second and sub-second precision, e.g. JS toISOString)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return &t, nil
	}

	// Support ISO-8601 strings using space separator instead of 'T' (e.g. "2026-10-01 08:00:00Z" or "2026-10-01 08:00:00+07:00")
	if len(s) > 10 && s[10] == ' ' {
		normalized := s[:10] + "T" + s[11:]
		if t, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
			return &t, nil
		}
		// Also handle space separator + unescaped URL timezone offset (e.g. "2026-10-01 15:04:05 07:00")
		if idx := strings.LastIndex(normalized, " "); idx != -1 && idx > 10 {
			candidate := normalized[:idx] + "+" + normalized[idx+1:]
			if t, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
				return &t, nil
			}
		}
	}

	// In URL query strings, a literal '+' in timezone offsets (e.g. +07:00) is often decoded into a space.
	// Restore '+' if the string matches an unescaped offset suffix (e.g. " 07:00" or " 0700").
	if idx := strings.LastIndex(s, " "); idx != -1 && idx > 10 {
		candidate := s[:idx] + "+" + s[idx+1:]
		if t, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
			return &t, nil
		}
	}

	loc := config.AppLocation()

	// Try datetime formats with fractional seconds without timezone in application location
	if t, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", s, loc); err == nil {
		return &t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.999999999", s, loc); err == nil {
		return &t, nil
	}

	// Try datetime formats without timezone in application location
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, loc); err == nil {
		return &t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc); err == nil {
		return &t, nil
	}

	// Try HTML5 datetime-local formats without seconds (YYYY-MM-DDTHH:mm or YYYY-MM-DD HH:mm)
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, loc); err == nil {
		if isEnd {
			endOfMinute := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 59, 999999999, loc)
			return &endOfMinute, nil
		}
		startOfMinute := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc)
		return &startOfMinute, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, loc); err == nil {
		if isEnd {
			endOfMinute := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 59, 999999999, loc)
			return &endOfMinute, nil
		}
		startOfMinute := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc)
		return &startOfMinute, nil
	}

	// Try YYYY-MM-DD in application location
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		if isEnd {
			endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, loc)
			return &endOfDay, nil
		}
		startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		return &startOfDay, nil
	}

	return nil, errors.New("invalid date format, expected YYYY-MM-DD or RFC3339")
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
