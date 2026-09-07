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

// loginRequest is the username/email + password payload.
type loginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // username or email
	Password   string `json:"password" binding:"required"`
}

// Login authenticates with username/email + password and returns a JWT.
func (s *Server) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var user models.User
	err := s.DB.Where("username = ? OR email = ?", req.Identifier, req.Identifier).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is disabled"})
		return
	}
	if user.PasswordHash == "" || !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

// WebAuthnRelatedOrigins serves the WebAuthn "Related Origin Requests"
// well-known document at /.well-known/webauthn. When a passkey's relying party
// (RPID) is served across several registrable domains, a browser performing a
// ceremony on one of those sibling origins fetches this document from the RPID
// host and allows the ceremony if its origin is listed here. Combined with the
// multi-value WEBAUTHN_RP_ORIGIN allow-list this is what makes passkeys usable
// across multiple domains rather than a single one.
//
// See: https://w3c.github.io/webauthn/#sctn-related-origins
func (s *Server) WebAuthnRelatedOrigins(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"origins": s.Cfg.RPOrigins})
}

// Me returns the currently authenticated user record.
func (s *Server) Me(c *gin.Context) {
	var user models.User
	if err := s.DB.Preload("CompanyRef").Preload("DivisionRef").Preload("SiteRef").
		First(&user, currentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// forgotRequest triggers a reset email.
type forgotRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword issues a reset token and emails a reset link. To avoid user
// enumeration it always returns 200 regardless of whether the email exists.
func (s *Server) ForgotPassword(c *gin.Context) {
	var req forgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var user models.User
	if err := s.DB.Where("email = ?", req.Email).First(&user).Error; err == nil {
		raw, hash, err := auth.GenerateResetToken()
		if err == nil {
			s.DB.Create(&models.PasswordResetToken{
				UserID:    user.ID,
				TokenHash: hash,
				ExpiresAt: time.Now().Add(s.Cfg.ResetTokenTTL),
			})
			link := s.publicBaseURL(c) + "/reset-password?token=" + raw
			_ = s.Mailer.SendResetEmail(user.Email, link)
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "if the email exists, a reset link has been sent"})
}

// resetRequest completes a password reset. The min length mirrors the NIST
// policy; ValidatePassword performs the full check (blocklist + context) below.
type resetRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// ResetPassword validates a reset token and sets a new password.
func (s *Server) ResetPassword(c *gin.Context) {
	var req resetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var token models.PasswordResetToken
	err := s.DB.Where("token_hash = ? AND used = ? AND expires_at > ?", auth.HashToken(req.Token), false, time.Now()).First(&token).Error
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired token"})
		return
	}

	// Enforce the NIST SP 800-63B password policy (length + blocklist +
	// context-specific terms) before accepting the new secret. Look up the
	// account so its username/email can be treated as context-specific words.
	var user models.User
	_ = s.DB.First(&user, token.UserID).Error
	if err := auth.ValidatePassword(req.Password, user.Username, user.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
		return
	}

	s.DB.Model(&models.User{}).Where("id = ?", token.UserID).Update("password_hash", hash)
	s.DB.Model(&token).Update("used", true)

	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

// --- WebAuthn: registering a passkey (authenticated) ---

// BeginPasskeyRegistration starts a passkey registration ceremony for the
// currently authenticated user.
func (s *Server) BeginPasskeyRegistration(c *gin.Context) {
	var user models.User
	if err := s.DB.Preload("Credentials").First(&user, currentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Request a resident (discoverable) credential so the user can later sign in
	// without typing a username.
	options, sessionData, err := s.WebAuthn.BeginRegistration(
		user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sid := uuid.NewString()
	s.putSession(sid, sessionData)
	c.JSON(http.StatusOK, gin.H{"session_id": sid, "options": options})
}

// FinishPasskeyRegistration completes and stores a new passkey credential.
func (s *Server) FinishPasskeyRegistration(c *gin.Context) {
	sid := c.Query("session_id")
	sessionData, ok := s.takeSession(sid)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown or expired session"})
		return
	}

	var user models.User
	if err := s.DB.Preload("Credentials").First(&user, currentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	credential, err := s.WebAuthn.FinishRegistration(user, *sessionData, c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := models.NewWebAuthnCredential(user.ID, credential, c.Query("name"))
	if err := s.DB.Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save credential"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "passkey registered"})
}

// --- WebAuthn: passwordless login ---

// beginPasskeyLoginRequest optionally carries a username/email. When empty the
// ceremony is usernameless (discoverable): the browser offers whatever resident
// passkey it holds for this site and the user is resolved from the assertion.
type beginPasskeyLoginRequest struct {
	Identifier string `json:"identifier"`
}

// BeginPasskeyLogin starts an assertion ceremony. With an identifier it scopes
// the challenge to that user's credentials; without one it starts a
// discoverable-credential login so no username is required.
func (s *Server) BeginPasskeyLogin(c *gin.Context) {
	var req beginPasskeyLoginRequest
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		options, sessionData, err = s.WebAuthn.BeginLogin(user)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sid := uuid.NewString()
	s.putSession(sid, sessionData)
	c.JSON(http.StatusOK, gin.H{"session_id": sid, "options": options})
}

// FinishPasskeyLogin validates the assertion and returns a JWT on success. It
// handles both the scoped and the usernameless (discoverable) flows.
func (s *Server) FinishPasskeyLogin(c *gin.Context) {
	sid := c.Query("session_id")
	sessionData, ok := s.takeSession(sid)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown or expired session"})
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		credential, err = s.WebAuthn.FinishLogin(user, *sessionData, c.Request)
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is disabled"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

// --- Passkey management (self-service for any authenticated user) ---

// ListPasskeys returns the current user's registered passkeys.
func (s *Server) ListPasskeys(c *gin.Context) {
	var creds []models.WebAuthnCredential
	if err := s.DB.Where("user_id = ?", currentUserID(c)).
		Order("created_at desc").Find(&creds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, creds)
}

// DeletePasskey removes one of the current user's own passkeys.
func (s *Server) DeletePasskey(c *gin.Context) {
	res := s.DB.Where("id = ? AND user_id = ?", c.Param("id"), currentUserID(c)).
		Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "passkey not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "passkey removed"})
}

// --- Passkey management (admin, for any user) ---

// AdminListPasskeys lists a given user's passkeys (admin only).
func (s *Server) AdminListPasskeys(c *gin.Context) {
	var creds []models.WebAuthnCredential
	if err := s.DB.Where("user_id = ?", c.Param("id")).
		Order("created_at desc").Find(&creds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, creds)
}

// AdminDeletePasskey removes a given user's passkey (admin only) — e.g. to let a
// user recover after losing a device or a broken credential.
func (s *Server) AdminDeletePasskey(c *gin.Context) {
	res := s.DB.Where("id = ? AND user_id = ?", c.Param("pid"), c.Param("id")).
		Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "passkey not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "passkey removed"})
}

// decodeUserHandle reverses User.WebAuthnID (little-endian uint64 -> id).
func decodeUserHandle(b []byte) uint {
	var id uint
	for i := 0; i < len(b) && i < 8; i++ {
		id |= uint(b[i]) << (8 * i)
	}
	return id
}
