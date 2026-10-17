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
	EmployeeName         string
	NPP                  string
	Department           string
	GroupName            string
	WitnessName          string
	TeamLeadName         string
	Items                []BeritaAcaraItem
	EmployeeSignaturePNG []byte
	WitnessSignaturePNG  []byte
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

func buildDocxInlineDrawingRun(relID string, docPrID int, name string) string {
	// 1260000 EMUs (~1.38 in) wide x 450000 EMUs (~0.49 in) high fits cleanly above the signer name
	const cx = 1260000
	const cy = 450000
	return fmt.Sprintf(
		`<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:docPr id="%d" name="%s"/><wp:cNvGraphicFramePr><a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`,
		cx, cy, docPrID, name, docPrID, name, relID, cx, cy,
	)
}

func injectDocxSignatureParagraph(xmlStr string, payload BeritaAcaraPayload) string {
	hasEmp := len(payload.EmployeeSignaturePNG) > 0
	hasWit := len(payload.WitnessSignaturePNG) > 0
	if !hasEmp && !hasWit {
		return xmlStr
	}

	witPos := strings.Index(xmlStr, "{{WITNESS_NAME}}")
	if witPos == -1 {
		return xmlStr
	}

	// Find the start of the paragraph (<w:p ...) containing {{WITNESS_NAME}} and {{EMPLOYEE_NAME}}
	pStart := strings.LastIndex(xmlStr[:witPos], "<w:p ")
	if pStart == -1 {
		pStart = strings.LastIndex(xmlStr[:witPos], "<w:p>")
	}
	if pStart == -1 {
		return xmlStr
	}

	var sigRuns strings.Builder
	sigRuns.WriteString(`<w:p><w:pPr><w:spacing w:line="240" w:lineRule="auto"/><w:ind w:firstLine="720"/><w:contextualSpacing/></w:pPr>`)
	if hasWit {
		sigRuns.WriteString(buildDocxInlineDrawingRun("rIdSigWitness", 901, "WitnessSignature"))
	} else {
		sigRuns.WriteString(`<w:r><w:t xml:space="preserve">                    </w:t></w:r>`)
	}
	// Preserve tab spacing between left column (Witness) and right column (Employee)
	sigRuns.WriteString(`<w:r><w:tab/></w:r><w:r><w:tab/></w:r><w:r><w:tab/></w:r><w:r><w:tab/></w:r><w:r><w:t xml:space="preserve">                                  </w:t></w:r><w:r><w:tab/></w:r>`)
	if hasEmp {
		sigRuns.WriteString(buildDocxInlineDrawingRun("rIdSigEmployee", 902, "EmployeeSignature"))
	}
	sigRuns.WriteString(`</w:p>`)

	// Remove up to two preceding empty spacer paragraphs so the vertical layout stays balanced
	prefix := xmlStr[:pStart]
	const emptyPara = `<w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:b/><w:sz w:val="18"/></w:rPr></w:pPr></w:p>`
	for i := 0; i < 2; i++ {
		if strings.HasSuffix(prefix, emptyPara) {
			if lastP := strings.LastIndex(prefix, "<w:p "); lastP != -1 {
				prefix = prefix[:lastP]
			}
		}
	}

	return prefix + sigRuns.String() + xmlStr[pStart:]
}

func replaceScalarPlaceholders(xmlStr string, payload BeritaAcaraPayload) string {
	xmlStr = injectDocxSignatureParagraph(xmlStr, payload)
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

func enrichContentTypesXML(xmlStr string, payload BeritaAcaraPayload) string {
	if len(payload.EmployeeSignaturePNG) == 0 && len(payload.WitnessSignaturePNG) == 0 {
		return xmlStr
	}
	if strings.Contains(xmlStr, `Extension="png"`) {
		return xmlStr
	}
	return strings.Replace(xmlStr, "</Types>", `<Default Extension="png" ContentType="image/png"/></Types>`, 1)
}

func enrichDocumentRelsXML(xmlStr string, payload BeritaAcaraPayload) string {
	var extra strings.Builder
	if len(payload.EmployeeSignaturePNG) > 0 {
		extra.WriteString(`<Relationship Id="rIdSigEmployee" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/sig_employee.png"/>`)
	}
	if len(payload.WitnessSignaturePNG) > 0 {
		extra.WriteString(`<Relationship Id="rIdSigWitness" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/sig_witness.png"/>`)
	}
	if extra.Len() == 0 {
		return xmlStr
	}
	return strings.Replace(xmlStr, "</Relationships>", extra.String()+"</Relationships>", 1)
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

	const maxZipEntrySize = 10 * 1024 * 1024 // 10MB limit per entry to prevent decompression bombs

	switch file.Name {
	case "word/document.xml":
		xmlBytes, err := io.ReadAll(io.LimitReader(rc, maxZipEntrySize))
		if err != nil {
			return fmt.Errorf("failed to read word/document.xml: %w", err)
		}
		populatedXML := populateDocumentXML(string(xmlBytes), payload)
		if _, err := io.WriteString(w, populatedXML); err != nil {
			return fmt.Errorf("failed to write populated word/document.xml: %w", err)
		}
		return nil

	case "[Content_Types].xml":
		xmlBytes, err := io.ReadAll(io.LimitReader(rc, maxZipEntrySize))
		if err != nil {
			return fmt.Errorf("failed to read [Content_Types].xml: %w", err)
		}
		if _, err := io.WriteString(w, enrichContentTypesXML(string(xmlBytes), payload)); err != nil {
			return fmt.Errorf("failed to write [Content_Types].xml: %w", err)
		}
		return nil

	case "word/_rels/document.xml.rels":
		xmlBytes, err := io.ReadAll(io.LimitReader(rc, maxZipEntrySize))
		if err != nil {
			return fmt.Errorf("failed to read word/_rels/document.xml.rels: %w", err)
		}
		if _, err := io.WriteString(w, enrichDocumentRelsXML(string(xmlBytes), payload)); err != nil {
			return fmt.Errorf("failed to write word/_rels/document.xml.rels: %w", err)
		}
		return nil

	default:
		if _, err := io.Copy(w, io.LimitReader(rc, maxZipEntrySize)); err != nil {
			return fmt.Errorf("failed to copy template file %s: %w", file.Name, err)
		}
		return nil
	}
}

func writeDocxSignatureMedia(writer *zip.Writer, payload BeritaAcaraPayload) error {
	if len(payload.EmployeeSignaturePNG) > 0 {
		w, err := writer.Create("word/media/sig_employee.png")
		if err != nil {
			return fmt.Errorf("failed to create word/media/sig_employee.png: %w", err)
		}
		if _, err := w.Write(payload.EmployeeSignaturePNG); err != nil {
			return fmt.Errorf("failed to write word/media/sig_employee.png: %w", err)
		}
	}
	if len(payload.WitnessSignaturePNG) > 0 {
		w, err := writer.Create("word/media/sig_witness.png")
		if err != nil {
			return fmt.Errorf("failed to create word/media/sig_witness.png: %w", err)
		}
		if _, err := w.Write(payload.WitnessSignaturePNG); err != nil {
			return fmt.Errorf("failed to write word/media/sig_witness.png: %w", err)
		}
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

	if err := writeDocxSignatureMedia(writer, payload); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize docx zip archive: %w", err)
	}

	return buf.Bytes(), nil
}
