package service

import (
	"context"
	"fmt"
	"strings"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

// SetupService defines application logic for system onboarding setup.
type SetupService interface {
	GetSetupStatus(ctx context.Context) (*response.SetupStatusResponse, error)
	InitSetup(ctx context.Context, req *request.InitSetupRequest) (*models.User, string, error)
}

type setupService struct {
	repo    repository.SetupRepository
	authSvc *auth.Service
}

// NewSetupService constructs a SetupService.
func NewSetupService(repo repository.SetupRepository, authSvc *auth.Service) SetupService {
	return &setupService{
		repo:    repo,
		authSvc: authSvc,
	}
}

func (s *setupService) GetSetupStatus(ctx context.Context) (*response.SetupStatusResponse, error) {
	adminCount, err := s.repo.GetAdminCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check admin count: %w", err)
	}

	settingVal, err := s.repo.GetSystemSetting(ctx, "is_new")
	isNew := "Y"
	if err == nil {
		isNew = strings.ToUpper(strings.TrimSpace(settingVal))
	} else if adminCount > 0 {
		isNew = "N"
	}

	return &response.SetupStatusResponse{
		IsNew:         isNew,
		IsInitialized: isNew == "N",
		RequiresSetup: isNew == "Y",
		AdminCount:    adminCount,
	}, nil
}

func (s *setupService) InitSetup(ctx context.Context, req *request.InitSetupRequest) (*models.User, string, error) {
	status, err := s.GetSetupStatus(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to verify setup status: %w", err)
	}
	if status.IsInitialized {
		return nil, "", domain.NewUserError(domain.ErrForbidden, "system is already initialized")
	}

	if err := auth.ValidatePassword(req.Admin.Password, req.Admin.Username, req.Admin.Email); err != nil {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "admin password policy violation: "+err.Error())
	}

	hash, err := auth.HashPassword(req.Admin.Password)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash admin password: %w", err)
	}

	adminUser := &models.User{
		Username:     strings.TrimSpace(req.Admin.Username),
		Email:        strings.TrimSpace(req.Admin.Email),
		Name:         strings.TrimSpace(req.Admin.Name),
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
		IsActive:     true,
	}

	companies := make([]models.Company, 0, len(req.Companies))
	for _, cr := range req.Companies {
		companies = append(companies, models.Company{
			Code:     cr.Code,
			Name:     cr.Name,
			IsActive: true,
		})
	}

	depts := make([]models.Department, 0, len(req.Departments))
	for _, dr := range req.Departments {
		depts = append(depts, models.Department{
			Code:     dr.Code,
			Name:     dr.Name,
			Division: dr.Division,
			IsActive: true,
		})
	}

	approvers := make([]models.Approver, 0, len(req.Approvers))
	for _, ar := range req.Approvers {
		approvers = append(approvers, models.Approver{
			Name:     ar.Name,
			RoleType: ar.RoleType,
			Title:    ar.Title,
			IsActive: true,
		})
	}

	if err := s.repo.ExecuteSetup(ctx, adminUser, companies, depts, approvers); err != nil {
		return nil, "", fmt.Errorf("failed to execute setup transaction: %w", err)
	}

	var token string
	if s.authSvc != nil {
		t, err := s.authSvc.GenerateToken(adminUser)
		if err != nil {
			return nil, "", fmt.Errorf("failed to issue session token: %w", err)
		}
		token = t
	}

	return adminUser, token, nil
}
