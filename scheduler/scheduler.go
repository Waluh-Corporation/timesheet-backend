package scheduler

import (
	"log"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"timesheet-backend/models"
	"timesheet-backend/push"
)

const (
	DefaultReminderCron = "0 17 * * *"
	DefaultCleanupCron  = "0 2 * * *"
)

// Scheduler owns the cron runner that dispatches the daily timesheet reminder and housekeeping tasks.
type Scheduler struct {
	db           *gorm.DB
	push         *push.Service
	cron         *cron.Cron
	loc          *time.Location
	reminderCron string
	cleanupCron  string
}

// isJobDisabled checks if a cron expression explicitly disables the scheduled task.
func isJobDisabled(expr string) bool {
	val := strings.ToLower(strings.TrimSpace(expr))
	return val == "disabled" || val == "off" || val == "false" || val == "none"
}

// New builds a Scheduler pinned to the given IANA timezone (Asia/Jakarta for
// WIB) and optional cron expressions for reminders and token housekeeping.
// If crons are omitted or empty, DefaultReminderCron ("0 17 * * *") and
// DefaultCleanupCron ("0 2 * * *") are used.
func New(db *gorm.DB, pushSvc *push.Service, tz string, crons ...string) *Scheduler {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("[scheduler] could not load timezone %q, falling back to UTC: %v", tz, err)
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))

	reminderCron := DefaultReminderCron
	cleanupCron := DefaultCleanupCron
	if len(crons) > 0 && strings.TrimSpace(crons[0]) != "" {
		reminderCron = strings.TrimSpace(crons[0])
	}
	if len(crons) > 1 && strings.TrimSpace(crons[1]) != "" {
		cleanupCron = strings.TrimSpace(crons[1])
	}

	return &Scheduler{
		db:           db,
		push:         pushSvc,
		cron:         c,
		loc:          loc,
		reminderCron: reminderCron,
		cleanupCron:  cleanupCron,
	}
}

// Start registers the configured daily reminder and token housekeeping cron jobs
// and launches the runner.
func (s *Scheduler) Start() {
	if s.reminderCron != "" && !isJobDisabled(s.reminderCron) {
		if _, err := s.cron.AddFunc(s.reminderCron, s.sendDailyReminders); err != nil {
			log.Printf("[scheduler] failed to register daily reminder (%s): %v, falling back to default %s", s.reminderCron, err, DefaultReminderCron)
			if _, fallbackErr := s.cron.AddFunc(DefaultReminderCron, s.sendDailyReminders); fallbackErr != nil {
				log.Printf("[scheduler] failed to register fallback daily reminder: %v", fallbackErr)
			}
		}
	} else {
		log.Printf("[scheduler] daily reminder is disabled")
	}

	if s.cleanupCron != "" && !isJobDisabled(s.cleanupCron) {
		if _, err := s.cron.AddFunc(s.cleanupCron, s.cleanupExpiredTokens); err != nil {
			log.Printf("[scheduler] failed to register token housekeeping (%s): %v, falling back to default %s", s.cleanupCron, err, DefaultCleanupCron)
			if _, fallbackErr := s.cron.AddFunc(DefaultCleanupCron, s.cleanupExpiredTokens); fallbackErr != nil {
				log.Printf("[scheduler] failed to register fallback token housekeeping: %v", fallbackErr)
			}
		}
	} else {
		log.Printf("[scheduler] token housekeeping is disabled")
	}
	s.cron.Start()
	log.Printf("[scheduler] daily timesheet reminder armed with cron %q in %s", s.reminderCron, s.loc.String())
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
