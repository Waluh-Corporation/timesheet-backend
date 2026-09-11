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
	Name              string       `json:"name" example:"John Doe"`
	BniID             string       `json:"bni_id" example:"12345678"`
	Site              string       `json:"site" example:"Jakarta"`
	SignatureEmployee string       `json:"signature_employee" example:"John Doe"`
	SignatureReviewer string       `json:"signature_reviewer" example:"Reviewer Name"`
	SignatureApprover string       `json:"signature_approver" example:"Approver Name"`
	DailyEntries      []DailyEntry `json:"daily_entries" binding:"required"`
}

// HolidayDTO represents a holiday exposed to the client or cached in system
type HolidayDTO struct {
	Date          string `json:"date"`
	Description   string `json:"description"`
	IsJointLeave  bool   `json:"is_joint_leave"`
	IsCutiBersama bool   `json:"is_cuti_bersama"`
	IsCivic       bool   `json:"is_civic"`
	IsReligious   bool   `json:"is_religious"`
}

// KemendesaHolidayItem represents a single holiday item from api.kemendesa.link
type KemendesaHolidayItem struct {
	Date          string `json:"date"`
	Name          string `json:"name"`
	IsCivic       bool   `json:"is_civic"`
	IsReligious   bool   `json:"is_religious"`
	IsCutiBersama bool   `json:"is_cuti_bersama"`
}

// KemendesaHolidayResponse represents the payload from api.kemendesa.link/libur-nasional
type KemendesaHolidayResponse struct {
	Metadata struct {
		Version     string `json:"version"`
		Year        int    `json:"year"`
		LastUpdated string `json:"last_updated"`
		Timezone    string `json:"timezone"`
	} `json:"metadata"`
	Data []KemendesaHolidayItem `json:"data"`
}
