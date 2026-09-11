package config

import (
	"reflect"
	"testing"
	"time"
)

func TestParseOrigins(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single", "https://a.com", []string{"https://a.com"}},
		{"commaSeparated", "https://a.com,https://b.org", []string{"https://a.com", "https://b.org"}},
		{"whitespaceAndTrailingSlash", " https://a.com/ ,\thttps://b.org/ ", []string{"https://a.com", "https://b.org"}},
		{"dedup", "https://a.com,https://a.com", []string{"https://a.com"}},
		{"empty", "", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseOrigins(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseOrigins(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	cases := []struct {
		envKey   string
		envVal   string
		setEnv   bool
		fallback bool
		want     bool
	}{
		{"TEST_BOOL_1", "true", true, false, true},
		{"TEST_BOOL_2", "1", true, false, true},
		{"TEST_BOOL_3", "false", true, true, false},
		{"TEST_BOOL_4", "0", true, true, false},
		{"TEST_BOOL_5", "invalid", true, true, true},
		{"TEST_BOOL_6", "", false, false, false},
		{"TEST_BOOL_7", "", false, true, true},
	}

	for _, tc := range cases {
		if tc.setEnv {
			t.Setenv(tc.envKey, tc.envVal)
		}
		got := getEnvBool(tc.envKey, tc.fallback)
		if got != tc.want {
			t.Errorf("getEnvBool(%q, %v) with val %q = %v; want %v", tc.envKey, tc.fallback, tc.envVal, got, tc.want)
		}
	}
}

func TestGetEnv_And_GetEnvInt(t *testing.T) {
	// getEnv fallback
	if val := getEnv("NON_EXISTENT_KEY_12345", "fallback_val"); val != "fallback_val" {
		t.Errorf("expected fallback_val, got %s", val)
	}
	// getEnv existing
	t.Setenv("EXISTENT_KEY_12345", "actual_val")
	if val := getEnv("EXISTENT_KEY_12345", "fallback_val"); val != "actual_val" {
		t.Errorf("expected actual_val, got %s", val)
	}

	// getEnvInt fallback
	if val := getEnvInt("NON_EXISTENT_INT_12345", 42); val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
	// getEnvInt invalid int
	t.Setenv("INVALID_INT_12345", "not_a_number")
	if val := getEnvInt("INVALID_INT_12345", 42); val != 42 {
		t.Errorf("expected 42 on invalid int, got %d", val)
	}
	// getEnvInt valid int
	t.Setenv("VALID_INT_12345", "99")
	if val := getEnvInt("VALID_INT_12345", 42); val != 99 {
		t.Errorf("expected 99, got %d", val)
	}
}

func TestSanitizeRPID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://example.com", "example.com"},
		{"http://localhost:8080", "localhost"},
		{"example.com/path", "example.com"},
		{"example.com\\trailing", "example.com"},
		{"sub.domain.org?query=1", "sub.domain.org"},
		{"  https://portal.timesheet.internal:3000/api  ", "portal.timesheet.internal"},
	}

	for _, tt := range tests {
		got := sanitizeRPID(tt.in)
		if got != tt.want {
			t.Errorf("sanitizeRPID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLoad_And_ValidateSecrets(t *testing.T) {
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("JWT_SECRET", "changeme") // weak secret in debug mode should generate ephemeral
	t.Setenv("PORT", "9999")
	t.Setenv("JWT_EXPIRY_HOURS", "48")
	t.Setenv("RESET_TOKEN_TTL_MINUTES", "30")

	cfg := Load()
	if cfg.Port != "9999" {
		t.Errorf("expected port 9999, got %s", cfg.Port)
	}
	if cfg.JWTExpiry != 48*time.Hour {
		t.Errorf("expected 48h, got %v", cfg.JWTExpiry)
	}
	if cfg.ResetTokenTTL != 30*time.Minute {
		t.Errorf("expected 30m, got %v", cfg.ResetTokenTTL)
	}
	if cfg.JWTSecret == "changeme" || len(cfg.JWTSecret) < 32 {
		t.Errorf("expected generated strong ephemeral secret, got %s", cfg.JWTSecret)
	}

	// Strong secret preserves value
	t.Setenv("JWT_SECRET", "this-is-a-very-strong-and-secure-custom-secret-key-12345")
	cfgStrong := Load()
	if cfgStrong.JWTSecret != "this-is-a-very-strong-and-secure-custom-secret-key-12345" {
		t.Errorf("expected custom secret preserved, got %s", cfgStrong.JWTSecret)
	}
}
