package services

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

// buildAdidataWorkbook generates the Adidata timesheet Excel document purely from code,
// including dynamic SPL sheets if overtime entries are present.
func buildAdidataWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheetTS = "TIMESHEET"
	_ = f.SetSheetName(f.GetSheetName(0), sheetTS)
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet Adidata", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	setAdidataColWidths(f, sheetTS)
	if len(assets.AdidataLogo) > 0 {
		_ = addHeaderLogo(f, sheetTS, "N2", assets.AdidataLogo, ".png", 0.7, 0.7)
	}

	empID := writeAdidataMetadata(f, sheetTS, in, st)
	writeAdidataHeaders(f, sheetTS, st)
	writeAdidataDailyRows(f, sheetTS, in, st)
	writeAdidataSummaryRow(f, sheetTS, st)
	writeAdidataSignatures(f, sheetTS, in, st)
	writeAdidataSPLSheets(f, in, empID, st)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func setAdidataColWidths(f *excelize.File, sheet string) {
	widths := map[string]float64{
		"A": 12, "B": 8, "C": 8, "D": 11,
		"E": 8, "F": 8, "G": 13, "H": 8, "I": 8, "J": 12,
		"K": 35, "L": 18, "M": 14, "N": 18, "O": 14,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}
}

func writeAdidataMetadata(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) string {
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
		div = "BDD / WDL"
	}
	periodStr := fmt.Sprintf("%s %d", indonesianMonth(in.Month), in.Year)

	setMeta("A1", "NAME of PROJECT", "C1", ": PT BANK NEGARA INDONESIA (PERSERO) Tbk")
	setMeta("A2", "UNIT/DIVISION", "C2", ": "+div)
	setMeta("A3", "NAME", "C3", ": "+in.User.Name)
	setMeta("A4", "NPP", "C4", ": "+empID)
	setMeta("A5", "PERIODE", "C5", ": "+periodStr)

	_ = f.SetCellValue(sheet, "G6", "P = Present; S = Sick; V = Vacation; BT = Business Trip; PM = Permit; X = Not Working Anymore")
	return empID
}

func writeAdidataHeaders(f *excelize.File, sheet string, st *BuilderStyles) {
	headers := []HeaderColumn{
		{From: "A7", To: "A8", Val: "DATE"},
		{From: "B7", To: "C7", Val: "WORKING HOUR"},
		{From: "D7", To: "D8", Val: "TOTAL HOUR"},
		{From: "E7", To: "J7", Val: "STATUS ATTENDANCE "},
		{From: "K7", To: "K8", Val: "ACTIVITY / REMARK"},
		{From: "L7", To: "L8", Val: "NAMA PROJECT"},
		{From: "M7", To: "M8", Val: "PROJECT CODE"},
		{From: "N7", To: "N8", Val: "APLIKASI TERDAMPAK"},
		{From: "O7", To: "O8", Val: "AIP FITUR"},
	}
	subHeaders := map[string]string{
		"B8": "START", "C8": "END",
		"E8": "Present", "F8": "Sick ", "G8": "Business Trip",
		"H8": "Permit", "I8": "Vacation", "J8": "Not Working",
	}
	WriteTableHeaders(f, sheet, headers, subHeaders, st)
}

func writeAdidataActRow(f *excelize.File, sheet, rs string, row int, act models.DailyActivity, timeStyle, decimalStyle int) {
	hasStart, hasEnd := WriteTimeCells(f, sheet, "B"+rs, "C"+rs, act.StartTime, act.EndTime, timeStyle)
	if hasStart && hasEnd {
		_ = f.SetCellFormula(sheet, "D"+rs, fmt.Sprintf("(C%s-B%s)*24", rs, rs))
		_ = f.SetCellStyle(sheet, "D"+rs, "D"+rs, decimalStyle)
	}

	_ = f.SetCellValue(sheet, "K"+rs, act.Activity)
	_ = f.SetCellValue(sheet, "L"+rs, act.GetProjectName())
	_ = f.SetCellValue(sheet, "M"+rs, act.GetProjectCode())
	_ = f.SetCellValue(sheet, "N"+rs, act.GetAppImpacted())

	h := calculateRowHeight(act.Activity, act.GetProjectName(), act.GetProjectCode(), act.GetAppImpacted(), "", "")
	_ = f.SetRowHeight(sheet, row, h)
}

func writeAdidataSingleDayRow(f *excelize.File, sheet string, in GenerationInput, day int, byDay map[int]models.DailyActivity, st *BuilderStyles) {
	row := 9 + (day - 1)
	rs := fmt.Sprintf("%d", row)
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"}

	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	if day > daysInMonth {
		ApplyBlankPaddingRow(f, sheet, row, allCols, []string{"K"}, st)
		return
	}

	dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
	ApplyDayRowStyles(f, sheet, "A", rs, allCols, []string{"K"}, dsc)

	status := ""
	if dsc.HasActivity {
		status = strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
		writeAdidataActRow(f, sheet, rs, row, dsc.Activity, dsc.TimeStyle, dsc.DecimalStyle)
	} else if dsc.IsHolidayOrWeekend {
		WriteHolidayRemarkRow(f, sheet, "K", rs, row, dsc.Holiday)
	}

	WriteAttendanceMatrixStatus(f, sheet, rs, status)
}

func writeAdidataDailyRows(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	byDay := BuildByDayMap(in.Activities)
	for day := 1; day <= 31; day++ {
		writeAdidataSingleDayRow(f, sheet, in, day, byDay, st)
	}
}

func writeAdidataSummaryRow(f *excelize.File, sheet string, st *BuilderStyles) {
	const sumRow = "40"
	formulas := map[string]string{
		"E": `COUNTIF(E9:E39,"P")`,
		"F": `COUNTIF(F9:F39,"S")`,
		"G": `COUNTIF(G9:G39,"BT")`,
		"H": `COUNTIF(H9:H39,"PM")`,
		"I": `COUNTIF(I9:I39,"V")`,
		"J": `COUNTIF(J9:J39,"x")`,
	}
	for col, formula := range formulas {
		cell := col + sumRow
		_ = f.SetCellFormula(sheet, cell, formula)
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldCenterStyle)
	}
}

func writeAdidataSignatures(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	// Headers (Row 42)
	styleMergedRange(f, sheet, "A42", "C42", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "A42", "TTD PEGAWAI,")
	styleMergedRange(f, sheet, "D42", "F42", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "D42", "DIPERIKSA OLEH,")
	styleMergedRange(f, sheet, "G42", "J42", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "G42", "DISETUJUI OLEH,")

	// Empty Signature Spaces with Borders (Rows 43-47)
	styleMergedRange(f, sheet, "A43", "C47", st.DataCenterStyle)
	styleMergedRange(f, sheet, "D43", "F47", st.DataCenterStyle)
	styleMergedRange(f, sheet, "G43", "J47", st.DataCenterStyle)

	// Names (Row 48)
	styleMergedRange(f, sheet, "A48", "C48", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "A48", "( "+in.User.Name+" )")

	tlName, dhName := ExtractApprovers(in.Overtimes)
	tlCell, dhCell := "", ""
	if tlName != "" {
		tlCell = "( " + tlName + " )"
	}
	if dhName != "" {
		dhCell = "( " + dhName + " )"
	}

	styleMergedRange(f, sheet, "D48", "F48", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "D48", tlCell)
	styleMergedRange(f, sheet, "G48", "J48", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "G48", dhCell)

	// Positions (Row 49)
	posTitle := in.User.Position
	if posTitle == "" {
		posTitle = "Junior Programmer"
	}
	styleMergedRange(f, sheet, "A49", "C49", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "A49", posTitle)
	styleMergedRange(f, sheet, "D49", "F49", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "D49", "TEAM LEADER")
	styleMergedRange(f, sheet, "G49", "J49", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "G49", "DEPARTEMEN HEAD")
}

func writeAdidataSingleSPL(f *excelize.File, splSheet string, in GenerationInput, ot models.OvertimeEntry, empID string, st *BuilderStyles) {
	_ = f.SetColWidth(splSheet, "A", "A", 4)
	_ = f.SetColWidth(splSheet, "B", "B", 4)
	_ = f.SetColWidth(splSheet, "C", "C", 6)
	_ = f.SetColWidth(splSheet, "D", "E", 12)
	_ = f.SetColWidth(splSheet, "F", "G", 10)
	_ = f.SetColWidth(splSheet, "H", "O", 10)

	if len(assets.AdidataLogo) > 0 {
		_ = addHeaderLogo(f, splSheet, "L3", assets.AdidataLogo, ".png", 0.6, 0.6)
	}

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Family: "Calibri"},
	})
	_ = f.SetCellValue(splSheet, "B2", "SURAT PERINTAH LEMBUR")
	_ = f.SetCellStyle(splSheet, "B2", "B2", titleStyle)

	pos := in.User.Position
	if pos == "" {
		pos = "Junior Programmer"
	}
	deptDiv := in.User.Department
	if deptDiv == "" {
		deptDiv = "WCH / WDL"
	} else if in.User.Division != "" {
		deptDiv = deptDiv + " / " + in.User.Division
	}

	_ = f.SetCellValue(splSheet, "C4", "NPP")
	_ = f.SetCellValue(splSheet, "E4", ": "+empID)
	_ = f.SetCellValue(splSheet, "C5", "NAMA")
	_ = f.SetCellValue(splSheet, "E5", ": "+in.User.Name)
	_ = f.SetCellValue(splSheet, "C6", "DEPARTEMENT / DIVISI")
	_ = f.SetCellValue(splSheet, "E6", ": "+deptDiv)
	_ = f.SetCellValue(splSheet, "C7", "POSISI")
	_ = f.SetCellValue(splSheet, "E7", ": "+pos)

	for r := 4; r <= 7; r++ {
		_ = f.SetCellStyle(splSheet, fmt.Sprintf("C%d", r), fmt.Sprintf("C%d", r), st.MetaLabelStyle)
		_ = f.SetCellStyle(splSheet, fmt.Sprintf("E%d", r), fmt.Sprintf("E%d", r), st.MetaValueStyle)
	}

	_ = f.MergeCell(splSheet, "C9", "O9")
	_ = f.SetCellValue(splSheet, "C9", "TUGAS YANG DIKERJAKAN")
	_ = f.SetCellStyle(splSheet, "C9", "O9", st.HeaderStyle)

	_ = f.SetCellValue(splSheet, "C11", "No")
	_ = f.SetCellStyle(splSheet, "C11", "C11", st.HeaderStyle)
	_ = f.MergeCell(splSheet, "D11", "E11")
	_ = f.SetCellValue(splSheet, "D11", "Tanggal")
	_ = f.SetCellStyle(splSheet, "D11", "E11", st.HeaderStyle)
	_ = f.MergeCell(splSheet, "F11", "G11")
	_ = f.SetCellValue(splSheet, "F11", "Jam")
	_ = f.SetCellStyle(splSheet, "F11", "G11", st.HeaderStyle)
	_ = f.MergeCell(splSheet, "H11", "O11")
	_ = f.SetCellValue(splSheet, "H11", "Tugas yang di Kerjakan")
	_ = f.SetCellStyle(splSheet, "H11", "O11", st.HeaderStyle)

	_ = f.SetCellValue(splSheet, "C12", "1")
	_ = f.SetCellStyle(splSheet, "C12", "C12", st.DataCenterStyle)
	_ = f.MergeCell(splSheet, "D12", "E12")
	_ = f.SetCellValue(splSheet, "D12", ot.Date.Format("02 January 2006"))
	_ = f.SetCellStyle(splSheet, "D12", "E12", st.DataCenterStyle)
	_ = f.MergeCell(splSheet, "F12", "G12")
	_ = f.SetCellValue(splSheet, "F12", fmt.Sprintf("%s - %s", ot.StartTime, ot.EndTime))
	_ = f.SetCellStyle(splSheet, "F12", "G12", st.DataCenterStyle)
	_ = f.MergeCell(splSheet, "H12", "O12")
	_ = f.SetCellValue(splSheet, "H12", ot.TaskDescription)
	_ = f.SetCellStyle(splSheet, "H12", "O12", st.DataCenterWrapStyle)

	writeAdidataSPLSignatures(f, splSheet, in, ot, pos, st)
}

func writeAdidataSPLSignatures(f *excelize.File, splSheet string, in GenerationInput, ot models.OvertimeEntry, pos string, st *BuilderStyles) {
	_ = f.MergeCell(splSheet, "C21", "E21")
	_ = f.SetCellValue(splSheet, "C21", "( "+in.User.Name+" )")
	_ = f.SetCellStyle(splSheet, "C21", "E21", st.BoldCenterStyle)
	_ = f.MergeCell(splSheet, "C22", "E22")
	_ = f.SetCellValue(splSheet, "C22", pos)
	_ = f.SetCellStyle(splSheet, "C22", "E22", st.DataCenterStyle)

	tlLabel := ""
	if ot.TeamLeader != nil && ot.TeamLeader.Name != "" {
		tlLabel = "( " + ot.TeamLeader.Name + " )"
	}
	_ = f.MergeCell(splSheet, "G21", "J21")
	_ = f.SetCellValue(splSheet, "G21", tlLabel)
	_ = f.SetCellStyle(splSheet, "G21", "J21", st.BoldCenterStyle)
	_ = f.MergeCell(splSheet, "G22", "J22")
	_ = f.SetCellValue(splSheet, "G22", "TEAM LEADER")
	_ = f.SetCellStyle(splSheet, "G22", "J22", st.DataCenterStyle)

	dhLabel := ""
	if ot.DepartmentHead != nil && ot.DepartmentHead.Name != "" {
		dhLabel = "( " + ot.DepartmentHead.Name + " )"
	}
	_ = f.MergeCell(splSheet, "L21", "N21")
	_ = f.SetCellValue(splSheet, "L21", dhLabel)
	_ = f.SetCellStyle(splSheet, "L21", "N21", st.BoldCenterStyle)
	_ = f.MergeCell(splSheet, "L22", "N22")
	_ = f.SetCellValue(splSheet, "L22", "DEPARTMENT HEAD")
	_ = f.SetCellStyle(splSheet, "L22", "N22", st.DataCenterStyle)
}

func writeAdidataSPLSheets(f *excelize.File, in GenerationInput, empID string, st *BuilderStyles) {
	for i, ot := range in.Overtimes {
		splSheet := fmt.Sprintf("SPL-%d", i+1)
		_, _ = f.NewSheet(splSheet)
		writeAdidataSingleSPL(f, splSheet, in, ot, empID, st)
	}
}
