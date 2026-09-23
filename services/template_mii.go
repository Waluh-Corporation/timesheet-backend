package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
)

const (
	miiProjectName = "BNI Direct"
	miiProjectID   = "P24015"
	miiDivision    = "Wholesale Digital Delivery"
	miiDepartment  = "Wholesale Channel and Service Delivery"
)

// renderMIITemplate renders a monthly timesheet based on MII's master template.
func renderMIITemplate(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(assets.MIITemplate))
	if err != nil {
		return nil, fmt.Errorf("open mii template: %w", err)
	}
	defer func() { _ = f.Close() }()

	const sheet = "Sheet1"
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet MII", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	div := in.User.Division
	if div == "" {
		div = miiDivision
	}
	empID := ResolveEmployeeID(in.User)
	site := in.User.Site
	if site == "" {
		site = "BNI - RDTX"
	}
	periodStr := fmt.Sprintf("%s-%02d", time.Month(in.Month).String()[:3], in.Year%100)

	_ = f.SetCellValue(sheet, "C1", ": "+miiProjectName)
	_ = f.SetCellValue(sheet, "C2", ": "+div)
	_ = f.SetCellValue(sheet, "C3", ": "+userName)
	_ = f.SetCellValue(sheet, "C4", ": "+empID)
	_ = f.SetCellValue(sheet, "C5", ": "+site)
	_ = f.SetCellValue(sheet, "C6", ": "+periodStr)

	byDay := BuildByDayMap(in.Activities)
	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"}

	for day := 1; day <= 31; day++ {
		row := 8 + day
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, allCols, []string{"K", "Q"}, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheet, "A", rs, allCols, []string{"K", "Q"}, dsc)
		_ = f.SetCellValue(sheet, "A"+rs, dsc.Date)

		WriteWorkingHoursRow(f, sheet, "B", "C", "D", rs, "C%s-B%s", false, dsc)

		status := ""
		if dsc.HasActivity {
			status = strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
			_ = f.SetCellValue(sheet, "K"+rs, dsc.Activity.Activity)
			_ = f.SetCellValue(sheet, "L"+rs, miiProjectName)
			_ = f.SetCellValue(sheet, "M"+rs, miiProjectID)
			_ = f.SetCellValue(sheet, "N"+rs, NormalizeMIIAppImpacted(dsc.Activity.GetAppImpacted()))
			_ = f.SetCellValue(sheet, "P"+rs, miiDivision)
			_ = f.SetCellValue(sheet, "Q"+rs, miiDepartment)

			h := calculateRowHeight(dsc.Activity.Activity, miiProjectName, miiProjectID, dsc.Activity.GetAppImpacted(), miiDivision, miiDepartment)
			_ = f.SetRowHeight(sheet, row, h)
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheet, "K", rs, row, dsc.Holiday)
		}

		WriteAttendanceMatrixStatus(f, sheet, rs, status)
	}

	WriteSignatures(f, sheet, in, "A46", "D46", "G46", "A47", "D47", "G47", "DATE : ")
	return WriteWorkbookToBuffer(f, "mii")
}
