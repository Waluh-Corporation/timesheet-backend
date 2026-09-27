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
	BadgeColor   string // e.g. indigo, emerald, amber, sky, rose
	HeaderTitle  string
	SupportEmail string
	PortalURL    string
	CurrentYear  int
	SystemNotice string
}

// AppInitial returns the first rune of AppName for monogram badge rendering.
func (b BaseLayoutData) AppInitial() string {
	runes := []rune(strings.TrimSpace(b.AppName))
	if len(runes) > 0 {
		return strings.ToUpper(string(runes[0]))
	}
	return "T"
}

// BadgeBgColor returns the background hex color for the category badge.
func (b BaseLayoutData) BadgeBgColor() string {
	switch b.BadgeColor {
	case "emerald":
		return "#ecfdf5"
	case "amber":
		return "#fffbeb"
	case "sky":
		return "#f0f9ff"
	case "rose":
		return "#fff1f2"
	default:
		return "#eef2ff"
	}
}

// BadgeTextColor returns the text hex color for the category badge.
func (b BaseLayoutData) BadgeTextColor() string {
	switch b.BadgeColor {
	case "emerald":
		return "#047857"
	case "amber":
		return "#b45309"
	case "sky":
		return "#0284c7"
	case "rose":
		return "#e11d48"
	default:
		return "#4f46e5"
	}
}

// BadgeBorderColor returns the border hex color for the category badge.
func (b BaseLayoutData) BadgeBorderColor() string {
	switch b.BadgeColor {
	case "emerald":
		return "#a7f3d0"
	case "amber":
		return "#fde68a"
	case "sky":
		return "#bae6fd"
	case "rose":
		return "#fecdd3"
	default:
		return "#c7d2fe"
	}
}

//go:embed layout.html
var baseLayoutHTML string

var baseTmpl = htmltemplate.Must(htmltemplate.New("base").Parse(baseLayoutHTML))

func buildEmailTemplate(contentHTML string) *htmltemplate.Template {
	tmpl := htmltemplate.Must(baseTmpl.Clone())
	htmltemplate.Must(tmpl.New("content").Parse(contentHTML))
	return tmpl
}

func renderWithLayout(layout BaseLayoutData, tmpl *htmltemplate.Template, data any) (string, error) {
	if layout.CurrentYear <= 0 {
		layout.CurrentYear = time.Now().Year()
	}
	if layout.AppName == "" {
		layout.AppName = "Timesheet Portal"
	}

	ctx := struct {
		BaseLayoutData
		Data any
	}{
		BaseLayoutData: layout,
		Data:           data,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to render email layout: %w", err)
	}

	return buf.String(), nil
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
