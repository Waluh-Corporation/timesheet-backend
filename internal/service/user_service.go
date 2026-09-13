package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"timesheet-backend/auth"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
)

// UserService defines user account management and password credential operations.
type UserService interface {
	CreateUserByAdmin(ctx context.Context, user *models.User, loginURL string) (initialPassword string, err error)
	ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error
}

type userService struct {
	repo   repository.UserRepository
	hasher auth.PasswordHasher
	mailer *mailer.Mailer
}

// NewUserService constructs an instance of UserService.
func NewUserService(repo repository.UserRepository, hasher auth.PasswordHasher, m *mailer.Mailer) UserService {
	if hasher == nil {
		hasher = auth.DefaultHasher
	}
	return &userService{
		repo:   repo,
		hasher: hasher,
		mailer: m,
	}
}

// CreateUserByAdmin generates a cryptographically-secure random initial password,
// hashes it with Argon2id, persists the user record, and dispatches a welcome notification.
func (s *userService) CreateUserByAdmin(ctx context.Context, user *models.User, loginURL string) (string, error) {
	// 1. Validasi keunikan username dan email agar tidak terjadi duplikasi data (HTTP 409)
	if existing, err := s.repo.FindByUsernameOrEmail(ctx, user.Username); err == nil && existing != nil {
		return "", domain.ErrUsernameConflict
	}
	if existing, err := s.repo.FindByUsernameOrEmail(ctx, user.Email); err == nil && existing != nil {
		return "", domain.ErrEmailConflict
	}

	// 2. Password awal TIDAK diinput manual; selalu auto-generate password acak yang kuat (16 chars, kombinasi lengkap)
	plainPass, err := auth.GenerateSecurePassword(auth.GeneratedPasswordLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure password: %w", err)
	}

	// 2. Hash menggunakan modern Argon2id
	hash, err := s.hasher.Hash(plainPass)
	if err != nil {
		return "", fmt.Errorf("could not hash password: %w", err)
	}
	user.PasswordHash = hash
	user.IsActive = true

	// 3. Simpan ke database via repository layer
	if err := s.repo.Create(ctx, user); err != nil {
		return "", err
	}

	// 4. Kirimkan notifikasi selamat datang dan kredensial terpisah dari flow reset password
	if s.mailer != nil && user.Email != "" {
		_ = s.mailer.SendAccountWelcomeEmail(user.Email, user.Username, plainPass, loginURL)
	}

	return plainPass, nil
}

// ChangePassword verifies current credentials, detects weak passwords, confirms difference,
// hashes with Argon2id, updates the timestamp, and sends a security notice email.
func (s *userService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return domain.ErrNotFound
	}
	if !user.IsActive {
		return domain.ErrAccountDisabled
	}

	// 1. Verifikasi password lama sesuai akun aktif di database
	if user.PasswordHash == "" || !s.hasher.Verify(user.PasswordHash, oldPassword) {
		return errors.New("old password does not match")
	}

	// 2. Konfirmasi password baru tidak boleh sama dengan password lama
	if oldPassword == newPassword || s.hasher.Verify(user.PasswordHash, newPassword) {
		return errors.New("new password cannot be the same as the old password")
	}

	// 3. Pengecekan kekuatan password baru: tolak jika lemah (pendek, pola sederhana, blocklist)
	if err := auth.ValidatePassword(newPassword, user.Username, user.Email, user.Name); err != nil {
		return err
	}

	// 4. Hash password baru dengan Argon2id
	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("could not hash password: %w", err)
	}

	// 5. Perbarui timestamp updated_at dan password_hash di database
	now := time.Now()
	if err := s.repo.UpdatePassword(ctx, user.ID, newHash, now); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// 6. Kirim notifikasi keamanan secara asinkron
	if s.mailer != nil && user.Email != "" {
		go func(to, username string) {
			_ = s.mailer.SendPasswordChangedEmail(to, username)
		}(user.Email, user.Username)
	}

	return nil
}
