package template

import (
	_ "embed"
	"fmt"
	"time"
)

// PasswordChangedEmailData holds data needed to render a password change notification email.
type PasswordChangedEmailData struct {
	AppName      string
	Username     string
	Email        string
	ChangedAt    string
	LoginURL     string
	SupportEmail string
	PortalURL    string
	IPAddress    string
	Location     string
}

//go:embed password_changed.html
var passwordChangedHTML string

var passwordChangedTmpl = buildEmailTemplate(passwordChangedHTML)

// RenderPasswordChangedEmail generates both HTML and plain-text password changed notification email content.
func RenderPasswordChangedEmail(data PasswordChangedEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.ChangedAt == "" {
		data.ChangedAt = time.Now().Format("02 Jan 2006, 15:04 WIB")
	}

	greetingName := data.Username
	if greetingName == "" {
		greetingName = "User"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Your %s account password has been successfully updated.", data.AppName),
		Subject:      fmt.Sprintf("Security Alert: Your %s Password Has Been Updated", data.AppName),
		HeaderTitle:  "Password Updated Successfully",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, passwordChangedTmpl, data)
	if err != nil {
		return "", "", err
	}

	ipLine := ""
	if data.IPAddress != "" {
		ipLine = fmt.Sprintf("\n- IP Address: %s", data.IPAddress)
	}
	locLine := ""
	if data.Location != "" {
		locLine = fmt.Sprintf("\n- Approximate Location: %s", data.Location)
	}

	textBody = fmt.Sprintf(`Hello %s,

Your %s account password was successfully updated at %s.%s%s

If you made this change, you can log back in with your new password:
%s

SECURITY NOTICE:
If you did NOT make this change, please contact your system administrator immediately to secure your account.

--
%s
`, greetingName, data.AppName, data.ChangedAt, ipLine, locLine, data.LoginURL, data.AppName)

	return htmlBody, textBody, nil
}
