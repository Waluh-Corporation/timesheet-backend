package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"timesheet-backend/auth"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

const (
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
)

// LoginDTO encapsulates input parameters required to authenticate a user.
type LoginDTO struct {
	Identifier string
	Password   string
	ClientIP   string
	UserAgent  string
}

// LoginResult contains tokens and user information returned upon successful authentication.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *models.User
}

// RefreshTokenDTO encapsulates input parameters for refreshing an access token.
type RefreshTokenDTO struct {
	RefreshToken string
	ClientIP     string
	UserAgent    string
}

// RefreshTokenResult contains newly issued access and rotated refresh tokens.
type RefreshTokenResult struct {
	AccessToken  string
	RefreshToken string
}

// AuthService defines business operations for credential authentication and token lifecycle management.
type AuthService interface {
	Login(ctx context.Context, dto LoginDTO) (*LoginResult, error)
	RefreshToken(ctx context.Context, dto RefreshTokenDTO) (*RefreshTokenResult, error)
	RevokeRefreshToken(ctx context.Context, refreshToken, clientIP string) error
	GetProfile(ctx context.Context, userID uint) (*models.User, error)
}

type authService struct {
	userRepo        repository.UserAccountRepository
	tokenRepo       repository.TokenRepository
	jwtSvc          *auth.Service
	auditRepo       repository.AuditRepository
	refreshTokenTTL time.Duration
	hasher          auth.PasswordHasher
}

// NewAuthService constructs an AuthService implementation.
func NewAuthService(
	userRepo repository.UserAccountRepository,
	tokenRepo repository.TokenRepository,
	jwtSvc *auth.Service,
	auditRepo repository.AuditRepository,
	refreshTokenTTL time.Duration,
	hasher ...auth.PasswordHasher,
) AuthService {
	h := auth.DefaultHasher
	if len(hasher) > 0 && hasher[0] != nil {
		h = hasher[0]
	}
	return &authService{
		userRepo:        userRepo,
		tokenRepo:       tokenRepo,
		jwtSvc:          jwtSvc,
		auditRepo:       auditRepo,
		refreshTokenTTL: refreshTokenTTL,
		hasher:          h,
	}
}

// GetTokenRepo returns the underlying TokenRepository.
func (s *authService) GetTokenRepo() repository.TokenRepository {
	return s.tokenRepo
}

// GetUserRepo returns the underlying UserAccountRepository.
func (s *authService) GetUserRepo() repository.UserAccountRepository {
	return s.userRepo
}

// GetJWTService returns the underlying JWT auth service.
func (s *authService) GetJWTService() *auth.Service {
	return s.jwtSvc
}

// GetAuditRepo returns the underlying AuditRepository.
func (s *authService) GetAuditRepo() repository.AuditRepository {
	return s.auditRepo
}

// GetHasher returns the underlying PasswordHasher.
func (s *authService) GetHasher() auth.PasswordHasher {
	return s.hasher
}

// Login authenticates a user by verifying password credentials, generating dual tokens,
// and optionally upgrading password hashes and recording audit events.
func (s *authService) Login(ctx context.Context, dto LoginDTO) (*LoginResult, error) {
	if s.userRepo == nil || s.jwtSvc == nil {
		return nil, domain.NewUserError(domain.ErrInternal, "database repository unavailable")
	}

	user, err := s.validateAndFetchUser(ctx, dto.Identifier, dto.Password, dto.ClientIP)
	if err != nil {
		return nil, err
	}

	s.handlePasswordRehash(ctx, user, dto.Password)

	accessToken, err := s.jwtSvc.GenerateToken(user)
	if err != nil {
		slog.Error("failed to generate access token", "user_id", user.ID, "error", err)
		return nil, domain.NewUserError(domain.ErrInternal, "could not issue token")
	}

	rawRefreshToken, err := s.issueRefreshToken(ctx, user.ID, dto.ClientIP, dto.UserAgent)
	if err != nil {
		return nil, err
	}

	s.recordLoginAuditLog(ctx, user, dto.Identifier, dto.ClientIP, dto.UserAgent)

	slog.Info("user logged in successfully", "user_id", user.ID, "username", user.Username, "ip", dto.ClientIP, "user_agent", dto.UserAgent)

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		User:         user,
	}, nil
}

func (s *authService) validateAndFetchUser(ctx context.Context, identifier, password, clientIP string) (*models.User, error) {
	if strings.TrimSpace(identifier) == "" || password == "" {
		slog.Warn("login failed: empty identifier or password", "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrUnauthorized, "invalid credentials")
	}

	user, err := s.userRepo.FindByUsernameOrEmail(ctx, identifier)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		slog.Warn("login failed: user not found", "identifier", identifier, "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrUnauthorized, "invalid credentials")
	}
	if user == nil {
		slog.Warn("login failed: user not found", "identifier", identifier, "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrUnauthorized, "invalid credentials")
	}

	if !user.IsActive {
		slog.Warn("login failed: account deactivated", "user_id", user.ID, "username", user.Username, "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrForbidden, "account is disabled")
	}

	hasher := s.hasher
	if hasher == nil {
		hasher = auth.DefaultHasher
	}

	if user.PasswordHash == "" || !hasher.Verify(user.PasswordHash, password) {
		slog.Warn("login failed: invalid password", "user_id", user.ID, "username", user.Username, "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrUnauthorized, "invalid credentials")
	}

	return user, nil
}

func (s *authService) handlePasswordRehash(ctx context.Context, user *models.User, password string) {
	hasher := s.hasher
	if hasher == nil {
		hasher = auth.DefaultHasher
	}
	if !hasher.NeedsRehash(user.PasswordHash) {
		return
	}
	newHash, herr := hasher.Hash(password)
	if herr != nil {
		slog.Warn("failed to generate updated password hash", "user_id", user.ID, "error", herr)
		return
	}
	if uerr := s.userRepo.UpdatePassword(ctx, user.ID, newHash, time.Now()); uerr != nil {
		slog.Warn("failed to persist upgraded password hash", "user_id", user.ID, "error", uerr)
	}
}

func (s *authService) issueRefreshToken(ctx context.Context, userID uint, clientIP, userAgent string) (string, error) {
	rawRefreshToken, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.Error("failed to generate refresh token", "user_id", userID, "error", err)
		return "", domain.NewUserError(domain.ErrInternal, "could not issue refresh token")
	}

	if s.tokenRepo != nil {
		ttl := s.refreshTokenTTL
		if ttl <= 0 {
			ttl = defaultRefreshTokenTTL
		}
		refreshRecord := &models.RefreshToken{
			UserID:    userID,
			TokenHash: refreshHash,
			FamilyID:  uuid.NewString(),
			ExpiresAt: time.Now().Add(ttl),
			CreatedIP: clientIP,
			UserAgent: userAgent,
		}
		if terr := s.tokenRepo.CreateRefreshToken(ctx, refreshRecord); terr != nil {
			if errors.Is(terr, context.Canceled) || errors.Is(terr, context.DeadlineExceeded) {
				return "", terr
			}
			slog.Error("failed to persist refresh token", "user_id", userID, "error", terr)
			return "", domain.NewUserError(domain.ErrInternal, "could not persist refresh token")
		}
	}

	return rawRefreshToken, nil
}

func (s *authService) recordLoginAuditLog(ctx context.Context, user *models.User, identifier, clientIP, userAgent string) {
	if s.auditRepo == nil {
		return
	}
	detailsBytes, err := json.Marshal(map[string]interface{}{
		"identifier": identifier,
	})
	if err != nil {
		slog.Warn("failed to serialize login audit log details", "user_id", user.ID, "error", err)
	}
	entry := &models.AuditLog{
		ActorID:       &user.ID,
		ActorUsername: user.Username,
		Action:        "USER_LOGIN",
		ResourceType:  "user",
		ResourceID:    strconv.FormatUint(uint64(user.ID), 10),
		IPAddress:     clientIP,
		UserAgent:     userAgent,
		Details:       datatypes.JSON(detailsBytes),
	}
	if aerr := s.auditRepo.Create(ctx, entry); aerr != nil {
		slog.Warn("failed to record login audit log", "user_id", user.ID, "error", aerr)
	}
}

// RefreshToken validates an active refresh token, detects reuse/replay, revokes token families
// if compromised, and produces a new access token paired with a rotated refresh token.
func (s *authService) RefreshToken(ctx context.Context, dto RefreshTokenDTO) (*RefreshTokenResult, error) {
	if s.tokenRepo == nil || s.userRepo == nil || s.jwtSvc == nil {
		return nil, domain.NewUserError(domain.ErrInternal, "service unavailable")
	}

	cleanToken := strings.TrimSpace(dto.RefreshToken)
	if cleanToken == "" {
		return nil, domain.NewUserError(domain.ErrUnauthorized, "invalid refresh token")
	}

	tokenRecord, err := s.lookupRefreshToken(ctx, cleanToken, dto.ClientIP)
	if err != nil {
		return nil, err
	}

	if err := s.validateTokenRecord(ctx, tokenRecord, dto.ClientIP); err != nil {
		return nil, err
	}

	user, err := s.validateActiveUser(ctx, tokenRecord.UserID, dto.ClientIP)
	if err != nil {
		return nil, err
	}

	newAccessToken, err := s.jwtSvc.GenerateToken(user)
	if err != nil {
		slog.Error("failed to generate new access token", "user_id", user.ID, "error", err)
		return nil, domain.NewUserError(domain.ErrInternal, "could not issue token")
	}

	newRaw, newHash, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.Error("failed to generate new refresh token", "user_id", user.ID, "error", err)
		return nil, domain.NewUserError(domain.ErrInternal, "could not issue refresh token")
	}

	now := time.Now()
	if err := s.tokenRepo.RevokeRefreshToken(ctx, tokenRecord.ID, now); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		slog.Error("failed to revoke old refresh token during rotation", "token_id", tokenRecord.ID, "error", err)
		return nil, domain.NewUserError(domain.ErrInternal, "failed to rotate token")
	}

	if err := s.persistRotatedRefreshToken(ctx, user.ID, newHash, tokenRecord.FamilyID, dto.ClientIP, dto.UserAgent, now); err != nil {
		return nil, err
	}

	slog.Info("refresh token rotated successfully",
		"user_id", user.ID,
		"family_id", tokenRecord.FamilyID,
		"ip", dto.ClientIP,
	)

	return &RefreshTokenResult{
		AccessToken:  newAccessToken,
		RefreshToken: newRaw,
	}, nil
}

func (s *authService) lookupRefreshToken(ctx context.Context, cleanToken, clientIP string) (*models.RefreshToken, error) {
	hash := auth.HashToken(cleanToken)
	tokenRecord, err := s.tokenRepo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		slog.Warn("refresh token lookup failed", "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrUnauthorized, "invalid refresh token")
	}
	if tokenRecord == nil {
		slog.Warn("refresh token lookup failed", "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrUnauthorized, "invalid refresh token")
	}
	return tokenRecord, nil
}

func (s *authService) validateTokenRecord(ctx context.Context, tokenRecord *models.RefreshToken, clientIP string) error {
	if tokenRecord == nil {
		return domain.NewUserError(domain.ErrUnauthorized, "invalid refresh token")
	}
	if tokenRecord.RevokedAt != nil {
		slog.Warn("SECURITY ALERT: refresh token reuse detected; revoking family",
			"family_id", tokenRecord.FamilyID,
			"user_id", tokenRecord.UserID,
			"ip", clientIP,
		)
		if rerr := s.tokenRepo.RevokeFamily(ctx, tokenRecord.FamilyID, time.Now()); rerr != nil {
			slog.Error("failed to revoke token family on reuse detection",
				"family_id", tokenRecord.FamilyID,
				"error", rerr,
			)
		}
		return domain.NewUserError(domain.ErrUnauthorized, "token reuse detected, please sign in again")
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		slog.Warn("expired refresh token presented", "user_id", tokenRecord.UserID, "ip", clientIP)
		return domain.NewUserError(domain.ErrUnauthorized, "refresh token expired")
	}
	return nil
}

func (s *authService) validateActiveUser(ctx context.Context, userID uint, clientIP string) (*models.User, error) {
	user, uerr := s.userRepo.FindByID(ctx, userID)
	if uerr != nil {
		if errors.Is(uerr, context.Canceled) || errors.Is(uerr, context.DeadlineExceeded) {
			return nil, uerr
		}
		slog.Error("failed to find user during token refresh", "user_id", userID, "error", uerr)
		return nil, domain.NewUserError(domain.ErrInternal, "failed to validate user account")
	}
	if user == nil || !user.IsActive {
		slog.Warn("refresh attempt for deactivated user", "user_id", userID, "ip", clientIP)
		return nil, domain.NewUserError(domain.ErrUnauthorized, "Account is deactivated")
	}
	return user, nil
}

func (s *authService) persistRotatedRefreshToken(ctx context.Context, userID uint, tokenHash, familyID, clientIP, userAgent string, now time.Time) error {
	ttl := s.refreshTokenTTL
	if ttl <= 0 {
		ttl = defaultRefreshTokenTTL
	}
	newRecord := &models.RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		ExpiresAt: now.Add(ttl),
		CreatedIP: clientIP,
		UserAgent: userAgent,
	}
	if err := s.tokenRepo.CreateRefreshToken(ctx, newRecord); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		slog.Error("failed to persist rotated refresh token", "user_id", userID, "error", err)
		return domain.NewUserError(domain.ErrInternal, "could not persist refresh token")
	}
	return nil
}

// RevokeRefreshToken revokes a refresh token by hash during logout.
func (s *authService) RevokeRefreshToken(ctx context.Context, refreshToken, clientIP string) error {
	cleanToken := strings.TrimSpace(refreshToken)
	if cleanToken == "" || s.tokenRepo == nil {
		return nil
	}
	hash := auth.HashToken(cleanToken)
	tokenRecord, err := s.tokenRepo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		slog.Warn("failed to lookup refresh token on revoke", "error", err, "ip", clientIP)
		return fmt.Errorf("lookup refresh token: %w", err)
	}
	if tokenRecord == nil {
		return nil
	}
	if rerr := s.tokenRepo.RevokeRefreshToken(ctx, tokenRecord.ID, time.Now()); rerr != nil {
		if errors.Is(rerr, context.Canceled) || errors.Is(rerr, context.DeadlineExceeded) {
			return rerr
		}
		slog.Warn("failed to revoke refresh token on logout", "token_id", tokenRecord.ID, "error", rerr, "ip", clientIP)
		return fmt.Errorf("revoke refresh token: %w", rerr)
	}
	slog.Info("refresh token revoked on logout", "token_id", tokenRecord.ID, "user_id", tokenRecord.UserID, "ip", clientIP)
	return nil
}

// GetProfile retrieves a user's profile information by ID.
func (s *authService) GetProfile(ctx context.Context, userID uint) (*models.User, error) {
	if s.userRepo == nil {
		return nil, domain.NewUserError(domain.ErrInternal, "database repository unavailable")
	}
	if userID == 0 {
		return nil, domain.NewUserError(domain.ErrNotFound, "user not found")
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, domain.NewUserError(domain.ErrNotFound, "user not found")
	}
	if user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "user not found")
	}
	return user, nil
}
