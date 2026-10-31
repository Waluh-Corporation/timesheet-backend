package template

import (
	_ "embed"
	"fmt"
	"time"
)

// ResetEmailData holds data needed to render a password reset email.
type ResetEmailData struct {
	AppName       string
	Username      string
	Email         string
	ResetURL      string
	ExpireMinutes int
	ExpiresAt     string
	SupportEmail  string
	PortalURL     string
	RequestedAt   string
	IPAddress     string
	Location      string
}

//go:embed reset.html
var resetHTML string

var resetTmpl = buildEmailTemplate(resetHTML)

// RenderResetEmail generates both HTML and plain-text password reset email content.
func RenderResetEmail(data ResetEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.ExpireMinutes <= 0 {
		data.ExpireMinutes = 60
	}
	if data.RequestedAt == "" {
		data.RequestedAt = time.Now().Format("02 Jan 2006, 15:04 WIB")
	}
	if data.ExpiresAt == "" {
		data.ExpiresAt = time.Now().Add(time.Duration(data.ExpireMinutes) * time.Minute).Format("02 Jan 2006, 15:04 WIB")
	}

	greetingName := data.Username
	if greetingName == "" {
		greetingName = "User"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Password reset instructions for your %s account.", data.AppName),
		Subject:      fmt.Sprintf("Reset Your %s Password", data.AppName),
		HeaderTitle:  "Password Reset Request",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, resetTmpl, data)
	if err != nil {
		return "", "", err
	}

	ipLine := ""
	if data.IPAddress != "" {
		ipLine = fmt.Sprintf("- IP Address: %s\n", data.IPAddress)
	}
	locLine := ""
	if data.Location != "" {
		locLine = fmt.Sprintf("- Approximate Location: %s\n", data.Location)
	}

	textBody = fmt.Sprintf(`Hello %s,

We received a request to reset the password for your %s account.
Use the link below to set a new password:

%s

Request Details:
- Requested At: %s
%s%s
IMPORTANT: This link is confidential and will expire at %s. Do not share this link with anyone.
If you did not request a password reset, you can safely ignore this email; your account remains secure.

--
%s
`, greetingName, data.AppName, data.ResetURL, data.RequestedAt, ipLine, locLine, data.ExpiresAt, data.AppName)

	return htmlBody, textBody, nil
}
