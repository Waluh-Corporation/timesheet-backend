package response

import (
	"time"

	"timesheet-backend/models"
)

// WitnessResponse represents safe public profile information of a witness.
type WitnessResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// DailyActivityDetailResponse represents the response payload for GET /api/v1/activities/:id.
type DailyActivityDetailResponse struct {
	ID               uint                   `json:"id"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	UserID           uint                   `json:"user_id"`
	Date             time.Time              `json:"date"`
	StartTime        string                 `json:"start_time"`
	EndTime          string                 `json:"end_time"`
	Activity         string                 `json:"activity"`
	ProjectName      string                 `json:"project_name"`
	ProjectID        string                 `json:"project_id"`
	AppImpacted      string                 `json:"app_impacted"`
	ProjectRefID     *uint                  `json:"project_ref_id,omitempty"`
	ProjectRef       *models.Project        `json:"project_ref,omitempty"`
	StatusRef        *models.ActivityStatus `json:"status_ref,omitempty"`
	IsBeritaAcara    bool                   `json:"is_berita_acara"`
	WitnessID        *uint                  `json:"witness_id,omitempty"`
	Witness          *WitnessResponse       `json:"witness,omitempty"`
	BeritaAcaraNotes string                 `json:"berita_acara_notes,omitempty"`
}

// DailyActivityResponse represents a concise daily activity response without heavy nested relation models.
type DailyActivityResponse struct {
	ID               uint                   `json:"id"`
	Date             time.Time              `json:"date"`
	StartTime        string                 `json:"start_time"`
	EndTime          string                 `json:"end_time"`
	Activity         string                 `json:"activity"`
	ProjectName      string                 `json:"project_name"`
	ProjectID        string                 `json:"project_id"`
	Status           string                 `json:"status"`
	AppImpacted      string                 `json:"app_impacted"`
	ProjectRefID     *uint                  `json:"project_ref_id,omitempty"`
	ProjectRef       *models.Project        `json:"project_ref,omitempty"`
	StatusRef        *models.ActivityStatus `json:"status_ref,omitempty"`
	IsBeritaAcara    bool                   `json:"is_berita_acara"`
	WitnessID        *uint                  `json:"witness_id,omitempty"`
	Witness          *WitnessResponse       `json:"witness,omitempty"`
	BeritaAcaraNotes string                 `json:"berita_acara_notes,omitempty"`
}
