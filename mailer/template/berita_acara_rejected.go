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
		Preheader:    fmt.Sprintf("Your Berita Acara for %s requires revision based on notes from %s.", data.Period, data.WitnessName),
		Subject:      fmt.Sprintf("Berita Acara Requires Revision (%s)", data.Period),
		HeaderTitle:  "Berita Acara Requires Revision",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, beritaAcaraRejectedTmpl, data)
	if err != nil {
		return "", "", fmt.Errorf("render berita acara rejected template: %w", err)
	}

	notesText := ""
	if data.Notes != "" {
		notesText = fmt.Sprintf("\nWitness Revision Notes:\n\"%s\"\n", data.Notes)
	}

	activityText := ""
	if data.ActivityURL != "" {
		activityText = fmt.Sprintf("\nActivity Update Link: %s\n", data.ActivityURL)
	}

	textBody = fmt.Sprintf(`Hello %s,

Your Berita Acara submission for the period of %s was reviewed by witness %s and requires revision before it can be approved.%s%s
Activity edits for this period have been unlocked automatically so you can adjust your records and resubmit.

--
%s
`, data.EmployeeName, data.Period, data.WitnessName, notesText, activityText, data.AppName)

	return htmlBody, textBody, nil
}
