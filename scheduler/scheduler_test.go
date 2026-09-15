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
