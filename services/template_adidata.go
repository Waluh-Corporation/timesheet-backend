package services

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

const (
	defaultPosition = "Junior Programmer"
	defaultProject  = "BNI Direct"
	sheetTS         = "TIMESHEET"
)

// renderAdidataTemplate renders a monthly timesheet based on Adidata's master template.
func renderAdidataTemplate(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(assets.AdidataTemplate))
	if err != nil {
		return nil, fmt.Errorf("open adidata template: %w", err)
	}
	defer func() { _ = f.Close() }()

	// If sheet is named "Sheet1", rename to "TIMESHEET"
	for _, sh := range f.GetSheetList() {
		if strings.EqualFold(sh, "Sheet1") {
			_ = f.SetSheetName(sh, sheetTS)
			break
		}
	}

	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet Adidata", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	empID := ResolveEmployeeID(in.User)
	div := ""
	pos := defaultPosition
	if in.User != nil {
		div = in.User.Division
		if in.User.Position != "" {
			pos = in.User.Position
		}
	}
	if div == "" {
		div = "Wholesale Digital Delivery"
	}
	periodStr := fmt.Sprintf("%s %d", MonthNameIndonesian(in.Month), in.Year)

	// In Adidata master template, labels are B2:C2, B3:C3, B4, B5:C5, B6:C6 and values are D2, D3, D4, D5, D6.
	_ = f.SetCellValue(sheetTS, "D2", ": PT BANK NEGARA INDONESIA (PERSERO) Tbk")
	_ = f.SetCellValue(sheetTS, "D3", ": "+div)
	_ = f.SetCellValue(sheetTS, "D4", ": "+userName)
	_ = f.SetCellValue(sheetTS, "D5", ": "+empID)
	_ = f.SetCellValue(sheetTS, "D6", ": "+periodStr)

	byDay := BuildByDayMap(in.Activities)
	statusCounts := renderAdidataDailyRows(f, sheetTS, in, byDay, st)

	adidataSummary := map[string]SummaryFormulaItem{
		"F": {Formula: `COUNTIF(F10:F40,"P")`, Value: statusCounts["P"]},
		"G": {Formula: `COUNTIF(G10:G40,"S")`, Value: statusCounts["S"]},
		"H": {Formula: `COUNTIF(H10:H40,"BT")`, Value: statusCounts["BT"]},
		"I": {Formula: `COUNTIF(I10:I40,"PM")`, Value: statusCounts["PM"]},
		"J": {Formula: `COUNTIF(J10:J40,"V")`, Value: statusCounts["V"]},
		"K": {Formula: `COUNTIF(K10:K40,"x")`, Value: statusCounts["X"]},
	}
	WriteSummaryRowWithValues(f, sheetTS, "41", adidataSummary, st.BoldCenterStyle)

	tlName, dhName := ResolveApprovers(in)
	WriteSignatures(f, sheetTS, in, SignatureCells{
		UserCell:     "B44",
		TLCell:       "E44",
		DHCell:       "I44",
		UserDateCell: "B50",
		TLDateCell:   "E50",
		DHDateCell:   "I50",
		DatePrefix:   "DATE : ",
		UserBoxCell:  "B45",
		QRCell:       "L45",
	})
	_ = f.SetCellValue(sheetTS, "B49", "( "+pos+" )")

	// Render consolidated multi-row SPL sheet
	renderAdidataOvertimeSheet(f, in, empID, userName, tlName, dhName, st)

	return WriteWorkbookToBuffer(f, "adidata")
}

func renderAdidataDailyRows(
	f *excelize.File,
	sheet string,
	in GenerationInput,
	byDay map[int]models.DailyActivity,
	st *BuilderStyles,
) map[string]int {
	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	allCols := []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"}
	wrapCols := []string{"L", "M", "N", "O", "P", "Q", "R"}
	statusCounts := make(map[string]int)

	for day := 1; day <= 31; day++ {
		row := 9 + day // Row 10 is Day 1; Row 40 is Day 31
		rs := fmt.Sprintf("%d", row)

		// Merge Activity/Remark columns (L:N) for this row to match header L8:N9
		_ = f.MergeCell(sheet, "L"+rs, "N"+rs)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, allCols, wrapCols, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheet, "B", rs, allCols, wrapCols, dsc)
		_ = f.SetCellValue(sheet, "B"+rs, dsc.Date)

		WriteWorkingHoursRow(f, sheet, WorkingHoursConfig{
			StartCol:       "C",
			EndCol:         "D",
			TotalCol:       "E",
			RowStr:         rs,
			FormulaFmt:     "(D%s-C%s)*24",
			IsDecimalTotal: true,
		}, dsc)

		status := renderAdidataDayActivity(f, sheet, rs, row, dsc)
		writeAdidataAttendanceMatrixStatus(f, sheet, rs, status)
		if status != "" {
			statusCounts[status]++
		}
	}
	return statusCounts
}

func renderAdidataDayActivity(f *excelize.File, sheet, rs string, row int, dsc DayStyleContext) string {
	if dsc.HasActivity {
		status := strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
		_ = f.SetCellValue(sheet, "L"+rs, dsc.Activity.Activity)
		projName := dsc.Activity.GetProjectName()
		if projName == "" {
			projName = defaultProject
		}
		_ = f.SetCellValue(sheet, "O"+rs, projName)
		_ = f.SetCellValue(sheet, "P"+rs, dsc.Activity.GetProjectCode())
		_ = f.SetCellValue(sheet, "Q"+rs, dsc.Activity.GetAppImpacted())
		_ = f.SetCellValue(sheet, "R"+rs, "")
		h := calculateRowHeight(dsc.Activity.Activity, projName, dsc.Activity.GetProjectCode(), dsc.Activity.GetAppImpacted(), "", "")
		_ = f.SetRowHeight(sheet, row, h)
		return status
	}
	if dsc.IsHolidayOrWeekend {
		WriteHolidayRemarkRow(f, sheet, "L", rs, row, dsc.Holiday)
	}
	return ""
}

// resolveUserPositionAndDept extracts employee position and formatted department/division.
func resolveUserPositionAndDept(user *models.User) (pos, deptDiv string) {
	pos = defaultPosition
	deptDiv = "WCH / WDL"
	if user == nil {
		return pos, deptDiv
	}
	if user.Position != "" {
		pos = user.Position
	}
	if user.Department != "" {
		deptDiv = user.Department
		if user.Division != "" {
			deptDiv += " / " + user.Division
		}
	} else if user.Division != "" {
		deptDiv = user.Division
	}
	return pos, deptDiv
}

func deleteSPLTemplateSheet(f *excelize.File) {
	if idx, _ := f.GetSheetIndex("SPL_TEMPLATE"); idx != -1 {
		_ = f.DeleteSheet("SPL_TEMPLATE")
	}
}

func writeAdidataSPLHeader(f *excelize.File, splSheet, empID, userName, deptDiv, pos string) {
	_ = f.SetCellValue(splSheet, "C7", "NPP                                          : "+empID)
	_ = f.SetCellValue(splSheet, "C8", "NAMA                                     : "+userName)
	_ = f.SetCellValue(splSheet, "C9", "DEPARTEMENT / DIVISI     : "+deptDiv)
	_ = f.SetCellValue(splSheet, "C10", "POSISI                                     : "+pos)
}

func expandAdidataSPLRows(f *excelize.File, splSheet string, count int) {
	if count <= 3 {
		return
	}
	extraRows := count - 3
	_ = f.InsertRows(splSheet, 18, extraRows)
	cols := []string{"C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"}
	for r := 18; r < 18+extraRows; r++ {
		rStr := fmt.Sprintf("%d", r)
		_ = f.MergeCell(splSheet, "D"+rStr, "E"+rStr)
		_ = f.MergeCell(splSheet, "F"+rStr, "G"+rStr)
		_ = f.MergeCell(splSheet, "I"+rStr, "O"+rStr)
		_ = f.SetRowHeight(splSheet, r, 21)
		for _, col := range cols {
			styleID, _ := f.GetCellStyle(splSheet, col+"15")
			_ = f.SetCellStyle(splSheet, col+rStr, col+rStr, styleID)
		}
	}
}

func writeAdidataOvertimeRows(f *excelize.File, splSheet string, overtimes []models.OvertimeEntry) float64 {
	var totalHours float64
	for i, ot := range overtimes {
		rStr := fmt.Sprintf("%d", 15+i)
		dur := CalculateTimeDurationFrac(ot.StartTime, ot.EndTime) * 24
		totalHours += dur

		_ = f.SetCellValue(splSheet, "C"+rStr, i+1)
		_ = f.SetCellValue(splSheet, "D"+rStr, ot.Date.Format("02 January 2006"))
		_ = f.SetCellValue(splSheet, "F"+rStr, fmt.Sprintf("%s - %s", ot.StartTime, ot.EndTime))
		_ = f.SetCellValue(splSheet, "H"+rStr, dur)
		_ = f.SetCellValue(splSheet, "I"+rStr, ot.TaskDescription)
	}
	return totalHours
}

func resolveOvertimeApprovers(overtimes []models.OvertimeEntry, defaultTL, defaultDH string) (tl, dh string) {
	otTL, otDH := ExtractApprovers(overtimes)
	if otTL == "" {
		otTL = defaultTL
	}
	if otDH == "" {
		otDH = defaultDH
	}
	return otTL, otDH
}

func writeAdidataSPLSignaturesMulti(f *excelize.File, splSheet string, totalRow int, userName, pos, tlName, dhName string) {
	sigNameRow := fmt.Sprintf("%d", totalRow+8)
	sigPosRow := fmt.Sprintf("%d", totalRow+9)

	if userName != "" {
		_ = f.SetCellValue(splSheet, "B"+sigNameRow, "( "+userName+" )")
	}
	_ = f.SetCellValue(splSheet, "B"+sigPosRow, pos)

	if tlName != "" {
		_ = f.SetCellValue(splSheet, "G"+sigNameRow, "( "+tlName+" )")
	}
	_ = f.SetCellValue(splSheet, "G"+sigPosRow, "TEAM LEADER")

	if dhName != "" {
		_ = f.SetCellValue(splSheet, "M"+sigNameRow, "( "+dhName+" )")
	}
	_ = f.SetCellValue(splSheet, "M"+sigPosRow, "DEPARTMENT HEAD")
}

// renderAdidataOvertimeSheet renders a single consolidated "SPL" sheet containing all overtime activities.
func renderAdidataOvertimeSheet(
	f *excelize.File,
	in GenerationInput,
	empID, userName, tlName, dhName string,
	st *BuilderStyles,
) {
	if len(in.Overtimes) == 0 {
		deleteSPLTemplateSheet(f)
		return
	}

	pos, deptDiv := resolveUserPositionAndDept(in.User)
	splSheet := "SPL"
	splTmplIdx, _ := f.GetSheetIndex("SPL_TEMPLATE")
	if splTmplIdx == -1 {
		_, _ = f.NewSheet(splSheet)
		writeAdidataMultiSPL(f, splSheet, in, empID, st)
		return
	}

	_, _ = f.NewSheet(splSheet)
	targetIdx, _ := f.GetSheetIndex(splSheet)
	_ = f.CopySheet(splTmplIdx, targetIdx)

	writeAdidataSPLHeader(f, splSheet, empID, userName, deptDiv, pos)
	expandAdidataSPLRows(f, splSheet, len(in.Overtimes))
	totalHours := writeAdidataOvertimeRows(f, splSheet, in.Overtimes)

	lastDataRow := 17
	if len(in.Overtimes) > 3 {
		lastDataRow = 14 + len(in.Overtimes)
	}
	totalRow := lastDataRow + 1
	totalRowStr := fmt.Sprintf("%d", totalRow)
	_ = f.SetCellValue(splSheet, "H"+totalRowStr, totalHours)
	_ = f.SetCellFormula(splSheet, "H"+totalRowStr, fmt.Sprintf("SUM(H15:H%d)", lastDataRow))

	otTL, otDH := resolveOvertimeApprovers(in.Overtimes, tlName, dhName)
	writeAdidataSPLSignaturesMulti(f, splSheet, totalRow, userName, pos, otTL, otDH)
	deleteSPLTemplateSheet(f)
}

// writeAdidataSingleSPL provides a bridge to writeAdidataMultiSPL for single-entry overtime calls.
func writeAdidataSingleSPL(f *excelize.File, splSheet string, in GenerationInput, ot models.OvertimeEntry, empID string, st *BuilderStyles) {
	inCopy := in
	inCopy.Overtimes = []models.OvertimeEntry{ot}
	writeAdidataMultiSPL(f, splSheet, inCopy, empID, st)
}

// writeAdidataMultiSPL builds the Adidata SPL sheet from scratch when SPL_TEMPLATE is not present.
func writeAdidataMultiSPL(f *excelize.File, splSheet string, in GenerationInput, empID string, st *BuilderStyles) {
	_ = f.SetColWidth(splSheet, "A", "B", 3)
	_ = f.SetColWidth(splSheet, "C", "C", 5.33)
	_ = f.SetColWidth(splSheet, "D", "E", 10)
	_ = f.SetColWidth(splSheet, "F", "G", 9.5)
	_ = f.SetColWidth(splSheet, "H", "H", 12.89)
	_ = f.SetColWidth(splSheet, "I", "O", 10)

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	_ = f.MergeCell(splSheet, "C4", "P5")
	_ = f.SetCellValue(splSheet, "C4", "SURAT PERINTAH LEMBUR")
	_ = f.SetCellStyle(splSheet, "C4", "P5", titleStyle)

	pos, deptDiv := resolveUserPositionAndDept(in.User)
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}

	_ = f.MergeCell(splSheet, "C7", "O7")
	_ = f.SetCellValue(splSheet, "C7", "NPP                                          : "+empID)
	_ = f.MergeCell(splSheet, "C8", "P8")
	_ = f.SetCellValue(splSheet, "C8", "NAMA                                     : "+userName)
	_ = f.MergeCell(splSheet, "C9", "P9")
	_ = f.SetCellValue(splSheet, "C9", "DEPARTEMENT / DIVISI     : "+deptDiv)
	_ = f.MergeCell(splSheet, "C10", "P10")
	_ = f.SetCellValue(splSheet, "C10", "POSISI                                     : "+pos)

	for r := 7; r <= 10; r++ {
		_ = f.SetCellStyle(splSheet, fmt.Sprintf("C%d", r), fmt.Sprintf("C%d", r), st.MetaLabelStyle)
	}

	_ = f.MergeCell(splSheet, "C12", "P12")
	_ = f.SetCellValue(splSheet, "C12", "TUGAS YANG DIKERJAKAN")
	_ = f.SetCellStyle(splSheet, "C12", "P12", st.HeaderStyle)

	_ = f.SetCellValue(splSheet, "C14", "No")
	_ = f.SetCellStyle(splSheet, "C14", "C14", st.HeaderStyle)
	_ = f.MergeCell(splSheet, "D14", "E14")
	_ = f.SetCellValue(splSheet, "D14", "Tanggal")
	_ = f.SetCellStyle(splSheet, "D14", "E14", st.HeaderStyle)
	_ = f.MergeCell(splSheet, "F14", "G14")
	_ = f.SetCellValue(splSheet, "F14", "Jam")
	_ = f.SetCellStyle(splSheet, "F14", "G14", st.HeaderStyle)
	_ = f.SetCellValue(splSheet, "H14", "Total Jam ")
	_ = f.SetCellStyle(splSheet, "H14", "H14", st.HeaderStyle)
	_ = f.MergeCell(splSheet, "I14", "O14")
	_ = f.SetCellValue(splSheet, "I14", "Tugas yang di Kerjakan")
	_ = f.SetCellStyle(splSheet, "I14", "O14", st.HeaderStyle)

	var totalHours float64
	slotCount := len(in.Overtimes)
	if slotCount < 3 {
		slotCount = 3
	}

	for i := 0; i < slotCount; i++ {
		r := 15 + i
		rStr := fmt.Sprintf("%d", r)
		_ = f.MergeCell(splSheet, "D"+rStr, "E"+rStr)
		_ = f.MergeCell(splSheet, "F"+rStr, "G"+rStr)
		_ = f.MergeCell(splSheet, "I"+rStr, "O"+rStr)
		_ = f.SetRowHeight(splSheet, r, 21)

		_ = f.SetCellStyle(splSheet, "C"+rStr, "C"+rStr, st.DataCenterStyle)
		styleMergedRange(f, splSheet, "D"+rStr, "E"+rStr, st.DataCenterStyle)
		styleMergedRange(f, splSheet, "F"+rStr, "G"+rStr, st.DataCenterStyle)
		_ = f.SetCellStyle(splSheet, "H"+rStr, "H"+rStr, st.DataCenterStyle)
		styleMergedRange(f, splSheet, "I"+rStr, "O"+rStr, st.DataCenterWrapStyle)

		if i < len(in.Overtimes) {
			ot := in.Overtimes[i]
			dur := CalculateTimeDurationFrac(ot.StartTime, ot.EndTime) * 24
			totalHours += dur
			_ = f.SetCellValue(splSheet, "C"+rStr, i+1)
			_ = f.SetCellValue(splSheet, "D"+rStr, ot.Date.Format("02 January 2006"))
			_ = f.SetCellValue(splSheet, "F"+rStr, fmt.Sprintf("%s - %s", ot.StartTime, ot.EndTime))
			_ = f.SetCellValue(splSheet, "H"+rStr, dur)
			_ = f.SetCellValue(splSheet, "I"+rStr, ot.TaskDescription)
		}
	}

	totalRow := 15 + slotCount
	totalRowStr := fmt.Sprintf("%d", totalRow)
	_ = f.MergeCell(splSheet, "C"+totalRowStr, "G"+totalRowStr)
	_ = f.SetCellValue(splSheet, "C"+totalRowStr, "Total Keseluruhan Jam Lembur")
	styleMergedRange(f, splSheet, "C"+totalRowStr, "G"+totalRowStr, st.BoldCenterStyle)

	sumFormula := fmt.Sprintf("SUM(H15:H%d)", 14+slotCount)
	_ = f.SetCellValue(splSheet, "H"+totalRowStr, totalHours)
	_ = f.SetCellFormula(splSheet, "H"+totalRowStr, sumFormula)
	_ = f.SetCellStyle(splSheet, "H"+totalRowStr, "H"+totalRowStr, st.BoldCenterStyle)

	tlName, dhName := ResolveApprovers(in)
	sigNameRow := fmt.Sprintf("%d", totalRow+8)
	sigPosRow := fmt.Sprintf("%d", totalRow+9)

	_ = f.MergeCell(splSheet, "B"+sigNameRow, "E"+sigNameRow)
	_ = f.MergeCell(splSheet, "B"+sigPosRow, "E"+sigPosRow)
	styleMergedRange(f, splSheet, "B"+sigNameRow, "E"+sigNameRow, st.BoldCenterStyle)
	styleMergedRange(f, splSheet, "B"+sigPosRow, "E"+sigPosRow, st.DataCenterStyle)
	_ = f.SetCellValue(splSheet, "B"+sigNameRow, "( "+userName+" )")
	_ = f.SetCellValue(splSheet, "B"+sigPosRow, pos)

	_ = f.MergeCell(splSheet, "G"+sigNameRow, "K"+sigNameRow)
	_ = f.MergeCell(splSheet, "G"+sigPosRow, "K"+sigPosRow)
	styleMergedRange(f, splSheet, "G"+sigNameRow, "K"+sigNameRow, st.BoldCenterStyle)
	styleMergedRange(f, splSheet, "G"+sigPosRow, "K"+sigPosRow, st.DataCenterStyle)
	_ = f.SetCellValue(splSheet, "G"+sigNameRow, "( "+tlName+" )")
	_ = f.SetCellValue(splSheet, "G"+sigPosRow, "TEAM LEADER")

	_ = f.MergeCell(splSheet, "M"+sigNameRow, "O"+sigNameRow)
	_ = f.MergeCell(splSheet, "M"+sigPosRow, "O"+sigPosRow)
	styleMergedRange(f, splSheet, "M"+sigNameRow, "O"+sigNameRow, st.BoldCenterStyle)
	styleMergedRange(f, splSheet, "M"+sigPosRow, "O"+sigPosRow, st.DataCenterStyle)
	_ = f.SetCellValue(splSheet, "M"+sigNameRow, "( "+dhName+" )")
	_ = f.SetCellValue(splSheet, "M"+sigPosRow, "DEPARTMENT HEAD")
}

// writeAdidataAttendanceMatrixStatus writes the 6-column matrix attendance status ("F" through "K") for Adidata.
func writeAdidataAttendanceMatrixStatus(f *excelize.File, sheet, rs, status string) {
	statusCol := map[string]string{
		"P":  "F",
		"S":  "G",
		"BT": "H",
		"PM": "I",
		"V":  "J",
		"X":  "K",
	}
	statusMark := map[string]string{
		"P":  "P",
		"S":  "S",
		"BT": "BT",
		"PM": "PM",
		"V":  "V",
		"X":  "x",
	}
	matrixCols := []string{"F", "G", "H", "I", "J", "K"}

	for _, col := range matrixCols {
		_ = f.SetCellValue(sheet, col+rs, "")
	}
	if col, ok := statusCol[status]; ok {
		_ = f.SetCellValue(sheet, col+rs, statusMark[status])
	}
}
