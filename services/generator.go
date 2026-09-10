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
func GenerateExcel(req *models.TimesheetRequest, holidayMap map[string]string) ([]byte, error) {
	// Open spreadsheet template
	templatePath := os.Getenv("TEMPLATE_PATH")
	if templatePath == "" {
		templatePath = "templates/master_template.xlsx"
	}

	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel template: %w", err)
	}
	defer f.Close()

	// Set document metadata properties
	_ = f.SetDocProps(&excelize.DocProperties{
		Creator:        "FAFR & Zaid © 2026",
		LastModifiedBy: "FAFR & Zaid © 2026",
	})

	sheetName := "Sheet1"

	// 1. Populate header metadata (C1-C6)
	if req.Project != "" {
		_ = f.SetCellValue(sheetName, "C1", req.Project)
	}
	if req.Division != "" {
		_ = f.SetCellValue(sheetName, "C2", req.Division)
	}
	if req.Name != "" {
		_ = f.SetCellValue(sheetName, "C3", req.Name)
	}
	if req.MiiID != "" {
		_ = f.SetCellValue(sheetName, "C4", req.MiiID)
	}
	if req.Site != "" {
		_ = f.SetCellValue(sheetName, "C5", req.Site)
	}

	// Write Period into C6 (e.g. YYYY-MM-01)
	periodStr := fmt.Sprintf("%d-%02d-01", req.Year, req.Month)
	_ = f.SetCellValue(sheetName, "C6", periodStr)

	// Style header input cells to highly readable standard Arial, size 11
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
	})
	if err == nil {
		for i := 1; i <= 6; i++ {
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("C%d", i), fmt.Sprintf("C%d", i), headerStyle)
		}
	}

	// 2. Calculate exact days in the month
	daysInMonth := GetDaysInMonth(req.Year, req.Month)

	// 3. Row Trimming (CRITICAL)
	// Delete excess rows in the bottom of the grid (rows 37-39 for short months)
	// To prevent indices shifting, we delete starting from the bottom row index (39) down to (9 + daysInMonth)
	for r := 39; r >= 9+daysInMonth; r-- {
		if err := f.RemoveRow(sheetName, r); err != nil {
			return nil, fmt.Errorf("failed to remove excess row: %w", err)
		}
	}

	// Rewrite attendance summation formulas dynamically to prevent circular references in short months
	sumRow := 9 + daysInMonth
	_ = f.SetCellFormula(sheetName, fmt.Sprintf("E%d", sumRow), fmt.Sprintf("COUNTIF(E9:E%d,\"P\")", 8+daysInMonth))
	_ = f.SetCellFormula(sheetName, fmt.Sprintf("F%d", sumRow), fmt.Sprintf("COUNTIF(F9:F%d,\"S\")", 8+daysInMonth))
	_ = f.SetCellFormula(sheetName, fmt.Sprintf("G%d", sumRow), fmt.Sprintf("COUNTIF(G9:G%d,\"BT\")", 8+daysInMonth))
	_ = f.SetCellFormula(sheetName, fmt.Sprintf("H%d", sumRow), fmt.Sprintf("COUNTIF(H9:H%d,\"PM\")", 8+daysInMonth))
	_ = f.SetCellFormula(sheetName, fmt.Sprintf("I%d", sumRow), fmt.Sprintf("COUNTIF(I9:I%d,\"V\")", 8+daysInMonth))
	_ = f.SetCellFormula(sheetName, fmt.Sprintf("J%d", sumRow), fmt.Sprintf("COUNTIF(J9:J%d,\"x\")", 8+daysInMonth))

	// Write signature blocks (A43, D43, G43 shifted up to finalRow)
	finalRow := 12 + daysInMonth
	sigStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "bottom",
			WrapText:   true,
		},
	})
	if err == nil {
		if req.SignatureEmployee != "" {
			_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", finalRow), req.SignatureEmployee)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("A%d", finalRow), fmt.Sprintf("C%d", finalRow+3), sigStyle)
		}
		if req.SignatureReviewer != "" {
			_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", finalRow), req.SignatureReviewer)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("D%d", finalRow), fmt.Sprintf("F%d", finalRow+3), sigStyle)
		}
		if req.SignatureApprover != "" {
			_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", finalRow), req.SignatureApprover)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("G%d", finalRow), fmt.Sprintf("J%d", finalRow+3), sigStyle)
		}
	}

	// 4. Define Excel styles
	// Soft gray fill style for weekends and holidays (#D3D3D3)
	grayStyle, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D3D3D3"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical:   "center",
			Horizontal: "center",
			WrapText:   true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate gray Excel styling: %w", err)
	}

	// Gray Date Style for Column A on weekends/holidays (keeps date number format intact)
	grayDateStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("d-m-yy"),
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D3D3D3"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical:   "center",
			Horizontal: "center",
		},
	})

	// Date style for Column A on working days
	dateStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("d-m-yy"),
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical:   "center",
			Horizontal: "center",
		},
	})

	// Time style for Columns B, C, D on working days
	timeStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("hh:mm"),
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical:   "center",
			Horizontal: "center",
		},
	})

	// Active Center Style for Columns E-J on working days
	activeStyleCenter, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical:   "center",
			Horizontal: "center",
		},
	})

	// Active Centered Style for Columns K-Q (Text columns) on working days
	activeStyleLeft, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Family: "Arial",
			Size:   11,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{
			Vertical:   "center",
			Horizontal: "center",
			WrapText:   true,
		},
	})

	// 5. Inject Data & Apply styles row by row
	for day := 1; day <= daysInMonth; day++ {
		r := 8 + day
		d := time.Date(req.Year, time.Month(req.Month), day, 0, 0, 0, 0, time.UTC)
		isWeekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday

		dateStr := fmt.Sprintf("%04d-%02d-%02d", req.Year, req.Month, day)
		holidayDesc, isHoliday := holidayMap[dateStr]
		var h float64 = 15.0

		if isWeekend || isHoliday {
			// Apply soft gray background across all fillable columns (B to Q)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("B%d", r), fmt.Sprintf("Q%d", r), grayStyle)
			// Apply gray date style to A (Date)
			_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", r), d)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), grayDateStyle)

			// Clear time columns (B and C)
			_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", r), "")
			_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", r), "")

			// Clear attendance columns (E-J)
			for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
				_ = f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, r), "")
			}

			if isHoliday {
				// Automatically insert the holiday's name into the Activity/Remark cell (Column K)
				_ = f.SetCellValue(sheetName, fmt.Sprintf("K%d", r), holidayDesc)
				// Clear other text columns
				for _, col := range []string{"L", "M", "N", "O", "P", "Q"} {
					_ = f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, r), "")
				}
				h = calculateRowHeight(holidayDesc, "", "", "", "", "")
			} else {
				// Clear all text columns for weekend
				for _, col := range []string{"K", "L", "M", "N", "O", "P", "Q"} {
					_ = f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, r), "")
				}
				h = 15.0
			}
		} else {
			// Working day: search for entry
			var entry *models.DailyEntry
			for i := range req.DailyEntries {
				if req.DailyEntries[i].Day == day {
					entry = &req.DailyEntries[i]
					break
				}
			}

			// Write date and format Column A
			_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", r), d)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), dateStyle)

			if entry != nil {
				// Inject time values
				if entry.StartTime != "" && entry.StartTime != "00:00" {
					startVal, err := parseTimeToExcelFraction(entry.StartTime)
					if err == nil {
						_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", r), startVal)
					} else {
						_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", r), entry.StartTime)
					}
				} else {
					_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", r), "")
				}

				if entry.EndTime != "" && entry.EndTime != "00:00" {
					endVal, err := parseTimeToExcelFraction(entry.EndTime)
					if err == nil {
						_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", r), endVal)
					} else {
						_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", r), entry.EndTime)
					}
				} else {
					_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", r), "")
				}

				// Clear and set status
				for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
					_ = f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, r), "")
				}

				statusUpper := strings.ToUpper(entry.Status)
				switch statusUpper {
				case "P":
					_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", r), "P")
				case "S":
					_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", r), "S")
				case "BT":
					_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", r), "BT")
				case "PM":
					_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", r), "PM")
				case "V":
					_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", r), "V")
				case "X":
					_ = f.SetCellValue(sheetName, fmt.Sprintf("J%d", r), "x")
				}

				// Set text columns
				_ = f.SetCellValue(sheetName, fmt.Sprintf("K%d", r), entry.Activity)
				_ = f.SetCellValue(sheetName, fmt.Sprintf("L%d", r), entry.ProjectName)
				_ = f.SetCellValue(sheetName, fmt.Sprintf("M%d", r), entry.ProjectID)
				_ = f.SetCellValue(sheetName, fmt.Sprintf("N%d", r), entry.AppImpacted)
				_ = f.SetCellValue(sheetName, fmt.Sprintf("O%d", r), "") // AIP Fitur
				_ = f.SetCellValue(sheetName, fmt.Sprintf("P%d", r), entry.Division)
				_ = f.SetCellValue(sheetName, fmt.Sprintf("Q%d", r), entry.Department)

				h = calculateRowHeight(entry.Activity, entry.ProjectName, entry.ProjectID, entry.AppImpacted, entry.Division, entry.Department)
			} else {
				// Clear fields if no entry was found for this working day
				_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", r), "")
				_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", r), "")
				for _, col := range []string{"E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q"} {
					_ = f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, r), "")
				}
				h = 15.0
			}

			// Format styles for working days
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("B%d", r), fmt.Sprintf("C%d", r), timeStyle)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("D%d", r), fmt.Sprintf("D%d", r), timeStyle)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("E%d", r), fmt.Sprintf("J%d", r), activeStyleCenter)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("K%d", r), fmt.Sprintf("Q%d", r), activeStyleLeft)
		}

		_ = f.SetRowHeight(sheetName, r, h)
	}

	// 6. Readability & Print Formatting
	// Set Page Layout: A4 Paper (PaperSize: 9) and Landscape orientation
	err = f.SetPageLayout(sheetName, &excelize.PageLayoutOptions{
		Size:        intPtr(9),              // A4 paper size
		Orientation: stringPtr("landscape"), // Landscape orientation
	})
	if err != nil {
		log.Printf("Warning: Failed to set page layout: %v", err)
	}

	// Save modified spreadsheet to buffer
	var excelBuf bytes.Buffer
	if err := f.Write(&excelBuf); err != nil {
		return nil, fmt.Errorf("failed to write Excel data: %w", err)
	}

	return excelBuf.Bytes(), nil
}
