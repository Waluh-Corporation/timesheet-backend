package services

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/ip2location/ip2location-go/v9"
)

// GeoLocation holds resolved geographic information for an IP address.
type GeoLocation struct {
	IP           string
	CountryShort string
	CountryLong  string
	Region       string
	City         string
	ZipCode      string
	TimeZone     string
}

// FormatLocation returns a human-readable location string, e.g. "Jakarta, Indonesia" or "Indonesia".
func (g GeoLocation) FormatLocation() string {
	parts := make([]string, 0, 2)
	if g.City != "" && g.City != "-" {
		parts = append(parts, g.City)
	} else if g.Region != "" && g.Region != "-" {
		parts = append(parts, g.Region)
	}
	if g.CountryLong != "" && g.CountryLong != "-" {
		parts = append(parts, g.CountryLong)
	} else if g.CountryShort != "" && g.CountryShort != "-" {
		parts = append(parts, g.CountryShort)
	}
	return strings.Join(parts, ", ")
}

// FormatIPWithLocation returns "IP (Location)" or simply "IP" if no location is available.
func (g GeoLocation) FormatIPWithLocation() string {
	loc := g.FormatLocation()
	if loc == "" {
		return g.IP
	}
	if g.IP == "" {
		return loc
	}
	return fmt.Sprintf("%s (%s)", g.IP, loc)
}

// GeoIPService provides IP lookup using the IP2Location binary database.
type GeoIPService struct {
	db *ip2location.DB
	mu sync.RWMutex
}

// NewGeoIPService initializes a GeoIPService. If dbPath is empty or the file does not exist,
// it gracefully returns a fallback service that returns basic IP records without crashing.
func NewGeoIPService(dbPath string) (*GeoIPService, error) {
	trimmed := strings.TrimSpace(dbPath)
	if trimmed == "" {
		slog.Debug("GeoIP: IP2Location database path not configured; using passthrough lookup")
		return &GeoIPService{}, nil
	}

	if _, err := os.Stat(trimmed); os.IsNotExist(err) {
		slog.Warn("GeoIP: IP2Location database file not found; using passthrough lookup", "path", trimmed)
		return &GeoIPService{}, nil
	}

	db, err := ip2location.OpenDB(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to open IP2Location database: %w", err)
	}

	slog.Info("GeoIP: IP2Location database successfully loaded", "path", trimmed)
	return &GeoIPService{db: db}, nil
}

// Close closes the underlying database file if open.
func (s *GeoIPService) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// Lookup queries the IP2Location database for geolocation metadata.
func (s *GeoIPService) Lookup(ipStr string) GeoLocation {
	trimmed := strings.TrimSpace(ipStr)
	result := GeoLocation{IP: trimmed}
	if trimmed == "" {
		return result
	}

	parsed := net.ParseIP(trimmed)
	if parsed != nil && (parsed.IsLoopback() || parsed.IsPrivate()) {
		result.City = "Jaringan Lokal"
		result.CountryLong = "Private Network"
		return result
	}

	if s == nil {
		return result
	}

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return result
	}

	rec, err := db.Get_all(trimmed)
	if err != nil {
		return result
	}

	clean := func(val string) string {
		v := strings.TrimSpace(val)
		if v == "?" || v == "-" || strings.EqualFold(v, "invalid ip address") || strings.EqualFold(v, "this parameter is unavailable for selected data file.") {
			return ""
		}
		return v
	}

	result.CountryShort = clean(rec.Country_short)
	result.CountryLong = clean(rec.Country_long)
	result.Region = clean(rec.Region)
	result.City = clean(rec.City)
	result.ZipCode = clean(rec.Zipcode)
	result.TimeZone = clean(rec.Timezone)
	return result
}
