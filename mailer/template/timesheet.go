package template

import (
	_ "embed"
	"fmt"
	htmltemplate "html/template"
)

// TimesheetEmailData holds data needed to render a timesheet delivery email.
type TimesheetEmailData struct {
	AppName      string
	Username     string
	Email        string
	Company      string
	Period       string
	Filename     string
	PortalURL    string
	SupportEmail string
}

//go:embed timesheet.html
var timesheetHTML string

var timesheetTmpl = htmltemplate.Must(htmltemplate.New("timesheet").Parse(timesheetHTML))

// RenderTimesheetEmail generates both HTML and plain-text timesheet delivery email content.
func RenderTimesheetEmail(data TimesheetEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}

	headerCompany := data.Company
	if headerCompany == "" {
		headerCompany = "Timesheet"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Dokumen Timesheet %s telah siap diunduh.", headerCompany),
		Subject:      fmt.Sprintf("Dokumen Timesheet %s Anda Telah Siap", headerCompany),
		BadgeText:    "Laporan Timesheet",
		BadgeColor:   "emerald",
		HeaderTitle:  fmt.Sprintf("Dokumen Timesheet %s Telah Siap", headerCompany),
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, timesheetTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Dokumen timesheet bulanan Anda telah berhasil dibuat dan dilampirkan pada email ini (format .xlsx).
Salinan dokumen ini juga telah otomatis diunduh pada peramban web Anda.

Detail Timesheet:
- Perusahaan: %s
- Periode: %s
- Nama File: %s

Langkah Selanjutnya:
Silakan periksa kembali rincian jam kerja dan kegiatan sebelum menyerahkan dokumen ini kepada Team Leader / Department Head untuk persetujuan (approval).

--
%s
%s
`, data.Username, data.Company, data.Period, data.Filename, data.AppName, data.PortalURL)

	return htmlBody, textBody, nil
}
