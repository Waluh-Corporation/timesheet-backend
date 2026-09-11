package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"timesheet-backend/models"
)

// Claims is the JWT payload carried in the session token.
type Claims struct {
	UserID uint        `json:"uid"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

// Service issues and validates authentication tokens.
type Service struct {
	secret []byte
	expiry time.Duration
}

// NewService constructs an auth Service.
func NewService(secret string, expiry time.Duration) *Service {
	return &Service{secret: []byte(secret), expiry: expiry}
}

// GenerateToken signs a JWT for the given user.
func (s *Service) GenerateToken(u *models.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: u.ID,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiry)),
			Subject:   u.Username,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ParseToken validates a JWT string and returns its claims.
func (s *Service) ParseToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// GenerateResetToken returns a random opaque token plus its SHA-256 hash. Only
// the hash is persisted; the raw token travels in the reset email.
func GenerateResetToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken hashes an opaque token for storage/lookup.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
