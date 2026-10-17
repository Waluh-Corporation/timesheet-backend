package services

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/models"
)

func TestFormatSignatureShortName(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		expected string
	}{
		{
			name:     "empty string",
			fullName: "   ",
			expected: "",
		},
		{
			name:     "single word",
			fullName: "Faisal",
			expected: "Faisal",
		},
		{
			name:     "two words unchanged",
			fullName: "Faisal Rahman",
			expected: "Faisal Rahman",
		},
		{
			name:     "multi-word name abbreviated to FirstName LastName",
			fullName: "Faisal Al Munawar Fathur Rahman",
			expected: "Faisal Rahman",
		},
		{
			name:     "three words abbreviated to FirstName LastName",
			fullName: "Budi Santoso Wijaya",
			expected: "Budi Wijaya",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FormatSignatureShortName(tc.fullName))
		})
	}
}

func TestRenderTypedSignaturePNG(t *testing.T) {
	t.Run("empty text returns error", func(t *testing.T) {
		out, err := RenderTypedSignaturePNG("   ")
		assert.Error(t, err)
		assert.Nil(t, out)
	})

	t.Run("renders cursive signature with transparent background", func(t *testing.T) {
		out, err := RenderTypedSignaturePNG("Faisal Rahman")
		require.NoError(t, err)
		require.NotEmpty(t, out)

		img, err := png.Decode(bytes.NewReader(out))
		require.NoError(t, err)
		bounds := img.Bounds()
		assert.Greater(t, bounds.Dx(), 100)
		assert.Greater(t, bounds.Dy(), 20)

		// Top-left corner pixel should be transparent
		_, _, _, a := img.At(bounds.Min.X, bounds.Min.Y).RGBA()
		assert.Equal(t, uint32(0), a)
	})
}

func TestNormalizeSignaturePNG_And_DecodeDataURI(t *testing.T) {
	// Create a synthetic 200x100 image with white background and a dark stroke in the center
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			src.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	for x := 50; x < 150; x++ {
		for y := 40; y < 55; y++ {
			src.Set(x, y, color.RGBA{R: 15, G: 23, B: 42, A: 255})
		}
	}

	var rawBuf bytes.Buffer
	require.NoError(t, png.Encode(&rawBuf, src))

	t.Run("normalizes white background to transparent and crops to ink bounds", func(t *testing.T) {
		normBytes, err := NormalizeSignaturePNG(rawBuf.Bytes())
		require.NoError(t, err)
		require.NotEmpty(t, normBytes)

		decoded, err := png.Decode(bytes.NewReader(normBytes))
		require.NoError(t, err)
		// Cropped dimensions should be smaller than the original 200x100
		assert.Less(t, decoded.Bounds().Dx(), 200)
		assert.Less(t, decoded.Bounds().Dy(), 100)

		// Corner pixel of padding should be transparent
		_, _, _, a := decoded.At(decoded.Bounds().Min.X, decoded.Bounds().Min.Y).RGBA()
		assert.Equal(t, uint32(0), a)
	})

	t.Run("decodes base64 data URI", func(t *testing.T) {
		uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(rawBuf.Bytes())
		decodedBytes, ok := DecodeDataURISignature(uri)
		require.True(t, ok)
		assert.Equal(t, rawBuf.Bytes(), decodedBytes)
	})

	t.Run("invalid data URI returns false", func(t *testing.T) {
		_, ok := DecodeDataURISignature("not-a-data-uri")
		assert.False(t, ok)
	})
}

func TestExcelAndBeritaAcaraSignatureInjection(t *testing.T) {
	sigPNG, err := RenderTypedSignaturePNG("Faisal Rahman")
	require.NoError(t, err)

	witPNG, err := RenderTypedSignaturePNG("Budi Santoso")
	require.NoError(t, err)

	user := &models.User{
		ID:         1,
		Username:   "faisal",
		Name:       "Faisal Al Munawar Fathur Rahman",
		BniID:      "NPP12345",
		Department: "IT Division",
		GroupName:  "Core Banking",
	}
	actDate := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	activities := []models.DailyActivity{
		{
			UserID:           1,
			Date:             actDate,
			StartTime:        "08:00",
			EndTime:          "17:00",
			Status:           "Hadir",
			Activity:         "Implemented DocuSeal e-signature",
			IsBeritaAcara:    true,
			BeritaAcaraNotes: "Deployment verification",
		},
	}
	overtimes := []models.OvertimeEntry{
		{
			UserID:          1,
			Date:            actDate,
			StartTime:       "18:00",
			EndTime:         "20:00",
			TaskDescription: "Release support",
		},
	}

	for _, companyCode := range []string{"MII", "SDD", "ADIDATA", "NTT"} {
		t.Run("Excel_"+companyCode+"_WithSignature", func(t *testing.T) {
			out, gerr := GenerateFromTemplate(GenerationInput{
				CompanyCode:      companyCode,
				User:             user,
				Month:            7,
				Year:             2026,
				Activities:       activities,
				Overtimes:        overtimes,
				UserSignaturePNG: sigPNG,
			})
			require.NoError(t, gerr)
			require.NotEmpty(t, out)
		})
	}

	t.Run("BeritaAcara_WithEmployeeAndWitnessSignatures", func(t *testing.T) {
		docxBytes, berr := GenerateBeritaAcaraDocx(BeritaAcaraPayload{
			EmployeeName: "Faisal Al Munawar Fathur Rahman",
			NPP:          "NPP12345",
			Department:   "IT Division",
			GroupName:    "Core Banking",
			WitnessName:  "Budi Santoso",
			TeamLeadName: "Andi Pratama",
			Items: []BeritaAcaraItem{
				{
					Date:      actDate,
					Day:       "Rabu",
					StartTime: "08:00",
					EndTime:   "17:00",
					Notes:     "Deployment verification",
				},
			},
			EmployeeSignaturePNG: sigPNG,
			WitnessSignaturePNG:  witPNG,
		})
		require.NoError(t, berr)
		require.NotEmpty(t, docxBytes)

		zr, zerr := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
		require.NoError(t, zerr)

		var hasEmpMedia, hasWitMedia, hasDrawingInDoc bool
		for _, f := range zr.File {
			if f.Name == "word/media/sig_employee.png" {
				hasEmpMedia = true
			}
			if f.Name == "word/media/sig_witness.png" {
				hasWitMedia = true
			}
			if f.Name == "word/document.xml" {
				rc, oerr := f.Open()
				require.NoError(t, oerr)
				buf := new(bytes.Buffer)
				_, _ = buf.ReadFrom(rc)
				_ = rc.Close()
				if strings.Contains(buf.String(), "rIdSigEmp") && strings.Contains(buf.String(), "rIdSigWit") {
					hasDrawingInDoc = true
				}
			}
		}
		assert.True(t, hasEmpMedia, "expected word/media/sig_employee.png in docx archive")
		assert.True(t, hasWitMedia, "expected word/media/sig_witness.png in docx archive")
		assert.True(t, hasDrawingInDoc, "expected inline drawing relationship IDs in word/document.xml")
	})
}
