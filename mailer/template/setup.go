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
		Preheader:    fmt.Sprintf("Akun %s Anda telah aktif dan siap digunakan.", data.AppName),
		Subject:      fmt.Sprintf("Selamat Datang di %s", data.AppName),
		BadgeText:    "Akun Baru",
		BadgeColor:   "indigo",
		HeaderTitle:  "Selamat Datang di " + data.AppName,
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, setupTmpl, data)
	if err != nil {
		return "", "", err
	}

	passLine := ""
	if data.InitialPassword != "" {
		passLine = fmt.Sprintf("- Password Awal: %s\n", data.InitialPassword)
	}
	emailLine := ""
	if data.Email != "" {
		emailLine = fmt.Sprintf("- Email: %s\n", data.Email)
	}

	textBody = fmt.Sprintf(`Halo %s,

Selamat datang di %s! Administrator telah membuat akun baru untuk Anda dan akun Anda saat ini telah berstatus aktif.
Silakan gunakan kredensial berikut untuk masuk ke portal:

Detail Kredensial Akun:
- Username: %s
%s%s- Status Akun: Aktif

Tautan Login:
%s

PENTING - IMBAUAN KEAMANAN:
Demi menjaga keamanan akun Anda, silakan segera ganti kata sandi awal ini melalui menu Profil/Akun setelah Anda berhasil masuk pertama kali.

Setelah mengatur kata sandi, Anda juga dapat mendaftarkan Passkey (biometrik) pada menu Profil untuk proses masuk yang lebih cepat dan aman.

Jika Anda tidak merasa meminta akun ini, Anda dapat mengabaikan email ini.

--
%s
`, data.Username, data.AppName, data.Username, emailLine, passLine, data.SetupURL, data.AppName)

	return htmlBody, textBody, nil
}
