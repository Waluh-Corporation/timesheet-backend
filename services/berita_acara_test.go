package services_test

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/services"
)

func TestIndonesianDayName(t *testing.T) {
	tests := []struct {
		day  time.Weekday
		want string
	}{
		{time.Sunday, "Minggu"},
		{time.Monday, "Senin"},
		{time.Tuesday, "Selasa"},
		{time.Wednesday, "Rabu"},
		{time.Thursday, "Kamis"},
		{time.Friday, "Jumat"},
		{time.Saturday, "Sabtu"},
		{time.Weekday(99), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := services.IndonesianDayName(tt.day)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateBeritaAcaraDocx_Success(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	require.NoError(t, err)

	date1 := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	date2 := time.Date(2026, 11, 15, 0, 0, 0, 0, loc)

	payload := services.BeritaAcaraPayload{
		EmployeeName: "Budi Santoso",
		NPP:          "BNI-98765",
		Department:   "Wholesale Channel and Service Delivery",
		GroupName:    "SDD Development",
		WitnessName:  "Citra Lestari",
		TeamLeadName: "Agus Pratama",
		Items: []services.BeritaAcaraItem{
			{
				Date:      date1,
				Day:       "Senin",
				StartTime: "08:15",
				EndTime:   "17:30",
				Notes:     "Lupa tap in karena gerbang antrean panjang",
			},
			{
				Date:      date2,
				Day:       "Minggu",
				StartTime: "09:00",
				EndTime:   "18:00",
				Notes:     "Maintenance server darurat di luar jam kerja",
			},
		},
	}

	docxBytes, err := services.GenerateBeritaAcaraDocx(payload)
	require.NoError(t, err)
	require.NotEmpty(t, docxBytes)

	// Validate valid docx zip structure
	zipReader, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	require.NoError(t, err)

	var docXML string
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			require.NoError(t, err)
			b, err := io.ReadAll(rc)
			require.NoError(t, err)
			_ = rc.Close()
			docXML = string(b)
			break
		}
	}

	require.NotEmpty(t, docXML)
	assert.Contains(t, docXML, "Budi Santoso")
	assert.Contains(t, docXML, "BNI-98765")
	assert.Contains(t, docXML, "Wholesale Channel and Service Delivery")
	assert.Contains(t, docXML, "SDD Development")
	assert.Contains(t, docXML, "Citra Lestari")
	assert.Contains(t, docXML, "Agus Pratama")
	assert.Contains(t, docXML, "02/11/2026")
	assert.Contains(t, docXML, "15/11/2026")
	assert.Contains(t, docXML, "08:15")
	assert.Contains(t, docXML, "17:30")
	assert.Contains(t, docXML, "Lupa tap in karena gerbang antrean panjang")
	assert.Contains(t, docXML, "Maintenance server darurat di luar jam kerja")

	// Ensure no unresolved placeholders remain
	assert.NotContains(t, docXML, "{{NAME}}")
	assert.NotContains(t, docXML, "{{NPP}}")
	assert.NotContains(t, docXML, "{{DEPARTMENT}}")
	assert.NotContains(t, docXML, "{{GROUP}}")
	assert.NotContains(t, docXML, "{{WITNESS_NAME}}")
	assert.NotContains(t, docXML, "{{EMPLOYEE_NAME}}")
	assert.NotContains(t, docXML, "{{TEAM_LEAD}}")
	assert.NotContains(t, docXML, "{{TANGGAL}}")
}

func TestGenerateBeritaAcaraDocx_SpecialCharactersEscaping(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	require.NoError(t, err)

	date := time.Date(2026, 11, 5, 0, 0, 0, 0, loc)

	payload := services.BeritaAcaraPayload{
		EmployeeName: "John & Jane <Developers>",
		NPP:          "BNI-\"123\"",
		Department:   "R&D / QA 'Special'",
		GroupName:    "Core & Integration",
		WitnessName:  "Witness & Co <Lead>",
		TeamLeadName: "Manager & Supervisor",
		Items: []services.BeritaAcaraItem{
			{
				Date:      date,
				Day:       "Kamis",
				StartTime: "08:00",
				EndTime:   "17:00",
				Notes:     "Testing <tags> & 'quotes' \"escaped\"",
			},
		},
	}

	docxBytes, err := services.GenerateBeritaAcaraDocx(payload)
	require.NoError(t, err)
	require.NotEmpty(t, docxBytes)

	zipReader, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	require.NoError(t, err)

	var docXML string
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			require.NoError(t, err)
			b, err := io.ReadAll(rc)
			require.NoError(t, err)
			_ = rc.Close()
			docXML = string(b)
			break
		}
	}

	require.NotEmpty(t, docXML)
	assert.Contains(t, docXML, "John &amp; Jane &lt;Developers&gt;")
	assert.Contains(t, docXML, "BNI-&quot;123&quot;")
	assert.Contains(t, docXML, "R&amp;D / QA &apos;Special&apos;")
	assert.Contains(t, docXML, "Testing &lt;tags&gt; &amp; &apos;quotes&apos; &quot;escaped&quot;")
}

func TestGenerateBeritaAcaraDocx_EmptyItems(t *testing.T) {
	payload := services.BeritaAcaraPayload{
		EmployeeName: "Budi Santoso",
		NPP:          "BNI-98765",
		Department:   "Wholesale Channel and Service Delivery",
		GroupName:    "SDD Development",
		WitnessName:  "Citra Lestari",
		TeamLeadName: "Agus Pratama",
		Items:        []services.BeritaAcaraItem{},
	}

	docxBytes, err := services.GenerateBeritaAcaraDocx(payload)
	require.NoError(t, err)
	require.NotEmpty(t, docxBytes)

	zipReader, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	require.NoError(t, err)

	var docXML string
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			require.NoError(t, err)
			b, err := io.ReadAll(rc)
			require.NoError(t, err)
			_ = rc.Close()
			docXML = string(b)
			break
		}
	}

	require.NotEmpty(t, docXML)
	assert.Contains(t, docXML, "Budi Santoso")
	assert.False(t, strings.Contains(docXML, "{{TANGGAL}}"))
}
