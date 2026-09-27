package mailer

import (
	"strings"
	"testing"
)

func TestReExportedRenderFunctions(t *testing.T) {
	t.Run("RenderSetupEmail", func(t *testing.T) {
		html, text, err := RenderSetupEmail(SetupEmailData{Username: "reexport_user"})
		if err != nil || !strings.Contains(html, "reexport_user") || !strings.Contains(text, "reexport_user") {
			t.Fatalf("unexpected result: %v", err)
		}
	})

	t.Run("RenderResetEmail", func(t *testing.T) {
		html, text, err := RenderResetEmail(ResetEmailData{Username: "reexport_user", IPAddress: "10.0.0.1", Location: "Jakarta"})
		if err != nil || !strings.Contains(html, "10.0.0.1") || !strings.Contains(text, "Jakarta") {
			t.Fatalf("unexpected result: %v", err)
		}
	})

	t.Run("RenderTimesheetEmail", func(t *testing.T) {
		html, text, err := RenderTimesheetEmail(TimesheetEmailData{Company: "PT Reexport"})
		if err != nil || !strings.Contains(html, "PT Reexport") || !strings.Contains(text, "PT Reexport") {
			t.Fatalf("unexpected result: %v", err)
		}
	})

	t.Run("RenderReminderEmail", func(t *testing.T) {
		html, text, err := RenderReminderEmail(ReminderEmailData{Username: "reexport_user"})
		if err != nil || !strings.Contains(html, "reexport_user") || !strings.Contains(text, "reexport_user") {
			t.Fatalf("unexpected result: %v", err)
		}
	})

	t.Run("RenderPasswordChangedEmail", func(t *testing.T) {
		html, text, err := RenderPasswordChangedEmail(PasswordChangedEmailData{Username: "reexport_user", IPAddress: "10.0.0.2", Location: "Surabaya"})
		if err != nil || !strings.Contains(html, "10.0.0.2") || !strings.Contains(text, "Surabaya") {
			t.Fatalf("unexpected result: %v", err)
		}
	})

	t.Run("FormatMonthYearIndonesian", func(t *testing.T) {
		if got := FormatMonthYearIndonesian(9, 2026); got != "September 2026" {
			t.Errorf("got %q, want 'September 2026'", got)
		}
	})

	t.Run("ParsePeriodFromFilename", func(t *testing.T) {
		period, user := ParsePeriodFromFilename("Timesheet_test_09_2026.xlsx")
		if period != "September 2026" || user != "test" {
			t.Errorf("got (%q, %q)", period, user)
		}
	})
}
