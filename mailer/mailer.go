package mailer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/time/rate"
	gomail "gopkg.in/gomail.v2"

	"timesheet-backend/config"
)

const defaultTimeFormatWIB = "02 Jan 2006, 15:04 WIB"

// Mailer sends transactional and delivery email over SMTP.
type Mailer struct {
	cfg     *config.Config
	limiter *rate.Limiter
}

// New constructs a Mailer.
func New(cfg *config.Config) *Mailer {
	var limiter *rate.Limiter
	if cfg != nil {
		rateLimit := cfg.MailerRateLimit
		if rateLimit <= 0 {
			rateLimit = 10.0
		}
		limiter = rate.NewLimiter(rate.Limit(rateLimit), int(rateLimit))
	}
	return &Mailer{
		cfg:     cfg,
		limiter: limiter,
	}
}

func (m *Mailer) dialer() *gomail.Dialer {
	// Dev relays and internal servers speak plain SMTP without auth; gomail only
	// attempts auth when a username is configured.
	return gomail.NewDialer(m.cfg.SMTPHost, m.cfg.SMTPPort, m.cfg.SMTPUser, m.cfg.SMTPPass)
}

const (
	maxSendRetries = 3
	smtpTimeout    = 10 * time.Second
)

func (m *Mailer) send(msg *gomail.Message) error {
	if m.cfg == nil || strings.TrimSpace(m.cfg.SMTPHost) == "" {
		log.Printf("[mailer] SMTPHost not configured; skipping email dispatch")
		return fmt.Errorf("smtp host not configured")
	}

	if m.limiter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.limiter.Wait(ctx); err != nil {
			log.Printf("[mailer] rate limiter error: %v", err)
		}
	}

	msg.SetHeader("Auto-Submitted", "auto-generated")
	msg.SetHeader("X-Mailer", "Timesheet-Portal-Mailer")

	dialer := m.dialer()
	var lastErr error
	backoff := 50 * time.Millisecond

	for attempt := 1; attempt <= maxSendRetries; attempt++ {
		done := make(chan error, 1)
		go func() {
			done <- dialer.DialAndSend(msg)
		}()

		select {
		case err := <-done:
			if err != nil {
				lastErr = err
				if attempt < maxSendRetries {
					time.Sleep(backoff)
					backoff *= 2
				}
				continue
			}
			return nil
		case <-time.After(smtpTimeout):
			lastErr = fmt.Errorf("smtp send timeout after %s", smtpTimeout)
			if attempt < maxSendRetries {
				time.Sleep(backoff)
				backoff *= 2
			}
			continue
		}
	}

	log.Printf("[mailer] failed to send mail after %d attempts: %v", maxSendRetries, lastErr)
	return lastErr
}

func (m *Mailer) appName() string {
	if m.cfg != nil && m.cfg.AppName != "" {
		return m.cfg.AppName
	}
	if m.cfg != nil && m.cfg.RPDisplayName != "" {
		return m.cfg.RPDisplayName
	}
	return "Timesheet Portal"
}

func (m *Mailer) frontendURL() string {
	if m.cfg != nil && m.cfg.FrontendURL != "" {
		return m.cfg.FrontendURL
	}
	return ""
}

func (m *Mailer) supportEmail() string {
	if m.cfg != nil && m.cfg.AdminEmail != "" {
		return m.cfg.AdminEmail
	}
	if m.cfg != nil && m.cfg.MailFrom != "" {
		if addr, err := mail.ParseAddress(m.cfg.MailFrom); err == nil && addr.Address != "" {
			return addr.Address
		}
	}
	return ""
}

func (m *Mailer) timeLocation() *time.Location {
	if m.cfg != nil && m.cfg.Timezone != "" {
		if loc, err := time.LoadLocation(m.cfg.Timezone); err == nil {
			return loc
		}
	}
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*3600)
}

func (m *Mailer) setMessageContent(msg *gomail.Message, htmlBody, textBody string) {
	if textBody != "" {
		msg.SetBody("text/plain", textBody)
		if htmlBody != "" {
			msg.AddAlternative("text/html", htmlBody)
		}
	} else if htmlBody != "" {
		msg.SetBody("text/html", htmlBody)
	}
}

// SendSetupEmail delivers initial credentials + a setup link to a new user.
func (m *Mailer) SendSetupEmail(to, username, setupLink string) error {
	loc := m.timeLocation()
	now := time.Now().In(loc)
	expiresAt := now.Add(7 * 24 * time.Hour).Format(defaultTimeFormatWIB)

	htmlBody, textBody, err := RenderSetupEmail(SetupEmailData{
		AppName:      m.appName(),
		Username:     username,
		Email:        to,
		SetupURL:     setupLink,
		ExpireDays:   7,
		ExpiresAt:    expiresAt,
		SupportEmail: m.supportEmail(),
		PortalURL:    m.frontendURL(),
	})
	if err != nil {
		log.Printf("[mailer] failed to render setup email template: %v", err)
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("Selamat Datang di %s", m.appName()))
	m.setMessageContent(msg, htmlBody, textBody)
	return m.send(msg)
}

// SendAccountWelcomeEmail delivers an activation notification with initial password and instructions to a newly created user,
// completely separated from the password reset flow.
func (m *Mailer) SendAccountWelcomeEmail(to, username, initialPassword, loginLink string) error {
	if loginLink == "" && m.frontendURL() != "" {
		loginLink = strings.TrimRight(m.frontendURL(), "/") + "/login"
	}

	loc := m.timeLocation()
	now := time.Now().In(loc)
	expiresAt := now.Add(30 * 24 * time.Hour).Format(defaultTimeFormatWIB)

	htmlBody, textBody, err := RenderSetupEmail(SetupEmailData{
		AppName:         m.appName(),
		Username:        username,
		Email:           to,
		InitialPassword: initialPassword,
		SetupURL:        loginLink,
		ExpireDays:      30,
		ExpiresAt:       expiresAt,
		SupportEmail:    m.supportEmail(),
		PortalURL:       m.frontendURL(),
	})
	if err != nil {
		log.Printf("[mailer] failed to render welcome email template: %v", err)
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("Selamat Datang di %s", m.appName()))
	m.setMessageContent(msg, htmlBody, textBody)
	return m.send(msg)
}

// SendResetEmail delivers a password-reset link.
func (m *Mailer) SendResetEmail(to, resetLink string) error {
	return m.SendResetEmailWithUser(to, "", resetLink)
}

// SendResetEmailWithUser delivers a password-reset link with personalized greeting.
func (m *Mailer) SendResetEmailWithUser(to, username, resetLink string) error {
	ttl := 60 * time.Minute
	if m.cfg != nil && m.cfg.ResetTokenTTL > 0 {
		ttl = m.cfg.ResetTokenTTL
	}
	expireMinutes := int(ttl.Minutes())

	loc := m.timeLocation()
	now := time.Now().In(loc)
	requestedAt := now.Format(defaultTimeFormatWIB)
	expiresAt := now.Add(ttl).Format(defaultTimeFormatWIB)

	htmlBody, textBody, err := RenderResetEmail(ResetEmailData{
		AppName:       m.appName(),
		Username:      username,
		Email:         to,
		ResetURL:      resetLink,
		ExpireMinutes: expireMinutes,
		RequestedAt:   requestedAt,
		ExpiresAt:     expiresAt,
		SupportEmail:  m.supportEmail(),
		PortalURL:     m.frontendURL(),
	})
	if err != nil {
		log.Printf("[mailer] failed to render reset email template: %v", err)
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("Reset Kata Sandi Akun %s", m.appName()))
	m.setMessageContent(msg, htmlBody, textBody)
	return m.send(msg)
}

// SendTimesheetEmail attaches the generated timesheet file and sends it to the
// user's registered address.
func (m *Mailer) SendTimesheetEmail(to, company, filename string, data []byte) error {
	period, userFromName := ParsePeriodFromFilename(filename)
	return m.SendTimesheetEmailWithDetails(to, userFromName, company, period, filename, data)
}

// SendTimesheetEmailWithDetails delivers timesheet email with explicit details.
func (m *Mailer) SendTimesheetEmailWithDetails(to, username, company, period, filename string, data []byte) error {
	comp := strings.TrimSpace(company)
	subject := "Dokumen Timesheet Anda Telah Siap"
	if comp != "" {
		subject = fmt.Sprintf("Dokumen Timesheet %s Anda Telah Siap", comp)
	}

	htmlBody, textBody, err := RenderTimesheetEmail(TimesheetEmailData{
		AppName:      m.appName(),
		Username:     username,
		Email:        to,
		Company:      comp,
		Period:       period,
		Filename:     filename,
		PortalURL:    m.frontendURL(),
		SupportEmail: m.supportEmail(),
	})
	if err != nil {
		log.Printf("[mailer] failed to render timesheet email template: %v", err)
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)
	m.setMessageContent(msg, htmlBody, textBody)
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

// SendTimesheetReadyEmail sends an email notification containing a secure presigned download link.
func (m *Mailer) SendTimesheetReadyEmail(to, username, company, period, filename, downloadURL string, expiresAt time.Time) error {
	comp := strings.TrimSpace(company)
	subject := "Dokumen Timesheet Anda Telah Siap"
	if comp != "" {
		subject = fmt.Sprintf("Dokumen Timesheet %s Anda Telah Siap", comp)
	}

	expStr := ""
	if !expiresAt.IsZero() {
		expStr = expiresAt.In(m.timeLocation()).Format(defaultTimeFormatWIB)
	}

	htmlBody, textBody, err := RenderTimesheetEmail(TimesheetEmailData{
		AppName:      m.appName(),
		Username:     username,
		Email:        to,
		Company:      comp,
		Period:       period,
		Filename:     filename,
		PortalURL:    m.frontendURL(),
		SupportEmail: m.supportEmail(),
		DownloadURL:  downloadURL,
		ExpiresAt:    expStr,
	})
	if err != nil {
		log.Printf("[mailer] failed to render timesheet ready email template: %v", err)
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)
	m.setMessageContent(msg, htmlBody, textBody)

	if err := m.send(msg); err != nil {
		return err
	}
	log.Printf("[mailer] timesheet ready email successfully sent to %s (file: %s)", to, filename)
	return nil
}

// SendReminderEmail delivers a daily reminder to fill today's timesheet.
func (m *Mailer) SendReminderEmail(to, username, dateStr, activityURL string) error {
	if activityURL == "" && m.frontendURL() != "" {
		activityURL = strings.TrimRight(m.frontendURL(), "/") + "/activity"
	}

	htmlBody, textBody, err := RenderReminderEmail(ReminderEmailData{
		AppName:      m.appName(),
		Username:     username,
		Email:        to,
		Date:         dateStr,
		ActivityURL:  activityURL,
		PortalURL:    m.frontendURL(),
		SupportEmail: m.supportEmail(),
	})
	if err != nil {
		log.Printf("[mailer] failed to render reminder email template: %v", err)
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("Pengingat Harian: Isi Timesheet Hari Ini (%s)", dateStr))
	m.setMessageContent(msg, htmlBody, textBody)
	return m.send(msg)
}

// SendPasswordChangedEmail delivers a security notice confirming password was updated.
func (m *Mailer) SendPasswordChangedEmail(to, username string) error {
	loginURL := ""
	if m.frontendURL() != "" {
		loginURL = strings.TrimRight(m.frontendURL(), "/") + "/login"
	}

	htmlBody, textBody, err := RenderPasswordChangedEmail(PasswordChangedEmailData{
		AppName:      m.appName(),
		Username:     username,
		Email:        to,
		ChangedAt:    time.Now().Format(defaultTimeFormatWIB),
		LoginURL:     loginURL,
		SupportEmail: m.supportEmail(),
		PortalURL:    m.frontendURL(),
	})
	if err != nil {
		log.Printf("[mailer] failed to render password changed email template: %v", err)
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.MailFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("Keamanan Akun: Kata Sandi %s Anda Berhasil Diperbarui", m.appName()))
	m.setMessageContent(msg, htmlBody, textBody)
	return m.send(msg)
}
