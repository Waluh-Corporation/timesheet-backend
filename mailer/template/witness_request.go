package template

import (
	_ "embed"
	"fmt"
)

// WitnessRequestEmailData holds data needed to render a witness review request email.
type WitnessRequestEmailData struct {
	AppName      string
	WitnessName  string
	EmployeeName string
	Period       string
	ReviewURL    string
	PortalURL    string
	SupportEmail string
}

//go:embed witness_request.html
var witnessRequestHTML string

var witnessRequestTmpl = buildEmailTemplate(witnessRequestHTML)

// RenderWitnessRequestEmail generates both HTML and plain-text witness request email content.
func RenderWitnessRequestEmail(data WitnessRequestEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Permintaan Tanda Tangan Saksi Berita Acara dari %s untuk periode %s.", data.EmployeeName, data.Period),
		Subject:      fmt.Sprintf("Permintaan Tanda Tangan Saksi Berita Acara - %s (%s)", data.EmployeeName, data.Period),
		BadgeText:    "Permintaan Saksi",
		BadgeColor:   "amber",
		HeaderTitle:  "Permintaan Tanda Tangan Saksi Berita Acara",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, witnessRequestTmpl, data)
	if err != nil {
		return "", "", fmt.Errorf("render witness request template: %w", err)
	}

	textBody = fmt.Sprintf(`Halo %s,

Rekan Anda %s telah mengajukan Berita Acara untuk periode %s dan memilih Anda sebagai saksi pendamping untuk menandatangani dokumen tersebut.

Silakan tinjau dan tanda tangani dokumen pada tautan berikut:
%s

Anda dapat menyetujui pengajuan ini untuk membubuhkan tanda tangan digital Anda sebagai saksi atau menolaknya dengan memberikan catatan revisi jika terdapat ketidaksesuaian.

--
%s
`, data.WitnessName, data.EmployeeName, data.Period, data.ReviewURL, data.AppName)

	return htmlBody, textBody, nil
}
