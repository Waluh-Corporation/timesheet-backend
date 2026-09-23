package services

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
)

const sddNamePrefix = "Nama : "

// indonesianMonth returns Indonesian month names for SDD, delegating to MonthNameIndonesian.
func indonesianMonth(m int) string {
	return MonthNameIndonesian(m)
}

// renderSDDTemplate renders a monthly timesheet based on PT Swadharma Duta Data (SDD)'s master template.
func renderSDDTemplate(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(assets.SDDTemplate))
	if err != nil {
		return nil, fmt.Errorf("open sdd template: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheet := MonthNameIndonesian(in.Month)
	// Template initially contains sheet named "Juni", rename it to current month
	sheetList := f.GetSheetList()
	if len(sheetList) > 0 && sheetList[0] != sheet {
		_ = f.SetSheetName(sheetList[0], sheet)
	}

	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet SDD", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	empID := ResolveEmployeeID(in.User)
	dept := in.User.Department
	if dept == "" {
		dept = "WCH / WDL"
	}
	grp := in.User.GroupName
	if grp == "" {
		grp = "Junior Programmer"
	}

	_ = f.SetCellValue(sheet, "F2", ": "+userName)
	_ = f.SetCellValue(sheet, "F3", ": "+empID)
	_ = f.SetCellValue(sheet, "F4", ": "+in.User.Division)
	_ = f.SetCellValue(sheet, "F5", ": "+dept)
	_ = f.SetCellValue(sheet, "F6", ": "+grp)
	_ = f.SetCellValue(sheet, "F7", fmt.Sprintf(": %s %d", MonthNameIndonesian(in.Month), in.Year))

	byDay := BuildByDayMap(in.Activities)
	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}
	statusCounts := make(map[string]int)

	for day := 1; day <= 31; day++ {
		row := 11 + day
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, allCols, []string{"N"}, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheet, "B", rs, allCols, []string{"N"}, dsc)
		_ = f.SetCellValue(sheet, "A"+rs, day)

		// Clear status cells
		for _, col := range []string{"F", "G", "H", "I", "J"} {
			_ = f.SetCellValue(sheet, col+rs, "")
		}

		WriteWorkingHoursRow(f, sheet, "C", "D", "E", rs, "D%s-C%s", false, dsc)

		if dsc.HasActivity {
			var col string
			switch strings.ToUpper(strings.TrimSpace(dsc.Activity.Status)) {
			case "H", "P", "HADIR", "PRESENT":
				col = "F"
			case "C", "CUTI", "V", "VACATION":
				col = "G"
			case "I", "IZIN", "PM", "PERMIT":
				col = "H"
			case "S", "SAKIT", "SICK":
				col = "I"
			case "L", "LEMBUR":
				col = "J"
			}
			if col != "" {
				_ = f.SetCellValue(sheet, col+rs, "v")
				statusCounts[col]++
			}

			_ = f.SetCellValue(sheet, "K"+rs, dsc.Activity.GetProjectName())
			_ = f.SetCellValue(sheet, "L"+rs, dsc.Activity.GetProjectCode())
			_ = f.SetCellValue(sheet, "N"+rs, dsc.Activity.Activity)

			h := calculateRowHeight(dsc.Activity.Activity, dsc.Activity.GetProjectName(), dsc.Activity.GetProjectCode(), "", "", "")
			_ = f.SetRowHeight(sheet, row, h)
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheet, "N", rs, row, dsc.Holiday)
		}
	}

	sddSummary := map[string]SummaryFormulaItem{
		"F": {Formula: `COUNTIF(F12:F42,"v")`, Value: statusCounts["F"]},
		"G": {Formula: `COUNTIF(G12:G42,"v")`, Value: statusCounts["G"]},
		"H": {Formula: `COUNTIF(H12:H42,"v")`, Value: statusCounts["H"]},
		"I": {Formula: `COUNTIF(I12:I42,"v")`, Value: statusCounts["I"]},
		"J": {Formula: `COUNTIF(J12:J42,"v")`, Value: statusCounts["J"]},
	}
	WriteSummaryRowWithValues(f, sheet, "43", sddSummary, st.BoldCenterStyle)

	tlName, dhName := ResolveApprovers(in)
	_ = f.SetCellValue(sheet, "C52", sddNamePrefix+userName)
	_ = f.SetCellValue(sheet, "H52", sddNamePrefix+tlName)
	_ = f.SetCellValue(sheet, "L52", sddNamePrefix+dhName)

	return WriteWorkbookToBuffer(f, "sdd")
}
