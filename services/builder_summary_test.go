package services_test

import (
	"testing"

	"timesheet-backend/dto/response"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

func TestBuildSummaryWorkbook_YearlyAndMonthly(t *testing.T) {
	user := &models.User{
		Username:   "testuser",
		Name:       "John Doe",
		EmployeeID: "EMP-101",
		Company:    "mii",
		Department: "IT Development",
		Division:   "Wholesale Digital Delivery",
	}

	monthInt := 6
	summary := &response.TimesheetSummaryResponse{
		Year:               2026,
		Month:              &monthInt,
		TotalWorkingDays:   21,
		TotalDaysFilled:    20,
		TotalWorkingHours:  160.0,
		TotalOvertimeHours: 5.5,
		YearlyAttendanceBreakdown: map[string]int{
			"P": 19, "S": 1,
		},
		Months: []response.MonthlySummaryDTO{
			{
				Month:         6,
				MonthName:     services.MonthNameIndonesian(6),
				WorkingDays:   21,
				DaysFilled:    20,
				IsComplete:    false,
				WorkingHours:  160.0,
				OvertimeHours: 5.5,
				AttendanceBreakdown: map[string]int{
					"P": 19, "S": 1, "V": 0, "PM": 0, "BT": 0, "X": 0,
				},
			},
		},
	}

	data, err := services.BuildSummaryWorkbook(services.SummaryReportInput{
		User:    user,
		Summary: summary,
	})
	if err != nil {
		t.Fatalf("failed to build summary workbook: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty workbook bytes")
	}

	// Test full year
	summary.Month = nil
	summary.Months = append(summary.Months, response.MonthlySummaryDTO{
		Month:         7,
		MonthName:     services.MonthNameIndonesian(7),
		WorkingDays:   22,
		DaysFilled:    22,
		IsComplete:    true,
		WorkingHours:  176.0,
		OvertimeHours: 0.0,
		AttendanceBreakdown: map[string]int{
			"P": 22,
		},
	})
	dataYearly, err := services.BuildSummaryWorkbook(services.SummaryReportInput{
		User:    user,
		Summary: summary,
	})
	if err != nil {
		t.Fatalf("failed to build yearly summary workbook: %v", err)
	}
	if len(dataYearly) == 0 {
		t.Fatalf("expected non-empty yearly workbook bytes")
	}
}

func TestMonthNameIndonesian(t *testing.T) {
	if services.MonthNameIndonesian(1) != "Januari" {
		t.Errorf("expected Januari, got %s", services.MonthNameIndonesian(1))
	}
	if services.MonthNameIndonesian(12) != "Desember" {
		t.Errorf("expected Desember, got %s", services.MonthNameIndonesian(12))
	}
	if services.MonthNameIndonesian(13) != "Bulan 13" {
		t.Errorf("expected Bulan 13, got %s", services.MonthNameIndonesian(13))
	}
}
