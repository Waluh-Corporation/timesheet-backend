package template

import (
	_ "embed" // Register embed package for go:embed directive
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
		Preheader:    fmt.Sprintf("Your Berita Acara for %s has been signed by witness %s.", data.Period, data.WitnessName),
		Subject:      fmt.Sprintf("Berita Acara Approved by Witness (%s)", data.Period),
		HeaderTitle:  "Your Berita Acara Has Been Approved",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, beritaAcaraApprovedTmpl, data)
	if err != nil {
		return "", "", fmt.Errorf("render berita acara approved template: %w", err)
	}

	downloadText := ""
	if data.DownloadURL != "" {
		downloadText = fmt.Sprintf("\nDownload Link: %s\n", data.DownloadURL)
	}

	textBody = fmt.Sprintf(`Hello %s,

Your Berita Acara for the period of %s has been approved and signed by witness %s.

The official document with complete signatures and verification QR Code is now ready to download.%s
The official document includes visual signatures from both parties as well as a digital verification QR Code.

--
%s
`, data.EmployeeName, data.Period, data.WitnessName, downloadText, data.AppName)

	return htmlBody, textBody, nil
}
