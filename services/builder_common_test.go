package services

import (
	"fmt"
	"strings"
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
	WriteActivityProjectCells(f, sheet, 10, ActivityProjectInfo{
		Activity:    "Dev Task",
		ProjectName: "Proj A",
		ProjectCode: "P01",
		AppImpacted: "App X",
	})
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

	// WriteSignatures
	WriteSignatures(f, sheet, GenerationInput{
		User: &models.User{Name: "Tester"},
	}, SignatureCells{
		UserCell:     "A52",
		TLCell:       "D52",
		DHCell:       "G52",
		UserDateCell: "A53",
		TLDateCell:   "D53",
		DHDateCell:   "G53",
		DatePrefix:   "DATE : ",
	})
	sigUser, _ := f.GetCellValue(sheet, "A52")
	if sigUser != "Tester" {
		t.Errorf("expected signature user 'Tester', got %q", sigUser)
	}

	// WriteWorkingHoursRow
	dscActivity := DayStyleContext{
		HasActivity: true,
		Activity:    models.DailyActivity{StartTime: "08:30", EndTime: "17:30"},
		TimeStyle:   st.TimeStyle,
	}
	WriteWorkingHoursRow(f, sheet, WorkingHoursConfig{
		StartCol: "B", EndCol: "C", TotalCol: "D", RowStr: "60", FormulaFmt: "C%s-B%s",
	}, dscActivity)
	startVal, _ := f.GetCellValue(sheet, "B60")
	if startVal == "" {
		t.Errorf("expected start time cell to be populated")
	}

	dscHoliday := DayStyleContext{
		IsHolidayOrWeekend: true,
		TimeStyle:          st.TimeStyle,
	}
	WriteWorkingHoursRow(f, sheet, WorkingHoursConfig{
		StartCol: "B", EndCol: "C", TotalCol: "D", RowStr: "61",
	}, dscHoliday)
	holidayStart, _ := f.GetCellValue(sheet, "B61")
	if holidayStart != "" {
		t.Errorf("expected holiday cell to be empty, got %q", holidayStart)
	}

	dscDefaultWorkday := DayStyleContext{
		TimeStyle:    st.TimeStyle,
		DecimalStyle: st.DecimalStyle,
	}
	WriteWorkingHoursRow(f, sheet, WorkingHoursConfig{
		StartCol: "B", EndCol: "C", TotalCol: "D", RowStr: "62", FormulaFmt: "(C{row}-B{row})*24", IsDecimalTotal: true,
	}, dscDefaultWorkday)

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
	WriteBuilderDailyRows(f, sheet, in, DailyRowsConfig{
		StartRow: 10,
		AllCols:  allCols,
		WrapCols: []string{"K"},
		Styles:   st,
		WriteAct: func(rs string, row int, dsc DayStyleContext) {
			called = true
			_ = f.SetCellValue(sheet, "K"+rs, dsc.Activity.Activity)
		},
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

func TestMonthNameIndonesian(t *testing.T) {
	if MonthNameIndonesian(1) != "Januari" {
		t.Errorf("expected Januari, got %s", MonthNameIndonesian(1))
	}
	if MonthNameIndonesian(12) != "Desember" {
		t.Errorf("expected Desember, got %s", MonthNameIndonesian(12))
	}
	if MonthNameIndonesian(13) != "Bulan 13" {
		t.Errorf("expected Bulan 13, got %s", MonthNameIndonesian(13))
	}
}

func TestResolveDayStyleContext_WeekendAndHolidayScenarios(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	st, err := NewBuilderStyles(f)
	if err != nil {
		t.Fatalf("failed to create builder styles: %v", err)
	}

	holidays := map[int]string{
		10: "Nyepi",
	}
	byDay := map[int]models.DailyActivity{
		10: {StartTime: "08:00", EndTime: "17:00", Activity: "Holiday Maintenance", Status: "P"},
		14: {StartTime: "09:00", EndTime: "18:00", Activity: "Saturday Deployment", Status: "P"},
	}

	tests := []struct {
		name              string
		year              int
		month             int
		day               int
		byDayMap          map[int]models.DailyActivity
		wantHolidayOrWknd bool
		wantHasActivity   bool
		wantCenterStyle   int
		wantDateStyle     int
		wantTimeStyle     int
	}{
		{
			name:              "Weekend without activity -> grey style",
			year:              2026,
			month:             3,
			day:               15, // Sunday
			byDayMap:          byDay,
			wantHolidayOrWknd: true,
			wantHasActivity:   false,
			wantCenterStyle:   st.GreyCenterStyle,
			wantDateStyle:     st.GreyDateStyle,
			wantTimeStyle:     st.GreyTimeStyle,
		},
		{
			name:              "Weekend with overtime activity -> standard clean/white style",
			year:              2026,
			month:             3,
			day:               14, // Saturday
			byDayMap:          byDay,
			wantHolidayOrWknd: true,
			wantHasActivity:   true,
			wantCenterStyle:   st.DataCenterStyle,
			wantDateStyle:     st.DateStyle,
			wantTimeStyle:     st.TimeStyle,
		},
		{
			name:              "Public holiday without activity -> grey style",
			year:              2026,
			month:             3,
			day:               10, // Nyepi
			byDayMap:          map[int]models.DailyActivity{},
			wantHolidayOrWknd: true,
			wantHasActivity:   false,
			wantCenterStyle:   st.GreyCenterStyle,
			wantDateStyle:     st.GreyDateStyle,
			wantTimeStyle:     st.GreyTimeStyle,
		},
		{
			name:              "Public holiday with overtime activity -> standard clean/white style",
			year:              2026,
			month:             3,
			day:               10, // Nyepi
			byDayMap:          byDay,
			wantHolidayOrWknd: true,
			wantHasActivity:   true,
			wantCenterStyle:   st.DataCenterStyle,
			wantDateStyle:     st.DateStyle,
			wantTimeStyle:     st.TimeStyle,
		},
		{
			name:              "Regular weekday without activity -> standard clean/white style",
			year:              2026,
			month:             3,
			day:               11, // Wednesday
			byDayMap:          byDay,
			wantHolidayOrWknd: false,
			wantHasActivity:   false,
			wantCenterStyle:   st.DataCenterStyle,
			wantDateStyle:     st.DateStyle,
			wantTimeStyle:     st.TimeStyle,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dsc := ResolveDayStyleContext(tc.year, tc.month, tc.day, holidays, tc.byDayMap, st)
			if dsc.IsHolidayOrWeekend != tc.wantHolidayOrWknd {
				t.Errorf("IsHolidayOrWeekend = %v, want %v", dsc.IsHolidayOrWeekend, tc.wantHolidayOrWknd)
			}
			if dsc.HasActivity != tc.wantHasActivity {
				t.Errorf("HasActivity = %v, want %v", dsc.HasActivity, tc.wantHasActivity)
			}
			if dsc.CenterStyle != tc.wantCenterStyle {
				t.Errorf("CenterStyle = %d, want %d", dsc.CenterStyle, tc.wantCenterStyle)
			}
			if dsc.DateStyle != tc.wantDateStyle {
				t.Errorf("DateStyle = %d, want %d", dsc.DateStyle, tc.wantDateStyle)
			}
			if dsc.TimeStyle != tc.wantTimeStyle {
				t.Errorf("TimeStyle = %d, want %d", dsc.TimeStyle, tc.wantTimeStyle)
			}
		})
	}
}

func TestWriteBuilderDailyRows_WeekendAndHolidayOvertime(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Sheet1"
	st, _ := NewBuilderStyles(f)

	in := GenerationInput{
		Year:  2026,
		Month: 3, // March 2026: Day 1 is Sunday, Day 10 is Nyepi (Holiday), Day 14 is Saturday (OT)
		Activities: []models.DailyActivity{
			{Date: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Activity: "Holiday Support", Status: "P"},
			{Date: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), Activity: "Weekend Deploy", Status: "P"},
		},
		Holidays: map[int]string{
			10: "Hari Raya Nyepi",
		},
	}

	WriteBuilderDailyRows(f, sheet, in, DailyRowsConfig{
		StartRow: 10,
		AllCols:  []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"},
		WrapCols: []string{"K"},
		Styles:   st,
		WriteAct: func(rs string, row int, dsc DayStyleContext) {
			_ = f.SetCellValue(sheet, "K"+rs, dsc.Activity.Activity)
		},
	})

	// Day 1: Sunday without activity -> Row 10
	valK10, _ := f.GetCellValue(sheet, "K10")
	if valK10 != "Weekend" {
		t.Errorf("K10 = %q, want 'Weekend'", valK10)
	}
	sID10, _ := f.GetCellStyle(sheet, "A10")
	if sID10 != st.GreyDateStyle {
		t.Errorf("A10 style = %d, want GreyDateStyle %d", sID10, st.GreyDateStyle)
	}
	// Assert status matrix columns E..J are all empty for Day 1
	for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
		v, _ := f.GetCellValue(sheet, col+"10")
		if v != "" {
			t.Errorf("Day 1 cell %s10 = %q, want empty", col, v)
		}
	}

	// Day 10: Holiday with activity -> Row 19
	valK19, _ := f.GetCellValue(sheet, "K19")
	if valK19 != "Holiday Support" {
		t.Errorf("K19 = %q, want 'Holiday Support'", valK19)
	}
	sID19, _ := f.GetCellStyle(sheet, "A19")
	if sID19 != st.DateStyle {
		t.Errorf("A19 style = %d, want standard DateStyle %d", sID19, st.DateStyle)
	}
	valE19, _ := f.GetCellValue(sheet, "E19")
	if valE19 != "P" {
		t.Errorf("E19 = %q, want 'P'", valE19)
	}
	valJ19, _ := f.GetCellValue(sheet, "J19")
	if valJ19 != "" {
		t.Errorf("J19 = %q, want empty inactive status cell", valJ19)
	}

	// Day 14: Weekend with activity -> Row 23
	valK23, _ := f.GetCellValue(sheet, "K23")
	if valK23 != "Weekend Deploy" {
		t.Errorf("K23 = %q, want 'Weekend Deploy'", valK23)
	}
	sID23, _ := f.GetCellStyle(sheet, "A23")
	if sID23 != st.DateStyle {
		t.Errorf("A23 style = %d, want standard DateStyle %d", sID23, st.DateStyle)
	}
	valE23, _ := f.GetCellValue(sheet, "E23")
	if valE23 != "P" {
		t.Errorf("E23 = %q, want 'P'", valE23)
	}
	valJ23, _ := f.GetCellValue(sheet, "J23")
	if valJ23 != "" {
		t.Errorf("J23 = %q, want empty inactive status cell", valJ23)
	}
}

func TestWriteAttendanceMatrixStatus_InactiveMarkerAndAllStatuses(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Sheet1"

	tests := []struct {
		name        string
		statusInput string
		expectedCol string
		expectedVal string
		wantJVal    string
	}{
		{
			name:        "Present 'P' -> col E has 'P', col J is empty",
			statusInput: "P",
			expectedCol: "E",
			expectedVal: "P",
			wantJVal:    "",
		},
		{
			name:        "Sick 'S' -> col F has 'S', col J is empty",
			statusInput: "S",
			expectedCol: "F",
			expectedVal: "S",
			wantJVal:    "",
		},
		{
			name:        "Business Trip 'BT' -> col G has 'BT', col J is empty",
			statusInput: "BT",
			expectedCol: "G",
			expectedVal: "BT",
			wantJVal:    "",
		},
		{
			name:        "Permit 'PM' -> col H has 'PM', col J is empty",
			statusInput: "PM",
			expectedCol: "H",
			expectedVal: "PM",
			wantJVal:    "",
		},
		{
			name:        "Vacation 'V' -> col I has 'V', col J is empty",
			statusInput: "V",
			expectedCol: "I",
			expectedVal: "V",
			wantJVal:    "",
		},
		{
			name:        "Inactive status uppercase 'X' -> col J must be EMPTY (not 'x' or 'X')",
			statusInput: "X",
			expectedCol: "J",
			expectedVal: "",
			wantJVal:    "",
		},
		{
			name:        "Inactive status lowercase 'x' -> col J must be EMPTY (not 'x' or 'X')",
			statusInput: "x",
			expectedCol: "J",
			expectedVal: "",
			wantJVal:    "",
		},
		{
			name:        "Inactive status with spaces ' X ' -> col J must be EMPTY",
			statusInput: " X ",
			expectedCol: "J",
			expectedVal: "",
			wantJVal:    "",
		},
		{
			name:        "Empty status -> all matrix cells empty",
			statusInput: "",
			expectedCol: "",
			expectedVal: "",
			wantJVal:    "",
		},
		{
			name:        "Unknown status -> all matrix cells empty",
			statusInput: "UNKNOWN",
			expectedCol: "",
			expectedVal: "",
			wantJVal:    "",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rowStr := fmt.Sprintf("%d", 30+i)
			WriteAttendanceMatrixStatus(f, sheet, rowStr, tc.statusInput)

			if tc.expectedCol != "" {
				val, _ := f.GetCellValue(sheet, tc.expectedCol+rowStr)
				if val != tc.expectedVal {
					t.Errorf("Cell %s%s = %q, want %q", tc.expectedCol, rowStr, val, tc.expectedVal)
				}
			}

			// Cell J must never contain "x" or "X" when status is inactive
			valJ, _ := f.GetCellValue(sheet, "J"+rowStr)
			if valJ == "x" || valJ == "X" {
				t.Errorf("Cell J%s contains %q, but inactive marker must be empty (\"\")", rowStr, valJ)
			}
			if valJ != tc.wantJVal {
				t.Errorf("Cell J%s = %q, want %q", rowStr, valJ, tc.wantJVal)
			}

			// Verify other columns are empty when status is "X", "x", or ""
			if tc.expectedVal == "" {
				for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
					v, _ := f.GetCellValue(sheet, col+rowStr)
					if v != "" {
						t.Errorf("Expected cell %s%s to be empty, got %q", col, rowStr, v)
					}
				}
			}
		})
	}
}

func TestMonthNameHelpers(t *testing.T) {
	t.Run("MonthNameIndonesian", func(t *testing.T) {
		if got := MonthNameIndonesian(1); got != "Januari" {
			t.Errorf("got %q, want 'Januari'", got)
		}
		if got := MonthNameIndonesian(10); got != "Oktober" {
			t.Errorf("got %q, want 'Oktober'", got)
		}
		if got := MonthNameIndonesian(12); got != "Desember" {
			t.Errorf("got %q, want 'Desember'", got)
		}
		if got := MonthNameIndonesian(0); got != "Bulan 0" {
			t.Errorf("got %q, want 'Bulan 0'", got)
		}
	})

	t.Run("MonthNameEnglish", func(t *testing.T) {
		if got := MonthNameEnglish(1); got != "January" {
			t.Errorf("got %q, want 'January'", got)
		}
		if got := MonthNameEnglish(10); got != "October" {
			t.Errorf("got %q, want 'October'", got)
		}
		if got := MonthNameEnglish(12); got != "December" {
			t.Errorf("got %q, want 'December'", got)
		}
		if got := MonthNameEnglish(13); got != "Month 13" {
			t.Errorf("got %q, want 'Month 13'", got)
		}
	})
}

func TestWriteSignatures_ApproversBlank(t *testing.T) {
	f := excelize.NewFile()
	sheet := "Sheet1"

	in := GenerationInput{
		User: &models.User{Name: "Alice Timesheet Owner"},
		Approvers: []models.Approver{
			{Name: "Approver TL", RoleType: models.ApproverRoleTeamLeader},
			{Name: "Approver DH", RoleType: models.ApproverRoleDepartmentHead},
		},
	}
	cells := SignatureCells{
		UserCell:     "A10",
		TLCell:       "B10",
		DHCell:       "C10",
		UserDateCell: "A11",
		TLDateCell:   "B11",
		DHDateCell:   "C11",
		DatePrefix:   "Date: ",
	}

	WriteSignatures(f, sheet, in, cells)

	userVal, _ := f.GetCellValue(sheet, "A10")
	if userVal != "Alice Timesheet Owner" {
		t.Errorf("expected user cell A10 to be %q, got %q", "Alice Timesheet Owner", userVal)
	}

	tlVal, _ := f.GetCellValue(sheet, "B10")
	if tlVal != "" {
		t.Errorf("expected TL cell B10 to be empty, got %q", tlVal)
	}

	dhVal, _ := f.GetCellValue(sheet, "C10")
	if dhVal != "" {
		t.Errorf("expected DH cell C10 to be empty, got %q", dhVal)
	}

	userDate, _ := f.GetCellValue(sheet, "A11")
	if !strings.HasPrefix(userDate, "Date: ") {
		t.Errorf("expected UserDateCell A11 to have prefix 'Date: ', got %q", userDate)
	}

	tlDate, _ := f.GetCellValue(sheet, "B11")
	if tlDate != "" {
		t.Errorf("expected TLDateCell B11 to be empty, got %q", tlDate)
	}

	dhDate, _ := f.GetCellValue(sheet, "C11")
	if dhDate != "" {
		t.Errorf("expected DHDateCell C11 to be empty, got %q", dhDate)
	}
}
