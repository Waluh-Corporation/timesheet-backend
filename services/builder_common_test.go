package services

import (
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/models"
)

func TestBuilderCommon_WriteSignaturesBlock(t *testing.T) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	st, err := NewBuilderStyles(f)
	if err != nil {
		t.Fatalf("failed to create builder styles: %v", err)
	}

	WriteSignaturesBlock(f, sheet, 40, "Alice", "Bob (TL)", "Charlie (DH)", st)

	valPrep, _ := f.GetCellValue(sheet, "A40")
	if valPrep != "Prepared by :" {
		t.Errorf("expected 'Prepared by :', got %q", valPrep)
	}
	valName, _ := f.GetCellValue(sheet, "A44")
	if valName != "Alice" {
		t.Errorf("expected 'Alice', got %q", valName)
	}
	valTL, _ := f.GetCellValue(sheet, "D44")
	if valTL != "Bob (TL)" {
		t.Errorf("expected 'Bob (TL)', got %q", valTL)
	}
	valDH, _ := f.GetCellValue(sheet, "G44")
	if valDH != "Charlie (DH)" {
		t.Errorf("expected 'Charlie (DH)', got %q", valDH)
	}
}

func TestBuilderCommon_WriteTotalSummaryRow(t *testing.T) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	st, err := NewBuilderStyles(f)
	if err != nil {
		t.Fatalf("failed to create builder styles: %v", err)
	}

	statusCols := map[string]string{
		"V": "E",
		"X": "F",
	}

	WriteTotalSummaryRow(f, sheet, 16, 10, 15, statusCols, st)

	label, _ := f.GetCellValue(sheet, "A16")
	if label != "TOTAL" {
		t.Errorf("expected label 'TOTAL', got %q", label)
	}
}

func TestBuilderCommon_ExtractApprovers(t *testing.T) {
	t.Run("empty overtimes", func(t *testing.T) {
		tl, dh := ExtractApprovers(nil)
		if tl != "" || dh != "" {
			t.Errorf("expected empty approvers, got tl=%q dh=%q", tl, dh)
		}
	})

	t.Run("with approvers", func(t *testing.T) {
		overtimes := []models.OvertimeEntry{
			{
				TeamLeader:     &models.Approver{Name: "Leader Bob"},
				DepartmentHead: &models.Approver{Name: "Head Alice"},
			},
		}
		tl, dh := ExtractApprovers(overtimes)
		if tl != "Leader Bob" {
			t.Errorf("expected tl 'Leader Bob', got %q", tl)
		}
		if dh != "Head Alice" {
			t.Errorf("expected dh 'Head Alice', got %q", dh)
		}
	})
}

func TestBuilderCommon_ResolveApprovers(t *testing.T) {
	masterApprovers := []models.Approver{
		{Name: "Master TL", RoleType: models.ApproverRoleTeamLeader, IsActive: true},
		{Name: "Master DH", RoleType: models.ApproverRoleDepartmentHead, IsActive: true},
		{Name: "Inactive TL", RoleType: models.ApproverRoleTeamLeader, IsActive: false},
	}

	t.Run("empty overtimes falls back to master approvers", func(t *testing.T) {
		in := GenerationInput{
			Approvers: masterApprovers,
		}
		tl, dh := ResolveApprovers(in)
		if tl != "Master TL" {
			t.Errorf("expected tl 'Master TL', got %q", tl)
		}
		if dh != "Master DH" {
			t.Errorf("expected dh 'Master DH', got %q", dh)
		}
	})

	t.Run("overtimes take precedence over master approvers", func(t *testing.T) {
		in := GenerationInput{
			Overtimes: []models.OvertimeEntry{
				{
					TeamLeader:     &models.Approver{Name: "Overtime TL"},
					DepartmentHead: &models.Approver{Name: "Overtime DH"},
				},
			},
			Approvers: masterApprovers,
		}
		tl, dh := ResolveApprovers(in)
		if tl != "Overtime TL" {
			t.Errorf("expected tl 'Overtime TL', got %q", tl)
		}
		if dh != "Overtime DH" {
			t.Errorf("expected dh 'Overtime DH', got %q", dh)
		}
	})

	t.Run("partial overtime approver falls back to master approver", func(t *testing.T) {
		in := GenerationInput{
			Overtimes: []models.OvertimeEntry{
				{
					TeamLeader: &models.Approver{Name: "Overtime TL Only"},
				},
			},
			Approvers: masterApprovers,
		}
		tl, dh := ResolveApprovers(in)
		if tl != "Overtime TL Only" {
			t.Errorf("expected tl 'Overtime TL Only', got %q", tl)
		}
		if dh != "Master DH" {
			t.Errorf("expected dh 'Master DH', got %q", dh)
		}
	})
}

func TestBuilderCommon_Helpers(t *testing.T) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	st, _ := NewBuilderStyles(f)

	// Test WriteTimeCells
	hasStart, hasEnd := WriteTimeCells(f, sheet, "C5", "D5", "08:00", "17:00", st.TimeStyle)
	if !hasStart || !hasEnd {
		t.Errorf("expected hasStart and hasEnd to be true, got %v, %v", hasStart, hasEnd)
	}
	c5, _ := f.GetCellValue(sheet, "C5")
	if c5 == "" {
		t.Error("expected non-empty time fraction in C5")
	}

	// Test ApplyBlankPaddingRow
	ApplyBlankPaddingRow(f, sheet, 6, []string{"A", "B"}, []string{"C"}, st)

	// Test BuildByDayMap
	acts := []models.DailyActivity{
		{
			Date:     time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			Activity: "Development",
		},
	}
	byDay := BuildByDayMap(acts)
	if len(byDay) != 1 || byDay[15].Activity != "Development" {
		t.Errorf("unexpected byDay map: %+v", byDay)
	}

	// Test WriteTableHeaders
	headers := []HeaderColumn{
		{From: "A1", To: "A2", Val: "Date"},
		{From: "B1", To: "C1", Val: "Hours"},
	}
	subHeaders := map[string]string{"B2": "Start", "C2": "End"}
	WriteTableHeaders(f, sheet, headers, subHeaders, st)
	vA1, _ := f.GetCellValue(sheet, "A1")
	if vA1 != "Date" {
		t.Errorf("expected A1 to be 'Date', got %q", vA1)
	}

	// Test WriteHolidayRemarkRow
	WriteHolidayRemarkRow(f, sheet, "K", "10", 10, "Nyepi")
	vK10, _ := f.GetCellValue(sheet, "K10")
	if vK10 != "Nyepi" {
		t.Errorf("expected K10 to be 'Nyepi', got %q", vK10)
	}
	WriteHolidayRemarkRow(f, sheet, "K", "11", 11, "")
	vK11, _ := f.GetCellValue(sheet, "K11")
	if vK11 != "Weekend" {
		t.Errorf("expected K11 to be 'Weekend', got %q", vK11)
	}

	// Test WriteAttendanceMatrixStatus
	WriteAttendanceMatrixStatus(f, sheet, "12", "P")
	vE12, _ := f.GetCellValue(sheet, "E12")
	if vE12 != "P" {
		t.Errorf("expected E12 to be 'P', got %q", vE12)
	}

	// Test ResolveDayStyleContext and ApplyDayRowStyles
	holidays := map[int]string{10: "Nyepi"}
	dscWeekday := ResolveDayStyleContext(2026, 3, 11, holidays, byDay, st)
	if dscWeekday.IsHolidayOrWeekend {
		t.Errorf("Wednesday March 11, 2026 should not be weekend or holiday")
	}
	dscWeekend := ResolveDayStyleContext(2026, 3, 14, holidays, byDay, st)
	if !dscWeekend.IsHolidayOrWeekend {
		t.Errorf("Saturday March 14, 2026 should be weekend")
	}
	dscHoliday := ResolveDayStyleContext(2026, 3, 10, holidays, byDay, st)
	if !dscHoliday.IsHolidayOrWeekend || dscHoliday.Holiday != "Nyepi" {
		t.Errorf("March 10 should be holiday 'Nyepi'")
	}
	ApplyDayRowStyles(f, sheet, "A", "15", []string{"A", "B"}, []string{"K"}, dscWeekday)
}

func TestSetWorkbookProperties(t *testing.T) {
	t.Run("with user name", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()

		SetWorkbookProperties(f, "Test Timesheet", "Faisal Rahman")
		props, err := f.GetDocProps()
		if err != nil {
			t.Fatalf("failed to get doc props: %v", err)
		}
		if props.Creator != "Waluh Corporation" {
			t.Errorf("expected Creator 'Waluh Corporation', got %q", props.Creator)
		}
		if props.LastModifiedBy != "Faisal Rahman" {
			t.Errorf("expected LastModifiedBy 'Faisal Rahman', got %q", props.LastModifiedBy)
		}
		if props.Title != "Test Timesheet" {
			t.Errorf("expected Title 'Test Timesheet', got %q", props.Title)
		}
	})

	t.Run("fallback when empty user name", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()

		SetWorkbookProperties(f, "Test Timesheet", "")
		props, err := f.GetDocProps()
		if err != nil {
			t.Fatalf("failed to get doc props: %v", err)
		}
		if props.LastModifiedBy != "Waluh Corporation" {
			t.Errorf("expected fallback LastModifiedBy 'Waluh Corporation', got %q", props.LastModifiedBy)
		}
	})
}

func TestResolveEmployeeID(t *testing.T) {
	t.Run("nil user", func(t *testing.T) {
		if got := ResolveEmployeeID(nil); got != "" {
			t.Errorf("expected empty string for nil user, got %q", got)
		}
	})

	t.Run("with employee id", func(t *testing.T) {
		u := &models.User{EmployeeID: "EMP001", BniID: "BNI001"}
		if got := ResolveEmployeeID(u); got != "EMP001" {
			t.Errorf("expected EMP001, got %q", got)
		}
	})

	t.Run("with bni id fallback", func(t *testing.T) {
		u := &models.User{BniID: "BNI002"}
		if got := ResolveEmployeeID(u); got != "BNI002" {
			t.Errorf("expected BNI002, got %q", got)
		}
	})
}

func TestBuilderCommon_NewHelpers(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Sheet1"
	st, _ := NewBuilderStyles(f)

	// WriteMetaField
	WriteMetaField(f, sheet, "A1", "NAME", "B1", ": Alice", st)
	vA1, _ := f.GetCellValue(sheet, "A1")
	vB1, _ := f.GetCellValue(sheet, "B1")
	if vA1 != "NAME" || vB1 != ": Alice" {
		t.Errorf("WriteMetaField failed, got %q, %q", vA1, vB1)
	}

	// WriteActivityProjectCells
	WriteActivityProjectCells(f, sheet, "10", 10, "Dev Task", "Proj A", "P01", "App X")
	vK, _ := f.GetCellValue(sheet, "K10")
	vL, _ := f.GetCellValue(sheet, "L10")
	if vK != "Dev Task" || vL != "Proj A" {
		t.Errorf("WriteActivityProjectCells failed, got %q, %q", vK, vL)
	}

	// WriteColumnFormulas
	WriteColumnFormulas(f, sheet, "40", map[string]string{
		"E": `COUNTIF(E9:E39,"P")`,
	}, st.BoldCenterStyle)
	formulaE, _ := f.GetCellFormula(sheet, "E40")
	if formulaE != `COUNTIF(E9:E39,"P")` {
		t.Errorf("expected formula COUNTIF, got %q", formulaE)
	}

	// WriteSignaturesLayout with Position
	WriteSignaturesLayout(f, sheet, 42, 5, []SignatureParty{
		{StartCol: "A", EndCol: "C", Title: "TTD PEGAWAI,", Name: "Alice", Position: "Developer"},
	}, st)
	title, _ := f.GetCellValue(sheet, "A42")
	name, _ := f.GetCellValue(sheet, "A48")
	pos, _ := f.GetCellValue(sheet, "A49")
	if title != "TTD PEGAWAI," || name != "Alice" || pos != "Developer" {
		t.Errorf("WriteSignaturesLayout failed, got title=%q name=%q pos=%q", title, name, pos)
	}

	// WriteBuilderDailyRows
	in := GenerationInput{
		Year:  2026,
		Month: 2, // 28 days
		User:  &models.User{Name: "Alice"},
		Activities: []models.DailyActivity{
			{Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Activity: "Feature X", Status: "P"},
		},
		Holidays: map[int]string{
			2: "Public Holiday",
		},
	}
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}
	called := false
	WriteBuilderDailyRows(f, sheet, in, 10, allCols, []string{"K"}, st, func(rs string, row int, dsc DayStyleContext) {
		called = true
		_ = f.SetCellValue(sheet, "K"+rs, dsc.Activity.Activity)
	})
	if !called {
		t.Errorf("expected writeAct to be called for day 1")
	}
	vK10, _ := f.GetCellValue(sheet, "K10")
	if vK10 != "Feature X" {
		t.Errorf("expected 'Feature X', got %q", vK10)
	}
	vK11, _ := f.GetCellValue(sheet, "K11")
	if vK11 != "Public Holiday" {
		t.Errorf("expected 'Public Holiday', got %q", vK11)
	}
}
