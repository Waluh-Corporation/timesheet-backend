package response

import (
	"time"

	"timesheet-backend/models"
)

// BeritaAcaraResponse represents a concise berita acara entry response.
type BeritaAcaraResponse struct {
	ID                 uint      `json:"id"`
	Date               time.Time `json:"date"`
	Day                string    `json:"day"`
	StartTime          string    `json:"start_time"`
	EndTime            string    `json:"end_time"`
	Keterangan         string    `json:"keterangan"`
	DailyActivityID    *uint     `json:"daily_activity_id,omitempty"`
	TeamLeaderID       *uint     `json:"team_leader_id,omitempty"`
	TeamLeaderName     string    `json:"team_leader_name,omitempty"`
	DepartmentHeadID   *uint     `json:"department_head_id,omitempty"`
	DepartmentHeadName string    `json:"department_head_name,omitempty"`
}

// BeritaAcaraDetailResponse represents detailed berita acara information with relations.
type BeritaAcaraDetailResponse struct {
	ID               uint                   `json:"id"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	UserID           uint                   `json:"user_id"`
	Date             time.Time              `json:"date"`
	Day              string                 `json:"day"`
	StartTime        string                 `json:"start_time"`
	EndTime          string                 `json:"end_time"`
	Keterangan       string                 `json:"keterangan"`
	DailyActivityID  *uint                  `json:"daily_activity_id,omitempty"`
	DailyActivity    *DailyActivityResponse `json:"daily_activity,omitempty"`
	TeamLeaderID     *uint                  `json:"team_leader_id,omitempty"`
	TeamLeader       *models.Approver       `json:"team_leader,omitempty"`
	DepartmentHeadID *uint                  `json:"department_head_id,omitempty"`
	DepartmentHead   *models.Approver       `json:"department_head,omitempty"`
}
