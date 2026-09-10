package config

import (
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
