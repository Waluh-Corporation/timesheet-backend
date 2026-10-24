package templates

import (
	"strings"
	"testing"
	"time"

	"timesheet-backend/dto/response"
)

func assertContains(t *testing.T, html string, expected []string) {
	t.Helper()
	for _, s := range expected {
		if !strings.Contains(html, s) {
			t.Errorf("expected HTML to contain %q, but got:\n%s", s, html)
		}
	}
}

func assertNotContains(t *testing.T, html string, forbidden []string) {
	t.Helper()
	for _, f := range forbidden {
		if strings.Contains(html, f) {
			t.Errorf("HTML should not contain forbidden text %q", f)
		}
	}
}

func TestRenderVerifyDocument(t *testing.T) {
	now := time.Date(2026, 10, 24, 15, 30, 0, 0, time.UTC)
	empSigned := now.Add(-2 * time.Hour)
	witSigned := now.Add(-1 * time.Hour)

	validBA := &response.PublicDocumentVerificationResponse{
		DocumentType:     "berita_acara",
		Status:           "valid",
		IsValid:          true,
		Month:            10,
		Year:             2026,
		EmployeeName:     "John Doe",
		EmployeeBNIID:    "12345",
		EmployeeSignedAt: &empSigned,
		WitnessName:      "Jane Smith",
		WitnessSignedAt:  &witSigned,
		DocumentHash:     "abcd1234efgh5678",
		VerifiedAt:       now,
	}

	revokedTS := &response.PublicDocumentVerificationResponse{
		DocumentType: "timesheet",
		Status:       "revoked",
		IsValid:      false,
		Month:        6,
		Year:         2026,
		EmployeeName: "Alice",
		DocumentHash: "hash-revoked",
		VerifiedAt:   now,
	}

	t.Run("English Language Rendering with Custom AppName", func(t *testing.T) {
		html, err := RenderVerifyDocument(validBA, "Acme Enterprise Portal")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		assertContains(t, html, []string{
			"Acme Enterprise Portal",
			"AUTHENTIC &amp; VERIFIED DOCUMENT",
			"Overtime Incident Report",
			"October 2026",
			"John Doe",
			"(12345)",
			"Jane Smith",
			"abcd1234efgh5678",
			"Document Information",
			"Document Type",
			"Employee Signature",
			"Witness",
			"Witness Signature",
			"Verification Time",
			"All Rights Reserved",
		})

		// Ensure no Indonesian text, no copy button, and no BNI branding remains
		assertNotContains(t, html, []string{
			"DOKUMEN ASLI",
			"Berita Acara",
			"Oktober",
			"Informasi Dokumen",
			"Jenis Dokumen",
			"Hak Cipta Dilindungi",
			"Bahasa Indonesia",
			"copy-btn",
			"copyDocumentHash",
			"<button",
			"Bank Negara Indonesia",
			"BNI",
		})
	})

	t.Run("English Revoked Timesheet", func(t *testing.T) {
		html, err := RenderVerifyDocument(revokedTS, "Timesheet System")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		assertContains(t, html, []string{
			"SIGNATURE REVOKED",
			"Monthly Timesheet",
			"June 2026",
			"Alice",
		})
	})

	t.Run("Nil / Invalid Document Fallback to Environment APP_NAME", func(t *testing.T) {
		t.Setenv("APP_NAME", "Env Custom System")
		html, err := RenderVerifyDocument(nil, "")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		assertContains(t, html, []string{
			"INVALID DOCUMENT",
			"Env Custom System",
		})
	})
}
