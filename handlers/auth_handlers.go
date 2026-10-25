package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/service"
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

	authSvc := s.getAuthService()
	if authSvc == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}

	result, err := authSvc.Login(c.Request.Context(), service.LoginDTO{
		Identifier: req.Identifier,
		Password:   req.Password,
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})
	if err != nil {
		RespondError(c, domain.MapErrorToHTTPStatus(err), err.Error())
		return
	}
	if result == nil {
		RespondError(c, http.StatusInternalServerError, "could not issue token")
		return
	}

	RespondSuccess(c, http.StatusOK, response.LoginResponse{
		Token:        result.AccessToken,
		RefreshToken: result.RefreshToken,
		User:         response.ToUserResponse(result.User),
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

	authSvc := s.getAuthService()
	if authSvc == nil {
		RespondError(c, http.StatusInternalServerError, "service unavailable")
		return
	}

	result, err := authSvc.RefreshToken(c.Request.Context(), service.RefreshTokenDTO{
		RefreshToken: req.RefreshToken,
		ClientIP:     c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
	})
	if err != nil {
		RespondError(c, domain.MapErrorToHTTPStatus(err), err.Error())
		return
	}
	if result == nil {
		RespondError(c, http.StatusInternalServerError, "failed to rotate token")
		return
	}

	RespondSuccess(c, http.StatusOK, response.RefreshResponse{
		Token:        result.AccessToken,
		RefreshToken: result.RefreshToken,
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
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		slog.Debug("optional logout payload binding skipped", "error", err)
	}

	s.blacklistAccessToken(c.Request.Context(), c.GetHeader("Authorization"))
	if authSvc := s.getAuthService(); authSvc != nil {
		if err := authSvc.RevokeRefreshToken(c.Request.Context(), req.RefreshToken, c.ClientIP()); err != nil {
			slog.Warn("failed to revoke refresh token on logout", "error", err, "ip", c.ClientIP())
		}
	}

	RespondMessage(c, http.StatusOK, "logged out successfully")
}

func (s *Server) blacklistAccessToken(ctx context.Context, authHeader string) {
	if !strings.HasPrefix(authHeader, "Bearer ") || s.Auth == nil {
		return
	}
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := s.Auth.ParseToken(rawToken)
	if err != nil || claims == nil || claims.ID == "" || claims.ExpiresAt == nil {
		return
	}
	rdb := s.getRedisClient()
	if rdb == nil {
		return
	}
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		return
	}
	blacklistKey := fmt.Sprintf("jwt:blacklist:%s", claims.ID)
	if err := rdb.Set(ctx, blacklistKey, "revoked", remaining).Err(); err != nil {
		slog.Warn("failed to blacklist access token in redis", "jti", claims.ID, "user_id", claims.UserID, "error", err)
		return
	}
	slog.Info("access token blacklisted in redis", "jti", claims.ID, "user_id", claims.UserID)
}

// Me godoc
// @Summary Get current user profile
// @Description Returns the profile information of the currently authenticated user.
// @Tags User
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.UserResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Router /api/v1/me [get]
func (s *Server) Me(c *gin.Context) {
	authSvc := s.getAuthService()
	if authSvc == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}
	user, err := authSvc.GetProfile(c.Request.Context(), currentUserID(c))
	if err != nil {
		RespondError(c, domain.MapErrorToHTTPStatus(err), err.Error())
		return
	}
	if user == nil {
		RespondError(c, http.StatusNotFound, errUserNotFound)
		return
	}
	RespondSuccess(c, http.StatusOK, response.ToUserResponse(user))
}
