package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"timesheet-backend/auth"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

type mockUserRepoForAuth struct {
	repository.UserAccountRepository
	user          *models.User
	findErr       error
	updatePassErr error
	passUpdated   bool
	updatedHash   string
}

func (m *mockUserRepoForAuth) FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.user, nil
}

func (m *mockUserRepoForAuth) FindByID(ctx context.Context, id uint) (*models.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.user != nil && m.user.ID == id {
		return m.user, nil
	}
	return nil, nil
}

func (m *mockUserRepoForAuth) UpdatePassword(ctx context.Context, id uint, hash string, changedAt time.Time) error {
	m.passUpdated = true
	m.updatedHash = hash
	return m.updatePassErr
}

type mockTokenRepoForAuth struct {
	repository.TokenRepository
	findToken      *models.RefreshToken
	findErr        error
	createErr      error
	revokeErr      error
	revokeFamErr   error
	familyRevoked  string
	tokenRevokedID uint
	createdToken   *models.RefreshToken
}

func (m *mockTokenRepoForAuth) FindRefreshTokenByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.findToken, nil
}

func (m *mockTokenRepoForAuth) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.createdToken = token
	return nil
}

func (m *mockTokenRepoForAuth) RevokeRefreshToken(ctx context.Context, id uint, revokedAt time.Time) error {
	m.tokenRevokedID = id
	return m.revokeErr
}

func (m *mockTokenRepoForAuth) RevokeFamily(ctx context.Context, familyID string, revokedAt time.Time) error {
	m.familyRevoked = familyID
	return m.revokeFamErr
}

type mockAuditRepoForAuth struct {
	repository.AuditRepository
	entries   []*models.AuditLog
	createErr error
}

func (m *mockAuditRepoForAuth) Create(ctx context.Context, entry *models.AuditLog) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.entries = append(m.entries, entry)
	return nil
}

type mockHasherForAuth struct {
	verifyResult bool
	needsRehash  bool
	hashResult   string
	hashErr      error
}

func (m *mockHasherForAuth) Hash(password string) (string, error) {
	if m.hashErr != nil {
		return "", m.hashErr
	}
	return m.hashResult, nil
}

func (m *mockHasherForAuth) Verify(hash, password string) bool {
	return m.verifyResult
}

func (m *mockHasherForAuth) NeedsRehash(hash string) bool {
	return m.needsRehash
}

func TestAuthService_Login(t *testing.T) {
	rawPass := "MyStr0ngPass!2026"
	hash, _ := auth.HashPassword(rawPass)
	jwtSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)

	t.Run("nil user repo returns error", func(t *testing.T) {
		svc := service.NewAuthService(nil, nil, jwtSvc, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "admin", Password: rawPass})
		if err == nil {
			t.Fatal("expected error with nil user repo, got nil")
		}
	})

	t.Run("user not found returns unauthorized", func(t *testing.T) {
		repo := &mockUserRepoForAuth{findErr: errors.New("not found")}
		svc := service.NewAuthService(repo, nil, jwtSvc, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "admin", Password: rawPass})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("user inactive returns forbidden", func(t *testing.T) {
		repo := &mockUserRepoForAuth{
			user: &models.User{
				ID:           1,
				Username:     "inactive",
				PasswordHash: hash,
				IsActive:     false,
			},
		}
		svc := service.NewAuthService(repo, nil, jwtSvc, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "inactive", Password: rawPass})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("invalid password returns unauthorized", func(t *testing.T) {
		repo := &mockUserRepoForAuth{
			user: &models.User{
				ID:           1,
				Username:     "active",
				PasswordHash: hash,
				IsActive:     true,
			},
		}
		svc := service.NewAuthService(repo, nil, jwtSvc, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "active", Password: "WrongPassword!"})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty password hash returns unauthorized", func(t *testing.T) {
		repo := &mockUserRepoForAuth{
			user: &models.User{
				ID:           1,
				Username:     "active",
				PasswordHash: "",
				IsActive:     true,
			},
		}
		svc := service.NewAuthService(repo, nil, jwtSvc, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "active", Password: rawPass})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("nil jwt service returns error", func(t *testing.T) {
		repo := &mockUserRepoForAuth{
			user: &models.User{
				ID:           1,
				Username:     "active",
				PasswordHash: hash,
				IsActive:     true,
			},
		}
		svc := service.NewAuthService(repo, nil, nil, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "active", Password: rawPass})
		if err == nil {
			t.Fatal("expected error with nil jwt service, got nil")
		}
	})

	t.Run("failed to persist refresh token returns error", func(t *testing.T) {
		repo := &mockUserRepoForAuth{
			user: &models.User{
				ID:           1,
				Username:     "active",
				PasswordHash: hash,
				IsActive:     true,
			},
		}
		tokenRepo := &mockTokenRepoForAuth{createErr: errors.New("db error")}
		svc := service.NewAuthService(repo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "active", Password: rawPass})
		if err == nil {
			t.Fatal("expected error when persisting token fails, got nil")
		}
	})

	t.Run("rehash weaker hash on successful login", func(t *testing.T) {
		weakerPass := "WeakerPassword123!"
		weakHasher := auth.NewArgon2idHasher(auth.Argon2idParams{
			Memory:      32 * 1024,
			Iterations:  2,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
		})
		wHash, err := weakHasher.Hash(weakerPass)
		if err != nil {
			t.Fatalf("failed to create weak hash: %v", err)
		}
		repo := &mockUserRepoForAuth{
			user: &models.User{
				ID:           1,
				Username:     "weaker",
				PasswordHash: wHash,
				IsActive:     true,
			},
		}
		tokenRepo := &mockTokenRepoForAuth{}
		auditRepo := &mockAuditRepoForAuth{}
		svc := service.NewAuthService(repo, tokenRepo, jwtSvc, auditRepo, time.Hour)
		res, err := svc.Login(context.Background(), service.LoginDTO{
			Identifier: "weaker",
			Password:   weakerPass,
			ClientIP:   "127.0.0.1",
			UserAgent:  "test-agent",
		})
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}
		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Fatal("expected tokens, got empty")
		}
		if !repo.passUpdated {
			t.Error("expected password to be rehashed and updated")
		}
		if len(auditRepo.entries) != 1 {
			t.Errorf("expected 1 audit log entry, got %d", len(auditRepo.entries))
		}
	})

	t.Run("successful login with audit log error does not fail login", func(t *testing.T) {
		repo := &mockUserRepoForAuth{
			user: &models.User{
				ID:           1,
				Username:     "admin",
				PasswordHash: hash,
				IsActive:     true,
			},
		}
		tokenRepo := &mockTokenRepoForAuth{}
		auditRepo := &mockAuditRepoForAuth{createErr: errors.New("audit db failure")}
		svc := service.NewAuthService(repo, tokenRepo, jwtSvc, auditRepo, 0)
		res, err := svc.Login(context.Background(), service.LoginDTO{
			Identifier: "admin",
			Password:   rawPass,
			ClientIP:   "192.168.1.1",
			UserAgent:  "Mozilla",
		})
		if err != nil {
			t.Fatalf("expected login to succeed even with audit error, got %v", err)
		}
		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Fatal("expected tokens to be issued")
		}
	})

	t.Run("empty identifier or password returns unauthorized", func(t *testing.T) {
		repo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, Username: "admin", PasswordHash: hash, IsActive: true},
		}
		svc := service.NewAuthService(repo, nil, jwtSvc, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "   ", Password: rawPass})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized for whitespace identifier, got %v", err)
		}
		_, err = svc.Login(context.Background(), service.LoginDTO{Identifier: "admin", Password: ""})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized for empty password, got %v", err)
		}
	})

	t.Run("rehash hash error still succeeds login", func(t *testing.T) {
		mockH := &mockHasherForAuth{
			verifyResult: true,
			needsRehash:  true,
			hashErr:      errors.New("hash generation error"),
		}
		repo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, Username: "user1", PasswordHash: "old-hash", IsActive: true},
		}
		tokenRepo := &mockTokenRepoForAuth{}
		svc := service.NewAuthService(repo, tokenRepo, jwtSvc, nil, time.Hour, mockH)
		res, err := svc.Login(context.Background(), service.LoginDTO{
			Identifier: "user1",
			Password:   rawPass,
		})
		if err != nil {
			t.Fatalf("expected login to succeed despite rehash hash error, got %v", err)
		}
		if res.AccessToken == "" {
			t.Fatal("expected access token to be generated")
		}
		if repo.passUpdated {
			t.Error("expected password update not to be called if hash fails")
		}
	})

	t.Run("rehash update password error still succeeds login", func(t *testing.T) {
		mockH := &mockHasherForAuth{
			verifyResult: true,
			needsRehash:  true,
			hashResult:   "new-upgraded-hash",
		}
		repo := &mockUserRepoForAuth{
			user:          &models.User{ID: 1, Username: "user1", PasswordHash: "old-hash", IsActive: true},
			updatePassErr: errors.New("db error updating password"),
		}
		tokenRepo := &mockTokenRepoForAuth{}
		svc := service.NewAuthService(repo, tokenRepo, jwtSvc, nil, time.Hour, mockH)
		res, err := svc.Login(context.Background(), service.LoginDTO{
			Identifier: "user1",
			Password:   rawPass,
		})
		if err != nil {
			t.Fatalf("expected login to succeed despite rehash update error, got %v", err)
		}
		if res.AccessToken == "" {
			t.Fatal("expected access token to be generated")
		}
		if !repo.passUpdated {
			t.Error("expected password update attempt")
		}
	})
}

func TestAuthService_RefreshToken(t *testing.T) {
	jwtSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	now := time.Now()

	t.Run("nil repos return error", func(t *testing.T) {
		svc := service.NewAuthService(nil, nil, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "abc"})
		if err == nil {
			t.Fatal("expected error with nil repos, got nil")
		}
	})

	t.Run("empty refresh token returns unauthorized", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{}
		tokenRepo := &mockTokenRepoForAuth{}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "   "})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized for whitespace token, got %v", err)
		}
	})

	t.Run("token not found returns unauthorized", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{}
		tokenRepo := &mockTokenRepoForAuth{findToken: nil}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "missing"})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("token reuse detected revokes family", func(t *testing.T) {
		revokedTime := now.Add(-10 * time.Minute)
		famID := uuid.NewString()
		userRepo := &mockUserRepoForAuth{}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  famID,
				RevokedAt: &revokedTime,
			},
		}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "reused"})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
		if tokenRepo.familyRevoked != famID {
			t.Fatalf("expected family %s to be revoked, got %s", famID, tokenRepo.familyRevoked)
		}
	})

	t.Run("token reuse detected with revocation error still returns unauthorized", func(t *testing.T) {
		revokedTime := now.Add(-10 * time.Minute)
		famID := uuid.NewString()
		userRepo := &mockUserRepoForAuth{}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  famID,
				RevokedAt: &revokedTime,
			},
			revokeFamErr: errors.New("db error revoking family"),
		}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "reused"})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("token expired returns unauthorized", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  "fam-1",
				ExpiresAt: now.Add(-time.Hour),
			},
		}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "expired"})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("user inactive returns unauthorized", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, IsActive: false},
		}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  "fam-1",
				ExpiresAt: now.Add(time.Hour),
			},
		}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "valid"})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("failed to revoke old token returns error", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, IsActive: true},
		}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  "fam-1",
				ExpiresAt: now.Add(time.Hour),
			},
			revokeErr: errors.New("db error"),
		}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "valid"})
		if err == nil {
			t.Fatal("expected error when revoking old token fails, got nil")
		}
	})

	t.Run("nil jwt service returns error", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, IsActive: true},
		}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  "fam-1",
				ExpiresAt: now.Add(time.Hour),
			},
		}
		svc := service.NewAuthService(userRepo, tokenRepo, nil, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "valid"})
		if err == nil {
			t.Fatal("expected error with nil jwt service, got nil")
		}
	})

	t.Run("failed to persist rotated token returns error", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, IsActive: true},
		}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  "fam-1",
				ExpiresAt: now.Add(time.Hour),
			},
			createErr: errors.New("persist error"),
		}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "valid"})
		if err == nil {
			t.Fatal("expected error when persisting rotated token fails, got nil")
		}
	})

	t.Run("successful refresh token rotation", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, IsActive: true},
		}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        10,
				UserID:    1,
				FamilyID:  "fam-1",
				ExpiresAt: now.Add(time.Hour),
			},
		}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, 0)
		res, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{
			RefreshToken: "valid",
			ClientIP:     "127.0.0.1",
			UserAgent:    "Agent",
		})
		if err != nil {
			t.Fatalf("expected successful rotation, got %v", err)
		}
		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Fatal("expected non-empty tokens")
		}
		if tokenRepo.tokenRevokedID != 10 {
			t.Errorf("expected token 10 to be revoked, got %d", tokenRepo.tokenRevokedID)
		}
		if tokenRepo.createdToken == nil || tokenRepo.createdToken.FamilyID != "fam-1" {
			t.Errorf("expected rotated token in family fam-1, got %+v", tokenRepo.createdToken)
		}
	})
}

func TestAuthService_RevokeRefreshToken(t *testing.T) {
	t.Run("empty token returns nil", func(t *testing.T) {
		svc := service.NewAuthService(nil, nil, nil, nil, 0)
		err := svc.RevokeRefreshToken(context.Background(), "", "127.0.0.1")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("nil token repo returns nil", func(t *testing.T) {
		svc := service.NewAuthService(nil, nil, nil, nil, 0)
		err := svc.RevokeRefreshToken(context.Background(), "abc", "127.0.0.1")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("token not found returns nil", func(t *testing.T) {
		tokenRepo := &mockTokenRepoForAuth{findToken: nil}
		svc := service.NewAuthService(nil, tokenRepo, nil, nil, 0)
		err := svc.RevokeRefreshToken(context.Background(), "abc", "127.0.0.1")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("token lookup error returns error", func(t *testing.T) {
		tokenRepo := &mockTokenRepoForAuth{findErr: errors.New("db error")}
		svc := service.NewAuthService(nil, tokenRepo, nil, nil, 0)
		err := svc.RevokeRefreshToken(context.Background(), "abc", "127.0.0.1")
		if err == nil {
			t.Fatal("expected error when token lookup fails, got nil")
		}
	})

	t.Run("token revoke error returns error", func(t *testing.T) {
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{ID: 42, UserID: 1},
			revokeErr: errors.New("revoke error"),
		}
		svc := service.NewAuthService(nil, tokenRepo, nil, nil, 0)
		err := svc.RevokeRefreshToken(context.Background(), "abc", "127.0.0.1")
		if err == nil {
			t.Fatal("expected error when revoking fails, got nil")
		}
	})

	t.Run("token found revokes token", func(t *testing.T) {
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{ID: 42, UserID: 1},
		}
		svc := service.NewAuthService(nil, tokenRepo, nil, nil, 0)
		err := svc.RevokeRefreshToken(context.Background(), "abc", "127.0.0.1")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if tokenRepo.tokenRevokedID != 42 {
			t.Errorf("expected token 42 to be revoked, got %d", tokenRepo.tokenRevokedID)
		}
	})
}

func TestAuthService_GetProfile(t *testing.T) {
	t.Run("nil user repo returns error", func(t *testing.T) {
		svc := service.NewAuthService(nil, nil, nil, nil, 0)
		_, err := svc.GetProfile(context.Background(), 1)
		if err == nil {
			t.Fatal("expected error with nil user repo, got nil")
		}
	})

	t.Run("user not found returns ErrNotFound", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{findErr: errors.New("not found")}
		svc := service.NewAuthService(userRepo, nil, nil, nil, 0)
		_, err := svc.GetProfile(context.Background(), 999)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("zero user ID returns ErrNotFound", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{}
		svc := service.NewAuthService(userRepo, nil, nil, nil, 0)
		_, err := svc.GetProfile(context.Background(), 0)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound for user id 0, got %v", err)
		}
	})

	t.Run("user found returns user", func(t *testing.T) {
		user := &models.User{ID: 1, Username: "founduser"}
		userRepo := &mockUserRepoForAuth{user: user}
		svc := service.NewAuthService(userRepo, nil, nil, nil, 0)
		res, err := svc.GetProfile(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if res.Username != "founduser" {
			t.Errorf("expected username founduser, got %s", res.Username)
		}
	})
}

func TestAuthService_Accessors(t *testing.T) {
	userRepo := &mockUserRepoForAuth{}
	tokenRepo := &mockTokenRepoForAuth{}
	auditRepo := &mockAuditRepoForAuth{}
	jwtSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	mockH := &mockHasherForAuth{}
	svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, auditRepo, 0, mockH)

	type repoChecker interface {
		GetTokenRepo() repository.TokenRepository
		GetUserRepo() repository.UserAccountRepository
		GetAuditRepo() repository.AuditRepository
		GetJWTService() *auth.Service
		GetHasher() auth.PasswordHasher
	}

	checker, ok := svc.(repoChecker)
	if !ok {
		t.Fatal("expected AuthService to implement repoChecker interface")
	}
	if checker.GetTokenRepo() != tokenRepo {
		t.Errorf("expected tokenRepo %v, got %v", tokenRepo, checker.GetTokenRepo())
	}
	if checker.GetUserRepo() != userRepo {
		t.Errorf("expected userRepo %v, got %v", userRepo, checker.GetUserRepo())
	}
	if checker.GetAuditRepo() != auditRepo {
		t.Errorf("expected auditRepo %v, got %v", auditRepo, checker.GetAuditRepo())
	}
	if checker.GetJWTService() != jwtSvc {
		t.Errorf("expected jwtSvc %v, got %v", jwtSvc, checker.GetJWTService())
	}
	if checker.GetHasher() != mockH {
		t.Errorf("expected hasher %v, got %v", mockH, checker.GetHasher())
	}
}

func TestAuthService_ContextCancellation(t *testing.T) {
	jwtSvc := auth.NewService("test-secret-at-least-32-chars-long!", time.Hour)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("Login respects context cancellation", func(t *testing.T) {
		repo := &mockUserRepoForAuth{findErr: context.Canceled}
		svc := service.NewAuthService(repo, nil, jwtSvc, nil, time.Hour)
		_, err := svc.Login(canceledCtx, service.LoginDTO{Identifier: "admin", Password: "any"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("RefreshToken respects context cancellation", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{}
		tokenRepo := &mockTokenRepoForAuth{findErr: context.Canceled}
		svc := service.NewAuthService(userRepo, tokenRepo, jwtSvc, nil, time.Hour)
		_, err := svc.RefreshToken(canceledCtx, service.RefreshTokenDTO{RefreshToken: "valid-token"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("GetProfile respects context cancellation", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{findErr: context.Canceled}
		svc := service.NewAuthService(userRepo, nil, nil, nil, 0)
		_, err := svc.GetProfile(canceledCtx, 1)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestAuthService_EdgeCasesAndOrdering(t *testing.T) {
	now := time.Now()

	t.Run("RefreshToken does not revoke token if jwtSvc is nil", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{
			user: &models.User{ID: 1, IsActive: true},
		}
		tokenRepo := &mockTokenRepoForAuth{
			findToken: &models.RefreshToken{
				ID:        55,
				UserID:    1,
				FamilyID:  "safe-fam",
				ExpiresAt: now.Add(time.Hour),
			},
		}
		svc := service.NewAuthService(userRepo, tokenRepo, nil, nil, time.Hour)
		_, err := svc.RefreshToken(context.Background(), service.RefreshTokenDTO{RefreshToken: "valid-tok"})
		if err == nil {
			t.Fatal("expected error with nil jwtSvc, got nil")
		}
		if tokenRepo.tokenRevokedID != 0 {
			t.Fatalf("expected token not to be revoked on nil jwtSvc, but was revoked: id=%d", tokenRepo.tokenRevokedID)
		}
	})

	t.Run("Login returns error upfront if jwtSvc is nil", func(t *testing.T) {
		userRepo := &mockUserRepoForAuth{}
		svc := service.NewAuthService(userRepo, nil, nil, nil, time.Hour)
		_, err := svc.Login(context.Background(), service.LoginDTO{Identifier: "admin", Password: "any"})
		if err == nil {
			t.Fatal("expected error when jwtSvc is nil, got nil")
		}
	})
}
