package handlers

import (
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
)

// Server carries the shared dependencies used by all HTTP handlers.
type Server struct {
	DB       *gorm.DB
	Cfg      *config.Config
	Auth     *auth.Service
	Mailer   *mailer.Mailer
	Push     *push.Service
	WebAuthn *webauthn.WebAuthn

	// webAuthnSessions holds in-flight ceremony data keyed by an opaque id
	// handed to the client for the duration of a single begin/finish exchange.
	webAuthnSessions map[string]*webauthn.SessionData
	sessionsMu       sync.Mutex
}

// NewServer wires up a Server and its WebAuthn relying party.
func NewServer(db *gorm.DB, cfg *config.Config, authSvc *auth.Service, m *mailer.Mailer, p *push.Service) (*Server, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		return nil, err
	}
	return &Server{
		DB:               db,
		Cfg:              cfg,
		Auth:             authSvc,
		Mailer:           m,
		Push:             p,
		WebAuthn:         wa,
		webAuthnSessions: make(map[string]*webauthn.SessionData),
	}, nil
}

func (s *Server) putSession(id string, data *webauthn.SessionData) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.webAuthnSessions[id] = data
}

func (s *Server) takeSession(id string) (*webauthn.SessionData, bool) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	data, ok := s.webAuthnSessions[id]
	if ok {
		delete(s.webAuthnSessions, id)
	}
	return data, ok
}

const (
	ctxUserID = "userID"
	ctxRole   = "userRole"
)

// AuthMiddleware validates the bearer JWT and injects the caller identity.
func (s *Server) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			RespondAbortError(c, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := s.Auth.ParseToken(token)
		if err != nil {
			RespondAbortError(c, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
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

// publicBaseURL returns the scheme://host base to build user-facing links
// (setup / password-reset emails) so they open on the PUBLIC URL rather than a
// hardcoded localhost default.
//
// When the portal runs behind a reverse proxy the proxy advertises the real
// public origin via X-Forwarded-Proto / X-Forwarded-Host — those win, so links
// always match the domain the user actually reached the portal on. Only when no
// forwarded host is present (e.g. the local split dev stack where the API and
// the Next.js frontend live on different ports) do we fall back to the
// configured FrontendURL.
func (s *Server) publicBaseURL(c *gin.Context) string {
	host := firstHeaderValue(c.GetHeader("X-Forwarded-Host"))
	if host != "" {
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
	return strings.TrimRight(s.Cfg.FrontendURL, "/")
}

// firstHeaderValue returns the first, trimmed entry of a possibly
// comma-separated proxy header (e.g. "public.example.com, internal:8080").
func firstHeaderValue(v string) string {
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// CORSMiddleware sets up cross-origin resource sharing headers.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RespondSuccess sends a 2xx JSON response wrapped in the unified envelope.
func RespondSuccess(c *gin.Context, code int, data interface{}) {
	c.JSON(code, gin.H{
		"code":   code,
		"status": "success",
		"data":   data,
	})
}

// RespondMessage sends a 2xx JSON message response wrapped in the unified envelope.
func RespondMessage(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"code":    code,
		"status":  "success",
		"message": message,
	})
}

// RespondDelete sends a 2xx JSON deletion confirmation wrapped in the unified envelope.
func RespondDelete(c *gin.Context, code int) {
	c.JSON(code, gin.H{
		"code":    code,
		"status":  "success",
		"deleted": true,
	})
}

// RespondError sends a 4xx/5xx JSON error response with status code and error message.
func RespondError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"code":    code,
		"status":  "error",
		"error":   message,
		"message": message,
	})
}

// RespondAbortError aborts the context with a 4xx/5xx JSON error response.
func RespondAbortError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{
		"code":    code,
		"status":  "error",
		"error":   message,
		"message": message,
	})
}

// RespondPaginated sends a 2xx JSON response wrapped with pagination metadata.
func RespondPaginated(c *gin.Context, code int, data interface{}, page int, limit int, totalRows int64) {
	totalPages := 0
	if limit > 0 {
		totalPages = int(math.Ceil(float64(totalRows) / float64(limit)))
	}
	c.JSON(code, gin.H{
		"code":   code,
		"status": "success",
		"data":   data,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total_rows":  totalRows,
			"total_pages": totalPages,
		},
	})
}

// Convenience methods on Server
func (s *Server) RespondSuccess(c *gin.Context, code int, data interface{}) {
	RespondSuccess(c, code, data)
}
func (s *Server) RespondPaginated(c *gin.Context, code int, data interface{}, page int, limit int, totalRows int64) {
	RespondPaginated(c, code, data, page, limit, totalRows)
}
func (s *Server) RespondMessage(c *gin.Context, code int, message string) {
	RespondMessage(c, code, message)
}
func (s *Server) RespondDelete(c *gin.Context, code int) { RespondDelete(c, code) }
func (s *Server) RespondError(c *gin.Context, code int, message string) {
	RespondError(c, code, message)
}
func (s *Server) RespondAbortError(c *gin.Context, code int, message string) {
	RespondAbortError(c, code, message)
}
