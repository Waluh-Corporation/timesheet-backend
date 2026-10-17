package request

// DailyActivityRequest represents a single day's timesheet input.
type DailyActivityRequest struct {
	Date             string `json:"date" binding:"required" example:"2026-09-01"`
	StartTime        string `json:"start_time" example:"08:00"`
	EndTime          string `json:"end_time" example:"17:00"`
	Status           string `json:"status" example:"P"`
	Activity         string `json:"activity" example:"Developing core features and API normalization"`
	ProjectName      string `json:"project_name" example:"BNI Direct Cash"`
	ProjectID        string `json:"project_id" example:"P24015"`
	AppImpacted      string `json:"app_impacted" example:"BNI Mobile"`
	ProjectRefID     *uint  `json:"project_ref_id" example:"1"`
	IsBeritaAcara    bool   `json:"is_berita_acara"`
	WitnessID        *uint  `json:"witness_id,omitempty" example:"2"`
	BeritaAcaraNotes string `json:"berita_acara_notes,omitempty" example:"Lupa tap absen masuk"`
}

// UpdateMonthlyWitnessRequest represents the payload to update the witness for all Berita Acara entries in a given month.
type UpdateMonthlyWitnessRequest struct {
	Month     int  `json:"month" binding:"required,min=1,max=12" example:"11"`
	Year      int  `json:"year" binding:"required,min=2000" example:"2026"`
	WitnessID uint `json:"witness_id" binding:"required" example:"2"`
}
