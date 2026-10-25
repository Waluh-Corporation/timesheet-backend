package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/scheduler"
)

// GetVAPIDKey godoc
// @Summary Get VAPID public key
// @Description Returns the application server public key for browser Web Push subscription.
// @Tags Push Notification
// @Produce json
// @Success 200 {object} response.VAPIDKeyResponse
// @Router /api/v1/push/vapid-public-key [get]
func (s *Server) GetVAPIDKey(c *gin.Context) {
	svc := s.getPushService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "push service unavailable")
		return
	}
	RespondSuccess(c, http.StatusOK, gin.H{"public_key": svc.GetPublicKey()})
}

// GetPushSchedule godoc
// @Summary Get push notification schedule
// @Description Returns the dynamic push reminder schedule, including hour, minute, active timezone, next run time, and human-readable description.
// @Tags Push Notification
// @Produce json
// @Success 200 {object} response.PushScheduleResponse
// @Router /api/v1/push/schedule [get]
func (s *Server) GetPushSchedule(c *gin.Context) {
	tz := ""
	cronExpr := scheduler.DefaultReminderCron

	if s != nil && s.Cfg != nil {
		tz = strings.TrimSpace(s.Cfg.Timezone)
		if trimmedCron := strings.TrimSpace(s.Cfg.ReminderCron); trimmedCron != "" {
			cronExpr = trimmedCron
		}
	}

	if tz == "" {
		if loc := config.AppLocation(); loc != nil && loc.String() != "" {
			tz = loc.String()
		} else {
			tz = "UTC"
		}
	}

	info := scheduler.GetScheduleInfo(tz, cronExpr)

	hour := 17
	minute := 0
	parts := strings.Fields(info.CronExpression)
	if len(parts) >= 2 {
		if m, err := strconv.Atoi(parts[0]); err == nil {
			minute = m
		} else if info.NextRun != nil {
			minute = info.NextRun.Minute()
		}
		if h, err := strconv.Atoi(parts[1]); err == nil {
			hour = h
		} else if info.NextRun != nil {
			hour = info.NextRun.Hour()
		}
	} else if info.NextRun != nil {
		hour = info.NextRun.Hour()
		minute = info.NextRun.Minute()
	}

	if hour < 0 || hour > 23 {
		hour = 17
	}
	if minute < 0 || minute > 59 {
		minute = 0
	}

	resp := response.PushScheduleResponse{
		Hour:           hour,
		Minute:         minute,
		Timezone:       info.Timezone,
		NextRun:        info.NextRun,
		HumanReadable:  info.HumanReadable,
		CronExpression: info.CronExpression,
		IsEnabled:      info.IsEnabled,
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// Subscribe godoc
// @Summary Subscribe to Web Push notifications
// @Description Registers or updates browser push notification subscription for the authenticated user.
// @Tags Push Notification
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.SubscribeRequest true "Push subscription payload"
// @Success 201 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/push/subscribe [post]
func (s *Server) Subscribe(c *gin.Context) {
	var req request.SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	svc := s.getPushService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "push service unavailable")
		return
	}
	if err := svc.Subscribe(c.Request.Context(), currentUserID(c), req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondMessage(c, http.StatusCreated, "subscribed")
}

// Unsubscribe godoc
// @Summary Unsubscribe from Web Push notifications
// @Description Removes active push subscription for the browser, silencing timesheet reminder notifications.
// @Tags Push Notification
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.UnsubscribeRequest false "Optional endpoint filter"
// @Success 200 {object} response.MessageResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/push/unsubscribe [post]
func (s *Server) Unsubscribe(c *gin.Context) {
	var req request.UnsubscribeRequest
	_ = c.ShouldBindJSON(&req)
	svc := s.getPushService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "push service unavailable")
		return
	}
	if err := svc.Unsubscribe(c.Request.Context(), currentUserID(c), req.Endpoint); err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondMessage(c, http.StatusOK, "unsubscribed")
}

// SendTestPush godoc
// @Summary Send test push notification
// @Description Dispatches an immediate test push notification to verify browser notification display.
// @Tags Push Notification
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.MessageResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Router /api/v1/push/test [post]
func (s *Server) SendTestPush(c *gin.Context) {
	svc := s.getPushService()
	if svc != nil {
		_ = svc.SendTestPush(c.Request.Context(), currentUserID(c))
	}
	RespondMessage(c, http.StatusOK, "test notification dispatched")
}
