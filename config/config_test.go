package config

import (
	"os"
	"reflect"
	"testing"
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

func TestSanitizeRPID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://example.com:8080/path", "example.com"},
		{"http://sub.domain.org/", "sub.domain.org"},
		{"  localhost:3000\\ ", "localhost"},
		{"bare-domain.com", "bare-domain.com"},
	}

	for _, tc := range cases {
		got := sanitizeRPID(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeRPID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGetEnvAndGetEnvInt(t *testing.T) {
	t.Setenv("TEST_STR_KEY", "custom_val")
	if v := getEnv("TEST_STR_KEY", "fallback"); v != "custom_val" {
		t.Errorf("expected custom_val, got %s", v)
	}
	if v := getEnv("UNSET_KEY_XYZ", "fallback"); v != "fallback" {
		t.Errorf("expected fallback, got %s", v)
	}

	t.Setenv("TEST_INT_KEY", "42")
	if v := getEnvInt("TEST_INT_KEY", 10); v != 42 {
		t.Errorf("expected 42, got %d", v)
	}
	t.Setenv("TEST_INT_INVALID", "not_a_number")
	if v := getEnvInt("TEST_INT_INVALID", 10); v != 10 {
		t.Errorf("expected fallback 10 for invalid int, got %d", v)
	}
}

func TestConfigLoad(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("WEBAUTHN_RP_ID", "test.example.com")
	t.Setenv("JWT_SECRET", "this-is-a-strong-custom-secret-key-32b")
	cfg := Load()

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Port != "9999" {
		t.Errorf("expected port 9999, got %s", cfg.Port)
	}
	if cfg.RPID != "test.example.com" {
		t.Errorf("expected RPID test.example.com, got %s", cfg.RPID)
	}
	if cfg.JWTSecret != "this-is-a-strong-custom-secret-key-32b" {
		t.Errorf("expected custom JWTSecret, got %s", cfg.JWTSecret)
	}
}

func TestLoadDotEnv(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_dotenv_*.env")
	if err != nil {
		t.Fatalf("create temp error: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	content := `# Comment line
TEST_DOTENV_KEY=sample_val
TEST_DOTENV_QUOTED="quoted_val"
EMPTY_KEY=
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write temp error: %v", err)
	}
	_ = tmpFile.Close()

	loadDotEnv(tmpFile.Name())
	if v := os.Getenv("TEST_DOTENV_KEY"); v != "sample_val" {
		t.Errorf("expected sample_val, got %s", v)
	}
	if v := os.Getenv("TEST_DOTENV_QUOTED"); v != "quoted_val" {
		t.Errorf("expected quoted_val, got %s", v)
	}
}

func TestRandomSecretAndValidateSecrets(t *testing.T) {
	sec := randomSecret(16)
	if len(sec) != 32 {
		t.Errorf("expected 32 hex chars for 16 bytes, got %d", len(sec))
	}

	cfg := &Config{
		JWTSecret: "dev-secret-change-me",
	}
	t.Setenv("GIN_MODE", "debug")
	cfg.validateSecrets()
	if cfg.JWTSecret == "dev-secret-change-me" || len(cfg.JWTSecret) == 0 {
		t.Errorf("expected ephemeral secret to replace weak secret")
	}

	if isRelease() {
		t.Error("expected isRelease to be false in debug mode")
	}
	t.Setenv("GIN_MODE", "release")
	if !isRelease() {
		t.Error("expected isRelease to be true in release mode")
	}
}

func TestLoad_AppConfigEnv(t *testing.T) {
	t.Setenv("APP_NAME", "Custom Portal Name")
	t.Setenv("FRONTEND_URL", "https://timesheet.custom.org")
	t.Setenv("ADMIN_EMAIL", "superadmin@custom.org")

	cfg := Load()
	if cfg.AppName != "Custom Portal Name" {
		t.Errorf("expected AppName to be 'Custom Portal Name', got %q", cfg.AppName)
	}
	if cfg.FrontendURL != "https://timesheet.custom.org" {
		t.Errorf("expected FrontendURL to be 'https://timesheet.custom.org', got %q", cfg.FrontendURL)
	}
	if cfg.AdminEmail != "superadmin@custom.org" {
		t.Errorf("expected AdminEmail to be 'superadmin@custom.org', got %q", cfg.AdminEmail)
	}
}
