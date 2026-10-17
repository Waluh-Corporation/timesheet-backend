package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
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
	statusCounts := renderNTTDailyRows(f, sheet, in, byDay, st)

	nttSummary := map[string]SummaryFormulaItem{
		"E": {Formula: `COUNTIF(E11:E41,"P")`, Value: statusCounts["P"]},
		"F": {Formula: `COUNTIF(F11:F41,"S")`, Value: statusCounts["S"]},
		"G": {Formula: `COUNTIF(G11:G41,"BT")`, Value: statusCounts["BT"]},
		"H": {Formula: `COUNTIF(H11:H41,"PM")`, Value: statusCounts["PM"]},
		"I": {Formula: `COUNTIF(I11:I41,"V")`, Value: statusCounts["V"]},
		"J": {Formula: `COUNTIF(J11:J41,"X")`, Value: statusCounts["X"]},
	}
	WriteSummaryRowWithValues(f, sheet, "42", nttSummary, st.BoldCenterStyle)

	WriteSignatures(f, sheet, in, SignatureCells{
		UserCell:     "B55",
		TLCell:       "F55",
		DHCell:       "J55",
		UserDateCell: "B56",
		TLDateCell:   "F56",
		DHDateCell:   "J56",
		DatePrefix:   "DATE: ",
	})
	return WriteWorkbookToBuffer(f, "ntt")
}

func renderNTTDailyRows(
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
		row := 10 + day
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, allCols, []string{"K"}, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheet, "A", rs, allCols, []string{"K"}, dsc)
		_ = f.SetCellValue(sheet, "A"+rs, dsc.Date)

		WriteWorkingHoursRow(f, sheet, WorkingHoursConfig{
			StartCol:   "B",
			EndCol:     "C",
			TotalCol:   "D",
			RowStr:     rs,
			FormulaFmt: "IF(C{row}>B{row},(C{row}-B{row}),C{row}-B{row}+1)",
		}, dsc)

		status := renderNTTDayActivity(f, sheet, rs, row, dsc)
		WriteAttendanceMatrixStatus(f, sheet, rs, status)
		if status != "" {
			statusCounts[status]++
		}
	}
	return statusCounts
}

func renderNTTDayActivity(f *excelize.File, sheet, rs string, row int, dsc DayStyleContext) string {
	if dsc.HasActivity {
		status := strings.ToUpper(strings.TrimSpace(dsc.Activity.Status))
		projName := dsc.Activity.GetProjectName()
		if projName == "" {
			projName = "BNI Direct"
		}
		projCode := dsc.Activity.GetProjectCode()
		if projCode == "" {
			projCode = "P24015"
		}
		WriteActivityProjectCells(f, sheet, row, ActivityProjectInfo{
			Activity:    dsc.Activity.Activity,
			ProjectName: projName,
			ProjectCode: projCode,
			AppImpacted: dsc.Activity.GetAppImpacted(),
		})
		return status
	}
	if dsc.IsHolidayOrWeekend {
		WriteHolidayRemarkRow(f, sheet, "K", rs, row, dsc.Holiday)
	}
	return ""
}
