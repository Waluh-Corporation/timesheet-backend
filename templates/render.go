package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
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
	AppName string
}

var monthNames = [...]string{
	"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

func parseTime(t any) (time.Time, bool) {
	if t == nil {
		return time.Time{}, false
	}
	switch val := t.(type) {
	case *time.Time:
		if val == nil || val.IsZero() {
			return time.Time{}, false
		}
		return *val, true
	case time.Time:
		if val.IsZero() {
			return time.Time{}, false
		}
		return val, true
	default:
		return time.Time{}, false
	}
}

func formatDateWIB(t any) string {
	tm, ok := parseTime(t)
	if !ok {
		return "-"
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	wibTime := tm.In(loc)

	m := int(wibTime.Month())
	mName := ""
	if m >= 1 && m <= 12 {
		mName = monthNames[m]
	}
	return fmt.Sprintf("%02d %s %04d, %02d:%02d WIB",
		wibTime.Day(), mName, wibTime.Year(), wibTime.Hour(), wibTime.Minute())
}

func formatMonthName(m int) string {
	if m >= 1 && m <= 12 {
		return monthNames[m]
	}
	return fmt.Sprintf("%d", m)
}

func docTypeName(docType string) string {
	if docType == "berita_acara" {
		return "Overtime Incident Report"
	}
	return "Monthly Timesheet"
}

func init() {
	funcMap := template.FuncMap{
		"formatDateWIB":   formatDateWIB,
		"formatMonthName": formatMonthName,
		"docTypeName":     docTypeName,
	}

	var err error
	verifyTemplate, err = template.New("verify_document.html").Funcs(funcMap).Parse(verifyDocHTML)
	if err != nil {
		panic(fmt.Sprintf("failed to parse verify_document.html template: %v", err))
	}
}

// RenderVerifyDocument renders the verification result to English HTML string with custom app name.
func RenderVerifyDocument(data *response.PublicDocumentVerificationResponse, appName string) (string, error) {
	if data == nil {
		data = &response.PublicDocumentVerificationResponse{
			IsValid: false,
			Status:  "invalid",
		}
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
		AppName: appName,
	}

	var buf bytes.Buffer
	if err := verifyTemplate.Execute(&buf, pageData); err != nil {
		return "", err
	}
	return buf.String(), nil
}
