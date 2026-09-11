package services

import (
	"os"
	"path/filepath"
	"testing"

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
	if h1 != 15.0 {
		t.Errorf("expected 15.0 minimum height, got %f", h1)
	}

	h2 := calculateRowHeight("A very long activity description that will wrap across multiple lines because it has lots of detail and explanations", "Project", "P01", "App", "Div", "Dept")
	if h2 < 15.0 {
		t.Errorf("expected height >= 15.0, got %f", h2)
	}
}

func TestGenerateExcel_MasterTemplate(t *testing.T) {
	// Locate template path relative to working directory or project root
	origTmpl := os.Getenv("TEMPLATE_PATH")
	defer func() {
		if origTmpl != "" {
			_ = os.Setenv("TEMPLATE_PATH", origTmpl)
		} else {
			_ = os.Unsetenv("TEMPLATE_PATH")
		}
	}()

	// If running from services package directory, template is at ../templates/master_template.xlsx
	tmplPath := "templates/master_template.xlsx"
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		tmplPath = filepath.Join("..", "templates", "master_template.xlsx")
	}
	_ = os.Setenv("TEMPLATE_PATH", tmplPath)

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
}
