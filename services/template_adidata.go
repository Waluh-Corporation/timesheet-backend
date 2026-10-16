package services

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

const defaultJuniorProgrammer = "Junior Programmer"

// renderAdidataTemplate renders a monthly timesheet based on Adidata's master template.
func renderAdidataTemplate(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(assets.AdidataTemplate))
	if err != nil {
		return nil, fmt.Errorf("open adidata template: %w", err)
	}
	defer func() { _ = f.Close() }()

	const sheetTS = "TIMESHEET"
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
	pos := defaultJuniorProgrammer
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

	writeAdidataMetadata(f, sheetTS, userName, empID, div, periodStr)

	byDay := BuildByDayMap(in.Activities)
	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	statusCounts := writeAdidataDailyRows(f, sheetTS, in, byDay, daysInMonth, st)

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
	WriteSignatures(f, sheetTS, in, SignatureCellsConfig{
		UserCell:     "B44",
		TLCell:       "E44",
		DHCell:       "I44",
		UserDateCell: "B50",
		TLDateCell:   "E50",
		DHDateCell:   "I50",
		DatePrefix:   "DATE : ",
	})
	_ = f.SetCellValue(sheetTS, "B49", "( "+pos+" )")

	// Dynamic Overtime SPL Sheets
	writeAdidataOvertimeSheets(f, in, empID, userName, tlName, dhName, st)

	// Always delete SPL_TEMPLATE if it was present so only generated SPL sheets remain
	if idx, _ := f.GetSheetIndex("SPL_TEMPLATE"); idx != -1 {
		_ = f.DeleteSheet("SPL_TEMPLATE")
	}

	return WriteWorkbookToBuffer(f, "adidata")
}

func writeAdidataMetadata(f *excelize.File, sheetTS, userName, empID, div, periodStr string) {
	_ = f.SetCellValue(sheetTS, "D2", ": PT BANK NEGARA INDONESIA (PERSERO) Tbk")
	_ = f.SetCellValue(sheetTS, "D3", ": "+div)
	_ = f.SetCellValue(sheetTS, "D4", ": "+userName)
	_ = f.SetCellValue(sheetTS, "D5", ": "+empID)
	_ = f.SetCellValue(sheetTS, "D6", ": "+periodStr)
}

func writeAdidataDailyRows(f *excelize.File, sheetTS string, in GenerationInput, byDay map[int]models.DailyActivity, daysInMonth int, st *BuilderStyles) map[string]int {
	allCols := []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"}
	wrapCols := []string{"L", "O", "Q", "R"}
	statusCounts := make(map[string]int)

	for day := 1; day <= 31; day++ {
		row := 9 + day // Row 10 is Day 1; Row 40 is Day 31
		rs := fmt.Sprintf("%d", row)

		// Merge Activity/Remark columns (L:N) for this row to match header L8:N9
		_ = f.MergeCell(sheetTS, "L"+rs, "N"+rs)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheetTS, row, allCols, wrapCols, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheetTS, "B", rs, allCols, wrapCols, dsc)
		_ = f.SetCellValue(sheetTS, "B"+rs, dsc.Date)

		WriteWorkingHoursRow(f, sheetTS, WorkingHoursConfig{
			StartCol:       "C",
			EndCol:         "D",
			TotalCol:       "E",
			RowStr:         rs,
			FormulaFmt:     "(D%s-C%s)*24",
			IsDecimalTotal: true,
		}, dsc)

		status := ""
		if dsc.HasActivity {
			status = strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
			_ = f.SetCellValue(sheetTS, "L"+rs, dsc.Activity.Activity)
			projName := dsc.Activity.GetProjectName()
			if projName == "" {
				projName = "BNI Direct"
			}
			_ = f.SetCellValue(sheetTS, "O"+rs, projName)
			_ = f.SetCellValue(sheetTS, "P"+rs, dsc.Activity.GetProjectCode())
			_ = f.SetCellValue(sheetTS, "Q"+rs, dsc.Activity.GetAppImpacted())
			_ = f.SetCellValue(sheetTS, "R"+rs, "")
			h := calculateRowHeight(dsc.Activity.Activity, projName, dsc.Activity.GetProjectCode(), dsc.Activity.GetAppImpacted(), "", "")
			_ = f.SetRowHeight(sheetTS, row, h)
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheetTS, "L", rs, row, dsc.Holiday)
		}

		writeAdidataAttendanceMatrixStatus(f, sheetTS, rs, status)
		if status != "" {
			statusCounts[status]++
		}
	}
	return statusCounts
}

type adidataSPLMeta struct {
	EmpID    string
	UserName string
	DeptDiv  string
	Pos      string
	TLName   string
	DHName   string
}

func resolveAdidataDeptDiv(u *models.User) string {
	if u == nil || u.Department == "" {
		return "WCH / WDL"
	}
	if u.Division != "" {
		return u.Department + " / " + u.Division
	}
	return u.Department
}

func writeAdidataOvertimeSheets(f *excelize.File, in GenerationInput, empID, userName, tlName, dhName string, st *BuilderStyles) {
	if len(in.Overtimes) == 0 {
		return
	}
	pos := in.User.Position
	if pos == "" {
		pos = defaultJuniorProgrammer
	}
	meta := adidataSPLMeta{
		EmpID:    empID,
		UserName: userName,
		DeptDiv:  resolveAdidataDeptDiv(in.User),
		Pos:      pos,
		TLName:   tlName,
		DHName:   dhName,
	}

	splTmplIdx, _ := f.GetSheetIndex("SPL_TEMPLATE")
	for i, ot := range in.Overtimes {
		splSheet := fmt.Sprintf("SPL-%d", i+1)
		createOrCopyAdidataSPLSheet(f, splSheet, splTmplIdx, in, ot, empID, st)
		populateAdidataSPLSheet(f, splSheet, ot, meta)
	}
}

func createOrCopyAdidataSPLSheet(f *excelize.File, splSheet string, tmplIdx int, in GenerationInput, ot models.OvertimeEntry, empID string, st *BuilderStyles) {
	_, _ = f.NewSheet(splSheet)
	if tmplIdx != -1 {
		targetIdx, _ := f.GetSheetIndex(splSheet)
		_ = f.CopySheet(tmplIdx, targetIdx)
		return
	}
	writeAdidataSingleSPL(f, splSheet, in, ot, empID, st)
}

func populateAdidataSPLSheet(f *excelize.File, splSheet string, ot models.OvertimeEntry, meta adidataSPLMeta) {
	_ = f.SetCellValue(splSheet, "B2", "SURAT PERINTAH LEMBUR")
	_ = f.SetCellValue(splSheet, "E4", ": "+meta.EmpID)
	_ = f.SetCellValue(splSheet, "E5", ": "+meta.UserName)
	_ = f.SetCellValue(splSheet, "E6", ": "+meta.DeptDiv)
	_ = f.SetCellValue(splSheet, "E7", ": "+meta.Pos)
	_ = f.SetCellValue(splSheet, "D12", ot.Date.Format("02 January 2006"))
	_ = f.SetCellValue(splSheet, "F12", fmt.Sprintf("%s - %s", ot.StartTime, ot.EndTime))
	_ = f.SetCellValue(splSheet, "H12", ot.TaskDescription)

	otTL, otDH := ExtractApprovers([]models.OvertimeEntry{ot})
	if otTL == "" {
		otTL = meta.TLName
	}
	if otDH == "" {
		otDH = meta.DHName
	}
	if meta.UserName != "" {
		_ = f.SetCellValue(splSheet, "C21", "( "+meta.UserName+" )")
	}
	if otTL != "" {
		_ = f.SetCellValue(splSheet, "G21", "( "+otTL+" )")
	}
	if otDH != "" {
		_ = f.SetCellValue(splSheet, "L21", "( "+otDH+" )")
	}
}

func writeAdidataSingleSPL(f *excelize.File, splSheet string, in GenerationInput, ot models.OvertimeEntry, empID string, st *BuilderStyles) {
	_ = f.SetColWidth(splSheet, "A", "A", 4)
	_ = f.SetColWidth(splSheet, "B", "B", 4)
	_ = f.SetColWidth(splSheet, "C", "C", 6)
	_ = f.SetColWidth(splSheet, "D", "E", 12)
	_ = f.SetColWidth(splSheet, "F", "G", 10)
	_ = f.SetColWidth(splSheet, "H", "O", 10)

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Family: "Calibri"},
	})
	_ = f.SetCellValue(splSheet, "B2", "SURAT PERINTAH LEMBUR")
	_ = f.SetCellStyle(splSheet, "B2", "B2", titleStyle)

	pos := in.User.Position
	if pos == "" {
		pos = defaultJuniorProgrammer
	}
	deptDiv := resolveAdidataDeptDiv(in.User)

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
	tlName, dhName := ExtractApprovers([]models.OvertimeEntry{ot})
	parties := []SignatureParty{
		{StartCol: "C", EndCol: "E", Name: in.User.Name, Position: pos},
		{StartCol: "G", EndCol: "J", Name: tlName, Position: "TEAM LEADER"},
		{StartCol: "L", EndCol: "N", Name: dhName, Position: "DEPARTMENT HEAD"},
	}
	for _, p := range parties {
		nameVal := ""
		if p.Name != "" {
			nameVal = "( " + p.Name + " )"
		}
		styleMergedRange(f, splSheet, p.StartCol+"21", p.EndCol+"21", st.BoldCenterStyle)
		_ = f.SetCellValue(splSheet, p.StartCol+"21", nameVal)
		styleMergedRange(f, splSheet, p.StartCol+"22", p.EndCol+"22", st.DataCenterStyle)
		_ = f.SetCellValue(splSheet, p.StartCol+"22", p.Position)
	}
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
