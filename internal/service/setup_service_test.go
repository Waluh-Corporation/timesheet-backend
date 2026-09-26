package service_test

import (
	"context"
	"errors"
	"testing"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

type mockSetupRepo struct {
	adminCount int64
	setting    string
	countErr   error
	settingErr error
	executeErr error
}

func (m *mockSetupRepo) GetAdminCount(ctx context.Context) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.adminCount, nil
}

func (m *mockSetupRepo) GetSystemSetting(ctx context.Context, key string) (string, error) {
	if m.settingErr != nil {
		return "", m.settingErr
	}
	return m.setting, nil
}

func (m *mockSetupRepo) ExecuteSetup(ctx context.Context, adminUser *models.User, companies []models.Company, depts []models.Department, approvers []models.Approver) error {
	if m.executeErr != nil {
		return m.executeErr
	}
	adminUser.ID = 1
	return nil
}

func TestSetupService_GetSetupStatus(t *testing.T) {
	tests := []struct {
		name          string
		adminCount    int64
		setting       string
		settingErr    error
		countErr      error
		expectedIsNew string
		expectedInit  bool
		expectedErr   bool
	}{
		{
			name:          "fresh database requires setup",
			adminCount:    0,
			setting:       "Y",
			expectedIsNew: "Y",
			expectedInit:  false,
		},
		{
			name:          "setting is N, initialized",
			adminCount:    1,
			setting:       "N",
			expectedIsNew: "N",
			expectedInit:  true,
		},
		{
			name:          "no setting but adminCount > 0 defaults to initialized",
			adminCount:    1,
			settingErr:    errors.New("not found"),
			expectedIsNew: "N",
			expectedInit:  true,
		},
		{
			name:        "repo count error propagates",
			countErr:    errors.New("connection failed"),
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockSetupRepo{
				adminCount: tc.adminCount,
				setting:    tc.setting,
				settingErr: tc.settingErr,
				countErr:   tc.countErr,
			}
			svc := service.NewSetupService(repo, nil)

			status, err := svc.GetSetupStatus(context.Background())
			if tc.expectedErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.IsNew != tc.expectedIsNew || status.IsInitialized != tc.expectedInit {
				t.Fatalf("mismatched status: %+v", status)
			}
		})
	}
}

func TestSetupService_InitSetup(t *testing.T) {
	validReq := &request.InitSetupRequest{
		Admin: request.InitSetupAdminRequest{
			Username: "superadmin",
			Email:    "superadmin@example.com",
			Name:     "Super Admin",
			Password: "Xk9#mQ2$vL7!zR4@",
		},
		Companies: []request.InitSetupCompanyRequest{
			{Code: "mii", Name: "MII"},
		},
	}

	t.Run("successfully initializes setup", func(t *testing.T) {
		repo := &mockSetupRepo{adminCount: 0, setting: "Y"}
		authSvc := auth.NewService("a-very-long-secret-key-that-is-at-least-32-bytes-long", 24)
		svc := service.NewSetupService(repo, authSvc)

		admin, token, err := svc.InitSetup(context.Background(), validReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if admin == nil || admin.Username != "superadmin" {
			t.Fatalf("unexpected admin user: %+v", admin)
		}
		if token == "" {
			t.Fatal("expected token to be generated")
		}
	})

	t.Run("fails when already initialized", func(t *testing.T) {
		repo := &mockSetupRepo{adminCount: 1, setting: "N"}
		svc := service.NewSetupService(repo, nil)

		_, _, err := svc.InitSetup(context.Background(), validReq)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("fails on weak admin password", func(t *testing.T) {
		repo := &mockSetupRepo{adminCount: 0, setting: "Y"}
		svc := service.NewSetupService(repo, nil)

		weakReq := *validReq
		weakReq.Admin.Password = "weak"
		_, _, err := svc.InitSetup(context.Background(), &weakReq)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("fails on repo execute error", func(t *testing.T) {
		repo := &mockSetupRepo{adminCount: 0, setting: "Y", executeErr: errors.New("db error")}
		svc := service.NewSetupService(repo, nil)
		_, _, err := svc.InitSetup(context.Background(), validReq)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("fails on get status error", func(t *testing.T) {
		repo := &mockSetupRepo{countErr: errors.New("db count error")}
		svc := service.NewSetupService(repo, nil)
		_, _, err := svc.InitSetup(context.Background(), validReq)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
