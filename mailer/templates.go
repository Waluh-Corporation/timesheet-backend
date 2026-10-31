package mailer

import (
	"timesheet-backend/mailer/template"
)

// Re-export template types and render functions for backward compatibility.
type (
	BaseLayoutData               = template.BaseLayoutData
	SetupEmailData               = template.SetupEmailData
	ResetEmailData               = template.ResetEmailData
	TimesheetEmailData           = template.TimesheetEmailData
	ReminderEmailData            = template.ReminderEmailData
	PasswordChangedEmailData     = template.PasswordChangedEmailData
	WitnessRequestEmailData      = template.WitnessRequestEmailData
	BeritaAcaraApprovedEmailData = template.BeritaAcaraApprovedEmailData
	BeritaAcaraRejectedEmailData = template.BeritaAcaraRejectedEmailData
)

var (
	RenderSetupEmail               = template.RenderSetupEmail
	RenderResetEmail               = template.RenderResetEmail
	RenderTimesheetEmail           = template.RenderTimesheetEmail
	RenderReminderEmail            = template.RenderReminderEmail
	RenderPasswordChangedEmail     = template.RenderPasswordChangedEmail
	RenderWitnessRequestEmail      = template.RenderWitnessRequestEmail
	RenderBeritaAcaraApprovedEmail = template.RenderBeritaAcaraApprovedEmail
	RenderBeritaAcaraRejectedEmail = template.RenderBeritaAcaraRejectedEmail
	FormatMonthYearEnglish         = template.FormatMonthYearEnglish
	FormatMonthYearIndonesian      = template.FormatMonthYearIndonesian
	ParsePeriodFromFilename        = template.ParsePeriodFromFilename
)
