package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

// ForgotPassword godoc
// @Summary Request password reset
// @Description Generates a password reset token and sends an email with the reset link. Always returns 200 to prevent user enumeration.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body request.ForgotRequest true "User email"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload"
// @Router /api/v1/auth/forgot-password [post]
func (s *Server) ForgotPassword(c *gin.Context) {
	var req request.ForgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidPayload)
		return
	}

	normEmail := strings.ToLower(strings.TrimSpace(req.Email))
	clientIP := c.ClientIP()

	cooldown := 60 * time.Second
	if s.Cfg != nil && s.Cfg.ResetPasswordCooldown > 0 {
		cooldown = s.Cfg.ResetPasswordCooldown
	}

	// 1. Fast in-memory cooldown check per email and per IP to prevent email bombing,
	// spamming, and resource exhaustion (applies uniformly to existing and non-existing accounts).
	if !s.checkAndRecordResetCooldown(normEmail, clientIP, cooldown) {
		slog.Warn("password reset throttled by cooldown", "email", maskEmail(normEmail), "ip", clientIP)
		RespondMessage(c, http.StatusOK, "if the email exists, a reset link has been sent")
		return
	}

	userRepo := s.getUserRepository()
	tokenRepo := s.getTokenRepository()
	if userRepo != nil && tokenRepo != nil {
		if user, err := userRepo.FindByEmail(c.Request.Context(), normEmail); err == nil && user != nil && user.IsActive {
			now := time.Now()

			// 2. Secondary database-backed check to preserve cooldown across server restarts or multi-pod replicas
			if latest, err := tokenRepo.GetLatestResetTokenByUserID(c.Request.Context(), user.ID); err == nil && latest != nil {
				if now.Sub(latest.CreatedAt) < cooldown {
					slog.Warn("password reset throttled by DB token cooldown", "user_id", user.ID, "ip", clientIP)
					RespondMessage(c, http.StatusOK, "if the email exists, a reset link has been sent")
					return
				}
			}

			raw, hash, err := auth.GenerateResetToken()
			if err == nil {
				ttl := 60 * time.Minute
				if s.Cfg != nil && s.Cfg.ResetTokenTTL > 0 {
					ttl = s.Cfg.ResetTokenTTL
				}

				resetToken := &models.PasswordResetToken{
					UserID:    user.ID,
					TokenType: "password_reset",
					TokenHash: hash,
					ExpiresAt: now.Add(ttl),
					CreatedIP: clientIP,
				}

				// 3. Atomically invalidate previous active reset tokens and save the new active token
				if err := tokenRepo.CreateResetTokenWithInvalidation(c.Request.Context(), resetToken, now); err == nil {
					link := s.publicBaseURL(c) + "/reset-password?token=" + raw
					s.dispatchResetEmail(user.Email, user.Username, link)
					slog.Info("password reset link issued", "user_id", user.ID, "ip", clientIP)
				} else {
					slog.Error("failed to create reset token with invalidation", "error", err, "user_id", user.ID)
				}
			}
		}
	}
	RespondMessage(c, http.StatusOK, "if the email exists, a reset link has been sent")
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}
	name, domain := parts[0], parts[1]
	if len(name) <= 2 {
		return name[:1] + "***@" + domain
	}
	return string(name[0]) + "***" + string(name[len(name)-1]) + "@" + domain
}

// VerifyResetPasswordToken godoc
// @Summary Verify password reset token
// @Description Checks if a password reset token is valid, expired, or already used before displaying the reset password form.
// @Tags Auth
// @Accept json
// @Produce json
// @Param token query string false "Reset token (query parameter)"
// @Param request body request.VerifyResetTokenRequest false "Reset token (JSON body)"
// @Success 200 {object} response.VerifyResetTokenResponse "Token is valid"
// @Failure 400 {object} response.VerifyResetTokenResponse "Token is invalid, expired, or already used"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/auth/reset-password/verify [get]
// @Router /api/v1/auth/reset-password/verify [post]
func (s *Server) VerifyResetPasswordToken(c *gin.Context) {
	rawToken := strings.TrimSpace(c.Query("token"))
	if rawToken == "" && c.Request.Method == http.MethodPost {
		var req request.VerifyResetTokenRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			rawToken = strings.TrimSpace(req.Token)
		}
	}

	if rawToken == "" {
		c.JSON(http.StatusBadRequest, response.VerifyResetTokenResponse{
			Valid:   false,
			Status:  "invalid",
			Message: "token is required",
		})
		return
	}

	tokenRepo := s.getTokenRepository()
	userRepo := s.getUserRepository()
	if tokenRepo == nil || userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}

	tokenHash := auth.HashToken(rawToken)
	token, err := tokenRepo.FindResetTokenByHash(c.Request.Context(), tokenHash)
	if err != nil || token == nil {
		c.JSON(http.StatusBadRequest, response.VerifyResetTokenResponse{
			Valid:   false,
			Status:  "invalid",
			Message: "invalid reset token",
		})
		return
	}

	if token.UsedAt != nil {
		c.JSON(http.StatusBadRequest, response.VerifyResetTokenResponse{
			Valid:   false,
			Status:  "already_used",
			Message: "reset token has already been used",
		})
		return
	}

	if time.Now().After(token.ExpiresAt) {
		c.JSON(http.StatusBadRequest, response.VerifyResetTokenResponse{
			Valid:   false,
			Status:  "expired",
			Message: "reset token has expired",
		})
		return
	}

	user, err := userRepo.FindByID(c.Request.Context(), token.UserID)
	if err != nil || user == nil || !user.IsActive {
		c.JSON(http.StatusBadRequest, response.VerifyResetTokenResponse{
			Valid:   false,
			Status:  "invalid",
			Message: "user not found or inactive",
		})
		return
	}

	c.JSON(http.StatusOK, response.VerifyResetTokenResponse{
		Valid:    true,
		Status:   "valid",
		Message:  "token valid",
		Email:    maskEmail(user.Email),
		Username: user.Username,
	})
}

// ResetPassword godoc
// @Summary Complete password reset
// @Description Validates a reset token and sets a new password adhering to NIST guidelines.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body request.ResetRequest true "Password reset payload"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid or expired token, or weak password"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/auth/reset-password [post]
func (s *Server) ResetPassword(c *gin.Context) {
	var req request.ResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidPayload)
		return
	}

	tokenRepo := s.getTokenRepository()
	userRepo := s.getUserRepository()
	if tokenRepo == nil || userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}

	tokenHash := auth.HashToken(req.Token)
	token, err := tokenRepo.FindResetTokenByHash(c.Request.Context(), tokenHash)
	if err != nil || token == nil {
		RespondError(c, http.StatusBadRequest, "invalid reset token")
		return
	}

	if token.UsedAt != nil {
		RespondError(c, http.StatusBadRequest, "reset token has already been used")
		return
	}

	if time.Now().After(token.ExpiresAt) {
		RespondError(c, http.StatusBadRequest, "reset token has expired")
		return
	}

	user, err := userRepo.FindByID(c.Request.Context(), token.UserID)
	if err != nil || user == nil {
		RespondError(c, http.StatusBadRequest, "user not found")
		return
	}

	// Enforce the NIST SP 800-63B password policy (length + blocklist + context-specific terms)
	if err := auth.ValidatePassword(req.Password, user.Username, user.Email); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "could not hash password")
		return
	}

	now := time.Now()
	if err := userRepo.UpdatePassword(c.Request.Context(), user.ID, hash, now); err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to update password")
		return
	}

	if err := tokenRepo.ConsumeResetToken(c.Request.Context(), token.ID, now, c.ClientIP()); err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to update reset token")
		return
	}

	// Invalidate any active refresh tokens for the user upon password reset
	_ = tokenRepo.RevokeUserTokens(c.Request.Context(), user.ID, now)

	slog.Info("password reset successfully completed", "user_id", user.ID, "ip", c.ClientIP())

	if s.Mailer != nil && user.Email != "" {
		go func(to, username string) {
			_ = s.Mailer.SendPasswordChangedEmail(to, username)
		}(user.Email, user.Username)
	}

	RespondMessage(c, http.StatusOK, "password updated")
}
