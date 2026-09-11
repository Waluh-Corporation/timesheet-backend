package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

func TestGenerateMII(t *testing.T) {
	if len(assets.MIILogo) == 0 {
		t.Fatal("MII logo asset is empty")
	}

	user := &models.User{Name: "Budi Santoso", Division: "Digital", BniID: "MII-123", Site: "Jakarta"}
	// Day 4 of May 2026 is a Monday (a working day).
	act := models.DailyActivity{
		Date:        time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		StartTime:   "08:00",
		EndTime:     "17:00",
		Status:      "P",
		Activity:    "Sprint work",
		AppImpacted: "cash", // lower-case, should normalize to "Cash"
	}

	in := GenerationInput{
		CompanyCode: "mii",
		User:       user,
		Month:      5,
		Year:       2026,
		Activities: []models.DailyActivity{act},
		Holidays:   map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	const sh = "Sheet1"

	row := 12 // day 4 -> row 9 + 3
	checks := map[string]string{
		"C1":  ": BNI Direct",
		"C3":  ": Budi Santoso",
		"C4":  ": MII-123",
		"E12": "P",
		"K12": "Sprint work",
		"L12": "BNI Direct",
		"M12": "P24015",
		"N12": "Cash",
		"P12": "Wholesale Digital Delivery",
		"Q12": "Wholesale Channel and Service Delivery",
	}
	for cell, want := range checks {
		got, _ := f.GetCellValue(sh, cell)
		if got != want {
			t.Errorf("%s = %q, want %q", cell, got, want)
		}
	}

	// Total-hour column must carry the End-Start formula for the worked day.
	if fx, _ := f.GetCellFormula(sh, "D12"); fx != "C12-B12" {
		t.Errorf("D%d formula = %q, want %q", row, fx, "C12-B12")
	}
}

func TestGenerateMIITrimsShortMonth(t *testing.T) {
	in := GenerationInput{
		CompanyCode: "mii",
		User:       &models.User{Name: "X"},
		Month:      4, // April has 30 days -> day 31 (row 39) must be cleared
		Year:       2026,
		Activities: []models.DailyActivity{},
		Holidays:   map[int]string{},
	}
	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(out))
	defer f.Close()
	for _, cell := range []string{"A39", "B39", "D39", "K39"} {
		if v, _ := f.GetCellValue("Sheet1", cell); v != "" {
			t.Errorf("row 39 (%s) should be trimmed/empty, got %q", cell, v)
		}
	}
}
