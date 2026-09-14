package request

// BeritaAcaraRequest represents the payload to create or update a Berita Acara entry.
type BeritaAcaraRequest struct {
	Date             string `json:"date" binding:"required" example:"2026-09-01"`
	Day              string `json:"day,omitempty" example:"Selasa"`
	StartTime        string `json:"start_time" example:"08:00"`
	EndTime          string `json:"end_time" example:"17:00"`
	Keterangan       string `json:"keterangan" binding:"required" example:"Lupa absen datang / Work From Office"`
	DailyActivityID  *uint  `json:"daily_activity_id,omitempty" example:"1"`
	TeamLeaderID     *uint  `json:"team_leader_id,omitempty" example:"2"`
	DepartmentHeadID *uint  `json:"department_head_id,omitempty" example:"3"`
}
