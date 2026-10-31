package template

import (
	_ "embed"
	"fmt"
	"time"
)

// SetupEmailData holds data needed to render an account welcome/setup email.
type SetupEmailData struct {
	AppName         string
	Username        string
	Email           string
	InitialPassword string
	SetupURL        string
	ExpireDays      int
	ExpiresAt       string
	SupportEmail    string
	PortalURL       string
}

//go:embed setup.html
var setupHTML string

var setupTmpl = buildEmailTemplate(setupHTML)

// RenderSetupEmail generates both HTML and plain-text welcome email content.
func RenderSetupEmail(data SetupEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.ExpireDays <= 0 {
		data.ExpireDays = 7
	}
	if data.ExpiresAt == "" {
		data.ExpiresAt = time.Now().AddDate(0, 0, data.ExpireDays).Format("02 Jan 2006, 15:04 WIB")
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Your %s account is active and ready to use.", data.AppName),
		Subject:      fmt.Sprintf("Welcome to %s", data.AppName),
		HeaderTitle:  "Welcome to " + data.AppName,
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, setupTmpl, data)
	if err != nil {
		return "", "", err
	}

	passLine := ""
	if data.InitialPassword != "" {
		passLine = fmt.Sprintf("- Temporary Password: %s\n", data.InitialPassword)
	}
	emailLine := ""
	if data.Email != "" {
		emailLine = fmt.Sprintf("- Email: %s\n", data.Email)
	}

	textBody = fmt.Sprintf(`Hello %s,

Welcome to %s! An administrator has created a new account for you, and your account is now active.
Please use the following credentials to log in to the portal:

Account Credentials:
- Username: %s
%s%s
Login Link:
%s

IMPORTANT - SECURITY ADVISORY:
For your account security and data privacy, please change this temporary password immediately via the Profile/Account settings upon logging in for the first time.

After setting your password, you can also register a Passkey (biometric login) in your Profile menu for fast and secure passwordless sign-in.

If you did not expect this account, you can safely ignore this email.

--
%s
`, data.Username, data.AppName, data.Username, emailLine, passLine, data.SetupURL, data.AppName)

	return htmlBody, textBody, nil
}
