package mailer

import (
	"timesheet-backend/mailer/template"
)

// Re-export template types and render functions for backward compatibility.
type (
	BaseLayoutData           = template.BaseLayoutData
	SetupEmailData           = template.SetupEmailData
	ResetEmailData           = template.ResetEmailData
	TimesheetEmailData       = template.TimesheetEmailData
	ReminderEmailData        = template.ReminderEmailData
	PasswordChangedEmailData = template.PasswordChangedEmailData
)

var (
	RenderSetupEmail           = template.RenderSetupEmail
	RenderResetEmail           = template.RenderResetEmail
	RenderTimesheetEmail       = template.RenderTimesheetEmail
	RenderReminderEmail        = template.RenderReminderEmail
	RenderPasswordChangedEmail = template.RenderPasswordChangedEmail
	FormatMonthYearIndonesian  = template.FormatMonthYearIndonesian
	ParsePeriodFromFilename    = template.ParsePeriodFromFilename
)
