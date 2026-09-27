package mailer

import (
	"testing"
	"time"

	"timesheet-backend/config"
)

func TestMailer_NewAndDialer(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:      "smtp.example.com",
		SMTPPort:      587,
		SMTPUser:      "user",
		SMTPPass:      "pass",
		MailFrom:      "Timesheet Portal <no-reply@example.com>",
		RPDisplayName: "Timesheet Corp",
		FrontendURL:   "https://timesheet.example.com",
		ResetTokenTTL: 45 * time.Minute,
	}

	m := New(cfg)
	if m == nil {
		t.Fatal("expected non-nil mailer")
	}

	if m.appName() != "Timesheet Corp" {
		t.Errorf("expected appName Timesheet Corp, got %s", m.appName())
	}
	if m.frontendURL() != "https://timesheet.example.com" {
		t.Errorf("expected frontendURL https://timesheet.example.com, got %s", m.frontendURL())
	}
	if m.supportEmail() != "no-reply@example.com" {
		t.Errorf("expected supportEmail no-reply@example.com, got %s", m.supportEmail())
	}

	// Test fallback helpers when config fields are empty
	emptyMailer := New(&config.Config{})
	if emptyMailer.appName() != "Timesheet Portal" {
		t.Errorf("expected default appName, got %s", emptyMailer.appName())
	}
	if emptyMailer.supportEmail() != "" {
		t.Errorf("expected empty supportEmail on empty MailFrom, got %s", emptyMailer.supportEmail())
	}
	if loc := emptyMailer.timeLocation(); loc == nil {
		t.Error("expected non-nil time location")
	}

	d := m.dialer()
	if d == nil {
		t.Fatal("expected non-nil dialer")
	}
	if d.Host != "smtp.example.com" {
		t.Errorf("expected host smtp.example.com, got %s", d.Host)
	}
	if d.Port != 587 {
		t.Errorf("expected port 587, got %d", d.Port)
	}
}

func TestMailer_SendMethodsFailGracefullyWithoutServer(t *testing.T) {
	// Points to unreachable port so network dial fails fast without crashing
	cfg := &config.Config{
		SMTPHost:      "127.0.0.1",
		SMTPPort:      1,
		MailFrom:      "no-reply@example.com",
		ResetTokenTTL: 60 * time.Minute,
	}

	m := New(cfg)

	// SendSetupEmail
	err := m.SendSetupEmail("test@example.com", "testuser", "http://localhost/setup")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendSetupEmail")
	}

	// SendAccountWelcomeEmail
	err = m.SendAccountWelcomeEmail("test@example.com", "testuser", "InitialSecret123!", "http://localhost/login")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendAccountWelcomeEmail")
	}

	// SendResetEmail
	err = m.SendResetEmail("test@example.com", "http://localhost/reset")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendResetEmail")
	}

	// SendResetEmailWithUser
	err = m.SendResetEmailWithUser("test@example.com", "testuser", "http://localhost/reset")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendResetEmailWithUser")
	}

	// SendResetEmailWithIP
	err = m.SendResetEmailWithIP("test@example.com", "testuser", "http://localhost/reset", "192.168.1.1")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendResetEmailWithIP")
	}

	// SendResetEmailWithDetails
	err = m.SendResetEmailWithDetails("test@example.com", "testuser", "http://localhost/reset", "192.168.1.1", "Jakarta, Indonesia")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendResetEmailWithDetails")
	}

	// SendTimesheetEmail
	err = m.SendTimesheetEmail("test@example.com", "MII", "Timesheet_testuser_09_2026.xlsx", []byte("fake-excel-data"))
	if err == nil {
		t.Error("expected error dialing unreachable port for SendTimesheetEmail")
	}

	// SendTimesheetEmailWithDetails
	err = m.SendTimesheetEmailWithDetails("test@example.com", "testuser", "PT Mitra Integrasi Informatika", "September 2026", "Timesheet_testuser_09_2026.xlsx", []byte("fake-excel-data"))
	if err == nil {
		t.Error("expected error dialing unreachable port for SendTimesheetEmailWithDetails")
	}

	// SendReminderEmail
	err = m.SendReminderEmail("test@example.com", "testuser", "13 September 2026", "http://localhost/activity")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendReminderEmail")
	}

	// SendPasswordChangedEmail
	err = m.SendPasswordChangedEmail("test@example.com", "testuser")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendPasswordChangedEmail")
	}

	// SendPasswordChangedEmailWithIP
	err = m.SendPasswordChangedEmailWithIP("test@example.com", "testuser", "192.168.1.1")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendPasswordChangedEmailWithIP")
	}

	// SendPasswordChangedEmailWithDetails
	err = m.SendPasswordChangedEmailWithDetails("test@example.com", "testuser", "192.168.1.1", "Jakarta, Indonesia")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendPasswordChangedEmailWithDetails")
	}

	// SendTimesheetReadyEmail
	err = m.SendTimesheetReadyEmail("test@example.com", "testuser", "MII", "September 2026", "Timesheet.xlsx", "https://timesheet.example.com/dl", time.Now().Add(24*time.Hour))
	if err == nil {
		t.Error("expected error dialing unreachable port for SendTimesheetReadyEmail")
	}
}
