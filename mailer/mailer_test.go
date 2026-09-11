package mailer

import (
	"testing"

	"timesheet-backend/config"
)

func TestMailer_NewAndDialer(t *testing.T) {
	cfg := &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		SMTPUser: "user",
		SMTPPass: "pass",
		MailFrom: "no-reply@example.com",
	}

	m := New(cfg)
	if m == nil {
		t.Fatal("expected non-nil mailer")
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
		SMTPHost: "127.0.0.1",
		SMTPPort: 1,
		MailFrom: "no-reply@example.com",
	}

	m := New(cfg)

	// SendSetupEmail
	err := m.SendSetupEmail("test@example.com", "testuser", "http://localhost/setup")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendSetupEmail")
	}

	// SendResetEmail
	err = m.SendResetEmail("test@example.com", "http://localhost/reset")
	if err == nil {
		t.Error("expected error dialing unreachable port for SendResetEmail")
	}

	// SendTimesheetEmail
	err = m.SendTimesheetEmail("test@example.com", "MII", "timesheet.xlsx", []byte("fake-excel-data"))
	if err == nil {
		t.Error("expected error dialing unreachable port for SendTimesheetEmail")
	}
}
