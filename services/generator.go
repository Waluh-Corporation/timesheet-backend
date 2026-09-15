package services

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"timesheet-backend/models"
)

// Helper functions for pointers
func intPtr(v int) *int          { return &v }
func stringPtr(v string) *string { return &v }

// parseTimeToExcelFraction parses a "hh:mm" time string to the fractional value of a day
func parseTimeToExcelFraction(timeStr string) (float64, error) {
	var h, m int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m)
	if err != nil {
		return 0, err
	}
	return float64(h*60+m) / 1440.0, nil
}

// estimateCellLines estimates the number of wrapped lines for a text cell given a specific column width
func estimateCellLines(text string, colWidth float64) int {
	if text == "" {
		return 1
	}
	// Proportional font adjustment factor: assume average character width is slightly larger than the grid units
	effectiveWidth := int(colWidth * 0.95)
	if effectiveWidth < 5 {
		effectiveWidth = 5
	}
	lines := 0
	segments := strings.Split(text, "\n")
	for _, seg := range segments {
		if len(seg) == 0 {
			lines++
			continue
		}
		segLines := (len(seg) + effectiveWidth - 1) / effectiveWidth
		if segLines == 0 {
			segLines = 1
		}
		lines += segLines
	}
	return lines
}

// calculateRowHeight calculates the dynamic height of a row based on text content and column widths
func calculateRowHeight(activity, projectName, projectID, appImpacted, division, department string) float64 {
	maxLines := 1

	colWidths := map[string]float64{
		"K": 62.2, // Activity
		"L": 15.0, // Project Name
		"M": 9.6,  // Project ID
		"N": 18.2, // App Impacted
		"P": 20.9, // Division
		"Q": 12.8, // Department
	}

	if l := estimateCellLines(activity, colWidths["K"]); l > maxLines {
		maxLines = l
	}
	if l := estimateCellLines(projectName, colWidths["L"]); l > maxLines {
		maxLines = l
	}
	if l := estimateCellLines(projectID, colWidths["M"]); l > maxLines {
		maxLines = l
	}
	if l := estimateCellLines(appImpacted, colWidths["N"]); l > maxLines {
		maxLines = l
	}
	if l := estimateCellLines(division, colWidths["P"]); l > maxLines {
		maxLines = l
	}
	if l := estimateCellLines(department, colWidths["Q"]); l > maxLines {
		maxLines = l
	}

	if maxLines > 1 {
		return 15.0 + float64(maxLines-1)*13.0
	}
	return 15.0
}

// GenerateExcel processes the master template and returns the filled spreadsheet as a byte array
type masterTemplateStyles struct {
	grayStyle         int
	grayDateStyle     int
	dateStyle         int
	timeStyle         int
	activeStyleCenter int
	activeStyleLeft   int
}

func initMasterStyles(f *excelize.File) (*masterTemplateStyles, error) {
	borderBlack := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
	fontArial11 := &excelize.Font{Family: "Arial", Size: 11}
	fillGray := excelize.Fill{Type: "pattern", Color: []string{"#D3D3D3"}, Pattern: 1}

	grayStyle, err := f.NewStyle(&excelize.Style{
		Fill:      fillGray,
		Font:      fontArial11,
		Border:    borderBlack,
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center", WrapText: true},
	})
	if err != nil {
		return nil, fmt.Errorf("generate gray Excel styling: %w", err)
	}

	grayDateStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("d-m-yy"),
		Fill:         fillGray,
		Font:         fontArial11,
		Border:       borderBlack,
		Alignment:    &excelize.Alignment{Vertical: "center", Horizontal: "center"},
	})

	dateStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("d-m-yy"),
		Font:         fontArial11,
		Border:       borderBlack,
		Alignment:    &excelize.Alignment{Vertical: "center", Horizontal: "center"},
	})

	timeStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("hh:mm"),
		Font:         fontArial11,
		Border:       borderBlack,
		Alignment:    &excelize.Alignment{Vertical: "center", Horizontal: "center"},
	})

	activeStyleCenter, _ := f.NewStyle(&excelize.Style{
		Font:      fontArial11,
		Border:    borderBlack,
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center"},
	})

	activeStyleLeft, _ := f.NewStyle(&excelize.Style{
		Font:      fontArial11,
		Border:    borderBlack,
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center", WrapText: true},
	})

	return &masterTemplateStyles{
		grayStyle:         grayStyle,
		grayDateStyle:     grayDateStyle,
		dateStyle:         dateStyle,
		timeStyle:         timeStyle,
		activeStyleCenter: activeStyleCenter,
		activeStyleLeft:   activeStyleLeft,
	}, nil
}

func writeMasterHeaderMetadata(f *excelize.File, sheet string, req *models.TimesheetRequest) {
	if req.Project != "" {
		_ = f.SetCellValue(sheet, "C1", req.Project)
	}
	if req.Division != "" {
		_ = f.SetCellValue(sheet, "C2", req.Division)
	}
	if req.Name != "" {
		_ = f.SetCellValue(sheet, "C3", req.Name)
	}
	if req.BniID != "" {
		_ = f.SetCellValue(sheet, "C4", req.BniID)
	}
	if req.Site != "" {
		_ = f.SetCellValue(sheet, "C5", req.Site)
	}

	periodStr := fmt.Sprintf("%d-%02d-01", req.Year, req.Month)
	_ = f.SetCellValue(sheet, "C6", periodStr)

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Family: "Arial", Size: 11},
	})
	if err == nil {
		for i := 1; i <= 6; i++ {
			_ = f.SetCellStyle(sheet, fmt.Sprintf("C%d", i), fmt.Sprintf("C%d", i), headerStyle)
		}
	}
}

func trimMasterExcessRows(f *excelize.File, sheet string, daysInMonth int) error {
	for r := 39; r >= 9+daysInMonth; r-- {
		if err := f.RemoveRow(sheet, r); err != nil {
			return fmt.Errorf("failed to remove excess row: %w", err)
		}
	}
	sumRow := 9 + daysInMonth
	lastDayRow := 8 + daysInMonth
	statusFormulas := map[string]string{
		"E": fmt.Sprintf("COUNTIF(E9:E%d,\"P\")", lastDayRow),
		"F": fmt.Sprintf("COUNTIF(F9:F%d,\"S\")", lastDayRow),
		"G": fmt.Sprintf("COUNTIF(G9:G%d,\"BT\")", lastDayRow),
		"H": fmt.Sprintf("COUNTIF(H9:H%d,\"PM\")", lastDayRow),
		"I": fmt.Sprintf("COUNTIF(I9:I%d,\"V\")", lastDayRow),
		"J": fmt.Sprintf("COUNTIF(J9:J%d,\"x\")", lastDayRow),
	}
	for col, formula := range statusFormulas {
		_ = f.SetCellFormula(sheet, fmt.Sprintf("%s%d", col, sumRow), formula)
	}
	return nil
}

func writeMasterSignatures(f *excelize.File, sheet string, req *models.TimesheetRequest, daysInMonth int) {
	finalRow := 12 + daysInMonth
	sigStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Family: "Arial", Size: 11},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom", WrapText: true},
	})
	if err != nil {
		return
	}
	if req.SignatureEmployee != "" {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", finalRow), req.SignatureEmployee)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", finalRow), fmt.Sprintf("C%d", finalRow+3), sigStyle)
	}
	if req.SignatureReviewer != "" {
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", finalRow), req.SignatureReviewer)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("D%d", finalRow), fmt.Sprintf("F%d", finalRow+3), sigStyle)
	}
	if req.SignatureApprover != "" {
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", finalRow), req.SignatureApprover)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("G%d", finalRow), fmt.Sprintf("J%d", finalRow+3), sigStyle)
	}
}

func writeMasterNonWorkingDay(f *excelize.File, sheet string, r int, d time.Time, isHoliday bool, holidayDesc string, st *masterTemplateStyles) float64 {
	_ = f.SetCellStyle(sheet, fmt.Sprintf("B%d", r), fmt.Sprintf("Q%d", r), st.grayStyle)
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r), d)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.grayDateStyle)

	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", r), "")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", r), "")

	for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, r), "")
	}

	if isHoliday {
		_ = f.SetCellValue(sheet, fmt.Sprintf("K%d", r), holidayDesc)
		for _, col := range []string{"L", "M", "N", "O", "P", "Q"} {
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, r), "")
		}
		return calculateRowHeight(holidayDesc, "", "", "", "", "")
	}

	for _, col := range []string{"K", "L", "M", "N", "O", "P", "Q"} {
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, r), "")
	}
	return 15.0
}

func writeMasterWorkingDay(f *excelize.File, sheet string, r int, d time.Time, entry *models.DailyEntry, st *masterTemplateStyles) float64 {
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r), d)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.dateStyle)

	var h float64
	if entry != nil {
		h = writeMasterEntryDetails(f, sheet, r, entry)
	} else {
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", r), "")
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", r), "")
		for _, col := range []string{"E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q"} {
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, r), "")
		}
		h = 15.0
	}

	_ = f.SetCellStyle(sheet, fmt.Sprintf("B%d", r), fmt.Sprintf("C%d", r), st.timeStyle)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("D%d", r), fmt.Sprintf("D%d", r), st.timeStyle)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("E%d", r), fmt.Sprintf("J%d", r), st.activeStyleCenter)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("K%d", r), fmt.Sprintf("Q%d", r), st.activeStyleLeft)
	return h
}

func writeMasterTimeCell(f *excelize.File, sheet, cell, timeStr string) {
	if timeStr == "" || timeStr == "00:00" {
		_ = f.SetCellValue(sheet, cell, "")
		return
	}
	if frac, err := parseTimeToExcelFraction(timeStr); err == nil {
		_ = f.SetCellValue(sheet, cell, frac)
		return
	}
	_ = f.SetCellValue(sheet, cell, timeStr)
}

func writeMasterEntryDetails(f *excelize.File, sheet string, r int, entry *models.DailyEntry) float64 {
	rs := fmt.Sprintf("%d", r)
	writeMasterTimeCell(f, sheet, "B"+rs, entry.StartTime)
	writeMasterTimeCell(f, sheet, "C"+rs, entry.EndTime)
	WriteAttendanceMatrixStatus(f, sheet, rs, strings.ToUpper(entry.Status))

	_ = f.SetCellValue(sheet, "K"+rs, entry.Activity)
	_ = f.SetCellValue(sheet, "L"+rs, entry.ProjectName)
	_ = f.SetCellValue(sheet, "M"+rs, entry.ProjectID)
	_ = f.SetCellValue(sheet, "N"+rs, entry.AppImpacted)
	_ = f.SetCellValue(sheet, "O"+rs, "")
	_ = f.SetCellValue(sheet, "P"+rs, entry.Division)
	_ = f.SetCellValue(sheet, "Q"+rs, entry.Department)

	return calculateRowHeight(entry.Activity, entry.ProjectName, entry.ProjectID, entry.AppImpacted, entry.Division, entry.Department)
}

// GenerateExcel processes the master template and returns the filled spreadsheet as a byte array
func GenerateExcel(req *models.TimesheetRequest, holidayMap map[string]string) ([]byte, error) {
	templatePath := os.Getenv("TEMPLATE_PATH")
	if templatePath == "" {
		templatePath = "templates/master_template.xlsx"
	}

	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel template: %w", err)
	}
	defer func() { _ = f.Close() }()

	userName := ""
	if req != nil {
		userName = req.Name
	}
	SetWorkbookProperties(f, "Monthly Timesheet", userName)

	const sheetName = "Sheet1"
	writeMasterHeaderMetadata(f, sheetName, req)

	daysInMonth := GetDaysInMonth(req.Year, req.Month)
	if err := trimMasterExcessRows(f, sheetName, daysInMonth); err != nil {
		return nil, err
	}

	writeMasterSignatures(f, sheetName, req, daysInMonth)

	st, err := initMasterStyles(f)
	if err != nil {
		return nil, err
	}

	entryMap := make(map[int]*models.DailyEntry, len(req.DailyEntries))
	for i := range req.DailyEntries {
		entryMap[req.DailyEntries[i].Day] = &req.DailyEntries[i]
	}

	for day := 1; day <= daysInMonth; day++ {
		r := 8 + day
		d := time.Date(req.Year, time.Month(req.Month), day, 0, 0, 0, 0, time.UTC)
		isWeekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
		dateStr := fmt.Sprintf("%04d-%02d-%02d", req.Year, req.Month, day)
		holidayDesc, isHoliday := holidayMap[dateStr]

		var h float64
		if isWeekend || isHoliday {
			h = writeMasterNonWorkingDay(f, sheetName, r, d, isHoliday, holidayDesc, st)
		} else {
			h = writeMasterWorkingDay(f, sheetName, r, d, entryMap[day], st)
		}
		_ = f.SetRowHeight(sheetName, r, h)
	}

	err = f.SetPageLayout(sheetName, &excelize.PageLayoutOptions{
		Size:        intPtr(9),
		Orientation: stringPtr("landscape"),
	})
	if err != nil {
		log.Printf("Warning: Failed to set page layout: %v", err)
	}

	var excelBuf bytes.Buffer
	if err := f.Write(&excelBuf); err != nil {
		return nil, fmt.Errorf("failed to write Excel data: %w", err)
	}

	return excelBuf.Bytes(), nil
}
