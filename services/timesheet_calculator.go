package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"timesheet-backend/models"
)

// DailyPresentationItem represents a single day's prepared timesheet data.
type DailyPresentationItem struct {
	Day               int       `json:"day"`
	Date              time.Time `json:"date"`
	DateFormatted     string    `json:"date_formatted"`
	DayOfWeek         string    `json:"day_of_week"`
	IsWeekend         bool      `json:"is_weekend"`
	IsHoliday         bool      `json:"is_holiday"`
	HolidayDesc       string    `json:"holiday_desc"`
	HasActivity       bool      `json:"has_activity"`
	IsOvertime        bool      `json:"is_overtime"`
	IsNonWorkingRow   bool      `json:"is_non_working_row"`
	StartTime         string    `json:"start_time"`
	EndTime           string    `json:"end_time"`
	DurationHours     float64   `json:"duration_hours"`
	DurationFraction  float64   `json:"duration_fraction"`
	DurationFormatted string    `json:"duration_formatted"`
	Status            string    `json:"status"`
	StatusDisplay     string    `json:"status_display"`
	Activity          string    `json:"activity"`
	ProjectName       string    `json:"project_name"`
	ProjectID         string    `json:"project_id"`
	AppImpacted       string    `json:"app_impacted"`
	Division          string    `json:"division"`
	Department        string    `json:"department"`
	Remark            string    `json:"remark"`
}

// SummaryPresentationModel encapsulates timesheet aggregate metrics.
type SummaryPresentationModel struct {
	TotalDays          int            `json:"total_days"`
	TotalWorkingDays   int            `json:"total_working_days"`
	TotalPresentDays   int            `json:"total_present_days"`
	TotalSickDays      int            `json:"total_sick_days"`
	TotalBusinessTrip  int            `json:"total_business_trip"`
	TotalPermitDays    int            `json:"total_permit_days"`
	TotalLeaveDays     int            `json:"total_leave_days"`
	TotalInactiveDays  int            `json:"total_inactive_days"`
	TotalWorkingHours  float64        `json:"total_working_hours"`
	TotalOvertimeHours float64        `json:"total_overtime_hours"`
	StatusCounts       map[string]int `json:"status_counts"`
}

// ApproverPresentationModel holds resolved signers and metadata.
type ApproverPresentationModel struct {
	EmployeeName        string    `json:"employee_name"`
	EmployeeID          string    `json:"employee_id"`
	EmployeeTitle       string    `json:"employee_title"`
	TeamLeadName        string    `json:"team_lead_name"`
	TeamLeadTitle       string    `json:"team_lead_title"`
	DepartmentHeadName  string    `json:"department_head_name"`
	DepartmentHeadTitle string    `json:"department_head_title"`
	SignatureBytes      []byte    `json:"-"`
	QRCodeBytes         []byte    `json:"-"`
	IsSigned            bool      `json:"is_signed"`
	SignDate            time.Time `json:"sign_date"`
}

// TimesheetPresentationModel is the canonical, printer-ready domain presentation model.
type TimesheetPresentationModel struct {
	CompanyCode   string                    `json:"company_code"`
	CompanyName   string                    `json:"company_name"`
	Month         int                       `json:"month"`
	Year          int                       `json:"year"`
	PeriodDisplay string                    `json:"period_display"`
	DaysInMonth   int                       `json:"days_in_month"`
	User          *models.User              `json:"-"`
	EmployeeName  string                    `json:"employee_name"`
	EmployeeID    string                    `json:"employee_id"`
	Division      string                    `json:"division"`
	Department    string                    `json:"department"`
	Site          string                    `json:"site"`
	ProjectName   string                    `json:"project_name"`
	ProjectID     string                    `json:"project_id"`
	Rows          []DailyPresentationItem   `json:"rows"`
	Summary       SummaryPresentationModel  `json:"summary"`
	Approvers     ApproverPresentationModel `json:"approvers"`
}

var indonesianMonths = [...]string{
	"",
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// FormatIndonesianPeriod returns a formatted month and year string in Indonesian (e.g. "Oktober 2026").
func FormatIndonesianPeriod(month, year int) string {
	if month >= 1 && month <= 12 {
		return fmt.Sprintf("%s %d", indonesianMonths[month], year)
	}
	return fmt.Sprintf("%02d/%d", month, year)
}

func formatDurationHours(frac float64) string {
	if frac <= 0 {
		return ""
	}
	totalMinutes := int(math.Round(frac * 1440.0))
	h := totalMinutes / 60
	m := totalMinutes % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

func resolveCompanyName(code string, user *models.User) string {
	if user != nil {
		if user.CompanyRel != nil && user.CompanyRel.Name != "" {
			return user.CompanyRel.Name
		}
		if user.Company != "" {
			return user.Company
		}
	}
	switch strings.ToLower(code) {
	case "mii":
		return "PT Mitra Integrasi Informatika"
	case "sdd":
		return "PT Sumber Daya Dian Mandiri"
	case "adidata":
		return "PT Adidata Informatika"
	case "ntt":
		return "PT NTT Indonesia"
	default:
		return "PT Mitra Integrasi Informatika"
	}
}

// CalculateTimesheetModel calculates and normalizes all business rules once to produce
// the canonical TimesheetPresentationModel consumed identically by Excelize and Maroto v2.
func CalculateTimesheetModel(input *GenerationInput) (*TimesheetPresentationModel, error) {
	if input == nil {
		return nil, errors.New("generation input cannot be nil")
	}
	if input.Month < 1 || input.Month > 12 {
		return nil, fmt.Errorf("invalid month: %d (must be 1-12)", input.Month)
	}
	if input.Year < 1900 {
		return nil, fmt.Errorf("invalid year: %d", input.Year)
	}

	daysInMonth := time.Date(input.Year, time.Month(input.Month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	byDay := make(map[int]models.DailyActivity, len(input.Activities))
	projectName := ""
	projectID := ""
	for _, a := range input.Activities {
		byDay[a.Date.Day()] = a
		if projectName == "" {
			projectName = a.GetProjectName()
		}
		if projectID == "" {
			projectID = a.GetProjectCode()
		}
	}

	empName := ""
	empID := ""
	division := ""
	department := ""
	site := ""

	if input.User != nil {
		empName = input.User.Name
		if empName == "" {
			empName = input.User.Username
		}
		empID = input.User.BniID
		division = input.User.Division
		department = input.User.Department
		site = input.User.Site
	}

	model := &TimesheetPresentationModel{
		CompanyCode:   strings.ToLower(input.CompanyCode),
		CompanyName:   resolveCompanyName(input.CompanyCode, input.User),
		Month:         input.Month,
		Year:          input.Year,
		PeriodDisplay: FormatIndonesianPeriod(input.Month, input.Year),
		DaysInMonth:   daysInMonth,
		User:          input.User,
		EmployeeName:  empName,
		EmployeeID:    empID,
		Division:      division,
		Department:    department,
		Site:          site,
		ProjectName:   projectName,
		ProjectID:     projectID,
		Rows:          make([]DailyPresentationItem, 0, daysInMonth),
		Summary: SummaryPresentationModel{
			TotalDays:    daysInMonth,
			StatusCounts: make(map[string]int),
		},
	}

	for day := 1; day <= daysInMonth; day++ {
		date := time.Date(input.Year, time.Month(input.Month), day, 0, 0, 0, 0, time.UTC)
		item := buildDailyPresentationItem(date, day, input, byDay, division, department, projectName, projectID)
		model.Rows = append(model.Rows, item)
		updateSummaryMetrics(&model.Summary, item)
	}

	tlName, dhName := ResolveApprovers(*input)
	model.Approvers = ApproverPresentationModel{
		EmployeeName:        empName,
		EmployeeID:          empID,
		EmployeeTitle:       "Consultant",
		TeamLeadName:        tlName,
		TeamLeadTitle:       "Team Leader",
		DepartmentHeadName:  dhName,
		DepartmentHeadTitle: "Department Head",
		SignatureBytes:      input.SignatureBytes,
		QRCodeBytes:         input.QRCodeBytes,
		IsSigned:            input.IsSigned,
		SignDate:            time.Now().UTC(),
	}

	return model, nil
}

func buildDailyPresentationItem(
	date time.Time,
	day int,
	input *GenerationInput,
	byDay map[int]models.DailyActivity,
	defaultDiv, defaultDept, defaultProjName, defaultProjID string,
) DailyPresentationItem {
	dayOfWeek := IndonesianDayName(date.Weekday())
	isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday

	holidayDesc := ""
	if input.Holidays != nil {
		holidayDesc = input.Holidays[day]
	}
	isHoliday := holidayDesc != ""
	isCalendarNonWorking := isWeekend || isHoliday

	act, hasAct := byDay[day]

	item := DailyPresentationItem{
		Day:           day,
		Date:          date,
		DateFormatted: date.Format("02/01/2006"),
		DayOfWeek:     dayOfWeek,
		IsWeekend:     isWeekend,
		IsHoliday:     isHoliday,
		HolidayDesc:   holidayDesc,
		Division:      defaultDiv,
		Department:    defaultDept,
		ProjectName:   defaultProjName,
		ProjectID:     defaultProjID,
	}

	if hasAct {
		populateActiveDailyItem(&item, act, isCalendarNonWorking)
	} else if isCalendarNonWorking {
		populateInactiveNonWorkingDailyItem(&item, isHoliday, holidayDesc)
	} else {
		populateDefaultWorkingDailyItem(&item)
	}

	return item
}

func populateActiveDailyItem(item *DailyPresentationItem, act models.DailyActivity, isCalendarNonWorking bool) {
	item.HasActivity = true
	item.IsOvertime = isCalendarNonWorking
	item.IsNonWorkingRow = false

	start := strings.TrimSpace(act.StartTime)
	end := strings.TrimSpace(act.EndTime)
	if start == "" {
		start = DefaultStartTime
	}
	if end == "" {
		end = DefaultEndTime
	}

	durFrac := CalculateTimeDurationFrac(start, end)
	durHours := durFrac * 24.0

	item.StartTime = start
	item.EndTime = end
	item.DurationFraction = durFrac
	item.DurationHours = durHours
	item.DurationFormatted = formatDurationHours(durFrac)

	status := strings.ToUpper(strings.TrimSpace(act.Status))
	if status == "" {
		status = "P"
	}
	item.Status = status
	if status == "X" {
		item.StatusDisplay = ""
	} else {
		item.StatusDisplay = status
	}

	item.Activity = act.Activity
	item.Remark = act.Activity
	if pName := act.GetProjectName(); pName != "" {
		item.ProjectName = pName
	}
	if pCode := act.GetProjectCode(); pCode != "" {
		item.ProjectID = pCode
	}
	item.AppImpacted = NormalizeMIIAppImpacted(act.GetAppImpacted())
}

func populateInactiveNonWorkingDailyItem(item *DailyPresentationItem, isHoliday bool, holidayDesc string) {
	item.HasActivity = false
	item.IsOvertime = false
	item.IsNonWorkingRow = true
	item.StartTime = ""
	item.EndTime = ""
	item.DurationFraction = 0
	item.DurationHours = 0
	item.DurationFormatted = ""
	item.Status = ""
	item.StatusDisplay = ""

	if isHoliday && holidayDesc != "" {
		item.Remark = holidayDesc
	} else {
		item.Remark = "Weekend"
	}
}

func populateDefaultWorkingDailyItem(item *DailyPresentationItem) {
	item.HasActivity = false
	item.IsOvertime = false
	item.IsNonWorkingRow = false
	item.StartTime = DefaultStartTime
	item.EndTime = DefaultEndTime
	item.DurationFraction = defaultTotalFrac
	item.DurationHours = defaultTotalFrac * 24.0
	item.DurationFormatted = formatDurationHours(defaultTotalFrac)
	item.Status = "P"
	item.StatusDisplay = "P"
	item.Remark = ""
}

func updateSummaryMetrics(summary *SummaryPresentationModel, item DailyPresentationItem) {
	if !item.IsNonWorkingRow {
		summary.TotalWorkingDays++
	}
	if item.Status != "" {
		summary.StatusCounts[item.Status]++
	}
	switch item.Status {
	case "P":
		summary.TotalPresentDays++
	case "S":
		summary.TotalSickDays++
	case "BT":
		summary.TotalBusinessTrip++
	case "PM":
		summary.TotalPermitDays++
	case "V":
		summary.TotalLeaveDays++
	case "X":
		summary.TotalInactiveDays++
	}
	if item.IsOvertime {
		summary.TotalOvertimeHours += item.DurationHours
	} else if !item.IsNonWorkingRow {
		summary.TotalWorkingHours += item.DurationHours
	}
}
