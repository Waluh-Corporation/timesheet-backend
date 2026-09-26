package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/models"
)

func TestReferenceHandlers_Helpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("queryIntDefault", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?valid=42&invalid=xyz", nil)

		if v := queryIntDefault(c, "valid", 10); v != 42 {
			t.Errorf("expected 42, got %d", v)
		}
		if v := queryIntDefault(c, "invalid", 10); v != 10 {
			t.Errorf("expected default 10 for invalid string, got %d", v)
		}
		if v := queryIntDefault(c, "missing", 10); v != 10 {
			t.Errorf("expected default 10 for missing key, got %d", v)
		}
	})

	t.Run("sanitize", func(t *testing.T) {
		input := "Hello, World! 123 @#$"
		want := "HelloWorld123"
		if got := sanitize(input); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", input, got, want)
		}
	})

	t.Run("parseActiveFilter", func(t *testing.T) {
		// Admin role returns nil
		wAdmin := httptest.NewRecorder()
		cAdmin, _ := gin.CreateTestContext(wAdmin)
		cAdmin.Set(ctxRole, models.RoleAdmin)
		if f := parseActiveFilter(cAdmin); f != nil {
			t.Errorf("expected nil for admin, got %v", f)
		}

		// User role returns true
		wUser := httptest.NewRecorder()
		cUser, _ := gin.CreateTestContext(wUser)
		cUser.Set(ctxRole, models.RoleUser)
		if f := parseActiveFilter(cUser); f == nil || !*f {
			t.Errorf("expected true for user, got %v", f)
		}
	})
}

func TestReferenceHandlers_UninitializedService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srvNil := &Server{} // MasterSvc == nil

	handlers := []struct {
		name    string
		handler func(*gin.Context)
		url     string
	}{
		{"ListProjects", srvNil.ListProjects, "/api/v1/projects"},
		{"ListCompanies", srvNil.ListCompanies, "/api/v1/companies"},
		{"ListSites", srvNil.ListSites, "/api/v1/sites"},
		{"ListDivisions", srvNil.ListDivisions, "/api/v1/divisions"},
		{"ListDepartments", srvNil.ListDepartments, "/api/v1/departments"},
		{"ListActivityStatuses", srvNil.ListActivityStatuses, "/api/v1/activity-statuses"},
		{"ListApprovers", srvNil.ListApprovers, "/api/v1/approvers"},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, h.url, nil)
			h.handler(c)
			assertFatalCode(t, w, http.StatusInternalServerError)
			if !strings.Contains(w.Body.String(), "master service not initialized") {
				t.Errorf("expected 'master service not initialized', got %s", w.Body.String())
			}
		})
	}
}

func TestReferenceHandlers_FailingService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srvFail := &Server{
		MasterSvc: &mockFailingMasterService{},
	}

	handlers := []struct {
		name    string
		handler func(*gin.Context)
		url     string
	}{
		{"ListProjects", srvFail.ListProjects, "/api/v1/projects"},
		{"ListCompanies", srvFail.ListCompanies, "/api/v1/companies"},
		{"ListSites", srvFail.ListSites, "/api/v1/sites"},
		{"ListDivisions", srvFail.ListDivisions, "/api/v1/divisions"},
		{"ListDepartments", srvFail.ListDepartments, "/api/v1/departments?division_id=1&division=IT"},
		{"ListActivityStatuses", srvFail.ListActivityStatuses, "/api/v1/activity-statuses"},
		{"ListApprovers", srvFail.ListApprovers, "/api/v1/approvers?role_type=team_leader"},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, h.url, nil)
			h.handler(c)
			assertFatalCode(t, w, http.StatusInternalServerError)
			if !strings.Contains(w.Body.String(), "db error") {
				t.Errorf("expected 'db error', got %s", w.Body.String())
			}
		})
	}
}

func TestReferenceHandlers_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)

	t.Run("ListProjects", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		srv.ListProjects(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ListCompanies as User", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxRole, models.RoleUser)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
		srv.ListCompanies(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ListCompanies as Admin", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxRole, models.RoleAdmin)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
		srv.ListCompanies(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ListSites", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxRole, models.RoleUser)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sites", nil)
		srv.ListSites(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ListDivisions", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxRole, models.RoleUser)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/divisions", nil)
		srv.ListDivisions(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ListDepartments with filters", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxRole, models.RoleUser)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/departments?division_id=1&division=Digital", nil)
		srv.ListDepartments(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ListActivityStatuses", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/activity-statuses", nil)
		srv.ListActivityStatuses(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("ListApprovers", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxRole, models.RoleUser)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/approvers?role_type=team_leader", nil)
		srv.ListApprovers(c)
		assertResponseCode(t, w, http.StatusOK)
	})
}
