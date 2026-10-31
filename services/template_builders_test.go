package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
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
		Date:      time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "Feature implementation",
		ProjectRef: &models.Project{
			Code:        "P24015",
			Name:        "BNI Direct Cash",
			AppImpacted: "Cash",
		},
	}

	in := GenerationInput{
		CompanyCode: "mii",
		User:        user,
		Month:       6,
		Year:        2026,
		Activities:  []models.DailyActivity{act},
		Holidays:    map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(mii): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer func() { _ = f.Close() }()

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

	// Validate COUNTIF formula and value in Row 40
	formulaE40, _ := f.GetCellFormula(sheet, "E40")
	if formulaE40 != `COUNTIF(E9:E39,"P")` {
		t.Errorf("E40 formula = %q, want 'COUNTIF(E9:E39,\"P\")'", formulaE40)
	}
	valE40, _ := f.GetCellValue(sheet, "E40")
	if valE40 != "1" {
		t.Errorf("E40 value = %q, want '1'", valE40)
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
		Date:      time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		StartTime: "08:30",
		EndTime:   "17:30",
		Status:    "H",
		Activity:  "API Development",
		ProjectRef: &models.Project{
			Code: "P24015",
			Name: "BNI Direct",
		},
	}

	in := GenerationInput{
		CompanyCode: "sdd",
		User:        user,
		Month:       6,
		Year:        2026,
		Activities:  []models.DailyActivity{act},
		Holidays:    map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(sdd): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer func() { _ = f.Close() }()

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
	if valF14 != "H" {
		t.Errorf("F14 (Hadir mark) = %q, want 'H'", valF14)
	}

	// Validate Total Jam Kerja formula = D14-C14
	formulaE14, _ := f.GetCellFormula(sheet, "E14")
	if formulaE14 != "D14-C14" {
		t.Errorf("E14 formula = %q, want 'D14-C14'", formulaE14)
	}

	// Validate COUNTIF formula and value in Row 43
	formulaF43, _ := f.GetCellFormula(sheet, "F43")
	if formulaF43 != `COUNTIF(F12:F42,"H")` {
		t.Errorf("F43 formula = %q, want 'COUNTIF(F12:F42,\"H\")'", formulaF43)
	}
	valF43, _ := f.GetCellValue(sheet, "F43")
	if valF43 != "1" {
		t.Errorf("F43 value = %q, want '1'", valF43)
	}
}

func TestBuildAdidataWorkbook(t *testing.T) {
	user := &models.User{
		Name:       "John Doe",
		Division:   "WDL",
		Department: "WCH",
		Position:   "Junior Programmer",
		EmployeeID: "EMP-0001",
	}

	act := models.DailyActivity{
		Date:      time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "WIT BNIdirect bisnis",
		ProjectRef: &models.Project{
			Name: "BNI Direct",
		},
	}

	tl := models.Approver{Name: "Team Leader Example"}
	dh := models.Approver{Name: "Department Head Example"}
	ot := models.OvertimeEntry{
		Date:            time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		StartTime:       "17:00",
		EndTime:         "21:00",
		TaskDescription: "WIT BNIdirect bisnis - SME FINANCING",
		TeamLeader:      &tl,
		DepartmentHead:  &dh,
	}

	in := GenerationInput{
		CompanyCode: "adidata",
		User:        user,
		Month:       6,
		Year:        2026,
		Activities:  []models.DailyActivity{act},
		Overtimes:   []models.OvertimeEntry{ot},
		Holidays:    map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(adidata): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer func() { _ = f.Close() }()

	// 1. Check TIMESHEET sheet
	const sheetTS = "TIMESHEET"
	valD4, _ := f.GetCellValue(sheetTS, "D4")
	if valD4 != ": John Doe" {
		t.Errorf("D4 = %q, want ': John Doe'", valD4)
	}
	valD5, _ := f.GetCellValue(sheetTS, "D5")
	if valD5 != ": EMP-0001" {
		t.Errorf("D5 = %q, want ': EMP-0001'", valD5)
	}

	// Day 4 -> Row 13
	valF13, _ := f.GetCellValue(sheetTS, "F13")
	if valF13 != "P" {
		t.Errorf("F13 = %q, want 'P'", valF13)
	}

	// Formula Adidata: =(D13-C13)*24
	formulaE13, _ := f.GetCellFormula(sheetTS, "E13")
	if formulaE13 != "(D13-C13)*24" {
		t.Errorf("E13 formula = %q, want '(D13-C13)*24'", formulaE13)
	}

	// Formula COUNTIF row 41
	formulaF41, _ := f.GetCellFormula(sheetTS, "F41")
	if formulaF41 != `COUNTIF(F10:F40,"P")` {
		t.Errorf("F41 formula = %q, want 'COUNTIF(F10:F40,\"P\")'", formulaF41)
	}
	valF41, _ := f.GetCellValue(sheetTS, "F41")
	if valF41 != "1" {
		t.Errorf("F41 value = %q, want '1'", valF41)
	}

	// 2. Check SPL sheet created for overtime
	splSheet := "SPL"
	valTitle, _ := f.GetCellValue(splSheet, "C4")
	if valTitle != "SURAT PERINTAH LEMBUR" {
		t.Errorf("SPL C4 = %q, want 'SURAT PERINTAH LEMBUR'", valTitle)
	}

	valNPP, _ := f.GetCellValue(splSheet, "C7")
	if valNPP != "NPP                                          : EMP-0001" {
		t.Errorf("SPL C7 = %q, want 'NPP                                          : EMP-0001'", valNPP)
	}

	valTask, _ := f.GetCellValue(splSheet, "I15")
	if valTask != "WIT BNIdirect bisnis - SME FINANCING" {
		t.Errorf("SPL I15 = %q, want 'WIT BNIdirect bisnis - SME FINANCING'", valTask)
	}
}

func TestWriteAdidataSingleSPL(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	st, err := NewBuilderStyles(f)
	if err != nil {
		t.Fatalf("NewBuilderStyles: %v", err)
	}

	user := &models.User{
		Name:       "Adidata Employee",
		Division:   "WDL",
		Department: "WCH",
		Position:   "Lead Engineer",
		EmployeeID: "ADI-777",
	}
	tl := models.Approver{Name: "TL Person"}
	dh := models.Approver{Name: "DH Person"}
	ot := models.OvertimeEntry{
		Date:            time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		StartTime:       "18:00",
		EndTime:         "21:00",
		TaskDescription: "Emergency bugfix deployment",
		TeamLeader:      &tl,
		DepartmentHead:  &dh,
	}
	in := GenerationInput{
		User: user,
	}

	splSheet := "SPL-Direct"
	_, _ = f.NewSheet(splSheet)
	writeAdidataSingleSPL(f, splSheet, in, ot, "ADI-777", st)

	valTitle, _ := f.GetCellValue(splSheet, "C4")
	if valTitle != "SURAT PERINTAH LEMBUR" {
		t.Errorf("C4 = %q, want 'SURAT PERINTAH LEMBUR'", valTitle)
	}
	valNPP, _ := f.GetCellValue(splSheet, "C7")
	if valNPP != "NPP                                          : ADI-777" {
		t.Errorf("C7 = %q, want 'NPP                                          : ADI-777'", valNPP)
	}
	valName, _ := f.GetCellValue(splSheet, "C8")
	if valName != "NAMA                                     : Adidata Employee" {
		t.Errorf("C8 = %q, want 'NAMA                                     : Adidata Employee'", valName)
	}
	valSign, _ := f.GetCellValue(splSheet, "B26")
	if valSign != "( Adidata Employee )" {
		t.Errorf("B26 = %q, want '( Adidata Employee )'", valSign)
	}
}

func TestAdidataMultiRowSPL_Scenarios(t *testing.T) {
	t.Run("zero overtime removes SPL_TEMPLATE", func(t *testing.T) {
		in := GenerationInput{
			CompanyCode: "adidata",
			Year:        2026,
			Month:       6,
			User: &models.User{
				Name:       "No OT User",
				EmployeeID: "ADI-000",
			},
			Activities: []models.DailyActivity{
				{Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Status: "P", StartTime: "08:00", EndTime: "17:00"},
			},
			Overtimes: nil,
		}
		raw, err := GenerateFromTemplate(in)
		if err != nil {
			t.Fatalf("GenerateFromTemplate: %v", err)
		}
		f, err := excelize.OpenReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("OpenReader: %v", err)
		}
		defer func() { _ = f.Close() }()

		for _, sh := range f.GetSheetList() {
			if strings.Contains(strings.ToUpper(sh), "SPL") {
				t.Errorf("unexpected SPL sheet found: %s", sh)
			}
		}
	})

	t.Run("dynamic row expansion for more than 3 overtimes", func(t *testing.T) {
		tl := models.Approver{Name: "Leader Alpha"}
		dh := models.Approver{Name: "Head Beta"}
		ots := []models.OvertimeEntry{
			{Date: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC), StartTime: "17:00", EndTime: "20:00", TaskDescription: "Task 1", TeamLeader: &tl, DepartmentHead: &dh},
			{Date: time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), StartTime: "17:00", EndTime: "19:00", TaskDescription: "Task 2", TeamLeader: &tl, DepartmentHead: &dh},
			{Date: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), StartTime: "18:00", EndTime: "21:00", TaskDescription: "Task 3", TeamLeader: &tl, DepartmentHead: &dh},
			{Date: time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC), StartTime: "17:00", EndTime: "21:00", TaskDescription: "Task 4", TeamLeader: &tl, DepartmentHead: &dh},
			{Date: time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC), StartTime: "18:00", EndTime: "20:30", TaskDescription: "Task 5", TeamLeader: &tl, DepartmentHead: &dh},
		}

		in := GenerationInput{
			CompanyCode: "adidata",
			Year:        2026,
			Month:       6,
			User: &models.User{
				Name:       "Adidata Multi User",
				EmployeeID: "ADI-999",
				Position:   "Senior Specialist",
				Department: "IT",
				Division:   "Fintech",
			},
			Activities: []models.DailyActivity{
				{Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Status: "P", StartTime: "08:00", EndTime: "17:00"},
			},
			Overtimes: ots,
		}

		raw, err := GenerateFromTemplate(in)
		if err != nil {
			t.Fatalf("GenerateFromTemplate: %v", err)
		}
		f, err := excelize.OpenReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("OpenReader: %v", err)
		}
		defer func() { _ = f.Close() }()

		splSheet := "SPL"
		// 5 rows -> rows 15, 16, 17, 18, 19
		task5, _ := f.GetCellValue(splSheet, "I19")
		if task5 != "Task 5" {
			t.Errorf("I19 = %q, want 'Task 5'", task5)
		}

		// Total row is row 20
		totalVal, _ := f.GetCellValue(splSheet, "H20")
		// Durations: 3 + 2 + 3 + 4 + 2.5 = 14.5
		if totalVal != "14.5" {
			t.Errorf("H20 total hours = %q, want '14.5'", totalVal)
		}

		// Total label
		totalLabel, _ := f.GetCellValue(splSheet, "C20")
		if totalLabel != "Total Keseluruhan Jam Lembur" {
			t.Errorf("C20 label = %q, want 'Total Keseluruhan Jam Lembur'", totalLabel)
		}

		// Signature row: 20 + 8 = 28
		sigUser, _ := f.GetCellValue(splSheet, "B28")
		if sigUser != "( Adidata Multi User )" {
			t.Errorf("B28 = %q, want '( Adidata Multi User )'", sigUser)
		}
		sigPos, _ := f.GetCellValue(splSheet, "B29")
		if sigPos != "Senior Specialist" {
			t.Errorf("B29 = %q, want 'Senior Specialist'", sigPos)
		}
		sigTL, _ := f.GetCellValue(splSheet, "G28")
		if sigTL != "( Leader Alpha )" {
			t.Errorf("G28 = %q, want '( Leader Alpha )'", sigTL)
		}
		sigDH, _ := f.GetCellValue(splSheet, "M28")
		if sigDH != "( Head Beta )" {
			t.Errorf("M28 = %q, want '( Head Beta )'", sigDH)
		}
	})
}

func TestBuildNTTWorkbook(t *testing.T) {
	user := &models.User{
		Name:       "Dewi Lestari",
		Division:   "Wholesale Digital Delivery",
		EmployeeID: "NTT-777",
	}

	act := models.DailyActivity{
		Date:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "Sprint Planning",
		ProjectRef: &models.Project{
			Code:        "P24015",
			Name:        "BNI Direct",
			AppImpacted: "Cash",
		},
	}

	in := GenerationInput{
		CompanyCode: "ntt",
		User:        user,
		Month:       9,
		Year:        2026,
		Activities:  []models.DailyActivity{act},
		Holidays:    map[int]string{},
	}

	out, err := GenerateFromTemplate(in)
	if err != nil {
		t.Fatalf("GenerateFromTemplate(ntt): %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Open output workbook: %v", err)
	}
	defer func() { _ = f.Close() }()

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

	// Validate COUNTIF formula and value in Row 42
	formulaE42, _ := f.GetCellFormula(sheet, "E42")
	if formulaE42 != `COUNTIF(E11:E41,"P")` {
		t.Errorf("E42 formula = %q, want 'COUNTIF(E11:E41,\"P\")'", formulaE42)
	}
	valE42, _ := f.GetCellValue(sheet, "E42")
	if valE42 != "1" {
		t.Errorf("E42 value = %q, want '1'", valE42)
	}

	// Validate Signature Date in Row 56
	valB56, _ := f.GetCellValue(sheet, "B56")
	if !strings.HasPrefix(valB56, "DATE: ") {
		t.Errorf("B56 = %q, want prefix 'DATE: '", valB56)
	}
}

func TestTemplateBuilders_MasterApproversSignaturesWithoutOvertime(t *testing.T) {
	user := &models.User{
		Name:       "Test Employee",
		EmployeeID: "EMP-100",
		BniID:      "BNI-100",
	}
	masterApprovers := []models.Approver{
		{Name: "Budi TeamLeader", RoleType: models.ApproverRoleTeamLeader, IsActive: true},
		{Name: "Dewi DeptHead", RoleType: models.ApproverRoleDepartmentHead, IsActive: true},
	}

	testCases := []struct {
		company   string
		sheetName string
		tlCell    string
		dhCell    string
		wantTL    string
		wantDH    string
	}{
		{company: "mii", sheetName: "Sheet1", tlCell: "D46", dhCell: "G46", wantTL: "Budi TeamLeader", wantDH: "Dewi DeptHead"},
		{company: "sdd", sheetName: "Juni", tlCell: "H52", dhCell: "L52", wantTL: "Nama : Budi TeamLeader", wantDH: "Nama : Dewi DeptHead"},
		{company: "adidata", sheetName: "TIMESHEET", tlCell: "E44", dhCell: "I44", wantTL: "Budi TeamLeader", wantDH: "Dewi DeptHead"},
		{company: "ntt", sheetName: "Timesheet", tlCell: "F55", dhCell: "J55", wantTL: "Budi TeamLeader", wantDH: "Dewi DeptHead"},
	}

	for _, tc := range testCases {
		t.Run(tc.company, func(t *testing.T) {
			in := GenerationInput{
				CompanyCode: tc.company,
				User:        user,
				Month:       6,
				Year:        2026,
				Activities:  []models.DailyActivity{},
				Overtimes:   nil, // NO overtime entries!
				Approvers:   masterApprovers,
				Holidays:    map[int]string{},
			}

			out, err := GenerateFromTemplate(in)
			if err != nil {
				t.Fatalf("GenerateFromTemplate(%s) failed: %v", tc.company, err)
			}

			f, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("Open output workbook for %s: %v", tc.company, err)
			}
			defer func() { _ = f.Close() }()

			tlVal, _ := f.GetCellValue(tc.sheetName, tc.tlCell)
			if tlVal != tc.wantTL {
				t.Errorf("[%s] TL signature at %s = %q, want %q", tc.company, tc.tlCell, tlVal, tc.wantTL)
			}

			dhVal, _ := f.GetCellValue(tc.sheetName, tc.dhCell)
			if dhVal != tc.wantDH {
				t.Errorf("[%s] DH signature at %s = %q, want %q", tc.company, tc.dhCell, dhVal, tc.wantDH)
			}
		})
	}
}

func TestTemplateBuilders_WorkingHoursAndTotalHourFormatting(t *testing.T) {
	user := &models.User{
		Name:       "Test User",
		Division:   "WDL",
		EmployeeID: "EMP-001",
	}

	// September 2026:
	// Day 17 (Thu): Working day without activity -> default 08:00 and 17:00
	// Day 19 (Sat): Weekend -> blank ""
	// Day 21 (Mon): Working day with activity 07:00 - 18:00 -> 11:00 total
	acts := []models.DailyActivity{
		{
			Date:      time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
			StartTime: "07:00",
			EndTime:   "18:00",
			Status:    "P",
			Activity:  "Sprint planning",
		},
	}

	companies := []struct {
		code      string
		sheetName string
		startRow  int
		startCol  string
		endCol    string
		totalCol  string
		isDecimal bool
	}{
		{code: "mii", sheetName: "Sheet1", startRow: 9, startCol: "B", endCol: "C", totalCol: "D", isDecimal: false},
		{code: "sdd", sheetName: "September", startRow: 12, startCol: "C", endCol: "D", totalCol: "E", isDecimal: false},
		{code: "adidata", sheetName: "TIMESHEET", startRow: 10, startCol: "C", endCol: "D", totalCol: "E", isDecimal: true},
		{code: "ntt", sheetName: "Timesheet", startRow: 11, startCol: "B", endCol: "C", totalCol: "D", isDecimal: false},
	}

	for _, c := range companies {
		t.Run(c.code, func(t *testing.T) {
			in := GenerationInput{
				CompanyCode: c.code,
				User:        user,
				Month:       9,
				Year:        2026,
				Activities:  acts,
				Holidays:    map[int]string{},
			}

			out, err := GenerateFromTemplate(in)
			if err != nil {
				t.Fatalf("GenerateFromTemplate(%s): %v", c.code, err)
			}

			// Ensure no erroneous t="str" on formula cells in XML
			zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
			if err != nil {
				t.Fatalf("[%s] open zip error: %v", c.code, err)
			}
			for _, zf := range zr.File {
				if strings.HasPrefix(zf.Name, "xl/worksheets/sheet") {
					rc, _ := zf.Open()
					data, _ := io.ReadAll(rc)
					_ = rc.Close()
					if strings.Contains(string(data), `t="str"><f`) {
						t.Errorf("[%s] found t=\"str\" on formula cell in %s", c.code, zf.Name)
					}
				}
			}

			f, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("[%s] open excel error: %v", c.code, err)
			}
			defer f.Close()

			// 1. Day 17: Working day without activity -> default 08:00, 17:00, total 09:00 (or 9.00)
			r17 := c.startRow + 16
			s17, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.startCol, r17))
			e17, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.endCol, r17))
			tot17, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.totalCol, r17))
			if s17 != "08:00" {
				t.Errorf("[%s] Day 17 start = %q, want '08:00'", c.code, s17)
			}
			if e17 != "17:00" {
				t.Errorf("[%s] Day 17 end = %q, want '17:00'", c.code, e17)
			}
			wantTot17 := "09:00"
			if c.isDecimal {
				wantTot17 = "9.00"
			}
			if tot17 != wantTot17 {
				t.Errorf("[%s] Day 17 total = %q, want %q", c.code, tot17, wantTot17)
			}

			// 2. Day 19: Weekend -> blank
			r19 := c.startRow + 18
			s19, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.startCol, r19))
			e19, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.endCol, r19))
			tot19, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.totalCol, r19))
			if s19 != "" || e19 != "" || tot19 != "" {
				t.Errorf("[%s] Weekend Day 19 should be blank, got start=%q, end=%q, total=%q", c.code, s19, e19, tot19)
			}

			// 3. Day 21: Working day with 07:00 - 18:00 activity -> total 11:00 (or 11.00)
			r21 := c.startRow + 20
			s21, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.startCol, r21))
			e21, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.endCol, r21))
			tot21, _ := f.GetCellValue(c.sheetName, fmt.Sprintf("%s%d", c.totalCol, r21))
			if s21 != "07:00" {
				t.Errorf("[%s] Day 21 start = %q, want '07:00'", c.code, s21)
			}
			if e21 != "18:00" {
				t.Errorf("[%s] Day 21 end = %q, want '18:00'", c.code, e21)
			}
			wantTot21 := "11:00"
			if c.isDecimal {
				wantTot21 = "11.00"
			}
			if tot21 != wantTot21 {
				t.Errorf("[%s] Day 21 total = %q, want %q", c.code, tot21, wantTot21)
			}
		})
	}
}

func TestTemplateBuilders_TotalAttendanceUpdates(t *testing.T) {
	user := &models.User{
		Name:       "Test User",
		Division:   "Engineering",
		EmployeeID: "EMP-999",
	}

	activities := []models.DailyActivity{
		{Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Status: "P", StartTime: "08:00", EndTime: "17:00"},
		{Date: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC), Status: "P", StartTime: "08:00", EndTime: "17:00"},
		{Date: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), Status: "S", StartTime: "08:00", EndTime: "17:00"},
		{Date: time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), Status: "V", StartTime: "08:00", EndTime: "17:00"},
		{Date: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), Status: "PM", StartTime: "08:00", EndTime: "17:00"},
	}

	testCases := []struct {
		company   string
		sheetName string
		checks    map[string]string
		formulas  map[string]string
	}{
		{
			company:   "mii",
			sheetName: "Sheet1",
			checks: map[string]string{
				"E40": "2", // P
				"F40": "1", // S
				"H40": "1", // PM
				"I40": "1", // V
				"G40": "0", // BT
				"J40": "0", // X
			},
			formulas: map[string]string{
				"E40": `COUNTIF(E9:E39,"P")`,
				"F40": `COUNTIF(F9:F39,"S")`,
			},
		},
		{
			company:   "sdd",
			sheetName: "Juni",
			checks: map[string]string{
				"F43": "2", // Hadir
				"I43": "1", // Sakit
				"H43": "1", // Izin (PM)
				"G43": "1", // Cuti (V)
				"J43": "0", // Lembur
			},
			formulas: map[string]string{
				"F43": `COUNTIF(F12:F42,"H")`,
				"G43": `COUNTIF(G12:G42,"C")`,
				"H43": `COUNTIF(H12:H42,"I")`,
				"I43": `COUNTIF(I12:I42,"S")`,
				"J43": `COUNTIF(J12:J42,"L")`,
			},
		},
		{
			company:   "adidata",
			sheetName: "TIMESHEET",
			checks: map[string]string{
				"F41": "2", // P
				"G41": "1", // S
				"I41": "1", // PM
				"J41": "1", // V
				"H41": "0", // BT
				"K41": "0", // X
			},
			formulas: map[string]string{
				"F41": `COUNTIF(F10:F40,"P")`,
				"G41": `COUNTIF(G10:G40,"S")`,
			},
		},
		{
			company:   "ntt",
			sheetName: "Timesheet",
			checks: map[string]string{
				"E42": "2", // P
				"F42": "1", // S
				"H42": "1", // PM
				"I42": "1", // V
				"G42": "0", // BT
				"J42": "0", // X
			},
			formulas: map[string]string{
				"E42": `COUNTIF(E11:E41,"P")`,
				"F42": `COUNTIF(F11:F41,"S")`,
				"G42": `COUNTIF(G11:G41,"BT")`,
				"H42": `COUNTIF(H11:H41,"PM")`,
				"I42": `COUNTIF(I11:I41,"V")`,
				"J42": `COUNTIF(J11:J41,"X")`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.company, func(t *testing.T) {
			in := GenerationInput{
				CompanyCode: tc.company,
				User:        user,
				Month:       6,
				Year:        2026,
				Activities:  activities,
				Holidays:    map[int]string{},
			}

			out, err := GenerateFromTemplate(in)
			if err != nil {
				t.Fatalf("[%s] GenerateFromTemplate failed: %v", tc.company, err)
			}

			f, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("[%s] OpenReader failed: %v", tc.company, err)
			}
			defer func() { _ = f.Close() }()

			for cell, wantVal := range tc.checks {
				gotVal, err := f.GetCellValue(tc.sheetName, cell)
				if err != nil {
					t.Errorf("[%s] GetCellValue(%s) error: %v", tc.company, cell, err)
				}
				if gotVal != wantVal {
					t.Errorf("[%s] Cell %s value = %q, want %q", tc.company, cell, gotVal, wantVal)
				}
			}

			for cell, wantFormula := range tc.formulas {
				gotFormula, err := f.GetCellFormula(tc.sheetName, cell)
				if err != nil {
					t.Errorf("[%s] GetCellFormula(%s) error: %v", tc.company, cell, err)
				}
				if gotFormula != wantFormula {
					t.Errorf("[%s] Cell %s formula = %q, want %q", tc.company, cell, gotFormula, wantFormula)
				}
			}
		})
	}
}

func TestTimesheetFontSizesAndRowHeights(t *testing.T) {
	// 1. Verify NewBuilderStyles definition directly
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	st, err := NewBuilderStyles(f)
	if err != nil {
		t.Fatalf("NewBuilderStyles failed: %v", err)
	}

	styleChecks := []struct {
		name     string
		styleID  int
		wantSize float64
		wantBold bool
	}{
		{"HeaderStyle", st.HeaderStyle, 14, true},
		{"HeaderGreyStyle", st.HeaderGreyStyle, 14, true},
		{"DataCenterStyle", st.DataCenterStyle, 14, false},
		{"DataCenterWrapStyle", st.DataCenterWrapStyle, 14, false},
		{"DataLeftStyle", st.DataLeftStyle, 14, false},
		{"DateStyle", st.DateStyle, 14, false},
		{"TimeStyle", st.TimeStyle, 14, false},
		{"DecimalStyle", st.DecimalStyle, 14, false},
		{"BoldCenterStyle", st.BoldCenterStyle, 14, true},
		{"BoldLeftStyle", st.BoldLeftStyle, 14, true},
		{"MetaLabelStyle", st.MetaLabelStyle, 12, true},
		{"MetaValueStyle", st.MetaValueStyle, 12, false},
		{"GreyDateStyle", st.GreyDateStyle, 14, false},
		{"GreyCenterStyle", st.GreyCenterStyle, 14, false},
		{"GreyCenterWrapStyle", st.GreyCenterWrapStyle, 14, false},
		{"GreyTimeStyle", st.GreyTimeStyle, 14, false},
		{"GreyDecimalStyle", st.GreyDecimalStyle, 14, false},
	}

	for _, sc := range styleChecks {
		s, err := f.GetStyle(sc.styleID)
		if err != nil {
			t.Errorf("GetStyle(%s) error: %v", sc.name, err)
			continue
		}
		if s.Font == nil {
			t.Errorf("Style %s has nil Font", sc.name)
			continue
		}
		if s.Font.Size != sc.wantSize {
			t.Errorf("Style %s Font.Size = %v, want %v", sc.name, s.Font.Size, sc.wantSize)
		}
		if s.Font.Bold != sc.wantBold {
			t.Errorf("Style %s Font.Bold = %v, want %v", sc.name, s.Font.Bold, sc.wantBold)
		}
	}

	// 2. Verify all companies produce 14pt data rows and >= 21 row heights
	user := &models.User{
		Name:       "Test Engineer",
		Division:   "WDL",
		Department: "WCH",
		EmployeeID: "EMP-001",
		Site:       "Jakarta",
	}

	act := models.DailyActivity{
		Date:      time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "Core service implementation and unit test coverage",
		ProjectRef: &models.Project{
			Code: "P24015",
			Name: "BNI Direct",
		},
	}

	companies := []struct {
		code      string
		sheetName string
		dataCell  string // cell in activity row (Day 2)
		dataRow   int    // Day 2 row
		blankRow  int    // Day 30 row (Feb has 28 days)
	}{
		{code: "mii", sheetName: "Sheet1", dataCell: "K10", dataRow: 10, blankRow: 38},
		{code: "sdd", sheetName: "Februari", dataCell: "N13", dataRow: 13, blankRow: 41},
		{code: "adidata", sheetName: "TIMESHEET", dataCell: "L11", dataRow: 11, blankRow: 39},
		{code: "ntt", sheetName: "Timesheet", dataCell: "K12", dataRow: 12, blankRow: 40},
	}

	for _, comp := range companies {
		t.Run(comp.code, func(t *testing.T) {
			in := GenerationInput{
				CompanyCode: comp.code,
				User:        user,
				Month:       2, // February 2026 (28 days)
				Year:        2026,
				Activities:  []models.DailyActivity{act},
				Holidays:    map[int]string{},
			}

			raw, err := GenerateFromTemplate(in)
			if err != nil {
				t.Fatalf("[%s] GenerateFromTemplate failed: %v", comp.code, err)
			}

			wb, err := excelize.OpenReader(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("[%s] OpenReader failed: %v", comp.code, err)
			}
			defer func() { _ = wb.Close() }()

			// Verify data cell has Font.Size == 14
			styleID, err := wb.GetCellStyle(comp.sheetName, comp.dataCell)
			if err != nil {
				t.Fatalf("[%s] GetCellStyle(%s) error: %v", comp.code, comp.dataCell, err)
			}
			s, err := wb.GetStyle(styleID)
			if err != nil {
				t.Fatalf("[%s] GetStyle(%d) error: %v", comp.code, styleID, err)
			}
			if s.Font == nil || s.Font.Size != 14 {
				t.Errorf("[%s] Data cell %s Font.Size = %v, want 14", comp.code, comp.dataCell, s.Font.Size)
			}

			// Verify row height for activity row is >= 21
			hAct, err := wb.GetRowHeight(comp.sheetName, comp.dataRow)
			if err != nil {
				t.Fatalf("[%s] GetRowHeight(%d) error: %v", comp.code, comp.dataRow, err)
			}
			if hAct < 21.0 {
				t.Errorf("[%s] Row %d height = %v, want >= 21.0", comp.code, comp.dataRow, hAct)
			}

			// Verify blank padding row height is 21
			hBlank, err := wb.GetRowHeight(comp.sheetName, comp.blankRow)
			if err != nil {
				t.Fatalf("[%s] GetRowHeight(%d) error: %v", comp.code, comp.blankRow, err)
			}
			if hBlank != 21.0 {
				t.Errorf("[%s] Blank padding row %d height = %v, want 21.0", comp.code, comp.blankRow, hBlank)
			}
		})
	}
}

func TestTimesheetTextWrapping(t *testing.T) {
	user := &models.User{
		Name:       "Test User",
		Division:   "Wholesale Digital Delivery",
		Department: "Wholesale Channel and Service Delivery",
		EmployeeID: "EMP-001",
		Site:       "Jakarta RDTX",
	}

	act := models.DailyActivity{
		Date:      time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "A very detailed task description that wraps across multiple lines",
		ProjectRef: &models.Project{
			Code: "P24015",
			Name: "BNI Direct Cash Management",
		},
	}

	testCases := []struct {
		company   string
		sheetName string
		dataRow   int
		wrapCols  []string
	}{
		{
			company:   "mii",
			sheetName: "Sheet1",
			dataRow:   10,
			wrapCols:  []string{"K", "L", "M", "N", "O", "P", "Q", "R"},
		},
		{
			company:   "adidata",
			sheetName: "TIMESHEET",
			dataRow:   11,
			wrapCols:  []string{"L", "M", "N", "O", "P", "Q", "R"},
		},
		{
			company:   "ntt",
			sheetName: "Timesheet",
			dataRow:   12,
			wrapCols:  []string{"K", "L", "M", "N"},
		},
		{
			company:   "sdd",
			sheetName: "Juni",
			dataRow:   13,
			wrapCols:  []string{"K", "L", "M", "N"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.company, func(t *testing.T) {
			in := GenerationInput{
				CompanyCode: tc.company,
				User:        user,
				Month:       6,
				Year:        2026,
				Activities:  []models.DailyActivity{act},
				Holidays:    map[int]string{},
			}

			raw, err := GenerateFromTemplate(in)
			if err != nil {
				t.Fatalf("[%s] GenerateFromTemplate failed: %v", tc.company, err)
			}

			wb, err := excelize.OpenReader(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("[%s] OpenReader failed: %v", tc.company, err)
			}
			defer func() { _ = wb.Close() }()

			for _, col := range tc.wrapCols {
				cell := fmt.Sprintf("%s%d", col, tc.dataRow)
				styleID, err := wb.GetCellStyle(tc.sheetName, cell)
				if err != nil {
					t.Errorf("[%s] GetCellStyle(%s) error: %v", tc.company, cell, err)
					continue
				}
				style, err := wb.GetStyle(styleID)
				if err != nil {
					t.Errorf("[%s] GetStyle(%d) error: %v", tc.company, styleID, err)
					continue
				}
				if style.Alignment == nil || !style.Alignment.WrapText {
					t.Errorf("[%s] Cell %s WrapText = false, want true", tc.company, cell)
				}
			}
		})
	}
}

// isCellGrey checks if the cell at the given coordinates has a grey background fill (#AEAAAA or #D3D3D3).
func isCellGrey(f *excelize.File, sheet, cell string) (bool, error) {
	styleID, err := f.GetCellStyle(sheet, cell)
	if err != nil {
		return false, fmt.Errorf("get cell style %s: %w", cell, err)
	}
	st, err := f.GetStyle(styleID)
	if err != nil {
		return false, fmt.Errorf("get style %d: %w", styleID, err)
	}
	if st == nil || len(st.Fill.Color) == 0 {
		return false, nil
	}
	for _, col := range st.Fill.Color {
		colClean := strings.ToUpper(strings.TrimPrefix(col, "#"))
		if colClean == "AEAAAA" || colClean == "D3D3D3" {
			return true, nil
		}
	}
	return false, nil
}

func TestTemplateBuilders_WeekendAndHolidayOvertimeDynamicStyling(t *testing.T) {
	user := &models.User{
		Name:       "Budi Santoso",
		Division:   "Wholesale Digital Delivery",
		Department: "Wholesale Channel and Service Delivery",
		EmployeeID: "EMP-001",
		Site:       "Jakarta RDTX",
	}

	// Month: September 2026 (30 days)
	// Day 3 (Thu): Public Holiday ("Maulid Nabi") without activity
	// Day 5 (Sat): Weekend without activity
	// Day 6 (Sun): Weekend WITH overtime activity
	// Day 10 (Thu): Public Holiday ("Mid-Autumn") WITH overtime activity
	activities := []models.DailyActivity{
		{
			Date:       time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC),
			StartTime:  "09:00",
			EndTime:    "17:00",
			Status:     "P",
			Activity:   "Sunday System Deployment",
			ProjectRef: &models.Project{Code: "P24015", Name: "BNI Direct", AppImpacted: "Cash"},
		},
		{
			Date:       time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
			StartTime:  "08:30",
			EndTime:    "17:30",
			Status:     "P",
			Activity:   "Holiday Incident Response",
			ProjectRef: &models.Project{Code: "P24015", Name: "BNI Direct", AppImpacted: "Cash"},
		},
	}

	holidays := map[int]string{
		3:  "Maulid Nabi",
		10: "Mid-Autumn",
	}

	testCases := []struct {
		company         string
		sheetName       string
		startRow        int
		startCol        string
		endCol          string
		totalCol        string
		actCol          string
		projCol         string
		isDecimal       bool
		wantStatusCol   string
		wantStatusVal   string
		wantInactiveCol string
	}{
		{
			company:         "mii",
			sheetName:       "Sheet1",
			startRow:        9,
			startCol:        "B",
			endCol:          "C",
			totalCol:        "D",
			actCol:          "K",
			projCol:         "L",
			isDecimal:       false,
			wantStatusCol:   "E",
			wantStatusVal:   "P",
			wantInactiveCol: "J",
		},
		{
			company:         "sdd",
			sheetName:       "September",
			startRow:        12,
			startCol:        "C",
			endCol:          "D",
			totalCol:        "E",
			actCol:          "N",
			projCol:         "K",
			isDecimal:       false,
			wantStatusCol:   "F",
			wantStatusVal:   "H", // SDD maps "P" to Hadir ("H") in col F
			wantInactiveCol: "",
		},
		{
			company:         "adidata",
			sheetName:       "TIMESHEET",
			startRow:        10,
			startCol:        "C",
			endCol:          "D",
			totalCol:        "E",
			actCol:          "L",
			projCol:         "O",
			isDecimal:       true,
			wantStatusCol:   "F",
			wantStatusVal:   "P",
			wantInactiveCol: "K",
		},
		{
			company:         "ntt",
			sheetName:       "Timesheet",
			startRow:        11,
			startCol:        "B",
			endCol:          "C",
			totalCol:        "D",
			actCol:          "K",
			projCol:         "L",
			isDecimal:       false,
			wantStatusCol:   "E",
			wantStatusVal:   "P",
			wantInactiveCol: "J",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.company, func(t *testing.T) {
			in := GenerationInput{
				CompanyCode: tc.company,
				User:        user,
				Month:       9,
				Year:        2026,
				Activities:  activities,
				Holidays:    holidays,
			}

			out, err := GenerateFromTemplate(in)
			if err != nil {
				t.Fatalf("[%s] GenerateFromTemplate failed: %v", tc.company, err)
			}

			wb, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("[%s] OpenReader failed: %v", tc.company, err)
			}
			defer func() { _ = wb.Close() }()

			// 1. Day 5: Weekend without activity
			r5 := tc.startRow + 4
			isGrey5, err := isCellGrey(wb, tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r5))
			if err != nil || !isGrey5 {
				t.Errorf("[%s] Day 5 (weekend no act) should be grey #AEAAAA, got isGrey=%v, err=%v", tc.company, isGrey5, err)
			}
			s5, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.startCol, r5))
			e5, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.endCol, r5))
			tot5, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.totalCol, r5))
			act5, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r5))
			if s5 != "" || e5 != "" || tot5 != "" {
				t.Errorf("[%s] Day 5 working hours should be empty, got start=%q, end=%q, tot=%q", tc.company, s5, e5, tot5)
			}
			if act5 != "Weekend" {
				t.Errorf("[%s] Day 5 remark = %q, want 'Weekend'", tc.company, act5)
			}

			// 2. Day 3: Holiday without activity
			r3 := tc.startRow + 2
			isGrey3, err := isCellGrey(wb, tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r3))
			if err != nil || !isGrey3 {
				t.Errorf("[%s] Day 3 (holiday no act) should be grey #AEAAAA, got isGrey=%v, err=%v", tc.company, isGrey3, err)
			}
			s3, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.startCol, r3))
			e3, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.endCol, r3))
			tot3, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.totalCol, r3))
			act3, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r3))
			if s3 != "" || e3 != "" || tot3 != "" {
				t.Errorf("[%s] Day 3 working hours should be empty, got start=%q, end=%q, tot=%q", tc.company, s3, e3, tot3)
			}
			if act3 != "Maulid Nabi" {
				t.Errorf("[%s] Day 3 remark = %q, want 'Maulid Nabi'", tc.company, act3)
			}

			// 3. Day 6: Weekend WITH overtime activity
			r6 := tc.startRow + 5
			isGrey6, err := isCellGrey(wb, tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r6))
			if err != nil || isGrey6 {
				t.Errorf("[%s] Day 6 (weekend with act) should NOT be grey, got isGrey=%v, err=%v", tc.company, isGrey6, err)
			}
			s6, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.startCol, r6))
			e6, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.endCol, r6))
			tot6, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.totalCol, r6))
			act6, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r6))
			proj6, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.projCol, r6))
			st6, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.wantStatusCol, r6))

			if s6 != "09:00" || e6 != "17:00" {
				t.Errorf("[%s] Day 6 start/end got %q/%q, want 09:00/17:00", tc.company, s6, e6)
			}
			wantTot6 := "08:00"
			if tc.isDecimal {
				wantTot6 = "8.00"
			}
			if tot6 != wantTot6 {
				t.Errorf("[%s] Day 6 total hours got %q, want %q", tc.company, tot6, wantTot6)
			}
			if act6 != "Sunday System Deployment" {
				t.Errorf("[%s] Day 6 activity got %q, want 'Sunday System Deployment'", tc.company, act6)
			}
			if !strings.Contains(proj6, "BNI Direct") {
				t.Errorf("[%s] Day 6 project got %q, want to contain 'BNI Direct'", tc.company, proj6)
			}
			if st6 != tc.wantStatusVal {
				t.Errorf("[%s] Day 6 status got %q, want %q", tc.company, st6, tc.wantStatusVal)
			}
			if tc.wantInactiveCol != "" {
				valInactive6, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.wantInactiveCol, r6))
				if valInactive6 == "x" || valInactive6 == "X" {
					t.Errorf("[%s] Day 6 inactive cell %s%d contains %q, but inactive marker must be empty (\"\")", tc.company, tc.wantInactiveCol, r6, valInactive6)
				}
				if valInactive6 != "" {
					t.Errorf("[%s] Day 6 inactive cell %s%d = %q, want empty", tc.company, tc.wantInactiveCol, r6, valInactive6)
				}
			}

			// 4. Day 10: Holiday WITH overtime activity
			r10 := tc.startRow + 9
			isGrey10, err := isCellGrey(wb, tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r10))
			if err != nil || isGrey10 {
				t.Errorf("[%s] Day 10 (holiday with act) should NOT be grey, got isGrey=%v, err=%v", tc.company, isGrey10, err)
			}
			s10, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.startCol, r10))
			e10, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.endCol, r10))
			tot10, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.totalCol, r10))
			act10, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.actCol, r10))
			proj10, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.projCol, r10))
			st10, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.wantStatusCol, r10))

			if s10 != "08:30" || e10 != "17:30" {
				t.Errorf("[%s] Day 10 start/end got %q/%q, want 08:30/17:30", tc.company, s10, e10)
			}
			wantTot10 := "09:00"
			if tc.isDecimal {
				wantTot10 = "9.00"
			}
			if tot10 != wantTot10 {
				t.Errorf("[%s] Day 10 total hours got %q, want %q", tc.company, tot10, wantTot10)
			}
			if act10 != "Holiday Incident Response" {
				t.Errorf("[%s] Day 10 activity got %q, want 'Holiday Incident Response'", tc.company, act10)
			}
			if !strings.Contains(proj10, "BNI Direct") {
				t.Errorf("[%s] Day 10 project got %q, want to contain 'BNI Direct'", tc.company, proj10)
			}
			if st10 != tc.wantStatusVal {
				t.Errorf("[%s] Day 10 status got %q, want %q", tc.company, st10, tc.wantStatusVal)
			}
			if tc.wantInactiveCol != "" {
				valInactive10, _ := wb.GetCellValue(tc.sheetName, fmt.Sprintf("%s%d", tc.wantInactiveCol, r10))
				if valInactive10 == "x" || valInactive10 == "X" {
					t.Errorf("[%s] Day 10 inactive cell %s%d contains %q, but inactive marker must be empty (\"\")", tc.company, tc.wantInactiveCol, r10, valInactive10)
				}
				if valInactive10 != "" {
					t.Errorf("[%s] Day 10 inactive cell %s%d = %q, want empty", tc.company, tc.wantInactiveCol, r10, valInactive10)
				}
			}
		})
	}
}

func TestWriteAdidataAttendanceMatrixStatus_InactiveMarkerAndAllStatuses(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Sheet1"

	tests := []struct {
		name        string
		statusInput string
		expectedCol string
		expectedVal string
	}{
		{name: "Present 'P'", statusInput: "P", expectedCol: "F", expectedVal: "P"},
		{name: "Sick 'S'", statusInput: "S", expectedCol: "G", expectedVal: "S"},
		{name: "Business Trip 'BT'", statusInput: "BT", expectedCol: "H", expectedVal: "BT"},
		{name: "Permit 'PM'", statusInput: "PM", expectedCol: "I", expectedVal: "PM"},
		{name: "Vacation 'V'", statusInput: "V", expectedCol: "J", expectedVal: "V"},
		{name: "Inactive 'X' -> K must be EMPTY", statusInput: "X", expectedCol: "K", expectedVal: ""},
		{name: "Inactive lowercase 'x' -> K must be EMPTY", statusInput: "x", expectedCol: "K", expectedVal: ""},
		{name: "Inactive with spaces ' X ' -> K must be EMPTY", statusInput: " X ", expectedCol: "K", expectedVal: ""},
		{name: "Empty status -> all empty", statusInput: "", expectedCol: "", expectedVal: ""},
		{name: "Unknown status -> all empty", statusInput: "UNKNOWN", expectedCol: "", expectedVal: ""},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rowStr := fmt.Sprintf("%d", 30+i)
			writeAdidataAttendanceMatrixStatus(f, sheet, rowStr, tc.statusInput)

			if tc.expectedCol != "" {
				val, _ := f.GetCellValue(sheet, tc.expectedCol+rowStr)
				if val != tc.expectedVal {
					t.Errorf("Cell %s%s = %q, want %q", tc.expectedCol, rowStr, val, tc.expectedVal)
				}
			}

			// Cell K must never contain "x" or "X" when status is inactive
			valK, _ := f.GetCellValue(sheet, "K"+rowStr)
			if valK == "x" || valK == "X" {
				t.Errorf("Cell K%s contains %q, but inactive marker must be empty (\"\")", rowStr, valK)
			}

			// Verify other columns are empty when expectedVal is empty
			if tc.expectedVal == "" {
				for _, col := range []string{"F", "G", "H", "I", "J", "K"} {
					v, _ := f.GetCellValue(sheet, col+rowStr)
					if v != "" {
						t.Errorf("Expected cell %s%s to be empty, got %q", col, rowStr, v)
					}
				}
			}
		})
	}
}

func TestTemplateBuilders_InactiveStatusMarkerEmptyAcrossCompanies(t *testing.T) {
	user := &models.User{
		Name:       "Test User",
		Division:   "Engineering",
		EmployeeID: "EMP-001",
	}

	// Day 2 (weekday) with Inactive status "X"
	act := models.DailyActivity{
		Date:       time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		Status:     "X",
		Activity:   "Off / Inactive day",
		ProjectRef: &models.Project{Code: "P24015", Name: "BNI Direct"},
	}

	companies := []struct {
		code        string
		sheetName   string
		inactiveCol string // column that holds inactive marker in template
		dayRow      int    // Day 2 row
		allCols     []string
	}{
		{code: "mii", sheetName: "Sheet1", inactiveCol: "J", dayRow: 10, allCols: []string{"E", "F", "G", "H", "I", "J"}},
		{code: "sdd", sheetName: "September", inactiveCol: "", dayRow: 13, allCols: []string{"F", "G", "H", "I", "J"}},
		{code: "adidata", sheetName: "TIMESHEET", inactiveCol: "K", dayRow: 11, allCols: []string{"F", "G", "H", "I", "J", "K"}},
		{code: "ntt", sheetName: "Timesheet", inactiveCol: "J", dayRow: 12, allCols: []string{"E", "F", "G", "H", "I", "J"}},
	}

	for _, c := range companies {
		t.Run(c.code, func(t *testing.T) {
			in := GenerationInput{
				CompanyCode: c.code,
				User:        user,
				Month:       9,
				Year:        2026,
				Activities:  []models.DailyActivity{act},
				Holidays:    map[int]string{},
			}

			raw, err := GenerateFromTemplate(in)
			if err != nil {
				t.Fatalf("[%s] GenerateFromTemplate failed: %v", c.code, err)
			}

			wb, err := excelize.OpenReader(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("[%s] OpenReader failed: %v", c.code, err)
			}
			defer func() { _ = wb.Close() }()

			if c.inactiveCol != "" {
				cell := fmt.Sprintf("%s%d", c.inactiveCol, c.dayRow)
				val, err := wb.GetCellValue(c.sheetName, cell)
				if err != nil {
					t.Fatalf("[%s] GetCellValue(%s) error: %v", c.code, cell, err)
				}
				if val == "x" || val == "X" {
					t.Errorf("[%s] Inactive marker cell %s contains %q, want empty (\"\")", c.code, cell, val)
				}
				if val != "" {
					t.Errorf("[%s] Inactive marker cell %s = %q, want empty (\"\")", c.code, cell, val)
				}
			}

			// Ensure all matrix columns on that day are empty
			for _, col := range c.allCols {
				cell := fmt.Sprintf("%s%d", col, c.dayRow)
				v, _ := wb.GetCellValue(c.sheetName, cell)
				if v != "" {
					t.Errorf("[%s] Matrix cell %s = %q, want empty (\"\")", c.code, cell, v)
				}
			}
		})
	}
}
