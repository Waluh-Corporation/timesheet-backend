package handlers

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/templates"
)

const contentTypeHTML = "text/html; charset=utf-8"

func (s *Server) resolveAppName() string {
	if s.Cfg != nil && strings.TrimSpace(s.Cfg.AppName) != "" {
		return strings.TrimSpace(s.Cfg.AppName)
	}
	if envName := strings.TrimSpace(os.Getenv("APP_NAME")); envName != "" {
		return envName
	}
	return "Timesheet Portal"
}

func isHTMLRequest(c *gin.Context) bool {
	formatQuery := strings.ToLower(c.Query("format"))
	if formatQuery == "html" {
		return true
	}
	return formatQuery != "json" && strings.Contains(c.GetHeader("Accept"), "text/html")
}

func (s *Server) renderInvalidHTML(c *gin.Context, status int, appName string) {
	html, _ := templates.RenderVerifyDocument(&response.PublicDocumentVerificationResponse{
		IsValid: false,
		Status:  "invalid",
	}, appName)
	c.Data(status, contentTypeHTML, []byte(html))
}

func (s *Server) handleVerifyError(c *gin.Context, isHTML bool, appName string, err error) {
	var userErr *domain.UserError
	status := http.StatusInternalServerError
	if errors.As(err, &userErr) {
		status = domain.MapErrorToHTTPStatus(userErr.Err)
	}

	if isHTML {
		s.renderInvalidHTML(c, status, appName)
		return
	}

	if userErr != nil {
		RespondError(c, status, userErr.Msg)
		return
	}
	RespondError(c, http.StatusInternalServerError, err.Error())
}

// VerifyDocument godoc
// @Summary Public document verification
// @Description Validates the authenticity and signature audit trail of a Timesheet or Berita Acara via verification token. Supports dual HTML and JSON rendering.
// @Tags Verification
// @Produce json,html
// @Param token path string true "Document verification token"
// @Param format query string false "Response format (json or html)"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/verify/doc/{token} [get]
func (s *Server) VerifyDocument(c *gin.Context) {
	isHTML := isHTMLRequest(c)
	appName := s.resolveAppName()

	if s.SignatureSvc == nil {
		if isHTML {
			s.renderInvalidHTML(c, http.StatusInternalServerError, appName)
			return
		}
		RespondError(c, http.StatusInternalServerError, "Signature service not initialized")
		return
	}

	token := c.Param("token")
	resp, err := s.SignatureSvc.VerifyDocument(c.Request.Context(), token)
	if err != nil {
		s.handleVerifyError(c, isHTML, appName, err)
		return
	}

	if isHTML {
		html, rErr := templates.RenderVerifyDocument(resp, appName)
		if rErr != nil {
			c.Data(http.StatusInternalServerError, contentTypeHTML, []byte("Internal server error rendering verification page"))
			return
		}
		c.Data(http.StatusOK, contentTypeHTML, []byte(html))
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}
