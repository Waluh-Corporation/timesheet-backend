package service

import (
	"context"
	"fmt"

	"timesheet-backend/dto/response"
	"timesheet-backend/internal/repository"
	"timesheet-backend/services"
)

// AuthenticatorService defines operations for managing passkey authenticator registry.
type AuthenticatorService interface {
	SyncCommunityAuthenticators(ctx context.Context) (int, error)
	ListAuthenticators(ctx context.Context, search string, page, limit int) ([]response.AuthenticatorItemResponse, int64, error)
}

type authenticatorService struct {
	repo repository.AuthenticatorRepository
}

// NewAuthenticatorService constructs an instance of AuthenticatorService.
func NewAuthenticatorService(repo repository.AuthenticatorRepository) AuthenticatorService {
	return &authenticatorService{repo: repo}
}

func (s *authenticatorService) SyncCommunityAuthenticators(ctx context.Context) (int, error) {
	entries, err := services.FetchCommunityAAGUIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch community authenticators: %w", err)
	}

	total, err := s.repo.SyncAAGUIDs(ctx, entries)
	if err != nil {
		return 0, fmt.Errorf("failed to sync authenticators to repository: %w", err)
	}
	return total, nil
}

func (s *authenticatorService) ListAuthenticators(ctx context.Context, search string, page, limit int) ([]response.AuthenticatorItemResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	records, total, err := s.repo.ListAuthenticators(ctx, search, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list authenticators: %w", err)
	}

	items := make([]response.AuthenticatorItemResponse, len(records))
	for i, r := range records {
		items[i] = response.AuthenticatorItemResponse{
			AAGUID:    r.AAGUID,
			Name:      r.Name,
			Icon:      r.Icon,
			UpdatedAt: r.UpdatedAt,
		}
	}

	return items, total, nil
}
