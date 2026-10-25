package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

type mockAuditRepository struct {
	entries    []models.AuditLog
	total      int64
	err        error
	lastFilter domain.AuditLogFilter
	lastLimit  int
	lastOffset int
}

func (m *mockAuditRepository) Create(ctx context.Context, entry *models.AuditLog) error {
	return nil
}

func (m *mockAuditRepository) List(ctx context.Context, filter domain.AuditLogFilter, limit, offset int) ([]models.AuditLog, int64, error) {
	m.lastFilter = filter
	m.lastLimit = limit
	m.lastOffset = offset
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.entries, m.total, nil
}

func TestListAuditLogs_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	actorID := uint(10)
	sampleTime := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	mockRepo := &mockAuditRepository{
		total: 2,
		entries: []models.AuditLog{
			{
				ID:            1,
				CreatedAt:     sampleTime,
				ActorID:       &actorID,
				ActorUsername: "admin_user",
				Action:        "USER_LOGIN",
				ResourceType:  "auth",
				ResourceID:    "10",
				IPAddress:     "192.168.1.1",
				UserAgent:     "Mozilla/5.0",
				Details:       datatypes.JSON([]byte(`{"browser":"Chrome"}`)),
			},
			{
				ID:            2,
				CreatedAt:     sampleTime.Add(-1 * time.Hour),
				ActorID:       nil,
				ActorUsername: "system",
				Action:        "CLEANUP_JOB",
				ResourceType:  "scheduler",
				ResourceID:    "0",
				IPAddress:     "127.0.0.1",
				UserAgent:     "InternalWorker",
				Details:       nil,
			},
		},
	}

	srv := &Server{
		AuditRepo: mockRepo,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)

	srv.ListAuditLogs(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resp["status"])
	}

	pagination, ok := resp["pagination"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected pagination map in response")
	}
	if pagination["page"] != float64(1) {
		t.Errorf("expected page 1, got %v", pagination["page"])
	}
	if pagination["limit"] != float64(50) {
		t.Errorf("expected limit 50, got %v", pagination["limit"])
	}
	if pagination["total_rows"] != float64(2) {
		t.Errorf("expected total_rows 2, got %v", pagination["total_rows"])
	}

	data, ok := resp["data"].([]interface{})
	if !ok || len(data) != 2 {
		t.Fatalf("expected 2 items in data, got %v", resp["data"])
	}

	item0 := data[0].(map[string]interface{})
	if item0["action"] != "USER_LOGIN" {
		t.Errorf("expected action USER_LOGIN, got %v", item0["action"])
	}
	if item0["actor_username"] != "admin_user" {
		t.Errorf("expected actor_username admin_user, got %v", item0["actor_username"])
	}
	details0, ok := item0["details"].(map[string]interface{})
	if !ok || details0["browser"] != "Chrome" {
		t.Errorf("expected details with browser Chrome, got %v", item0["details"])
	}
}

func TestListAuditLogs_Filters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Action and ResourceType filter", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?action=USER_LOGIN&resource_type=auth&actor_username=admin", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if mockRepo.lastFilter.Action != "USER_LOGIN" {
			t.Errorf("expected filter Action 'USER_LOGIN', got '%s'", mockRepo.lastFilter.Action)
		}
		if mockRepo.lastFilter.ResourceType != "auth" {
			t.Errorf("expected filter ResourceType 'auth', got '%s'", mockRepo.lastFilter.ResourceType)
		}
		if mockRepo.lastFilter.ActorUsername != "admin" {
			t.Errorf("expected filter ActorUsername 'admin', got '%s'", mockRepo.lastFilter.ActorUsername)
		}
	})

	t.Run("YYYY-MM-DD date range filter", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01&end_date=2026-10-31", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.Year() != 2026 || mockRepo.lastFilter.StartDate.Month() != 10 || mockRepo.lastFilter.StartDate.Day() != 1 {
			t.Errorf("unexpected StartDate: %v", mockRepo.lastFilter.StartDate)
		}
		if mockRepo.lastFilter.EndDate.Hour() != 23 || mockRepo.lastFilter.EndDate.Minute() != 59 {
			t.Errorf("expected EndDate at end of day, got %v", mockRepo.lastFilter.EndDate)
		}
	})

	t.Run("RFC3339 date range filter", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01T08:00:00Z&end_date=2026-10-02T18:00:00Z", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.Hour() != 8 || mockRepo.lastFilter.EndDate.Hour() != 18 {
			t.Errorf("unexpected RFC3339 parsed timestamps: start=%v, end=%v", mockRepo.lastFilter.StartDate, mockRepo.lastFilter.EndDate)
		}
	})

	t.Run("RFC3339Nano with fractional seconds (JS toISOString)", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01T08:00:00.000Z&end_date=2026-10-02T18:30:00.123Z", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.Hour() != 8 || mockRepo.lastFilter.EndDate.Hour() != 18 {
			t.Errorf("unexpected parsed timestamps: start=%v, end=%v", mockRepo.lastFilter.StartDate, mockRepo.lastFilter.EndDate)
		}
	})

	t.Run("Datetime without offset parsed in application location", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01T09:00:00&end_date=2026-10-02T17:00:00", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.Hour() != 9 || mockRepo.lastFilter.EndDate.Hour() != 17 {
			t.Errorf("unexpected parsed timestamps: start=%v, end=%v", mockRepo.lastFilter.StartDate, mockRepo.lastFilter.EndDate)
		}
	})

	t.Run("Unescaped timezone offset in URL query restored and parsed", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01T08:00:00+07:00&end_date=2026-10-02T18:00:00+07:00", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.UTC().Hour() != 1 || mockRepo.lastFilter.EndDate.UTC().Hour() != 11 {
			t.Errorf("unexpected UTC timestamps: start=%v, end=%v", mockRepo.lastFilter.StartDate.UTC(), mockRepo.lastFilter.EndDate.UTC())
		}
	})

	t.Run("Datetime with fractional seconds without offset in AppLocation", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01T09:00:00.456&end_date=2026-10-02T17:00:00.789", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.Nanosecond() != 456000000 || mockRepo.lastFilter.EndDate.Nanosecond() != 789000000 {
			t.Errorf("unexpected nanoseconds: start=%v, end=%v", mockRepo.lastFilter.StartDate.Nanosecond(), mockRepo.lastFilter.EndDate.Nanosecond())
		}
	})

	t.Run("HTML5 datetime-local without seconds", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01T14:30&end_date=2026-10-01T18:45", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.Minute() != 30 || mockRepo.lastFilter.StartDate.Second() != 0 {
			t.Errorf("expected StartDate at beginning of minute, got %v", mockRepo.lastFilter.StartDate)
		}
		if mockRepo.lastFilter.EndDate.Minute() != 45 || mockRepo.lastFilter.EndDate.Second() != 59 {
			t.Errorf("expected EndDate at end of minute, got %v", mockRepo.lastFilter.EndDate)
		}
	})

	t.Run("Quoted date strings", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, `/api/v1/admin/audit-logs?start_date="2026-10-01"&end_date='2026-10-31'`, nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
	})

	t.Run("Space-separated ISO-8601 timestamps with offset", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-01%2008:00:00Z&end_date=2026-10-02%2018:00:00%2B07:00", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.StartDate == nil || mockRepo.lastFilter.EndDate == nil {
			t.Fatalf("expected non-nil StartDate and EndDate")
		}
		if mockRepo.lastFilter.StartDate.UTC().Hour() != 8 || mockRepo.lastFilter.EndDate.UTC().Hour() != 11 {
			t.Errorf("unexpected UTC timestamps: start=%v, end=%v", mockRepo.lastFilter.StartDate.UTC(), mockRepo.lastFilter.EndDate.UTC())
		}
	})

	t.Run("Filter strings with quotes trimmed", func(t *testing.T) {
		mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, `/api/v1/admin/audit-logs?action="USER_LOGIN"&resource_type='auth'&actor_username="admin"`, nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if mockRepo.lastFilter.Action != "USER_LOGIN" {
			t.Errorf("expected Action 'USER_LOGIN', got '%s'", mockRepo.lastFilter.Action)
		}
		if mockRepo.lastFilter.ResourceType != "auth" {
			t.Errorf("expected ResourceType 'auth', got '%s'", mockRepo.lastFilter.ResourceType)
		}
		if mockRepo.lastFilter.ActorUsername != "admin" {
			t.Errorf("expected ActorUsername 'admin', got '%s'", mockRepo.lastFilter.ActorUsername)
		}
	})
}

func TestListAuditLogs_Pagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := &mockAuditRepository{total: 100, entries: []models.AuditLog{}}
	srv := &Server{AuditRepo: mockRepo}

	t.Run("Page and limit calculated correctly", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?page=3&limit=20", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if mockRepo.lastLimit != 20 {
			t.Errorf("expected limit 20, got %d", mockRepo.lastLimit)
		}
		if mockRepo.lastOffset != 40 {
			t.Errorf("expected offset 40, got %d", mockRepo.lastOffset)
		}
	})

	t.Run("Limit max cap at 100", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?limit=500", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if mockRepo.lastLimit != 100 {
			t.Errorf("expected limit capped at 100, got %d", mockRepo.lastLimit)
		}
	})

	t.Run("Negative page and limit fallback to defaults", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?page=-1&limit=-5", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if mockRepo.lastLimit != 50 {
			t.Errorf("expected default limit 50, got %d", mockRepo.lastLimit)
		}
		if mockRepo.lastOffset != 0 {
			t.Errorf("expected default offset 0, got %d", mockRepo.lastOffset)
		}
	})

	t.Run("Massive page number capped safely without integer overflow", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?page=50000000&limit=50", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		expectedOffset := (10_000_000 - 1) * 50
		if mockRepo.lastOffset != expectedOffset {
			t.Errorf("expected offset capped at %d, got %d", expectedOffset, mockRepo.lastOffset)
		}
	})

	t.Run("Empty results serializes as empty array not null", func(t *testing.T) {
		emptyRepo := &mockAuditRepository{total: 0, entries: nil}
		emptySrv := &Server{AuditRepo: emptyRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)

		emptySrv.ListAuditLogs(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data, ok := resp["data"].([]interface{})
		if !ok || data == nil {
			t.Errorf("expected non-nil empty array for data, got %v", resp["data"])
		}
	})
}

func TestListAuditLogs_ValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := &mockAuditRepository{}
	srv := &Server{AuditRepo: mockRepo}

	t.Run("Invalid start_date returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=not-a-valid-date", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid start_date, got %d", w.Code)
		}
	})

	t.Run("Invalid end_date returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?end_date=not-a-valid-date", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid end_date, got %d", w.Code)
		}
	})

	t.Run("start_date after end_date returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?start_date=2026-10-31&end_date=2026-10-01", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 when start_date > end_date, got %d", w.Code)
		}
	})
}

func TestListAuditLogs_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Nil audit repository returns 500", func(t *testing.T) {
		srv := &Server{AuditRepo: nil}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when repo is nil, got %d", w.Code)
		}
	})

	t.Run("Database error returns 500", func(t *testing.T) {
		mockRepo := &mockAuditRepository{err: errors.New("db connection failure")}
		srv := &Server{AuditRepo: mockRepo}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)

		srv.ListAuditLogs(c)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 on db error, got %d", w.Code)
		}
	})
}

func TestListAuditLogs_EmptyAndNullDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockAuditRepository{
		total: 4,
		entries: []models.AuditLog{
			{
				ID:           1,
				CreatedAt:    time.Now(),
				Action:       "SYS_CHECK",
				ResourceType: "system",
				Details:      datatypes.JSON([]byte("null")),
			},
			{
				ID:           2,
				CreatedAt:    time.Now(),
				Action:       "SYS_CHECK_WS",
				ResourceType: "system",
				Details:      datatypes.JSON([]byte("   null \n ")),
			},
			{
				ID:           3,
				CreatedAt:    time.Now(),
				Action:       "SYS_CHECK_EMPTY",
				ResourceType: "system",
				Details:      datatypes.JSON([]byte("")),
			},
			{
				ID:           4,
				CreatedAt:    time.Now(),
				Action:       "SYS_CHECK_CORRUPT",
				ResourceType: "system",
				Details:      datatypes.JSON([]byte("{broken json syntax")),
			},
		},
	}
	srv := &Server{AuditRepo: mockRepo}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)

	srv.ListAuditLogs(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := resp["data"].([]interface{})
	if len(data) != 4 {
		t.Fatalf("expected 4 items in data, got %d", len(data))
	}
	for i, itemRaw := range data {
		item := itemRaw.(map[string]interface{})
		if item["details"] != nil {
			t.Errorf("item %d: expected null/empty/corrupted details to be omitted or nil, got %v", i, item["details"])
		}
	}
}

func TestListAuditLogs_RouteProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtSecret := "super-secure-secret-key-at-least-32-bytes!"
	authSvc := auth.NewService(jwtSecret, 15*time.Minute)

	mockRepo := &mockAuditRepository{total: 0, entries: []models.AuditLog{}}
	srv := &Server{
		Auth:      authSvc,
		AuditRepo: mockRepo,
		Cfg: &config.Config{
			JWTSecret: jwtSecret,
		},
	}

	r := gin.New()
	api := r.Group("/api/v1")
	admin := api.Group("/admin")
	admin.Use(srv.AuthMiddleware(), srv.AdminOnly())
	{
		admin.GET("/audit-logs", srv.ListAuditLogs)
	}

	t.Run("No token returns 401 Unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("User token returns 403 Forbidden", func(t *testing.T) {
		userToken, err := authSvc.GenerateToken(&models.User{
			ID:       10,
			Username: "regularuser",
			Role:     models.RoleUser,
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("Admin token returns 200 OK", func(t *testing.T) {
		adminToken, err := authSvc.GenerateToken(&models.User{
			ID:       1,
			Username: "adminuser",
			Role:     models.RoleAdmin,
		})
		if err != nil {
			t.Fatalf("failed to generate admin token: %v", err)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})
}
