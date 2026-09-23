package queue

const (
	// TypeTimesheetGenerate is the task type identifier for timesheet generation jobs.
	TypeTimesheetGenerate = "timesheet:generate"
)

// GenerateTimesheetPayload carries the job parameters dispatched to Asynq workers.
type GenerateTimesheetPayload struct {
	JobID  string `json:"job_id"`
	UserID uint   `json:"user_id"`
	Month  int    `json:"month"`
	Year   int    `json:"year"`
}
