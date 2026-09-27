package services

import (
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
