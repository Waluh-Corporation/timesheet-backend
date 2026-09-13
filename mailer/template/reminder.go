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
		Preheader:    fmt.Sprintf("Pengingat: Waktunya mencatat aktivitas kerja hari ini (%s).", data.Date),
		Subject:      fmt.Sprintf("Pengingat Harian: Isi Timesheet Hari Ini (%s)", data.Date),
		BadgeText:    "Pengingat Harian",
		BadgeColor:   "sky",
		HeaderTitle:  "Pengingat Pengisian Timesheet",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, reminderTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Ini adalah pengingat harian otomatis dari %s.
Anda belum mencatat aktivitas kerja untuk hari ini (%s).

Silakan isi aktivitas kerja Anda melalui tautan berikut:
%s

Mencatat aktivitas kerja secara rutin setiap hari membantu menjaga keakuratan jam kerja dan kelancaran rekapitulasi di akhir bulan.

--
%s
`, data.Username, data.AppName, data.Date, data.ActivityURL, data.AppName)

	return htmlBody, textBody, nil
}
