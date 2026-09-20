package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

const (
	errInvalidPayload = "invalid payload"
	errInvalidAuth    = "invalid credentials" //nolint:gosec // G101: error message text, not a credential
	errUserNotFound   = "user not found"
)

// Login godoc
// @Summary Authenticate user with credentials
// @Description Authenticates user with username/email and password, returning a JWT access token, refresh token, and user profile.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body request.LoginRequest true "Login credentials"
// @Success 200 {object} response.LoginResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials"
// @Failure 403 {object} response.ErrorResponse "Account is disabled"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/auth/login [post]
func (s *Server) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidPayload)
		return
	}

	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}

	user, err := userRepo.FindByUsernameOrEmail(c.Request.Context(), req.Identifier)
	if err != nil || user == nil {
		slog.Warn("login failed: user not found", "identifier", req.Identifier, "ip", c.ClientIP())
		RespondError(c, http.StatusUnauthorized, errInvalidAuth)
		return
	}
	if !user.IsActive {
		slog.Warn("login failed: account deactivated", "user_id", user.ID, "username", user.Username, "ip", c.ClientIP())
		RespondError(c, http.StatusForbidden, "account is disabled")
		return
	}
	if user.PasswordHash == "" || !auth.CheckPassword(user.PasswordHash, req.Password) {
		slog.Warn("login failed: invalid password", "user_id", user.ID, "username", user.Username, "ip", c.ClientIP())
		RespondError(c, http.StatusUnauthorized, errInvalidAuth)
		return
	}

	// Opportunistically upgrade weaker hashes (e.g. from previous Argon2id parameter
	// configurations) to the current Argon2id parameters now that we have the plaintext in hand.
	if auth.NeedsRehash(user.PasswordHash) {
		if newHash, herr := auth.HashPassword(req.Password); herr == nil {
			_ = userRepo.UpdatePassword(c.Request.Context(), user.ID, newHash, time.Now())
		}
	}

	token, err := s.Auth.GenerateToken(user)
	if err != nil {
		slog.Error("failed to generate access token", "user_id", user.ID, "error", err)
		RespondError(c, http.StatusInternalServerError, "could not issue token")
		return
	}

	// Issue Dual-Token: Cryptographically secure Refresh Token
	rawRefreshToken, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.Error("failed to generate refresh token", "user_id", user.ID, "error", err)
		RespondError(c, http.StatusInternalServerError, "could not issue refresh token")
		return
	}

	tokenRepo := s.getTokenRepository()
	if tokenRepo != nil {
		refreshRecord := &models.RefreshToken{
			UserID:    user.ID,
			TokenHash: refreshHash,
			FamilyID:  uuid.NewString(),
			ExpiresAt: time.Now().Add(s.Cfg.RefreshTokenTTL),
			CreatedIP: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}
		if terr := tokenRepo.CreateRefreshToken(c.Request.Context(), refreshRecord); terr != nil {
			slog.Error("failed to persist refresh token", "user_id", user.ID, "error", terr)
			RespondError(c, http.StatusInternalServerError, "could not persist refresh token")
			return
		}
	}

	slog.Info("user logged in successfully", "user_id", user.ID, "username", user.Username, "ip", c.ClientIP(), "user_agent", c.Request.UserAgent())
	RespondSuccess(c, http.StatusOK, response.LoginResponse{
		Token:        token,
		RefreshToken: rawRefreshToken,
		User:         *user,
	})
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Validates refresh token, rotates it, and issues a new access token and rotated refresh token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body request.RefreshRequest true "Refresh token payload"
// @Success 200 {object} response.RefreshResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload"
// @Failure 401 {object} response.ErrorResponse "Invalid, expired, or reused token"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/auth/refresh [post]
func (s *Server) RefreshToken(c *gin.Context) {
	var req request.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		RespondError(c, http.StatusBadRequest, errInvalidPayload)
		return
	}

	tokenRepo := s.getTokenRepository()
	userRepo := s.getUserRepository()
	if tokenRepo == nil || userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "service unavailable")
		return
	}

	hash := auth.HashToken(req.RefreshToken)
	tokenRecord, err := tokenRepo.FindRefreshTokenByHash(c.Request.Context(), hash)
	if err != nil || tokenRecord == nil {
		slog.Warn("refresh token lookup failed", "ip", c.ClientIP())
		RespondError(c, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	// Token Reuse Detection: If token is already revoked, an attacker or compromised client is reusing it
	if tokenRecord.RevokedAt != nil {
		slog.Warn("SECURITY ALERT: refresh token reuse detected; revoking family",
			"family_id", tokenRecord.FamilyID,
			"user_id", tokenRecord.UserID,
			"ip", c.ClientIP(),
		)
		// Revoke the entire family
		_ = tokenRepo.RevokeFamily(c.Request.Context(), tokenRecord.FamilyID, time.Now())
		RespondError(c, http.StatusUnauthorized, "token reuse detected, please sign in again")
		return
	}

	// Expiration check
	if time.Now().After(tokenRecord.ExpiresAt) {
		slog.Warn("expired refresh token presented", "user_id", tokenRecord.UserID, "ip", c.ClientIP())
		RespondError(c, http.StatusUnauthorized, "refresh token expired")
		return
	}

	// Validate user status
	user, uerr := userRepo.FindByID(c.Request.Context(), tokenRecord.UserID)
	if uerr != nil || user == nil || !user.IsActive {
		slog.Warn("refresh attempt for deactivated user", "user_id", tokenRecord.UserID, "ip", c.ClientIP())
		RespondError(c, http.StatusUnauthorized, "Account is deactivated")
		return
	}

	// Rotate token: revoke current token
	now := time.Now()
	if err := tokenRepo.RevokeRefreshToken(c.Request.Context(), tokenRecord.ID, now); err != nil {
		slog.Error("failed to revoke old refresh token during rotation", "token_id", tokenRecord.ID, "error", err)
		RespondError(c, http.StatusInternalServerError, "failed to rotate token")
		return
	}

	// Generate new access token
	newAccessToken, err := s.Auth.GenerateToken(user)
	if err != nil {
		slog.Error("failed to generate new access token", "user_id", user.ID, "error", err)
		RespondError(c, http.StatusInternalServerError, "could not issue token")
		return
	}

	// Generate new rotated refresh token in the same family
	newRaw, newHash, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.Error("failed to generate new refresh token", "user_id", user.ID, "error", err)
		RespondError(c, http.StatusInternalServerError, "could not issue refresh token")
		return
	}

	newRecord := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: newHash,
		FamilyID:  tokenRecord.FamilyID,
		ExpiresAt: now.Add(s.Cfg.RefreshTokenTTL),
		CreatedIP: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
	if err := tokenRepo.CreateRefreshToken(c.Request.Context(), newRecord); err != nil {
		slog.Error("failed to persist rotated refresh token", "user_id", user.ID, "error", err)
		RespondError(c, http.StatusInternalServerError, "could not persist refresh token")
		return
	}

	slog.Info("refresh token rotated successfully",
		"user_id", user.ID,
		"family_id", tokenRecord.FamilyID,
		"ip", c.ClientIP(),
	)

	RespondSuccess(c, http.StatusOK, response.RefreshResponse{
		Token:        newAccessToken,
		RefreshToken: newRaw,
	})
}

// Logout godoc
// @Summary Revoke session and refresh token
// @Description Invalidate the refresh token on server logout.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body request.LogoutRequest false "Optional refresh token to revoke"
// @Success 200 {object} response.MessageResponse
// @Router /api/v1/auth/logout [post]
func (s *Server) Logout(c *gin.Context) {
	var req request.LogoutRequest
	_ = c.ShouldBindJSON(&req)

	if req.RefreshToken != "" {
		tokenRepo := s.getTokenRepository()
		if tokenRepo != nil {
			hash := auth.HashToken(req.RefreshToken)
			if tokenRecord, err := tokenRepo.FindRefreshTokenByHash(c.Request.Context(), hash); err == nil && tokenRecord != nil {
				_ = tokenRepo.RevokeRefreshToken(c.Request.Context(), tokenRecord.ID, time.Now())
				slog.Info("refresh token revoked on logout", "token_id", tokenRecord.ID, "user_id", tokenRecord.UserID, "ip", c.ClientIP())
			}
		}
	}

	RespondMessage(c, http.StatusOK, "logged out successfully")
}

// Me godoc
// @Summary Get current user profile
// @Description Returns the profile information of the currently authenticated user.
// @Tags User
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.User
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Router /api/v1/me [get]
func (s *Server) Me(c *gin.Context) {
	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	user, err := userRepo.FindByID(c.Request.Context(), currentUserID(c))
	if err != nil || user == nil {
		RespondError(c, http.StatusNotFound, errUserNotFound)
		return
	}
	RespondSuccess(c, http.StatusOK, user)
}
