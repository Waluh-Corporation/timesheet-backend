package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
	"timesheet-backend/services"
	"timesheet-backend/storage"
)

// AuditMeta captures request metadata for signing audit trail records.
type AuditMeta struct {
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
}

const (
	errUserNotFoundMsg  = "Pengguna tidak ditemukan"
	errInvalidPeriodMsg = "Bulan atau tahun tidak valid"
	beritaAcaraPath     = "/berita-acara"
)

// SignatureService provides business operations for profile signature management,
// document signing lifecycles (Timesheet & Berita Acara), and public verification.
type SignatureService interface {
	// Profile signature
	UploadSignature(ctx context.Context, userID uint, base64Str string) (*response.UserSignatureResponse, error)
	GetSignature(ctx context.Context, userID uint) (*response.UserSignatureResponse, error)
	DeleteSignature(ctx context.Context, userID uint) error

	// Timesheet signing
	SignTimesheet(ctx context.Context, userID uint, req *request.SignTimesheetRequest, meta AuditMeta) (*response.TimesheetSubmissionResponse, error)
	RevokeTimesheet(ctx context.Context, userID uint, req *request.RevokeTimesheetRequest, meta AuditMeta) error
	GetTimesheetSubmission(ctx context.Context, userID uint, month, year int) (*response.TimesheetSubmissionResponse, error)

	// Berita Acara signing
	SubmitBeritaAcara(ctx context.Context, userID uint, req *request.SubmitBeritaAcaraRequest, meta AuditMeta) (*response.BeritaAcaraSubmissionResponse, error)
	WitnessAction(ctx context.Context, witnessID uint, submissionID string, req *request.WitnessActionRequest, meta AuditMeta) (*response.BeritaAcaraSubmissionResponse, error)
	RevokeBeritaAcara(ctx context.Context, userID uint, submissionID string, meta AuditMeta) error
	GetBeritaAcaraSubmission(ctx context.Context, userID uint, month, year int) (*response.BeritaAcaraSubmissionResponse, error)
	ListPendingWitnessRequests(ctx context.Context, witnessID uint) ([]response.BeritaAcaraSubmissionResponse, error)

	// Public Verification
	VerifyDocument(ctx context.Context, token string) (*response.PublicDocumentVerificationResponse, error)
}

type signatureService struct {
	userRepo       repository.UserRepository
	activityRepo   repository.ActivityRepository
	submissionRepo repository.SubmissionRepository
	storageSvc     storage.StorageService
	mailer         *mailer.Mailer
	pushSvc        *push.Service
	baseURL        string
	nowFunc        func() time.Time
}

// NewSignatureService constructs a SignatureService implementation.
func NewSignatureService(
	userRepo repository.UserRepository,
	activityRepo repository.ActivityRepository,
	submissionRepo repository.SubmissionRepository,
	storageSvc storage.StorageService,
	m *mailer.Mailer,
	p *push.Service,
	baseURL string,
) SignatureService {
	return &signatureService{
		userRepo:       userRepo,
		activityRepo:   activityRepo,
		submissionRepo: submissionRepo,
		storageSvc:     storageSvc,
		mailer:         m,
		pushSvc:        p,
		baseURL:        baseURL,
		nowFunc:        time.Now,
	}
}

// SetNowFunc overrides the clock for testing.
func (s *signatureService) SetNowFunc(fn func() time.Time) {
	s.nowFunc = fn
}

func (s *signatureService) verificationURL(token string) string {
	base := strings.TrimRight(s.baseURL, "/")
	if base == "" {
		base = "https://timesheet.local"
	}
	return fmt.Sprintf("%s/verify/doc/%s", base, token)
}

// UploadSignature saves a transparent PNG signature to storage and links it to the user.
func (s *signatureService) UploadSignature(ctx context.Context, userID uint, base64Str string) (*response.UserSignatureResponse, error) {
	b64 := strings.TrimSpace(base64Str)
	if idx := strings.Index(b64, ","); idx != -1 {
		b64 = b64[idx+1:]
	}
	if b64 == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Data gambar tanda tangan tidak boleh kosong")
	}

	imgBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Format base64 tanda tangan tidak valid")
	}

	const maxSignatureBytes = 2 * 1024 * 1024 // 2MB
	if len(imgBytes) > maxSignatureBytes {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Ukuran gambar tanda tangan maksimal 2MB")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Format file bukan gambar yang didukung (gunakan PNG)")
	}
	if format != "png" && format != "jpeg" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Format gambar harus PNG atau JPEG")
	}
	if cfg.Width < 20 || cfg.Height < 20 {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Dimensi tanda tangan terlalu kecil")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, errUserNotFoundMsg)
	}

	key := fmt.Sprintf("signatures/%d_%d.png", userID, s.nowFunc().Unix())
	if s.storageSvc != nil {
		if err := s.storageSvc.Upload(ctx, key, bytes.NewReader(imgBytes), "image/png"); err != nil {
			return nil, fmt.Errorf("failed to upload signature: %w", err)
		}
	}

	// Update user record
	if err := s.userRepo.UpdateSignatureFileKey(ctx, userID, key); err != nil {
		return nil, fmt.Errorf("failed to update user signature key: %w", err)
	}

	sigURL := ""
	if s.storageSvc != nil {
		sigURL, _ = s.storageSvc.GetPresignedDownloadURL(ctx, key, 1*time.Hour)
	}

	return &response.UserSignatureResponse{
		HasSignature: true,
		SignatureURL: sigURL,
	}, nil
}

// GetSignature returns the presigned URL for the user's signature.
func (s *signatureService) GetSignature(ctx context.Context, userID uint) (*response.UserSignatureResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, errUserNotFoundMsg)
	}

	if user.SignatureFileKey == "" {
		return &response.UserSignatureResponse{HasSignature: false}, nil
	}

	sigURL := ""
	if s.storageSvc != nil {
		sigURL, _ = s.storageSvc.GetPresignedDownloadURL(ctx, user.SignatureFileKey, 1*time.Hour)
	}

	return &response.UserSignatureResponse{
		HasSignature: true,
		SignatureURL: sigURL,
	}, nil
}

// DeleteSignature removes the user's stored signature.
func (s *signatureService) DeleteSignature(ctx context.Context, userID uint) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return domain.NewUserError(domain.ErrNotFound, errUserNotFoundMsg)
	}

	if user.SignatureFileKey != "" && s.storageSvc != nil {
		_ = s.storageSvc.Delete(ctx, user.SignatureFileKey)
	}

	if err := s.userRepo.UpdateSignatureFileKey(ctx, userID, ""); err != nil {
		return fmt.Errorf("failed to clear signature key: %w", err)
	}
	return nil
}

// SignTimesheet signs the timesheet for the given month/year and locks activities.
func (s *signatureService) SignTimesheet(ctx context.Context, userID uint, req *request.SignTimesheetRequest, meta AuditMeta) (*response.TimesheetSubmissionResponse, error) {
	if req == nil || req.Month < 1 || req.Month > 12 || req.Year < 2000 || req.Year > 2100 {
		return nil, domain.NewUserError(domain.ErrInvalidInput, errInvalidPeriodMsg)
	}

	user, err := s.userRepo.FindByIDWithDetails(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, errUserNotFoundMsg)
	}

	if user.SignatureFileKey == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Silakan unggah tanda tangan profil terlebih dahulu sebelum menandatangani timesheet")
	}

	// Verify activities exist
	filter := repository.ActivityFilter{
		Month: &req.Month,
		Year:  &req.Year,
		IsAll: true,
	}
	activities, _, err := s.activityRepo.ListActiveByUser(ctx, userID, filter)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	if len(activities) == 0 {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Tidak ada aktivitas yang tercatat untuk periode ini")
	}

	token := uuid.New().String()
	now := s.nowFunc()

	// Compute SHA-256 fingerprint
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "type:timesheet|uid:%d|month:%d|year:%d|acts:%d|token:%s|time:%d",
		userID, req.Month, req.Year, len(activities), token, now.Unix())
	docHash := hex.EncodeToString(h.Sum(nil))

	metaBytes, _ := json.Marshal(map[string]interface{}{
		"ip":         meta.IP,
		"user_agent": meta.UserAgent,
		"user_name":  user.Name,
		"bni_id":     user.BniID,
	})

	sub := &models.TimesheetSubmission{
		ID:                uuid.New().String(),
		UserID:            userID,
		Month:             req.Month,
		Year:              req.Year,
		Status:            models.SubmissionStatusSigned,
		SignedAt:          &now,
		VerificationToken: token,
		DocumentHash:      docHash,
		AuditMetadata:     datatypes.JSON(metaBytes),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.submissionRepo.UpsertTimesheetSubmission(ctx, sub); err != nil {
		return nil, fmt.Errorf("save timesheet submission: %w", err)
	}

	return &response.TimesheetSubmissionResponse{
		ID:                sub.ID,
		UserID:            sub.UserID,
		Month:             sub.Month,
		Year:              sub.Year,
		Status:            sub.Status,
		SignedAt:          sub.SignedAt,
		VerificationToken: sub.VerificationToken,
		DocumentHash:      sub.DocumentHash,
		VerificationURL:   s.verificationURL(sub.VerificationToken),
	}, nil
}

// RevokeTimesheet revokes a timesheet's signature and unlocks the activities.
func (s *signatureService) RevokeTimesheet(ctx context.Context, userID uint, req *request.RevokeTimesheetRequest, meta AuditMeta) error {
	if req == nil || req.Month < 1 || req.Month > 12 || req.Year < 2000 || req.Year > 2100 {
		return domain.NewUserError(domain.ErrInvalidInput, errInvalidPeriodMsg)
	}

	sub, err := s.submissionRepo.FindTimesheetSubmission(ctx, userID, req.Month, req.Year)
	if err != nil {
		return fmt.Errorf("find timesheet submission: %w", err)
	}
	if sub == nil || (sub.Status != models.SubmissionStatusSigned && sub.Status != models.SubmissionStatusSubmitted) {
		return domain.NewUserError(domain.ErrInvalidInput, "Timesheet belum ditandatangani atau sudah dibatalkan")
	}

	now := s.nowFunc()
	sub.Status = models.SubmissionStatusRevoked
	sub.UpdatedAt = now

	metaMap := make(map[string]interface{})
	_ = json.Unmarshal(sub.AuditMetadata, &metaMap)
	metaMap["revoked_at"] = now.Format(time.RFC3339)
	metaMap["revoked_ip"] = meta.IP
	metaBytes, _ := json.Marshal(metaMap)
	sub.AuditMetadata = datatypes.JSON(metaBytes)

	if err := s.submissionRepo.UpsertTimesheetSubmission(ctx, sub); err != nil {
		return fmt.Errorf("revoke timesheet submission: %w", err)
	}
	return nil
}

// GetTimesheetSubmission fetches the timesheet submission status for the period.
func (s *signatureService) GetTimesheetSubmission(ctx context.Context, userID uint, month, year int) (*response.TimesheetSubmissionResponse, error) {
	sub, err := s.submissionRepo.FindTimesheetSubmission(ctx, userID, month, year)
	if err != nil {
		return nil, fmt.Errorf("find timesheet submission: %w", err)
	}
	if sub == nil {
		return &response.TimesheetSubmissionResponse{
			UserID: userID,
			Month:  month,
			Year:   year,
			Status: models.SubmissionStatusDraft,
		}, nil
	}

	return &response.TimesheetSubmissionResponse{
		ID:                sub.ID,
		UserID:            sub.UserID,
		Month:             sub.Month,
		Year:              sub.Year,
		Status:            sub.Status,
		SignedAt:          sub.SignedAt,
		VerificationToken: sub.VerificationToken,
		DocumentHash:      sub.DocumentHash,
		VerificationURL:   s.verificationURL(sub.VerificationToken),
	}, nil
}

func (s *signatureService) validateBeritaAcaraActivities(ctx context.Context, userID uint, month, year int) error {
	filter := repository.ActivityFilter{
		Month: &month,
		Year:  &year,
		IsAll: true,
	}
	activities, _, err := s.activityRepo.ListActiveByUser(ctx, userID, filter)
	if err != nil {
		return fmt.Errorf("list activities: %w", err)
	}
	for _, a := range activities {
		if a.IsBeritaAcara {
			return nil
		}
	}
	return domain.NewUserError(domain.ErrInvalidInput, "Tidak ada catatan Berita Acara pada bulan ini")
}

func (s *signatureService) notifyWitnessRequest(witness *models.User, employeeName, periodStr string) {
	if s.mailer != nil && witness.Email != "" {
		reviewLink := fmt.Sprintf("%s%s", strings.TrimRight(s.baseURL, "/"), beritaAcaraPath)
		_ = s.mailer.SendWitnessRequestEmail(witness.Email, witness.Name, employeeName, periodStr, reviewLink)
	}
	if s.pushSvc != nil {
		s.pushSvc.SendToUser(witness.ID, push.Payload{
			Title: "Permintaan Tanda Tangan Saksi",
			Body:  fmt.Sprintf("%s meminta Anda menjadi saksi Berita Acara (%s)", employeeName, periodStr),
			URL:   beritaAcaraPath,
		})
	}
}

// SubmitBeritaAcara submits a Berita Acara for witness review.
func (s *signatureService) SubmitBeritaAcara(ctx context.Context, userID uint, req *request.SubmitBeritaAcaraRequest, meta AuditMeta) (*response.BeritaAcaraSubmissionResponse, error) {
	if req == nil || req.Month < 1 || req.Month > 12 || req.Year < 2000 || req.Year > 2100 {
		return nil, domain.NewUserError(domain.ErrInvalidInput, errInvalidPeriodMsg)
	}

	if req.WitnessID == 0 {
		if mW, err := s.activityRepo.GetMonthlyWitness(ctx, userID, req.Month, req.Year); err == nil && mW != nil {
			req.WitnessID = mW.ID
		}
		if req.WitnessID == 0 {
			filter := repository.ActivityFilter{
				Month: &req.Month,
				Year:  &req.Year,
				IsAll: true,
			}
			acts, _, aerr := s.activityRepo.ListActiveByUser(ctx, userID, filter)
			if aerr == nil {
				for _, a := range acts {
					if a.IsBeritaAcara && a.WitnessID != nil && *a.WitnessID != 0 {
						req.WitnessID = *a.WitnessID
						break
					}
				}
			}
		}
		if req.WitnessID == 0 {
			return nil, domain.NewUserError(domain.ErrInvalidInput, "Belum ada saksi yang ditentukan untuk Berita Acara bulan ini. Silakan tentukan saksi terlebih dahulu.")
		}
	}

	if req.WitnessID == userID {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Saksi tidak boleh sama dengan pembuat Berita Acara")
	}

	user, err := s.userRepo.FindByIDWithDetails(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, errUserNotFoundMsg)
	}
	if user.SignatureFileKey == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Silakan unggah tanda tangan profil Anda terlebih dahulu")
	}

	witness, err := s.userRepo.FindByIDWithDetails(ctx, req.WitnessID)
	if err != nil || witness == nil || !witness.IsActive {
		return nil, domain.NewUserError(domain.ErrNotFound, "Saksi yang dipilih tidak ditemukan atau tidak aktif")
	}

	if err := s.validateBeritaAcaraActivities(ctx, userID, req.Month, req.Year); err != nil {
		return nil, err
	}

	// Ensure all Berita Acara activities for this month have the consistent designated witness
	_, _ = s.activityRepo.UpdateMonthlyWitness(ctx, userID, req.Month, req.Year, req.WitnessID)

	now := s.nowFunc()
	token := uuid.New().String()

	metaBytes, _ := json.Marshal(map[string]interface{}{
		"submit_ip":         meta.IP,
		"submit_user_agent": meta.UserAgent,
		"employee_name":     user.Name,
		"witness_name":      witness.Name,
	})

	sub := &models.BeritaAcaraSubmission{
		ID:                uuid.New().String(),
		UserID:            userID,
		WitnessID:         req.WitnessID,
		Month:             req.Month,
		Year:              req.Year,
		Status:            models.SubmissionStatusPendingWitness,
		EmployeeSignedAt:  &now,
		VerificationToken: token,
		AuditMetadata:     datatypes.JSON(metaBytes),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.submissionRepo.UpsertBeritaAcaraSubmission(ctx, sub); err != nil {
		return nil, fmt.Errorf("save berita acara submission: %w", err)
	}

	periodStr := fmt.Sprintf("%s %d", services.MonthNameIndonesian(req.Month), req.Year)
	s.notifyWitnessRequest(witness, user.Name, periodStr)

	return &response.BeritaAcaraSubmissionResponse{
		ID:                sub.ID,
		UserID:            sub.UserID,
		EmployeeName:      user.Name,
		WitnessID:         sub.WitnessID,
		WitnessName:       witness.Name,
		Month:             sub.Month,
		Year:              sub.Year,
		Status:            sub.Status,
		EmployeeSignedAt:  sub.EmployeeSignedAt,
		VerificationToken: sub.VerificationToken,
		VerificationURL:   s.verificationURL(sub.VerificationToken),
		CreatedAt:         sub.CreatedAt,
		UpdatedAt:         sub.UpdatedAt,
	}, nil
}

func (s *signatureService) approveWitness(
	ctx context.Context,
	sub *models.BeritaAcaraSubmission,
	witness, employee *models.User,
	meta AuditMeta,
) error {
	if witness.SignatureFileKey == "" {
		return domain.NewUserError(domain.ErrInvalidInput, "Silakan unggah tanda tangan profil Anda terlebih dahulu untuk menyetujui sebagai saksi")
	}

	now := s.nowFunc()
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "type:berita_acara|uid:%d|wid:%d|month:%d|year:%d|token:%s|time:%d",
		sub.UserID, sub.WitnessID, sub.Month, sub.Year, sub.VerificationToken, now.Unix())
	docHash := hex.EncodeToString(h.Sum(nil))

	sub.Status = models.SubmissionStatusSigned
	sub.WitnessSignedAt = &now
	sub.DocumentHash = docHash
	sub.UpdatedAt = now

	metaMap := make(map[string]interface{})
	_ = json.Unmarshal(sub.AuditMetadata, &metaMap)
	metaMap["witness_ip"] = meta.IP
	metaMap["witness_user_agent"] = meta.UserAgent
	metaMap["approved_at"] = now.Format(time.RFC3339)
	metaBytes, _ := json.Marshal(metaMap)
	sub.AuditMetadata = datatypes.JSON(metaBytes)

	if err := s.submissionRepo.UpsertBeritaAcaraSubmission(ctx, sub); err != nil {
		return fmt.Errorf("save witness approval: %w", err)
	}

	periodStr := fmt.Sprintf("%s %d", services.MonthNameIndonesian(sub.Month), sub.Year)
	dlLink := fmt.Sprintf("%s%s", strings.TrimRight(s.baseURL, "/"), beritaAcaraPath)
	if s.mailer != nil && employee.Email != "" {
		_ = s.mailer.SendBeritaAcaraApprovedEmail(employee.Email, employee.Name, witness.Name, periodStr, dlLink)
	}
	if s.pushSvc != nil {
		s.pushSvc.SendToUser(employee.ID, push.Payload{
			Title: "Berita Acara Disetujui",
			Body:  fmt.Sprintf("Saksi %s telah menandatangani Berita Acara Anda (%s)", witness.Name, periodStr),
			URL:   beritaAcaraPath,
		})
	}
	return nil
}

func (s *signatureService) rejectWitness(
	ctx context.Context,
	sub *models.BeritaAcaraSubmission,
	witness, employee *models.User,
	notes string,
	meta AuditMeta,
) error {
	trimmedNotes := strings.TrimSpace(notes)
	if trimmedNotes == "" {
		return domain.NewUserError(domain.ErrInvalidInput, "Catatan penolakan wajib diisi")
	}

	now := s.nowFunc()
	sub.Status = models.SubmissionStatusRejected
	sub.WitnessRejectionNotes = trimmedNotes
	sub.UpdatedAt = now

	metaMap := make(map[string]interface{})
	_ = json.Unmarshal(sub.AuditMetadata, &metaMap)
	metaMap["witness_ip"] = meta.IP
	metaMap["witness_user_agent"] = meta.UserAgent
	metaMap["rejected_at"] = now.Format(time.RFC3339)
	metaMap["rejection_notes"] = trimmedNotes
	metaBytes, _ := json.Marshal(metaMap)
	sub.AuditMetadata = datatypes.JSON(metaBytes)

	if err := s.submissionRepo.UpsertBeritaAcaraSubmission(ctx, sub); err != nil {
		return fmt.Errorf("save witness rejection: %w", err)
	}

	periodStr := fmt.Sprintf("%s %d", services.MonthNameIndonesian(sub.Month), sub.Year)
	if s.mailer != nil && employee.Email != "" {
		_ = s.mailer.SendBeritaAcaraRejectedEmail(employee.Email, employee.Name, witness.Name, periodStr, trimmedNotes)
	}
	if s.pushSvc != nil {
		s.pushSvc.SendToUser(employee.ID, push.Payload{
			Title: "Berita Acara Ditolak",
			Body:  fmt.Sprintf("Saksi %s menolak Berita Acara Anda: %s", witness.Name, trimmedNotes),
			URL:   beritaAcaraPath,
		})
	}
	return nil
}

// WitnessAction processes witness approval or rejection.
func (s *signatureService) WitnessAction(ctx context.Context, witnessID uint, submissionID string, req *request.WitnessActionRequest, meta AuditMeta) (*response.BeritaAcaraSubmissionResponse, error) {
	if req == nil {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Request payload tidak boleh kosong")
	}

	sub, err := s.submissionRepo.FindBeritaAcaraByID(ctx, submissionID)
	if err != nil {
		return nil, fmt.Errorf("find berita acara by id: %w", err)
	}
	if sub == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Permintaan Berita Acara tidak ditemukan")
	}

	if sub.WitnessID != witnessID {
		return nil, domain.ErrForbidden
	}
	if sub.Status != models.SubmissionStatusPendingWitness {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Permintaan Berita Acara sudah tidak dalam status menunggu saksi")
	}

	witness, err := s.userRepo.FindByIDWithDetails(ctx, witnessID)
	if err != nil || witness == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Saksi tidak ditemukan")
	}

	employee, err := s.userRepo.FindByIDWithDetails(ctx, sub.UserID)
	if err != nil || employee == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Pemilik Berita Acara tidak ditemukan")
	}

	switch req.Action {
	case "approve":
		if err := s.approveWitness(ctx, sub, witness, employee, meta); err != nil {
			return nil, err
		}
	case "reject":
		if err := s.rejectWitness(ctx, sub, witness, employee, req.Notes, meta); err != nil {
			return nil, err
		}
	default:
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Aksi tidak dikenal")
	}

	return &response.BeritaAcaraSubmissionResponse{
		ID:                    sub.ID,
		UserID:                sub.UserID,
		EmployeeName:          employee.Name,
		WitnessID:             sub.WitnessID,
		WitnessName:           witness.Name,
		Month:                 sub.Month,
		Year:                  sub.Year,
		Status:                sub.Status,
		EmployeeSignedAt:      sub.EmployeeSignedAt,
		WitnessSignedAt:       sub.WitnessSignedAt,
		WitnessRejectionNotes: sub.WitnessRejectionNotes,
		VerificationToken:     sub.VerificationToken,
		DocumentHash:          sub.DocumentHash,
		VerificationURL:       s.verificationURL(sub.VerificationToken),
		CreatedAt:             sub.CreatedAt,
		UpdatedAt:             sub.UpdatedAt,
	}, nil
}

// RevokeBeritaAcara revokes an existing Berita Acara submission and resets to draft.
func (s *signatureService) RevokeBeritaAcara(ctx context.Context, userID uint, submissionID string, meta AuditMeta) error {
	sub, err := s.submissionRepo.FindBeritaAcaraByID(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("find berita acara by id: %w", err)
	}
	if sub == nil {
		return domain.NewUserError(domain.ErrNotFound, "Berita Acara tidak ditemukan")
	}
	if sub.UserID != userID {
		return domain.ErrForbidden
	}

	now := s.nowFunc()
	sub.Status = models.SubmissionStatusRevoked
	sub.UpdatedAt = now

	metaMap := make(map[string]interface{})
	_ = json.Unmarshal(sub.AuditMetadata, &metaMap)
	metaMap["revoked_at"] = now.Format(time.RFC3339)
	metaMap["revoked_ip"] = meta.IP
	metaBytes, _ := json.Marshal(metaMap)
	sub.AuditMetadata = datatypes.JSON(metaBytes)

	if err := s.submissionRepo.UpsertBeritaAcaraSubmission(ctx, sub); err != nil {
		return fmt.Errorf("revoke berita acara: %w", err)
	}
	return nil
}

// GetBeritaAcaraSubmission fetches the submission status for the user and month.
func (s *signatureService) GetBeritaAcaraSubmission(ctx context.Context, userID uint, month, year int) (*response.BeritaAcaraSubmissionResponse, error) {
	sub, err := s.submissionRepo.FindBeritaAcaraSubmission(ctx, userID, month, year)
	if err != nil {
		return nil, fmt.Errorf("find berita acara submission: %w", err)
	}
	if sub == nil {
		return &response.BeritaAcaraSubmissionResponse{
			UserID: userID,
			Month:  month,
			Year:   year,
			Status: models.SubmissionStatusDraft,
		}, nil
	}

	empName := ""
	if sub.User != nil {
		empName = sub.User.Name
	}
	witName := ""
	if sub.Witness != nil {
		witName = sub.Witness.Name
	}

	return &response.BeritaAcaraSubmissionResponse{
		ID:                    sub.ID,
		UserID:                sub.UserID,
		EmployeeName:          empName,
		WitnessID:             sub.WitnessID,
		WitnessName:           witName,
		Month:                 sub.Month,
		Year:                  sub.Year,
		Status:                sub.Status,
		EmployeeSignedAt:      sub.EmployeeSignedAt,
		WitnessSignedAt:       sub.WitnessSignedAt,
		WitnessRejectionNotes: sub.WitnessRejectionNotes,
		VerificationToken:     sub.VerificationToken,
		DocumentHash:          sub.DocumentHash,
		VerificationURL:       s.verificationURL(sub.VerificationToken),
		CreatedAt:             sub.CreatedAt,
		UpdatedAt:             sub.UpdatedAt,
	}, nil
}

// ListPendingWitnessRequests returns submissions pending approval by the witness.
func (s *signatureService) ListPendingWitnessRequests(ctx context.Context, witnessID uint) ([]response.BeritaAcaraSubmissionResponse, error) {
	list, err := s.submissionRepo.ListPendingWitnessRequests(ctx, witnessID)
	if err != nil {
		return nil, fmt.Errorf("list pending requests: %w", err)
	}

	res := make([]response.BeritaAcaraSubmissionResponse, 0, len(list))
	for _, sub := range list {
		empName := ""
		if sub.User != nil {
			empName = sub.User.Name
		}
		witName := ""
		if sub.Witness != nil {
			witName = sub.Witness.Name
		}

		res = append(res, response.BeritaAcaraSubmissionResponse{
			ID:                sub.ID,
			UserID:            sub.UserID,
			EmployeeName:      empName,
			WitnessID:         sub.WitnessID,
			WitnessName:       witName,
			Month:             sub.Month,
			Year:              sub.Year,
			Status:            sub.Status,
			EmployeeSignedAt:  sub.EmployeeSignedAt,
			VerificationToken: sub.VerificationToken,
			VerificationURL:   s.verificationURL(sub.VerificationToken),
			CreatedAt:         sub.CreatedAt,
			UpdatedAt:         sub.UpdatedAt,
		})
	}
	return res, nil
}

func (s *signatureService) resolveUserNameAndBNI(ctx context.Context, user *models.User, userID uint) (string, string) {
	if user != nil {
		return user.Name, user.BniID
	}
	if userID > 0 {
		if u, err := s.userRepo.FindByID(ctx, userID); err == nil && u != nil {
			return u.Name, u.BniID
		}
	}
	return "", ""
}

func (s *signatureService) resolveWitnessName(ctx context.Context, witness *models.User, witnessID uint) string {
	if witness != nil {
		return witness.Name
	}
	if witnessID > 0 {
		if w, err := s.userRepo.FindByID(ctx, witnessID); err == nil && w != nil {
			return w.Name
		}
	}
	return ""
}

func (s *signatureService) verifyTimesheet(ctx context.Context, token string, now time.Time) (*response.PublicDocumentVerificationResponse, error) {
	ts, err := s.submissionRepo.FindTimesheetByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("find timesheet by token: %w", err)
	}
	if ts == nil {
		return nil, nil
	}

	empName, bniID := s.resolveUserNameAndBNI(ctx, ts.User, ts.UserID)
	isValid := ts.Status == models.SubmissionStatusSigned || ts.Status == models.SubmissionStatusSubmitted
	statusStr := "valid"
	if !isValid {
		statusStr = "revoked"
	}

	return &response.PublicDocumentVerificationResponse{
		DocumentType:     "timesheet",
		Status:           statusStr,
		IsValid:          isValid,
		Month:            ts.Month,
		Year:             ts.Year,
		EmployeeName:     empName,
		EmployeeBNIID:    bniID,
		EmployeeSignedAt: ts.SignedAt,
		DocumentHash:     ts.DocumentHash,
		VerifiedAt:       now,
	}, nil
}

func (s *signatureService) verifyBeritaAcara(ctx context.Context, token string, now time.Time) (*response.PublicDocumentVerificationResponse, error) {
	ba, err := s.submissionRepo.FindBeritaAcaraByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("find berita acara by token: %w", err)
	}
	if ba == nil {
		return nil, nil
	}

	empName, bniID := s.resolveUserNameAndBNI(ctx, ba.User, ba.UserID)
	witName := s.resolveWitnessName(ctx, ba.Witness, ba.WitnessID)

	isValid := ba.Status == models.SubmissionStatusSigned
	statusStr := "pending"
	if isValid {
		statusStr = "valid"
	} else if ba.Status == models.SubmissionStatusRevoked || ba.Status == models.SubmissionStatusRejected {
		statusStr = ba.Status
	}

	return &response.PublicDocumentVerificationResponse{
		DocumentType:     "berita_acara",
		Status:           statusStr,
		IsValid:          isValid,
		Month:            ba.Month,
		Year:             ba.Year,
		EmployeeName:     empName,
		EmployeeBNIID:    bniID,
		EmployeeSignedAt: ba.EmployeeSignedAt,
		WitnessName:      witName,
		WitnessSignedAt:  ba.WitnessSignedAt,
		DocumentHash:     ba.DocumentHash,
		VerifiedAt:       now,
	}, nil
}

// VerifyDocument verifies a document token publicly.
func (s *signatureService) VerifyDocument(ctx context.Context, token string) (*response.PublicDocumentVerificationResponse, error) {
	tok := strings.TrimSpace(token)
	if tok == "" {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Token verifikasi tidak boleh kosong")
	}

	now := s.nowFunc()

	// 1. Check Timesheet Submission
	if res, err := s.verifyTimesheet(ctx, tok, now); err != nil {
		return nil, err
	} else if res != nil {
		return res, nil
	}

	// 2. Check Berita Acara Submission
	if res, err := s.verifyBeritaAcara(ctx, tok, now); err != nil {
		return nil, err
	} else if res != nil {
		return res, nil
	}

	return nil, domain.NewUserError(domain.ErrNotFound, "Dokumen tidak ditemukan atau token tidak valid")
}
