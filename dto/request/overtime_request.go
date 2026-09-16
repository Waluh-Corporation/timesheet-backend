package request

// OvertimeRequest carries data to create/update an overtime entry.
type OvertimeRequest struct {
	ID               uint   `json:"id" example:"1"`
	Date             string `json:"date" binding:"required" example:"2026-09-01"` // YYYY-MM-DD
	StartTime        string `json:"start_time" binding:"required" example:"17:00"`
	EndTime          string `json:"end_time" binding:"required" example:"21:00"`
	TaskDescription  string `json:"task_description" binding:"required" example:"Production bug fixing and system deployment"`
	TeamLeaderID     *uint  `json:"team_leader_id" example:"2"`
	DepartmentHeadID *uint  `json:"department_head_id" example:"3"`
}
