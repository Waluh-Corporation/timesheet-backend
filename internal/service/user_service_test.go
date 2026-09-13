package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"timesheet-backend/auth"
	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

type mockUserRepo struct {
	users          map[uint]*models.User
	updatedPass    map[uint]string
	updatedAtTimes map[uint]time.Time
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:          make(map[uint]*models.User),
		updatedPass:    make(map[uint]string),
		updatedAtTimes: make(map[uint]time.Time),
	}
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) FindByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error) {
	for _, u := range m.users {
		if u.Username == identifier || u.Email == identifier {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	user.ID = uint(len(m.users) + 1)
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) UpdatePassword(ctx context.Context, id uint, passwordHash string, updatedAt time.Time) error {
	m.updatedPass[id] = passwordHash
	m.updatedAtTimes[id] = updatedAt
	if u, ok := m.users[id]; ok {
		u.PasswordHash = passwordHash
		u.UpdatedAt = updatedAt
	}
	return nil
}

func TestUserService_CreateUserByAdmin(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, auth.DefaultHasher, nil)

	user := &models.User{
		Username: "newadminuser",
		Email:    "newadminuser@example.com",
		Role:     models.RoleUser,
	}

	pass, err := svc.CreateUserByAdmin(context.Background(), user, "http://localhost/login")
	if err != nil {
		t.Fatalf("CreateUserByAdmin failed: %v", err)
	}

	if len(pass) != 16 {
		t.Fatalf("expected 16 chars generated password, got %d", len(pass))
	}
	if !strings.HasPrefix(user.PasswordHash, "$argon2id$") {
		t.Errorf("expected Argon2id hash, got %s", user.PasswordHash)
	}
	if !auth.VerifyPassword(user.PasswordHash, pass) {
		t.Errorf("generated password failed verification against stored hash")
	}

	// Duplicate username rejection
	dupUser := &models.User{
		Username: "newadminuser",
		Email:    "different@example.com",
	}
	_, err = svc.CreateUserByAdmin(context.Background(), dupUser, "http://localhost/login")
	if !errors.Is(err, domain.ErrUsernameConflict) {
		t.Errorf("expected ErrUsernameConflict, got %v", err)
	}

	// Duplicate email rejection
	dupEmail := &models.User{
		Username: "different_username",
		Email:    "newadminuser@example.com",
	}
	_, err = svc.CreateUserByAdmin(context.Background(), dupEmail, "http://localhost/login")
	if !errors.Is(err, domain.ErrEmailConflict) {
		t.Errorf("expected ErrEmailConflict, got %v", err)
	}
}

func TestUserService_ChangePassword(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, auth.DefaultHasher, nil)

	oldPass := "InitialPass123!@#"
	oldHash, _ := auth.HashPassword(oldPass)

	activeUser := &models.User{
		ID:           1,
		Username:     "activeuser",
		Email:        "active@example.com",
		Name:         "Active User",
		PasswordHash: oldHash,
		IsActive:     true,
	}
	repo.users[1] = activeUser

	disabledUser := &models.User{
		ID:           2,
		Username:     "disableduser",
		Email:        "disabled@example.com",
		PasswordHash: oldHash,
		IsActive:     false,
	}
	repo.users[2] = disabledUser

	t.Run("User not found", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 999, oldPass, "NewPass123!@#")
		if err == nil {
			t.Fatal("expected error for non-existent user")
		}
	})

	t.Run("User disabled", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 2, oldPass, "NewPass123!@#")
		if err == nil {
			t.Fatal("expected error for disabled user")
		}
	})

	t.Run("Wrong old password", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 1, "IncorrectOldPass1!", "NewPass123!@#")
		if err == nil || !strings.Contains(err.Error(), "old password does not match") {
			t.Fatalf("expected old password mismatch error, got: %v", err)
		}
	})

	t.Run("Same new password as old password", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 1, oldPass, oldPass)
		if err == nil || !strings.Contains(err.Error(), "new password cannot be the same") {
			t.Fatalf("expected same password error, got: %v", err)
		}
	})

	t.Run("Weak new password rejected", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), 1, oldPass, "123")
		if err == nil {
			t.Fatal("expected error for weak new password")
		}
	})

	t.Run("Successful password change", func(t *testing.T) {
		newPass := "SuperSecretNewPassword123!@#"
		err := svc.ChangePassword(context.Background(), 1, oldPass, newPass)
		if err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}

		if !auth.VerifyPassword(repo.updatedPass[1], newPass) {
			t.Fatal("new password failed verification against updated hash")
		}
		if auth.VerifyPassword(repo.updatedPass[1], oldPass) {
			t.Fatal("old password unexpectedly verified against updated hash")
		}
		if repo.updatedAtTimes[1].IsZero() {
			t.Fatal("expected updated_at time to be set")
		}
	})
}
