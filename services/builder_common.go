package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/config"
	"timesheet-backend/models"
)

// Default working hours constants.
const (
	DefaultStartTime = "08:00"
	DefaultEndTime   = "17:00"
)

var (
	defaultStartFrac, _ = parseTimeToExcelFraction(DefaultStartTime)
	defaultEndFrac, _   = parseTimeToExcelFraction(DefaultEndTime)
	defaultTotalFrac    = defaultEndFrac - defaultStartFrac // 9 hours fraction: 0.375
)

// CalculateTimeDurationFrac calculates duration in fraction of a 24-hour day.
func CalculateTimeDurationFrac(startStr, endStr string) float64 {
	startFrac, errStart := parseTimeToExcelFraction(startStr)
	endFrac, errEnd := parseTimeToExcelFraction(endStr)
	if errStart != nil || errEnd != nil {
		return defaultTotalFrac
	}
	dur := endFrac - startFrac
	if dur < 0 {
		dur += 1.0
	}
	return dur
}

// WorkingHoursConfig encapsulates column layout, row, formula, and numeric options for daily working hours.
type WorkingHoursConfig struct {
	StartCol       string
	EndCol         string
	TotalCol       string
	RowStr         string
	FormulaFmt     string
	IsDecimalTotal bool
}

// WriteWorkingHoursRow writes Start, End, and Total Hour cells according to activity or standard defaults.
// - If has activity: writes activity Start, End, calculated total duration, formula, and timeStyle.
// - If holiday/weekend without activity: clears Start, End, Total Hour cells and applies timeStyle.
// - If working day without activity: writes default 08:00, 17:00, 09:00 duration, formula, and timeStyle.
func WriteWorkingHoursRow(
	f *excelize.File,
	sheet string,
	cfg WorkingHoursConfig,
	dsc DayStyleContext,
) {
	totalStyle := dsc.TimeStyle
	if cfg.IsDecimalTotal {
		totalStyle = dsc.DecimalStyle
	}

	if dsc.HasActivity {
		writeActivityWorkingHours(f, sheet, cfg, totalStyle, dsc)
		return
	}
	if dsc.IsHolidayOrWeekend {
		writeHolidayWorkingHours(f, sheet, cfg, totalStyle, dsc.TimeStyle)
		return
	}
	writeDefaultWorkingHours(f, sheet, cfg, totalStyle, dsc.TimeStyle)
}

func writeActivityWorkingHours(
	f *excelize.File,
	sheet string,
	cfg WorkingHoursConfig,
	totalStyle int,
	dsc DayStyleContext,
) {
	startCell := cfg.StartCol + cfg.RowStr
	endCell := cfg.EndCol + cfg.RowStr
	totalCell := cfg.TotalCol + cfg.RowStr

	start := strings.TrimSpace(dsc.Activity.StartTime)
	end := strings.TrimSpace(dsc.Activity.EndTime)
	if start == "" {
		start = DefaultStartTime
	}
	if end == "" {
		end = DefaultEndTime
	}
	WriteTimeCells(f, sheet, startCell, endCell, start, end, dsc.TimeStyle)
	dur := CalculateTimeDurationFrac(start, end)
	if cfg.IsDecimalTotal {
		_ = f.SetCellValue(sheet, totalCell, dur*24.0)
	} else {
		_ = f.SetCellValue(sheet, totalCell, dur)
	}
	applyHoursFormula(f, sheet, totalCell, cfg.FormulaFmt, cfg.RowStr)
	_ = f.SetCellStyle(sheet, totalCell, totalCell, totalStyle)
}

func writeHolidayWorkingHours(
	f *excelize.File,
	sheet string,
	cfg WorkingHoursConfig,
	totalStyle, timeStyle int,
) {
	startCell := cfg.StartCol + cfg.RowStr
	endCell := cfg.EndCol + cfg.RowStr
	totalCell := cfg.TotalCol + cfg.RowStr

	_ = f.SetCellValue(sheet, startCell, "")
	_ = f.SetCellValue(sheet, endCell, "")
	_ = f.SetCellValue(sheet, totalCell, "")
	_ = f.SetCellStyle(sheet, startCell, startCell, timeStyle)
	_ = f.SetCellStyle(sheet, endCell, endCell, timeStyle)
	_ = f.SetCellStyle(sheet, totalCell, totalCell, totalStyle)
}

func writeDefaultWorkingHours(
	f *excelize.File,
	sheet string,
	cfg WorkingHoursConfig,
	totalStyle, timeStyle int,
) {
	startCell := cfg.StartCol + cfg.RowStr
	endCell := cfg.EndCol + cfg.RowStr
	totalCell := cfg.TotalCol + cfg.RowStr

	_ = f.SetCellValue(sheet, startCell, defaultStartFrac)
	_ = f.SetCellValue(sheet, endCell, defaultEndFrac)
	_ = f.SetCellStyle(sheet, startCell, startCell, timeStyle)
	_ = f.SetCellStyle(sheet, endCell, endCell, timeStyle)

	if cfg.IsDecimalTotal {
		_ = f.SetCellValue(sheet, totalCell, defaultTotalFrac*24.0)
	} else {
		_ = f.SetCellValue(sheet, totalCell, defaultTotalFrac)
	}
	applyHoursFormula(f, sheet, totalCell, cfg.FormulaFmt, cfg.RowStr)
	_ = f.SetCellStyle(sheet, totalCell, totalCell, totalStyle)
}

func applyHoursFormula(f *excelize.File, sheet, cell, formulaFmt, rs string) {
	if formulaFmt == "" {
		return
	}
	form := strings.ReplaceAll(formulaFmt, "{row}", rs)
	if strings.Contains(form, "%s") {
		form = fmt.Sprintf(form, rs, rs)
	}
	_ = f.SetCellFormula(sheet, cell, form)
}

// CleanWorkbookBuffer strips erroneous t="str" attributes from formula cells across all worksheet XMLs,
// ensuring Excel treats formula results as numbers/times instead of string literals.
func CleanWorkbookBuffer(in []byte) []byte {
	zr, err := zip.NewReader(bytes.NewReader(in), int64(len(in)))
	if err != nil {
		return in
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	reFormulaStr := regexp.MustCompile(`(<c\s+r="[A-Z0-9]+"[^>]*?)\s+t="str"([^>]*?>\s*<f[^>]*>.*?</f>)`)
	sheetXmlRe := regexp.MustCompile(`^xl/worksheets/.*\.xml$`)

	for _, zf := range zr.File {
		w, err := zw.CreateHeader(&zf.FileHeader)
		if err != nil {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}

		if sheetXmlRe.MatchString(zf.Name) {
			data = reFormulaStr.ReplaceAll(data, []byte(`$1$2`))
		}
		_, _ = w.Write(data)
	}
	_ = zw.Close()
	return out.Bytes()
}

// HeaderColumn defines a merged header cell range and its text label.
type HeaderColumn struct {
	From string
	To   string
	Val  string
}

// WriteTableHeaders merges and sets header cells and optional subheaders.
func WriteTableHeaders(f *excelize.File, sheet string, headers []HeaderColumn, subHeaders map[string]string, st *BuilderStyles) {
	for _, h := range headers {
		if h.From != h.To {
			_ = f.MergeCell(sheet, h.From, h.To)
		}
		_ = f.SetCellValue(sheet, h.From, h.Val)
		_ = f.SetCellStyle(sheet, h.From, h.To, st.HeaderStyle)
	}
	for cell, val := range subHeaders {
		_ = f.SetCellValue(sheet, cell, val)
		_ = f.SetCellStyle(sheet, cell, cell, st.HeaderGreyStyle)
	}
}

// WriteHolidayRemarkRow sets holiday or weekend text in the activity column and adjusts row height.
func WriteHolidayRemarkRow(f *excelize.File, sheet, col, rs string, row int, holiday string) {
	if holiday != "" {
		_ = f.SetCellValue(sheet, col+rs, holiday)
	} else {
		_ = f.SetCellValue(sheet, col+rs, "Weekend")
	}
	_ = f.SetRowHeight(sheet, row, 21)
}

// WriteAttendanceMatrixStatus writes the standard 6-column matrix attendance status ("E" through "J").
// When status is "X" (inactive), the cell is left empty (""), conforming to timesheet presentation rules.
func WriteAttendanceMatrixStatus(f *excelize.File, sheet, rs, status string) {
	normStatus := strings.ToUpper(strings.TrimSpace(status))
	statusCol := map[string]string{"P": "E", "S": "F", "BT": "G", "PM": "H", "V": "I", "X": "J"}
	statusMark := map[string]string{"P": "P", "S": "S", "BT": "BT", "PM": "PM", "V": "V", "X": ""}
	matrixCols := []string{"E", "F", "G", "H", "I", "J"}

	for _, col := range matrixCols {
		_ = f.SetCellValue(sheet, col+rs, "")
	}
	if col, ok := statusCol[normStatus]; ok {
		_ = f.SetCellValue(sheet, col+rs, statusMark[normStatus])
	}
}

// DayStyleContext holds metadata and style IDs resolved for a specific calendar day.
type DayStyleContext struct {
	Date               time.Time
	Holiday            string
	Activity           models.DailyActivity
	HasActivity        bool
	IsHolidayOrWeekend bool
	CenterStyle        int
	CenterWrapStyle    int
	DateStyle          int
	TimeStyle          int
	DecimalStyle       int
}

// ResolveDayStyleContext prepares date, holiday, activity, and style IDs for a day row.
func ResolveDayStyleContext(year, month, day int, holidays map[int]string, byDay map[int]models.DailyActivity, st *BuilderStyles) DayStyleContext {
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
	holiday := holidays[day]
	act, hasAct := byDay[day]
	isHolidayOrWeekend := isWeekend || holiday != ""

	ctx := DayStyleContext{
		Date:               date,
		Holiday:            holiday,
		Activity:           act,
		HasActivity:        hasAct,
		IsHolidayOrWeekend: isHolidayOrWeekend,
		CenterStyle:        st.DataCenterStyle,
		CenterWrapStyle:    st.DataCenterWrapStyle,
		DateStyle:          st.DateStyle,
		TimeStyle:          st.TimeStyle,
		DecimalStyle:       st.DecimalStyle,
	}
	if isHolidayOrWeekend && !hasAct {
		ctx.CenterStyle = st.GreyCenterStyle
		ctx.CenterWrapStyle = st.GreyCenterWrapStyle
		ctx.DateStyle = st.GreyDateStyle
		ctx.TimeStyle = st.GreyTimeStyle
		ctx.DecimalStyle = st.GreyDecimalStyle
	}
	return ctx
}

// ApplyDayRowStyles applies center style across all given columns, wraps specific columns, and writes the date cell.
func ApplyDayRowStyles(f *excelize.File, sheet, dateCol, rs string, cols, wrapCols []string, dsc DayStyleContext) {
	for _, col := range cols {
		_ = f.SetCellStyle(sheet, col+rs, col+rs, dsc.CenterStyle)
	}
	for _, col := range wrapCols {
		_ = f.SetCellStyle(sheet, col+rs, col+rs, dsc.CenterWrapStyle)
	}
	_ = f.SetCellValue(sheet, dateCol+rs, dsc.Date)
	_ = f.SetCellStyle(sheet, dateCol+rs, dateCol+rs, dsc.DateStyle)
}

// DailyRowConfig configures vendor-specific column mappings for daily row generation.
type DailyRowConfig struct {
	Columns       []string          // all table columns (e.g. ["A", "B", ...])
	WrapColumns   []string          // columns that require word wrap
	StatusColumns map[string]string // status code -> column letter (e.g. "P" -> "E")
	StatusMarks   map[string]string // status code -> mark string (e.g. "P" -> "P")
	HourFormula   string            // e.g. "C%s-B%s" or "IF(C%s>B%s,(C%s-B%s),C%s-B%s+1)"
	TimeColumn    string            // total hour column, e.g. "D"
	DefaultStatus string            // default status for working days without entry, e.g. "P"
}

// BuildByDayMap maps daily activities by day of month (1-31).
func BuildByDayMap(activities []models.DailyActivity) map[int]models.DailyActivity {
	byDay := make(map[int]models.DailyActivity, len(activities))
	for _, a := range activities {
		byDay[a.Date.Day()] = a
	}
	return byDay
}

// ExtractApprovers finds team leader and department head names from overtime entries.
func ExtractApprovers(overtimes []models.OvertimeEntry) (tlName string, dhName string) {
	for _, ot := range overtimes {
		if ot.TeamLeader != nil && ot.TeamLeader.Name != "" && tlName == "" {
			tlName = ot.TeamLeader.Name
		}
		if ot.DepartmentHead != nil && ot.DepartmentHead.Name != "" && dhName == "" {
			dhName = ot.DepartmentHead.Name
		}
	}
	return tlName, dhName
}

// ResolveApprovers finds team leader and department head names for timesheet signatures.
// It checks overtime entries first, and falls back to active approvers from master data.
func ResolveApprovers(in GenerationInput) (tlName string, dhName string) {
	tlName, dhName = ExtractApprovers(in.Overtimes)
	for _, appr := range in.Approvers {
		if tlName == "" && appr.RoleType == models.ApproverRoleTeamLeader && appr.IsActive {
			tlName = appr.Name
		}
		if dhName == "" && appr.RoleType == models.ApproverRoleDepartmentHead && appr.IsActive {
			dhName = appr.Name
		}
	}
	return tlName, dhName
}

// WriteTimeCells parses start and end times to Excel fractions and sets their cells.
func WriteTimeCells(f *excelize.File, sheet, startCell, endCell string, startStr, endStr string, timeStyle int) (hasStart, hasEnd bool) {
	if startStr != "" {
		if frac, err := parseTimeToExcelFraction(startStr); err == nil {
			_ = f.SetCellValue(sheet, startCell, frac)
			_ = f.SetCellStyle(sheet, startCell, startCell, timeStyle)
			hasStart = true
		}
	}
	if endStr != "" {
		if frac, err := parseTimeToExcelFraction(endStr); err == nil {
			_ = f.SetCellValue(sheet, endCell, frac)
			_ = f.SetCellStyle(sheet, endCell, endCell, timeStyle)
			hasEnd = true
		}
	}
	return hasStart, hasEnd
}

// ApplyBlankPaddingRow clears cell values and formats blank days beyond the current month's end.
func ApplyBlankPaddingRow(f *excelize.File, sheet string, row int, cols []string, wrapCols []string, st *BuilderStyles) {
	rs := fmt.Sprintf("%d", row)
	for _, col := range cols {
		_ = f.SetCellValue(sheet, col+rs, "")
		_ = f.SetCellStyle(sheet, col+rs, col+rs, st.DataCenterStyle)
	}
	for _, col := range wrapCols {
		_ = f.SetCellStyle(sheet, col+rs, col+rs, st.DataCenterWrapStyle)
	}
	_ = f.SetRowHeight(sheet, row, 21)
}

// WriteTotalSummaryRow writes the standard TOTAL row with formula sum and attendance counts.
func WriteTotalSummaryRow(f *excelize.File, sheet string, summaryRow, firstDataRow, lastDataRow int, statusCols map[string]string, st *BuilderStyles) {
	sr := fmt.Sprintf("%d", summaryRow)
	fdr := fmt.Sprintf("%d", firstDataRow)
	ldr := fmt.Sprintf("%d", lastDataRow)

	_ = f.SetCellValue(sheet, "A"+sr, "TOTAL")
	_ = f.SetCellStyle(sheet, "A"+sr, "A"+sr, st.BoldCenterStyle)

	for _, col := range []string{"B", "C"} {
		_ = f.SetCellStyle(sheet, col+sr, col+sr, st.BoldCenterStyle)
	}

	_ = f.SetCellFormula(sheet, "D"+sr, fmt.Sprintf("SUM(D%s:D%s)", fdr, ldr))
	_ = f.SetCellStyle(sheet, "D"+sr, "D"+sr, st.TimeStyle)

	for status, col := range statusCols {
		cell := col + sr
		mark := status
		if status == "X" {
			mark = "x"
		}
		_ = f.SetCellFormula(sheet, cell, fmt.Sprintf("COUNTIF(%s%s:%s%s, \"%s\")", col, fdr, col, ldr, mark))
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldCenterStyle)
	}
}

// SignatureParty defines one signer column block.
type SignatureParty struct {
	StartCol   string
	EndCol     string
	Title      string
	Name       string
	DatePrefix string
	Position   string
}

// WriteSignaturesLayout renders a configurable multi-party signature section.
func WriteSignaturesLayout(f *excelize.File, sheet string, headerRow int, boxRows int, parties []SignatureParty, st *BuilderStyles) {
	rHeader := fmt.Sprintf("%d", headerRow)
	rName := fmt.Sprintf("%d", headerRow+boxRows+1)
	rDate := fmt.Sprintf("%d", headerRow+boxRows+2)

	for _, p := range parties {
		// Header / Title
		styleMergedRange(f, sheet, p.StartCol+rHeader, p.EndCol+rHeader, st.BoldCenterStyle)
		_ = f.SetCellValue(sheet, p.StartCol+rHeader, p.Title)

		// Empty Signature Box
		if boxRows > 0 {
			rBoxStart := fmt.Sprintf("%d", headerRow+1)
			rBoxEnd := fmt.Sprintf("%d", headerRow+boxRows)
			styleMergedRange(f, sheet, p.StartCol+rBoxStart, p.EndCol+rBoxEnd, st.DataCenterStyle)
		}

		// Name
		nameStyle := st.BoldCenterStyle
		nameVal := p.Name
		if strings.HasPrefix(p.DatePrefix, "Nama :") {
			nameStyle = st.DataLeftStyle
			if p.Name != "" {
				nameVal = "Nama : " + p.Name
			} else {
				nameVal = "Nama : "
			}
		}
		styleMergedRange(f, sheet, p.StartCol+rName, p.EndCol+rName, nameStyle)
		_ = f.SetCellValue(sheet, p.StartCol+rName, nameVal)

		// Position (if provided)
		if p.Position != "" {
			styleMergedRange(f, sheet, p.StartCol+rDate, p.EndCol+rDate, st.BoldCenterStyle)
			_ = f.SetCellValue(sheet, p.StartCol+rDate, p.Position)
		} else if p.DatePrefix != "" && !strings.HasPrefix(p.DatePrefix, "Nama :") {
			// Date (if provided and not position)
			styleMergedRange(f, sheet, p.StartCol+rDate, p.EndCol+rDate, st.DataLeftStyle)
			_ = f.SetCellValue(sheet, p.StartCol+rDate, p.DatePrefix)
		}
	}
}

// WriteMetaField writes a label and value cell pair with their respective styles.
func WriteMetaField(f *excelize.File, sheet, lblCell, lblVal, valCell, valStr string, st *BuilderStyles) {
	_ = f.SetCellValue(sheet, lblCell, lblVal)
	_ = f.SetCellStyle(sheet, lblCell, lblCell, st.MetaLabelStyle)
	_ = f.SetCellValue(sheet, valCell, valStr)
	_ = f.SetCellStyle(sheet, valCell, valCell, st.MetaValueStyle)
}

// ResolveEmployeeID returns the user's EmployeeID or falls back to BniID.
func ResolveEmployeeID(u *models.User) string {
	if u == nil {
		return ""
	}
	if u.EmployeeID != "" {
		return u.EmployeeID
	}
	return u.BniID
}

// ActivityProjectInfo groups activity description and project metadata for timesheet row rendering.
type ActivityProjectInfo struct {
	Activity    string
	ProjectName string
	ProjectCode string
	AppImpacted string
}

// WriteActivityProjectCells populates activity and project fields (cols K, L, M, N) and sets calculated row height.
func WriteActivityProjectCells(f *excelize.File, sheet string, row int, info ActivityProjectInfo) {
	rs := fmt.Sprintf("%d", row)
	_ = f.SetCellValue(sheet, "K"+rs, info.Activity)
	_ = f.SetCellValue(sheet, "L"+rs, info.ProjectName)
	_ = f.SetCellValue(sheet, "M"+rs, info.ProjectCode)
	_ = f.SetCellValue(sheet, "N"+rs, info.AppImpacted)
	h := calculateRowHeight(info.Activity, info.ProjectName, info.ProjectCode, info.AppImpacted, "", "")
	_ = f.SetRowHeight(sheet, row, h)
}

// WriteColumnFormulas sets formulas and styles for a map of column letters to formula expressions on targetRow.
func WriteColumnFormulas(f *excelize.File, sheet, targetRow string, formulas map[string]string, styleID int) {
	for col, formula := range formulas {
		cell := col + targetRow
		_ = f.SetCellFormula(sheet, cell, formula)
		_ = f.SetCellStyle(sheet, cell, cell, styleID)
	}
}

// StandardGrid14Cols represents standard columns A through N for 14-column timesheet templates (NTT, SDD).
var StandardGrid14Cols = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}

// StandardGridWrapCols represents standard text-wrapped columns K through N for timesheets.
var StandardGridWrapCols = []string{"K", "L", "M", "N"}

// DailyRowSetupConfig configures common timesheet row setup.
type DailyRowSetupConfig struct {
	Row         int
	Day         int
	DaysInMonth int
	DateCol     string
	AllCols     []string
	WrapCols    []string
	HoursCfg    WorkingHoursConfig
}

// SetupDayRowStylesAndHours applies blank padding if day > daysInMonth, or resolves styles, sets the date cell, and writes working hours.
func SetupDayRowStylesAndHours(
	f *excelize.File,
	sheet string,
	in GenerationInput,
	byDay map[int]models.DailyActivity,
	st *BuilderStyles,
	cfg DailyRowSetupConfig,
) (*DayStyleContext, bool) {
	if cfg.Day > cfg.DaysInMonth {
		ApplyBlankPaddingRow(f, sheet, cfg.Row, cfg.AllCols, cfg.WrapCols, st)
		return nil, false
	}
	dsc := ResolveDayStyleContext(in.Year, in.Month, cfg.Day, in.Holidays, byDay, st)
	rs := fmt.Sprintf("%d", cfg.Row)
	ApplyDayRowStyles(f, sheet, cfg.DateCol, rs, cfg.AllCols, cfg.WrapCols, dsc)
	_ = f.SetCellValue(sheet, cfg.DateCol+rs, dsc.Date)

	cfg.HoursCfg.RowStr = rs
	WriteWorkingHoursRow(f, sheet, cfg.HoursCfg, dsc)
	return &dsc, true
}

// DailyRowsConfig encapsulates options for generating the 31-day timesheet grid.
type DailyRowsConfig struct {
	StartRow int
	AllCols  []string
	WrapCols []string
	Styles   *BuilderStyles
	WriteAct func(rs string, row int, dsc DayStyleContext)
}

// WriteBuilderDailyRows writes all 31 days for timesheet builders, handling padding, styles, and activities.
func WriteBuilderDailyRows(
	f *excelize.File,
	sheet string,
	in GenerationInput,
	cfg DailyRowsConfig,
) {
	byDay := BuildByDayMap(in.Activities)
	daysInMonth := GetDaysInMonth(in.Year, in.Month)

	for day := 1; day <= 31; day++ {
		row := cfg.StartRow + (day - 1)
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, cfg.AllCols, cfg.WrapCols, cfg.Styles)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, cfg.Styles)
		ApplyDayRowStyles(f, sheet, "A", rs, cfg.AllCols, cfg.WrapCols, dsc)

		status := ""
		if dsc.HasActivity {
			status = strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
			if cfg.WriteAct != nil {
				cfg.WriteAct(rs, row, dsc)
			}
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheet, "K", rs, row, dsc.Holiday)
		}

		WriteAttendanceMatrixStatus(f, sheet, rs, status)
	}
}

// CurrentDateFormatted returns the current date in the configured application timezone formatted as "02-Jan-2006".
func CurrentDateFormatted() string {
	return time.Now().In(config.AppLocation()).Format("02-Jan-2006")
}

// DatePrefixUpper is the standard uppercase prefix used in timesheet signature blocks.
const DatePrefixUpper = "DATE : "

// WriteSignaturesBlock renders the 3-party signature section (Employee, Team Leader, Dept Head).
func WriteSignaturesBlock(f *excelize.File, sheet string, startRow int, userName, tlName, dhName string, st *BuilderStyles) {
	dateStr := DatePrefixUpper + CurrentDateFormatted()
	WriteSignaturesLayout(f, sheet, startRow, 3, []SignatureParty{
		{StartCol: "A", EndCol: "C", Title: "Prepared by :", Name: userName, DatePrefix: dateStr},
		{StartCol: "D", EndCol: "F", Title: "Approved by :", Name: tlName, DatePrefix: dateStr},
		{StartCol: "G", EndCol: "J", Title: "Approved by :", Name: dhName, DatePrefix: dateStr},
	}, st)

	_ = f.SetRowHeight(sheet, startRow, 20)
	for i := 1; i <= 3; i++ {
		_ = f.SetRowHeight(sheet, startRow+i, 16)
	}
	_ = f.SetRowHeight(sheet, startRow+4, 22)
	_ = f.SetRowHeight(sheet, startRow+5, 18)
}

// SetWorkbookProperties configures document metadata properties for the workbook.
// Creator is set to "Waluh Corporation" while LastModifiedBy is set to the downloading user's name.
func SetWorkbookProperties(f *excelize.File, title, lastModifiedBy string) {
	author := "Waluh Corporation"
	modifiedBy := strings.TrimSpace(lastModifiedBy)
	if modifiedBy == "" {
		modifiedBy = author
	}
	_ = f.SetDocProps(&excelize.DocProperties{
		Title:          title,
		Creator:        author,
		LastModifiedBy: modifiedBy,
		Category:       "Timesheet",
	})
	bTrue := true
	_ = f.SetCalcProps(&excelize.CalcPropsOptions{
		FullCalcOnLoad: &bTrue,
		ForceFullCalc:  &bTrue,
	})
}

// MonthNameIndonesian returns the full Indonesian month name for month 1-12.
func MonthNameIndonesian(month int) string {
	names := []string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	if month >= 1 && month <= 12 {
		return names[month-1]
	}
	return fmt.Sprintf("Bulan %d", month)
}

// SignatureCells specifies the target cell coordinates and formatting for timesheet sign-offs.
type SignatureCells struct {
	UserCell     string
	TLCell       string
	DHCell       string
	UserDateCell string
	TLDateCell   string
	DHDateCell   string
	DatePrefix   string
	UserBoxCell  string
	QRCell       string
}

// EmbedPicture inserts a PNG picture into a cell with specified scale.
func EmbedPicture(f *excelize.File, sheet, cell string, picBytes []byte, scaleX, scaleY float64) error {
	if f == nil || len(picBytes) == 0 || cell == "" {
		return nil
	}
	bTrue := true
	return f.AddPictureFromBytes(sheet, cell, &excelize.Picture{
		Extension: ".png",
		File:      picBytes,
		Format: &excelize.GraphicOptions{
			PrintObject:     &bTrue,
			LockAspectRatio: true,
			ScaleX:          scaleX,
			ScaleY:          scaleY,
			Positioning:     "oneCell",
		},
	})
}

// WriteSignatures writes the user, team leader, department head names, and date stamp into signature cells.
func WriteSignatures(f *excelize.File, sheet string, in GenerationInput, cells SignatureCells) {
	tlName, dhName := ResolveApprovers(in)
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	setNonEmptyCell(f, sheet, cells.UserCell, userName)
	setNonEmptyCell(f, sheet, cells.TLCell, tlName)
	setNonEmptyCell(f, sheet, cells.DHCell, dhName)

	writeSignatureDates(f, sheet, cells)
	embedSignaturePictures(f, sheet, in, cells)
}

func setNonEmptyCell(f *excelize.File, sheet, cell, val string) {
	if cell != "" && val != "" {
		_ = f.SetCellValue(sheet, cell, val)
	}
}

func writeSignatureDates(f *excelize.File, sheet string, cells SignatureCells) {
	if cells.DatePrefix == "" {
		return
	}
	dateStr := cells.DatePrefix + CurrentDateFormatted()
	setNonEmptyCell(f, sheet, cells.UserDateCell, dateStr)
	setNonEmptyCell(f, sheet, cells.TLDateCell, dateStr)
	setNonEmptyCell(f, sheet, cells.DHDateCell, dateStr)
}

func embedSignaturePictures(f *excelize.File, sheet string, in GenerationInput, cells SignatureCells) {
	if !in.IsSigned {
		return
	}
	if len(in.SignatureBytes) > 0 && cells.UserBoxCell != "" {
		_ = EmbedPicture(f, sheet, cells.UserBoxCell, in.SignatureBytes, 0.4, 0.4)
	}
	if len(in.QRCodeBytes) > 0 && cells.QRCell != "" {
		_ = EmbedPicture(f, sheet, cells.QRCell, in.QRCodeBytes, 0.6, 0.6)
	}
}

// SummaryFormulaItem defines a column formula and pre-calculated value for summary rows.
type SummaryFormulaItem struct {
	Formula string
	Value   int
}

// WriteSummaryRowWithValues writes both the pre-calculated integer value and Excel formula
// into summary cells, ensuring the total displays immediately upon opening and recalculates on edit.
func WriteSummaryRowWithValues(f *excelize.File, sheet, targetRow string, items map[string]SummaryFormulaItem, styleID int) {
	for col, item := range items {
		cell := col + targetRow
		_ = f.SetCellValue(sheet, cell, item.Value)
		_ = f.SetCellFormula(sheet, cell, item.Formula)
		if styleID > 0 {
			_ = f.SetCellStyle(sheet, cell, cell, styleID)
		}
	}
}

// WriteWorkbookToBuffer writes the workbook to a byte slice buffer, cleans formula attributes, or returns an error.
func WriteWorkbookToBuffer(f *excelize.File, companyName string) ([]byte, error) {
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write %s buffer: %w", companyName, err)
	}
	return CleanWorkbookBuffer(buf.Bytes()), nil
}
