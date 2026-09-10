package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Argon2id parameters. These follow the OWASP Password Storage recommendations
// for argon2id (64 MiB memory, 3 iterations, parallelism 2) and produce a
// 32-byte key with a 16-byte random salt. They are encoded into every hash so
// stored credentials remain verifiable even if the defaults change later.
const (
	argon2Memory      uint32 = 64 * 1024 // 64 MiB
	argon2Iterations  uint32 = 3
	argon2Parallelism uint8  = 2
	argon2SaltLength  uint32 = 16
	argon2KeyLength   uint32 = 32
)

// ErrInvalidHash is returned when a stored hash string is malformed or uses an
// unsupported algorithm.
var ErrInvalidHash = errors.New("invalid or unsupported password hash")

// HashPassword hashes a plaintext password with Argon2id and returns a
// self-describing PHC-style string:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)

	b64 := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Iterations, argon2Parallelism,
		b64(salt), b64(key),
	), nil
}

// CheckPassword reports whether plain matches the stored hash. It supports
// Argon2id hashes produced by HashPassword and, for backward compatibility,
// legacy bcrypt hashes ($2a$/$2b$/$2y$) created before the migration — so
// existing accounts keep working until their next password change re-hashes
// them with Argon2id.
func CheckPassword(hash, plain string) bool {
	switch {
	case strings.HasPrefix(hash, "$argon2id$"):
		ok, err := verifyArgon2id(hash, plain)
		return err == nil && ok
	case strings.HasPrefix(hash, "$2a$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2y$"):
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
	default:
		return false
	}
}

// NeedsRehash reports whether a stored hash should be upgraded to the current
// Argon2id parameters on the next successful login — true for any legacy
// (non-argon2id) hash or an argon2id hash created with weaker parameters.
func NeedsRehash(hash string) bool {
	if !strings.HasPrefix(hash, "$argon2id$") {
		return true
	}
	var version int
	var mem, iter uint32
	var par uint8
	// Parse only the parameter section; ignore salt/hash.
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return true
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return true
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &par); err != nil {
		return true
	}
	return mem != argon2Memory || iter != argon2Iterations || par != argon2Parallelism
}

// verifyArgon2id parses a PHC argon2id string, recomputes the key from plain
// with the encoded parameters and salt, and compares in constant time.
func verifyArgon2id(hash, plain string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrInvalidHash
	}

	var mem, iter uint32
	var par uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &par); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	got := argon2.IDKey([]byte(plain), salt, iter, mem, par, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
