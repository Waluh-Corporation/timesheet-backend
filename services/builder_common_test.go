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
