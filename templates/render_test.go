package templates

import (
	"strings"
	"testing"
	"time"

	"timesheet-backend/dto/response"
)

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

	t.Run("Default UTC Timezone Verification Time", func(t *testing.T) {
		t.Setenv("TZ", "")
		t.Setenv("SCHEDULER_TZ", "")

		html, err := RenderVerifyDocument(validBA, "Acme Enterprise Portal")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		expectedSubstrings := []string{
			"Acme Enterprise Portal",
			"AUTHENTIC &amp; VERIFIED DOCUMENT",
			"Berita Acara - Attendance Incident Report",
			"October 2026",
			"John Doe",
			"(12345)",
			"Jane Smith",
			"ABCD1234EFGH5678",
			"Document Information",
			"Document Type",
			"Employee Signature",
			"13:30 UTC",
			"Witness",
			"14:30 UTC",
			"Verification Time",
			"15:30 UTC",
			"All Rights Reserved",
		}
		for _, s := range expectedSubstrings {
			if !strings.Contains(html, s) {
				t.Errorf("expected HTML to contain %q, but got:\n%s", s, html)
			}
		}

		forbiddenSubstrings := []string{
			"DOKUMEN ASLI",
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
		}
		for _, f := range forbiddenSubstrings {
			if strings.Contains(html, f) {
				t.Errorf("HTML should not contain forbidden text %q", f)
			}
		}
	})

	t.Run("Configured via existing TZ env variable Asia/Jakarta", func(t *testing.T) {
		t.Setenv("TZ", "Asia/Jakarta")

		html, err := RenderVerifyDocument(validBA, "Acme Enterprise Portal")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		expectedSubstrings := []string{
			"22:30 WIB", // 15:30 UTC + 7h = 22:30 WIB
			"20:30 WIB", // 13:30 UTC + 7h = 20:30 WIB
		}
		for _, s := range expectedSubstrings {
			if !strings.Contains(html, s) {
				t.Errorf("expected HTML to contain %q for Asia/Jakarta, but got:\n%s", s, html)
			}
		}
	})

	t.Run("English Revoked Timesheet", func(t *testing.T) {
		t.Setenv("TZ", "UTC")
		html, err := RenderVerifyDocument(revokedTS, "Timesheet System")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		expectedSubstrings := []string{
			"SIGNATURE REVOKED",
			"Monthly Timesheet",
			"June 2026",
			"Alice",
		}
		for _, s := range expectedSubstrings {
			if !strings.Contains(html, s) {
				t.Errorf("expected HTML to contain %q, but got:\n%s", s, html)
			}
		}
	})

	t.Run("Nil / Invalid Document Fallback to Environment APP_NAME", func(t *testing.T) {
		t.Setenv("APP_NAME", "Env Custom System")
		html, err := RenderVerifyDocument(nil, "")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		if !strings.Contains(html, "INVALID DOCUMENT") {
			t.Errorf("expected HTML to contain 'INVALID DOCUMENT', got:\n%s", html)
		}
		if !strings.Contains(html, "Env Custom System") {
			t.Errorf("expected HTML to contain 'Env Custom System', got:\n%s", html)
		}
	})

	t.Run("Document with Audit Trail Timeline", func(t *testing.T) {
		stepTime := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
		docWithTrail := &response.PublicDocumentVerificationResponse{
			DocumentType: "berita_acara",
			Status:       "valid",
			IsValid:      true,
			Month:        10,
			Year:         2026,
			EmployeeName: "John Doe",
			WitnessName:  "Jane Smith",
			DocumentHash: "hash12345",
			VerifiedAt:   now,
			AuditTrail: []response.DocumentAuditTrailItem{
				{
					Step:        1,
					Title:       "Document Submitted & Signed",
					Description: "Signed and submitted by John Doe",
					Actor:       "John Doe",
					Role:        "Employee",
					Timestamp:   &stepTime,
					MaskedIP:    "192.168.***.***",
					Status:      "completed",
				},
				{
					Step:        2,
					Title:       "Witness Signature Affixed",
					Description: "Approved and signed by Jane Smith",
					Actor:       "Jane Smith",
					Role:        "Witness",
					Timestamp:   &stepTime,
					MaskedIP:    "10.0.***.***",
					Status:      "completed",
				},
				{
					Step:        3,
					Title:       "Witness Review Revoked",
					Description: "Submission was recalled by submitter prior to witness completion",
					Actor:       "Jane Smith",
					Role:        "Witness",
					Timestamp:   nil,
					Status:      "revoked",
				},
			},
		}

		html, err := RenderVerifyDocument(docWithTrail, "Timesheet Portal")
		if err != nil {
			t.Fatalf("RenderVerifyDocument with audit trail failed: %v", err)
		}

		expectedSnippets := []string{
			"Signing Audit Trail",
			"Document Submitted &amp; Signed",
			"Signed and submitted by John Doe",
			"Witness Signature Affixed",
			"IP: 192.168.***.***",
			"IP: 10.0.***.***",
			"Witness Review Revoked",
			"Revoked",
		}
		for _, snip := range expectedSnippets {
			if !strings.Contains(html, snip) {
				t.Errorf("expected HTML to contain %q, but got:\n%s", snip, html)
			}
		}
	})
}
