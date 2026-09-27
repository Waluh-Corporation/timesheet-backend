package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeoLocation_Formatting(t *testing.T) {
	tests := []struct {
		name     string
		geo      GeoLocation
		wantLoc  string
		wantFull string
	}{
		{
			name: "City and CountryLong present",
			geo: GeoLocation{
				IP:          "8.8.8.8",
				City:        "Jakarta",
				CountryLong: "Indonesia",
			},
			wantLoc:  "Jakarta, Indonesia",
			wantFull: "8.8.8.8 (Jakarta, Indonesia)",
		},
		{
			name: "Region and CountryLong present without city",
			geo: GeoLocation{
				IP:          "1.2.3.4",
				Region:      "DKI Jakarta",
				CountryLong: "Indonesia",
			},
			wantLoc:  "DKI Jakarta, Indonesia",
			wantFull: "1.2.3.4 (DKI Jakarta, Indonesia)",
		},
		{
			name: "CountryShort fallback when CountryLong empty",
			geo: GeoLocation{
				IP:           "1.2.3.4",
				CountryShort: "ID",
			},
			wantLoc:  "ID",
			wantFull: "1.2.3.4 (ID)",
		},
		{
			name: "Empty location returns only IP",
			geo: GeoLocation{
				IP: "192.168.1.1",
			},
			wantLoc:  "",
			wantFull: "192.168.1.1",
		},
		{
			name: "Ignores hyphens and question marks",
			geo: GeoLocation{
				IP:          "10.0.0.1",
				City:        "-",
				CountryLong: "-",
			},
			wantLoc:  "",
			wantFull: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.geo.FormatLocation(); got != tt.wantLoc {
				t.Errorf("FormatLocation() = %q, want %q", got, tt.wantLoc)
			}
			if got := tt.geo.FormatIPWithLocation(); got != tt.wantFull {
				t.Errorf("FormatIPWithLocation() = %q, want %q", got, tt.wantFull)
			}
		})
	}
}

func TestGeoIPService_FallbackWithoutDB(t *testing.T) {
	svc, err := NewGeoIPService("")
	if err != nil {
		t.Fatalf("unexpected error on empty path: %v", err)
	}
	defer svc.Close()

	// Public IP on unconfigured DB returns IP with empty location
	res := svc.Lookup("8.8.8.8")
	if res.IP != "8.8.8.8" {
		t.Errorf("expected IP 8.8.8.8, got %q", res.IP)
	}
	if res.FormatLocation() != "" {
		t.Errorf("expected empty location without DB, got %q", res.FormatLocation())
	}

	// Loopback / Private IP returns Jaringan Lokal
	loopbackRes := svc.Lookup("127.0.0.1")
	if !strings.Contains(loopbackRes.FormatLocation(), "Jaringan Lokal") {
		t.Errorf("expected Jaringan Lokal for loopback IP, got %q", loopbackRes.FormatLocation())
	}

	privateRes := svc.Lookup("192.168.1.50")
	if !strings.Contains(privateRes.FormatLocation(), "Jaringan Lokal") {
		t.Errorf("expected Jaringan Lokal for private IP, got %q", privateRes.FormatLocation())
	}
}

func TestGeoIPService_NonexistentPath(t *testing.T) {
	svc, err := NewGeoIPService("nonexistent_ip2location_db.bin")
	if err != nil {
		t.Fatalf("unexpected error on nonexistent path: %v", err)
	}
	defer svc.Close()

	res := svc.Lookup("1.1.1.1")
	if res.IP != "1.1.1.1" {
		t.Errorf("expected IP 1.1.1.1, got %q", res.IP)
	}
}

func TestGeoIPService_EdgeCases(t *testing.T) {
	// 1. Nil service receiver
	var nilSvc *GeoIPService
	nilSvc.Close()
	nilRes := nilSvc.Lookup("8.8.8.8")
	if nilRes.IP != "8.8.8.8" || nilRes.FormatLocation() != "" {
		t.Errorf("expected fallback on nil service, got %+v", nilRes)
	}

	// 2. Empty string IP
	svc, _ := NewGeoIPService("")
	emptyRes := svc.Lookup("")
	if emptyRes.IP != "" {
		t.Errorf("expected empty IP on empty input, got %q", emptyRes.IP)
	}

	// 3. Invalid IP string
	invalidRes := svc.Lookup("invalid.ip.string")
	if invalidRes.IP != "invalid.ip.string" || invalidRes.FormatLocation() != "" {
		t.Errorf("expected fallback on invalid IP, got %+v", invalidRes)
	}

	// 4. Repeated Close is safe
	svc.Close()
	svc.Close()

	// 5. Corrupt / invalid database file path returns error on OpenDB
	tmpDir := t.TempDir()
	corruptFile := filepath.Join(tmpDir, "corrupt.bin")
	_ = os.WriteFile(corruptFile, []byte("invalid-bin-content"), 0644)
	corruptSvc, err := NewGeoIPService(corruptFile)
	if err == nil {
		t.Errorf("expected error on corrupt database file, got svc=%+v", corruptSvc)
	}

	// 6. Test with real database file if present in workspace root
	candidates := []string{"../IP2LOCATION-LITE-DB3.BIN", "IP2LOCATION-LITE-DB3.BIN"}
	for _, cand := range candidates {
		if _, statErr := os.Stat(cand); statErr == nil {
			realSvc, realErr := NewGeoIPService(cand)
			if realErr == nil && realSvc != nil {
				defer realSvc.Close()
				r := realSvc.Lookup("8.8.8.8")
				if r.IP != "8.8.8.8" {
					t.Errorf("expected 8.8.8.8, got %q", r.IP)
				}
				_ = r.FormatLocation()
				_ = r.FormatIPWithLocation()
				break
			}
		}
	}
}
