package services

import (
	"math"
	"testing"
	"time"

	"timesheet-backend/models"
)

func TestCalculateTimesheetModel_InputValidation(t *testing.T) {
	t.Run("nil input returns error", func(t *testing.T) {
		_, err := CalculateTimesheetModel(nil)
		if err == nil {
			t.Fatal("expected error for nil input, got nil")
		}
	})

	t.Run("invalid month returns error", func(t *testing.T) {
		for _, m := range []int{0, 13, -1} {
			in := &GenerationInput{Month: m, Year: 2026}
			_, err := CalculateTimesheetModel(in)
			if err == nil {
				t.Fatalf("expected error for invalid month %d, got nil", m)
			}
		}
	})

	t.Run("invalid year returns error", func(t *testing.T) {
		in := &GenerationInput{Month: 10, Year: 1800}
		_, err := CalculateTimesheetModel(in)
		if err == nil {
			t.Fatal("expected error for year < 1900, got nil")
		}
	})
}

func TestCalculateTimesheetModel_BusinessCalculations(t *testing.T) {
	// October 2026 has 31 days.
	// 2026-10-01 is Thursday (weekday)
	// 2026-10-03 is Saturday (weekend)
	// 2026-10-04 is Sunday (weekend)
	// Let's set 2026-10-08 as a public holiday.
	// Day 3 (Saturday) has NO activity -> Inactive weekend
	// Day 4 (Sunday) has active overtime activity -> Overtime non-working day
	// Day 8 (Thursday) is a holiday with NO activity -> Inactive holiday
	// Day 9 (Friday) is a holiday with overtime activity -> Overtime holiday
	// Day 1 (Thursday) is weekday with NO activity -> Default working day
	// Day 2 (Friday) is weekday with active activity (Status: "X", Inactive marker) -> Active row with empty marker
	// Day 5 (Monday) is weekday with active activity (Status: "S", Sick)
	holidays := map[int]string{
		8: "Maulid Nabi Muhammad SAW",
		9: "Hari Libur Nasional Tambahan",
	}

	testPrj := &models.Project{
		Name: "Timesheet Core",
		Code: "PRJ-001",
	}

	activities := []models.DailyActivity{
		{
			Date:       time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), // Sunday overtime
			StartTime:  "09:00",
			EndTime:    "18:00",
			Status:     "P",
			Activity:   "Deploy emergency release hotfix",
			ProjectRef: testPrj,
		},
		{
			Date:       time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), // Holiday overtime
			StartTime:  "10:00",
			EndTime:    "16:00",
			Status:     "BT",
			Activity:   "On-site disaster recovery support",
			ProjectRef: testPrj,
		},
		{
			Date:       time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), // Weekday with Inactive status "X"
			StartTime:  "08:00",
			EndTime:    "17:00",
			Status:     "X",
			Activity:   "Unpaid inactive absence",
			ProjectRef: testPrj,
		},
		{
			Date:       time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), // Weekday Sick
			StartTime:  "08:00",
			EndTime:    "17:00",
			Status:     "S",
			Activity:   "Sick with medical certificate",
			ProjectRef: testPrj,
		},
	}

	user := &models.User{
		Name:       "Aditya Pratama",
		Username:   "aditya.p",
		BniID:      "123456",
		Division:   "Technology",
		Department: "Core Banking",
		Site:       "Jakarta HQ",
		Company:    "PT Mitra Integrasi Informatika",
	}

	approvers := []models.Approver{
		{Name: "Budi Santoso", RoleType: models.ApproverRoleTeamLeader, IsActive: true},
		{Name: "Siti Rahma", RoleType: models.ApproverRoleDepartmentHead, IsActive: true},
	}

	input := &GenerationInput{
		CompanyCode: "mii",
		User:        user,
		Month:       10,
		Year:        2026,
		Activities:  activities,
		Approvers:   approvers,
		Holidays:    holidays,
		IsSigned:    true,
	}

	model, err := CalculateTimesheetModel(input)
	if err != nil {
		t.Fatalf("CalculateTimesheetModel failed: %v", err)
	}

	if model.DaysInMonth != 31 {
		t.Errorf("expected 31 days in October 2026, got %d", model.DaysInMonth)
	}
	if model.PeriodDisplay != "Oktober 2026" {
		t.Errorf("expected period 'Oktober 2026', got '%s'", model.PeriodDisplay)
	}
	if model.CompanyName != "PT Mitra Integrasi Informatika" {
		t.Errorf("expected company name 'PT Mitra Integrasi Informatika', got '%s'", model.CompanyName)
	}

	// 1. Day 1: Regular working day without activity
	day1 := model.Rows[0]
	if day1.Day != 1 || day1.IsWeekend || day1.IsHoliday || day1.HasActivity || day1.IsNonWorkingRow {
		t.Errorf("day 1 invalid flags: %+v", day1)
	}
	if day1.StartTime != "08:00" || day1.EndTime != "17:00" || day1.Status != "P" || day1.StatusDisplay != "P" {
		t.Errorf("day 1 expected default hours 08:00-17:00 and status P, got start=%s end=%s status=%s display=%s",
			day1.StartTime, day1.EndTime, day1.Status, day1.StatusDisplay)
	}
	if math.Abs(day1.DurationHours-9.0) > 0.001 {
		t.Errorf("day 1 expected 9.0 hours, got %f", day1.DurationHours)
	}

	// 2. Day 2: Weekday with Inactive status "X" -> StatusDisplay MUST be empty ""
	day2 := model.Rows[1]
	if day2.Status != "X" {
		t.Errorf("day 2 expected Status 'X', got '%s'", day2.Status)
	}
	if day2.StatusDisplay != "" {
		t.Errorf("day 2 expected StatusDisplay to be empty for 'X' per business rule, got '%s'", day2.StatusDisplay)
	}

	// 3. Day 3: Saturday without activity -> Inactive weekend
	day3 := model.Rows[2]
	if !day3.IsWeekend || day3.HasActivity || !day3.IsNonWorkingRow || day3.IsOvertime {
		t.Errorf("day 3 flags invalid for inactive weekend: %+v", day3)
	}
	if day3.StartTime != "" || day3.EndTime != "" || day3.DurationHours != 0 {
		t.Errorf("day 3 inactive weekend should have empty hours, got start=%s end=%s dur=%f",
			day3.StartTime, day3.EndTime, day3.DurationHours)
	}
	if day3.Remark != "Weekend" {
		t.Errorf("day 3 expected remark 'Weekend', got '%s'", day3.Remark)
	}

	// 4. Day 4: Sunday with active activity -> Weekend Overtime!
	day4 := model.Rows[3]
	if !day4.IsWeekend || !day4.HasActivity || day4.IsNonWorkingRow || !day4.IsOvertime {
		t.Errorf("day 4 flags invalid for overtime weekend: %+v", day4)
	}
	if day4.StartTime != "09:00" || day4.EndTime != "18:00" {
		t.Errorf("day 4 expected 09:00-18:00, got %s-%s", day4.StartTime, day4.EndTime)
	}
	if math.Abs(day4.DurationHours-9.0) > 0.001 {
		t.Errorf("day 4 expected 9.0 hours duration, got %f", day4.DurationHours)
	}
	if day4.Remark != "Deploy emergency release hotfix" {
		t.Errorf("day 4 expected activity remark, got '%s'", day4.Remark)
	}

	// 5. Day 8: Holiday without activity -> Inactive holiday
	day8 := model.Rows[7]
	if !day8.IsHoliday || day8.HasActivity || !day8.IsNonWorkingRow || day8.IsOvertime {
		t.Errorf("day 8 flags invalid for inactive holiday: %+v", day8)
	}
	if day8.Remark != "Maulid Nabi Muhammad SAW" {
		t.Errorf("day 8 expected holiday remark, got '%s'", day8.Remark)
	}
	if day8.StartTime != "" || day8.DurationHours != 0 {
		t.Errorf("day 8 expected clear hours, got start=%s dur=%f", day8.StartTime, day8.DurationHours)
	}

	// 6. Day 9: Holiday with overtime activity -> Overtime Holiday!
	day9 := model.Rows[8]
	if !day9.IsHoliday || !day9.HasActivity || day9.IsNonWorkingRow || !day9.IsOvertime {
		t.Errorf("day 9 flags invalid for overtime holiday: %+v", day9)
	}
	if day9.StartTime != "10:00" || day9.EndTime != "16:00" {
		t.Errorf("day 9 expected 10:00-16:00, got %s-%s", day9.StartTime, day9.EndTime)
	}
	if math.Abs(day9.DurationHours-6.0) > 0.001 {
		t.Errorf("day 9 expected 6.0 hours duration, got %f", day9.DurationHours)
	}
	if day9.Status != "BT" || day9.StatusDisplay != "BT" {
		t.Errorf("day 9 expected status BT, got status=%s display=%s", day9.Status, day9.StatusDisplay)
	}

	// 7. Verify Summary Metrics
	if model.Summary.TotalDays != 31 {
		t.Errorf("expected total days 31, got %d", model.Summary.TotalDays)
	}
	if math.Abs(model.Summary.TotalOvertimeHours-15.0) > 0.001 { // Day 4 (9h) + Day 9 (6h) = 15.0h
		t.Errorf("expected total overtime hours 15.0, got %f", model.Summary.TotalOvertimeHours)
	}
	if model.Summary.TotalSickDays != 1 {
		t.Errorf("expected 1 sick day, got %d", model.Summary.TotalSickDays)
	}
	if model.Summary.TotalBusinessTrip != 1 {
		t.Errorf("expected 1 business trip day, got %d", model.Summary.TotalBusinessTrip)
	}
	if model.Summary.TotalInactiveDays != 1 {
		t.Errorf("expected 1 inactive day (X), got %d", model.Summary.TotalInactiveDays)
	}

	// 8. Verify Approvers
	if model.Approvers.TeamLeadName != "Budi Santoso" {
		t.Errorf("expected team lead 'Budi Santoso', got '%s'", model.Approvers.TeamLeadName)
	}
	if model.Approvers.DepartmentHeadName != "Siti Rahma" {
		t.Errorf("expected department head 'Siti Rahma', got '%s'", model.Approvers.DepartmentHeadName)
	}
	if !model.Approvers.IsSigned {
		t.Error("expected IsSigned to be true")
	}
}
