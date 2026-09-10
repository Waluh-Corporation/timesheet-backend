// Package assets embeds static files bundled into the binary, such as the
// built-in default timesheet template.
package assets

import _ "embed"

// MIITemplate is the raw .xlsx bytes of the built-in "MII Timesheet" template,
// seeded as the default template on first boot.
//
//go:embed mii_timesheet_template.xlsx
var MIITemplate []byte

//go:embed images/mii_logo.png
var MIILogo []byte

//go:embed images/sdd_logo.jpg
var SDDLogo []byte

//go:embed images/adidata_logo.png
var AdidataLogo []byte

//go:embed images/ntt_logo.png
var NTTLogo []byte
