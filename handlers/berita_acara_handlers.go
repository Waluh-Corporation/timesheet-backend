package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
)

const errBeritaAcaraServiceNotInitialized = "berita acara service not initialized"

func (s *Server) getBeritaAcaraService() service.BeritaAcaraService {
	if s.BeritaAcaraSvc != nil {
		return s.BeritaAcaraSvc
	}
	if s.DB != nil {
		return service.NewBeritaAcaraService(repository.NewBeritaAcaraRepository(s.DB))
	}
	return nil
}

// UpsertBeritaAcara godoc
// @Summary Upsert Berita Acara kehadiran
// @Description Creates or updates a Berita Acara kehadiran entry for the authenticated user on a given date.
// @Tags Berita Acara
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.BeritaAcaraRequest true "Berita Acara payload"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid date format or payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/berita-acara [post]
func (s *Server) UpsertBeritaAcara(c *gin.Context) {
	var req request.BeritaAcaraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	svc := s.getBeritaAcaraService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errBeritaAcaraServiceNotInitialized)
		return
	}

	if err := svc.UpsertBeritaAcara(reqContext(c), currentUserID(c), &req); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			msg := strings.TrimPrefix(err.Error(), domain.ErrInvalidInput.Error()+": ")
			RespondError(c, http.StatusBadRequest, msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "berita acara saved successfully")
}

// GetBeritaAcara godoc
// @Summary Get Berita Acara detail
// @Description Retrieves details of a specific Berita Acara record by ID.
// @Tags Berita Acara
// @Security BearerAuth
// @Produce json
// @Param id path int true "Berita Acara ID"
// @Success 200 {object} response.BeritaAcaraDetailResponse
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden: not authorized to access another user's record"
// @Failure 404 {object} response.ErrorResponse "Record not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/berita-acara/{id} [get]
func (s *Server) GetBeritaAcara(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid berita acara ID, expected positive integer")
		return
	}

	svc := s.getBeritaAcaraService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errBeritaAcaraServiceNotInitialized)
		return
	}

	resp, err := svc.GetBeritaAcara(reqContext(c), currentUserID(c), uint(id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "berita acara not found")
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(c, http.StatusForbidden, "you are not authorized to access another user's berita acara")
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// ListBeritaAcara godoc
// @Summary List Berita Acara records
// @Description Retrieves a paginated list of Berita Acara records for the authenticated user.
// @Tags Berita Acara
// @Security BearerAuth
// @Produce json
// @Param month query int false "Month filter (1-12)"
// @Param year query int false "Year filter"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Items per page (default 10)"
// @Param all query bool false "Fetch all records without pagination"
// @Success 200 {object} response.PaginatedResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/berita-acara [get]
func (s *Server) ListBeritaAcara(c *gin.Context) {
	svc := s.getBeritaAcaraService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, errBeritaAcaraServiceNotInitialized)
		return
	}

	filter := repository.BeritaAcaraFilter{
		SortOrder: c.DefaultQuery("sort_order", "asc"),
	}

	if m, err := strconv.Atoi(c.Query("month")); err == nil && m >= 1 && m <= 12 {
		filter.Month = &m
	}
	if y, err := strconv.Atoi(c.Query("year")); err == nil && y > 0 {
		filter.Year = &y
	}
	if sd, err := time.Parse(dateFormatYYYYMMDD, c.Query("start_date")); err == nil {
		filter.StartDate = &sd
	}
	if ed, err := time.Parse(dateFormatYYYYMMDD, c.Query("end_date")); err == nil {
		filter.EndDate = &ed
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	all := c.Query("all") == "true"

	filter.Page = page
	filter.Limit = limit
	filter.IsAll = all

	list, meta, err := svc.ListBeritaAcara(reqContext(c), currentUserID(c), filter)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondPaginated(c, http.StatusOK, list, meta.Page, meta.Limit, meta.TotalRows)
}
