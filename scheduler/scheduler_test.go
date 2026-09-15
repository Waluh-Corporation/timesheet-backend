package scheduler

import (
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
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
	s := New(nil, nil, "Asia/Jakarta", "15 18 * * *", "30 3 * * *")
	if s.reminderCron != "15 18 * * *" {
		t.Errorf("expected reminderCron '15 18 * * *', got %q", s.reminderCron)
	}
	if s.cleanupCron != "30 3 * * *" {
		t.Errorf("expected cleanupCron '30 3 * * *', got %q", s.cleanupCron)
	}
	s.Start()
	s.Stop()

	// Empty strings fallback to defaults
	sDef := New(nil, nil, "Asia/Jakarta", "", "  ")
	if sDef.reminderCron != DefaultReminderCron {
		t.Errorf("expected reminderCron fallback %q, got %q", DefaultReminderCron, sDef.reminderCron)
	}
	if sDef.cleanupCron != DefaultCleanupCron {
		t.Errorf("expected cleanupCron fallback %q, got %q", DefaultCleanupCron, sDef.cleanupCron)
	}

	// Disabled crons
	sDisabled := New(nil, nil, "Asia/Jakarta", "disabled", "off")
	sDisabled.Start()
	sDisabled.Stop()

	// Invalid cron expression falls back gracefully without panic
	sInvalid := New(nil, nil, "Asia/Jakarta", "not-a-valid-cron", "invalid-cleanup")
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
