package services

import (
	"fmt"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/models"
)

// DailyRowConfig configures vendor-specific column mappings for daily row generation.
type DailyRowConfig struct {
	Columns       []string          // all table columns (e.g. ["A", "B", ...])
	WrapColumns   []string          // columns that require word wrap
	StatusColumns map[string]string // status code -> column letter (e.g. "P" -> "E")
	StatusMarks   map[string]string // status code -> mark string (e.g. "P" -> "P")
	HourFormula   string            // e.g. "C%s-B%s" or "IF(C%s>B%s,(C%s-B%s),C%s-B%s+1)"
	TimeColumn    string            // total hour column, e.g. "D"
	DefaultStatus string            // default status for working days without entry, e.g. "P"
}

// BuildByDayMap maps daily activities by day of month (1-31).
func BuildByDayMap(activities []models.DailyActivity) map[int]models.DailyActivity {
	byDay := make(map[int]models.DailyActivity, len(activities))
	for _, a := range activities {
		byDay[a.Date.Day()] = a
	}
	return byDay
}

// ExtractApprovers finds team leader and department head names from overtime entries.
func ExtractApprovers(overtimes []models.OvertimeEntry) (tlName string, dhName string) {
	for _, ot := range overtimes {
		if ot.TeamLeader != nil && ot.TeamLeader.Name != "" && tlName == "" {
			tlName = ot.TeamLeader.Name
		}
		if ot.DepartmentHead != nil && ot.DepartmentHead.Name != "" && dhName == "" {
			dhName = ot.DepartmentHead.Name
		}
	}
	return tlName, dhName
}

// WriteTimeCells parses start and end times to Excel fractions and sets their cells.
func WriteTimeCells(f *excelize.File, sheet, startCell, endCell string, startStr, endStr string, timeStyle int) (hasStart, hasEnd bool) {
	if startStr != "" {
		if frac, err := parseTimeToExcelFraction(startStr); err == nil {
			_ = f.SetCellValue(sheet, startCell, frac)
			_ = f.SetCellStyle(sheet, startCell, startCell, timeStyle)
			hasStart = true
		}
	}
	if endStr != "" {
		if frac, err := parseTimeToExcelFraction(endStr); err == nil {
			_ = f.SetCellValue(sheet, endCell, frac)
			_ = f.SetCellStyle(sheet, endCell, endCell, timeStyle)
			hasEnd = true
		}
	}
	return hasStart, hasEnd
}

// ApplyBlankPaddingRow formats blank days beyond the current month's end.
func ApplyBlankPaddingRow(f *excelize.File, sheet string, row int, cols []string, wrapCols []string, st *BuilderStyles) {
	rs := fmt.Sprintf("%d", row)
	for _, col := range cols {
		_ = f.SetCellStyle(sheet, col+rs, col+rs, st.DataCenterStyle)
	}
	for _, col := range wrapCols {
		_ = f.SetCellStyle(sheet, col+rs, col+rs, st.DataCenterWrapStyle)
	}
	_ = f.SetRowHeight(sheet, row, 15)
}

// WriteTotalSummaryRow writes the standard TOTAL row with formula sum and attendance counts.
func WriteTotalSummaryRow(f *excelize.File, sheet string, summaryRow, firstDataRow, lastDataRow int, statusCols map[string]string, st *BuilderStyles) {
	sr := fmt.Sprintf("%d", summaryRow)
	fdr := fmt.Sprintf("%d", firstDataRow)
	ldr := fmt.Sprintf("%d", lastDataRow)

	_ = f.SetCellValue(sheet, "A"+sr, "TOTAL")
	_ = f.SetCellStyle(sheet, "A"+sr, "A"+sr, st.BoldCenterStyle)

	for _, col := range []string{"B", "C"} {
		_ = f.SetCellStyle(sheet, col+sr, col+sr, st.BoldCenterStyle)
	}

	_ = f.SetCellFormula(sheet, "D"+sr, fmt.Sprintf("SUM(D%s:D%s)", fdr, ldr))
	_ = f.SetCellStyle(sheet, "D"+sr, "D"+sr, st.TimeStyle)

	for status, col := range statusCols {
		cell := col + sr
		mark := status
		if status == "X" {
			mark = "x"
		}
		_ = f.SetCellFormula(sheet, cell, fmt.Sprintf("COUNTIF(%s%s:%s%s, \"%s\")", col, fdr, col, ldr, mark))
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldCenterStyle)
	}
}

// WriteSignaturesBlock renders the 3-party signature section (Employee, Team Leader, Dept Head).
func WriteSignaturesBlock(f *excelize.File, sheet string, startRow int, userName, tlName, dhName string, st *BuilderStyles) {
	rTitles := fmt.Sprintf("%d", startRow)
	rNames := fmt.Sprintf("%d", startRow+4)
	rDates := fmt.Sprintf("%d", startRow+5)

	// Titles
	styleMergedRange(f, sheet, "A"+rTitles, "C"+rTitles, st.HeaderStyle)
	_ = f.SetCellValue(sheet, "A"+rTitles, "Prepared by :")

	styleMergedRange(f, sheet, "D"+rTitles, "F"+rTitles, st.HeaderStyle)
	_ = f.SetCellValue(sheet, "D"+rTitles, "Approved by :")

	styleMergedRange(f, sheet, "G"+rTitles, "J"+rTitles, st.HeaderStyle)
	_ = f.SetCellValue(sheet, "G"+rTitles, "Approved by :")

	// Empty signature box rows
	for i := 1; i <= 3; i++ {
		rEmpty := fmt.Sprintf("%d", startRow+i)
		styleMergedRange(f, sheet, "A"+rEmpty, "C"+rEmpty, st.DataCenterStyle)
		styleMergedRange(f, sheet, "D"+rEmpty, "F"+rEmpty, st.DataCenterStyle)
		styleMergedRange(f, sheet, "G"+rEmpty, "J"+rEmpty, st.DataCenterStyle)
		_ = f.SetRowHeight(sheet, startRow+i, 16)
	}

	// Names
	styleMergedRange(f, sheet, "A"+rNames, "C"+rNames, st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "A"+rNames, userName)

	styleMergedRange(f, sheet, "D"+rNames, "F"+rNames, st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "D"+rNames, tlName)

	styleMergedRange(f, sheet, "G"+rNames, "J"+rNames, st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "G"+rNames, dhName)

	// Dates
	styleMergedRange(f, sheet, "A"+rDates, "C"+rDates, st.DataLeftStyle)
	_ = f.SetCellValue(sheet, "A"+rDates, "DATE : ")

	styleMergedRange(f, sheet, "D"+rDates, "F"+rDates, st.DataLeftStyle)
	_ = f.SetCellValue(sheet, "D"+rDates, "DATE : ")

	styleMergedRange(f, sheet, "G"+rDates, "J"+rDates, st.DataLeftStyle)
	_ = f.SetCellValue(sheet, "G"+rDates, "DATE : ")

	_ = f.SetRowHeight(sheet, startRow, 20)
	_ = f.SetRowHeight(sheet, startRow+4, 22)
	_ = f.SetRowHeight(sheet, startRow+5, 18)
}
