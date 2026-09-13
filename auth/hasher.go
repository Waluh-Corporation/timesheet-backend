package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrInvalidHash is returned when a stored hash string is malformed or uses an
// unsupported algorithm or version.
var ErrInvalidHash = errors.New("invalid or unsupported password hash")

// PasswordHasher defines a clean, modular, and thread-safe contract for password
// hashing, verification, and algorithm upgrade detection.
type PasswordHasher interface {
	// Hash computes a cryptographically secure hash of the plaintext password.
	Hash(plain string) (string, error)
	// Verify reports whether the plaintext password matches the stored hash in constant time.
	Verify(hash, plain string) bool
	// NeedsRehash reports whether the stored hash was produced with parameters older or weaker
	// than the current configuration and should be refreshed.
	NeedsRehash(hash string) bool
}

// Argon2idParams defines the computational cost and sizing parameters for Argon2id.
type Argon2idParams struct {
	Memory      uint32 // in KiB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2idParams returns parameters adhering to OWASP Password Storage
// recommendations for Argon2id (64 MiB memory, 3 iterations, 2 parallelism).
func DefaultArgon2idParams() Argon2idParams {
	return Argon2idParams{
		Memory:      64 * 1024, // 64 MiB
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// Argon2idHasher implements PasswordHasher using the Argon2id key derivation function.
// All methods are safe for concurrent use by multiple goroutines.
type Argon2idHasher struct {
	params Argon2idParams
}

// NewArgon2idHasher constructs an Argon2idHasher with the specified parameters.
func NewArgon2idHasher(params Argon2idParams) *Argon2idHasher {
	if params.Memory == 0 || params.Iterations == 0 || params.Parallelism == 0 || params.SaltLength == 0 || params.KeyLength == 0 {
		params = DefaultArgon2idParams()
	}
	return &Argon2idHasher{params: params}
}

// DefaultHasher is the default application-wide PasswordHasher instance configured with Argon2id.
var DefaultHasher PasswordHasher = NewArgon2idHasher(DefaultArgon2idParams())

// Hash hashes a plaintext password with Argon2id and returns a self-describing
// PHC string: $argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
func (h *Argon2idHasher) Hash(plain string) (string, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain), salt, h.params.Iterations, h.params.Memory, h.params.Parallelism, h.params.KeyLength)

	b64 := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.Memory, h.params.Iterations, h.params.Parallelism,
		b64(salt), b64(key),
	), nil
}

// Verify checks whether plain matches the Argon2id PHC hash in constant time.
func (h *Argon2idHasher) Verify(hash, plain string) bool {
	if !strings.HasPrefix(hash, "$argon2id$") {
		return false
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}

	var mem, iter uint32
	var par uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &par); err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}

	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	if len(want) > 1024 {
		return false
	}

	//nolint:gosec // G115: len(want) bounded above
	got := argon2.IDKey([]byte(plain), salt, iter, mem, par, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NeedsRehash checks if the hash string does not match the configured memory, iterations,
// or parallelism parameters, or is not an argon2id hash.
func (h *Argon2idHasher) NeedsRehash(hash string) bool {
	if !strings.HasPrefix(hash, "$argon2id$") {
		return true
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return true
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return true
	}

	var mem, iter uint32
	var par uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &par); err != nil {
		return true
	}

	return mem != h.params.Memory || iter != h.params.Iterations || par != h.params.Parallelism
}
