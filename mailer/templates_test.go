package mailer

import (
	"strings"
	"testing"
)

func TestRenderSetupEmail(t *testing.T) {
	data := SetupEmailData{
		AppName:      "Timesheet Test",
		Username:     "johndoe",
		Email:        "john@example.com",
		SetupURL:     "https://timesheet.example.com/setup?token=xyz",
		ExpireDays:   7,
		SupportEmail: "support@example.com",
		PortalURL:    "https://timesheet.example.com",
	}

	html, text, err := RenderSetupEmail(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key elements in HTML
	if !strings.Contains(html, "johndoe") {
		t.Errorf("HTML should contain username johndoe")
	}
	if !strings.Contains(html, "https://timesheet.example.com/setup?token=xyz") {
		t.Errorf("HTML should contain setup URL")
	}
	if !strings.Contains(html, "Masuk ke Portal") {
		t.Errorf("HTML should contain button 'Masuk ke Portal'")
	}
	if !strings.Contains(html, "Passkey") {
		t.Errorf("HTML should contain Passkey notice")
	}
	if strings.Contains(html, "Status Akun") {
		t.Errorf("HTML should NOT contain 'Status Akun'")
	}
	if strings.Contains(text, "Status Akun") {
		t.Errorf("text should NOT contain 'Status Akun'")
	}
	if !strings.Contains(html, "Akun Baru") {
		t.Errorf("HTML should contain badge 'Akun Baru'")
	}
	if !strings.Contains(html, "color-scheme") {
		t.Errorf("HTML should contain color-scheme meta tag")
	}
	if !strings.Contains(html, "&zwnj;&nbsp;") {
		t.Errorf("HTML should contain preheader whitespace padding")
	}

	// Verify plain text
	if !strings.Contains(text, "johndoe") || !strings.Contains(text, "https://timesheet.example.com/setup?token=xyz") {
		t.Errorf("text should contain username and setup URL")
	}
}

func TestRenderResetEmail(t *testing.T) {
	data := ResetEmailData{
		AppName:       "Timesheet Test",
		Username:      "alexander",
		ResetURL:      "https://timesheet.example.com/reset?token=abc",
		ExpireMinutes: 60,
		RequestedAt:   "13 Sep 2026, 08:30 WIB",
		ExpiresAt:     "13 Sep 2026, 09:30 WIB",
		PortalURL:     "https://timesheet.example.com",
	}

	html, text, err := RenderResetEmail(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "alexander") {
		t.Errorf("HTML should contain username")
	}
	if !strings.Contains(html, "13 Sep 2026, 09:30 WIB") {
		t.Errorf("HTML should contain expiration timestamp")
	}
	if strings.Contains(html, "Berlaku Hingga") {
		t.Errorf("HTML should NOT contain 'Berlaku Hingga' in details box")
	}
	if !strings.Contains(html, "Peringatan Keamanan") {
		t.Errorf("HTML should contain security warning")
	}
	if !strings.Contains(text, "https://timesheet.example.com/reset?token=abc") {
		t.Errorf("text should contain reset URL")
	}

	// Test with IP address
	dataWithIP := ResetEmailData{
		AppName:       "Timesheet Test",
		Username:      "alexander",
		ResetURL:      "https://timesheet.example.com/reset?token=abc",
		ExpireMinutes: 60,
		RequestedAt:   "13 Sep 2026, 08:30 WIB",
		ExpiresAt:     "13 Sep 2026, 09:30 WIB",
		PortalURL:     "https://timesheet.example.com",
		IPAddress:     "192.168.1.100",
		Location:      "Jakarta, Indonesia",
	}
	htmlWithIP, textWithIP, err := RenderResetEmail(dataWithIP)
	if err != nil {
		t.Fatalf("unexpected error rendering reset email with IP: %v", err)
	}
	if !strings.Contains(htmlWithIP, "Alamat IP") || !strings.Contains(htmlWithIP, "192.168.1.100") {
		t.Errorf("HTML should contain Alamat IP when provided")
	}
	if !strings.Contains(htmlWithIP, "Lokasi") || !strings.Contains(htmlWithIP, "Jakarta, Indonesia") {
		t.Errorf("HTML should contain Lokasi row when provided")
	}
	if !strings.Contains(textWithIP, "192.168.1.100") {
		t.Errorf("text should contain IP address")
	}
	if !strings.Contains(textWithIP, "Lokasi") || !strings.Contains(textWithIP, "Jakarta, Indonesia") {
		t.Errorf("text should contain Lokasi line when provided")
	}
}

func TestRenderTimesheetEmail(t *testing.T) {
	data := TimesheetEmailData{
		AppName:   "Timesheet Test",
		Username:  "budi",
		Company:   "PT Mitra Integrasi Informatika",
		Period:    "September 2026",
		Filename:  "Timesheet_budi_09_2026.xlsx",
		PortalURL: "https://timesheet.example.com",
	}

	html, text, err := RenderTimesheetEmail(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "PT Mitra Integrasi Informatika") {
		t.Errorf("HTML should contain company name")
	}
	if !strings.Contains(html, "September 2026") {
		t.Errorf("HTML should contain period")
	}
	if !strings.Contains(html, "Timesheet_budi_09_2026.xlsx") {
		t.Errorf("HTML should contain filename")
	}
	if !strings.Contains(html, "Langkah Selanjutnya") {
		t.Errorf("HTML should contain next steps box")
	}
	if !strings.Contains(text, "Timesheet_budi_09_2026.xlsx") {
		t.Errorf("text should contain filename")
	}

	// Test with secure download URL
	dataWithURL := TimesheetEmailData{
		AppName:     "Timesheet Test",
		Username:    "budi",
		Company:     "PT Mitra Integrasi Informatika",
		Period:      "September 2026",
		Filename:    "Timesheet_budi_09_2026.xlsx",
		PortalURL:   "https://timesheet.example.com",
		DownloadURL: "https://timesheet.example.com/api/v1/timesheet/downloads/token123",
	}
	htmlWithURL, textWithURL, err := RenderTimesheetEmail(dataWithURL)
	if err != nil {
		t.Fatalf("unexpected error rendering timesheet email with download URL: %v", err)
	}
	if !strings.Contains(htmlWithURL, "Download Timesheet (.xlsx)") {
		t.Errorf("HTML should contain download button")
	}
	if !strings.Contains(textWithURL, "https://timesheet.example.com/api/v1/timesheet/downloads/token123") {
		t.Errorf("text should contain download URL")
	}
}

func TestRenderReminderEmail(t *testing.T) {
	data := ReminderEmailData{
		AppName:     "Timesheet Test",
		Username:    "citra",
		Date:        "13 September 2026",
		ActivityURL: "https://timesheet.example.com/activity",
		PortalURL:   "https://timesheet.example.com",
	}

	html, text, err := RenderReminderEmail(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "citra") {
		t.Errorf("HTML should contain username")
	}
	if !strings.Contains(html, "13 September 2026") {
		t.Errorf("HTML should contain date")
	}
	if !strings.Contains(html, "https://timesheet.example.com/activity") {
		t.Errorf("HTML should contain activity URL")
	}
	if !strings.Contains(text, "https://timesheet.example.com/activity") {
		t.Errorf("text should contain activity URL")
	}
}

func TestRenderPasswordChangedEmail(t *testing.T) {
	data := PasswordChangedEmailData{
		AppName:   "Timesheet Test",
		Username:  "deni",
		ChangedAt: "13 Sep 2026, 09:00 WIB",
		LoginURL:  "https://timesheet.example.com/login",
		PortalURL: "https://timesheet.example.com",
	}

	html, text, err := RenderPasswordChangedEmail(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "deni") {
		t.Errorf("HTML should contain username")
	}
	if !strings.Contains(html, "13 Sep 2026, 09:00 WIB") {
		t.Errorf("HTML should contain changed timestamp")
	}
	if strings.Contains(html, "Status Keamanan") {
		t.Errorf("HTML should NOT contain 'Status Keamanan'")
	}
	if !strings.Contains(html, "Bukan Anda yang Melakukan Perubahan Ini?") {
		t.Errorf("HTML should contain security alert")
	}
	if !strings.Contains(text, "https://timesheet.example.com/login") {
		t.Errorf("text should contain login URL")
	}

	// Test with IP address
	dataWithIP := PasswordChangedEmailData{
		AppName:   "Timesheet Test",
		Username:  "deni",
		ChangedAt: "13 Sep 2026, 09:00 WIB",
		LoginURL:  "https://timesheet.example.com/login",
		PortalURL: "https://timesheet.example.com",
		IPAddress: "203.0.113.195",
		Location:  "Jakarta, Indonesia",
	}
	htmlWithIP, textWithIP, err := RenderPasswordChangedEmail(dataWithIP)
	if err != nil {
		t.Fatalf("unexpected error rendering password changed email with IP: %v", err)
	}
	if !strings.Contains(htmlWithIP, "Alamat IP") || !strings.Contains(htmlWithIP, "203.0.113.195") {
		t.Errorf("HTML should contain Alamat IP when provided")
	}
	if !strings.Contains(htmlWithIP, "Lokasi") || !strings.Contains(htmlWithIP, "Jakarta, Indonesia") {
		t.Errorf("HTML should contain Lokasi row when provided")
	}
	if !strings.Contains(textWithIP, "203.0.113.195") {
		t.Errorf("text should contain IP address")
	}
	if !strings.Contains(textWithIP, "Lokasi") || !strings.Contains(textWithIP, "Jakarta, Indonesia") {
		t.Errorf("text should contain Lokasi line when provided")
	}
}

func TestHTMLEscapingSafety(t *testing.T) {
	data := SetupEmailData{
		AppName:      "Timesheet Safe",
		Username:     `<script>alert("xss")</script>`,
		SetupURL:     "https://example.com/setup",
		SupportEmail: "admin@example.com",
	}

	html, _, err := RenderSetupEmail(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(html, `<script>alert("xss")</script>`) {
		t.Errorf("HTML injection detected! Script tag was not escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;") {
		t.Errorf("Expected escaped HTML for dangerous input")
	}
}

func TestFormatMonthYearIndonesian(t *testing.T) {
	cases := []struct {
		m, y int
		want string
	}{
		{1, 2026, "Januari 2026"},
		{9, 2026, "September 2026"},
		{12, 2025, "Desember 2025"},
		{0, 2026, "Bulan 00 2026"},
		{13, 2026, "Bulan 13 2026"},
	}

	for _, tc := range cases {
		got := FormatMonthYearIndonesian(tc.m, tc.y)
		if got != tc.want {
			t.Errorf("FormatMonthYearIndonesian(%d, %d) = %q, want %q", tc.m, tc.y, got, tc.want)
		}
	}
}

func TestParsePeriodFromFilename(t *testing.T) {
	cases := []struct {
		filename     string
		wantPeriod   string
		wantUsername string
	}{
		{"Timesheet_john_09_2026.xlsx", "September 2026", "john"},
		{"Timesheet_john_doe_01_2025.xlsx", "Januari 2025", "john_doe"},
		{"random_file.pdf", "", ""},
	}

	for _, tc := range cases {
		period, user := ParsePeriodFromFilename(tc.filename)
		if period != tc.wantPeriod || user != tc.wantUsername {
			t.Errorf("ParsePeriodFromFilename(%q) = (%q, %q), want (%q, %q)", tc.filename, period, user, tc.wantPeriod, tc.wantUsername)
		}
	}
}
