package auth

// HashPassword hashes a plaintext password with the default Argon2id hasher
// and returns a self-describing PHC-style string:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
func HashPassword(plain string) (string, error) {
	return DefaultHasher.Hash(plain)
}

// VerifyPassword reports whether plain matches the stored Argon2id hash in constant time.
func VerifyPassword(hash, plain string) bool {
	return DefaultHasher.Verify(hash, plain)
}

// CheckPassword is an alias for VerifyPassword for backward compatibility.
func CheckPassword(hash, plain string) bool {
	return VerifyPassword(hash, plain)
}

// NeedsRehash reports whether a stored hash should be upgraded to the current
// Argon2id parameters on the next successful login.
func NeedsRehash(hash string) bool {
	return DefaultHasher.NeedsRehash(hash)
}
