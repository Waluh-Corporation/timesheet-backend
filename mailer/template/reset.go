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
		greetingName = "Pengguna"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Instruksi pengaturan ulang kata sandi akun %s Anda.", data.AppName),
		Subject:      fmt.Sprintf("Reset Kata Sandi Akun %s", data.AppName),
		BadgeText:    "Keamanan Akun",
		BadgeColor:   "amber",
		HeaderTitle:  "Permintaan Reset Kata Sandi",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, resetTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Kami menerima permintaan untuk mengatur ulang kata sandi akun %s Anda.
Gunakan tautan di bawah ini untuk membuat kata sandi baru:

%s

Detail Permintaan:
- Waktu Permintaan: %s
- Berlaku Hingga: %s

PENTING: Tautan ini bersifat rahasia dan hanya berlaku hingga %s. Jangan bagikan tautan ini kepada siapapun.
Jika Anda tidak meminta pengaturan ulang kata sandi ini, abaikan email ini; akun Anda tetap aman.

--
%s
`, greetingName, data.AppName, data.ResetURL, data.RequestedAt, data.ExpiresAt, data.ExpiresAt, data.AppName)

	return htmlBody, textBody, nil
}
