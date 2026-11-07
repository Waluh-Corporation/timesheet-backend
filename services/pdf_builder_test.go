package services

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"timesheet-backend/models"
)

func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 50, G: 100, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestColumnSchemas_SumTwelve(t *testing.T) {
	t.Run("StandardTimesheetPDFColumns grid ratio sums to 12", func(t *testing.T) {
		cols := StandardTimesheetPDFColumns()
		sum := 0
		for _, c := range cols {
			sum += c.GridRatio
		}
		if sum != 12 {
			t.Fatalf("expected sum of timesheet grid ratios to be 12, got %d", sum)
		}
	})

	t.Run("BeritaAcaraPDFColumns grid ratio sums to 12", func(t *testing.T) {
		cols := BeritaAcaraPDFColumns()
		sum := 0
		for _, c := range cols {
			sum += c.GridRatio
		}
		if sum != 12 {
			t.Fatalf("expected sum of berita acara grid ratios to be 12, got %d", sum)
		}
	})
}

func TestBuildTimesheetPDF(t *testing.T) {
	ctx := context.Background()

	t.Run("nil model returns error", func(t *testing.T) {
		_, err := BuildTimesheetPDF(ctx, nil)
		if err == nil {
			t.Fatal("expected error for nil model, got nil")
		}
	})

	t.Run("canceled context returns context error", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		model := &TimesheetPresentationModel{Month: 10, Year: 2026}
		_, err := BuildTimesheetPDF(cancelCtx, model)
		if err == nil {
			t.Fatal("expected error on canceled context, got nil")
		}
	})

	t.Run("renders valid landscape PDF with magic bytes", func(t *testing.T) {
		testPNG := createTestPNG(80, 40)
		input := &GenerationInput{
			CompanyCode: "mii",
			User: &models.User{
				Name:       "Citra Dewi",
				Username:   "citra.d",
				BniID:      "654321",
				Division:   "Engineering",
				Department: "Core Systems",
				Site:       "Jakarta",
				Company:    "PT Mitra Integrasi Informatika",
			},
			Month:          10,
			Year:           2026,
			SignatureBytes: testPNG,
			QRCodeBytes:    testPNG,
			IsSigned:       true,
		}

		model, err := CalculateTimesheetModel(input)
		if err != nil {
			t.Fatalf("CalculateTimesheetModel failed: %v", err)
		}

		pdfBytes, err := BuildTimesheetPDF(ctx, model)
		if err != nil {
			t.Fatalf("BuildTimesheetPDF failed: %v", err)
		}

		if len(pdfBytes) < 5 {
			t.Fatalf("expected non-empty PDF bytes, got %d bytes", len(pdfBytes))
		}
		if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
			t.Fatalf("expected output to begin with %%PDF- magic bytes, got %q", string(pdfBytes[:min(10, len(pdfBytes))]))
		}
	})
}

func TestBuildBeritaAcaraPDF(t *testing.T) {
	ctx := context.Background()

	t.Run("nil payload returns error", func(t *testing.T) {
		_, err := BuildBeritaAcaraPDF(ctx, nil)
		if err == nil {
			t.Fatal("expected error for nil payload, got nil")
		}
	})

	t.Run("canceled context returns context error", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		payload := &BeritaAcaraPayload{EmployeeName: "Test"}
		_, err := BuildBeritaAcaraPDF(cancelCtx, payload)
		if err == nil {
			t.Fatal("expected error on canceled context, got nil")
		}
	})

	t.Run("renders valid portrait PDF with incidents and signatures", func(t *testing.T) {
		sigPNG := createTestPNG(100, 50)
		qrPNG := createTestPNG(60, 60)

		payload := &BeritaAcaraPayload{
			EmployeeName: "Fajar Nugraha",
			NPP:          "98765",
			Department:   "Information Technology",
			GroupName:    "DevOps",
			WitnessName:  "Indra Kusuma",
			Items: []BeritaAcaraItem{
				{
					Date:      time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
					Day:       "Senin",
					StartTime: "08:30",
					EndTime:   "17:30",
					Notes:     "Kartu akses tertinggal di rumah",
				},
				{
					Date:      time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
					Day:       "Senin",
					StartTime: "08:45",
					EndTime:   "17:45",
					Notes:     "Portal biometric SSO under maintenance",
				},
			},
			EmployeeSignatureBytes: sigPNG,
			WitnessSignatureBytes:  sigPNG,
			QRCodeBytes:            qrPNG,
			IsSigned:               true,
		}

		pdfBytes, err := BuildBeritaAcaraPDF(ctx, payload)
		if err != nil {
			t.Fatalf("BuildBeritaAcaraPDF failed: %v", err)
		}

		if len(pdfBytes) < 5 {
			t.Fatalf("expected non-empty PDF bytes, got %d bytes", len(pdfBytes))
		}
		if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
			t.Fatalf("expected output to begin with %%PDF- magic bytes, got %q", string(pdfBytes[:min(10, len(pdfBytes))]))
		}
	})

	t.Run("renders valid portrait PDF when incidents list is empty", func(t *testing.T) {
		payload := &BeritaAcaraPayload{
			EmployeeName: "Fajar Nugraha",
			NPP:          "98765",
			Department:   "Information Technology",
			GroupName:    "DevOps",
			WitnessName:  "Indra Kusuma",
			Items:        nil,
			IsSigned:     false,
		}

		pdfBytes, err := BuildBeritaAcaraPDF(ctx, payload)
		if err != nil {
			t.Fatalf("BuildBeritaAcaraPDF with empty items failed: %v", err)
		}

		if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
			t.Fatalf("expected %%PDF- prefix, got %q", string(pdfBytes[:min(10, len(pdfBytes))]))
		}
	})
}
