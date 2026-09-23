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

// WriteWorkingHoursRow writes Start, End, and Total Hour cells according to activity or standard defaults.
// - If has activity: writes activity Start, End, calculated total duration, formula, and timeStyle.
// - If holiday/weekend without activity: clears Start, End, Total Hour cells and applies timeStyle.
// - If working day without activity: writes default 08:00, 17:00, 09:00 duration, formula, and timeStyle.
func WriteWorkingHoursRow(
	f *excelize.File,
	sheet string,
	startCol, endCol, totalCol, rs string,
	formulaFmt string,
	isDecimalTotal bool,
	dsc DayStyleContext,
) {
	startCell := startCol + rs
	endCell := endCol + rs
	totalCell := totalCol + rs

	totalStyle := dsc.TimeStyle
	if isDecimalTotal {
		totalStyle = dsc.DecimalStyle
	}

	if dsc.HasActivity {
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
		if isDecimalTotal {
			_ = f.SetCellValue(sheet, totalCell, dur*24.0)
		} else {
			_ = f.SetCellValue(sheet, totalCell, dur)
		}
		if formulaFmt != "" {
			form := strings.ReplaceAll(formulaFmt, "{row}", rs)
			if strings.Contains(form, "%s") {
				form = fmt.Sprintf(form, rs, rs)
			}
			_ = f.SetCellFormula(sheet, totalCell, form)
		}
		_ = f.SetCellStyle(sheet, totalCell, totalCell, totalStyle)
	} else if dsc.IsHolidayOrWeekend {
		_ = f.SetCellValue(sheet, startCell, "")
		_ = f.SetCellValue(sheet, endCell, "")
		_ = f.SetCellValue(sheet, totalCell, "")
		_ = f.SetCellStyle(sheet, startCell, startCell, dsc.TimeStyle)
		_ = f.SetCellStyle(sheet, endCell, endCell, dsc.TimeStyle)
		_ = f.SetCellStyle(sheet, totalCell, totalCell, totalStyle)
	} else {
		// Working day without activity: default 08:00 and 17:00
		_ = f.SetCellValue(sheet, startCell, defaultStartFrac)
		_ = f.SetCellValue(sheet, endCell, defaultEndFrac)
		_ = f.SetCellStyle(sheet, startCell, startCell, dsc.TimeStyle)
		_ = f.SetCellStyle(sheet, endCell, endCell, dsc.TimeStyle)

		if isDecimalTotal {
			_ = f.SetCellValue(sheet, totalCell, defaultTotalFrac*24.0)
		} else {
			_ = f.SetCellValue(sheet, totalCell, defaultTotalFrac)
		}
		if formulaFmt != "" {
			form := strings.ReplaceAll(formulaFmt, "{row}", rs)
			if strings.Contains(form, "%s") {
				form = fmt.Sprintf(form, rs, rs)
			}
			_ = f.SetCellFormula(sheet, totalCell, form)
		}
		_ = f.SetCellStyle(sheet, totalCell, totalCell, totalStyle)
	}
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
	sheetXmlRe := regexp.MustCompile(`^xl/worksheets/sheet\d+\.xml$`)

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
	_ = f.SetRowHeight(sheet, row, 15)
}

// WriteAttendanceMatrixStatus writes the standard 6-column matrix attendance status ("E" through "J").
func WriteAttendanceMatrixStatus(f *excelize.File, sheet, rs, status string) {
	statusCol := map[string]string{"P": "E", "S": "F", "BT": "G", "PM": "H", "V": "I", "X": "J"}
	statusMark := map[string]string{"P": "P", "S": "S", "BT": "BT", "PM": "PM", "V": "V", "X": "x"}
	matrixCols := []string{"E", "F", "G", "H", "I", "J"}

	for _, col := range matrixCols {
		_ = f.SetCellValue(sheet, col+rs, "")
	}
	if col, ok := statusCol[status]; ok {
		_ = f.SetCellValue(sheet, col+rs, statusMark[status])
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
	if isHolidayOrWeekend {
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
	_ = f.SetRowHeight(sheet, row, 15)
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

// WriteActivityProjectCells populates activity and project fields (cols K, L, M, N) and sets calculated row height.
func WriteActivityProjectCells(f *excelize.File, sheet, rs string, row int, activity, projName, projCode, appImpacted string) {
	_ = f.SetCellValue(sheet, "K"+rs, activity)
	_ = f.SetCellValue(sheet, "L"+rs, projName)
	_ = f.SetCellValue(sheet, "M"+rs, projCode)
	_ = f.SetCellValue(sheet, "N"+rs, appImpacted)
	h := calculateRowHeight(activity, projName, projCode, appImpacted, "", "")
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

// WriteBuilderDailyRows writes all 31 days for timesheet builders, handling padding, styles, and activities.
func WriteBuilderDailyRows(
	f *excelize.File,
	sheet string,
	in GenerationInput,
	startRow int,
	allCols []string,
	wrapCols []string,
	st *BuilderStyles,
	writeAct func(rs string, row int, dsc DayStyleContext),
) {
	byDay := BuildByDayMap(in.Activities)
	daysInMonth := GetDaysInMonth(in.Year, in.Month)

	for day := 1; day <= 31; day++ {
		row := startRow + (day - 1)
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, allCols, wrapCols, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheet, "A", rs, allCols, wrapCols, dsc)

		status := ""
		if dsc.HasActivity {
			status = strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
			if writeAct != nil {
				writeAct(rs, row, dsc)
			}
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheet, "K", rs, row, dsc.Holiday)
		}

		WriteAttendanceMatrixStatus(f, sheet, rs, status)
	}
}

// CurrentDateFormatted returns the current date in Asia/Jakarta timezone formatted as "02-Jan-2006".
func CurrentDateFormatted() string {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return time.Now().In(loc).Format("02-Jan-2006")
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

// WriteSignatures writes the user, team leader, department head names, and date stamp into signature cells.
func WriteSignatures(f *excelize.File, sheet string, in GenerationInput, userCell, tlCell, dhCell, userDateCell, tlDateCell, dhDateCell, datePrefix string) {
	tlName, dhName := ResolveApprovers(in)
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	if userName != "" && userCell != "" {
		_ = f.SetCellValue(sheet, userCell, userName)
	}
	if tlName != "" && tlCell != "" {
		_ = f.SetCellValue(sheet, tlCell, tlName)
	}
	if dhName != "" && dhCell != "" {
		_ = f.SetCellValue(sheet, dhCell, dhName)
	}
	if datePrefix != "" {
		dateStr := datePrefix + CurrentDateFormatted()
		if userDateCell != "" {
			_ = f.SetCellValue(sheet, userDateCell, dateStr)
		}
		if tlDateCell != "" {
			_ = f.SetCellValue(sheet, tlDateCell, dateStr)
		}
		if dhDateCell != "" {
			_ = f.SetCellValue(sheet, dhDateCell, dateStr)
		}
	}
}

// WriteWorkbookToBuffer writes the workbook to a byte slice buffer or returns an error.
func WriteWorkbookToBuffer(f *excelize.File, companyName string) ([]byte, error) {
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write %s buffer: %w", companyName, err)
	}
	return buf.Bytes(), nil
}
