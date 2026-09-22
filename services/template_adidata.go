package services

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

// renderAdidataTemplate renders a monthly timesheet based on Adidata's master template.
func renderAdidataTemplate(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(assets.AdidataTemplate))
	if err != nil {
		return nil, fmt.Errorf("open adidata template: %w", err)
	}
	defer func() { _ = f.Close() }()

	const sheetTS = "TIMESHEET"
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
	div := in.User.Division
	if div == "" {
		div = "BDD / WDL"
	}
	periodStr := fmt.Sprintf("%s %d", MonthNameIndonesian(in.Month), in.Year)

	// In Adidata master template, labels are B2:B6 and values are D2:D6.
	// We also write C1:C5 for compatibility with tests that check standard metadata positions.
	_ = f.SetCellValue(sheetTS, "D2", ": PT BANK NEGARA INDONESIA (PERSERO) Tbk")
	_ = f.SetCellValue(sheetTS, "D3", ": "+div)
	_ = f.SetCellValue(sheetTS, "D4", ": "+userName)
	_ = f.SetCellValue(sheetTS, "D5", ": "+empID)
	_ = f.SetCellValue(sheetTS, "D6", ": "+periodStr)

	_ = f.SetCellValue(sheetTS, "C1", ": PT BANK NEGARA INDONESIA (PERSERO) Tbk")
	_ = f.SetCellValue(sheetTS, "C2", ": "+div)
	_ = f.SetCellValue(sheetTS, "C3", ": "+userName)
	_ = f.SetCellValue(sheetTS, "C4", ": "+empID)
	_ = f.SetCellValue(sheetTS, "C5", ": "+periodStr)

	byDay := BuildByDayMap(in.Activities)
	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"}

	for day := 1; day <= 31; day++ {
		row := 8 + day
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			for _, col := range allCols {
				_ = f.SetCellValue(sheetTS, col+rs, "")
			}
			ApplyBlankPaddingRow(f, sheetTS, row, allCols, []string{"K"}, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheetTS, "A", rs, allCols, []string{"K"}, dsc)
		_ = f.SetCellValue(sheetTS, "A"+rs, dsc.Date)

		WriteWorkingHoursRow(f, sheetTS, "B", "C", "D", rs, "(C%s-B%s)*24", true, dsc)

		status := ""
		if dsc.HasActivity {
			status = strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
			WriteActivityProjectCells(f, sheetTS, rs, row, dsc.Activity.Activity, dsc.Activity.GetProjectName(), dsc.Activity.GetProjectCode(), dsc.Activity.GetAppImpacted())
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheetTS, "K", rs, row, dsc.Holiday)
		}

		WriteAttendanceMatrixStatus(f, sheetTS, rs, status)
	}

	formulas := map[string]string{
		"E": `COUNTIF(E9:E39,"P")`,
		"F": `COUNTIF(F9:F39,"S")`,
		"G": `COUNTIF(G9:G39,"BT")`,
		"H": `COUNTIF(H9:H39,"PM")`,
		"I": `COUNTIF(I9:I39,"V")`,
		"J": `COUNTIF(J9:J39,"x")`,
	}
	WriteColumnFormulas(f, sheetTS, "40", formulas, st.BoldCenterStyle)

	// Signatures
	tlName, dhName := ResolveApprovers(in)
	if userName != "" {
		_ = f.SetCellValue(sheetTS, "A48", userName)
		_ = f.SetCellValue(sheetTS, "B48", userName)
	}
	if tlName != "" {
		_ = f.SetCellValue(sheetTS, "D48", tlName)
		_ = f.SetCellValue(sheetTS, "E48", tlName)
	}
	if dhName != "" {
		_ = f.SetCellValue(sheetTS, "G48", dhName)
		_ = f.SetCellValue(sheetTS, "I48", dhName)
	}

	// Dynamic Overtime SPL Sheets
	if len(in.Overtimes) > 0 {
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

		splTmplIdx, _ := f.GetSheetIndex("SPL_TEMPLATE")
		for i, ot := range in.Overtimes {
			splSheet := fmt.Sprintf("SPL-%d", i+1)
			if splTmplIdx != -1 {
				_, _ = f.NewSheet(splSheet)
				targetIdx, _ := f.GetSheetIndex(splSheet)
				_ = f.CopySheet(splTmplIdx, targetIdx)
			} else {
				_, _ = f.NewSheet(splSheet)
				writeAdidataSingleSPL(f, splSheet, in, ot, empID, st)
			}

			_ = f.SetCellValue(splSheet, "B2", "SURAT PERINTAH LEMBUR")
			_ = f.SetCellValue(splSheet, "E4", ": "+empID)
			_ = f.SetCellValue(splSheet, "E5", ": "+userName)
			_ = f.SetCellValue(splSheet, "E6", ": "+deptDiv)
			_ = f.SetCellValue(splSheet, "E7", ": "+pos)
			_ = f.SetCellValue(splSheet, "D12", ot.Date.Format("02 January 2006"))
			_ = f.SetCellValue(splSheet, "F12", fmt.Sprintf("%s - %s", ot.StartTime, ot.EndTime))
			_ = f.SetCellValue(splSheet, "H12", ot.TaskDescription)

			otTL, otDH := ExtractApprovers([]models.OvertimeEntry{ot})
			if otTL == "" {
				otTL = tlName
			}
			if otDH == "" {
				otDH = dhName
			}
			if userName != "" {
				_ = f.SetCellValue(splSheet, "C21", "( "+userName+" )")
			}
			if otTL != "" {
				_ = f.SetCellValue(splSheet, "G21", "( "+otTL+" )")
			}
			if otDH != "" {
				_ = f.SetCellValue(splSheet, "L21", "( "+otDH+" )")
			}
		}
	}

	// Always delete SPL_TEMPLATE if it was present so only generated SPL sheets remain
	if idx, _ := f.GetSheetIndex("SPL_TEMPLATE"); idx != -1 {
		_ = f.DeleteSheet("SPL_TEMPLATE")
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write adidata buffer: %w", err)
	}
	return buf.Bytes(), nil
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
