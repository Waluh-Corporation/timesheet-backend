package auth

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"unicode/utf8"
)

// Password policy constants aligned with NIST SP 800-63B (memorized secrets).
const (
	// PasswordMinLength is the NIST-recommended minimum length for a
	// user-chosen memorized secret (§5.1.1.2: at least 8 characters).
	PasswordMinLength = 8
	// PasswordMaxLength is the maximum a verifier must accept. NIST requires
	// accepting secrets of at least 64 characters; we cap there as a sane upper
	// bound (Argon2id itself imposes no practical length limit).
	PasswordMaxLength = 64
	// GeneratedPasswordLength is the length of an auto-generated default
	// password. 20 characters from a 64-symbol alphabet is ~120 bits of
	// entropy — comfortably above any brute-force concern.
	GeneratedPasswordLength = 20
)

// ErrPasswordTooShort / ErrPasswordTooLong / ErrPasswordBlocked describe why a
// candidate secret was rejected. They are returned by ValidatePassword.
var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 64 characters")
	ErrPasswordBlocked  = errors.New("password is too common or easily guessed; choose another")
)

// blockedPasswords is a compact blocklist of values that must never be accepted
// as a memorized secret. NIST SP 800-63B §5.1.1.2 requires verifiers to compare
// prospective secrets against a list of commonly-used, expected, or compromised
// values (top breach-corpus passwords, dictionary words, repetitive/sequential
// strings, and context-specific terms). A production deployment should back
// this with a full breached-password service (e.g. HaveIBeenPwned k-anonymity);
// this in-process list covers the most probable guesses without a network call.
var blockedPasswords = map[string]bool{
	"password":      true,
	"password1":     true,
	"password123":   true,
	"passw0rd":      true,
	"12345678":      true,
	"123456789":     true,
	"1234567890":    true,
	"qwerty":        true,
	"qwertyuiop":    true,
	"qwerty123":     true,
	"111111":        true,
	"11111111":      true,
	"00000000":      true,
	"abc123":        true,
	"abcdefgh":      true,
	"iloveyou":      true,
	"admin":         true,
	"admin123":      true,
	"administrator": true,
	"root":          true,
	"letmein":       true,
	"welcome":       true,
	"welcome1":      true,
	"changeme":      true,
	"secret":        true,
	"timesheet":     true,
	"timesheet123":  true,
	"portal":        true,
}

// ValidatePassword enforces the NIST SP 800-63B memorized-secret rules on a
// candidate password. Following NIST guidance it does NOT impose composition
// rules (required mixes of upper/lower/digit/symbol) or arbitrary complexity —
// only a length floor and ceiling and a blocklist check against common,
// expected, or context-specific values.
//
// The variadic context arguments (e.g. the username and email) are treated as
// context-specific words: a password equal to, or trivially derived from, any
// of them is rejected.
func ValidatePassword(password string, context ...string) error {
	// Length is measured in Unicode code points, and all printable characters
	// (including spaces and emoji) are accepted, per NIST §5.1.1.2.
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

	// Context-specific check: reject the password if it matches, contains, or is
	// contained by any provided context value (username/email local-part/etc.).
	for _, ctx := range context {
		ctx = strings.ToLower(strings.TrimSpace(ctx))
		if ctx == "" {
			continue
		}
		// Compare against the email local-part too, not just the whole address.
		if at := strings.IndexByte(ctx, '@'); at > 0 {
			ctx = ctx[:at]
		}
		if len(ctx) >= 4 && (lower == ctx || strings.Contains(lower, ctx) || strings.Contains(ctx, lower)) {
			return ErrPasswordBlocked
		}
	}

	return nil
}

// generationAlphabet is a 64-character URL-safe alphabet. Its length is a power
// of two, so modulo-free unbiased selection is trivial with crypto/rand.
const generationAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// GeneratePassword returns a cryptographically-random password of the requested
// length drawn from a 64-symbol URL-safe alphabet. The result always satisfies
// ValidatePassword for lengths within the policy bounds.
func GeneratePassword(length int) (string, error) {
	if length < PasswordMinLength {
		length = PasswordMinLength
	}
	if length > PasswordMaxLength {
		length = PasswordMaxLength
	}
	max := big.NewInt(int64(len(generationAlphabet)))
	b := make([]byte, length)
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = generationAlphabet[idx.Int64()]
	}
	return string(b), nil
}
