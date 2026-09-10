package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestArgon2idRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected argon2id PHC string, got %q", hash)
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("CheckPassword rejected the correct password")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("CheckPassword accepted a wrong password")
	}
	if NeedsRehash(hash) {
		t.Fatal("freshly generated argon2id hash should not need rehash")
	}
}

func TestCheckPasswordLegacyBcrypt(t *testing.T) {
	// A legacy bcrypt hash must still verify, and must be flagged for rehash.
	b, err := bcrypt.GenerateFromPassword([]byte("legacyPass123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	legacy := string(b)
	if !CheckPassword(legacy, "legacyPass123") {
		t.Fatal("legacy bcrypt hash failed to verify")
	}
	if CheckPassword(legacy, "nope") {
		t.Fatal("legacy bcrypt hash verified a wrong password")
	}
	if !NeedsRehash(legacy) {
		t.Fatal("legacy bcrypt hash should need rehash")
	}
}

func TestCheckPasswordRejectsGarbage(t *testing.T) {
	if CheckPassword("not-a-hash", "whatever") {
		t.Fatal("garbage hash should never verify")
	}
	if CheckPassword("", "whatever") {
		t.Fatal("empty hash should never verify")
	}
}
