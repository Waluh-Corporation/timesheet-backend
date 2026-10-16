package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
)

// ListWitnesses godoc
// @Summary List eligible witnesses for Berita Acara
// @Description Returns active users with role 'user' who can be selected as a witness for Berita Acara, excluding the requesting user.
// @Tags Berita Acara
// @Security BearerAuth
// @Produce json
// @Param q query string false "Search query by name, username, employee ID, or BNI ID"
// @Success 200 {array} response.WitnessResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Database error"
// @Router /api/v1/berita-acara/witnesses [get]
func (s *Server) ListWitnesses(c *gin.Context) {
	uid := currentUserID(c)
	repo := s.getUserRepository()
	if repo == nil {
		RespondError(c, http.StatusInternalServerError, "database repository unavailable")
		return
	}

	search := strings.TrimSpace(c.Query("q"))
	if search == "" {
		search = strings.TrimSpace(c.Query("search"))
	}

	witnesses, err := repo.ListWitnesses(reqContext(c), uid, search)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to list witnesses: "+err.Error())
		return
	}

	resp := make([]response.WitnessResponse, len(witnesses))
	for i := range witnesses {
		resp[i] = response.ToWitnessResponse(&witnesses[i])
	}

	RespondSuccess(c, http.StatusOK, resp)
}
