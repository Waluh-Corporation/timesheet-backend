package models

import (
	"time"
)

// TimesheetJobStatus represents the current lifecycle status of a timesheet generation job.
type TimesheetJobStatus string

const (
	JobStatusQueued     TimesheetJobStatus = "queued"
	JobStatusProcessing TimesheetJobStatus = "processing"
	JobStatusCompleted  TimesheetJobStatus = "completed"
	JobStatusFailed     TimesheetJobStatus = "failed"
)

// TimesheetJob tracks asynchronous background timesheet generation requests and resulting S3 assets.
type TimesheetJob struct {
	ID             string             `gorm:"primaryKey;size:64" json:"id"`
	UserID         uint               `gorm:"not null;index" json:"user_id"`
	User           *User              `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Month          int                `gorm:"not null" json:"month"`
	Year           int                `gorm:"not null" json:"year"`
	Status         TimesheetJobStatus `gorm:"size:30;not null;default:'queued';index" json:"status"`
	FileKey        string             `gorm:"size:255" json:"file_key,omitempty"`
	DownloadURL    string             `gorm:"type:text" json:"-"`
	DownloadToken  *string            `gorm:"size:64;uniqueIndex" json:"-"`
	DownloadCount  int                `gorm:"not null;default:0" json:"-"`
	MaxDownloads   int                `gorm:"not null;default:3" json:"-"`
	TokenExpiresAt *time.Time         `gorm:"index" json:"-"`
	ExpiresAt      *time.Time         `gorm:"index" json:"expires_at,omitempty"`
	ErrorMessage   string             `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// TableName explicitly binds TimesheetJob to the database table.
func (TimesheetJob) TableName() string {
	return "timesheet_jobs"
}
