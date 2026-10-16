package request

// DailyActivityRequest represents a single day's timesheet input.
type DailyActivityRequest struct {
	Date         string `json:"date" binding:"required" example:"2026-09-01"`
	StartTime    string `json:"start_time" example:"08:00"`
	EndTime      string `json:"end_time" example:"17:00"`
	Status       string `json:"status" example:"P"`
	Activity     string `json:"activity" example:"Developing core features and API normalization"`
	ProjectName  string `json:"project_name" example:"BNI Direct Cash"`
	ProjectID    string `json:"project_id" example:"P24015"`
	AppImpacted  string `json:"app_impacted" example:"BNI Mobile"`
	ProjectRefID *uint  `json:"project_ref_id" example:"1"`
}
