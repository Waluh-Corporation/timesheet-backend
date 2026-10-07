package domain

import (
	"context"
	"time"
)

type contextKey string

const (
	// CtxRoleKey is the context key for the caller's role string.
	CtxRoleKey contextKey = "user_role"
)

// WithUserRole injects the user's role into the context.
func WithUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, CtxRoleKey, role)
}

// IsAdminFromContext checks if the caller in ctx has the admin role.
func IsAdminFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	role, ok := ctx.Value(CtxRoleKey).(string)
	return ok && role == "admin"
}

// AllowedTimesheetDateRange returns the inclusive start and end timestamps (in the given location)
// for which timesheet activities and overtime entries may be recorded.
// The allowed window spans from the 1st day of the previous month 00:00:00 up to the last day
// of the current month 23:59:59.999999999.
func AllowedTimesheetDateRange(ref time.Time, loc *time.Location) (time.Time, time.Time) {
	if loc == nil {
		loc = time.Local
	}
	r := ref.In(loc)
	startOfPrevMonth := time.Date(r.Year(), r.Month()-1, 1, 0, 0, 0, 0, loc)
	startOfNextMonth := time.Date(r.Year(), r.Month()+1, 1, 0, 0, 0, 0, loc)
	endOfCurrentMonth := startOfNextMonth.Add(-time.Nanosecond)
	return startOfPrevMonth, endOfCurrentMonth
}

// ValidateTimesheetDate checks whether target date t is within the allowed timesheet window
// (from 1st of previous month up to the end of the current month).
func ValidateTimesheetDate(t time.Time, ref time.Time, loc *time.Location) error {
	if loc == nil {
		loc = time.Local
	}
	startOfPrevMonth, endOfCurrentMonth := AllowedTimesheetDateRange(ref, loc)
	target := t.In(loc)

	if target.Before(startOfPrevMonth) {
		return NewUserError(ErrInvalidInput, "timesheet date cannot be older than the previous month (maximum 1 month)")
	}
	if target.After(endOfCurrentMonth) {
		return NewUserError(ErrInvalidInput, "timesheet date cannot be in future months")
	}
	return nil
}

// ValidateTimesheetPeriod checks whether the (month, year) pair is either the current month
// or the previous month (maximum 1 month back).
func ValidateTimesheetPeriod(month, year int, ref time.Time, loc *time.Location) error {
	if loc == nil {
		loc = time.Local
	}
	r := ref.In(loc)
	currMonthStart := time.Date(r.Year(), r.Month(), 1, 0, 0, 0, 0, loc)
	prevMonthStart := time.Date(r.Year(), r.Month()-1, 1, 0, 0, 0, 0, loc)

	reqStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)

	if reqStart.Before(prevMonthStart) {
		return NewUserError(ErrInvalidInput, "timesheet generation cannot be older than the previous month (maximum 1 month)")
	}
	if reqStart.After(currMonthStart) {
		return NewUserError(ErrInvalidInput, "timesheet generation cannot be in future months")
	}
	return nil
}
