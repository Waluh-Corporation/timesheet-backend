package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/services"
)

// GetMySignature godoc
// @Summary Get current user's signature specimen status
// @Description Returns the active DocuSeal signature specimen status, preview URL, and default abbreviated signature text for the authenticated user.
// @Tags Profile
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SignatureStatusResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Router /api/v1/profile/signature [get]
func (s *Server) GetMySignature(c *gin.Context) {
	sigSvc := s.getSignatureService()
	if sigSvc == nil {
		RespondError(c, http.StatusInternalServerError, "signature service not initialized")
		return
	}

	status, err := sigSvc.GetStatus(reqContext(c), currentUserID(c))
	if err != nil {
		s.respondSignatureError(c, err)
		return
	}

	RespondSuccess(c, http.StatusOK, status)
}

// CreateSignatureSession godoc
// @Summary Create an embedded DocuSeal signing session
// @Description Initializes a new DocuSeal specimen signing session for the authenticated user and returns the public embed URL.
// @Tags Profile
// @Security BearerAuth
// @Produce json
// @Success 201 {object} response.SignatureSessionResponse
// @Failure 400 {object} response.ErrorResponse "DocuSeal integration disabled or misconfigured"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Router /api/v1/profile/signature/session [post]
func (s *Server) CreateSignatureSession(c *gin.Context) {
	sigSvc := s.getSignatureService()
	if sigSvc == nil {
		RespondError(c, http.StatusInternalServerError, "signature service not initialized")
		return
	}

	uid := currentUserID(c)
	session, err := sigSvc.CreateSession(reqContext(c), uid)
	if err != nil {
		s.respondSignatureError(c, err)
		return
	}

	s.recordAuditLog(reqContext(c), AuditLogParams{
		ActorID:      &uid,
		Action:       "signature.session.create",
		ResourceType: "user_signature",
		ResourceID:   strconv.FormatInt(session.SubmitterID, 10),
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
	})

	RespondSuccess(c, http.StatusCreated, session)
}

// SyncMySignature godoc
// @Summary Synchronize completed DocuSeal signature specimen
// @Description Fetches the completed signature from DocuSeal, normalizes/renders a transparent PNG specimen in S3, and activates it for timesheet and Berita Acara generation.
// @Tags Profile
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body request.SyncSignatureRequest false "Optional submitter ID or slug"
// @Success 200 {object} response.SignatureStatusResponse
// @Failure 400 {object} response.ErrorResponse "Signature not completed or invalid"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 404 {object} response.ErrorResponse "No signature session found"
// @Router /api/v1/profile/signature/sync [post]
func (s *Server) SyncMySignature(c *gin.Context) {
	sigSvc := s.getSignatureService()
	if sigSvc == nil {
		RespondError(c, http.StatusInternalServerError, "signature service not initialized")
		return
	}

	var req request.SyncSignatureRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	uid := currentUserID(c)
	status, err := sigSvc.SyncSignature(reqContext(c), uid, &req)
	if err != nil {
		s.respondSignatureError(c, err)
		return
	}

	s.recordAuditLog(reqContext(c), AuditLogParams{
		ActorID:      &uid,
		Action:       "signature.specimen.sync",
		ResourceType: "user_signature",
		ResourceID:   strconv.FormatUint(uint64(uid), 10),
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Details: map[string]interface{}{
			"signature_type": status.SignatureType,
		},
	})

	RespondSuccess(c, http.StatusOK, status)
}

// DeleteMySignature godoc
// @Summary Revoke active signature specimen
// @Description Deactivates the user's active signature specimen and removes the stored PNG from S3.
// @Tags Profile
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.MessageResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "No active signature found"
// @Router /api/v1/profile/signature [delete]
func (s *Server) DeleteMySignature(c *gin.Context) {
	sigSvc := s.getSignatureService()
	if sigSvc == nil {
		RespondError(c, http.StatusInternalServerError, "signature service not initialized")
		return
	}

	uid := currentUserID(c)
	if err := sigSvc.RevokeSignature(reqContext(c), uid); err != nil {
		s.respondSignatureError(c, err)
		return
	}

	s.recordAuditLog(reqContext(c), AuditLogParams{
		ActorID:      &uid,
		Action:       "signature.specimen.revoke",
		ResourceType: "user_signature",
		ResourceID:   strconv.FormatUint(uint64(uid), 10),
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
	})

	RespondMessage(c, http.StatusOK, "Signature specimen revoked successfully")
}

// DocusealWebhook godoc
// @Summary Receive DocuSeal completion webhook events
// @Description Processes form.completed and submission.completed webhook callbacks from the self-hosted DocuSeal instance.
// @Tags Webhooks
// @Accept json
// @Produce json
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Malformed payload"
// @Failure 401 {object} response.ErrorResponse "Invalid webhook signature"
// @Router /api/v1/webhooks/docuseal [post]
func (s *Server) DocusealWebhook(c *gin.Context) {
	sigSvc := s.getSignatureService()
	if sigSvc == nil {
		RespondError(c, http.StatusInternalServerError, "signature service not initialized")
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(c.Request.Body, 2<<20))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "failed to read webhook body")
		return
	}

	if !s.verifyDocusealWebhookRequest(c, rawBody) {
		RespondError(c, http.StatusUnauthorized, "invalid webhook signature")
		return
	}

	var payload request.DocusealWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid webhook JSON payload")
		return
	}

	if err := sigSvc.HandleWebhook(reqContext(c), &payload); err != nil {
		s.respondSignatureError(c, err)
		return
	}

	RespondMessage(c, http.StatusOK, "webhook processed")
}

func (s *Server) verifyDocusealWebhookRequest(c *gin.Context, rawBody []byte) bool {
	if s == nil || s.Cfg == nil {
		return true
	}
	secret := strings.TrimSpace(s.Cfg.DocusealWebhookSecret)
	if secret == "" {
		return true
	}

	sigHeader := strings.TrimSpace(c.GetHeader("X-Docuseal-Signature"))
	if sigHeader == "" {
		sigHeader = strings.TrimSpace(c.GetHeader("X-Webhook-Secret"))
	}
	if sigHeader == "" {
		return false
	}

	if services.VerifyDocusealWebhookSignature(sigHeader, rawBody, secret, time.Now()) {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(sigHeader), []byte(secret)) == 1
}

func (s *Server) respondSignatureError(c *gin.Context, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, domain.ErrForbidden) {
		RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	RespondError(c, http.StatusInternalServerError, err.Error())
}
