package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"timesheet-backend/internal/domain"
)

// VerifyDocument godoc
// @Summary Public document verification
// @Description Validates the authenticity and signature audit trail of a Timesheet or Berita Acara via verification token.
// @Tags Verification
// @Produce json
// @Param token path string true "Document verification token"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/verify/doc/{token} [get]
func (s *Server) VerifyDocument(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, "Signature service not initialized")
		return
	}

	token := c.Param("token")
	resp, err := s.SignatureSvc.VerifyDocument(c.Request.Context(), token)
	if err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}
