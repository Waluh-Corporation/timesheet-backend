package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"timesheet-backend/models"
)

func queryIntDefault(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		}
	}
	return string(out)
}

func parseActiveFilter(c *gin.Context) *bool {
	if role, _ := c.Get(ctxRole); role == models.RoleAdmin {
		return nil
	}
	t := true
	return &t
}

// ListProjects godoc
// @Summary List active projects
// @Description Returns all active projects.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Project
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/projects [get]
func (s *Server) ListProjects(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	projects, err := svc.ListProjects(reqContext(c), true)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, projects)
}

// ListCompanies godoc
// @Summary List all companies
// @Description Returns all companies with their associated departments.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Company
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/companies [get]
func (s *Server) ListCompanies(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	companies, err := svc.ListCompanies(reqContext(c), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, companies)
}

// ListSites godoc
// @Summary List all sites
// @Description Returns all registered office/placement sites.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Site
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/sites [get]
func (s *Server) ListSites(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	sites, err := svc.ListSites(reqContext(c), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, sites)
}

// ListDivisions godoc
// @Summary List all divisions
// @Description Returns all organizational divisions.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Division
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/divisions [get]
func (s *Server) ListDivisions(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	divisions, err := svc.ListDivisions(reqContext(c), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, divisions)
}

// ListDepartments godoc
// @Summary List active departments
// @Description Returns departments, optionally filtered by division, division_id, or is_active.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param division query string false "Division name filter"
// @Param division_id query int false "Division ID filter"
// @Success 200 {array} models.Department
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/departments [get]
func (s *Server) ListDepartments(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	var divID *uint
	if divIDStr := c.Query("division_id"); divIDStr != "" {
		if id, err := strconv.ParseUint(divIDStr, 10, 64); err == nil {
			uID := uint(id)
			divID = &uID
		}
	}
	divName := strings.TrimSpace(c.Query("division"))
	depts, err := svc.ListDepartments(reqContext(c), divID, divName, parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, depts)
}

// ListActivityStatuses godoc
// @Summary List activity status options
// @Description Returns all normalized activity status options (e.g. Present, Sick, Vacation).
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.ActivityStatus
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/activity-statuses [get]
func (s *Server) ListActivityStatuses(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	statuses, err := svc.ListActivityStatuses(reqContext(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, statuses)
}

// ListApprovers godoc
// @Summary List active approvers
// @Description Retrieves active approvers (Team Leaders, Department Heads), optionally filtered by role_type.
// @Tags Master Data
// @Security BearerAuth
// @Produce json
// @Param role_type query string false "Role type filter (team_leader, department_head)"
// @Success 200 {array} models.Approver
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/approvers [get]
func (s *Server) ListApprovers(c *gin.Context) {
	svc := s.getMasterService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "master service not initialized")
		return
	}
	approvers, err := svc.ListApprovers(reqContext(c), c.Query("role_type"), parseActiveFilter(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, approvers)
}
