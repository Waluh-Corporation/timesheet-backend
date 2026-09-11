// Package assets embeds static files bundled into the binary, such as the
// built-in default timesheet template.
package assets

import _ "embed"

//go:embed images/mii_logo.png
var MIILogo []byte

//go:embed images/sdd_logo.jpg
var SDDLogo []byte

//go:embed images/adidata_logo.png
var AdidataLogo []byte

//go:embed images/ntt_logo.png
var NTTLogo []byte
