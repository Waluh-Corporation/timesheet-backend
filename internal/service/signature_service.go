package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
	"timesheet-backend/services"
	"timesheet-backend/storage"
)

// SignatureService defines business operations for managing DocuSeal e-signature specimens.
type SignatureService interface {
	GetStatus(ctx context.Context, userID uint) (*response.SignatureStatusResponse, error)
	CreateSession(ctx context.Context, userID uint) (*response.SignatureSessionResponse, error)
	SyncSignature(ctx context.Context, userID uint, req *request.SyncSignatureRequest) (*response.SignatureStatusResponse, error)
	HandleWebhook(ctx context.Context, payload *request.DocusealWebhookPayload) error
	RevokeSignature(ctx context.Context, userID uint) error
	GetActiveSignaturePNG(ctx context.Context, userID uint) ([]byte, error)
}

type signatureService struct {
	cfg            *config.Config
	sigRepo        repository.SignatureRepository
	userRepo       repository.UserRepository
	docusealClient services.DocusealClient
	storage        storage.StorageService
}

// NewSignatureService constructs a SignatureService implementation.
func NewSignatureService(
	cfg *config.Config,
	sigRepo repository.SignatureRepository,
	userRepo repository.UserRepository,
	docusealClient services.DocusealClient,
	storageSvc storage.StorageService,
) SignatureService {
	return &signatureService{
		cfg:            cfg,
		sigRepo:        sigRepo,
		userRepo:       userRepo,
		docusealClient: docusealClient,
		storage:        storageSvc,
	}
}

func (s *signatureService) defaultSignatureTextForUser(user *models.User) string {
	if user == nil {
		return ""
	}
	fullName := strings.TrimSpace(user.Name)
	if fullName == "" {
		fullName = strings.TrimSpace(user.Username)
	}
	return services.FormatSignatureShortName(fullName)
}

func (s *signatureService) isDocusealConfigured() bool {
	return s.cfg != nil &&
		s.cfg.DocusealEnabled &&
		s.docusealClient != nil &&
		s.docusealClient.IsEnabled()
}

func (s *signatureService) GetStatus(ctx context.Context, userID uint) (*response.SignatureStatusResponse, error) {
	if s.userRepo == nil || s.sigRepo == nil {
		return nil, fmt.Errorf("signature service dependencies not initialized")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "User not found")
	}

	defaultText := s.defaultSignatureTextForUser(user)
	enabled := s.cfg != nil && s.cfg.DocusealEnabled

	active, err := s.sigRepo.FindActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active signature: %w", err)
	}

	if active != nil {
		var previewURL string
		if s.storage != nil && active.S3SignatureKey != "" {
			if url, uerr := s.storage.GetPresignedDownloadURL(ctx, active.S3SignatureKey, 15*time.Minute); uerr == nil {
				previewURL = url
			}
		}
		updatedAt := active.UpdatedAt
		return &response.SignatureStatusResponse{
			DocusealEnabled:      enabled,
			HasActiveSignature:   true,
			Status:               string(active.Status),
			SignatureType:        string(active.SignatureType),
			DefaultSignatureText: defaultText,
			PreviewURL:           previewURL,
			SignedAt:             active.SignedAt,
			UpdatedAt:            &updatedAt,
		}, nil
	}

	pending, _ := s.sigRepo.FindLatestPendingByUserID(ctx, userID)
	statusStr := "none"
	var updatedAt *time.Time
	if pending != nil {
		statusStr = string(pending.Status)
		t := pending.UpdatedAt
		updatedAt = &t
	}

	return &response.SignatureStatusResponse{
		DocusealEnabled:      enabled,
		HasActiveSignature:   false,
		Status:               statusStr,
		DefaultSignatureText: defaultText,
		UpdatedAt:            updatedAt,
	}, nil
}

func (s *signatureService) CreateSession(ctx context.Context, userID uint) (*response.SignatureSessionResponse, error) {
	if !s.isDocusealConfigured() {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "DocuSeal e-signature integration is not enabled or configured")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "User not found")
	}

	fullName := strings.TrimSpace(user.Name)
	if fullName == "" {
		fullName = strings.TrimSpace(user.Username)
	}
	email := strings.TrimSpace(user.Email)
	if email == "" {
		email = fmt.Sprintf("%s@timesheet.local", user.Username)
	}

	submitter, err := s.docusealClient.CreateSpecimenSubmission(ctx, user.ID, fullName, email, user.BniID, user.Division)
	if err != nil {
		return nil, fmt.Errorf("failed to create DocuSeal signing session: %w", err)
	}

	sig := &models.UserSignature{
		UserID:               user.ID,
		DocusealSubmissionID: submitter.SubmissionID,
		DocusealSubmitterID:  submitter.ID,
		DocusealSlug:         submitter.Slug,
		Status:               models.SignatureStatusPending,
		IsActive:             false,
	}
	if err := s.sigRepo.Create(ctx, sig); err != nil {
		return nil, fmt.Errorf("failed to store signature session: %w", err)
	}

	return &response.SignatureSessionResponse{
		SubmissionID:         submitter.SubmissionID,
		SubmitterID:          submitter.ID,
		Slug:                 submitter.Slug,
		EmbedSrc:             submitter.EmbedSrc,
		DefaultSignatureText: s.defaultSignatureTextForUser(user),
	}, nil
}

func (s *signatureService) SyncSignature(ctx context.Context, userID uint, req *request.SyncSignatureRequest) (*response.SignatureStatusResponse, error) {
	if s.docusealClient == nil || (s.cfg != nil && !s.cfg.DocusealEnabled) {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "DocuSeal e-signature integration is not enabled")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "User not found")
	}

	sigRecord, submitterID, err := s.resolveTargetSyncRecord(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	submitter, err := s.docusealClient.GetSubmitter(ctx, submitterID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch submitter from DocuSeal: %w", err)
	}

	expectedExtID := fmt.Sprintf("user-%d", userID)
	if sigRecord == nil && submitter.ExternalID != "" && submitter.ExternalID != expectedExtID {
		return nil, domain.NewUserError(domain.ErrForbidden, "Signature session does not belong to the authenticated user")
	}

	if err := s.processSubmitterSpecimen(ctx, user, sigRecord, submitter); err != nil {
		return nil, err
	}

	return s.GetStatus(ctx, userID)
}

func (s *signatureService) resolveTargetSyncRecord(ctx context.Context, userID uint, req *request.SyncSignatureRequest) (*models.UserSignature, int64, error) {
	if req != nil && req.SubmitterID > 0 {
		rec, err := s.sigRepo.FindBySubmitterID(ctx, req.SubmitterID)
		if err != nil {
			return nil, 0, err
		}
		if rec != nil && rec.UserID != userID {
			return nil, 0, domain.NewUserError(domain.ErrForbidden, "Signature session does not belong to the authenticated user")
		}
		return rec, req.SubmitterID, nil
	}

	if req != nil && strings.TrimSpace(req.Slug) != "" {
		rec, err := s.sigRepo.FindBySlugAndUserID(ctx, strings.TrimSpace(req.Slug), userID)
		if err != nil {
			return nil, 0, err
		}
		if rec != nil && rec.DocusealSubmitterID > 0 {
			return rec, rec.DocusealSubmitterID, nil
		}
	}

	pending, err := s.sigRepo.FindLatestPendingByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	if pending != nil && pending.DocusealSubmitterID > 0 {
		return pending, pending.DocusealSubmitterID, nil
	}

	active, err := s.sigRepo.FindActiveByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	if active != nil && active.DocusealSubmitterID > 0 {
		return active, active.DocusealSubmitterID, nil
	}

	return nil, 0, domain.NewUserError(domain.ErrNotFound, "No pending signature session found to synchronize")
}

func (s *signatureService) HandleWebhook(ctx context.Context, payload *request.DocusealWebhookPayload) error {
	if payload == nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Empty webhook payload")
	}

	eventType := strings.ToLower(strings.TrimSpace(payload.EventType))
	if eventType != "form.completed" && eventType != "submission.completed" {
		return nil
	}

	submitterID := payload.Data.ID
	sigRecord, _ := s.sigRepo.FindBySubmitterID(ctx, submitterID)

	var userID uint
	if sigRecord != nil {
		userID = sigRecord.UserID
	} else if strings.HasPrefix(payload.Data.ExternalID, "user-") {
		if parsed, err := strconv.ParseUint(strings.TrimPrefix(payload.Data.ExternalID, "user-"), 10, 64); err == nil {
			userID = uint(parsed)
		}
	}

	if userID == 0 {
		slog.Warn("received docuseal webhook for unmapped submitter",
			slog.Int64("submitter_id", submitterID),
			slog.String("external_id", payload.Data.ExternalID))
		return nil
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user %d for webhook: %w", userID, err)
	}
	if user == nil {
		return nil
	}

	values := make([]services.DocusealSubmitterValue, 0, len(payload.Data.Values))
	for _, v := range payload.Data.Values {
		values = append(values, services.DocusealSubmitterValue{
			Field: v.Field,
			Value: v.Value,
		})
	}

	submitter := &services.DocusealSubmitterResponse{
		ID:           payload.Data.ID,
		SubmissionID: payload.Data.SubmissionID,
		UUID:         payload.Data.UUID,
		Email:        payload.Data.Email,
		Slug:         payload.Data.Slug,
		Name:         payload.Data.Name,
		Status:       payload.Data.Status,
		ExternalID:   payload.Data.ExternalID,
		Values:       values,
	}
	if submitter.SubmissionID == 0 && payload.Data.Submission != nil {
		submitter.SubmissionID = payload.Data.Submission.ID
	}

	if len(submitter.Values) == 0 && s.docusealClient != nil && submitterID > 0 {
		if fetched, ferr := s.docusealClient.GetSubmitter(ctx, submitterID); ferr == nil && fetched != nil {
			submitter = fetched
		}
	}

	return s.processSubmitterSpecimen(ctx, user, sigRecord, submitter)
}

func (s *signatureService) processSubmitterSpecimen(
	ctx context.Context,
	user *models.User,
	sigRecord *models.UserSignature,
	submitter *services.DocusealSubmitterResponse,
) error {
	if submitter == nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Submitter details are missing")
	}

	fieldName := "Signature"
	if s.cfg != nil && strings.TrimSpace(s.cfg.DocusealSignatureFieldName) != "" {
		fieldName = s.cfg.DocusealSignatureFieldName
	}

	rawVal, isImageURL, isDataURI := services.ExtractSignatureValue(submitter.Values, fieldName)
	if rawVal == "" && strings.ToLower(submitter.Status) != "completed" {
		return domain.NewUserError(domain.ErrInvalidInput, "Signature session has not been completed yet")
	}

	pngBytes, sigType, sigText, err := s.buildTransparentSignaturePNG(ctx, user, rawVal, isImageURL, isDataURI)
	if err != nil {
		return fmt.Errorf("failed to process signature image: %w", err)
	}

	if s.storage == nil {
		return fmt.Errorf("object storage service is not initialized")
	}

	var prevKey string
	if prevActive, _ := s.sigRepo.FindActiveByUserID(ctx, user.ID); prevActive != nil {
		prevKey = prevActive.S3SignatureKey
	}

	objectKey := fmt.Sprintf("signatures/%d/specimen_%s.png", user.ID, uuid.New().String())
	if err := s.storage.Upload(ctx, objectKey, bytes.NewReader(pngBytes), "image/png"); err != nil {
		return fmt.Errorf("failed to upload signature PNG to storage: %w", err)
	}

	now := time.Now()
	if sigRecord == nil {
		sigRecord = &models.UserSignature{
			UserID:               user.ID,
			DocusealSubmissionID: submitter.SubmissionID,
			DocusealSubmitterID:  submitter.ID,
			DocusealSlug:         submitter.Slug,
		}
	}
	sigRecord.S3SignatureKey = objectKey
	sigRecord.SignatureType = sigType
	sigRecord.SignatureText = sigText
	sigRecord.Status = models.SignatureStatusCompleted
	sigRecord.IsActive = true
	sigRecord.SignedAt = &now

	if err := s.sigRepo.ActivateSpecimen(ctx, sigRecord); err != nil {
		return fmt.Errorf("failed to activate signature specimen: %w", err)
	}

	if prevKey != "" && prevKey != objectKey {
		_ = s.storage.Delete(ctx, prevKey)
	}

	return nil
}

func (s *signatureService) buildTransparentSignaturePNG(
	ctx context.Context,
	user *models.User,
	rawVal string,
	isImageURL bool,
	isDataURI bool,
) ([]byte, models.SignatureType, string, error) {
	trimmed := strings.TrimSpace(rawVal)
	if trimmed == "" {
		shortName := s.defaultSignatureTextForUser(user)
		pngBytes, err := services.RenderTypedSignaturePNG(shortName)
		return pngBytes, models.SignatureTypeTyped, shortName, err
	}

	if isDataURI || strings.HasPrefix(trimmed, "data:image/") {
		rawImg, ok := services.DecodeDataURISignature(trimmed)
		if !ok {
			return nil, "", "", fmt.Errorf("invalid base64 data URI signature image")
		}
		pngBytes, err := services.NormalizeSignaturePNG(rawImg)
		return pngBytes, models.SignatureTypeDrawn, "", err
	}

	if isImageURL || strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "/") {
		if s.docusealClient == nil {
			return nil, "", "", fmt.Errorf("docuseal client unavailable to download signature asset")
		}
		rawImg, err := s.docusealClient.DownloadAsset(ctx, trimmed)
		if err != nil {
			return nil, "", "", err
		}
		pngBytes, err := services.NormalizeSignaturePNG(rawImg)
		return pngBytes, models.SignatureTypeDrawn, "", err
	}

	// Plain text string from a Typed signature in DocuSeal
	pngBytes, err := services.RenderTypedSignaturePNG(trimmed)
	return pngBytes, models.SignatureTypeTyped, trimmed, err
}

func (s *signatureService) RevokeSignature(ctx context.Context, userID uint) error {
	revoked, err := s.sigRepo.RevokeActiveByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke signature: %w", err)
	}
	if revoked == nil {
		return domain.NewUserError(domain.ErrNotFound, "No active signature specimen found")
	}
	if s.storage != nil && revoked.S3SignatureKey != "" {
		_ = s.storage.Delete(ctx, revoked.S3SignatureKey)
	}
	return nil
}

func (s *signatureService) GetActiveSignaturePNG(ctx context.Context, userID uint) ([]byte, error) {
	if s.sigRepo == nil || s.storage == nil || userID == 0 {
		return nil, nil
	}
	active, err := s.sigRepo.FindActiveByUserID(ctx, userID)
	if err != nil || active == nil || active.S3SignatureKey == "" {
		return nil, err
	}
	return s.storage.Download(ctx, active.S3SignatureKey)
}
