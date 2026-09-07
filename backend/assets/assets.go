// Package assets embeds static files bundled into the binary, such as the
// built-in default timesheet template.
package assets

import _ "embed"

// BNITemplate is the raw .xlsx bytes of the built-in "BNI DEV Timesheet"
// template (the second-company layout), seeded as a non-default template.
//
//go:embed bni_dev_timesheet_template.xlsx
var BNITemplate []byte

// MIITemplate is the raw .xlsx bytes of the built-in "MII Timesheet" template,
// seeded as the default template on first boot.
//
//go:embed mii_timesheet_template.xlsx
var MIITemplate []byte

// SDDTemplate is the raw .xlsx bytes of the built-in "SDD Timesheet" template.
//
//go:embed sdd_timesheet_template.xlsx
var SDDTemplate []byte

// AdidataTemplate is the raw .xlsx bytes of the built-in "Adidata Timesheet" template.
//
//go:embed adidata_timesheet_template.xlsx
var AdidataTemplate []byte

