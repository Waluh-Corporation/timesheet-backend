package response

import "time"

// OvertimeResponse represents a concise overtime record response for SPL reporting.
type OvertimeResponse struct {
	ID                 uint      `json:"id"`
	Date               time.Time `json:"date"`
	StartTime          string    `json:"start_time"`
	EndTime            string    `json:"end_time"`
	TaskDescription    string    `json:"task_description"`
	TeamLeaderID       *uint     `json:"team_leader_id,omitempty"`
	TeamLeaderName     string    `json:"team_leader_name,omitempty"`
	DepartmentHeadID   *uint     `json:"department_head_id,omitempty"`
	DepartmentHeadName string    `json:"department_head_name,omitempty"`
}
