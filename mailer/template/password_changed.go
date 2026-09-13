package template

import (
	_ "embed"
	"fmt"
	htmltemplate "html/template"
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
}

//go:embed password_changed.html
var passwordChangedHTML string

var passwordChangedTmpl = htmltemplate.Must(htmltemplate.New("pwd_changed").Parse(passwordChangedHTML))

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
		greetingName = "Pengguna"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Kata sandi akun %s Anda telah berhasil diperbarui.", data.AppName),
		Subject:      fmt.Sprintf("Keamanan Akun: Kata Sandi %s Telah Diperbarui", data.AppName),
		BadgeText:    "Pembaruan Keamanan",
		BadgeColor:   "emerald",
		HeaderTitle:  "Kata Sandi Berhasil Diperbarui",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, passwordChangedTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Kata sandi akun %s Anda telah berhasil diperbarui pada %s.

Jika Anda yang melakukan perubahan ini, Anda dapat masuk kembali dengan kata sandi baru Anda:
%s

PERINGATAN KEAMANAN:
Jika Anda TIDAK pernah meminta atau melakukan perubahan ini, segera hubungi administrator sistem Anda untuk mengamankan akun Anda.

--
%s
`, greetingName, data.AppName, data.ChangedAt, data.LoginURL, data.AppName)

	return htmlBody, textBody, nil
}
