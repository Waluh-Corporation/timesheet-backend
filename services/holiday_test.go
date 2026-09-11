package services

import (
	"testing"
)

func TestGetDaysInMonth(t *testing.T) {
	tests := []struct {
		year     int
		month    int
		expected int
	}{
		{2026, 1, 31},
		{2026, 2, 28},
		{2026, 4, 30},
		{2024, 2, 29}, // leap year
	}

	for _, tc := range tests {
		got := GetDaysInMonth(tc.year, tc.month)
		if got != tc.expected {
			t.Errorf("GetDaysInMonth(%d, %d) = %d, expected %d", tc.year, tc.month, got, tc.expected)
		}
	}
}

func TestFetchHolidaysKemendesa(t *testing.T) {
	// Integration test with Kemendesa API for 2026
	holidays2026, err := FetchHolidaysByYear(2026)
	if err != nil {
		t.Skipf("Skipping live API test due to network availability: %v", err)
		return
	}

	if len(holidays2026) == 0 {
		t.Fatalf("Expected holidays for 2026, got 0")
	}

	// Verify field mapping
	foundCutiBersama := false
	for _, h := range holidays2026 {
		if h.Date == "" {
			t.Errorf("Expected date to not be empty")
		}
		if h.Description == "" {
			t.Errorf("Expected description to not be empty")
		}
		if h.IsJointLeave {
			foundCutiBersama = true
		}
	}

	if !foundCutiBersama {
		t.Errorf("Expected at least one cuti bersama in 2026")
	}

	// Test monthly filter
	augHolidays, err := FetchHolidays(2026, 8)
	if err != nil {
		t.Fatalf("Failed to fetch August 2026 holidays: %v", err)
	}

	foundIndependenceDay := false
	for _, h := range augHolidays {
		if h.Date == "2026-08-17" {
			foundIndependenceDay = true
			if !h.IsCivic {
				t.Errorf("Expected 2026-08-17 to have IsCivic == true")
			}
		}
	}

	if !foundIndependenceDay {
		t.Errorf("Expected 2026-08-17 (Proklamasi Kemerdekaan) in August 2026 holidays")
	}
}
