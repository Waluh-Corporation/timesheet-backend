package template

import (
	_ "embed" // Register embed package for go:embed directive
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
		Preheader:    fmt.Sprintf("Berita Acara witness signature request from %s for %s.", data.EmployeeName, data.Period),
		Subject:      fmt.Sprintf("Berita Acara Witness Signature Request - %s (%s)", data.EmployeeName, data.Period),
		HeaderTitle:  "Berita Acara Witness Review Request",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, witnessRequestTmpl, data)
	if err != nil {
		return "", "", fmt.Errorf("render witness request template: %w", err)
	}

	textBody = fmt.Sprintf(`Hello %s,

Your colleague %s has submitted a Berita Acara for the period of %s and nominated you as a witness signer.

Please review and sign the document using the following link:
%s

You can approve this request to affix your digital signature as a witness or reject it with revision notes if any discrepancies are found.

--
%s
`, data.WitnessName, data.EmployeeName, data.Period, data.ReviewURL, data.AppName)

	return htmlBody, textBody, nil
}
