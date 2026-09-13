package auth

import (
	"strings"
	"testing"
	"unicode"
)

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		context  []string
		wantErr  error
	}{
		{"tooShort", "short7!", nil, ErrPasswordTooShort},
		{"minLengthOK", "Abcd1234#efgh", nil, nil},
		{"tooLong", repeat("aA1!", 17), nil, ErrPasswordTooLong},
		{"commonBlocked", "password", nil, ErrPasswordBlocked},
		{"commonBlockedCase", "Password123", nil, ErrPasswordBlocked},
		{"adminBlocked", "admin123", nil, ErrPasswordBlocked},
		{"repetitiveWeak", "11111111", nil, ErrPasswordBlocked},
		{"allNumericWeak", "98765432", nil, ErrPasswordTooWeak},
		{"missingUpper", "lowercase123!", nil, ErrPasswordTooWeak},
		{"missingLower", "UPPERCASE123!", nil, ErrPasswordTooWeak},
		{"missingDigit", "UpperLower!#@$", nil, ErrPasswordTooWeak},
		{"missingSymbol", "UpperLower1234", nil, ErrPasswordTooWeak},
		{"contextUsername", "alicewonderland123!", []string{"alice"}, ErrPasswordBlocked},
		{"contextEmailLocalPart", "johndoexyz456!", []string{"johndoe@example.com"}, ErrPasswordBlocked},
		{"contextEmpty", "Abcd1234#efgh", []string{""}, nil},
		{"contextShortIgnored", "Abcd1234#efgh", []string{"ab"}, nil},
		{"strongPassphrase", "Correct-Horse-Battery-Staple-99!", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidatePassword(tc.password, tc.context...)
			if (tc.wantErr == nil && got != nil) || (tc.wantErr != nil && got == nil) {
				t.Fatalf("ValidatePassword(%q) = %v, want %v", tc.password, got, tc.wantErr)
			}
		})
	}
}

func TestGenerateSecurePassword(t *testing.T) {
	for i := 0; i < 50; i++ {
		pw, err := GenerateSecurePassword(16)
		if err != nil {
			t.Fatalf("GenerateSecurePassword error: %v", err)
		}
		if len(pw) != 16 {
			t.Fatalf("length = %d, want 16", len(pw))
		}

		var hasUpper, hasLower, hasDigit, hasSymbol bool
		for _, r := range pw {
			switch {
			case unicode.IsUpper(r):
				hasUpper = true
			case unicode.IsLower(r):
				hasLower = true
			case unicode.IsDigit(r):
				hasDigit = true
			case strings.ContainsRune(SymbolChars, r):
				hasSymbol = true
			}
		}

		if !hasUpper || !hasLower || !hasDigit || !hasSymbol {
			t.Fatalf("password %q does not contain all required character types (upper: %v, lower: %v, digit: %v, symbol: %v)",
				pw, hasUpper, hasLower, hasDigit, hasSymbol)
		}

		if err := ValidatePassword(pw); err != nil {
			t.Fatalf("generated password %q failed policy: %v", pw, err)
		}
	}

	// Boundary length tests
	shortPW, err := GenerateSecurePassword(5)
	if err != nil || len(shortPW) != DefaultGeneratedLength {
		t.Fatalf("expected length %d for short input, got %d (err: %v)", DefaultGeneratedLength, len(shortPW), err)
	}

	longPW, err := GenerateSecurePassword(100)
	if err != nil || len(longPW) != PasswordMaxLength {
		t.Fatalf("expected length %d for overlong input, got %d (err: %v)", PasswordMaxLength, len(longPW), err)
	}

	aliasPW, err := GeneratePassword(16)
	if err != nil || len(aliasPW) != 16 {
		t.Fatalf("expected GeneratePassword alias to return 16 chars, got %d (err: %v)", len(aliasPW), err)
	}
}

func TestIsRepetitiveEmpty(t *testing.T) {
	if isRepetitive("") {
		t.Fatal("isRepetitive(\"\") must return false")
	}
	if !isRepetitive("aaaa") {
		t.Fatal("isRepetitive(\"aaaa\") must return true")
	}
	if isRepetitive("abaa") {
		t.Fatal("isRepetitive(\"abaa\") must return false")
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
