package template

import (
	_ "embed" // Register embed package for go:embed directive
	"fmt"
)

// BeritaAcaraRejectedEmailData holds data needed to render a Berita Acara rejection / revision notice email.
type BeritaAcaraRejectedEmailData struct {
	AppName      string
	EmployeeName string
	WitnessName  string
	Period       string
	Notes        string
	ActivityURL  string
	PortalURL    string
	SupportEmail string
}

//go:embed berita_acara_rejected.html
var beritaAcaraRejectedHTML string

var beritaAcaraRejectedTmpl = buildEmailTemplate(beritaAcaraRejectedHTML)

// RenderBeritaAcaraRejectedEmail generates both HTML and plain-text rejection / revision email content.
func RenderBeritaAcaraRejectedEmail(data BeritaAcaraRejectedEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Berita Acara periode %s memerlukan revisi sesuai catatan dari %s.", data.Period, data.WitnessName),
		Subject:      fmt.Sprintf("Berita Acara Perlu Revisi (%s)", data.Period),
		BadgeText:    "Perlu Revisi",
		BadgeColor:   "rose",
		HeaderTitle:  "Berita Acara Memerlukan Revisi",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, beritaAcaraRejectedTmpl, data)
	if err != nil {
		return "", "", fmt.Errorf("render berita acara rejected template: %w", err)
	}

	notesText := ""
	if data.Notes != "" {
		notesText = fmt.Sprintf("\nCatatan Revisi dari Saksi:\n\"%s\"\n", data.Notes)
	}

	activityText := ""
	if data.ActivityURL != "" {
		activityText = fmt.Sprintf("\nTautan Perbaikan Aktivitas: %s\n", data.ActivityURL)
	}

	textBody = fmt.Sprintf(`Halo %s,

Pengajuan Berita Acara Anda untuk periode %s telah ditinjau oleh saksi %s dan memerlukan revisi sebelum dapat disetujui.%s%s
Kunci mutasi periode bulan ini telah dibuka kembali secara otomatis agar Anda dapat menyesuaikan catatan aktivitas dan mengajukannya kembali.

--
%s
`, data.EmployeeName, data.Period, data.WitnessName, notesText, activityText, data.AppName)

	return htmlBody, textBody, nil
}
