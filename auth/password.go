package auth

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Password policy constants aligned with NIST SP 800-63B.
const (
	PasswordMinLength       = 8
	PasswordMaxLength       = 64
	DefaultGeneratedLength  = 16
	GeneratedPasswordLength = 16
)

// Sentinel errors returned by ValidatePassword and password generators.
var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 64 characters")
	ErrPasswordBlocked  = errors.New("password is too common or easily guessed; choose another")
	ErrPasswordTooWeak  = errors.New("password is too weak; must contain a combination of character types and avoid simple patterns")
)

// blockedPasswords holds commonly compromised, trivial, or predictable passwords.
var blockedPasswords = map[string]bool{
	"password":        true,
	"password1":       true,
	"password123":     true,
	"password1234":    true,
	"passw0rd":        true,
	"12345678":        true,
	"123456789":       true,
	"1234567890":      true,
	"qwerty":          true,
	"qwerty1":         true,
	"qwerty123":       true,
	"qwertyuiop":      true,
	"11111111":        true,
	"00000000":        true,
	"abc123":          true,
	"abcdefgh":        true,
	"admin":           true,
	"admin123":        true,
	"admin1234":       true,
	"administrator":   true,
	"root":            true,
	"root123":         true,
	"letmein":         true,
	"welcome":         true,
	"welcome1":        true,
	"changeme":        true,
	"secret":          true,
	"timesheet":       true,
	"timesheet123":    true,
	"timesheetportal": true,
	"portal":          true,
	"portal123":       true,
}

// Character sets for cryptographically-secure random password generation.
const (
	UpperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	LowerChars  = "abcdefghijklmnopqrstuvwxyz"
	DigitChars  = "0123456789"
	SymbolChars = "!@#$%^&*()-_=+[]{}|;:,.<>?"
	AllChars    = UpperChars + LowerChars + DigitChars + SymbolChars
)

// ValidatePassword checks whether a candidate password adheres to complexity and security rules:
// 1. Minimum 8 characters, maximum 64 characters.
// 2. Not in the blocklist of common or compromised passwords.
// 3. Not purely repetitive characters (e.g. 'aaaaaaaa').
// 4. Contains reasonable character variety (avoids pure single-class passwords like all digits).
// 5. Does not contain or derive from context strings (username, email prefix, name).
func ValidatePassword(password string, context ...string) error {
	n := utf8.RuneCountInString(password)
	if n < PasswordMinLength {
		return ErrPasswordTooShort
	}
	if n > PasswordMaxLength {
		return ErrPasswordTooLong
	}

	lower := strings.ToLower(strings.TrimSpace(password))
	if blockedPasswords[lower] {
		return ErrPasswordBlocked
	}

	// Reject repetitive strings (e.g. '11111111', 'aaaaaaaa')
	if isRepetitive(password) {
		return ErrPasswordTooWeak
	}

	// Must be combination of uppercase, lowercase, number, and symbol/special character
	if !hasRequiredCharacterClasses(password) {
		return ErrPasswordTooWeak
	}

	// Context check: reject passwords matching or containing user identifiers
	if containsContextIdentifier(lower, context) {
		return ErrPasswordBlocked
	}

	return nil
}

func hasRequiredCharacterClasses(password string) bool {
	var hasDigit, hasUpper, hasLower, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSymbol
}

func containsContextIdentifier(lowerPassword string, context []string) bool {
	for _, ctx := range context {
		ctx = strings.ToLower(strings.TrimSpace(ctx))
		if ctx == "" {
			continue
		}
		if at := strings.IndexByte(ctx, '@'); at > 0 {
			ctx = ctx[:at]
		}
		if len(ctx) >= 4 && (lowerPassword == ctx || strings.Contains(lowerPassword, ctx) || strings.Contains(ctx, lowerPassword)) {
			return true
		}
	}
	return false
}

// isRepetitive returns true if all characters in the string are identical.
func isRepetitive(s string) bool {
	runes := []rune(s)
	if len(runes) == 0 {
		return false
	}
	first := runes[0]
	for _, r := range runes[1:] {
		if r != first {
			return false
		}
	}
	return true
}

// GenerateSecurePassword generates a cryptographically-secure random password using crypto/rand.
// It guarantees a mix of uppercase letters, lowercase letters, digits, and symbols,
// and enforces a length between 12 and 64 characters (defaulting to 16).
func GenerateSecurePassword(length int) (string, error) {
	if length < 12 {
		length = DefaultGeneratedLength
	}
	if length > PasswordMaxLength {
		length = PasswordMaxLength
	}

	// Guarantee at least one character of each category
	buf := make([]byte, length)
	categories := []string{UpperChars, LowerChars, DigitChars, SymbolChars}
	for i, cat := range categories {
		idx, err := cryptoRandInt(len(cat))
		if err != nil {
			return "", err
		}
		buf[i] = cat[idx]
	}

	// Fill remainder from combined character set
	for i := len(categories); i < length; i++ {
		idx, err := cryptoRandInt(len(AllChars))
		if err != nil {
			return "", err
		}
		buf[i] = AllChars[idx]
	}

	// Fisher-Yates shuffle with crypto/rand
	for i := length - 1; i > 0; i-- {
		j, err := cryptoRandInt(i + 1)
		if err != nil {
			return "", err
		}
		buf[i], buf[j] = buf[j], buf[i]
	}

	res := string(buf)
	// Safety check: ensure generated password satisfies ValidatePassword
	if err := ValidatePassword(res); err != nil {
		// Retry once if edge collision occurred
		return GenerateSecurePassword(length)
	}
	return res, nil
}

// GeneratePassword is an alias for GenerateSecurePassword to preserve backward compatibility.
func GeneratePassword(length int) (string, error) {
	return GenerateSecurePassword(length)
}

func cryptoRandInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
