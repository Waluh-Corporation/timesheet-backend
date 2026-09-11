package scheduler

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"timesheet-backend/models"
	"timesheet-backend/push"
)

// Scheduler owns the cron runner that dispatches the daily timesheet reminder.
type Scheduler struct {
	db   *gorm.DB
	push *push.Service
	cron *cron.Cron
	loc  *time.Location
}

// New builds a Scheduler pinned to the given IANA timezone (Asia/Jakarta for
// WIB). If the zone can't be loaded it falls back to UTC and logs loudly.
func New(db *gorm.DB, pushSvc *push.Service, tz string) *Scheduler {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("[scheduler] could not load timezone %q, falling back to UTC: %v", tz, err)
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))
	return &Scheduler{db: db, push: pushSvc, cron: c, loc: loc}
}

// Start registers the 17:00 WIB daily reminder job and launches the runner.
func (s *Scheduler) Start() {
	// "0 17 * * *" => every day at 17:00 in the scheduler's location (WIB).
	if _, err := s.cron.AddFunc("0 17 * * *", s.sendDailyReminders); err != nil {
		log.Printf("[scheduler] failed to register daily reminder: %v", err)
		return
	}
	// "0 2 * * *" => every day at 02:00 in the scheduler's location (WIB) for token housekeeping.
	if _, err := s.cron.AddFunc("0 2 * * *", s.cleanupExpiredTokens); err != nil {
		log.Printf("[scheduler] failed to register token housekeeping: %v", err)
	}
	s.cron.Start()
	log.Printf("[scheduler] daily timesheet reminder armed for 17:00 %s", s.loc.String())
}

// Stop gracefully halts the cron runner.
func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}

// sendDailyReminders notifies every active user who has not yet recorded an
// activity for "today" (in WIB).
func (s *Scheduler) sendDailyReminders() {
	if s.db == nil || s.push == nil {
		return
	}

	now := time.Now().In(s.loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	endOfDay := startOfDay.Add(24 * time.Hour)

	log.Printf("[scheduler] running daily reminder for %s", startOfDay.Format("2006-01-02"))

	var users []models.User
	if err := s.db.Where("is_active = ? AND role = ?", true, models.RoleUser).Find(&users).Error; err != nil {
		log.Printf("[scheduler] failed to load users: %v", err)
		return
	}

	for _, u := range users {
		var count int64
		s.db.Model(&models.DailyActivity{}).
			Where("user_id = ? AND date >= ? AND date < ?", u.ID, startOfDay, endOfDay).
			Count(&count)
		if count > 0 {
			continue // already filled today
		}
		s.push.SendToUser(u.ID, push.Payload{
			Title: "Timesheet Reminder",
			Body:  "Waktunya isi timesheet hari ini!",
			URL:   "/activity",
		})
	}
}

// cleanupExpiredTokens performs DBA housekeeping on password_reset_tokens to prevent table bloat.
func (s *Scheduler) cleanupExpiredTokens() {
	if s.db == nil {
		return
	}

	now := time.Now()
	// Delete tokens that expired more than 7 days ago, or were used more than 30 days ago.
	res := s.db.Where("expires_at < ?", now.AddDate(0, 0, -7)).
		Or("used_at IS NOT NULL AND created_at < ?", now.AddDate(0, 0, -30)).
		Delete(&models.PasswordResetToken{})
	if res.Error != nil {
		log.Printf("[scheduler] token housekeeping error: %v", res.Error)
	} else if res.RowsAffected > 0 {
		log.Printf("[scheduler] token housekeeping purged %d stale tokens", res.RowsAffected)
	}
}
