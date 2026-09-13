package template

import (
	"bytes"
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	"time"
)

// BaseLayoutData represents the common wrapper context for email rendering.
type BaseLayoutData struct {
	AppName      string
	Preheader    string
	Subject      string
	BadgeText    string
	BadgeColor   string // e.g. indigo, emerald, amber, sky
	HeaderTitle  string
	ContentHTML  htmltemplate.HTML
	SupportEmail string
	PortalURL    string
	CurrentYear  int
	SystemNotice string
}

//go:embed layout.html
var baseLayoutHTML string

var baseTmpl = htmltemplate.Must(htmltemplate.New("base").Parse(baseLayoutHTML))

func renderWithLayout(layout BaseLayoutData, contentTmpl *htmltemplate.Template, data any) (string, error) {
	var contentBuf bytes.Buffer
	if err := contentTmpl.Execute(&contentBuf, data); err != nil {
		return "", fmt.Errorf("failed to render content template: %w", err)
	}

	//nolint:gosec // G203: contentBuf is pre-rendered and auto-escaped by html/template
	layout.ContentHTML = htmltemplate.HTML(contentBuf.String())
	if layout.CurrentYear <= 0 {
		layout.CurrentYear = time.Now().Year()
	}
	if layout.AppName == "" {
		layout.AppName = "Timesheet Portal"
	}

	var finalBuf bytes.Buffer
	if err := baseTmpl.Execute(&finalBuf, layout); err != nil {
		return "", fmt.Errorf("failed to render base email layout: %w", err)
	}

	return finalBuf.String(), nil
}

// FormatMonthYearIndonesian converts numeric month and year to Indonesian string (e.g. 9, 2026 -> "September 2026").
func FormatMonthYearIndonesian(month int, year int) string {
	months := [...]string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	if month >= 1 && month <= 12 {
		return fmt.Sprintf("%s %04d", months[month-1], year)
	}
	if year > 0 {
		return fmt.Sprintf("Bulan %02d %04d", month, year)
	}
	return ""
}

// ParsePeriodFromFilename attempts to extract month and year from a standard timesheet filename
// like "Timesheet_john_doe_09_2026.xlsx".
func ParsePeriodFromFilename(filename string) (string, string) {
	base := strings.TrimSuffix(filename, ".xlsx")
	parts := strings.Split(base, "_")
	if len(parts) >= 3 {
		yearStr := parts[len(parts)-1]
		monthStr := parts[len(parts)-2]
		var m, y int
		if _, err := fmt.Sscanf(monthStr, "%d", &m); err == nil {
			if _, err := fmt.Sscanf(yearStr, "%d", &y); err == nil && y >= 2000 && y <= 2100 {
				return FormatMonthYearIndonesian(m, y), strings.Join(parts[1:len(parts)-2], "_")
			}
		}
	}
	return "", ""
}
