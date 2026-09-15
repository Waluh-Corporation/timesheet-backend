package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
	"timesheet-backend/push"
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
	RespondSuccess(c, http.StatusOK, gin.H{"public_key": s.Push.PublicKey()})
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
	sub := models.PushSubscription{
		UserID:   currentUserID(c),
		Endpoint: req.Endpoint,
		P256dh:   req.Keys.P256dh,
		Auth:     req.Keys.Auth,
	}
	// Idempotent on endpoint: re-subscribing updates the owning user + keys.
	err := s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "p256dh", "auth"}),
	}).Create(&sub).Error
	if err != nil {
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
	q := s.DB.Where("user_id = ?", currentUserID(c))
	if req.Endpoint != "" {
		q = q.Where("endpoint = ?", req.Endpoint)
	}
	if err := q.Delete(&models.PushSubscription{}).Error; err != nil {
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
	s.Push.SendToUser(currentUserID(c), push.Payload{
		Title: "Timesheet Portal",
		Body:  "Waktunya isi timesheet hari ini!",
		URL:   "/activity",
	})
	RespondMessage(c, http.StatusOK, "test notification dispatched")
}

// GetPushSchedule godoc
// @Summary Get Web Push reminder schedule
// @Description Returns the configured cron schedule, timezone, status, and next execution time for daily timesheet reminders. Accessible by authenticated users and admins.
// @Tags Push Notification
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.PushScheduleResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Router /api/v1/push/schedule [get]
func (s *Server) GetPushSchedule(c *gin.Context) {
	info := scheduler.GetScheduleInfo(s.Cfg.Timezone, s.Cfg.ReminderCron)
	RespondSuccess(c, http.StatusOK, response.PushScheduleResponse{
		CronExpression: info.CronExpression,
		Timezone:       info.Timezone,
		IsEnabled:      info.IsEnabled,
		NextRun:        info.NextRun,
		HumanReadable:  info.HumanReadable,
	})
}

// resolveTargetUserID determines the target user ID from request body, path parameter, or query string.
func resolveTargetUserID(c *gin.Context, bodyUserID uint) uint {
	if bodyUserID != 0 {
		return bodyUserID
	}
	if idStr := c.Param("id"); idStr != "" {
		if id, err := strconv.ParseUint(idStr, 10, 32); err == nil {
			return uint(id)
		}
	}
	if idQuery := c.Query("user_id"); idQuery != "" {
		if id, err := strconv.ParseUint(idQuery, 10, 32); err == nil {
			return uint(id)
		}
	}
	return 0
}

// AdminSendTestPush godoc
// @Summary Send test push notification to a user (Admin only)
// @Description Dispatches an immediate test push notification to a designated user to verify browser notifications.
// @Tags Push Notification, Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.AdminTestPushRequest false "Push notification test parameters including target user_id"
// @Param id path int false "Target User ID (when using /admin/users/:id/push/test)"
// @Success 200 {object} response.AdminTestPushResponse
// @Failure 400 {object} response.ErrorResponse "Bad request (missing user_id)"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden (admin only)"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Router /api/v1/admin/push/test [post]
func (s *Server) AdminSendTestPush(c *gin.Context) {
	var req request.AdminTestPushRequest
	_ = c.ShouldBindJSON(&req)

	targetID := resolveTargetUserID(c, req.UserID)
	if targetID == 0 {
		RespondError(c, http.StatusBadRequest, "user_id is required")
		return
	}

	var targetUser models.User
	if err := s.DB.First(&targetUser, targetID).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	var subCount int64
	s.DB.Model(&models.PushSubscription{}).Where("user_id = ?", targetID).Count(&subCount)

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Timesheet Reminder (Test Admin)"
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		body = "Waktunya isi timesheet hari ini! (Pesan uji coba dari Administrator)"
	}
	targetURL := strings.TrimSpace(req.URL)
	if targetURL == "" {
		targetURL = "/activity"
	}

	s.Push.SendToUser(targetID, push.Payload{
		Title: title,
		Body:  body,
		URL:   targetURL,
	})

	msg := "test notification dispatched"
	if subCount == 0 {
		msg = "test notification dispatched (user currently has 0 active browser subscriptions)"
	}

	RespondSuccess(c, http.StatusOK, response.AdminTestPushResponse{
		UserID:             targetID,
		Username:           targetUser.Username,
		SubscriptionsCount: subCount,
		Message:            msg,
	})
}
