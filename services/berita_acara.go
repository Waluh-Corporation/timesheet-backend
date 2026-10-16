package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"timesheet-backend/assets"
)

const (
	tagTanggal    = "{{TANGGAL}}"
	tagHari       = "{{HARI}}"
	tagJamDatang  = "{{JAM_DATANG}}"
	tagJamPulang  = "{{JAM_PULANG}}"
	tagKeterangan = "{{KETERANGAN}}"
)

// BeritaAcaraItem represents one daily incident row in the Berita Acara table.
type BeritaAcaraItem struct {
	Date      time.Time
	Day       string
	StartTime string
	EndTime   string
	Notes     string
}

// BeritaAcaraPayload contains all data required to generate the Berita Acara Word document.
type BeritaAcaraPayload struct {
	EmployeeName string
	NPP          string
	Department   string
	GroupName    string
	WitnessName  string
	TeamLeadName string
	Items        []BeritaAcaraItem
}

// IndonesianDayName returns the Indonesian translation of the given weekday.
func IndonesianDayName(weekday time.Weekday) string {
	switch weekday {
	case time.Sunday:
		return "Minggu"
	case time.Monday:
		return "Senin"
	case time.Tuesday:
		return "Selasa"
	case time.Wednesday:
		return "Rabu"
	case time.Thursday:
		return "Kamis"
	case time.Friday:
		return "Jumat"
	case time.Saturday:
		return "Sabtu"
	default:
		return ""
	}
}

func escapeXML(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func replaceScalarPlaceholders(xmlStr string, payload BeritaAcaraPayload) string {
	xmlStr = strings.ReplaceAll(xmlStr, "{{NAME}}", escapeXML(payload.EmployeeName))
	xmlStr = strings.ReplaceAll(xmlStr, "{{NPP}}", escapeXML(payload.NPP))
	xmlStr = strings.ReplaceAll(xmlStr, "{{DEPARTMENT}}", escapeXML(payload.Department))
	xmlStr = strings.ReplaceAll(xmlStr, "{{GROUP}}", escapeXML(payload.GroupName))
	xmlStr = strings.ReplaceAll(xmlStr, "{{WITNESS_NAME}}", escapeXML(payload.WitnessName))
	xmlStr = strings.ReplaceAll(xmlStr, "{{EMPLOYEE_NAME}}", escapeXML(payload.EmployeeName))
	return strings.ReplaceAll(xmlStr, "{{TEAM_LEAD}}", escapeXML(payload.TeamLeadName))
}

func findTableRowBoundaries(xmlStr string) (int, int) {
	pos := strings.Index(xmlStr, tagTanggal)
	if pos == -1 {
		return -1, -1
	}

	rStart := -1
	for idx := pos; idx >= 0; idx-- {
		if idx+5 <= len(xmlStr) && xmlStr[idx:idx+5] == "<w:tr" {
			if idx+5 == len(xmlStr) || xmlStr[idx+5] == ' ' || xmlStr[idx+5] == '>' {
				rStart = idx
				break
			}
		}
	}

	rEndIndex := strings.Index(xmlStr[pos:], "</w:tr>")
	if rStart == -1 || rEndIndex == -1 {
		return -1, -1
	}
	rEnd := pos + rEndIndex + len("</w:tr>")
	return rStart, rEnd
}

func renderItemRow(rowTemplate string, item BeritaAcaraItem) string {
	dayName := item.Day
	if dayName == "" && !item.Date.IsZero() {
		dayName = IndonesianDayName(item.Date.Weekday())
	}
	dateStr := ""
	if !item.Date.IsZero() {
		dateStr = item.Date.Format("02/01/2006")
	}

	row := strings.ReplaceAll(rowTemplate, tagTanggal, escapeXML(dateStr))
	row = strings.ReplaceAll(row, tagHari, escapeXML(dayName))
	row = strings.ReplaceAll(row, tagJamDatang, escapeXML(item.StartTime))
	row = strings.ReplaceAll(row, tagJamPulang, escapeXML(item.EndTime))
	return strings.ReplaceAll(row, tagKeterangan, escapeXML(item.Notes))
}

func renderTableRows(rowTemplate string, items []BeritaAcaraItem) string {
	if len(items) == 0 {
		emptyRow := strings.ReplaceAll(rowTemplate, tagTanggal, "")
		emptyRow = strings.ReplaceAll(emptyRow, tagHari, "")
		emptyRow = strings.ReplaceAll(emptyRow, tagJamDatang, "")
		emptyRow = strings.ReplaceAll(emptyRow, tagJamPulang, "")
		return strings.ReplaceAll(emptyRow, tagKeterangan, "")
	}

	var rowsBuilder strings.Builder
	for _, item := range items {
		rowsBuilder.WriteString(renderItemRow(rowTemplate, item))
	}
	return rowsBuilder.String()
}

func populateDocumentXML(xmlStr string, payload BeritaAcaraPayload) string {
	xmlStr = replaceScalarPlaceholders(xmlStr, payload)

	rStart, rEnd := findTableRowBoundaries(xmlStr)
	if rStart != -1 && rEnd != -1 {
		rowTemplate := xmlStr[rStart:rEnd]
		renderedRows := renderTableRows(rowTemplate, payload.Items)
		xmlStr = xmlStr[:rStart] + renderedRows + xmlStr[rEnd:]
	}
	return xmlStr
}

func processZipEntry(writer *zip.Writer, file *zip.File, payload BeritaAcaraPayload) error {
	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", file.Name, err)
	}
	defer func() { _ = rc.Close() }()

	w, err := writer.Create(file.Name)
	if err != nil {
		return fmt.Errorf("failed to create zip entry %s: %w", file.Name, err)
	}

	if file.Name != "word/document.xml" {
		if _, err := io.Copy(w, rc); err != nil {
			return fmt.Errorf("failed to copy template file %s: %w", file.Name, err)
		}
		return nil
	}

	xmlBytes, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read word/document.xml: %w", err)
	}

	populatedXML := populateDocumentXML(string(xmlBytes), payload)
	if _, err := io.WriteString(w, populatedXML); err != nil {
		return fmt.Errorf("failed to write populated word/document.xml: %w", err)
	}
	return nil
}

// GenerateBeritaAcaraDocx generates a populated Berita Acara .docx document based on assets.BeritaAcaraTemplate.
func GenerateBeritaAcaraDocx(payload BeritaAcaraPayload) ([]byte, error) {
	if len(assets.BeritaAcaraTemplate) == 0 {
		return nil, fmt.Errorf("berita acara template is not loaded or empty")
	}

	reader, err := zip.NewReader(bytes.NewReader(assets.BeritaAcaraTemplate), int64(len(assets.BeritaAcaraTemplate)))
	if err != nil {
		return nil, fmt.Errorf("failed to open berita acara docx template: %w", err)
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for _, file := range reader.File {
		if err := processZipEntry(writer, file, payload); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize docx zip archive: %w", err)
	}

	return buf.Bytes(), nil
}
