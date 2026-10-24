package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"time"

	"timesheet-backend/dto/response"
)

//go:embed verify_document.html
var verifyDocHTML string

var verifyTemplate *template.Template

func init() {
	funcMap := template.FuncMap{
		"formatDateWIB": func(t interface{}) string {
			if t == nil {
				return "-"
			}
			var tm time.Time
			switch val := t.(type) {
			case *time.Time:
				if val == nil || val.IsZero() {
					return "-"
				}
				tm = *val
			case time.Time:
				if val.IsZero() {
					return "-"
				}
				tm = val
			default:
				return "-"
			}

			loc, err := time.LoadLocation("Asia/Jakarta")
			if err != nil {
				loc = time.FixedZone("WIB", 7*3600)
			}
			wibTime := tm.In(loc)
			monthNames := [...]string{
				"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
				"Juli", "Agustus", "September", "Oktober", "November", "Desember",
			}
			m := int(wibTime.Month())
			mName := ""
			if m >= 1 && m <= 12 {
				mName = monthNames[m]
			}
			return fmt.Sprintf("%02d %s %04d, %02d:%02d WIB",
				wibTime.Day(), mName, wibTime.Year(), wibTime.Hour(), wibTime.Minute())
		},
		"formatMonthName": func(m int) string {
			monthNames := [...]string{
				"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
				"Juli", "Agustus", "September", "Oktober", "November", "Desember",
			}
			if m >= 1 && m <= 12 {
				return monthNames[m]
			}
			return fmt.Sprintf("%d", m)
		},
		"docTypeName": func(docType string) string {
			if docType == "berita_acara" {
				return "Berita Acara Lembur"
			}
			return "Timesheet Bulanan"
		},
	}

	var err error
	verifyTemplate, err = template.New("verify_document.html").Funcs(funcMap).Parse(verifyDocHTML)
	if err != nil {
		panic(fmt.Sprintf("failed to parse verify_document.html template: %v", err))
	}
}

// RenderVerifyDocument renders the verification result to HTML string.
func RenderVerifyDocument(data *response.PublicDocumentVerificationResponse) (string, error) {
	if data == nil {
		data = &response.PublicDocumentVerificationResponse{
			IsValid: false,
			Status:  "invalid",
		}
	}
	var buf bytes.Buffer
	if err := verifyTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
