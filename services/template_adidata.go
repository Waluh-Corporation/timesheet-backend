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
	})
	_ = f.SetCellValue(sheetTS, "B49", "( "+pos+" )")

	renderAdidataOvertimeSheets(f, in, empID, userName, tlName, dhName, st)

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
	wrapCols := []string{"L", "O", "Q", "R"}
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

// AdidataSPLConfig encapsulates parameters required to render a single Adidata SPL sheet.
type AdidataSPLConfig struct {
	SheetNum  int
	TmplIndex int
	Ot        models.OvertimeEntry
	EmpID     string
	UserName  string
	DeptDiv   string
	Position  string
	TLName    string
	DHName    string
}

func renderAdidataOvertimeSheets(
	f *excelize.File,
	in GenerationInput,
	empID, userName, tlName, dhName string,
	st *BuilderStyles,
) {
	if len(in.Overtimes) == 0 {
		if idx, _ := f.GetSheetIndex("SPL_TEMPLATE"); idx != -1 {
			_ = f.DeleteSheet("SPL_TEMPLATE")
		}
		return
	}

	pos := in.User.Position
	if pos == "" {
		pos = defaultPosition
	}
	deptDiv := in.User.Department
	if deptDiv == "" {
		deptDiv = "WCH / WDL"
	} else if in.User.Division != "" {
		deptDiv = deptDiv + " / " + in.User.Division
	}

	splTmplIdx, _ := f.GetSheetIndex("SPL_TEMPLATE")
	for i, ot := range in.Overtimes {
		renderSingleAdidataOvertimeSheet(f, in, AdidataSPLConfig{
			SheetNum:  i + 1,
			TmplIndex: splTmplIdx,
			Ot:        ot,
			EmpID:     empID,
			UserName:  userName,
			DeptDiv:   deptDiv,
			Position:  pos,
			TLName:    tlName,
			DHName:    dhName,
		}, st)
	}

	if splTmplIdx != -1 {
		_ = f.DeleteSheet("SPL_TEMPLATE")
	}
}

func renderSingleAdidataOvertimeSheet(
	f *excelize.File,
	in GenerationInput,
	cfg AdidataSPLConfig,
	st *BuilderStyles,
) {
	splSheet := fmt.Sprintf("SPL-%d", cfg.SheetNum)
	if cfg.TmplIndex != -1 {
		_, _ = f.NewSheet(splSheet)
		targetIdx, _ := f.GetSheetIndex(splSheet)
		_ = f.CopySheet(cfg.TmplIndex, targetIdx)
	} else {
		_, _ = f.NewSheet(splSheet)
		writeAdidataSingleSPL(f, splSheet, in, cfg.Ot, cfg.EmpID, st)
	}

	_ = f.SetCellValue(splSheet, "B2", "SURAT PERINTAH LEMBUR")
	_ = f.SetCellValue(splSheet, "E4", ": "+cfg.EmpID)
	_ = f.SetCellValue(splSheet, "E5", ": "+cfg.UserName)
	_ = f.SetCellValue(splSheet, "E6", ": "+cfg.DeptDiv)
	_ = f.SetCellValue(splSheet, "E7", ": "+cfg.Position)
	_ = f.SetCellValue(splSheet, "D12", cfg.Ot.Date.Format("02 January 2006"))
	_ = f.SetCellValue(splSheet, "F12", fmt.Sprintf("%s - %s", cfg.Ot.StartTime, cfg.Ot.EndTime))
	_ = f.SetCellValue(splSheet, "H12", cfg.Ot.TaskDescription)

	otTL, otDH := ExtractApprovers([]models.OvertimeEntry{cfg.Ot})
	if otTL == "" {
		otTL = cfg.TLName
	}
	if otDH == "" {
		otDH = cfg.DHName
	}
	if cfg.UserName != "" {
		_ = f.SetCellValue(splSheet, "C21", "( "+cfg.UserName+" )")
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
		pos = defaultPosition
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
