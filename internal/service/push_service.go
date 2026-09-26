package service

import (
	"context"
	"fmt"

	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
	"timesheet-backend/push"
)

// PushService defines operations for web push notification subscriptions.
type PushService interface {
	GetPublicKey() string
	Subscribe(ctx context.Context, userID uint, endpoint, p256dh, auth string) error
	Unsubscribe(ctx context.Context, userID uint, endpoint string) error
	SendTestPush(ctx context.Context, userID uint) error
}

type pushService struct {
	repo       repository.PushRepository
	dispatcher *push.Service
}

// NewPushService constructs an instance of PushService.
func NewPushService(repo repository.PushRepository, dispatcher *push.Service) PushService {
	return &pushService{
		repo:       repo,
		dispatcher: dispatcher,
	}
}

func (s *pushService) GetPublicKey() string {
	if s.dispatcher == nil {
		return ""
	}
	return s.dispatcher.PublicKey()
}

func (s *pushService) Subscribe(ctx context.Context, userID uint, endpoint, p256dh, auth string) error {
	if endpoint == "" || p256dh == "" || auth == "" {
		return domain.NewUserError(domain.ErrInvalidInput, "invalid subscription keys or endpoint")
	}
	sub := &models.PushSubscription{
		UserID:   userID,
		Endpoint: endpoint,
		P256dh:   p256dh,
		Auth:     auth,
	}
	if err := s.repo.Subscribe(ctx, sub); err != nil {
		return fmt.Errorf("failed to save push subscription: %w", err)
	}
	return nil
}

func (s *pushService) Unsubscribe(ctx context.Context, userID uint, endpoint string) error {
	if err := s.repo.Unsubscribe(ctx, userID, endpoint); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}
	return nil
}

func (s *pushService) SendTestPush(ctx context.Context, userID uint) error {
	if s.dispatcher != nil {
		s.dispatcher.SendToUser(userID, push.Payload{
			Title: "Timesheet Portal",
			Body:  "Waktunya isi timesheet hari ini!",
			URL:   "/activity",
		})
	}
	return nil
}
