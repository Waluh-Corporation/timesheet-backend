package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"timesheet-backend/auth"
	"timesheet-backend/models"
)

// Login godoc
// @Summary Authenticate user with credentials
// @Description Authenticates user with username/email and password, returning a JWT token and user profile.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} models.ErrorResponse "Invalid payload"
// @Failure 401 {object} models.ErrorResponse "Invalid credentials"
// @Failure 403 {object} models.ErrorResponse "Account is disabled"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/auth/login [post]
func (s *Server) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid payload")
		return
	}

	var user models.User
	err := s.DB.Where("username = ? OR email = ?", req.Identifier, req.Identifier).First(&user).Error
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !user.IsActive {
		RespondError(c, http.StatusForbidden, "account is disabled")
		return
	}
	if user.PasswordHash == "" || !auth.CheckPassword(user.PasswordHash, req.Password) {
		RespondError(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Opportunistically upgrade legacy/weaker hashes (e.g. bcrypt from before the
	// Argon2id migration) to the current Argon2id parameters now that we have the
	// plaintext in hand.
	if auth.NeedsRehash(user.PasswordHash) {
		if newHash, herr := auth.HashPassword(req.Password); herr == nil {
			s.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("password_hash", newHash)
		}
	}

	token, err := s.Auth.GenerateToken(&user)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "could not issue token")
		return
	}
	RespondSuccess(c, http.StatusOK, gin.H{"token": token, "user": user})
}

// WebAuthnRelatedOrigins godoc
// @Summary WebAuthn related origins document
// @Description Serves the WebAuthn Related Origin Requests well-known document for multi-domain passkeys.
// @Tags Passkey
// @Produce json
// @Success 200 {object} models.OriginsResponse
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
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Router /api/v1/me [get]
func (s *Server) Me(c *gin.Context) {
	var user models.User
	if err := s.DB.First(&user, currentUserID(c)).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
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
// @Param request body models.ForgotRequest true "User email"
// @Success 200 {object} models.MessageResponse
// @Failure 400 {object} models.ErrorResponse "Invalid payload"
// @Router /api/v1/auth/forgot-password [post]
func (s *Server) ForgotPassword(c *gin.Context) {
	var req models.ForgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid payload")
		return
	}

	var user models.User
	if err := s.DB.Where("email = ?", req.Email).First(&user).Error; err == nil {
		raw, hash, err := auth.GenerateResetToken()
		if err == nil {
			now := time.Now()
			// Invalidate any previously unconsumed active tokens for this user
			s.DB.Model(&models.PasswordResetToken{}).
				Where("user_id = ? AND used_at IS NULL", user.ID).
				Update("used_at", now)

			s.DB.Create(&models.PasswordResetToken{
				UserID:    user.ID,
				TokenType: "password_reset",
				TokenHash: hash,
				ExpiresAt: now.Add(s.Cfg.ResetTokenTTL),
				CreatedIP: c.ClientIP(),
			})
			link := s.publicBaseURL(c) + "/reset-password?token=" + raw
			_ = s.Mailer.SendResetEmail(user.Email, link)
		}
	}
	RespondMessage(c, http.StatusOK, "if the email exists, a reset link has been sent")
}

// ResetPassword godoc
// @Summary Complete password reset
// @Description Validates a reset token and sets a new password adhering to NIST guidelines.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.ResetRequest true "Password reset payload"
// @Success 200 {object} models.MessageResponse
// @Failure 400 {object} models.ErrorResponse "Invalid or expired token, or weak password"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/auth/reset-password [post]
func (s *Server) ResetPassword(c *gin.Context) {
	var req models.ResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid payload")
		return
	}

	var token models.PasswordResetToken
	err := s.DB.Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", auth.HashToken(req.Token), time.Now()).First(&token).Error
	if err != nil {
		RespondError(c, http.StatusBadRequest, "invalid or expired token")
		return
	}

	// Enforce the NIST SP 800-63B password policy (length + blocklist +
	// context-specific terms) before accepting the new secret. Look up the
	// account so its username/email can be treated as context-specific words.
	var user models.User
	_ = s.DB.First(&user, token.UserID).Error
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
	s.DB.Model(&models.User{}).Where("id = ?", token.UserID).Update("password_hash", hash)
	s.DB.Model(&token).Updates(map[string]interface{}{
		"used_at": now,
		"used_ip": c.ClientIP(),
	})

	RespondMessage(c, http.StatusOK, "password updated")
}

// --- WebAuthn: registering a passkey (authenticated) ---

// BeginPasskeyRegistration godoc
// @Summary Begin passkey registration
// @Description Initiates a WebAuthn registration ceremony for the authenticated user.
// @Tags Passkey
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.PasskeySessionResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/passkey/register/begin [post]
func (s *Server) BeginPasskeyRegistration(c *gin.Context) {
	var user models.User
	if err := s.DB.Preload("Credentials").First(&user, currentUserID(c)).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	// Request a resident (discoverable) credential so the user can later sign in
	// without typing a username.
	options, sessionData, err := s.WebAuthn.BeginRegistration(
		user,
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
// @Success 200 {object} models.MessageResponse
// @Failure 400 {object} models.ErrorResponse "Invalid or expired session"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/passkey/register/finish [post]
func (s *Server) FinishPasskeyRegistration(c *gin.Context) {
	sid := c.Query("session_id")
	sessionData, ok := s.takeSession(sid)
	if !ok {
		RespondError(c, http.StatusBadRequest, "unknown or expired session")
		return
	}

	var user models.User
	if err := s.DB.Preload("Credentials").First(&user, currentUserID(c)).Error; err != nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return
	}

	credential, err := s.WebAuthn.FinishRegistration(user, *sessionData, c.Request)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	record := models.NewWebAuthnCredential(user.ID, credential, c.Query("name"))
	if err := s.DB.Create(&record).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "could not save credential")
		return
	}
	RespondMessage(c, http.StatusOK, "passkey registered")
}

// --- WebAuthn: passwordless login ---

// BeginPasskeyLogin godoc
// @Summary Begin passkey login
// @Description Starts WebAuthn assertion ceremony for passwordless sign-in (discoverable or username-scoped).
// @Tags Passkey
// @Accept json
// @Produce json
// @Param request body models.BeginPasskeyLoginRequest false "Optional user identifier"
// @Success 200 {object} models.PasskeySessionResponse
// @Failure 401 {object} models.ErrorResponse "Invalid credentials"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/auth/passkey/login/begin [post]
func (s *Server) BeginPasskeyLogin(c *gin.Context) {
	var req models.BeginPasskeyLoginRequest
	_ = c.ShouldBindJSON(&req) // identifier is optional

	var (
		options     *protocol.CredentialAssertion
		sessionData *webauthn.SessionData
		err         error
	)

	if strings.TrimSpace(req.Identifier) == "" {
		options, sessionData, err = s.WebAuthn.BeginDiscoverableLogin()
	} else {
		var user models.User
		if e := s.DB.Preload("Credentials").
			Where("username = ? OR email = ?", req.Identifier, req.Identifier).
			First(&user).Error; e != nil {
			RespondError(c, http.StatusUnauthorized, "invalid credentials")
			return
		}
		options, sessionData, err = s.WebAuthn.BeginLogin(user)
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
// @Description Verifies WebAuthn assertion signature and returns a JWT token on success.
// @Tags Passkey
// @Accept json
// @Produce json
// @Param session_id query string true "Session ID returned from begin ceremony"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} models.ErrorResponse "Invalid or expired session"
// @Failure 401 {object} models.ErrorResponse "Invalid credentials"
// @Failure 403 {object} models.ErrorResponse "Account disabled"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/auth/passkey/login/finish [post]
func (s *Server) FinishPasskeyLogin(c *gin.Context) {
	sid := c.Query("session_id")
	sessionData, ok := s.takeSession(sid)
	if !ok {
		RespondError(c, http.StatusBadRequest, "unknown or expired session")
		return
	}

	var user models.User
	var credential *webauthn.Credential
	var err error

	if len(sessionData.UserID) == 0 {
		// Discoverable login: resolve the user from the assertion's user handle.
		handler := func(_, userHandle []byte) (webauthn.User, error) {
			if e := s.DB.Preload("Credentials").First(&user, decodeUserHandle(userHandle)).Error; e != nil {
				return nil, e
			}
			return user, nil
		}
		credential, err = s.WebAuthn.FinishDiscoverableLogin(handler, *sessionData, c.Request)
	} else {
		if e := s.DB.Preload("Credentials").First(&user, decodeUserHandle(sessionData.UserID)).Error; e != nil {
			RespondError(c, http.StatusUnauthorized, "invalid credentials")
			return
		}
		credential, err = s.WebAuthn.FinishLogin(user, *sessionData, c.Request)
	}
	if err != nil {
		RespondError(c, http.StatusUnauthorized, err.Error())
		return
	}
	if !user.IsActive {
		RespondError(c, http.StatusForbidden, "account is disabled")
		return
	}

	// Persist the updated signature counter (clone detection) and backup state,
	// which the spec allows to change over the credential's lifetime.
	s.DB.Model(&models.WebAuthnCredential{}).
		Where("credential_id = ?", credential.ID).
		Updates(map[string]interface{}{
			"sign_count":   credential.Authenticator.SignCount,
			"backup_state": credential.Flags.BackupState,
		})

	token, err := s.Auth.GenerateToken(&user)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "could not issue token")
		return
	}
	RespondSuccess(c, http.StatusOK, gin.H{"token": token, "user": user})
}

// --- Passkey management (self-service for any authenticated user) ---

// ListPasskeys godoc
// @Summary List current user's passkeys
// @Description Retrieves all registered passkeys for the currently authenticated user.
// @Tags Passkey
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.WebAuthnCredential
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/passkeys [get]
func (s *Server) ListPasskeys(c *gin.Context) {
	var creds []models.WebAuthnCredential
	if err := s.DB.Where("user_id = ?", currentUserID(c)).
		Order("created_at desc").Find(&creds).Error; err != nil {
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
// @Success 200 {object} models.MessageResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Passkey not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/passkeys/{id} [delete]
func (s *Server) DeletePasskey(c *gin.Context) {
	res := s.DB.Where("id = ? AND user_id = ?", c.Param("id"), currentUserID(c)).
		Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		RespondError(c, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		RespondError(c, http.StatusNotFound, "passkey not found")
		return
	}
	RespondMessage(c, http.StatusOK, "passkey removed")
}

// --- Passkey management (admin, for any user) ---

// AdminListPasskeys godoc
// @Summary List passkeys for a user (Admin)
// @Description Retrieves all registered passkeys for the specified user (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {array} models.WebAuthnCredential
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users/{id}/passkeys [get]
func (s *Server) AdminListPasskeys(c *gin.Context) {
	var creds []models.WebAuthnCredential
	if err := s.DB.Where("user_id = ?", c.Param("id")).
		Order("created_at desc").Find(&creds).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, creds)
}

// AdminDeletePasskey godoc
// @Summary Delete a passkey for a user (Admin)
// @Description Removes a specified passkey belonging to a user (admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Param pid path int true "Passkey ID"
// @Success 200 {object} models.MessageResponse
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Admin only"
// @Failure 404 {object} models.ErrorResponse "Passkey not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/admin/users/{id}/passkeys/{pid} [delete]
func (s *Server) AdminDeletePasskey(c *gin.Context) {
	res := s.DB.Where("id = ? AND user_id = ?", c.Param("pid"), c.Param("id")).
		Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		RespondError(c, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		RespondError(c, http.StatusNotFound, "passkey not found")
		return
	}
	RespondMessage(c, http.StatusOK, "passkey removed")
}

// decodeUserHandle reverses User.WebAuthnID (little-endian uint64 -> id).
func decodeUserHandle(b []byte) uint {
	var id uint
	for i := 0; i < len(b) && i < 8; i++ {
		id |= uint(b[i]) << (8 * i)
	}
	return id
}
