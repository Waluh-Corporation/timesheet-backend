package template

import (
	_ "embed"
	"fmt"
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
	DownloadURL  string
	ExpiresAt    string
}

//go:embed timesheet.html
var timesheetHTML string

var timesheetTmpl = buildEmailTemplate(timesheetHTML)

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
		return "", "", fmt.Errorf("render timesheet template: %w", err)
	}

	downloadInfo := ""
	if data.DownloadURL != "" {
		downloadInfo = fmt.Sprintf("\nTautan Unduhan: %s\n(*Tautan unduhan ini privat, dapat digunakan maksimal 3 kali unduhan, dan aktif selama 7 hari.)\n", data.DownloadURL)
	}

	textBody = fmt.Sprintf(`Halo %s,

Dokumen timesheet bulanan Anda telah berhasil dibuat dalam format Microsoft Excel (.xlsx).
%s
Detail Timesheet:
- Perusahaan: %s
- Periode: %s
- Nama File: %s

Langkah Selanjutnya:
Silakan periksa kembali rincian jam kerja dan kegiatan sebelum menyerahkan dokumen ini kepada Team Leader / Department Head untuk persetujuan (approval).

--
%s
%s
`, data.Username, downloadInfo, data.Company, data.Period, data.Filename, data.AppName, data.PortalURL)

	return htmlBody, textBody, nil
}
