package templates

import (
	"strings"
	"testing"
	"time"

	"timesheet-backend/dto/response"
)

func TestResolveLanguage(t *testing.T) {
	tests := []struct {
		name       string
		acceptLang string
		queryLang  string
		expected   string
	}{
		{
			name:       "Query param en overrides header",
			acceptLang: "id-ID,id;q=0.9",
			queryLang:  "en",
			expected:   "en",
		},
		{
			name:       "Query param id overrides header",
			acceptLang: "en-US,en;q=0.9",
			queryLang:  "id",
			expected:   "id",
		},
		{
			name:       "Accept-Language id higher weight",
			acceptLang: "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7",
			queryLang:  "",
			expected:   "id",
		},
		{
			name:       "Accept-Language en higher weight",
			acceptLang: "en-US,en;q=0.9,id;q=0.8",
			queryLang:  "",
			expected:   "en",
		},
		{
			name:       "Equal weight takes first position (id first)",
			acceptLang: "id,en",
			queryLang:  "",
			expected:   "id",
		},
		{
			name:       "Equal weight takes first position (en first)",
			acceptLang: "en,id",
			queryLang:  "",
			expected:   "en",
		},
		{
			name:       "Non-matching language falls back to en",
			acceptLang: "ja-JP,ja;q=0.9,fr;q=0.8",
			queryLang:  "",
			expected:   "en",
		},
		{
			name:       "Empty header and query defaults to id",
			acceptLang: "",
			queryLang:  "",
			expected:   "id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveLanguage(tc.acceptLang, tc.queryLang)
			if got != tc.expected {
				t.Errorf("ResolveLanguage(%q, %q) = %q; want %q", tc.acceptLang, tc.queryLang, got, tc.expected)
			}
		})
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

	t.Run("Indonesian Language Rendering with Custom AppName", func(t *testing.T) {
		html, err := RenderVerifyDocument(validBA, "id", "Bank BNI Timesheet Portal")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		expectedSubstrings := []string{
			"Bank BNI Timesheet Portal",
			"DOKUMEN ASLI &amp; TERVERIFIKASI",
			"Berita Acara Lembur",
			"Oktober 2026",
			"John Doe",
			"(12345)",
			"Jane Smith",
			"abcd1234efgh5678",
			"Informasi Dokumen",
			"Jenis Dokumen",
			"Hak Cipta Dilindungi",
			"?lang=en",
		}
		for _, s := range expectedSubstrings {
			if !strings.Contains(html, s) {
				t.Errorf("expected HTML to contain %q, but got:\n%s", s, html)
			}
		}
	})

	t.Run("English Language Rendering with Custom AppName", func(t *testing.T) {
		html, err := RenderVerifyDocument(validBA, "en", "Enterprise Time Tracker")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		expectedSubstrings := []string{
			"Enterprise Time Tracker",
			"AUTHENTIC &amp; VERIFIED DOCUMENT",
			"Overtime Incident Report (Berita Acara)",
			"October 2026",
			"John Doe",
			"Jane Smith",
			"Document Information",
			"Document Type",
			"Employee Signature",
			"All Rights Reserved",
			"?lang=id",
		}
		for _, s := range expectedSubstrings {
			if !strings.Contains(html, s) {
				t.Errorf("expected HTML to contain %q, but got:\n%s", s, html)
			}
		}
	})

	t.Run("English Revoked Timesheet", func(t *testing.T) {
		html, err := RenderVerifyDocument(revokedTS, "en", "Timesheet System")
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
		html, err := RenderVerifyDocument(nil, "id", "")
		if err != nil {
			t.Fatalf("RenderVerifyDocument failed: %v", err)
		}

		if !strings.Contains(html, "DOKUMEN TIDAK VALID") {
			t.Errorf("expected HTML to contain 'DOKUMEN TIDAK VALID', got:\n%s", html)
		}
		if !strings.Contains(html, "Env Custom System") {
			t.Errorf("expected HTML to contain 'Env Custom System', got:\n%s", html)
		}
	})
}
