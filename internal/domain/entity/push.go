package entity

import "time"

// PushSubscription represents a browser Web Push subscription for timesheet reminders.
type PushSubscription struct {
	ID        uint
	CreatedAt time.Time
	UserID    uint
	Endpoint  string
	P256dh    string
	Auth      string
}
