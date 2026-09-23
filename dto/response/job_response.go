package response

import (
	"time"

	"timesheet-backend/models"
)

// TimesheetJobResponse represents the API response for an asynchronous timesheet generation task.
type TimesheetJobResponse struct {
	ID           string                    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID       uint                      `json:"user_id" example:"1"`
	Month        int                       `json:"month" example:"7"`
	Year         int                       `json:"year" example:"2026"`
	Status       models.TimesheetJobStatus `json:"status" example:"queued"`
	DownloadURL  string                    `json:"download_url,omitempty" example:"https://s3.example.com/..."`
	ExpiresAt    *time.Time                `json:"expires_at,omitempty"`
	ErrorMessage string                    `json:"error_message,omitempty"`
	CreatedAt    time.Time                 `json:"created_at"`
	UpdatedAt    time.Time                 `json:"updated_at"`
}

// ToTimesheetJobResponse converts a models.TimesheetJob entity to TimesheetJobResponse DTO.
func ToTimesheetJobResponse(job *models.TimesheetJob) TimesheetJobResponse {
	if job == nil {
		return TimesheetJobResponse{}
	}
	return TimesheetJobResponse{
		ID:           job.ID,
		UserID:       job.UserID,
		Month:        job.Month,
		Year:         job.Year,
		Status:       job.Status,
		DownloadURL:  job.DownloadURL,
		ExpiresAt:    job.ExpiresAt,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}
}
