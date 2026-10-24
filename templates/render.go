package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
	"time"

	"timesheet-backend/dto/response"
)

//go:embed verify_document.html
var verifyDocHTML string

var verifyTemplate *template.Template

// VerifyDocumentPageData encapsulates the data required by the verification template.
type VerifyDocumentPageData struct {
	Data    *response.PublicDocumentVerificationResponse
	Lang    string
	AppName string
}

// ResolveLanguage detects the preferred language ("id" or "en") based on the query parameter and Accept-Language header.
func ResolveLanguage(acceptLang, queryLang string) string {
	q := strings.TrimSpace(strings.ToLower(queryLang))
	if q == "id" || q == "in" {
		return "id"
	}
	if q == "en" {
		return "en"
	}

	if strings.TrimSpace(acceptLang) == "" {
		return "id"
	}

	// Parse Accept-Language header (RFC 7231)
	// Example: id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7
	var idWeight, enWeight = -1.0, -1.0
	var idPos, enPos = 999, 999

	parts := strings.Split(acceptLang, ",")
	for idx, part := range parts {
		subparts := strings.Split(strings.TrimSpace(part), ";")
		tag := strings.ToLower(strings.TrimSpace(subparts[0]))
		weight := 1.0
		if len(subparts) > 1 {
			for _, param := range subparts[1:] {
				param = strings.TrimSpace(param)
				if strings.HasPrefix(param, "q=") {
					if w, err := strconv.ParseFloat(param[2:], 64); err == nil {
						weight = w
					}
				}
			}
		}

		if strings.HasPrefix(tag, "id") || strings.HasPrefix(tag, "in") {
			if weight > idWeight {
				idWeight = weight
				idPos = idx
			}
		} else if strings.HasPrefix(tag, "en") {
			if weight > enWeight {
				enWeight = weight
				enPos = idx
			}
		}
	}

	if idWeight > enWeight {
		return "id"
	}
	if enWeight > idWeight {
		return "en"
	}
	if idWeight == enWeight && idWeight >= 0 {
		if idPos <= enPos {
			return "id"
		}
		return "en"
	}

	// Non-matching foreign languages fall back to international standard (en)
	return "en"
}

func init() {
	funcMap := template.FuncMap{
		"formatDateWIB": func(t interface{}, lang string) string {
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

			var monthNames [13]string
			if lang == "en" {
				monthNames = [...]string{
					"", "January", "February", "March", "April", "May", "June",
					"July", "August", "September", "October", "November", "December",
				}
			} else {
				monthNames = [...]string{
					"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
					"Juli", "Agustus", "September", "Oktober", "November", "Desember",
				}
			}

			m := int(wibTime.Month())
			mName := ""
			if m >= 1 && m <= 12 {
				mName = monthNames[m]
			}
			return fmt.Sprintf("%02d %s %04d, %02d:%02d WIB",
				wibTime.Day(), mName, wibTime.Year(), wibTime.Hour(), wibTime.Minute())
		},
		"formatMonthName": func(m int, lang string) string {
			var monthNames [13]string
			if lang == "en" {
				monthNames = [...]string{
					"", "January", "February", "March", "April", "May", "June",
					"July", "August", "September", "October", "November", "December",
				}
			} else {
				monthNames = [...]string{
					"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
					"Juli", "Agustus", "September", "Oktober", "November", "Desember",
				}
			}

			if m >= 1 && m <= 12 {
				return monthNames[m]
			}
			return fmt.Sprintf("%d", m)
		},
		"docTypeName": func(docType string, lang string) string {
			if docType == "berita_acara" {
				if lang == "en" {
					return "Overtime Incident Report (Berita Acara)"
				}
				return "Berita Acara Lembur"
			}
			if lang == "en" {
				return "Monthly Timesheet"
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

// RenderVerifyDocument renders the verification result to HTML string with internationalization and custom app name.
func RenderVerifyDocument(data *response.PublicDocumentVerificationResponse, lang, appName string) (string, error) {
	if data == nil {
		data = &response.PublicDocumentVerificationResponse{
			IsValid: false,
			Status:  "invalid",
		}
	}

	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang != "en" && lang != "id" {
		lang = "id"
	}

	appName = strings.TrimSpace(appName)
	if appName == "" {
		if envName := strings.TrimSpace(os.Getenv("APP_NAME")); envName != "" {
			appName = envName
		} else {
			appName = "Timesheet Portal"
		}
	}

	pageData := VerifyDocumentPageData{
		Data:    data,
		Lang:    lang,
		AppName: appName,
	}

	var buf bytes.Buffer
	if err := verifyTemplate.Execute(&buf, pageData); err != nil {
		return "", err
	}
	return buf.String(), nil
}
