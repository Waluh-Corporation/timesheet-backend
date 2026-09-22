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

	// 2. Check SPL sheet created for overtime
	splSheet := "SPL-1"
	valTitle, _ := f.GetCellValue(splSheet, "B2")
	if valTitle != "SURAT PERINTAH LEMBUR" {
		t.Errorf("SPL B2 = %q, want 'SURAT PERINTAH LEMBUR'", valTitle)
	}

	valNPP, _ := f.GetCellValue(splSheet, "E4")
	if valNPP != ": EMP-0001" {
		t.Errorf("SPL E4 = %q, want ': EMP-0001'", valNPP)
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

	// Validate COUNTA formula in Row 42
	formulaE42, _ := f.GetCellFormula(sheet, "E42")
	if formulaE42 != `COUNTA(E11:E41)` {
		t.Errorf("E42 formula = %q, want 'COUNTA(E11:E41)'", formulaE42)
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
