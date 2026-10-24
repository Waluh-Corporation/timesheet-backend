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

	tests := []struct {
		name     string
		input    *response.PublicDocumentVerificationResponse
		contains []string
	}{
		{
			name: "Valid Berita Acara",
			input: &response.PublicDocumentVerificationResponse{
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
			},
			contains: []string{
				"DOKUMEN ASLI &amp; TERVERIFIKASI",
				"Berita Acara Lembur",
				"Oktober 2026",
				"John Doe",
				"(12345)",
				"Jane Smith",
				"abcd1234efgh5678",
			},
		},
		{
			name: "Revoked Timesheet",
			input: &response.PublicDocumentVerificationResponse{
				DocumentType: "timesheet",
				Status:       "revoked",
				IsValid:      false,
				Month:        6,
				Year:         2026,
				EmployeeName: "Alice",
				DocumentHash: "hash-revoked",
				VerifiedAt:   now,
			},
			contains: []string{
				"TANDA TANGAN DIBATALKAN",
				"Timesheet Bulanan",
				"Juni 2026",
				"Alice",
			},
		},
		{
			name:  "Nil / Invalid Document",
			input: nil,
			contains: []string{
				"DOKUMEN TIDAK VALID",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html, err := RenderVerifyDocument(tc.input)
			if err != nil {
				t.Fatalf("RenderVerifyDocument failed: %v", err)
			}
			for _, c := range tc.contains {
				if !strings.Contains(html, c) {
					t.Errorf("expected HTML to contain %q, but got:\n%s", c, html)
				}
			}
		})
	}
}
