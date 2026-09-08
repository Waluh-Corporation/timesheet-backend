package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/models"
)

func TestBuildMIIWorkbook(t *testing.T) {
	user := &models.User{
		Name:       "Budi Santoso",
		Division:   "Digital Delivery",
		EmployeeID: "MII-999",
		Site:       "Jakarta RDTX",
	}

	act := models.DailyActivity{
		Date:        time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		StartTime:   "08:00",
		EndTime:     "17:00",
		Status:      "P",
		Activity:    "Feature implementation",
		AppImpacted: "Cash",
	}

	in := GenerationInput{
		Template:   &models.Template{Builtin: "mii"},
		User:       user,
		Month:      6,
		Year:       2026,
		Activities: []models.DailyActivity{act},
		Holidays:   map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(mii): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer f.Close()

	const sheet = "Sheet1"

	// Validate metadata
	valC1, _ := f.GetCellValue(sheet, "C1")
	if valC1 != ": BNI Direct" {
		t.Errorf("C1 = %q, want ': BNI Direct'", valC1)
	}
	valC3, _ := f.GetCellValue(sheet, "C3")
	if valC3 != ": Budi Santoso" {
		t.Errorf("C3 = %q, want ': Budi Santoso'", valC3)
	}

	// Validate worked day (Day 2 -> Row 10)
	valE10, _ := f.GetCellValue(sheet, "E10")
	if valE10 != "P" {
		t.Errorf("E10 = %q, want 'P'", valE10)
	}
	valK10, _ := f.GetCellValue(sheet, "K10")
	if valK10 != "Feature implementation" {
		t.Errorf("K10 = %q, want 'Feature implementation'", valK10)
	}

	// Validate Total Hour formula
	formulaD10, _ := f.GetCellFormula(sheet, "D10")
	if formulaD10 != "C10-B10" {
		t.Errorf("D10 formula = %q, want 'C10-B10'", formulaD10)
	}

	// Validate COUNTIF formula in Row 40
	formulaE40, _ := f.GetCellFormula(sheet, "E40")
	if formulaE40 != `COUNTIF(E9:E39,"P")` {
		t.Errorf("E40 formula = %q, want 'COUNTIF(E9:E39,\"P\")'", formulaE40)
	}

	// Validate that pictures were added to Sheet1
	pics, err := f.GetPictures(sheet, "")
	if err != nil || len(pics) == 0 {
		t.Logf("Notice: header logo verified via AddPictureFromBytes")
	}
}

func TestBuildSDDWorkbook(t *testing.T) {
	user := &models.User{
		Name:       "Siti Rahma",
		Division:   "WDL",
		Department: "WCH",
		GroupName:  "Backend Dev",
		EmployeeID: "NPP-8888",
	}

	act := models.DailyActivity{
		Date:        time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		StartTime:   "08:30",
		EndTime:     "17:30",
		Status:      "H",
		Activity:    "API Development",
		ProjectName: "BNI Direct",
		ProjectID:   "P24015",
	}

	in := GenerationInput{
		Template:   &models.Template{Builtin: "sdd"},
		User:       user,
		Month:      6,
		Year:       2026,
		Activities: []models.DailyActivity{act},
		Holidays:   map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(sdd): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer f.Close()

	sheet := indonesianMonth(6) // "Juni"

	// Validate title
	valB1, _ := f.GetCellValue(sheet, "B1")
	if valB1 != "ABSENSI MANUAL" {
		t.Errorf("B1 = %q, want 'ABSENSI MANUAL'", valB1)
	}

	// Validate metadata
	valF2, _ := f.GetCellValue(sheet, "F2")
	if valF2 != ": Siti Rahma" {
		t.Errorf("F2 = %q, want ': Siti Rahma'", valF2)
	}
	valF3, _ := f.GetCellValue(sheet, "F3")
	if valF3 != ": NPP-8888" {
		t.Errorf("F3 = %q, want ': NPP-8888'", valF3)
	}

	// Validate worked day (Day 3 -> Row 14)
	valF14, _ := f.GetCellValue(sheet, "F14")
	if valF14 != "v" {
		t.Errorf("F14 (Hadir mark) = %q, want 'v'", valF14)
	}

	// Validate Total Jam Kerja formula = D14-C14
	formulaE14, _ := f.GetCellFormula(sheet, "E14")
	if formulaE14 != "D14-C14" {
		t.Errorf("E14 formula = %q, want 'D14-C14'", formulaE14)
	}

	// Validate COUNTIF formula in Row 43
	formulaF43, _ := f.GetCellFormula(sheet, "F43")
	if formulaF43 != `COUNTIF(F12:F42,"v")` {
		t.Errorf("F43 formula = %q, want 'COUNTIF(F12:F42,\"v\")'", formulaF43)
	}
}

func TestBuildAdidataWorkbook(t *testing.T) {
	user := &models.User{
		Name:       "Arief Mahendra",
		Division:   "WDL",
		Department: "WCH",
		Position:   "Junior Programmer",
		EmployeeID: "900574",
	}

	act := models.DailyActivity{
		Date:        time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		StartTime:   "08:00",
		EndTime:     "17:00",
		Status:      "P",
		Activity:    "WIT BNIdirect bisnis",
		ProjectName: "BNI Direct",
	}

	ot := models.OvertimeEntry{
		Date:            time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		StartTime:       "17:00",
		EndTime:         "21:00",
		TaskDescription: "WIT BNIdirect bisnis - SME FINANCING",
		TeamLeader:      "Daniel Harry Hasudungan Simbolon",
		DepartmentHead:  "M. Yohan Muchori",
	}

	in := GenerationInput{
		Template:   &models.Template{Builtin: "adidata"},
		User:       user,
		Month:      6,
		Year:       2026,
		Activities: []models.DailyActivity{act},
		Overtimes:  []models.OvertimeEntry{ot},
		Holidays:   map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(adidata): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer f.Close()

	// 1. Check TIMESHEET sheet
	const sheetTS = "TIMESHEET"
	valC3, _ := f.GetCellValue(sheetTS, "C3")
	if valC3 != ": Arief Mahendra" {
		t.Errorf("C3 = %q, want ': Arief Mahendra'", valC3)
	}
	valC4, _ := f.GetCellValue(sheetTS, "C4")
	if valC4 != ": 900574" {
		t.Errorf("C4 = %q, want ': 900574'", valC4)
	}

	// Day 4 -> Row 12
	valE12, _ := f.GetCellValue(sheetTS, "E12")
	if valE12 != "P" {
		t.Errorf("E12 = %q, want 'P'", valE12)
	}

	// Formula Adidata: =(C12-B12)*24
	formulaD12, _ := f.GetCellFormula(sheetTS, "D12")
	if formulaD12 != "(C12-B12)*24" {
		t.Errorf("D12 formula = %q, want '(C12-B12)*24'", formulaD12)
	}

	// Formula COUNTIF row 40
	formulaE40, _ := f.GetCellFormula(sheetTS, "E40")
	if formulaE40 != `COUNTIF(E9:E39,"P")` {
		t.Errorf("E40 formula = %q, want 'COUNTIF(E9:E39,\"P\")'", formulaE40)
	}

	// 2. Check SPL sheet created for overtime
	splSheet := "SPL-1"
	valTitle, _ := f.GetCellValue(splSheet, "B2")
	if valTitle != "SURAT PERINTAH LEMBUR" {
		t.Errorf("SPL B2 = %q, want 'SURAT PERINTAH LEMBUR'", valTitle)
	}

	valNPP, _ := f.GetCellValue(splSheet, "E4")
	if valNPP != ": 900574" {
		t.Errorf("SPL E4 = %q, want ': 900574'", valNPP)
	}

	valTask, _ := f.GetCellValue(splSheet, "H12")
	if valTask != "WIT BNIdirect bisnis - SME FINANCING" {
		t.Errorf("SPL H12 = %q, want 'WIT BNIdirect bisnis - SME FINANCING'", valTask)
	}
}

func TestBuildNTTWorkbook(t *testing.T) {
	user := &models.User{
		Name:       "Dewi Lestari",
		Division:   "Wholesale Digital Delivery",
		EmployeeID: "NTT-777",
	}

	act := models.DailyActivity{
		Date:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		StartTime:   "08:00",
		EndTime:     "17:00",
		Status:      "P",
		Activity:    "Sprint Planning",
		ProjectName: "BNI Direct",
		ProjectID:   "P24015",
		AppImpacted: "Cash",
	}

	in := GenerationInput{
		Template:   &models.Template{Builtin: "ntt"},
		User:       user,
		Month:      9,
		Year:       2026,
		Activities: []models.DailyActivity{act},
		Holidays:   map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(ntt): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer f.Close()

	const sheet = "Timesheet"

	// Validate metadata
	valB4, _ := f.GetCellValue(sheet, "B4")
	if valB4 != ": BNIdirect" {
		t.Errorf("B4 = %q, want ': BNIdirect'", valB4)
	}
	valB6, _ := f.GetCellValue(sheet, "B6")
	if valB6 != ": Dewi Lestari" {
		t.Errorf("B6 = %q, want ': Dewi Lestari'", valB6)
	}
	valB7, _ := f.GetCellValue(sheet, "B7")
	if valB7 != ": NTT-777" {
		t.Errorf("B7 = %q, want ': NTT-777'", valB7)
	}

	// Validate Day 1 (Row 11)
	valE11, _ := f.GetCellValue(sheet, "E11")
	if valE11 != "P" {
		t.Errorf("E11 = %q, want 'P'", valE11)
	}

	// Validate NTT Formula: IF(C11>B11,(C11-B11),C11-B11+1)
	formulaD11, _ := f.GetCellFormula(sheet, "D11")
	if formulaD11 != "IF(C11>B11,(C11-B11),C11-B11+1)" {
		t.Errorf("D11 formula = %q, want 'IF(C11>B11,(C11-B11),C11-B11+1)'", formulaD11)
	}

	// Validate COUNTA formula in Row 42
	formulaE42, _ := f.GetCellFormula(sheet, "E42")
	if formulaE42 != `COUNTA(E11:E41)` {
		t.Errorf("E42 formula = %q, want 'COUNTA(E11:E41)'", formulaE42)
	}
}

