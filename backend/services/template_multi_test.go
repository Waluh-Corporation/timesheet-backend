package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

func TestGenerateSDD(t *testing.T) {
	if len(assets.SDDTemplate) == 0 {
		t.Fatal("SDD template asset is empty")
	}

	user := &models.User{
		Name:     "Faisal Al Munawar",
		Division: "Wholesale Digital Delivery",
		MiiID:    "10000500",
		Site:     "Jakarta",
	}

	// 2026-06-02 is day 2 (row 12 + 1 = 13)
	act := models.DailyActivity{
		Date:        time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		StartTime:   "08:00",
		EndTime:     "17:00",
		Status:      "P",
		Activity:    "Backend API Development",
		ProjectName: "BNIdirect",
		ProjectID:   "P24015",
	}

	in := GenerationInput{
		Template:   &models.Template{Builtin: "sdd", FileData: assets.SDDTemplate},
		User:       user,
		Month:      6,
		Year:       2026,
		Activities: []models.DailyActivity{act},
		Holidays:   map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate (sdd): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("open sdd output: %v", err)
	}
	defer f.Close()

	sh := f.GetSheetName(0)

	// Verify Header
	if got, _ := f.GetCellValue(sh, "G2"); got != "Faisal Al Munawar" {
		t.Errorf("G2 (name) = %q, want %q", got, "Faisal Al Munawar")
	}
	if got, _ := f.GetCellValue(sh, "G3"); got != "10000500" {
		t.Errorf("G3 (npp) = %q, want %q", got, "10000500")
	}
	if got, _ := f.GetCellValue(sh, "G4"); got != "Wholesale Digital Delivery" {
		t.Errorf("G4 (div) = %q, want %q", got, "Wholesale Digital Delivery")
	}


	if got, _ := f.GetCellValue(sh, "F13"); got != "v" {
		t.Errorf("F13 (hadir check) = %q, want 'v'", got)
	}
	if got, _ := f.GetCellValue(sh, "K13"); got != "BNIdirect" {
		t.Errorf("K13 (proj) = %q, want 'BNIdirect'", got)
	}
	if got, _ := f.GetCellValue(sh, "N13"); got != "Backend API Development" {
		t.Errorf("N13 (activity) = %q, want 'Backend API Development'", got)
	}
}

func TestGenerateAdidata(t *testing.T) {
	if len(assets.AdidataTemplate) == 0 {
		t.Fatal("Adidata template asset is empty")
	}

	user := &models.User{
		Name:     "Arief Mahendra",
		Division: "BDD / WDL",
		MiiID:    "900574",
		Site:     "BNI / Jakarta",
	}

	// 2026-06-02 is day 2 (row 9 + 1 = 10)
	act := models.DailyActivity{
		Date:        time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		StartTime:   "08:00",
		EndTime:     "17:00",
		Status:      "P",
		Activity:    "Frontend UI",
		ProjectName: "PT BANK NEGARA INDONESIA (PERSERO) Tbk",
		ProjectID:   "P24015",
		AppImpacted: "Bisnis",
	}

	in := GenerationInput{
		Template:   &models.Template{Builtin: "adidata", FileData: assets.AdidataTemplate},
		User:       user,
		Month:      6,
		Year:       2026,
		Activities: []models.DailyActivity{act},
		Holidays:   map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate (adidata): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("open adidata output: %v", err)
	}
	defer f.Close()

	sh := "TIMESHEET"

	// Verify Header
	if got, _ := f.GetCellValue(sh, "C3"); got != ": Arief Mahendra" {
		t.Errorf("C3 (name) = %q, want ': Arief Mahendra'", got)
	}
	if got, _ := f.GetCellValue(sh, "C4"); got != ": 900574" {
		t.Errorf("C4 (npp) = %q, want ': 900574'", got)
	}

	// Verify Day 2 row (row 10)
	if got, _ := f.GetCellValue(sh, "E10"); got != "P" {
		t.Errorf("E10 (status) = %q, want 'P'", got)
	}
	if got, _ := f.GetCellValue(sh, "K10"); got != "Frontend UI" {
		t.Errorf("K10 (activity) = %q, want 'Frontend UI'", got)
	}
	if got, _ := f.GetCellValue(sh, "N10"); got != "Bisnis" {
		t.Errorf("N10 (app) = %q, want 'Bisnis'", got)
	}
	if fx, _ := f.GetCellFormula(sh, "D10"); fx != "(C10-B10)*24" {
		t.Errorf("D10 (formula) = %q, want '(C10-B10)*24'", fx)
	}
}
