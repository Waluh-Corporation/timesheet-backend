package entity

import "time"

// SystemSetting represents a persistent key-value configuration setting.
type SystemSetting struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}
