package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/models"
)

const (
	ctxUserID = "userID"
	ctxRole   = "userRole"
	queryID   = "id = ?"
)

// AuthMiddleware validates the bearer JWT and injects the caller identity.
// It also enforces immediate account revocation by verifying that the user account
// exists and is currently active (is_active = true) in the database.
func (s *Server) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := extractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			RespondAbortError(c, http.StatusUnauthorized, "missing bearer token")
			return
		}

		claims, err := s.Auth.ParseToken(token)
		if err != nil {
			RespondAbortError(c, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := c.Request.Context()
		if s.isTokenBlacklisted(ctx, claims.ID) {
			RespondAbortError(c, http.StatusUnauthorized, "token has been revoked")
			return
		}

		if !s.isAccountActive(ctx, claims.UserID) {
			RespondAbortError(c, http.StatusUnauthorized, "account is deactivated or suspended")
			return
		}

		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

func extractBearerToken(header string) (string, bool) {
	if header == "" || !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	return strings.TrimPrefix(header, "Bearer "), true
}

func (s *Server) isTokenBlacklisted(ctx context.Context, tokenID string) bool {
	if tokenID == "" {
		return false
	}
	rdb := s.getRedisClient()
	if rdb == nil {
		return false
	}
	blacklistKey := fmt.Sprintf("jwt:blacklist:%s", tokenID)
	exists, err := rdb.Exists(ctx, blacklistKey).Result()
	return err == nil && exists > 0
}

func (s *Server) isAccountActive(ctx context.Context, userID uint) bool {
	activeKey := fmt.Sprintf("user:active:%d", userID)
	if s.isUserActiveInCache(ctx, activeKey) {
		return true
	}

	userRepo := s.getUserRepository()
	if userRepo == nil {
		return true
	}

	user, err := userRepo.FindByID(ctx, userID)
	if err != nil || user == nil || !user.IsActive {
		return false
	}

	if rdb := s.getRedisClient(); rdb != nil {
		_ = rdb.Set(ctx, activeKey, "1", 2*time.Minute).Err()
	}
	return true
}

func (s *Server) isUserActiveInCache(ctx context.Context, activeKey string) bool {
	rdb := s.getRedisClient()
	if rdb == nil {
		return false
	}
	val, err := rdb.Get(ctx, activeKey).Result()
	return err == nil && val == "1"
}

// AdminOnly rejects non-admin callers. Must run after AuthMiddleware.
func (s *Server) AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ctxRole)
		if role != models.RoleAdmin {
			RespondAbortError(c, http.StatusForbidden, "admin privileges required")
			return
		}
		c.Next()
	}
}

// currentUserID returns the authenticated user id from context.
func currentUserID(c *gin.Context) uint {
	v, _ := c.Get(ctxUserID)
	id, _ := v.(uint)
	return id
}

// currentUserRole returns the authenticated user role from context.
// It safely handles both models.Role and string types stored in ctxRole.
func currentUserRole(c *gin.Context) models.Role {
	if c == nil {
		return ""
	}
	v, ok := c.Get(ctxRole)
	if !ok || v == nil {
		return ""
	}
	switch r := v.(type) {
	case models.Role:
		return r
	case string:
		return models.Role(r)
	default:
		return ""
	}
}

// matchHostOrURL checks whether host matches target or target's host.
func matchHostOrURL(target, host, hostOnly string) bool {
	if target == "" {
		return false
	}
	if u, err := url.Parse(target); err == nil && u.Host != "" {
		targetHost := u.Host
		if th, _, err := net.SplitHostPort(u.Host); err == nil {
			targetHost = th
		}
		return strings.EqualFold(host, u.Host) || strings.EqualFold(hostOnly, targetHost)
	}
	return strings.EqualFold(host, target) || strings.EqualFold(hostOnly, target)
}

func isLoopbackHost(hostOnly string) bool {
	if strings.EqualFold(os.Getenv("GIN_MODE"), "release") {
		return false
	}
	return hostOnly == "localhost" || hostOnly == "127.0.0.1" || hostOnly == "::1"
}

// isTrustedHost verifies that a requested host originates from an approved domain
// (FrontendURL, RPOrigins, RPID, or loopback in development) to prevent Host Header Poisoning.
func (s *Server) isTrustedHost(host string) bool {
	if host == "" {
		return false
	}
	hostOnly := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostOnly = h
	}

	if isLoopbackHost(hostOnly) {
		return true
	}

	if s == nil || s.Cfg == nil {
		return false
	}

	if matchHostOrURL(s.Cfg.RPID, host, hostOnly) || matchHostOrURL(s.Cfg.FrontendURL, host, hostOnly) {
		return true
	}

	for _, origin := range s.Cfg.RPOrigins {
		if matchHostOrURL(origin, host, hostOnly) {
			return true
		}
	}

	for _, origin := range s.Cfg.CORSAllowedOrigins {
		if origin != "*" && matchHostOrURL(origin, host, hostOnly) {
			return true
		}
	}

	return false
}

// publicBaseURL returns the scheme://host base to build user-facing links
// (setup / password-reset emails) so they open on the PUBLIC URL rather than a
// hardcoded localhost default.
//
// When the portal runs behind a reverse proxy the proxy advertises the real
// public origin via X-Forwarded-Proto / X-Forwarded-Host — those win only if
// the host is in our trusted allowlist, preventing password-reset link poisoning.
func (s *Server) publicBaseURL(c *gin.Context) string {
	host := firstHeaderValue(c.GetHeader("X-Forwarded-Host"))
	if host != "" && s.isTrustedHost(host) {
		scheme := firstHeaderValue(c.GetHeader("X-Forwarded-Proto"))
		if scheme == "" {
			if c.Request.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}
		return strings.TrimRight(scheme+"://"+host, "/")
	}
	if s != nil && s.Cfg != nil && s.Cfg.FrontendURL != "" {
		return strings.TrimRight(s.Cfg.FrontendURL, "/")
	}
	return "http://localhost:3000"
}

// firstHeaderValue returns the first, trimmed entry of a possibly
// comma-separated proxy header (e.g. "public.example.com, internal:8080").
func firstHeaderValue(v string) string {
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

func matchCleanOrigin(target, candidate string) bool {
	if target == "" {
		return false
	}
	return target == "*" || strings.TrimRight(strings.ToLower(target), "/") == candidate
}

func containsCleanOrigin(origins []string, candidate string) bool {
	for _, o := range origins {
		if matchCleanOrigin(o, candidate) {
			return true
		}
	}
	return false
}

func (s *Server) isConfigOriginAllowed(clean string) bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	if containsCleanOrigin(s.Cfg.CORSAllowedOrigins, clean) {
		return true
	}
	if matchCleanOrigin(s.Cfg.FrontendURL, clean) {
		return true
	}
	return containsCleanOrigin(s.Cfg.RPOrigins, clean)
}

func isDevOriginAllowed(clean string) bool {
	if strings.EqualFold(os.Getenv("GIN_MODE"), "release") {
		return false
	}
	switch clean {
	case "http://localhost:3000", "http://localhost:8080", "http://127.0.0.1:3000", "http://127.0.0.1:8080":
		return true
	default:
		return false
	}
}

// isOriginAllowed checks whether an origin header is in the allowed CORS list.
func (s *Server) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	clean := strings.TrimRight(strings.ToLower(origin), "/")
	return s.isConfigOriginAllowed(clean) || isDevOriginAllowed(clean)
}

// CORSMiddleware sets up cross-origin resource sharing headers.
// Origins in the allowlist receive reflected Origin and credentials allowance.
func (s *Server) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if s.isOriginAllowed(origin) {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// CORSMiddleware is a package-level fallback that delegates to a Server instance.
func CORSMiddleware() gin.HandlerFunc {
	s := &Server{}
	return s.CORSMiddleware()
}
