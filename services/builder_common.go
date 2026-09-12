package services

import (
	"fmt"
	"strings"

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

// SignatureParty defines one signer column block.
type SignatureParty struct {
	StartCol   string
	EndCol     string
	Title      string
	Name       string
	DatePrefix string
}

// WriteSignaturesLayout renders a configurable multi-party signature section.
func WriteSignaturesLayout(f *excelize.File, sheet string, headerRow int, boxRows int, parties []SignatureParty, st *BuilderStyles) {
	rHeader := fmt.Sprintf("%d", headerRow)
	rName := fmt.Sprintf("%d", headerRow+boxRows+1)
	rDate := fmt.Sprintf("%d", headerRow+boxRows+2)

	for _, p := range parties {
		// Header / Title
		styleMergedRange(f, sheet, p.StartCol+rHeader, p.EndCol+rHeader, st.BoldCenterStyle)
		_ = f.SetCellValue(sheet, p.StartCol+rHeader, p.Title)

		// Empty Signature Box
		if boxRows > 0 {
			rBoxStart := fmt.Sprintf("%d", headerRow+1)
			rBoxEnd := fmt.Sprintf("%d", headerRow+boxRows)
			styleMergedRange(f, sheet, p.StartCol+rBoxStart, p.EndCol+rBoxEnd, st.DataCenterStyle)
		}

		// Name
		nameStyle := st.BoldCenterStyle
		nameVal := p.Name
		if strings.HasPrefix(p.DatePrefix, "Nama :") {
			nameStyle = st.DataLeftStyle
			if p.Name != "" {
				nameVal = "Nama : " + p.Name
			} else {
				nameVal = "Nama : "
			}
		}
		styleMergedRange(f, sheet, p.StartCol+rName, p.EndCol+rName, nameStyle)
		_ = f.SetCellValue(sheet, p.StartCol+rName, nameVal)

		// Date (if provided)
		if p.DatePrefix != "" && !strings.HasPrefix(p.DatePrefix, "Nama :") {
			styleMergedRange(f, sheet, p.StartCol+rDate, p.EndCol+rDate, st.DataLeftStyle)
			_ = f.SetCellValue(sheet, p.StartCol+rDate, p.DatePrefix)
		}
	}
}

// DatePrefixUpper is the standard uppercase prefix used in timesheet signature blocks.
const DatePrefixUpper = "DATE : "

// WriteSignaturesBlock renders the 3-party signature section (Employee, Team Leader, Dept Head).
func WriteSignaturesBlock(f *excelize.File, sheet string, startRow int, userName, tlName, dhName string, st *BuilderStyles) {
	WriteSignaturesLayout(f, sheet, startRow, 3, []SignatureParty{
		{StartCol: "A", EndCol: "C", Title: "Prepared by :", Name: userName, DatePrefix: DatePrefixUpper},
		{StartCol: "D", EndCol: "F", Title: "Approved by :", Name: tlName, DatePrefix: DatePrefixUpper},
		{StartCol: "G", EndCol: "J", Title: "Approved by :", Name: dhName, DatePrefix: DatePrefixUpper},
	}, st)

	_ = f.SetRowHeight(sheet, startRow, 20)
	for i := 1; i <= 3; i++ {
		_ = f.SetRowHeight(sheet, startRow+i, 16)
	}
	_ = f.SetRowHeight(sheet, startRow+4, 22)
	_ = f.SetRowHeight(sheet, startRow+5, 18)
}
