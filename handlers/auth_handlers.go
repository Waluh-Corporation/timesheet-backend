package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
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

// WebAuthnRelatedOrigins godoc
// @Summary WebAuthn related origins document
// @Description Serves the WebAuthn Related Origin Requests well-known document for multi-domain passkeys.
// @Tags Passkey
// @Produce json
// @Success 200 {object} response.OriginsResponse
// @Router /.well-known/webauthn [get]
func (s *Server) WebAuthnRelatedOrigins(c *gin.Context) {
	RespondSuccess(c, http.StatusOK, gin.H{"origins": s.Cfg.RPOrigins})
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

	userRepo := s.getUserRepository()
	tokenRepo := s.getTokenRepository()
	if userRepo != nil && tokenRepo != nil {
		if user, err := userRepo.FindByEmail(c.Request.Context(), req.Email); err == nil && user != nil {
			raw, hash, err := auth.GenerateResetToken()
			if err == nil {
				now := time.Now()
				_ = tokenRepo.InvalidateResetTokensByUserID(c.Request.Context(), user.ID, now)
				_ = tokenRepo.CreateResetToken(c.Request.Context(), &models.PasswordResetToken{
					UserID:    user.ID,
					TokenType: "password_reset",
					TokenHash: hash,
					ExpiresAt: now.Add(s.Cfg.ResetTokenTTL),
					CreatedIP: c.ClientIP(),
				})
				link := s.publicBaseURL(c) + "/reset-password?token=" + raw
				if s.Mailer != nil {
					go func(toEmail, username, resetLink string) {
						if err := s.Mailer.SendResetEmailWithUser(toEmail, username, resetLink); err != nil {
							slog.Error("failed to send password reset email", "error", err, "email", toEmail)
						}
					}(user.Email, user.Username, link)
				}
				slog.Info("password reset link issued", "user_id", user.ID, "ip", c.ClientIP())
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

// --- WebAuthn: registering a passkey (authenticated) ---

// BeginPasskeyRegistration godoc
// @Summary Begin passkey registration
// @Description Initiates a WebAuthn registration ceremony for the authenticated user.
// @Tags Passkey
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.PasskeySessionResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/passkey/register/begin [post]
func (s *Server) BeginPasskeyRegistration(c *gin.Context) {
	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	user, err := userRepo.FindByIDWithCredentials(c.Request.Context(), currentUserID(c))
	if err != nil || user == nil {
		RespondError(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Request a resident (discoverable) credential so the user can later sign in without typing a username.
	options, sessionData, err := s.WebAuthn.BeginRegistration(
		*user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
	)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	sid := uuid.NewString()
	s.putSession(sid, sessionData)
	RespondSuccess(c, http.StatusOK, gin.H{"session_id": sid, "options": options})
}

// FinishPasskeyRegistration godoc
// @Summary Finish passkey registration
// @Description Completes WebAuthn credential registration and stores the credential.
// @Tags Passkey
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param session_id query string true "Session ID returned from registration begin"
// @Param name query string false "Friendly name for the passkey"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid or expired session"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/passkey/register/finish [post]
func (s *Server) FinishPasskeyRegistration(c *gin.Context) {
	sid := c.Query("session_id")
	sessionData, ok := s.takeSession(sid)
	if !ok {
		RespondError(c, http.StatusBadRequest, "unknown or expired session")
		return
	}

	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	user, err := userRepo.FindByIDWithCredentials(c.Request.Context(), currentUserID(c))
	if err != nil || user == nil {
		RespondError(c, http.StatusNotFound, errUserNotFound)
		return
	}

	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	credential, err := s.finishRegistration(*user, *sessionData, c.Request)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	resolvePasskeyTransports(credential, bodyBytes)

	passkeyName := strings.TrimSpace(c.Query("name"))
	if passkeyName == "" {
		passkeyName = strings.TrimSpace(c.GetHeader("X-Passkey-Name"))
	}

	record := models.NewWebAuthnCredential(user.ID, credential, passkeyName)

	if err := userRepo.CreatePasskeyCredential(c.Request.Context(), &record); err != nil {
		RespondError(c, http.StatusInternalServerError, "could not save credential")
		return
	}

	info := models.GetAuthenticatorInfo(record.AAGUID)
	record.Icon = info.Icon

	slog.Info("passkey registered successfully", "user_id", user.ID, "name", record.FriendlyName, "ip", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"status":  "success",
		"message": "passkey registered",
		"data": gin.H{
			"id":                   record.ID,
			"friendly_name":        record.FriendlyName,
			"authenticator_aaguid": record.AuthenticatorAAGUID,
			"icon":                 record.Icon,
			"created_at":           record.CreatedAt,
		},
	})
}

func (s *Server) finishRegistration(user models.User, session webauthn.SessionData, r *http.Request) (*webauthn.Credential, error) {
	if s.finishRegistrationFunc != nil {
		return s.finishRegistrationFunc(user, session, r)
	}
	return s.WebAuthn.FinishRegistration(user, session, r)
}

func resolvePasskeyTransports(cred *webauthn.Credential, bodyBytes []byte) {
	if cred == nil {
		return
	}
	if len(cred.Transport) == 0 && len(bodyBytes) > 0 {
		var rawPayload struct {
			Transports              []string `json:"transports"`
			AuthenticatorAttachment string   `json:"authenticatorAttachment"`
			Response                struct {
				Transports []string `json:"transports"`
			} `json:"response"`
		}
		if err := json.Unmarshal(bodyBytes, &rawPayload); err == nil {
			var parsedTransports []string
			if len(rawPayload.Response.Transports) > 0 {
				parsedTransports = rawPayload.Response.Transports
			} else if len(rawPayload.Transports) > 0 {
				parsedTransports = rawPayload.Transports
			}
			for _, t := range parsedTransports {
				if trimmed := strings.TrimSpace(t); trimmed != "" {
					cred.Transport = append(cred.Transport, protocol.AuthenticatorTransport(trimmed))
				}
			}
			if rawPayload.AuthenticatorAttachment == "platform" && cred.Authenticator.Attachment == "" {
				cred.Authenticator.Attachment = protocol.Platform
			}
			if len(cred.Transport) == 0 && (rawPayload.AuthenticatorAttachment == "platform" || cred.Authenticator.Attachment == protocol.Platform) {
				cred.Transport = append(cred.Transport, protocol.Internal)
			}
		}
	}
}

// --- WebAuthn: passwordless login ---

// BeginPasskeyLogin godoc
// @Summary Begin passkey login
// @Description Starts WebAuthn assertion ceremony for passwordless sign-in (discoverable or username-scoped).
// @Tags Passkey
// @Accept json
// @Produce json
// @Param request body request.BeginPasskeyLoginRequest false "Optional user identifier"
// @Success 200 {object} response.PasskeySessionResponse
// @Failure 401 {object} response.ErrorResponse "Invalid credentials"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/auth/passkey/login/begin [post]
func (s *Server) BeginPasskeyLogin(c *gin.Context) {
	if s.WebAuthn == nil {
		RespondError(c, http.StatusInternalServerError, "webauthn not configured")
		return
	}

	var req request.BeginPasskeyLoginRequest
	_ = c.ShouldBindJSON(&req) // identifier is optional

	var (
		options     *protocol.CredentialAssertion
		sessionData *webauthn.SessionData
		err         error
	)

	if strings.TrimSpace(req.Identifier) == "" {
		options, sessionData, err = s.WebAuthn.BeginDiscoverableLogin()
	} else {
		userRepo := s.getUserRepository()
		if userRepo == nil {
			RespondError(c, http.StatusInternalServerError, "database repository unavailable")
			return
		}
		user, e := userRepo.FindByUsernameOrEmailWithCredentials(c.Request.Context(), req.Identifier)
		if e != nil || user == nil {
			RespondError(c, http.StatusUnauthorized, "invalid credentials")
			return
		}
		options, sessionData, err = s.WebAuthn.BeginLogin(*user)
	}
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	sid := uuid.NewString()
	s.putSession(sid, sessionData)
	RespondSuccess(c, http.StatusOK, gin.H{"session_id": sid, "options": options})
}

// FinishPasskeyLogin godoc
// @Summary Finish passkey login
// @Description Verifies WebAuthn assertion signature and returns JWT access token and refresh token on success.
// @Tags Passkey
// @Accept json
// @Produce json
// @Param session_id query string true "Session ID returned from begin ceremony"
// @Success 200 {object} response.LoginResponse
// @Failure 400 {object} response.ErrorResponse "Invalid or expired session"
// @Failure 401 {object} response.ErrorResponse "Invalid credentials"
// @Failure 403 {object} response.ErrorResponse "Account disabled"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/auth/passkey/login/finish [post]
func (s *Server) FinishPasskeyLogin(c *gin.Context) {
	sid := c.Query("session_id")
	sessionData, ok := s.takeSession(sid)
	if !ok {
		RespondError(c, http.StatusBadRequest, "unknown or expired session")
		return
	}

	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}

	var user *models.User
	var credential *webauthn.Credential
	var err error

	if len(sessionData.UserID) == 0 {
		// Discoverable login: resolve the user from the assertion's user handle.
		handler := func(_, userHandle []byte) (webauthn.User, error) {
			u, e := userRepo.FindByIDWithCredentials(c.Request.Context(), decodeUserHandle(userHandle))
			if e != nil || u == nil {
				return nil, e
			}
			user = u
			return *u, nil
		}
		credential, err = s.WebAuthn.FinishDiscoverableLogin(handler, *sessionData, c.Request)
	} else {
		user, err = userRepo.FindByIDWithCredentials(c.Request.Context(), decodeUserHandle(sessionData.UserID))
		if err != nil || user == nil {
			RespondError(c, http.StatusUnauthorized, "invalid credentials")
			return
		}
		credential, err = s.WebAuthn.FinishLogin(*user, *sessionData, c.Request)
	}
	if err != nil {
		RespondError(c, http.StatusUnauthorized, err.Error())
		return
	}
	if user == nil || !user.IsActive {
		RespondError(c, http.StatusForbidden, "account is disabled")
		return
	}

	// Persist the updated signature counter (clone detection) and backup state
	_ = userRepo.UpdatePasskeySignCount(c.Request.Context(), credential.ID, credential.Authenticator.SignCount, credential.Flags.BackupState)

	token, err := s.Auth.GenerateToken(user)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "could not issue token")
		return
	}

	// Generate Dual-Token refresh token
	rawRefreshToken, refreshHash, err := auth.GenerateRefreshToken()
	if err == nil {
		tokenRepo := s.getTokenRepository()
		if tokenRepo != nil {
			_ = tokenRepo.CreateRefreshToken(c.Request.Context(), &models.RefreshToken{
				UserID:    user.ID,
				TokenHash: refreshHash,
				FamilyID:  uuid.NewString(),
				ExpiresAt: time.Now().Add(s.Cfg.RefreshTokenTTL),
				CreatedIP: c.ClientIP(),
				UserAgent: c.Request.UserAgent(),
			})
		}
	}

	slog.Info("passkey login successful", "user_id", user.ID, "username", user.Username, "ip", c.ClientIP())
	RespondSuccess(c, http.StatusOK, response.LoginResponse{
		Token:        token,
		RefreshToken: rawRefreshToken,
		User:         *user,
	})
}

// --- Passkey management (self-service for any authenticated user) ---

// ListPasskeys godoc
// @Summary List current user's passkeys
// @Description Retrieves all registered passkeys for the currently authenticated user.
// @Tags Passkey
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.WebAuthnCredential
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/passkeys [get]
func (s *Server) ListPasskeys(c *gin.Context) {
	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	creds, err := userRepo.ListPasskeysByUserID(c.Request.Context(), currentUserID(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, creds)
}

// DeletePasskey godoc
// @Summary Delete current user's passkey
// @Description Deletes a registered passkey owned by the authenticated user.
// @Tags Passkey
// @Security BearerAuth
// @Produce json
// @Param id path int true "Passkey credential ID"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid passkey ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Passkey not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// DeletePasskey removes a registered WebAuthn credential owned by the caller.
func (s *Server) DeletePasskey(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid passkey ID")
		return
	}
	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	uid := currentUserID(c)
	deleted, err := userRepo.DeletePasskey(c.Request.Context(), uint(id64), &uid)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		RespondError(c, http.StatusNotFound, "passkey not found")
		return
	}
	slog.Info("passkey deleted", "user_id", uid, "passkey_id", id64, "ip", c.ClientIP())
	RespondMessage(c, http.StatusOK, "passkey removed")
}

// UpdatePasskey godoc
// @Summary Update passkey name
// @Description Updates the friendly name of a registered passkey owned by the authenticated user (accessible to all roles; users can only rename their own passkeys).
// @Tags Passkey
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Passkey credential ID"
// @Param request body request.UpdatePasskeyRequest true "Updated passkey name"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid passkey ID or payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Passkey not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/passkeys/{id} [patch]
func (s *Server) UpdatePasskey(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid passkey ID")
		return
	}
	var req request.UpdatePasskeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidPayload)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		RespondError(c, http.StatusBadRequest, "passkey name cannot be empty")
		return
	}

	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	uid := currentUserID(c)
	updated, err := userRepo.UpdatePasskeyName(c.Request.Context(), uint(id64), &uid, name)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !updated {
		RespondError(c, http.StatusNotFound, "passkey not found")
		return
	}
	slog.Info("passkey updated", "user_id", uid, "passkey_id", id64, "name", name, "ip", c.ClientIP())
	RespondMessage(c, http.StatusOK, "passkey updated")
}

// --- Passkey management (admin, for any user) ---

// AdminListPasskeys godoc
// @Summary List passkeys for a user (Admin)
// @Description Retrieves all registered passkeys for the specified user (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {array} response.AdminPasskeyResponse
// @Failure 400 {object} response.ErrorResponse "Invalid user ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users/{id}/passkeys [get]
func (s *Server) AdminListPasskeys(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid user ID")
		return
	}
	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	creds, err := userRepo.ListPasskeysByUserID(c.Request.Context(), uint(id64))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]response.AdminPasskeyResponse, len(creds))
	for i, cr := range creds {
		resp[i] = response.AdminPasskeyResponse{
			ID:                  cr.ID,
			FriendlyName:        cr.FriendlyName,
			AuthenticatorAAGUID: cr.AuthenticatorAAGUID,
			Icon:                cr.Icon,
			CreatedAt:           cr.CreatedAt,
		}
	}
	RespondSuccess(c, http.StatusOK, resp)
}

// AdminDeletePasskey godoc
// @Summary Delete a passkey for a user (Forbidden)
// @Description Passkeys are strictly user-managed credentials; admins cannot delete user passkeys.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Param pid path int true "Passkey ID"
// @Failure 403 {object} response.ErrorResponse "Admin cannot delete user passkeys"
// @Router /api/v1/admin/users/{id}/passkeys/{pid} [delete]
func (s *Server) AdminDeletePasskey(c *gin.Context) {
	RespondError(c, http.StatusForbidden, "admin cannot delete user passkeys; passkeys are strictly user-managed credentials")
}

// AdminUpdatePasskey godoc
// @Summary Update a passkey name for a user (Forbidden)
// @Description Passkeys are strictly user-managed credentials; admins cannot modify user passkeys.
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param pid path int true "Passkey ID"
// @Param request body request.UpdatePasskeyRequest true "Updated passkey name"
// @Failure 403 {object} response.ErrorResponse "Admin cannot modify user passkeys"
// @Router /api/v1/admin/users/{id}/passkeys/{pid} [patch]
func (s *Server) AdminUpdatePasskey(c *gin.Context) {
	RespondError(c, http.StatusForbidden, "admin cannot modify user passkeys; passkeys are strictly user-managed credentials")
}

// decodeUserHandle reverses User.WebAuthnID (little-endian uint64 -> id).
func decodeUserHandle(b []byte) uint {
	var id uint
	for i := 0; i < len(b) && i < 8; i++ {
		id |= uint(b[i]) << (8 * i)
	}
	return id
}
