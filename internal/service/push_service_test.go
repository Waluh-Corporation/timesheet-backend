package service_test

import (
	"context"
	"errors"
	"testing"

	"timesheet-backend/config"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
	"timesheet-backend/push"
)

type mockPushRepo struct {
	subscribed   []*models.PushSubscription
	unsubscribed map[string]bool
	subErr       error
	unsubErr     error
}

func (m *mockPushRepo) Subscribe(ctx context.Context, sub *models.PushSubscription) error {
	if m.subErr != nil {
		return m.subErr
	}
	m.subscribed = append(m.subscribed, sub)
	return nil
}

func (m *mockPushRepo) Unsubscribe(ctx context.Context, userID uint, endpoint string) error {
	if m.unsubErr != nil {
		return m.unsubErr
	}
	if m.unsubscribed == nil {
		m.unsubscribed = make(map[string]bool)
	}
	m.unsubscribed[endpoint] = true
	return nil
}

func (m *mockPushRepo) ListByUserID(ctx context.Context, userID uint) ([]models.PushSubscription, error) {
	return nil, nil
}

func (m *mockPushRepo) DeleteByID(ctx context.Context, id uint) error {
	return nil
}

func TestPushService_Subscribe(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		p256dh      string
		authKey     string
		repoErr     error
		expectError bool
		isInvalid   bool
	}{
		{
			name:        "successful subscription",
			endpoint:    "https://fcm.googleapis.com/fcm/send/123",
			p256dh:      "test-p256dh",
			authKey:     "test-auth",
			expectError: false,
		},
		{
			name:        "missing endpoint validation error",
			endpoint:    "",
			p256dh:      "test-p256dh",
			authKey:     "test-auth",
			expectError: true,
			isInvalid:   true,
		},
		{
			name:        "repository error wrapped",
			endpoint:    "https://example.com/push",
			p256dh:      "key",
			authKey:     "auth",
			repoErr:     errors.New("db error"),
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockPushRepo{subErr: tc.repoErr}
			cfg := &config.Config{VAPIDPublicKey: "pub-key", VAPIDPrivateKey: "priv-key"}
			dispatcher := push.New(cfg, nil)
			svc := service.NewPushService(repo, dispatcher)

			if pk := svc.GetPublicKey(); pk != "pub-key" {
				t.Fatalf("expected public key %q, got %q", "pub-key", pk)
			}

			err := svc.Subscribe(context.Background(), 1, tc.endpoint, tc.p256dh, tc.authKey)
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.isInvalid && !errors.Is(err, domain.ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPushService_Unsubscribe(t *testing.T) {
	t.Run("successful unsubscribe", func(t *testing.T) {
		repo := &mockPushRepo{}
		svc := service.NewPushService(repo, nil)

		err := svc.Unsubscribe(context.Background(), 1, "https://push")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.unsubscribed["https://push"] {
			t.Fatal("expected endpoint to be recorded in unsubscribed")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &mockPushRepo{unsubErr: errors.New("db error")}
		svc := service.NewPushService(repo, nil)

		err := svc.Unsubscribe(context.Background(), 1, "https://push")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("send test push does not panic", func(t *testing.T) {
		repo := &mockPushRepo{}
		svc := service.NewPushService(repo, nil)
		err := svc.SendTestPush(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("GetPublicKey with nil dispatcher", func(t *testing.T) {
		svc := service.NewPushService(&mockPushRepo{}, nil)
		if key := svc.GetPublicKey(); key != "" {
			t.Fatalf("expected empty string, got %s", key)
		}
	})
}
