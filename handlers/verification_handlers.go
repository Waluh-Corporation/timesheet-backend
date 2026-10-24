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

// VerifyDocument godoc
// @Summary Public document verification
// @Description Validates the authenticity and signature audit trail of a Timesheet or Berita Acara via verification token. Supports dual HTML and JSON rendering.
// @Tags Verification
// @Produce json,html
// @Param token path string true "Document verification token"
// @Param format query string false "Response format (json or html)"
// @Param lang query string false "Language preference (id or en)"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/verify/doc/{token} [get]
func (s *Server) VerifyDocument(c *gin.Context) {
	acceptHeader := c.GetHeader("Accept")
	formatQuery := strings.ToLower(c.Query("format"))
	isHTML := formatQuery == "html" || (formatQuery != "json" && strings.Contains(acceptHeader, "text/html"))

	appName := ""
	if s.Cfg != nil && strings.TrimSpace(s.Cfg.AppName) != "" {
		appName = strings.TrimSpace(s.Cfg.AppName)
	} else if envName := strings.TrimSpace(os.Getenv("APP_NAME")); envName != "" {
		appName = envName
	} else {
		appName = "Timesheet Portal"
	}

	lang := templates.ResolveLanguage(c.GetHeader("Accept-Language"), c.Query("lang"))

	if s.SignatureSvc == nil {
		if isHTML {
			html, _ := templates.RenderVerifyDocument(&response.PublicDocumentVerificationResponse{
				IsValid: false,
				Status:  "invalid",
			}, lang, appName)
			c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(html))
			return
		}
		RespondError(c, http.StatusInternalServerError, "Signature service not initialized")
		return
	}

	token := c.Param("token")
	resp, err := s.SignatureSvc.VerifyDocument(c.Request.Context(), token)
	if err != nil {
		if isHTML {
			html, _ := templates.RenderVerifyDocument(&response.PublicDocumentVerificationResponse{
				IsValid: false,
				Status:  "invalid",
			}, lang, appName)
			status := http.StatusInternalServerError
			var userErr *domain.UserError
			if errors.As(err, &userErr) {
				status = domain.MapErrorToHTTPStatus(userErr.Err)
			}
			c.Data(status, "text/html; charset=utf-8", []byte(html))
			return
		}

		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if isHTML {
		html, rErr := templates.RenderVerifyDocument(resp, lang, appName)
		if rErr != nil {
			c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte("Internal server error rendering verification page"))
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}
