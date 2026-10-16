package entity

import "time"

// DailyActivity represents a recorded daily timesheet activity of an employee.
type DailyActivity struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	UserID       uint
	Date         time.Time
	StartTime    string
	EndTime      string
	Status       string
	Activity     string
	IsActive     bool
	ProjectRefID *uint
}

// OvertimeEntry represents an employee's recorded overtime work entry.
type OvertimeEntry struct {
	ID               uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
	UserID           uint
	DailyActivityID  *uint
	Date             time.Time
	StartTime        string
	EndTime          string
	TaskDescription  string
	TeamLeaderID     *uint
	DepartmentHeadID *uint
	IsActive         bool
}
