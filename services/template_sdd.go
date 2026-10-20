package services

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
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
	statusCounts := renderSDDDailyRows(f, sheet, in, byDay, st)

	sddSummary := map[string]SummaryFormulaItem{
		"F": {Formula: `COUNTIF(F12:F42,"H")`, Value: statusCounts["F"]},
		"G": {Formula: `COUNTIF(G12:G42,"C")`, Value: statusCounts["G"]},
		"H": {Formula: `COUNTIF(H12:H42,"I")`, Value: statusCounts["H"]},
		"I": {Formula: `COUNTIF(I12:I42,"S")`, Value: statusCounts["I"]},
		"J": {Formula: `COUNTIF(J12:J42,"L")`, Value: statusCounts["J"]},
	}
	WriteSummaryRowWithValues(f, sheet, "43", sddSummary, st.BoldCenterStyle)

	tlName, dhName := ResolveApprovers(in)
	_ = f.SetCellValue(sheet, "C52", sddNamePrefix+userName)
	_ = f.SetCellValue(sheet, "H52", sddNamePrefix+tlName)
	_ = f.SetCellValue(sheet, "L52", sddNamePrefix+dhName)

	if in.IsSigned && len(in.SignatureBytes) > 0 {
		_ = EmbedPicture(f, sheet, "C46", in.SignatureBytes, 0.4, 0.4)
	}
	if in.IsSigned && len(in.QRCodeBytes) > 0 {
		_ = EmbedPicture(f, sheet, "M46", in.QRCodeBytes, 0.6, 0.6)
	}

	return WriteWorkbookToBuffer(f, "sdd")
}

func renderSDDDailyRows(
	f *excelize.File,
	sheet string,
	in GenerationInput,
	byDay map[int]models.DailyActivity,
	st *BuilderStyles,
) map[string]int {
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

		WriteWorkingHoursRow(f, sheet, WorkingHoursConfig{
			StartCol: "C", EndCol: "D", TotalCol: "E", RowStr: rs, FormulaFmt: "D%s-C%s",
		}, dsc)

		renderSDDDayActivity(f, sheet, rs, row, dsc, statusCounts)
	}
	return statusCounts
}

func renderSDDDayActivity(
	f *excelize.File,
	sheet, rs string,
	row int,
	dsc DayStyleContext,
	statusCounts map[string]int,
) {
	if dsc.HasActivity {
		col, mark := resolveSDDStatusMark(dsc.Activity.Status)
		if col != "" {
			_ = f.SetCellValue(sheet, col+rs, mark)
			statusCounts[col]++
		}

		_ = f.SetCellValue(sheet, "K"+rs, dsc.Activity.GetProjectName())
		_ = f.SetCellValue(sheet, "L"+rs, dsc.Activity.GetProjectCode())
		_ = f.SetCellValue(sheet, "N"+rs, dsc.Activity.Activity)

		h := calculateRowHeight(dsc.Activity.Activity, dsc.Activity.GetProjectName(), dsc.Activity.GetProjectCode(), "", "", "")
		_ = f.SetRowHeight(sheet, row, h)
		return
	}
	if dsc.IsHolidayOrWeekend {
		WriteHolidayRemarkRow(f, sheet, "N", rs, row, dsc.Holiday)
	}
}

func resolveSDDStatusMark(status string) (col, mark string) {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "H", "P", "HADIR", "PRESENT":
		return "F", "H"
	case "C", "CUTI", "V", "VACATION":
		return "G", "C"
	case "I", "IZIN", "PM", "PERMIT":
		return "H", "I"
	case "S", "SAKIT", "SICK":
		return "I", "S"
	case "L", "LEMBUR":
		return "J", "L"
	default:
		return "", ""
	}
}
