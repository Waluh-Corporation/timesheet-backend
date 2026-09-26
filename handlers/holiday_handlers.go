package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/models"
)

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

	holidays, err := s.getMasterService().GetHolidays(c.Request.Context(), year, month)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, holidays)
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

	syncedCount, err := s.getMasterService().SyncHolidays(c.Request.Context(), year)
	if err != nil {
		RespondError(c, http.StatusBadGateway, err.Error())
		return
	}
	if syncedCount == 0 {
		RespondMessage(c, http.StatusOK, fmt.Sprintf("no holidays found for year %d", year))
		return
	}

	RespondMessage(c, http.StatusOK, fmt.Sprintf("successfully synchronized %d holidays for year %d", syncedCount, year))
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
	var yearPtr *int
	if yearStr := c.Query("year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			yearPtr = &y
		}
	}

	holidays, err := s.getMasterService().ListAllHolidays(c.Request.Context(), yearPtr)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, holidays)
}
