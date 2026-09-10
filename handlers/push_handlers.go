package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"timesheet-backend/models"
	"timesheet-backend/push"
)

// GetVAPIDKey godoc
// @Summary Get VAPID public key
// @Description Returns the application server public key for browser Web Push subscription.
// @Tags Push Notification
// @Produce json
// @Success 200 {object} models.VAPIDKeyResponse
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
// @Param request body models.SubscribeRequest true "Push subscription payload"
// @Success 201 {object} models.MessageResponse
// @Failure 400 {object} models.ErrorResponse "Invalid payload"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/push/subscribe [post]
func (s *Server) Subscribe(c *gin.Context) {
	var req models.SubscribeRequest
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
// @Param request body models.UnsubscribeRequest false "Optional endpoint filter"
// @Success 200 {object} models.MessageResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/push/unsubscribe [post]
func (s *Server) Unsubscribe(c *gin.Context) {
	var req models.UnsubscribeRequest
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
// @Success 200 {object} models.MessageResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /api/v1/push/test [post]
func (s *Server) SendTestPush(c *gin.Context) {
	s.Push.SendToUser(currentUserID(c), push.Payload{
		Title: "Timesheet Portal",
		Body:  "Waktunya isi timesheet hari ini!",
		URL:   "/activity",
	})
	RespondMessage(c, http.StatusOK, "test notification dispatched")
}
