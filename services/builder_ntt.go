package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

const nttDatePrefix = "DATE: "

func setNTTColWidths(f *excelize.File, sheet string) {
	widths := map[string]float64{
		"A": 12, "B": 8, "C": 8, "D": 11,
		"E": 8, "F": 8, "G": 13, "H": 8, "I": 8, "J": 12,
		"K": 35, "L": 18, "M": 14, "N": 25,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}
}

func writeNTTMetadata(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	setMeta := func(cellLabel, label, cellVal, val string) {
		_ = f.SetCellValue(sheet, cellLabel, label)
		_ = f.SetCellStyle(sheet, cellLabel, cellLabel, st.MetaLabelStyle)
		_ = f.SetCellValue(sheet, cellVal, val)
		_ = f.SetCellStyle(sheet, cellVal, cellVal, st.MetaValueStyle)
	}

	empID := in.User.EmployeeID
	if empID == "" {
		empID = in.User.BniID
	}
	div := in.User.Division
	if div == "" {
		div = "WDL"
	}
	periodStr := fmt.Sprintf("%s-%02d", time.Month(in.Month).String()[:3], in.Year%100)

	setMeta("A4", "NAME of PROJECT", "B4", ": BNIdirect")
	setMeta("A5", "UNIT / DIVISION", "B5", ": "+div)
	setMeta("A6", "NAME", "B6", ": "+in.User.Name)
	setMeta("A7", "NTT ID", "B7", ": "+empID)
	_ = f.SetCellValue(sheet, "J7", ": Holiday")

	setMeta("A8", "PERIODE", "B8", ": "+periodStr)
	_ = f.SetCellValue(sheet, "I8", "P = Present; S = Sick;  V = Vacation; BT = Business Trip; PM = Permit; X = Not Working Anymore")
}

func writeNTTHeaders(f *excelize.File, sheet string, st *BuilderStyles) {
	type colHeader struct {
		from, to string
		val      string
	}
	mergedHeaders := []colHeader{
		{"A9", "A10", "DATE"},
		{"B9", "C9", "WORKING HOUR"},
		{"D9", "D10", "TOTAL HOUR"},
		{"E9", "J9", "STATUS  ATTENDANCE"},
		{"K9", "K10", "ACTIVITY / REMARK"},
		{"L9", "L10", "Project Name"},
		{"M9", "M10", "Project Code"},
		{"N9", "N10", "Application Name"},
	}
	for _, mh := range mergedHeaders {
		if mh.from != mh.to {
			_ = f.MergeCell(sheet, mh.from, mh.to)
		}
		_ = f.SetCellValue(sheet, mh.from, mh.val)
		_ = f.SetCellStyle(sheet, mh.from, mh.to, st.HeaderStyle)
	}

	subHeaders := map[string]string{
		"B10": "START", "C10": "END",
		"E10": "Present", "F10": "Sick", "G10": "Business Trip",
		"H10": "Permit", "I10": "Vacation", "J10": "Not Working",
	}
	for cell, val := range subHeaders {
		_ = f.SetCellValue(sheet, cell, val)
		_ = f.SetCellStyle(sheet, cell, cell, st.HeaderGreyStyle)
	}
}

func writeNTTActRow(f *excelize.File, sheet, rs string, row int, act models.DailyActivity, timeStyle int) {
	hasStart, hasEnd := WriteTimeCells(f, sheet, "B"+rs, "C"+rs, act.StartTime, act.EndTime, timeStyle)
	if hasStart && hasEnd {
		_ = f.SetCellFormula(sheet, "D"+rs, fmt.Sprintf("IF(C%s>B%s,(C%s-B%s),C%s-B%s+1)", rs, rs, rs, rs, rs, rs))
		_ = f.SetCellStyle(sheet, "D"+rs, "D"+rs, timeStyle)
	}

	_ = f.SetCellValue(sheet, "K"+rs, act.Activity)
	projName := act.GetProjectName()
	if projName == "" {
		projName = "BNI Direct"
	}
	projCode := act.GetProjectCode()
	if projCode == "" {
		projCode = "P24015"
	}
	_ = f.SetCellValue(sheet, "L"+rs, projName)
	_ = f.SetCellValue(sheet, "M"+rs, projCode)
	_ = f.SetCellValue(sheet, "N"+rs, act.GetAppImpacted())

	h := calculateRowHeight(act.Activity, projName, projCode, act.GetAppImpacted(), "", "")
	_ = f.SetRowHeight(sheet, row, h)
}

func writeNTTHolidayRow(f *excelize.File, sheet, rs string, row int, holiday string) {
	if holiday != "" {
		_ = f.SetCellValue(sheet, "K"+rs, holiday)
	} else {
		_ = f.SetCellValue(sheet, "K"+rs, "Weekend")
	}
	_ = f.SetRowHeight(sheet, row, 15)
}

func writeNTTMatrixStatus(f *excelize.File, sheet, rs, status string) {
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

func writeNTTSingleDayRow(f *excelize.File, sheet string, in GenerationInput, day int, byDay map[int]models.DailyActivity, st *BuilderStyles) {
	row := 11 + (day - 1)
	rs := fmt.Sprintf("%d", row)
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}

	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	if day > daysInMonth {
		ApplyBlankPaddingRow(f, sheet, row, allCols, []string{"K"}, st)
		return
	}

	date := time.Date(in.Year, time.Month(in.Month), day, 0, 0, 0, 0, time.UTC)
	isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
	holiday := in.Holidays[day]
	act, hasAct := byDay[day]
	isHolidayOrWeekend := isWeekend || holiday != ""

	centerStyle := st.DataCenterStyle
	centerWrapStyle := st.DataCenterWrapStyle
	dateStyle := st.DateStyle
	timeStyle := st.TimeStyle
	if isHolidayOrWeekend {
		centerStyle = st.GreyCenterStyle
		centerWrapStyle = st.GreyCenterWrapStyle
		dateStyle = st.GreyDateStyle
		timeStyle = st.GreyTimeStyle
	}

	for _, col := range allCols {
		_ = f.SetCellStyle(sheet, col+rs, col+rs, centerStyle)
	}
	_ = f.SetCellStyle(sheet, "K"+rs, "K"+rs, centerWrapStyle)

	_ = f.SetCellValue(sheet, "A"+rs, date)
	_ = f.SetCellStyle(sheet, "A"+rs, "A"+rs, dateStyle)

	status := ""
	if hasAct {
		status = strings.ToUpper(strings.TrimSpace(act.Status))
		writeNTTActRow(f, sheet, rs, row, act, timeStyle)
	} else if isHolidayOrWeekend {
		writeNTTHolidayRow(f, sheet, rs, row, holiday)
	}

	writeNTTMatrixStatus(f, sheet, rs, status)
}

func writeNTTDailyRows(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	byDay := BuildByDayMap(in.Activities)
	for day := 1; day <= 31; day++ {
		writeNTTSingleDayRow(f, sheet, in, day, byDay, st)
	}
}

func writeNTTProjectSummary(f *excelize.File, sheet string, st *BuilderStyles) {
	const sumRow = "42"
	formulas := map[string]string{
		"E": `COUNTA(E11:E41)`,
		"F": `COUNTA(F11:F41)`,
		"G": `COUNTA(G11:G41)`,
		"H": `COUNTA(H11:H41)`,
		"I": `COUNTA(I11:I41)`,
		"J": `COUNTA(J11:J41)`,
	}
	for col, formula := range formulas {
		cell := col + sumRow
		_ = f.SetCellFormula(sheet, cell, formula)
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldCenterStyle)
	}

	_ = f.SetCellValue(sheet, "A44", "No.")
	_ = f.SetCellStyle(sheet, "A44", "A45", st.HeaderStyle)
	_ = f.MergeCell(sheet, "A44", "A45")

	_ = f.MergeCell(sheet, "B44", "D44")
	_ = f.SetCellValue(sheet, "B44", "Project Code / Name")
	_ = f.SetCellStyle(sheet, "B44", "D45", st.HeaderStyle)
	_ = f.MergeCell(sheet, "B44", "D45")

	_ = f.MergeCell(sheet, "E44", "J44")
	_ = f.SetCellValue(sheet, "E44", "STATUS  ATTENDANCE")
	_ = f.SetCellStyle(sheet, "E44", "J44", st.HeaderStyle)

	subProjHeaders := map[string]string{
		"E45": "Present\n(Days)", "F45": "Sick\n(Days)", "G45": "Business Trip\n(Days)",
		"H45": "Permit\n(Days)", "I45": "Vacation\n(Days)", "J45": "Not Working\n(Days)",
	}
	for cell, val := range subProjHeaders {
		_ = f.SetCellValue(sheet, cell, val)
		_ = f.SetCellStyle(sheet, cell, cell, st.HeaderGreyStyle)
	}

	_ = f.MergeCell(sheet, "K44", "K45")
	_ = f.SetCellValue(sheet, "K44", "Total Days")
	_ = f.SetCellStyle(sheet, "K44", "K45", st.HeaderStyle)

	_ = f.MergeCell(sheet, "L44", "L45")
	_ = f.SetCellValue(sheet, "L44", "Status (CR done/ in progress)")
	_ = f.SetCellStyle(sheet, "L44", "L45", st.HeaderStyle)

	_ = f.SetCellValue(sheet, "A46", 1)
	_ = f.SetCellStyle(sheet, "A46", "A46", st.DataCenterStyle)
	_ = f.MergeCell(sheet, "B46", "D46")
	_ = f.SetCellValue(sheet, "B46", "P24015 - BNI Direct")
	_ = f.SetCellStyle(sheet, "B46", "D46", st.DataLeftStyle)

	_ = f.SetCellFormula(sheet, "E46", "E42")
	_ = f.SetCellStyle(sheet, "E46", "E46", st.DataCenterStyle)
	_ = f.SetCellFormula(sheet, "F46", "F42")
	_ = f.SetCellStyle(sheet, "F46", "F46", st.DataCenterStyle)
	_ = f.SetCellFormula(sheet, "G46", "G42")
	_ = f.SetCellStyle(sheet, "G46", "G46", st.DataCenterStyle)
	_ = f.SetCellFormula(sheet, "H46", "H42")
	_ = f.SetCellStyle(sheet, "H46", "H46", st.DataCenterStyle)
	_ = f.SetCellFormula(sheet, "I46", "I42")
	_ = f.SetCellStyle(sheet, "I46", "I46", st.DataCenterStyle)
	_ = f.SetCellFormula(sheet, "J46", "J42")
	_ = f.SetCellStyle(sheet, "J46", "J46", st.DataCenterStyle)
	_ = f.SetCellFormula(sheet, "K46", "SUM(E46:J46)")
	_ = f.SetCellStyle(sheet, "K46", "K46", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "L46", "In Progress")
	_ = f.SetCellStyle(sheet, "L46", "L46", st.DataCenterStyle)

	_ = f.MergeCell(sheet, "A51", "D51")
	_ = f.SetCellValue(sheet, "A51", "TOTAL")
	_ = f.SetCellStyle(sheet, "A51", "D51", st.BoldCenterStyle)

	for _, col := range []string{"E", "F", "G", "H", "I", "J", "K"} {
		cell := fmt.Sprintf("%s51", col)
		_ = f.SetCellFormula(sheet, cell, fmt.Sprintf("SUM(%s46:%s50)", col, col))
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldCenterStyle)
	}
}

func writeNTTSignatures(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	tlName, dhName := ExtractApprovers(in.Overtimes)

	WriteSignaturesLayout(f, sheet, 53, 3, []SignatureParty{
		{StartCol: "B", EndCol: "E", Title: "TTD PEGAWAI,", Name: in.User.Name, DatePrefix: nttDatePrefix},
		{StartCol: "F", EndCol: "I", Title: "DIPERIKSA OLEH,", Name: tlName, DatePrefix: nttDatePrefix},
		{StartCol: "J", EndCol: "L", Title: "DISETUJUI OLEH,", Name: dhName, DatePrefix: nttDatePrefix},
	}, st)

	_ = f.SetRowHeight(sheet, 53, 20)
	_ = f.SetRowHeight(sheet, 54, 16)
	_ = f.SetRowHeight(sheet, 55, 16)
	_ = f.SetRowHeight(sheet, 56, 18)
	_ = f.SetRowHeight(sheet, 57, 22)
	_ = f.SetRowHeight(sheet, 58, 18)
}

// buildNTTWorkbook generates the NTT timesheet Excel document purely from code.
func buildNTTWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheet = "Timesheet"
	_ = f.SetSheetName(f.GetSheetName(0), sheet)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	setNTTColWidths(f, sheet)

	if len(assets.NTTLogo) > 0 {
		_ = addHeaderLogo(f, sheet, "N2", assets.NTTLogo, ".png", 0.6, 0.6)
	}

	writeNTTMetadata(f, sheet, in, st)
	writeNTTHeaders(f, sheet, st)
	writeNTTDailyRows(f, sheet, in, st)
	writeNTTProjectSummary(f, sheet, st)
	writeNTTSignatures(f, sheet, in, st)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
