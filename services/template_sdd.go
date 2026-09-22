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

	for day := 1; day <= 31; day++ {
		row := 11 + day
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			for _, col := range allCols {
				_ = f.SetCellValue(sheet, col+rs, "")
			}
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
			switch strings.ToUpper(strings.TrimSpace(dsc.Activity.Status)) {
			case "H", "P", "HADIR", "PRESENT":
				_ = f.SetCellValue(sheet, "F"+rs, "v")
			case "C", "CUTI", "V", "VACATION":
				_ = f.SetCellValue(sheet, "G"+rs, "v")
			case "I", "IZIN", "PM", "PERMIT":
				_ = f.SetCellValue(sheet, "H"+rs, "v")
			case "S", "SAKIT", "SICK":
				_ = f.SetCellValue(sheet, "I"+rs, "v")
			case "L", "LEMBUR":
				_ = f.SetCellValue(sheet, "J"+rs, "v")
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

	formulas := map[string]string{
		"F": `COUNTIF(F12:F42,"v")`,
		"G": `COUNTIF(G12:G42,"v")`,
		"H": `COUNTIF(H12:H42,"v")`,
		"I": `COUNTIF(I12:I42,"v")`,
		"J": `COUNTIF(J12:J42,"v")`,
	}
	WriteColumnFormulas(f, sheet, "43", formulas, st.BoldCenterStyle)

	tlName, dhName := ResolveApprovers(in)
	_ = f.SetCellValue(sheet, "C52", sddNamePrefix+userName)
	_ = f.SetCellValue(sheet, "H52", sddNamePrefix+tlName)
	_ = f.SetCellValue(sheet, "L52", sddNamePrefix+dhName)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write sdd buffer: %w", err)
	}
	return buf.Bytes(), nil
}
