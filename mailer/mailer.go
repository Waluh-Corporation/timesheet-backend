package mailer

import (
	"fmt"
	"io"
	"log"
	"strings"

	gomail "gopkg.in/gomail.v2"

	"timesheet-backend/config"
)

const contentTypeHTML = "text/html"

// Mailer sends transactional and delivery email over SMTP.
type Mailer struct {
	cfg *config.Config
}

// New constructs a Mailer.
func New(cfg *config.Config) *Mailer {
	return &Mailer{cfg: cfg}
}

func (m *Mailer) dialer() *gomail.Dialer {
	// Mailpit and other dev relays speak plain SMTP without auth; gomail only
	// attempts auth when a username is configured.
	return gomail.NewDialer(m.cfg.SMTPHost, m.cfg.SMTPPort, m.cfg.SMTPUser, m.cfg.SMTPPass)
}

func (m *Mailer) send(msg *gomail.Message) error {
	if err := m.dialer().DialAndSend(msg); err != nil {
		log.Printf("[mailer] failed to send mail: %v", err)
		return err
	}
	return nil
}

// SendSetupEmail delivers initial credentials + a setup link to a new user.
func (m *Mailer) SendSetupEmail(to, username, setupLink string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Your Timesheet Portal account is ready")
	body := fmt.Sprintf(
		`<p>Hi %s,</p>
<p>An administrator has created a Timesheet Portal account for you.</p>
<p>Use the link below to set your password and (optionally) register a passkey:</p>
<p><a href="%s">%s</a></p>
<p>If you were not expecting this email you can safely ignore it.</p>`,
		username, setupLink, setupLink,
	)
	msg.SetBody(contentTypeHTML, body)
	return m.send(msg)
}

// SendResetEmail delivers a password-reset link.
func (m *Mailer) SendResetEmail(to, resetLink string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Reset your Timesheet Portal password")
	body := fmt.Sprintf(
		`<p>We received a request to reset your Timesheet Portal password.</p>
<p><a href="%s">Click here to choose a new password.</a></p>
<p>This link expires soon. If you did not request a reset, ignore this email.</p>`,
		resetLink,
	)
	msg.SetBody(contentTypeHTML, body)
	return m.send(msg)
}

// SendTimesheetEmail attaches the generated timesheet file and sends it to the
// user's registered address.
func (m *Mailer) SendTimesheetEmail(to, company, filename string, data []byte) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)

	subject := "Your Timesheet is Ready"
	if trimmed := strings.TrimSpace(company); trimmed != "" {
		subject = fmt.Sprintf("Your %s Timesheet is Ready", trimmed)
	}
	msg.SetHeader("Subject", subject)
	msg.SetBody(contentTypeHTML, "<p>Attached is your generated timesheet. A copy has also been downloaded in your browser.</p>")
	msg.Attach(filename, gomail.SetCopyFunc(func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}))
	if err := m.send(msg); err != nil {
		return err
	}
	log.Printf("[mailer] timesheet email successfully sent to %s (file: %s)", to, filename)
	return nil
}
