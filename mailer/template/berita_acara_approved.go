package template

import (
	_ "embed"
	"fmt"
)

// BeritaAcaraApprovedEmailData holds data needed to render a Berita Acara approval notification email.
type BeritaAcaraApprovedEmailData struct {
	AppName      string
	EmployeeName string
	WitnessName  string
	Period       string
	DownloadURL  string
	PortalURL    string
	SupportEmail string
}

//go:embed berita_acara_approved.html
var beritaAcaraApprovedHTML string

var beritaAcaraApprovedTmpl = buildEmailTemplate(beritaAcaraApprovedHTML)

// RenderBeritaAcaraApprovedEmail generates both HTML and plain-text approval email content.
func RenderBeritaAcaraApprovedEmail(data BeritaAcaraApprovedEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Berita Acara periode %s telah disetujui dan ditandatangani oleh %s.", data.Period, data.WitnessName),
		Subject:      fmt.Sprintf("Berita Acara Disetujui oleh Saksi (%s)", data.Period),
		BadgeText:    "Berita Acara Disetujui",
		BadgeColor:   "emerald",
		HeaderTitle:  "Berita Acara Anda Telah Disetujui",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, beritaAcaraApprovedTmpl, data)
	if err != nil {
		return "", "", fmt.Errorf("render berita acara approved template: %w", err)
	}

	downloadText := ""
	if data.DownloadURL != "" {
		downloadText = fmt.Sprintf("\nTautan Unduhan: %s\n", data.DownloadURL)
	}

	textBody = fmt.Sprintf(`Halo %s,

Berita Acara Anda untuk periode %s telah disetujui dan ditandatangani oleh saksi %s.

Dokumen resmi dengan tanda tangan lengkap dan QR Code verifikasi kini siap diunduh.%s
Dokumen resmi telah dilengkapi dengan gambar tanda tangan visual dari kedua pihak serta QR Code verifikasi dokumen digital.

--
%s
`, data.EmployeeName, data.Period, data.WitnessName, downloadText, data.AppName)

	return htmlBody, textBody, nil
}
