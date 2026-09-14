package request

// GenerateRequest specifies parameters to render and download a timesheet workbook, berita acara, or both.
type GenerateRequest struct {
	Month int    `json:"month" binding:"required,min=1,max=12" example:"9"`
	Year  int    `json:"year" binding:"required,min=2000,max=9999" example:"2026"`
	Type  string `json:"type" example:"both"` // "timesheet", "berita_acara", or "both" (defaults to "timesheet")
}
