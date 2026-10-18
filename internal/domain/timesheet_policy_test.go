package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTimesheetPolicy_Context(t *testing.T) {
	ctx := context.Background()
	if IsAdminFromContext(ctx) {
		t.Errorf("expected non-admin on empty context")
	}

	userCtx := WithUserRole(ctx, "user")
	if IsAdminFromContext(userCtx) {
		t.Errorf("expected non-admin on user role context")
	}

	adminCtx := WithUserRole(ctx, "admin")
	if !IsAdminFromContext(adminCtx) {
		t.Errorf("expected admin on admin role context")
	}
}

func assertExpectedError(t *testing.T, err error, wantErr error) {
	t.Helper()
	if wantErr == nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected error wrapping %v, got nil", wantErr)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error wrapping %v, got: %v", wantErr, err)
	}
}

func TestValidateTimesheetDate(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	// Reference time: 2026-10-15 12:00:00 WIB (Current month = October 2026)
	ref := time.Date(2026, 10, 15, 12, 0, 0, 0, loc)

	tests := []struct {
		name    string
		date    time.Time
		wantErr error
	}{
		{
			name:    "first day of previous month (September 1, 2026)",
			date:    time.Date(2026, 9, 1, 0, 0, 0, 0, loc),
			wantErr: nil,
		},
		{
			name:    "last day of previous month (September 30, 2026)",
			date:    time.Date(2026, 9, 30, 23, 59, 59, 0, loc),
			wantErr: nil,
		},
		{
			name:    "first day of current month (October 1, 2026)",
			date:    time.Date(2026, 10, 1, 0, 0, 0, 0, loc),
			wantErr: nil,
		},
		{
			name:    "last day of current month (October 31, 2026)",
			date:    time.Date(2026, 10, 31, 23, 59, 59, 0, loc),
			wantErr: nil,
		},
		{
			name:    "day before previous month (August 31, 2026)",
			date:    time.Date(2026, 8, 31, 23, 59, 59, 0, loc),
			wantErr: ErrInvalidInput,
		},
		{
			name:    "two months ago (August 15, 2026)",
			date:    time.Date(2026, 8, 15, 10, 0, 0, 0, loc),
			wantErr: ErrInvalidInput,
		},
		{
			name:    "future month (November 1, 2026)",
			date:    time.Date(2026, 11, 1, 0, 0, 0, 0, loc),
			wantErr: ErrInvalidInput,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTimesheetDate(tc.date, ref, loc)
			assertExpectedError(t, err, tc.wantErr)
		})
	}
}

func TestValidateTimesheetDate_YearBoundary(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	// Reference time: 2027-01-05 10:00:00 WIB (Current month = January 2027, previous month = December 2026)
	ref := time.Date(2027, 1, 5, 10, 0, 0, 0, loc)

	// Dec 1, 2026 -> valid
	if err := ValidateTimesheetDate(time.Date(2026, 12, 1, 0, 0, 0, 0, loc), ref, loc); err != nil {
		t.Errorf("expected Dec 1, 2026 to be valid, got: %v", err)
	}

	// Nov 30, 2026 -> invalid (older than previous month)
	if err := ValidateTimesheetDate(time.Date(2026, 11, 30, 23, 59, 59, 0, loc), ref, loc); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected Nov 30, 2026 to be invalid, got: %v", err)
	}

	// Jan 31, 2027 -> valid
	if err := ValidateTimesheetDate(time.Date(2027, 1, 31, 23, 59, 59, 0, loc), ref, loc); err != nil {
		t.Errorf("expected Jan 31, 2027 to be valid, got: %v", err)
	}

	// Feb 1, 2027 -> invalid (future month)
	if err := ValidateTimesheetDate(time.Date(2027, 2, 1, 0, 0, 0, 0, loc), ref, loc); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected Feb 1, 2027 to be invalid, got: %v", err)
	}
}

func TestValidateTimesheetPeriod(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600)
	// Reference: 2026-10-20
	ref := time.Date(2026, 10, 20, 10, 0, 0, 0, loc)

	tests := []struct {
		name    string
		month   int
		year    int
		wantErr error
	}{
		{"current month (10/2026)", 10, 2026, nil},
		{"previous month (9/2026)", 9, 2026, nil},
		{"two months ago (8/2026)", 8, 2026, ErrInvalidInput},
		{"future month (11/2026)", 11, 2026, ErrInvalidInput},
		{"previous year (10/2025)", 10, 2025, ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTimesheetPeriod(tc.month, tc.year, ref, loc)
			assertExpectedError(t, err, tc.wantErr)
		})
	}
}
