package models

// DailyEntry represents a user's daily input for a specific day
type DailyEntry struct {
	Day         int    `json:"day" binding:"required,min=1,max=31" example:"1"`
	StartTime   string `json:"start_time" example:"08:00"`
	EndTime     string `json:"end_time" example:"17:00"`
	Status      string `json:"status" example:"P"`
	Activity    string `json:"activity" example:"Developing core features"`
	ProjectName string `json:"project_name" example:"BNI Direct Cash"`
	ProjectID   string `json:"project_id" example:"P24015"`
	AppImpacted string `json:"app_impacted" example:"BNI Mobile"`
	Division    string `json:"division" example:"Wholesale Digital Delivery"`
	Department  string `json:"department" example:"Wholesale Channel and Service Delivery"`
}

// TimesheetRequest represents the expected payload for timesheet generation
type TimesheetRequest struct {
	Month             int          `json:"month" binding:"required,min=1,max=12" example:"7"`
	Year              int          `json:"year" binding:"required,min=1000,max=9999" example:"2026"`
	Format            string       `json:"format" binding:"required,oneof=excel pdf" example:"excel"`
	Project           string       `json:"project" example:"Core Banking Modernization"`
	Division          string       `json:"division" example:"Application Development Division"`
	Name              string       `json:"name" example:"Faisal Al Munawar Fathur Rahman"`
	MiiID             string       `json:"mii_id" example:"MII-04828"`
	Site              string       `json:"site" example:"Jakarta"`
	SignatureEmployee string       `json:"signature_employee" example:"Faisal Al Munawar Fathur Rahman"`
	SignatureReviewer string       `json:"signature_reviewer" example:"Reviewer Name"`
	SignatureApprover string       `json:"signature_approver" example:"Approver Name"`
	DailyEntries      []DailyEntry `json:"daily_entries" binding:"required"`
}

// HolidayDTO represents a holiday from the external API
type HolidayDTO struct {
	Date        string `json:"date"`
	Description string `json:"description"`
}

// HolidayResponse represents the response format of the public holiday API
type HolidayResponse struct {
	Status  string       `json:"status"`
	Code    int          `json:"code"`
	Data    []HolidayDTO `json:"data"`
	Message string       `json:"message"`
}
