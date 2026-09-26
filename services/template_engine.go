package services

import (
	"strings"
)

// GenerateFromTemplate routes generation through the template-driven injection engine
// based on the user's company code ("mii", "sdd", "adidata", "ntt").
func GenerateFromTemplate(in GenerationInput) ([]byte, error) {
	var raw []byte
	var err error

	switch strings.ToLower(strings.TrimSpace(in.CompanyCode)) {
	case "mii":
		raw, err = renderMIITemplate(in)
	case "sdd":
		raw, err = renderSDDTemplate(in)
	case "adidata":
		raw, err = renderAdidataTemplate(in)
	case "ntt":
		raw, err = renderNTTTemplate(in)
	default:
		raw, err = renderMIITemplate(in)
	}

	if err != nil {
		return nil, err
	}
	return CleanWorkbookBuffer(raw), nil
}
