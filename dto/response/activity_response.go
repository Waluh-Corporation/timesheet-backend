package response

import (
	"time"

	"timesheet-backend/models"
)

// DailyActivityDetailResponse represents the response payload for GET /api/v1/activities/:id.
type DailyActivityDetailResponse struct {
	ID          uint                   `json:"id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	UserID      uint                   `json:"user_id"`
	Date        time.Time              `json:"date"`
	StartTime   string                 `json:"start_time"`
	EndTime     string                 `json:"end_time"`
	Activity    string                 `json:"activity"`
	ProjectName string                 `json:"project_name"`
	ProjectID   string                 `json:"project_id"`
	ProjectRef  *models.Project        `json:"project_ref,omitempty"`
	StatusRef   *models.ActivityStatus `json:"status_ref,omitempty"`
}

// DailyActivityResponse represents a concise daily activity response without heavy nested relation models.
type DailyActivityResponse struct {
	ID          uint      `json:"id"`
	Date        time.Time `json:"date"`
	StartTime   string    `json:"start_time"`
	EndTime     string    `json:"end_time"`
	Activity    string    `json:"activity"`
	ProjectName string    `json:"project_name"`
	ProjectID   string    `json:"project_id"`
	Status      string    `json:"status"`
}
