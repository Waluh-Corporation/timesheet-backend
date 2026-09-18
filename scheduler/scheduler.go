package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/service"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
)

const (
	DefaultReminderCron = "0 17 * * *"
	DefaultCleanupCron  = "0 2 * * *"
	DefaultEOMCron      = "0 18 * * *"
)

// ScheduleInfo carries details about a cron schedule for frontend presentation.
type ScheduleInfo struct {
	CronExpression string     `json:"cron_expression"`
	Timezone       string     `json:"timezone"`
	IsEnabled      bool       `json:"is_enabled"`
	NextRun        *time.Time `json:"next_run"`
	HumanReadable  string     `json:"human_readable"`
}

// formatHumanSchedule generates a friendly description of standard 5-part cron schedules.
func formatHumanSchedule(expr, tz string) string {
	parts := strings.Fields(expr)
	if len(parts) == 5 && parts[2] == "*" && parts[3] == "*" {
		hour, errH := strconv.Atoi(parts[1])
		min, errM := strconv.Atoi(parts[0])
		if errH == nil && errM == nil {
			timeStr := fmt.Sprintf("%02d:%02d", hour, min)
			switch parts[4] {
			case "*":
				return fmt.Sprintf("Setiap hari pukul %s (%s)", timeStr, tz)
			case "1-5":
				return fmt.Sprintf("Setiap hari kerja (Senin-Jumat) pukul %s (%s)", timeStr, tz)
			}
		}
	}
	return fmt.Sprintf("Jadwal cron: %s (%s)", expr, tz)
}

// GetScheduleInfo computes the status, human-readable description, and next run time for a cron expression.
func GetScheduleInfo(tz, cronExpr string) ScheduleInfo {
	if strings.TrimSpace(cronExpr) == "" {
		cronExpr = DefaultReminderCron
	}
	if strings.TrimSpace(tz) == "" {
		tz = "Asia/Jakarta"
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
		tz = "UTC"
	}

	disabled := IsJobDisabled(cronExpr)
	info := ScheduleInfo{
		CronExpression: cronExpr,
		Timezone:       tz,
		IsEnabled:      !disabled,
	}

	if disabled {
		info.HumanReadable = "Dinonaktifkan / Disabled"
		return info
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(cronExpr)
	if err != nil {
		sched, _ = parser.Parse(DefaultReminderCron)
		info.HumanReadable = formatHumanSchedule(DefaultReminderCron, tz) + " (fallback dari ekspresi tidak valid)"
	} else {
		info.HumanReadable = formatHumanSchedule(cronExpr, tz)
	}

	if sched != nil {
		next := sched.Next(time.Now().In(loc))
		info.NextRun = &next
	}

	return info
}

// Scheduler owns the cron runner that dispatches the daily timesheet reminder,
// end-of-the-month automated timesheet delivery, and housekeeping tasks.
type Scheduler struct {
	db           *gorm.DB
	push         *push.Service
	timesheetSvc service.TimesheetService
	cron         *cron.Cron
	loc          *time.Location
	reminderCron string
	cleanupCron  string
	eomCron      string
}

// IsJobDisabled checks if a cron expression explicitly disables the scheduled task.
func IsJobDisabled(expr string) bool {
	val := strings.ToLower(strings.TrimSpace(expr))
	return val == "disabled" || val == "off" || val == "false" || val == "none"
}

// New builds a Scheduler pinned to the given IANA timezone (Asia/Jakarta for
// WIB) and optional cron expressions for reminders, token housekeeping, and
// end-of-the-month timesheet delivery.
// If crons are omitted or empty, DefaultReminderCron ("0 17 * * *"),
// DefaultCleanupCron ("0 2 * * *"), and DefaultEOMCron ("0 18 * * *") are used.
func New(db *gorm.DB, pushSvc *push.Service, tz string, crons ...string) *Scheduler {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("[scheduler] could not load timezone %q, falling back to UTC: %v", tz, err)
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))

	reminderCron := DefaultReminderCron
	cleanupCron := DefaultCleanupCron
	eomCron := DefaultEOMCron
	if len(crons) > 0 && strings.TrimSpace(crons[0]) != "" {
		reminderCron = strings.TrimSpace(crons[0])
	}
	if len(crons) > 1 && strings.TrimSpace(crons[1]) != "" {
		cleanupCron = strings.TrimSpace(crons[1])
	}
	if len(crons) > 2 && strings.TrimSpace(crons[2]) != "" {
		eomCron = strings.TrimSpace(crons[2])
	}

	return &Scheduler{
		db:           db,
		push:         pushSvc,
		cron:         c,
		loc:          loc,
		reminderCron: reminderCron,
		cleanupCron:  cleanupCron,
		eomCron:      eomCron,
	}
}

// SetTimesheetService assigns the timesheet business service used for automated generation and delivery.
func (s *Scheduler) SetTimesheetService(ts service.TimesheetService) {
	s.timesheetSvc = ts
}

// registerJob attempts to register a task with the given cron expression,
// falling back to a default expression if registration fails.
func (s *Scheduler) registerJob(name, cronExpr, defaultCron string, task func()) {
	if cronExpr == "" || IsJobDisabled(cronExpr) {
		log.Printf("[scheduler] %s is disabled", name)
		return
	}

	if _, err := s.cron.AddFunc(cronExpr, task); err != nil {
		log.Printf("[scheduler] failed to register %s (%s): %v, falling back to default %s", name, cronExpr, err, defaultCron)
		if _, fallbackErr := s.cron.AddFunc(defaultCron, task); fallbackErr != nil {
			log.Printf("[scheduler] failed to register fallback %s: %v", name, fallbackErr)
		}
	}
}

// Start registers the configured daily reminder, housekeeping, and EOM delivery cron jobs
// and launches the runner.
func (s *Scheduler) Start() {
	s.registerJob("daily reminder", s.reminderCron, DefaultReminderCron, s.sendDailyReminders)
	s.registerJob("token housekeeping", s.cleanupCron, DefaultCleanupCron, s.cleanupExpiredTokens)
	s.registerJob("end-of-the-month timesheet delivery", s.eomCron, DefaultEOMCron, s.sendEndOfMonthTimesheets)

	s.cron.Start()
	log.Printf("[scheduler] daily timesheet reminder armed with cron %q in %s", s.reminderCron, s.loc.String())
	log.Printf("[scheduler] end-of-the-month timesheet delivery armed with cron %q in %s", s.eomCron, s.loc.String())
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

// IsEndOfMonth determines whether the given date is the last calendar day of its month.
func IsEndOfMonth(t time.Time) bool {
	return t.AddDate(0, 0, 1).Day() == 1
}

// sendEndOfMonthTimesheets executes automated timesheet delivery on the last day of the current month.
func (s *Scheduler) sendEndOfMonthTimesheets() {
	s.SendEndOfMonthTimesheetsAt(time.Now().In(s.loc))
}

// SendEndOfMonthTimesheetsAt triggers automated timesheet generation and delivery for all active users
// evaluated against targetDate. If targetDate is not the last day of the month, the process is skipped.
// Daily activity entries on targetDate itself are NOT required; if activities exist for the month,
// the timesheet is generated and delivered via email and push notification.
func (s *Scheduler) SendEndOfMonthTimesheetsAt(targetDate time.Time) {
	if !IsEndOfMonth(targetDate) {
		log.Printf("[scheduler] %s is not end-of-the-month, skipping automated timesheet delivery", targetDate.Format("2006-01-02"))
		return
	}

	if s.db == nil || s.timesheetSvc == nil {
		log.Printf("[scheduler] db or timesheet service is nil, skipping automated timesheet delivery")
		return
	}

	month := int(targetDate.Month())
	year := targetDate.Year()
	period := mailer.FormatMonthYearIndonesian(month, year)

	log.Printf("[scheduler] running automated end-of-the-month timesheet delivery for %s (period: %s)", targetDate.Format("2006-01-02"), period)

	var users []models.User
	if err := s.db.Where("is_active = ? AND role = ?", true, models.RoleUser).Find(&users).Error; err != nil {
		log.Printf("[scheduler] failed to load active users for timesheet delivery: %v", err)
		return
	}

	ctx := context.Background()
	sentCount := 0
	skippedCount := 0
	failedCount := 0

	for _, u := range users {
		// Notice: We intentionally do NOT check or require that daily activity on targetDate is filled.
		// GenerateWorkbook generates the workbook and sends it asynchronously via mailer if configured.
		_, filename, err := s.timesheetSvc.GenerateWorkbook(ctx, u.ID, month, year)
		if err != nil {
			if errors.Is(err, domain.ErrInvalidInput) {
				// No activities recorded at all for this month
				log.Printf("[scheduler] user %s (ID %d) has no activities for period %s, skipping", u.Username, u.ID, period)
				skippedCount++
				if s.push != nil {
					s.push.SendToUser(u.ID, push.Payload{
						Title: "Timesheet Belum Lengkap",
						Body:  fmt.Sprintf("Timesheet periode %s belum dapat dikirim otomatis karena belum ada aktivitas yang tercatat.", period),
						URL:   "/activity",
					})
				}
				continue
			}
			log.Printf("[scheduler] failed to generate/send timesheet for user %s (ID %d): %v", u.Username, u.ID, err)
			failedCount++
			continue
		}

		sentCount++
		log.Printf("[scheduler] timesheet %s generated and dispatched for user %s (ID %d)", filename, u.Username, u.ID)

		if s.push != nil {
			s.push.SendToUser(u.ID, push.Payload{
				Title: "Timesheet Bulanan Terkirim",
				Body:  fmt.Sprintf("Timesheet periode %s telah dibuat dan dikirimkan otomatis ke email Anda.", period),
				URL:   "/dashboard",
			})
		}
	}

	log.Printf("[scheduler] end-of-the-month timesheet delivery complete: %d sent, %d skipped (no activities), %d failed", sentCount, skippedCount, failedCount)
}
