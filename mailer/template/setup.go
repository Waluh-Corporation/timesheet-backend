package template

import (
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"time"
)

// SetupEmailData holds data needed to render an account activation email.
type SetupEmailData struct {
	AppName      string
	Username     string
	Email        string
	SetupURL     string
	ExpireDays   int
	ExpiresAt    string
	SupportEmail string
	PortalURL    string
}

//go:embed setup.html
var setupHTML string

var setupTmpl = htmltemplate.Must(htmltemplate.New("setup").Parse(setupHTML))

// RenderSetupEmail generates both HTML and plain-text activation email content.
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
		Preheader:    fmt.Sprintf("Aktivasi akun %s Anda dan atur kata sandi baru.", data.AppName),
		Subject:      fmt.Sprintf("Aktivasi Akun %s - Selamat Datang!", data.AppName),
		BadgeText:    "Aktivasi Akun",
		BadgeColor:   "indigo",
		HeaderTitle:  "Selamat Datang di " + data.AppName,
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, setupTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Administrator telah membuat akun baru untuk Anda pada %s.
Silakan selesaikan aktivasi akun dengan mengatur kata sandi Anda melalui tautan berikut:

%s

Detail Akun:
- Username: %s
- Berlaku Hingga: %s (%d hari)

Setelah mengatur kata sandi, Anda juga dapat mendaftarkan Passkey (biometrik) pada menu Profil untuk login yang lebih cepat dan aman.

Jika Anda tidak merasa meminta akun ini, Anda dapat mengabaikan email ini.

--
%s
`, data.Username, data.AppName, data.SetupURL, data.Username, data.ExpiresAt, data.ExpireDays, data.AppName)

	return htmlBody, textBody, nil
}
