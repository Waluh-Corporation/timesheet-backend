package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

// BeritaAcaraInput contains the context and entities needed to generate a Berita Acara document.
type BeritaAcaraInput struct {
	User       *models.User
	Witness    *models.User
	Month      int
	Year       int
	Activities []models.DailyActivity
	Approvers  []models.Approver
}

// DayNameIndonesian returns the Indonesian day name for a time.Weekday.
func DayNameIndonesian(w time.Weekday) string {
	switch w {
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

// GenerateBeritaAcara renders the Berita Acara document using the master docx template,
// GenerateBeritaAcara renders the Berita Acara document using the master docx template,
// filling in employee metadata, attendance entries table, and witness, employee, and team lead names.
func GenerateBeritaAcara(in BeritaAcaraInput) ([]byte, error) {
	if len(assets.BeritaAcaraTemplate) == 0 {
		return nil, fmt.Errorf("berita acara template is not loaded")
	}
	if in.User == nil {
		return nil, fmt.Errorf("user is required")
	}
	if in.Witness == nil {
		return nil, fmt.Errorf("witness is required")
	}

	zr, err := zip.NewReader(bytes.NewReader(assets.BeritaAcaraTemplate), int64(len(assets.BeritaAcaraTemplate)))
	if err != nil {
		return nil, fmt.Errorf("failed to open docx zip reader: %w", err)
	}

	files := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open %s in docx: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read %s in docx: %w", f.Name, err)
		}
		files[f.Name] = data
	}

	docXMLBytes, ok := files["word/document.xml"]
	if !ok {
		return nil, fmt.Errorf("word/document.xml not found in template")
	}
	docXML := string(docXMLBytes)

	// 1. Process metadata: Nama, NPP BNI, Departement, Kelompok
	userName := in.User.Name
	if userName == "" {
		userName = in.User.Username
	}
	bniID := in.User.BniID
	if bniID == "" {
		bniID = in.User.EmployeeID
	}
	dept := in.User.Department
	if in.User.DepartmentRel != nil && in.User.DepartmentRel.Name != "" {
		dept = in.User.DepartmentRel.Name
	}
	group := in.User.GroupName
	if group == "" {
		group = in.User.Division
	}

	docXML = fillMetadataFields(docXML, userName, bniID, dept, group)

	// 2. Process Activity Table
	docXML = fillActivityTable(docXML, in.Activities)

	// 3. Process Signatures / Names
	docXML = processSignatures(docXML, in)

	files["word/document.xml"] = []byte(docXML)

	// Repack into new docx zip
	var outBuf bytes.Buffer
	zw := zip.NewWriter(&outBuf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip file %s: %w", name, err)
		}
		if _, err := w.Write(data); err != nil {
			return nil, fmt.Errorf("failed to write zip file %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize docx zip: %w", err)
	}

	return outBuf.Bytes(), nil
}

func fillMetadataFields(docXML, name, bniID, dept, group string) string {
	// Nama: replace placeholder after colon in Nama paragraph
	docXML = replaceAfterColon(docXML, "Nama", name)
	// NPP BNI:
	docXML = replaceAfterColon(docXML, "NPP", bniID)
	// Departement:
	docXML = replaceAfterColon(docXML, "Departement", dept)
	// Kelompok:
	docXML = replaceAfterColon(docXML, "Kelompok", group)
	return docXML
}

func replaceAfterColon(docXML, labelKey, value string) string {
	idx := strings.Index(docXML, ">"+labelKey+"<")
	if idx == -1 {
		return docXML
	}
	sub := docXML[idx:]
	colonIdx := strings.Index(sub, ">:<")
	if colonIdx == -1 {
		colonIdx = strings.Index(sub, ">: <")
	}
	if colonIdx == -1 {
		return docXML
	}

	absColonIdx := idx + colonIdx
	afterColon := docXML[absColonIdx:]
	pEndIdx := strings.Index(afterColon, "</w:p>")
	if pEndIdx == -1 {
		return docXML
	}

	targetChunk := afterColon[:pEndIdx]
	replacementRun := fmt.Sprintf(`<w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr><w:t xml:space="preserve"> %s</w:t></w:r>`, xmlEscape(value))

	newChunk := targetChunk + replacementRun
	return docXML[:absColonIdx] + newChunk + docXML[absColonIdx+pEndIdx:]
}

func fillActivityTable(docXML string, activities []models.DailyActivity) string {
	tblStart := strings.Index(docXML, "<w:tbl>")
	if tblStart == -1 {
		return docXML
	}
	tblEnd := strings.Index(docXML[tblStart:], "</w:tbl>")
	if tblEnd == -1 {
		return docXML
	}
	tblContent := docXML[tblStart : tblStart+tblEnd]

	// Find first data row (the second <w:tr> in the table)
	firstRowEnd := strings.Index(tblContent, "</w:tr>")
	if firstRowEnd == -1 {
		return docXML
	}
	headerEnd := firstRowEnd + len("</w:tr>")
	headerPart := tblContent[:headerEnd]

	var rowBuilder strings.Builder
	for _, act := range activities {
		dateStr := act.Date.Format("02/01/2006")
		dayStr := DayNameIndonesian(act.Date.Weekday())
		startTime := act.StartTime
		endTime := act.EndTime
		keterangan := act.Activity
		if strings.TrimSpace(keterangan) == "" {
			keterangan = act.GetProjectName()
		}
		if strings.TrimSpace(keterangan) == "" {
			keterangan = "Hadir"
		}

		rowBuilder.WriteString(buildTableRowXML(dateStr, dayStr, startTime, endTime, keterangan))
	}

	if len(activities) == 0 {
		rowBuilder.WriteString(buildTableRowXML("", "", "", "", ""))
	}

	newTable := headerPart + rowBuilder.String()
	return docXML[:tblStart] + newTable + docXML[tblStart+tblEnd:]
}

func buildTableRowXML(dateStr, dayStr, startTime, endTime, keterangan string) string {
	return fmt.Sprintf(`<w:tr><w:trPr><w:trHeight w:val="360"/></w:trPr>`+
		`<w:tc><w:tcPr><w:tcW w:w="2268" w:type="dxa"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:pPr><w:jc w:val="center"/><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr><w:t>%s</w:t></w:r></w:p></w:tc>`+
		`<w:tc><w:tcPr><w:tcW w:w="993" w:type="dxa"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:pPr><w:jc w:val="center"/><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr><w:t>%s</w:t></w:r></w:p></w:tc>`+
		`<w:tc><w:tcPr><w:tcW w:w="1417" w:type="dxa"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:pPr><w:jc w:val="center"/><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr><w:t>%s</w:t></w:r></w:p></w:tc>`+
		`<w:tc><w:tcPr><w:tcW w:w="1418" w:type="dxa"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:pPr><w:jc w:val="center"/><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr><w:t>%s</w:t></w:r></w:p></w:tc>`+
		`<w:tc><w:tcPr><w:tcW w:w="4110" w:type="dxa"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:pPr><w:spacing w:line="240" w:lineRule="auto"/><w:jc w:val="left"/><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="18"/></w:rPr><w:t>%s</w:t></w:r></w:p></w:tc>`+
		`</w:tr>`,
		xmlEscape(dateStr),
		xmlEscape(dayStr),
		xmlEscape(startTime),
		xmlEscape(endTime),
		xmlEscape(keterangan),
	)
}

func processSignatures(docXML string, in BeritaAcaraInput) string {
	witnessName := in.Witness.Name
	if witnessName == "" {
		witnessName = in.Witness.Username
	}
	userName := in.User.Name
	if userName == "" {
		userName = in.User.Username
	}

	tlName := resolveApproverTeamLead(in.Approvers)

	// Replace placeholder names
	docXML = strings.Replace(docXML, "Nama Saksi", xmlEscape(witnessName), 1)
	docXML = strings.Replace(docXML, "Nama Karyawan", xmlEscape(userName), 1)
	docXML = strings.Replace(docXML, "Nama Team Lead", xmlEscape(tlName), 1)

	return docXML
}

func resolveApproverTeamLead(approvers []models.Approver) string {
	for _, a := range approvers {
		if a.RoleType == models.ApproverRoleTeamLeader && a.IsActive {
			return a.Name
		}
	}
	return "Team Leader"
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
