package auth_test

import (
	"testing"
	"time"

	"timesheet-backend/auth"
	"timesheet-backend/models"
)

func TestAuthService_GenerateAndParseToken(t *testing.T) {
	svc := auth.NewService("super-secret-key-for-unit-testing-32b", 1*time.Hour)

	user := &models.User{
		ID:       101,
		Username: "testuser",
		Role:     models.RoleUser,
	}

	// 1. Generate Token
	tokenStr, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("generated token is empty")
	}

	// 2. Parse Token Successfully
	claims, err := svc.ParseToken(tokenStr)
	if err != nil {
		t.Fatalf("failed to parse valid token: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("expected UserID %d, got %d", user.ID, claims.UserID)
	}
	if claims.Role != user.Role {
		t.Errorf("expected Role %s, got %s", user.Role, claims.Role)
	}
	if claims.Subject != user.Username {
		t.Errorf("expected Subject %s, got %s", user.Username, claims.Subject)
	}

	// 3. Reject Tampered Token
	tampered := tokenStr + "tampered"
	_, err = svc.ParseToken(tampered)
	if err == nil {
		t.Errorf("expected error parsing tampered token")
	}

	// 4. Reject Expired Token
	expiredSvc := auth.NewService("super-secret-key-for-unit-testing-32b", -1*time.Hour)
	expToken, err := expiredSvc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}
	_, err = svc.ParseToken(expToken)
	if err == nil {
		t.Errorf("expected error parsing expired token")
	}

	// 5. Reject Token Signed With Different Key
	otherSvc := auth.NewService("completely-different-key-12345678", 1*time.Hour)
	_, err = otherSvc.ParseToken(tokenStr)
	if err == nil {
		t.Errorf("expected error parsing token with different secret")
	}
}

func TestResetTokens(t *testing.T) {
	raw, hash, err := auth.GenerateResetToken()
	if err != nil {
		t.Fatalf("failed to generate reset token: %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("raw token or hash is empty")
	}

	computedHash := auth.HashToken(raw)
	if computedHash != hash {
		t.Errorf("expected hash %s, got %s", hash, computedHash)
	}
}
