package response

// MonthlySummaryDTO represents an aggregated summary for a single month.
type MonthlySummaryDTO struct {
	Month               int            `json:"month"`
	MonthName           string         `json:"month_name"`
	WorkingDays         int            `json:"working_days"`
	DaysFilled          int            `json:"days_filled"`
	IsComplete          bool           `json:"is_complete"`
	WorkingHours        float64        `json:"working_hours"`
	OvertimeHours       float64        `json:"overtime_hours"`
	AttendanceBreakdown map[string]int `json:"attendance_breakdown"`
}

// TimesheetSummaryResponse represents monthly and yearly aggregated timesheet metrics.
type TimesheetSummaryResponse struct {
	Year                      int                 `json:"year"`
	Month                     *int                `json:"month,omitempty"`
	TotalWorkingDays          int                 `json:"total_working_days"`
	TotalDaysFilled           int                 `json:"total_days_filled"`
	TotalWorkingHours         float64             `json:"total_working_hours"`
	TotalOvertimeHours        float64             `json:"total_overtime_hours"`
	YearlyAttendanceBreakdown map[string]int      `json:"yearly_attendance_breakdown"`
	Months                    []MonthlySummaryDTO `json:"months"`
}
