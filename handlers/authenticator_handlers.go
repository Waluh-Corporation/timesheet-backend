package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

// SyncCommunityAuthenticators godoc
// @Summary Synchronize authenticators from community registry (Admin)
// @Description Fetches the latest AAGUID and icon definitions from passkeydeveloper/passkey-authenticator-aaguids, upserts them into the database, and refreshes the in-memory cache.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.AuthenticatorSyncResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 502 {object} response.ErrorResponse "Bad gateway / failed to fetch external registry"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/authenticators/sync [post]
func (s *Server) SyncCommunityAuthenticators(c *gin.Context) {
	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database connection unavailable")
		return
	}

	total, err := services.SyncCommunityAAGUIDsToDB(c.Request.Context(), s.DB)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "unexpected HTTP status") ||
			strings.Contains(errStr, "execute http request") ||
			strings.Contains(errStr, "parse community aaguid JSON") {
			slog.Warn("failed to fetch community authenticators", "error", err, "ip", c.ClientIP())
			RespondError(c, http.StatusBadGateway, fmt.Sprintf("failed to fetch community authenticators: %v", err))
			return
		}
		slog.Error("failed to sync authenticators to database", "error", err, "ip", c.ClientIP())
		RespondError(c, http.StatusInternalServerError, fmt.Sprintf("failed to sync authenticators: %v", err))
		return
	}

	slog.Info("community authenticators synchronized", "total", total, "ip", c.ClientIP())
	RespondSuccess(c, http.StatusOK, response.AuthenticatorSyncResponse{
		TotalSynced: total,
		SyncedAt:    time.Now().UTC(),
	})
}

// AdminListAuthenticators godoc
// @Summary List registered authenticators (Admin)
// @Description Retrieves all known AAGUID authenticators stored in the database with pagination and search.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param search query string false "Search query by name or AAGUID"
// @Param page query int false "Page number (defaults to 1)"
// @Param limit query int false "Items per page (defaults to 50, max 200)"
// @Success 200 {object} response.AuthenticatorListResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/authenticators [get]
func (s *Server) AdminListAuthenticators(c *gin.Context) {
	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database connection unavailable")
		return
	}

	page := queryIntDefault(c, "page", 1)
	if page < 1 {
		page = 1
	}

	limit := queryIntDefault(c, "limit", 50)
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	search := strings.TrimSpace(c.Query("search"))

	query := s.DB.Model(&models.AuthenticatorAAGUID{})
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(aaguid) LIKE LOWER(?)", pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var records []models.AuthenticatorAAGUID
	offset := (page - 1) * limit
	if err := query.Order("name ASC").Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]response.AuthenticatorItemResponse, len(records))
	for i, r := range records {
		items[i] = response.AuthenticatorItemResponse{
			AAGUID:    r.AAGUID,
			Name:      r.Name,
			Icon:      r.Icon,
			UpdatedAt: r.UpdatedAt,
		}
	}

	RespondSuccess(c, http.StatusOK, response.AuthenticatorListResponse{
		Authenticators: items,
		Total:          total,
		Page:           page,
		Limit:          limit,
	})
}
