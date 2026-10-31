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
	EmployeeName           string
	NPP                    string
	Department             string
	GroupName              string
	WitnessName            string
	TeamLeadName           string
	Items                  []BeritaAcaraItem
	EmployeeSignatureBytes []byte
	WitnessSignatureBytes  []byte
	QRCodeBytes            []byte
	IsSigned               bool
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

const footer1XML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <w:p>
    <w:pPr>
      <w:jc w:val="right"/>
    </w:pPr>
    <w:r>
      <w:drawing>
        <wp:inline distT="0" distB="0" distL="0" distR="0">
          <wp:extent cx="600000" cy="600000"/>
          <wp:docPr id="201" name="QRCode"/>
          <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
            <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
              <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:nvPicPr>
                  <pic:cNvPr id="201" name="QRCode"/>
                  <pic:cNvPicPr/>
                </pic:nvPicPr>
                <pic:blipFill>
                  <a:blip r:embed="rIdDocQR"/>
                  <a:stretch><a:fillRect/></a:stretch>
                </pic:blipFill>
                <pic:spPr>
                  <a:xfrm><a:off x="0" y="0"/><a:ext cx="600000" cy="600000"/></a:xfrm>
                  <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
                </pic:spPr>
              </pic:pic>
            </a:graphicData>
          </a:graphic>
        </wp:inline>
      </w:drawing>
    </w:r>
  </w:p>
</w:ftr>`

const footer1RelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdDocQR" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/doc_qr.png"/>
</Relationships>`

func buildFloatingAnchorXML(relID, name string, docPrID int, offsetX, offsetY, cx, cy int64) string {
	return fmt.Sprintf(`<w:r><w:drawing><wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" relativeHeight="251658240" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1"><wp:simplePos x="0" y="0"/><wp:positionH relativeFrom="column"><wp:posOffset>%d</wp:posOffset></wp:positionH><wp:positionV relativeFrom="paragraph"><wp:posOffset>%d</wp:posOffset></wp:positionV><wp:extent cx="%d" cy="%d"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:wrapNone/><wp:docPr id="%d" name="%s"/><wp:cNvGraphicFramePr><a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:anchor></w:drawing></w:r>`,
		offsetX, offsetY, cx, cy, docPrID, name, docPrID, name, relID, cx, cy)
}

func findParagraphStart(s string, beforePos int) int {
	for i := beforePos; i >= 0; i-- {
		if i+4 <= len(s) && s[i:i+4] == "<w:p" {
			if i+4 == len(s) || s[i+4] == ' ' || s[i+4] == '>' {
				return i
			}
		}
	}
	return -1
}

func populateTableRowsInXML(xmlStr string, items []BeritaAcaraItem) string {
	rStart, rEnd := findTableRowBoundaries(xmlStr)
	if rStart == -1 || rEnd == -1 {
		return xmlStr
	}
	rowTemplate := xmlStr[rStart:rEnd]
	renderedRows := renderTableRows(rowTemplate, items)
	return xmlStr[:rStart] + renderedRows + xmlStr[rEnd:]
}

func injectFloatingSignatures(xmlStr string, payload BeritaAcaraPayload) string {
	witnessPos := strings.Index(xmlStr, "{{WITNESS_NAME}}")
	if witnessPos == -1 {
		return xmlStr
	}
	witPStart := findParagraphStart(xmlStr, witnessPos)
	if witPStart == -1 {
		return xmlStr
	}
	// Inject floating signatures directly into the witness/employee name paragraph
	// Layout: "In front of text" (wp:anchor with wrapNone and behindDoc="0")
	firstRPos := strings.Index(xmlStr[witPStart:], "<w:r")
	if firstRPos == -1 {
		return xmlStr
	}
	insertIdx := witPStart + firstRPos
	var anchorRuns strings.Builder
	if len(payload.WitnessSignatureBytes) > 0 {
		anchorRuns.WriteString(buildFloatingAnchorXML("rIdWitSig", "WitnessSignature", 101, 200000, -650000, 1200000, 600000))
	}
	if len(payload.EmployeeSignatureBytes) > 0 {
		anchorRuns.WriteString(buildFloatingAnchorXML("rIdEmpSig", "EmployeeSignature", 102, 4500000, -650000, 1200000, 600000))
	}
	return xmlStr[:insertIdx] + anchorRuns.String() + xmlStr[insertIdx:]
}

func injectFooterReference(xmlStr string) string {
	const footerRef = `<w:footerReference w:type="default" r:id="rIdFooter1"/>`
	const headerRef = `<w:headerReference w:type="default" r:id="rId7"/>`

	// Link footer1 to section properties so QR code renders dynamically in the page footer
	if headerRefIdx := strings.Index(xmlStr, headerRef); headerRefIdx != -1 {
		insertPos := headerRefIdx + len(headerRef)
		return xmlStr[:insertPos] + footerRef + xmlStr[insertPos:]
	}

	sectPrIdx := strings.Index(xmlStr, "<w:sectPr")
	if sectPrIdx == -1 {
		return xmlStr
	}

	gtIdx := strings.Index(xmlStr[sectPrIdx:], ">")
	if gtIdx == -1 {
		return xmlStr
	}

	insertPos := sectPrIdx + gtIdx + 1
	return xmlStr[:insertPos] + footerRef + xmlStr[insertPos:]
}

func populateDocumentXML(xmlStr string, payload BeritaAcaraPayload) string {
	xmlStr = populateTableRowsInXML(xmlStr, payload.Items)
	if payload.IsSigned {
		xmlStr = injectFloatingSignatures(xmlStr, payload)
		if len(payload.QRCodeBytes) > 0 && !strings.Contains(xmlStr, "rIdFooter1") {
			xmlStr = injectFooterReference(xmlStr)
		}
	}
	return replaceScalarPlaceholders(xmlStr, payload)
}

const maxZipEntrySize = 10 * 1024 * 1024 // 10MB limit per entry to prevent decompression bombs

func processContentTypesEntry(w io.Writer, rc io.Reader, payload BeritaAcaraPayload) error {
	xmlBytes, rerr := io.ReadAll(io.LimitReader(rc, maxZipEntrySize))
	if rerr != nil {
		return fmt.Errorf("failed to read [Content_Types].xml: %w", rerr)
	}
	ctStr := string(xmlBytes)
	if len(payload.QRCodeBytes) > 0 && !strings.Contains(ctStr, "/word/footer1.xml") {
		ctStr = strings.Replace(ctStr, "</Types>", `<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/></Types>`, 1)
	}
	if !strings.Contains(ctStr, `Extension="png"`) {
		ctStr = strings.Replace(ctStr, "</Types>", `<Default Extension="png" ContentType="image/png"/></Types>`, 1)
	}
	_, werr := io.WriteString(w, ctStr)
	return werr
}

func processRelsEntry(w io.Writer, rc io.Reader, payload BeritaAcaraPayload) error {
	xmlBytes, rerr := io.ReadAll(io.LimitReader(rc, maxZipEntrySize))
	if rerr != nil {
		return fmt.Errorf("failed to read document.xml.rels: %w", rerr)
	}
	relsStr := string(xmlBytes)
	var relsBuilder strings.Builder
	if len(payload.WitnessSignatureBytes) > 0 {
		relsBuilder.WriteString(`<Relationship Id="rIdWitSig" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/wit_sig.png"/>`)
	}
	if len(payload.EmployeeSignatureBytes) > 0 {
		relsBuilder.WriteString(`<Relationship Id="rIdEmpSig" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/emp_sig.png"/>`)
	}
	if len(payload.QRCodeBytes) > 0 && !strings.Contains(relsStr, "footer1.xml") {
		relsBuilder.WriteString(`<Relationship Id="rIdFooter1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>`)
	}
	relsStr = strings.Replace(relsStr, "</Relationships>", relsBuilder.String()+"</Relationships>", 1)
	_, werr := io.WriteString(w, relsStr)
	return werr
}

func processDocumentXMLEntry(w io.Writer, rc io.Reader, payload BeritaAcaraPayload) error {
	xmlBytes, err := io.ReadAll(io.LimitReader(rc, maxZipEntrySize))
	if err != nil {
		return fmt.Errorf("failed to read word/document.xml: %w", err)
	}

	populatedXML := populateDocumentXML(string(xmlBytes), payload)
	if _, err := io.WriteString(w, populatedXML); err != nil {
		return fmt.Errorf("failed to write populated word/document.xml: %w", err)
	}
	return nil
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

	switch {
	case file.Name == "[Content_Types].xml" && payload.IsSigned:
		return processContentTypesEntry(w, rc, payload)
	case file.Name == "word/_rels/document.xml.rels" && payload.IsSigned:
		return processRelsEntry(w, rc, payload)
	case file.Name == "word/document.xml":
		return processDocumentXMLEntry(w, rc, payload)
	default:
		if _, err := io.Copy(w, io.LimitReader(rc, maxZipEntrySize)); err != nil {
			return fmt.Errorf("failed to copy template file %s: %w", file.Name, err)
		}
		return nil
	}
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

	if payload.IsSigned {
		if len(payload.WitnessSignatureBytes) > 0 {
			w, err := writer.Create("word/media/wit_sig.png")
			if err != nil {
				return nil, fmt.Errorf("failed to create media entry wit_sig: %w", err)
			}
			if _, err := w.Write(payload.WitnessSignatureBytes); err != nil {
				return nil, err
			}
		}
		if len(payload.EmployeeSignatureBytes) > 0 {
			w, err := writer.Create("word/media/emp_sig.png")
			if err != nil {
				return nil, fmt.Errorf("failed to create media entry emp_sig: %w", err)
			}
			if _, err := w.Write(payload.EmployeeSignatureBytes); err != nil {
				return nil, err
			}
		}
		if len(payload.QRCodeBytes) > 0 {
			fw, err := writer.Create("word/footer1.xml")
			if err != nil {
				return nil, fmt.Errorf("failed to create word/footer1.xml: %w", err)
			}
			if _, err := io.WriteString(fw, footer1XML); err != nil {
				return nil, err
			}

			frw, err := writer.Create("word/_rels/footer1.xml.rels")
			if err != nil {
				return nil, fmt.Errorf("failed to create word/_rels/footer1.xml.rels: %w", err)
			}
			if _, err := io.WriteString(frw, footer1RelsXML); err != nil {
				return nil, err
			}

			w, err := writer.Create("word/media/doc_qr.png")
			if err != nil {
				return nil, fmt.Errorf("failed to create media entry doc_qr: %w", err)
			}
			if _, err := w.Write(payload.QRCodeBytes); err != nil {
				return nil, err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize docx zip archive: %w", err)
	}

	return buf.Bytes(), nil
}
