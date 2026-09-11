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
}
