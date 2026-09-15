package services

import (
	"strings"

	"timesheet-backend/models"
)

// GenerationInput bundles everything needed to render a user's monthly file.
type GenerationInput struct {
	CompanyCode string // "mii", "sdd", "adidata", "ntt"
	User        *models.User
	Month       int
	Year        int
	Activities  []models.DailyActivity
	Overtimes   []models.OvertimeEntry
	Approvers   []models.Approver
	// Holidays maps day-of-month to a public-holiday name for the month.
	Holidays map[int]string
}

// GenerateFromTemplate routes generation to the dedicated programmatic builder
// based on the user's company code ("mii", "sdd", "adidata", "ntt").
func GenerateFromTemplate(in GenerationInput) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(in.CompanyCode)) {
	case "mii":
		return buildMIIWorkbook(in)
	case "sdd":
		return buildSDDWorkbook(in)
	case "adidata":
		return buildAdidataWorkbook(in)
	case "ntt":
		return buildNTTWorkbook(in)
	default:
		// Default to MII layout if company not matched
		return buildMIIWorkbook(in)
	}
}

// MIIAppImpactedOptions lists the valid client options for MII's "Aplikasi
// Terdampak" (application impacted) column.
var MIIAppImpactedOptions = []string{"Bisnis", "Cash", "Overseas"}

// NormalizeMIIAppImpacted maps a user-entered value to its canonical MII option
// (case-insensitively). It returns the canonical string when it matches one of
// the allowed options, otherwise the trimmed input is returned unchanged so no
// data is silently dropped.
func NormalizeMIIAppImpacted(v string) string {
	t := strings.TrimSpace(v)
	for _, opt := range MIIAppImpactedOptions {
		if strings.EqualFold(t, opt) {
			return opt
		}
	}
	return t
}
