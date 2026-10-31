package template

import (
	_ "embed"
	"fmt"
	"time"
)

// ReminderEmailData holds data needed to render a daily timesheet reminder email.
type ReminderEmailData struct {
	AppName      string
	Username     string
	Email        string
	Date         string
	ActivityURL  string
	PortalURL    string
	SupportEmail string
}

//go:embed reminder.html
var reminderHTML string

var reminderTmpl = buildEmailTemplate(reminderHTML)

// RenderReminderEmail generates both HTML and plain-text reminder email content.
func RenderReminderEmail(data ReminderEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.Date == "" {
		data.Date = time.Now().Format("02 January 2006")
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Reminder: It's time to log today's work activities (%s).", data.Date),
		Subject:      fmt.Sprintf("Daily Reminder: Log Your Timesheet Today (%s)", data.Date),
		HeaderTitle:  "Timesheet Logging Reminder",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, reminderTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Hello %s,

This is an automated daily reminder from %s.
You have not logged your work activities for today (%s).

Please log your work activities using the following link:
%s

Regularly logging your work activities every day helps maintain accurate records and ensures a smooth cutoff recap at the end of the month.

--
%s
`, data.Username, data.AppName, data.Date, data.ActivityURL, data.AppName)

	return htmlBody, textBody, nil
}
