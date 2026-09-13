package auth

import (
	"strings"
	"sync"
	"testing"
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

func TestCheckPasswordRejectsGarbage(t *testing.T) {
	if CheckPassword("not-a-hash", "whatever") {
		t.Fatal("garbage hash should never verify")
	}
	if CheckPassword("", "whatever") {
		t.Fatal("empty hash should never verify")
	}
	if CheckPassword("$2a$10$legacyBcryptNotSupportedAnymore", "whatever") {
		t.Fatal("legacy bcrypt hash must no longer be accepted")
	}

	// Oversized hash (> 1024 bytes)
	salt := "c2FsdHNhbHRzYWx0c2FsdA" // 16 bytes base64
	oversizedWant := strings.Repeat("A", 1400)
	hugeHash := "$argon2id$v=19$m=65536,t=3,p=2$" + salt + "$" + oversizedWant
	if CheckPassword(hugeHash, "whatever") {
		t.Fatal("oversized hash should never verify")
	}
}

func TestArgon2idHasher_CustomParametersAndNeedsRehash(t *testing.T) {
	hasher := NewArgon2idHasher(Argon2idParams{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})

	hash, err := hasher.Hash("customParamSecret123")
	if err != nil {
		t.Fatalf("hasher.Hash failed: %v", err)
	}
	if !hasher.Verify(hash, "customParamSecret123") {
		t.Fatal("hasher.Verify failed with matching password")
	}
	if hasher.Verify(hash, "wrongPass") {
		t.Fatal("hasher.Verify succeeded with invalid password")
	}

	// Default hasher uses m=65536, t=3, p=2, so this weaker hash must trigger NeedsRehash
	if !DefaultHasher.NeedsRehash(hash) {
		t.Fatal("weaker hash should require rehash against DefaultHasher")
	}
}

func TestArgon2idHasher_ConcurrentThreadSafety(t *testing.T) {
	hasher := DefaultHasher
	var wg sync.WaitGroup
	workers := 8

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plain := "concurrentSafetyCheck123!"
			h, err := hasher.Hash(plain)
			if err != nil {
				t.Errorf("concurrent Hash returned error: %v", err)
				return
			}
			if !hasher.Verify(h, plain) {
				t.Errorf("concurrent Verify failed for valid password")
			}
		}()
	}

	wg.Wait()
}
