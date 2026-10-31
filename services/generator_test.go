package services

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/models"
)

func TestGeneratorHelpers(t *testing.T) {
	// intPtr and stringPtr
	ip := intPtr(42)
	if ip == nil || *ip != 42 {
		t.Errorf("expected *ip == 42, got %v", ip)
	}

	sp := stringPtr("hello")
	if sp == nil || *sp != "hello" {
		t.Errorf("expected *sp == 'hello', got %v", sp)
	}

	// parseTimeToExcelFraction
	frac, err := parseTimeToExcelFraction("12:00")
	if err != nil {
		t.Fatalf("parseTimeToExcelFraction(12:00) error: %v", err)
	}
	if frac != 0.5 {
		t.Errorf("expected 0.5 for 12:00, got %f", frac)
	}

	_, errInvalid := parseTimeToExcelFraction("invalid")
	if errInvalid == nil {
		t.Errorf("expected error for invalid time string")
	}

	// estimateCellLines
	if lines := estimateCellLines("", 10); lines != 1 {
		t.Errorf("expected 1 line for empty string, got %d", lines)
	}
	if lines := estimateCellLines("short", 20); lines != 1 {
		t.Errorf("expected 1 line for short string, got %d", lines)
	}
	if lines := estimateCellLines("a very very long sentence that wraps across multiple lines in excel", 10); lines < 3 {
		t.Errorf("expected >= 3 lines for long string, got %d", lines)
	}
	if lines := estimateCellLines("line1\n\nline2", 20); lines != 3 {
		t.Errorf("expected 3 lines for multiline with empty segment, got %d", lines)
	}

	// calculateRowHeight
	h1 := calculateRowHeight("Short", "Project", "P01", "App", "Div", "Dept")
	if h1 != 21.0 {
		t.Errorf("expected 21.0 minimum height, got %f", h1)
	}

	h2 := calculateRowHeight("A very long activity description that will wrap across multiple lines because it has lots of detail and explanations", "Project", "P01", "App", "Div", "Dept")
	if h2 < 21.0 {
		t.Errorf("expected height >= 21.0, got %f", h2)
	}
}

func TestGenerateExcel_MasterTemplate(t *testing.T) {
	req := &models.TimesheetRequest{
		Month:             9,
		Year:              2026,
		Format:            "excel",
		Project:           "BNI Direct",
		Division:          "Digital Banking",
		Name:              "John Doe",
		BniID:             "123456",
		Site:              "Jakarta",
		SignatureEmployee: "John Doe",
		SignatureReviewer: "Reviewer Name",
		SignatureApprover: "Approver Name",
		DailyEntries: []models.DailyEntry{
			{
				Day:         1,
				StartTime:   "08:00",
				EndTime:     "17:00",
				Status:      "P",
				Activity:    "Developing APIs",
				ProjectName: "BNI Direct Cash",
				ProjectID:   "P24015",
				AppImpacted: "BNI Mobile",
				Division:    "Digital Banking",
				Department:  "Wholesale",
			},
			{
				Day:         2,
				StartTime:   "",
				EndTime:     "",
				Status:      "S",
				Activity:    "Sick leave",
				ProjectName: "",
				ProjectID:   "",
				AppImpacted: "",
				Division:    "",
				Department:  "",
			},
			{
				Day:         3,
				StartTime:   "",
				EndTime:     "",
				Status:      "X",
				Activity:    "Holiday",
				ProjectName: "",
				ProjectID:   "",
				AppImpacted: "",
				Division:    "",
				Department:  "",
			},
		},
	}

	holidayMap := map[string]string{
		"2026-09-03": "Maulid Nabi",
	}

	excelBytes, err := GenerateExcel(req, holidayMap)
	if err != nil {
		t.Fatalf("GenerateExcel failed: %v", err)
	}
	if len(excelBytes) == 0 {
		t.Fatalf("GenerateExcel returned empty bytes")
	}

	f, err := excelize.OpenReader(bytes.NewReader(excelBytes))
	if err != nil {
		t.Fatalf("OpenReader failed: %v", err)
	}
	defer func() { _ = f.Close() }()

	const sheet = "Sheet1"
	// Day 1 (Row 9): Status "P" -> E9 == "P", J9 == ""
	valE9, _ := f.GetCellValue(sheet, "E9")
	if valE9 != "P" {
		t.Errorf("Day 1 cell E9 = %q, want 'P'", valE9)
	}
	valJ9, _ := f.GetCellValue(sheet, "J9")
	if valJ9 != "" {
		t.Errorf("Day 1 cell J9 = %q, want empty", valJ9)
	}

	// Day 2 (Row 10): Status "S" -> F10 == "S", J10 == ""
	valF10, _ := f.GetCellValue(sheet, "F10")
	if valF10 != "S" {
		t.Errorf("Day 2 cell F10 = %q, want 'S'", valF10)
	}
	valJ10, _ := f.GetCellValue(sheet, "J10")
	if valJ10 != "" {
		t.Errorf("Day 2 cell J10 = %q, want empty", valJ10)
	}

	// Day 3 (Row 11): Status "X", Holiday "Maulid Nabi"
	// Inactive marker J11 must be empty ("") and not "x" or "X"
	valJ11, _ := f.GetCellValue(sheet, "J11")
	if valJ11 == "x" || valJ11 == "X" {
		t.Errorf("Day 3 cell J11 contains %q, but inactive marker must be empty (\"\")", valJ11)
	}
	if valJ11 != "" {
		t.Errorf("Day 3 cell J11 = %q, want empty (\"\")", valJ11)
	}
	valK11, _ := f.GetCellValue(sheet, "K11")
	if valK11 != "Maulid Nabi" {
		t.Errorf("Day 3 remark K11 = %q, want 'Maulid Nabi'", valK11)
	}
	for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
		v, _ := f.GetCellValue(sheet, col+"11")
		if v != "" {
			t.Errorf("Day 3 matrix cell %s11 = %q, want empty", col, v)
		}
	}
}

func TestGenerateExcel_InactiveStatusMarkerEmpty(t *testing.T) {
	req := &models.TimesheetRequest{
		Month:    9,
		Year:     2026,
		Format:   "excel",
		Project:  "BNI Direct",
		Division: "Digital Banking",
		Name:     "John Doe",
		DailyEntries: []models.DailyEntry{
			{
				Day:      2, // Wednesday, normal weekday
				Status:   "X",
				Activity: "",
			},
		},
	}

	excelBytes, err := GenerateExcel(req, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateExcel failed: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(excelBytes))
	if err != nil {
		t.Fatalf("OpenReader failed: %v", err)
	}
	defer func() { _ = f.Close() }()

	const sheet = "Sheet1"
	// Day 2 corresponds to row 10 (startRow 9 + day 2 - 1 = 10)
	valJ, err := f.GetCellValue(sheet, "J10")
	if err != nil {
		t.Fatalf("GetCellValue(J10) error: %v", err)
	}
	if valJ == "x" || valJ == "X" {
		t.Errorf("Day 2 matrix status cell J10 contains %q, want empty (\"\")", valJ)
	}
	if valJ != "" {
		t.Errorf("Day 2 matrix status cell J10 = %q, want empty (\"\")", valJ)
	}

	// Ensure all other status cells on Day 2 are also empty
	for _, col := range []string{"E", "F", "G", "H", "I"} {
		v, _ := f.GetCellValue(sheet, col+"10")
		if v != "" {
			t.Errorf("Day 2 cell %s10 = %q, want empty (\"\")", col, v)
		}
	}
}

func TestGenerateExcel_WeekendAndHolidayOvertimeDynamicStyling(t *testing.T) {
	// September 2026:
	// Day 3 (Thu): Holiday ("Maulid Nabi") without activity -> grey style #D3D3D3, remark "Maulid Nabi", empty hours
	// Day 5 (Sat): Weekend without activity -> grey style #D3D3D3, empty hours, "Weekend" remark
	// Day 6 (Sun): Weekend WITH overtime activity -> clean white style, hours 09:00 - 18:00, status "P", activity populated
	// Day 10 (Thu): Holiday ("Mid-Autumn") WITH overtime activity -> clean white style, hours 08:30 - 17:30, status "P"
	req := &models.TimesheetRequest{
		Month:    9,
		Year:     2026,
		Format:   "excel",
		Project:  "BNI Direct",
		Division: "Digital Banking",
		Name:     "John Tester",
		BniID:    "123456",
		Site:     "Jakarta",
		DailyEntries: []models.DailyEntry{
			{
				Day:         6, // Sunday (weekend) with activity
				StartTime:   "09:00",
				EndTime:     "18:00",
				Status:      "P",
				Activity:    "Weekend Core Migration",
				ProjectName: "BNI Direct Cash",
				ProjectID:   "P24015",
				AppImpacted: "Cash",
				Division:    "Digital Banking",
				Department:  "Wholesale",
			},
			{
				Day:         10, // Holiday with activity
				StartTime:   "08:30",
				EndTime:     "17:30",
				Status:      "P",
				Activity:    "Holiday Production Support",
				ProjectName: "BNI Direct Cash",
				ProjectID:   "P24015",
				AppImpacted: "Cash",
				Division:    "Digital Banking",
				Department:  "Wholesale",
			},
		},
	}

	holidayMap := map[string]string{
		"2026-09-03": "Maulid Nabi",
		"2026-09-10": "Mid-Autumn",
	}

	excelBytes, err := GenerateExcel(req, holidayMap)
	if err != nil {
		t.Fatalf("GenerateExcel failed: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(excelBytes))
	if err != nil {
		t.Fatalf("OpenReader failed: %v", err)
	}
	defer func() { _ = f.Close() }()

	const sheet = "Sheet1"

	// 1. Day 3 (Thu, Row 11): Holiday without activity
	isGrey3, err := isCellGrey(f, sheet, "B11")
	if err != nil || !isGrey3 {
		t.Errorf("Day 3 row 11 should be grey #D3D3D3, got isGrey=%v, err=%v", isGrey3, err)
	}
	valB11, _ := f.GetCellValue(sheet, "B11")
	valC11, _ := f.GetCellValue(sheet, "C11")
	valK11, _ := f.GetCellValue(sheet, "K11")
	if valB11 != "" || valC11 != "" {
		t.Errorf("Day 3 working hours should be empty, got B11=%q, C11=%q", valB11, valC11)
	}
	if valK11 != "Maulid Nabi" {
		t.Errorf("Day 3 remark K11 = %q, want 'Maulid Nabi'", valK11)
	}
	for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
		v, _ := f.GetCellValue(sheet, col+"11")
		if v != "" {
			t.Errorf("Day 3 matrix cell %s11 = %q, want empty", col, v)
		}
	}

	// 2. Day 5 (Sat, Row 13): Weekend without activity
	isGrey5, err := isCellGrey(f, sheet, "B13")
	if err != nil || !isGrey5 {
		t.Errorf("Day 5 row 13 should be grey #D3D3D3, got isGrey=%v, err=%v", isGrey5, err)
	}
	valB13, _ := f.GetCellValue(sheet, "B13")
	valC13, _ := f.GetCellValue(sheet, "C13")
	valK13, _ := f.GetCellValue(sheet, "K13")
	if valB13 != "" || valC13 != "" {
		t.Errorf("Day 5 working hours should be empty, got B13=%q, C13=%q", valB13, valC13)
	}
	if valK13 != "Weekend" {
		t.Errorf("Day 5 remark K13 = %q, want 'Weekend'", valK13)
	}
	for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
		v, _ := f.GetCellValue(sheet, col+"13")
		if v != "" {
			t.Errorf("Day 5 matrix cell %s13 = %q, want empty", col, v)
		}
	}

	// 3. Day 6 (Sun, Row 14): Weekend WITH overtime activity
	isGrey6, err := isCellGrey(f, sheet, "B14")
	if err != nil || isGrey6 {
		t.Errorf("Day 6 row 14 should NOT be grey, got isGrey=%v, err=%v", isGrey6, err)
	}
	valB14, _ := f.GetCellValue(sheet, "B14")
	valC14, _ := f.GetCellValue(sheet, "C14")
	valE14, _ := f.GetCellValue(sheet, "E14")
	valJ14, _ := f.GetCellValue(sheet, "J14")
	valK14, _ := f.GetCellValue(sheet, "K14")
	valL14, _ := f.GetCellValue(sheet, "L14")
	if valB14 != "09:00" || valC14 != "18:00" {
		t.Errorf("Day 6 working hours got B14=%q, C14=%q, want 09:00, 18:00", valB14, valC14)
	}
	if valE14 != "P" {
		t.Errorf("Day 6 status E14 = %q, want 'P'", valE14)
	}
	if valJ14 != "" {
		t.Errorf("Day 6 status J14 = %q, want empty inactive status cell", valJ14)
	}
	if valK14 != "Weekend Core Migration" {
		t.Errorf("Day 6 activity K14 = %q, want 'Weekend Core Migration'", valK14)
	}
	if valL14 != "BNI Direct Cash" {
		t.Errorf("Day 6 project L14 = %q, want 'BNI Direct Cash'", valL14)
	}

	// 4. Day 10 (Thu, Row 18): Holiday WITH overtime activity
	isGrey10, err := isCellGrey(f, sheet, "B18")
	if err != nil || isGrey10 {
		t.Errorf("Day 10 row 18 should NOT be grey, got isGrey=%v, err=%v", isGrey10, err)
	}
	valB18, _ := f.GetCellValue(sheet, "B18")
	valC18, _ := f.GetCellValue(sheet, "C18")
	valE18, _ := f.GetCellValue(sheet, "E18")
	valJ18, _ := f.GetCellValue(sheet, "J18")
	valK18, _ := f.GetCellValue(sheet, "K18")
	if valB18 != "08:30" || valC18 != "17:30" {
		t.Errorf("Day 10 working hours got B18=%q, C18=%q, want 08:30, 17:30", valB18, valC18)
	}
	if valE18 != "P" {
		t.Errorf("Day 10 status E18 = %q, want 'P'", valE18)
	}
	if valJ18 != "" {
		t.Errorf("Day 10 status J18 = %q, want empty inactive status cell", valJ18)
	}
	if valK18 != "Holiday Production Support" {
		t.Errorf("Day 10 activity K18 = %q, want 'Holiday Production Support'", valK18)
	}
}
