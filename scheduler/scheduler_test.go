package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
	"timesheet-backend/push"
)

func TestScheduler_NewAndStop(t *testing.T) {
	// Valid timezone
	s := New(nil, nil, "Asia/Jakarta")
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if s.loc.String() != "Asia/Jakarta" {
		t.Errorf("expected timezone Asia/Jakarta, got %s", s.loc.String())
	}

	// Invalid timezone fallback to UTC
	sUTC := New(nil, nil, "Invalid/Timezone_Name")
	if sUTC.loc != time.UTC {
		t.Errorf("expected fallback to UTC, got %v", sUTC.loc)
	}

	// Start, execute handlers, and Stop
	s.Start()
	s.sendDailyReminders()
	s.cleanupExpiredTokens()
	s.Stop()
}

func TestScheduler_CustomAndFallbackCron(t *testing.T) {
	// Custom valid crons
	s := New(nil, nil, "Asia/Jakarta", "15 18 * * *", "30 3 * * *", "0 19 * * *")
	if s.reminderCron != "15 18 * * *" {
		t.Errorf("expected reminderCron '15 18 * * *', got %q", s.reminderCron)
	}
	if s.cleanupCron != "30 3 * * *" {
		t.Errorf("expected cleanupCron '30 3 * * *', got %q", s.cleanupCron)
	}
	if s.eomCron != "0 19 * * *" {
		t.Errorf("expected eomCron '0 19 * * *', got %q", s.eomCron)
	}
	s.Start()
	s.Stop()

	// Empty strings fallback to defaults
	sDef := New(nil, nil, "Asia/Jakarta", "", "  ", "")
	if sDef.reminderCron != DefaultReminderCron {
		t.Errorf("expected reminderCron fallback %q, got %q", DefaultReminderCron, sDef.reminderCron)
	}
	if sDef.cleanupCron != DefaultCleanupCron {
		t.Errorf("expected cleanupCron fallback %q, got %q", DefaultCleanupCron, sDef.cleanupCron)
	}
	if sDef.eomCron != DefaultEOMCron {
		t.Errorf("expected eomCron fallback %q, got %q", DefaultEOMCron, sDef.eomCron)
	}

	// Disabled crons
	sDisabled := New(nil, nil, "Asia/Jakarta", "disabled", "off", "false")
	sDisabled.Start()
	sDisabled.Stop()

	// Invalid cron expression falls back gracefully without panic
	sInvalid := New(nil, nil, "Asia/Jakarta", "not-a-valid-cron", "invalid-cleanup", "invalid-eom")
	sInvalid.Start()
	sInvalid.Stop()
}

func TestIsJobDisabled(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"disabled", true},
		{"Disabled", true},
		{"OFF", true},
		{"off", true},
		{"false", true},
		{"none", true},
		{"0 17 * * *", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsJobDisabled(tc.in); got != tc.want {
			t.Errorf("IsJobDisabled(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestGetScheduleInfo(t *testing.T) {
	// Standard enabled
	info := GetScheduleInfo("Asia/Jakarta", "0 17 * * *")
	if !info.IsEnabled {
		t.Errorf("expected is_enabled true")
	}
	if info.CronExpression != "0 17 * * *" {
		t.Errorf("expected 0 17 * * *, got %s", info.CronExpression)
	}
	if info.Timezone != "Asia/Jakarta" {
		t.Errorf("expected Asia/Jakarta, got %s", info.Timezone)
	}
	if info.NextRun == nil {
		t.Errorf("expected next_run not nil")
	}
	if info.HumanReadable != "Setiap hari pukul 17:00 (Asia/Jakarta)" {
		t.Errorf("unexpected human readable: %s", info.HumanReadable)
	}

	// Weekdays
	infoWeekdays := GetScheduleInfo("Asia/Jakarta", "30 18 * * 1-5")
	if infoWeekdays.HumanReadable != "Setiap hari kerja (Senin-Jumat) pukul 18:30 (Asia/Jakarta)" {
		t.Errorf("unexpected weekdays human readable: %s", infoWeekdays.HumanReadable)
	}

	// Disabled
	infoDisabled := GetScheduleInfo("Asia/Jakarta", "disabled")
	if infoDisabled.IsEnabled {
		t.Errorf("expected is_enabled false for disabled")
	}
	if infoDisabled.NextRun != nil {
		t.Errorf("expected next_run nil for disabled")
	}

	// Invalid timezone fallback
	infoInvalidTZ := GetScheduleInfo("Invalid/Zone", "0 17 * * *")
	if infoInvalidTZ.Timezone != "UTC" {
		t.Errorf("expected fallback to UTC, got %s", infoInvalidTZ.Timezone)
	}

	// Invalid cron expression fallback
	infoInvalidCron := GetScheduleInfo("Asia/Jakarta", "invalid-cron")
	if !infoInvalidCron.IsEnabled {
		t.Errorf("expected is_enabled true with fallback")
	}
	if infoInvalidCron.NextRun == nil {
		t.Errorf("expected next_run not nil with fallback")
	}
}

func TestScheduler_WithDB(t *testing.T) {
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skip("Postgres DB not available")
	}

	tx := db.Begin()
	defer tx.Rollback()

	pushSvc := push.New(cfg, tx)
	s := New(tx, pushSvc, "Asia/Jakarta")

	u := models.User{
		Username: "schedusertest",
		Email:    "sched@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	_ = tx.Create(&u).Error

	s.sendDailyReminders()

	tok := models.PasswordResetToken{
		UserID:    u.ID,
		TokenType: "password_reset",
		TokenHash: "dummyhash123",
		ExpiresAt: time.Now().AddDate(0, 0, -10),
	}
	_ = tx.Create(&tok).Error

	s.cleanupExpiredTokens()
}

type mockTimesheetService struct {
	generateWorkbookFunc func(ctx context.Context, userID uint, month, year int) ([]byte, string, error)
}

func (m *mockTimesheetService) GenerateWorkbook(ctx context.Context, userID uint, month, year int) ([]byte, string, error) {
	if m.generateWorkbookFunc != nil {
		return m.generateWorkbookFunc(ctx, userID, month, year)
	}
	return []byte("fake-excel-data"), "Timesheet_test_09_2026.xlsx", nil
}

func (m *mockTimesheetService) UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error {
	return nil
}

func (m *mockTimesheetService) ListMonthlyOvertimes(ctx context.Context, userID uint, month, year int) ([]response.OvertimeResponse, error) {
	return nil, nil
}

func (m *mockTimesheetService) DeleteOvertime(ctx context.Context, id, userID uint) error {
	return nil
}

func TestIsEndOfMonth(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		day   int
		want  bool
	}{
		// 31-day months
		{2026, time.January, 31, true},
		{2026, time.January, 30, false},
		{2026, time.March, 31, true},
		{2026, time.March, 15, false},
		{2026, time.December, 31, true},
		{2026, time.December, 1, false},

		// 30-day months
		{2026, time.April, 30, true},
		{2026, time.April, 29, false},
		{2026, time.June, 30, true},
		{2026, time.September, 30, true},
		{2026, time.September, 29, false},
		{2026, time.November, 30, true},

		// Leap year February (2024)
		{2024, time.February, 29, true},
		{2024, time.February, 28, false},

		// Non-leap year February (2025/2026)
		{2025, time.February, 28, true},
		{2025, time.February, 27, false},
		{2026, time.February, 28, true},
		{2026, time.February, 27, false},
	}

	for _, tc := range cases {
		date := time.Date(tc.year, tc.month, tc.day, 12, 0, 0, 0, time.UTC)
		got := IsEndOfMonth(date)
		if got != tc.want {
			t.Errorf("IsEndOfMonth(%s) = %v, want %v", date.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestScheduler_SendEndOfMonthTimesheetsAt(t *testing.T) {
	// Guard against nil DB or timesheet service
	sNil := New(nil, nil, "Asia/Jakarta")
	sNil.SendEndOfMonthTimesheetsAt(time.Date(2026, time.September, 30, 18, 0, 0, 0, time.UTC))

	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skip("Postgres DB not available")
	}

	tx := db.Begin()
	defer tx.Rollback()

	pushSvc := push.New(cfg, tx)
	s := New(tx, pushSvc, "Asia/Jakarta")

	// 1. Not end of month -> should return early without executing
	called := false
	mockSvc := &mockTimesheetService{
		generateWorkbookFunc: func(ctx context.Context, userID uint, month, year int) ([]byte, string, error) {
			called = true
			return []byte("data"), "file.xlsx", nil
		},
	}
	s.SetTimesheetService(mockSvc)

	nonEOMDate := time.Date(2026, time.September, 15, 18, 0, 0, 0, s.loc)
	s.SendEndOfMonthTimesheetsAt(nonEOMDate)
	if called {
		t.Error("expected SendEndOfMonthTimesheetsAt to skip when not EOM")
	}

	// 2. On End of Month (September 30) with active users
	u1 := models.User{
		Username: "eom_user_with_act",
		Email:    "eom1@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	u2 := models.User{
		Username: "eom_user_no_act",
		Email:    "eom2@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	if err := tx.Create(&u1).Error; err != nil {
		t.Fatalf("failed to create u1: %v", err)
	}
	if err := tx.Create(&u2).Error; err != nil {
		t.Fatalf("failed to create u2: %v", err)
	}

	// Seed activity on September 25 for u1, but NOT on September 30 (EOM)
	actDate := time.Date(2026, time.September, 25, 0, 0, 0, 0, time.UTC)
	seedAct := models.DailyActivity{
		UserID:    u1.ID,
		Date:      actDate,
		StartTime: "09:00",
		EndTime:   "18:00",
		Status:    "P",
		Activity:  "Development work prior to EOM",
		IsActive:  true,
	}
	if err := tx.Create(&seedAct).Error; err != nil {
		t.Fatalf("failed to create seed activity: %v", err)
	}

	generatedUsers := make(map[uint]bool)
	mockSvc.generateWorkbookFunc = func(ctx context.Context, userID uint, month, year int) ([]byte, string, error) {
		if userID == u1.ID {
			generatedUsers[userID] = true
			return []byte("fake-excel-u1"), "Timesheet_u1_09_2026.xlsx", nil
		}
		if userID == u2.ID {
			// Simulates domain.ErrInvalidInput when user has no activities in the month
			return nil, "", fmt.Errorf("%w: timesheet belum dapat dibuat karena belum ada aktivitas", domain.ErrInvalidInput)
		}
		return []byte("data"), "file.xlsx", nil
	}

	eomDate := time.Date(2026, time.September, 30, 18, 0, 0, 0, s.loc)
	s.SendEndOfMonthTimesheetsAt(eomDate)

	if !generatedUsers[u1.ID] {
		t.Errorf("expected timesheet to be generated for u1 even though EOM daily activity was not filled")
	}

	// 3. sendEndOfMonthTimesheets wrapper test (executes without panic)
	s.sendEndOfMonthTimesheets()
}
