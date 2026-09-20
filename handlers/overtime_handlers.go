package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	_ "timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
)

// OvertimeRequest carries data to create/update an overtime entry.
type OvertimeRequest = request.OvertimeRequest

// UpsertOvertime godoc
// @Summary Create or update overtime record
// @Description Upserts an overtime entry for the authenticated user for SPL reporting.
// @Tags Overtime
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body handlers.OvertimeRequest true "Overtime entry data"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload or date format"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/overtimes [post]
func (s *Server) UpsertOvertime(c *gin.Context) {
	var req request.OvertimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	tsSvc := s.getTimesheetService()
	if tsSvc == nil {
		RespondError(c, http.StatusInternalServerError, "timesheet service not initialized")
		return
	}

	if err := tsSvc.UpsertOvertime(reqContext(c), currentUserID(c), &req); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "overtime entry saved successfully")
}

// ListMonthlyOvertimes godoc
// @Summary List monthly overtime records
// @Description Retrieves all overtime records for the authenticated user for a specific month and year.
// @Tags Overtime
// @Security BearerAuth
// @Produce json
// @Param year query int false "Year (defaults to current year)"
// @Param month query int false "Month 1-12 (defaults to current month)"
// @Success 200 {array} response.OvertimeResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/overtimes [get]
func (s *Server) ListMonthlyOvertimes(c *gin.Context) {
	year := queryIntDefault(c, "year", time.Now().In(jakarta()).Year())
	month := queryIntDefault(c, "month", int(time.Now().In(jakarta()).Month()))

	tsSvc := s.getTimesheetService()
	if tsSvc == nil {
		RespondError(c, http.StatusInternalServerError, "timesheet service not initialized")
		return
	}

	resp, err := tsSvc.ListMonthlyOvertimes(reqContext(c), currentUserID(c), month, year)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, resp)
}

// DeleteOvertime godoc
// @Summary Delete overtime record
// @Description Removes a specific overtime entry belonging to the authenticated user.
// @Tags Overtime
// @Security BearerAuth
// @Produce json
// @Param id path int true "Overtime Entry ID"
// @Success 200 {object} response.DeleteResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/overtimes/{id} [delete]
func (s *Server) DeleteOvertime(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid overtime ID")
		return
	}

	tsSvc := s.getTimesheetService()
	if tsSvc == nil {
		RespondError(c, http.StatusInternalServerError, "timesheet service not initialized")
		return
	}

	if err := tsSvc.DeleteOvertime(reqContext(c), uint(id), currentUserID(c)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "overtime entry not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondDelete(c, http.StatusOK)
}
