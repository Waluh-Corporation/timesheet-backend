package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
)

// renderNTTTemplate renders a monthly timesheet based on NTT's master template.
func renderNTTTemplate(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(assets.NTTTemplate))
	if err != nil {
		return nil, fmt.Errorf("open ntt template: %w", err)
	}
	defer func() { _ = f.Close() }()

	const sheet = "Timesheet"
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet NTT", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	empID := ResolveEmployeeID(in.User)
	div := in.User.Division
	if div == "" {
		div = "WDL"
	}
	periodStr := fmt.Sprintf("%s-%02d", time.Month(in.Month).String()[:3], in.Year%100)

	_ = f.SetCellValue(sheet, "B4", ": BNIdirect")
	_ = f.SetCellValue(sheet, "B5", ": "+div)
	_ = f.SetCellValue(sheet, "B6", ": "+userName)
	_ = f.SetCellValue(sheet, "B7", ": "+empID)
	_ = f.SetCellValue(sheet, "B8", ": "+periodStr)

	byDay := BuildByDayMap(in.Activities)
	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}

	for day := 1; day <= 31; day++ {
		row := 10 + day
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, allCols, []string{"K"}, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheet, "A", rs, allCols, []string{"K"}, dsc)
		_ = f.SetCellValue(sheet, "A"+rs, dsc.Date)

		WriteWorkingHoursRow(f, sheet, "B", "C", "D", rs, "IF(C{row}>B{row},(C{row}-B{row}),C{row}-B{row}+1)", false, dsc)

		status := ""
		if dsc.HasActivity {
			status = strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
			projName := dsc.Activity.GetProjectName()
			if projName == "" {
				projName = "BNI Direct"
			}
			projCode := dsc.Activity.GetProjectCode()
			if projCode == "" {
				projCode = "P24015"
			}
			WriteActivityProjectCells(f, sheet, rs, row, dsc.Activity.Activity, projName, projCode, dsc.Activity.GetAppImpacted())
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheet, "K", rs, row, dsc.Holiday)
		}

		WriteAttendanceMatrixStatus(f, sheet, rs, status)
	}

	WriteSignatures(f, sheet, in, "B55", "F55", "J55", "B56", "F56", "J56", "DATE: ")
	return WriteWorkbookToBuffer(f, "ntt")
}
