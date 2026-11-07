package service_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
)

// Helper to generate a valid test base64 PNG
func generateTestSignatureBase64(w, h int) string {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

type mockSubmissionRepo struct {
	tsSubmissions  map[string]*models.TimesheetSubmission
	baSubmissions  map[string]*models.BeritaAcaraSubmission
	tsLocked       map[string]bool
	baLocked       map[string]bool
	findTsErr      error
	findTsTokenErr error
	upsertTsErr    error
	findBaErr      error
	findBaTokenErr error
	upsertBaErr    error
	listPendErr    error
}

func newMockSubmissionRepo() *mockSubmissionRepo {
	return &mockSubmissionRepo{
		tsSubmissions: make(map[string]*models.TimesheetSubmission),
		baSubmissions: make(map[string]*models.BeritaAcaraSubmission),
		tsLocked:      make(map[string]bool),
		baLocked:      make(map[string]bool),
	}
}

func (m *mockSubmissionRepo) FindTimesheetSubmission(ctx context.Context, userID uint, month, year int) (*models.TimesheetSubmission, error) {
	if m.findTsErr != nil {
		return nil, m.findTsErr
	}
	for _, s := range m.tsSubmissions {
		if s.UserID == userID && s.Month == month && s.Year == year {
			clone := *s
			return &clone, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) FindTimesheetByToken(ctx context.Context, token string) (*models.TimesheetSubmission, error) {
	if m.findTsTokenErr != nil {
		return nil, m.findTsTokenErr
	}
	for _, s := range m.tsSubmissions {
		if s.VerificationToken == token {
			clone := *s
			return &clone, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) UpsertTimesheetSubmission(ctx context.Context, sub *models.TimesheetSubmission) error {
	if m.upsertTsErr != nil {
		return m.upsertTsErr
	}
	clone := *sub
	for k, existing := range m.tsSubmissions {
		if existing.UserID == sub.UserID && existing.Month == sub.Month && existing.Year == sub.Year {
			delete(m.tsSubmissions, k)
			break
		}
	}
	m.tsSubmissions[sub.ID] = &clone
	if sub.Status == models.SubmissionStatusSigned || sub.Status == models.SubmissionStatusSubmitted {
		m.tsLocked[sub.ID] = true
	} else {
		m.tsLocked[sub.ID] = false
	}
	return nil
}

func (m *mockSubmissionRepo) FindBeritaAcaraSubmission(ctx context.Context, userID uint, month, year int) (*models.BeritaAcaraSubmission, error) {
	if m.findBaErr != nil {
		return nil, m.findBaErr
	}
	for _, s := range m.baSubmissions {
		if s.UserID == userID && s.Month == month && s.Year == year {
			clone := *s
			return &clone, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) FindBeritaAcaraByID(ctx context.Context, id string) (*models.BeritaAcaraSubmission, error) {
	if m.findBaErr != nil {
		return nil, m.findBaErr
	}
	s, ok := m.baSubmissions[id]
	if !ok {
		return nil, nil
	}
	clone := *s
	return &clone, nil
}

func (m *mockSubmissionRepo) FindBeritaAcaraByToken(ctx context.Context, token string) (*models.BeritaAcaraSubmission, error) {
	if m.findBaTokenErr != nil {
		return nil, m.findBaTokenErr
	}
	for _, s := range m.baSubmissions {
		if s.VerificationToken == token {
			clone := *s
			return &clone, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) UpsertBeritaAcaraSubmission(ctx context.Context, sub *models.BeritaAcaraSubmission) error {
	if m.upsertBaErr != nil {
		return m.upsertBaErr
	}
	clone := *sub
	for k, existing := range m.baSubmissions {
		if existing.UserID == sub.UserID && existing.Month == sub.Month && existing.Year == sub.Year {
			delete(m.baSubmissions, k)
			break
		}
	}
	m.baSubmissions[sub.ID] = &clone
	if sub.Status == models.SubmissionStatusPendingWitness || sub.Status == models.SubmissionStatusSigned {
		m.baLocked[sub.ID] = true
	} else {
		m.baLocked[sub.ID] = false
	}
	return nil
}

func (m *mockSubmissionRepo) ListPendingWitnessRequests(ctx context.Context, witnessID uint) ([]models.BeritaAcaraSubmission, error) {
	if m.listPendErr != nil {
		return nil, m.listPendErr
	}
	var list []models.BeritaAcaraSubmission
	for _, s := range m.baSubmissions {
		if s.WitnessID == witnessID && s.Status == models.SubmissionStatusPendingWitness {
			list = append(list, *s)
		}
	}
	return list, nil
}

func (m *mockSubmissionRepo) IsTimesheetLocked(ctx context.Context, userID uint, month, year int) (bool, error) {
	for _, s := range m.tsSubmissions {
		if s.UserID == userID && s.Month == month && s.Year == year {
			return s.Status == models.SubmissionStatusSigned || s.Status == models.SubmissionStatusSubmitted, nil
		}
	}
	return false, nil
}

func (m *mockSubmissionRepo) IsBeritaAcaraLocked(ctx context.Context, userID uint, month, year int) (bool, error) {
	for _, s := range m.baSubmissions {
		if s.UserID == userID && s.Month == month && s.Year == year {
			return s.Status == models.SubmissionStatusPendingWitness || s.Status == models.SubmissionStatusSigned, nil
		}
	}
	return false, nil
}

type mockStorageSvc struct {
	files     map[string][]byte
	uploadErr error
}

func newMockStorageSvc() *mockStorageSvc {
	return &mockStorageSvc{files: make(map[string][]byte)}
}

func (m *mockStorageSvc) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	if m.uploadErr != nil {
		return m.uploadErr
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.files[key] = b
	return nil
}

func (m *mockStorageSvc) Download(ctx context.Context, key string) ([]byte, error) {
	b, ok := m.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return b, nil
}

func (m *mockStorageSvc) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}

func (m *mockStorageSvc) GetPresignedDownloadURLWithFilename(ctx context.Context, key string, filename string, expiry time.Duration) (string, error) {
	return "https://storage.test/" + key + "?att=" + filename, nil
}

func (m *mockStorageSvc) Delete(ctx context.Context, key string) error {
	delete(m.files, key)
	return nil
}

func (m *mockStorageSvc) FileExists(ctx context.Context, key string) (bool, error) {
	_, ok := m.files[key]
	return ok, nil
}

type mockUserRepo struct {
	repository.UserAccountRepository
	users        map[uint]*models.User
	passkeys     map[uint][]models.WebAuthnCredential
	updateSigErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:    make(map[uint]*models.User),
		passkeys: make(map[uint][]models.WebAuthnCredential),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) FindByIDWithDetails(ctx context.Context, id uint) (*models.User, error) {
	return m.FindByID(ctx, id)
}

func (m *mockUserRepo) UpdateSignatureFileKey(ctx context.Context, id uint, fileKey string) error {
	if m.updateSigErr != nil {
		return m.updateSigErr
	}
	u, ok := m.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.SignatureFileKey = fileKey
	return nil
}

func (m *mockUserRepo) ListPasskeysByUserID(ctx context.Context, userID uint) ([]models.WebAuthnCredential, error) {
	return m.passkeys[userID], nil
}

func (m *mockUserRepo) UpdatePasskeySignCount(ctx context.Context, credID []byte, signCount uint32, backupState bool) error {
	for userID, list := range m.passkeys {
		for i := range list {
			if bytes.Equal(list[i].CredentialID, credID) {
				list[i].SignCount = signCount
				list[i].BackupState = backupState
				m.passkeys[userID] = list
				return nil
			}
		}
	}
	return nil
}

type mockActivityRepo struct {
	repository.ActivityRepository
	activities []models.DailyActivity
	listErr    error
}

func newMockActivityRepo() *mockActivityRepo {
	return &mockActivityRepo{}
}

func (m *mockActivityRepo) ListActiveByUser(ctx context.Context, userID uint, filter repository.ActivityFilter) ([]models.DailyActivity, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.activities, int64(len(m.activities)), nil
}

func (m *mockActivityRepo) UpdateMonthlyWitness(ctx context.Context, userID uint, month, year int, newWitnessID uint) (int64, error) {
	return 1, nil
}

func (m *mockActivityRepo) GetMonthlyWitness(ctx context.Context, userID uint, month, year int) (*models.User, error) {
	return nil, nil
}

func TestSignatureService_ProfileSignature(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	user := &models.User{
		ID:       1,
		Username: "sig_user",
		Email:    "sig@example.com",
		Name:     "Signature User",
		IsActive: true,
	}
	_ = userRepo.Create(ctx, user)

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")

	// 1. Initial check -> no signature
	resp, err := svc.GetSignature(ctx, user.ID)
	require.NoError(t, err)
	assert.False(t, resp.HasSignature)
	assert.Empty(t, resp.SignatureURL)

	// 2. Upload invalid base64
	_, err = svc.UploadSignature(ctx, user.ID, "not-valid-base64!!!")
	assert.Error(t, err)

	// 3. Upload tiny dimension
	tinyB64 := generateTestSignatureBase64(10, 10)
	_, err = svc.UploadSignature(ctx, user.ID, tinyB64)
	assert.Error(t, err)

	// 4. Upload valid signature
	validB64 := generateTestSignatureBase64(100, 50)
	uploadResp, err := svc.UploadSignature(ctx, user.ID, validB64)
	require.NoError(t, err)
	assert.True(t, uploadResp.HasSignature)
	assert.Contains(t, uploadResp.SignatureURL, "https://storage.test/signatures/1_")

	// 5. GetSignature should now return HasSignature=true
	getResp, err := svc.GetSignature(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, getResp.HasSignature)
	assert.Contains(t, getResp.SignatureURL, "https://storage.test/signatures/1_")

	// 6. DeleteSignature
	err = svc.DeleteSignature(ctx, user.ID)
	require.NoError(t, err)

	delResp, err := svc.GetSignature(ctx, user.ID)
	require.NoError(t, err)
	assert.False(t, delResp.HasSignature)
}

func TestSignatureService_TimesheetSignAndRevoke(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	user := &models.User{
		ID:               1,
		Username:         "alice",
		Name:             "Alice",
		Email:            "alice@example.com",
		SignatureFileKey: "signatures/1_123.png",
		IsActive:         true,
	}
	_ = userRepo.Create(ctx, user)

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")
	meta := service.AuditMeta{IP: "127.0.0.1", UserAgent: "Go-Test"}

	// Case 1: Sign with no activities -> fails
	_, err := svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
	assert.Error(t, err)

	// Seed 1 activity
	actRepo.activities = []models.DailyActivity{{
		ID:       1,
		UserID:   user.ID,
		Date:     time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
		Activity: "Working on tests",
		IsActive: true,
	}}

	// Case 2: Sign successfully
	signResp, err := svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
	require.NoError(t, err)
	assert.Equal(t, models.SubmissionStatusSigned, signResp.Status)
	assert.NotEmpty(t, signResp.VerificationToken)
	assert.NotEmpty(t, signResp.DocumentHash)
	assert.Contains(t, signResp.VerificationURL, "/verify/doc/"+signResp.VerificationToken)

	// Case 3: Public verification returns valid
	verifyResp, err := svc.VerifyDocument(ctx, signResp.VerificationToken)
	require.NoError(t, err)
	assert.True(t, verifyResp.IsValid)
	assert.Equal(t, "valid", verifyResp.Status)
	assert.Equal(t, "timesheet", verifyResp.DocumentType)
	assert.Equal(t, "Alice", verifyResp.EmployeeName)

	// Case 4: Revoke Timesheet
	err = svc.RevokeTimesheet(ctx, user.ID, &request.RevokeTimesheetRequest{Month: 10, Year: 2026}, meta)
	require.NoError(t, err)

	// Verification after revoke returns revoked status
	verifyRevoked, err := svc.VerifyDocument(ctx, signResp.VerificationToken)
	require.NoError(t, err)
	assert.False(t, verifyRevoked.IsValid)
	assert.Equal(t, "revoked", verifyRevoked.Status)
}

func TestSignatureService_BeritaAcaraWorkflow(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	employee := &models.User{
		ID:               1,
		Username:         "emp",
		Name:             "Employee One",
		Email:            "emp@example.com",
		SignatureFileKey: "signatures/1_sig.png",
		IsActive:         true,
	}
	witness := &models.User{
		ID:               2,
		Username:         "wit",
		Name:             "Witness Two",
		Email:            "wit@example.com",
		SignatureFileKey: "signatures/2_sig.png",
		IsActive:         true,
	}
	_ = userRepo.Create(ctx, employee)
	_ = userRepo.Create(ctx, witness)

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")
	meta := service.AuditMeta{IP: "10.0.0.1", UserAgent: "TestClient"}

	// 1. Submit without BA activities -> fails
	_, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
		Month:     10,
		Year:      2026,
		WitnessID: witness.ID,
	}, meta)
	assert.Error(t, err)

	// Seed BA activity
	actRepo.activities = []models.DailyActivity{{
		ID:            1,
		UserID:        employee.ID,
		Date:          time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		Activity:      "Incident handling",
		IsBeritaAcara: true,
		IsActive:      true,
	}}

	// 2. Submit successfully -> status: pending_witness
	submitResp, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
		Month:     10,
		Year:      2026,
		WitnessID: witness.ID,
	}, meta)
	require.NoError(t, err)
	assert.Equal(t, models.SubmissionStatusPendingWitness, submitResp.Status)
	assert.Equal(t, witness.ID, submitResp.WitnessID)

	// 3. Witness lists pending requests
	pendingList, err := svc.ListPendingWitnessRequests(ctx, witness.ID)
	require.NoError(t, err)
	assert.Len(t, pendingList, 1)
	assert.Equal(t, submitResp.ID, pendingList[0].ID)

	// 4. Non-witness cannot approve/reject -> ErrForbidden
	_, err = svc.WitnessAction(ctx, 999, submitResp.ID, &request.WitnessActionRequest{Action: "approve"}, meta)
	assert.ErrorIs(t, err, domain.ErrForbidden)

	// 5. Witness approves & signs
	approveResp, err := svc.WitnessAction(ctx, witness.ID, submitResp.ID, &request.WitnessActionRequest{Action: "approve"}, meta)
	require.NoError(t, err)
	assert.Equal(t, models.SubmissionStatusSigned, approveResp.Status)
	assert.NotEmpty(t, approveResp.WitnessSignedAt)
	assert.NotEmpty(t, approveResp.DocumentHash)

	// 6. Verify signed Berita Acara
	verifyResp, err := svc.VerifyDocument(ctx, approveResp.VerificationToken)
	require.NoError(t, err)
	assert.True(t, verifyResp.IsValid)
	assert.Equal(t, "berita_acara", verifyResp.DocumentType)
	assert.Equal(t, "Employee One", verifyResp.EmployeeName)
	assert.Equal(t, "Witness Two", verifyResp.WitnessName)

	// 7. Revoke Berita Acara
	err = svc.RevokeBeritaAcara(ctx, employee.ID, submitResp.ID, meta)
	require.NoError(t, err)

	verifyAfterRevoke, err := svc.VerifyDocument(ctx, approveResp.VerificationToken)
	require.NoError(t, err)
	assert.False(t, verifyAfterRevoke.IsValid)
	assert.Equal(t, "revoked", verifyAfterRevoke.Status)
}

func TestSignatureService_ValidationErrorsAndEdgeCases(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()
	meta := service.AuditMeta{IP: "127.0.0.1", UserAgent: "EdgeCaseClient"}

	user := &models.User{
		ID:       10,
		Username: "edge_user",
		Email:    "edge@example.com",
		Name:     "Edge User",
		IsActive: true,
	}
	_ = userRepo.Create(ctx, user)

	witness := &models.User{
		ID:       20,
		Username: "edge_witness",
		Email:    "edge_wit@example.com",
		Name:     "Edge Witness",
		IsActive: true,
	}
	_ = userRepo.Create(ctx, witness)

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "")
	if sigImpl, ok := svc.(interface{ SetNowFunc(func() time.Time) }); ok {
		sigImpl.SetNowFunc(func() time.Time {
			return time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
		})
	}

	t.Run("UploadSignature edge cases", func(t *testing.T) {
		// Empty base64
		_, err := svc.UploadSignature(ctx, user.ID, "")
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		_, err = svc.UploadSignature(ctx, user.ID, "data:image/png;base64,   ")
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-image text
		textB64 := base64.StdEncoding.EncodeToString([]byte("just plain text"))
		_, err = svc.UploadSignature(ctx, user.ID, textB64)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-existent user
		validB64 := generateTestSignatureBase64(60, 40)
		_, err = svc.UploadSignature(ctx, 99999, validB64)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("Get and Delete signature non-existent user", func(t *testing.T) {
		_, err := svc.GetSignature(ctx, 99999)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		err = svc.DeleteSignature(ctx, 99999)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("SignTimesheet edge cases", func(t *testing.T) {
		// Invalid period
		_, err := svc.SignTimesheet(ctx, user.ID, nil, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		_, err = svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 0, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-existent user
		_, err = svc.SignTimesheet(ctx, 99999, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// User without profile signature
		_, err = svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Upload signature, but no activities
		validB64 := generateTestSignatureBase64(60, 40)
		_, _ = svc.UploadSignature(ctx, user.ID, validB64)

		_, err = svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("RevokeTimesheet edge cases", func(t *testing.T) {
		// Invalid period
		err := svc.RevokeTimesheet(ctx, user.ID, nil, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		err = svc.RevokeTimesheet(ctx, user.ID, &request.RevokeTimesheetRequest{Month: 13, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-existent user
		err = svc.RevokeTimesheet(ctx, 99999, &request.RevokeTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-existent submission
		err = svc.RevokeTimesheet(ctx, user.ID, &request.RevokeTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("GetTimesheetSubmission draft fallback", func(t *testing.T) {
		statusResp, err := svc.GetTimesheetSubmission(ctx, user.ID, 1, 2026)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusDraft, statusResp.Status)
	})

	t.Run("SubmitBeritaAcara edge cases", func(t *testing.T) {
		// Invalid period
		_, err := svc.SubmitBeritaAcara(ctx, user.ID, nil, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		_, err = svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 0, Year: 2026}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-existent employee
		_, err = svc.SubmitBeritaAcara(ctx, 99999, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: witness.ID}, meta)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// Non-existent witness
		_, err = svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: 88888}, meta)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// Self-witnessing
		_, err = svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: user.ID}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Witness has no signature
		_, err = svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: witness.ID}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("WitnessAction rejection and validation", func(t *testing.T) {
		validB64 := generateTestSignatureBase64(60, 40)
		_, _ = svc.UploadSignature(ctx, witness.ID, validB64)

		// Seed BA activity
		actRepo.activities = []models.DailyActivity{{
			ID:            101,
			UserID:        user.ID,
			Date:          time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
			Activity:      "Deployment issue",
			IsBeritaAcara: true,
			IsActive:      true,
		}}

		subResp, err := svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{
			Month:     10,
			Year:      2026,
			WitnessID: witness.ID,
		}, meta)
		require.NoError(t, err)

		// Unknown action
		_, err = svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{Action: "invalid_action"}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Reject without notes
		_, err = svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{Action: "reject", Notes: "   "}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Reject successfully with notes
		rejResp, err := svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{Action: "reject", Notes: "Revisi jam kerja"}, meta)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusRejected, rejResp.Status)
		assert.Equal(t, "Revisi jam kerja", rejResp.WitnessRejectionNotes)

		// Acting on already rejected submission
		_, err = svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{Action: "approve"}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-existent submission ID
		_, err = svc.WitnessAction(ctx, witness.ID, "non-existent-sub-id", &request.WitnessActionRequest{Action: "approve"}, meta)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("RevokeBeritaAcara and GetBeritaAcaraSubmission edge cases", func(t *testing.T) {
		// Non-existent submission ID
		err := svc.RevokeBeritaAcara(ctx, user.ID, "non-existent-sub", meta)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// Non-existent user
		err = svc.RevokeBeritaAcara(ctx, 99999, "any-id", meta)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// Draft status when not found
		baResp, err := svc.GetBeritaAcaraSubmission(ctx, user.ID, 12, 2026)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusDraft, baResp.Status)
	})

	t.Run("VerifyDocument edge cases", func(t *testing.T) {
		// Empty token
		_, err := svc.VerifyDocument(ctx, "")
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Non-existent token -> ErrNotFound
		_, err = svc.VerifyDocument(ctx, "non-existent-token")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestSignatureService_ComprehensiveBranches(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	user := &models.User{
		ID:       1,
		Username: "employee_comp",
		Email:    "emp@example.com",
		Name:     "Employee Comp",
		IsActive: true,
	}
	witness := &models.User{
		ID:       2,
		Username: "witness_comp",
		Email:    "wit@example.com",
		Name:     "Witness Comp",
		IsActive: true,
	}
	_ = userRepo.Create(ctx, user)
	_ = userRepo.Create(ctx, witness)

	meta := service.AuditMeta{IP: "127.0.0.1", UserAgent: "TestAgent/1.0"}

	// Mailer & Push initialized
	m := mailer.New(&config.Config{})
	p := push.New(&config.Config{}, nil)
	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, m, p, "https://timesheet.local")

	t.Run("UploadSignature oversized and non-png/jpeg format", func(t *testing.T) {
		// Oversized (> 2MB)
		bigBytes := make([]byte, 3*1024*1024)
		bigB64 := base64.StdEncoding.EncodeToString(bigBytes)
		_, err := svc.UploadSignature(ctx, user.ID, bigB64)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Valid GIF (not PNG or JPEG)
		gifB64 := "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"
		_, err = svc.UploadSignature(ctx, user.ID, gifB64)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("UploadSignature storage upload error and update key error", func(t *testing.T) {
		validB64 := generateTestSignatureBase64(50, 50)
		storageSvc.uploadErr = errors.New("s3 connection failed")
		_, err := svc.UploadSignature(ctx, user.ID, validB64)
		assert.ErrorContains(t, err, "failed to upload signature")
		storageSvc.uploadErr = nil

		userRepo.updateSigErr = errors.New("db error")
		_, err = svc.UploadSignature(ctx, user.ID, validB64)
		assert.ErrorContains(t, err, "failed to update user signature key")
		userRepo.updateSigErr = nil
	})

	t.Run("DeleteSignature update key error", func(t *testing.T) {
		userRepo.updateSigErr = errors.New("db clear error")
		err := svc.DeleteSignature(ctx, user.ID)
		assert.ErrorContains(t, err, "failed to clear signature key")
		userRepo.updateSigErr = nil
	})

	t.Run("SignTimesheet activity list error and upsert submission error", func(t *testing.T) {
		// Ensure signature is uploaded
		validB64 := generateTestSignatureBase64(50, 50)
		_, _ = svc.UploadSignature(ctx, user.ID, validB64)

		actRepo.listErr = errors.New("db list error")
		_, err := svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorContains(t, err, "list activities")
		actRepo.listErr = nil

		// Seed activities
		actRepo.activities = []models.DailyActivity{{
			ID:       1,
			UserID:   user.ID,
			Date:     time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			Activity: "Work",
			IsActive: true,
		}}

		subRepo.upsertTsErr = errors.New("save ts error")
		_, err = svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorContains(t, err, "save timesheet submission")
		subRepo.upsertTsErr = nil
	})

	t.Run("RevokeTimesheet find error and upsert error", func(t *testing.T) {
		subRepo.findTsErr = errors.New("find ts error")
		err := svc.RevokeTimesheet(ctx, user.ID, &request.RevokeTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorContains(t, err, "find timesheet submission")
		subRepo.findTsErr = nil

		// Sign first to have valid submission
		_, err = svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
		require.NoError(t, err)

		subRepo.upsertTsErr = errors.New("upsert ts error")
		err = svc.RevokeTimesheet(ctx, user.ID, &request.RevokeTimesheetRequest{Month: 10, Year: 2026}, meta)
		assert.ErrorContains(t, err, "revoke timesheet submission")
		subRepo.upsertTsErr = nil

		// Re-sign to restore signed status before successful revoke
		tsResp, err := svc.SignTimesheet(ctx, user.ID, &request.SignTimesheetRequest{Month: 10, Year: 2026}, meta)
		require.NoError(t, err)

		// Successful revoke
		err = svc.RevokeTimesheet(ctx, user.ID, &request.RevokeTimesheetRequest{Month: 10, Year: 2026}, meta)
		require.NoError(t, err)

		// Verify revoked timesheet
		vResp, err := svc.VerifyDocument(ctx, tsResp.VerificationToken)
		require.NoError(t, err)
		assert.False(t, vResp.IsValid)
		assert.Equal(t, "revoked", vResp.Status)
	})

	t.Run("GetTimesheetSubmission find error and existing return", func(t *testing.T) {
		subRepo.findTsErr = errors.New("get ts error")
		_, err := svc.GetTimesheetSubmission(ctx, user.ID, 10, 2026)
		assert.ErrorContains(t, err, "find timesheet submission")
		subRepo.findTsErr = nil

		res, err := svc.GetTimesheetSubmission(ctx, user.ID, 10, 2026)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusRevoked, res.Status)
	})

	t.Run("SubmitBeritaAcara with notifications and error branches", func(t *testing.T) {
		// User has no signature
		userWithoutSig := &models.User{ID: 3, Username: "no_sig", Email: "ns@test.com", IsActive: true}
		_ = userRepo.Create(ctx, userWithoutSig)
		_, err := svc.SubmitBeritaAcara(ctx, userWithoutSig.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: witness.ID}, meta)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		// Activity list error
		actRepo.listErr = errors.New("act list error")
		_, err = svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: witness.ID}, meta)
		assert.ErrorContains(t, err, "list activities")
		actRepo.listErr = nil

		// Seed BA activity
		actRepo.activities = []models.DailyActivity{{
			ID:            10,
			UserID:        user.ID,
			Date:          time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
			Activity:      "BA Work",
			IsBeritaAcara: true,
			IsActive:      true,
		}}

		// Upsert submission error
		subRepo.upsertBaErr = errors.New("upsert ba error")
		_, err = svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: witness.ID}, meta)
		assert.ErrorContains(t, err, "save berita acara submission")
		subRepo.upsertBaErr = nil

		// Successful submit with mailer & push notification dispatch
		baSub, err := svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: witness.ID}, meta)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusPendingWitness, baSub.Status)
	})

	t.Run("WitnessAction find error, approve error, and reject error", func(t *testing.T) {
		// Upload witness signature
		validB64 := generateTestSignatureBase64(50, 50)
		_, _ = svc.UploadSignature(ctx, witness.ID, validB64)

		baSub, err := svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 10, Year: 2026, WitnessID: witness.ID}, meta)
		require.NoError(t, err)

		subRepo.findBaErr = errors.New("find ba error")
		_, err = svc.WitnessAction(ctx, witness.ID, baSub.ID, &request.WitnessActionRequest{Action: "approve"}, meta)
		assert.ErrorContains(t, err, "find berita acara by id")
		subRepo.findBaErr = nil

		// Approve upsert error
		subRepo.upsertBaErr = errors.New("upsert approve error")
		_, err = svc.WitnessAction(ctx, witness.ID, baSub.ID, &request.WitnessActionRequest{Action: "approve"}, meta)
		assert.ErrorContains(t, err, "save witness approval")
		subRepo.upsertBaErr = nil

		// Submit fresh BA for successful approval
		baSubApprove, err := svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 12, Year: 2026, WitnessID: witness.ID}, meta)
		require.NoError(t, err)

		appResp, err := svc.WitnessAction(ctx, witness.ID, baSubApprove.ID, &request.WitnessActionRequest{Action: "approve"}, meta)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusSigned, appResp.Status)

		// New submission for rejection testing
		baSub2, err := svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 11, Year: 2026, WitnessID: witness.ID}, meta)
		require.NoError(t, err)

		subRepo.upsertBaErr = errors.New("upsert reject error")
		_, err = svc.WitnessAction(ctx, witness.ID, baSub2.ID, &request.WitnessActionRequest{Action: "reject", Notes: "perbaiki catatan"}, meta)
		assert.ErrorContains(t, err, "save witness rejection")
		subRepo.upsertBaErr = nil

		// Fresh BA for successful rejection
		baSubRejectFresh, err := svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 7, Year: 2026, WitnessID: witness.ID}, meta)
		require.NoError(t, err)

		rejResp, err := svc.WitnessAction(ctx, witness.ID, baSubRejectFresh.ID, &request.WitnessActionRequest{Action: "reject", Notes: "perbaiki catatan"}, meta)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusRejected, rejResp.Status)

		// Verify rejected BA token
		vResp, err := svc.VerifyDocument(ctx, baSubRejectFresh.VerificationToken)
		require.NoError(t, err)
		assert.False(t, vResp.IsValid)
		assert.Equal(t, "rejected", vResp.Status)
	})

	t.Run("RevokeBeritaAcara find error, forbidden, and upsert error", func(t *testing.T) {
		baSub, err := svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 9, Year: 2026, WitnessID: witness.ID}, meta)
		require.NoError(t, err)

		subRepo.findBaErr = errors.New("find ba error")
		err = svc.RevokeBeritaAcara(ctx, user.ID, baSub.ID, meta)
		assert.ErrorContains(t, err, "find berita acara by id")
		subRepo.findBaErr = nil

		// Forbidden: different user revokes
		err = svc.RevokeBeritaAcara(ctx, witness.ID, baSub.ID, meta)
		assert.ErrorIs(t, err, domain.ErrForbidden)

		// Upsert error
		subRepo.upsertBaErr = errors.New("upsert revoke error")
		err = svc.RevokeBeritaAcara(ctx, user.ID, baSub.ID, meta)
		assert.ErrorContains(t, err, "revoke berita acara")
		subRepo.upsertBaErr = nil

		// Successful revoke
		err = svc.RevokeBeritaAcara(ctx, user.ID, baSub.ID, meta)
		require.NoError(t, err)

		// Verify revoked BA token
		vResp, err := svc.VerifyDocument(ctx, baSub.VerificationToken)
		require.NoError(t, err)
		assert.False(t, vResp.IsValid)
		assert.Equal(t, "revoked", vResp.Status)
	})

	t.Run("GetBeritaAcaraSubmission and ListPendingWitnessRequests error and found branches", func(t *testing.T) {
		subRepo.findBaErr = errors.New("find ba error")
		_, err := svc.GetBeritaAcaraSubmission(ctx, user.ID, 9, 2026)
		assert.ErrorContains(t, err, "find berita acara submission")
		subRepo.findBaErr = nil

		// Existing BA submission with user and witness details
		baResp, err := svc.GetBeritaAcaraSubmission(ctx, user.ID, 9, 2026)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusRevoked, baResp.Status)

		// List pending requests error
		subRepo.listPendErr = errors.New("list pending error")
		_, err = svc.ListPendingWitnessRequests(ctx, witness.ID)
		assert.ErrorContains(t, err, "list pending requests")
		subRepo.listPendErr = nil

		// Submit a pending request to verify list return
		_, err = svc.SubmitBeritaAcara(ctx, user.ID, &request.SubmitBeritaAcaraRequest{Month: 8, Year: 2026, WitnessID: witness.ID}, meta)
		require.NoError(t, err)

		pendingList, err := svc.ListPendingWitnessRequests(ctx, witness.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, pendingList)
	})

	t.Run("VerifyDocument token query error branches", func(t *testing.T) {
		subRepo.findTsTokenErr = errors.New("db ts token error")
		_, err := svc.VerifyDocument(ctx, "any-tok")
		assert.ErrorContains(t, err, "find timesheet by token")
		subRepo.findTsTokenErr = nil

		subRepo.findBaTokenErr = errors.New("db ba token error")
		_, err = svc.VerifyDocument(ctx, "any-tok")
		assert.ErrorContains(t, err, "find berita acara by token")
		subRepo.findBaTokenErr = nil
	})
}

func TestSignatureService_SubmitBeritaAcara_AutoResolveWitness(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	employee := &models.User{
		ID:               1,
		Username:         "emp",
		Name:             "Employee One",
		Email:            "emp@example.com",
		SignatureFileKey: "signatures/1_sig.png",
		IsActive:         true,
	}
	witness := &models.User{
		ID:               2,
		Username:         "wit",
		Name:             "Witness Two",
		Email:            "wit@example.com",
		SignatureFileKey: "signatures/2_sig.png",
		IsActive:         true,
	}
	_ = userRepo.Create(ctx, employee)
	_ = userRepo.Create(ctx, witness)

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")
	meta := service.AuditMeta{IP: "10.0.0.1", UserAgent: "TestClient"}

	// 1. Submit with WitnessID == 0 fails when no monthly witness is assigned
	actRepo.activities = []models.DailyActivity{{
		ID:            1,
		UserID:        employee.ID,
		Date:          time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
		Activity:      "Incident handling",
		IsBeritaAcara: true,
		WitnessID:     nil,
		IsActive:      true,
	}}
	_, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
		Month: 11,
		Year:  2026,
	}, meta)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Belum ada saksi yang ditentukan")

	// 2. Submit with WitnessID == 0 auto-resolves when activity has monthly witness assigned
	actRepo.activities[0].WitnessID = &witness.ID
	submitAutoResp, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
		Month: 11,
		Year:  2026,
	}, meta)
	require.NoError(t, err)
	assert.Equal(t, witness.ID, submitAutoResp.WitnessID)
	assert.Equal(t, models.SubmissionStatusPendingWitness, submitAutoResp.Status)
}

func TestMaskIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"whitespace string", "   ", ""},
		{"IPv4 standard", "192.168.1.100", "192.168.***.***"},
		{"IPv4 loopback", "127.0.0.1", "127.0.***.***"},
		{"IPv4 with port", "192.168.1.100:8080", "192.168.***.***"},
		{"IPv6 standard", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", "2001:db8:****::1"},
		{"IPv6 compressed", "2001:db8::1", "2001:db8:****::1"},
		{"IPv6 with brackets and port", "[2001:db8::1]:443", "2001:db8:****::1"},
		{"IPv6 link-local", "fe80::1", "fe80:0:****::1"},
		{"IPv6 loopback", "::1", "0:0:****::1"},
		{"IPv6-mapped IPv4", "::ffff:192.168.1.1", "192.168.***.***"},
		{"IPv6-mapped IPv4 with port", "[::ffff:192.168.1.1]:8080", "192.168.***.***"},
		{"already masked IPv4", "192.168.***.***", "192.168.***.***"},
		{"non-IP fallback", "unknown-host", "unknown-host"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.MaskIP(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSignatureService_AuditTrail_Comprehensive(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	employee := &models.User{
		ID:               10,
		Username:         "emp10",
		Name:             "Alice Employee",
		Email:            "alice@example.com",
		SignatureFileKey: "signatures/10_sig.png",
		IsActive:         true,
	}
	witness := &models.User{
		ID:               20,
		Username:         "wit20",
		Name:             "Bob Witness",
		Email:            "bob@example.com",
		SignatureFileKey: "signatures/20_sig.png",
		IsActive:         true,
	}
	_ = userRepo.Create(ctx, employee)
	_ = userRepo.Create(ctx, witness)

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")
	meta := service.AuditMeta{IP: "192.168.10.45", UserAgent: "TestBrowser/1.0"}

	t.Run("Timesheet Audit Trail: Submit, Verify, Revoke", func(t *testing.T) {
		// Mock PDF in storage for submission
		storageSvc.files["signatures/10_sig.png"] = []byte("pngdata")
		actRepo.activities = []models.DailyActivity{{
			ID:       101,
			UserID:   employee.ID,
			Date:     time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
			Activity: "Development work",
			IsActive: true,
		}}

		subResp, err := svc.SignTimesheet(ctx, employee.ID, &request.SignTimesheetRequest{
			Month: 10,
			Year:  2026,
		}, meta)
		require.NoError(t, err)
		require.NotEmpty(t, subResp.VerificationToken)

		// 1. Verify Timesheet (Signed state)
		vResp, err := svc.VerifyDocument(ctx, subResp.VerificationToken)
		require.NoError(t, err)
		assert.True(t, vResp.IsValid)
		assert.Equal(t, "valid", vResp.Status)
		require.Len(t, vResp.AuditTrail, 3)

		// Step 1: Submission
		assert.Equal(t, 1, vResp.AuditTrail[0].Step)
		assert.Equal(t, "Timesheet Submitted & Signed", vResp.AuditTrail[0].Title)
		assert.Equal(t, "192.168.***.***", vResp.AuditTrail[0].MaskedIP)
		assert.Equal(t, "completed", vResp.AuditTrail[0].Status)
		assert.Equal(t, "Alice Employee", vResp.AuditTrail[0].Actor)

		// Step 2: SHA-256 Seal
		assert.Equal(t, 2, vResp.AuditTrail[1].Step)
		assert.Equal(t, "Cryptographic SHA-256 Seal", vResp.AuditTrail[1].Title)
		assert.Equal(t, "completed", vResp.AuditTrail[1].Status)

		// Step 3: Public Verification
		assert.Equal(t, 3, vResp.AuditTrail[2].Step)
		assert.Equal(t, "Public Verification Gateway", vResp.AuditTrail[2].Title)
		assert.Equal(t, "completed", vResp.AuditTrail[2].Status)

		// 2. Revoke Timesheet
		revokeMeta := service.AuditMeta{IP: "10.0.5.99:9090", UserAgent: "RevokeAgent/1.0"}
		err = svc.RevokeTimesheet(ctx, employee.ID, &request.RevokeTimesheetRequest{Month: 10, Year: 2026}, revokeMeta)
		require.NoError(t, err)

		// Verify Timesheet (Revoked state)
		vRevoked, err := svc.VerifyDocument(ctx, subResp.VerificationToken)
		require.NoError(t, err)
		assert.False(t, vRevoked.IsValid)
		assert.Equal(t, "revoked", vRevoked.Status)
		require.Len(t, vRevoked.AuditTrail, 3)

		// Step 3 should now be Revocation
		assert.Equal(t, 3, vRevoked.AuditTrail[2].Step)
		assert.Equal(t, "Timesheet Signature Revoked", vRevoked.AuditTrail[2].Title)
		assert.Equal(t, "10.0.***.***", vRevoked.AuditTrail[2].MaskedIP)
		assert.Equal(t, "revoked", vRevoked.AuditTrail[2].Status)
	})

	t.Run("Berita Acara Audit Trail: Submit, Witness Approve, and Revoke", func(t *testing.T) {
		// Mock signatures for Berita Acara
		storageSvc.files["signatures/20_sig.png"] = []byte("pngdata")
		actRepo.activities = []models.DailyActivity{{
			ID:            102,
			UserID:        employee.ID,
			Date:          time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
			Activity:      "Deployment incident",
			IsBeritaAcara: true,
			WitnessID:     &witness.ID,
			IsActive:      true,
		}}

		baSub, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
			Month:     10,
			Year:      2026,
			WitnessID: witness.ID,
		}, service.AuditMeta{IP: "192.168.1.50"})
		require.NoError(t, err)

		// 1. Pending Witness
		vPending, err := svc.VerifyDocument(ctx, baSub.VerificationToken)
		require.NoError(t, err)
		assert.False(t, vPending.IsValid)
		assert.Equal(t, "pending", vPending.Status)
		require.Len(t, vPending.AuditTrail, 3)
		assert.Equal(t, "pending", vPending.AuditTrail[1].Status)
		assert.Equal(t, "Witness Review Pending", vPending.AuditTrail[1].Title)

		// 2. Approve by Witness
		_, err = svc.WitnessAction(ctx, witness.ID, baSub.ID, &request.WitnessActionRequest{
			Action: "approve",
		}, service.AuditMeta{IP: "10.10.20.30"})
		require.NoError(t, err)

		// Verify Approved
		vApproved, err := svc.VerifyDocument(ctx, baSub.VerificationToken)
		require.NoError(t, err)
		assert.True(t, vApproved.IsValid)
		assert.Equal(t, "valid", vApproved.Status)
		require.Len(t, vApproved.AuditTrail, 4)

		assert.Equal(t, "192.168.***.***", vApproved.AuditTrail[0].MaskedIP)
		assert.Equal(t, "10.10.***.***", vApproved.AuditTrail[1].MaskedIP)
		assert.Equal(t, "Endorsed by Witness Signatory", vApproved.AuditTrail[1].Title)
		assert.Equal(t, "Cryptographic SHA-256 Seal", vApproved.AuditTrail[2].Title)
		assert.Equal(t, "Public Verification Gateway", vApproved.AuditTrail[3].Title)

		// 3. Revoke after Approval
		err = svc.RevokeBeritaAcara(ctx, employee.ID, baSub.ID, service.AuditMeta{IP: "172.16.5.1"})
		require.NoError(t, err)

		vRevoked, err := svc.VerifyDocument(ctx, baSub.VerificationToken)
		require.NoError(t, err)
		assert.False(t, vRevoked.IsValid)
		assert.Equal(t, "revoked", vRevoked.Status)
		require.Len(t, vRevoked.AuditTrail, 4)
		// Step 2 remains completed witness signature
		assert.Equal(t, "completed", vRevoked.AuditTrail[1].Status)
		// Step 4 is Document Revoked
		assert.Equal(t, "Document Revoked", vRevoked.AuditTrail[3].Title)
		assert.Equal(t, "172.16.***.***", vRevoked.AuditTrail[3].MaskedIP)
		assert.Equal(t, "revoked", vRevoked.AuditTrail[3].Status)
	})
}

func TestSignatureService_GetBeritaAcaraReview(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	employee := &models.User{
		ID:               10,
		Username:         "emp_ba_review",
		Name:             "Budi Santoso",
		BniID:            "12345",
		Department:       "Engineering",
		GroupName:        "Core Systems",
		SignatureFileKey: "signatures/10_sig.png",
		IsActive:         true,
	}
	witness := &models.User{
		ID:               20,
		Username:         "wit_ba_review",
		Name:             "Dewi Sartika",
		SignatureFileKey: "signatures/20_sig.png",
		IsActive:         true,
	}
	otherUser := &models.User{
		ID:       30,
		Username: "third_party_or_admin",
		Name:     "Admin Or Other",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	_ = userRepo.Create(ctx, employee)
	_ = userRepo.Create(ctx, witness)
	_ = userRepo.Create(ctx, otherUser)

	// Register passkey for witness
	userRepo.passkeys[witness.ID] = []models.WebAuthnCredential{
		{
			ID:           1,
			UserID:       witness.ID,
			CredentialID: []byte("test_cred_id_32_bytes_long_1234"),
			Transports:   datatypes.JSON([]byte(`["internal"]`)),
			FriendlyName: "MacBook Touch ID",
		},
	}

	storageSvc.files["signatures/10_sig.png"] = []byte("pngdata")
	storageSvc.files["signatures/20_sig.png"] = []byte("pngdata")

	// Seed BA incident activity
	actRepo.activities = []models.DailyActivity{
		{
			ID:               101,
			UserID:           employee.ID,
			Date:             time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
			StartTime:        "08:30",
			EndTime:          "17:30",
			Status:           "P",
			Activity:         "Core engine migration",
			BeritaAcaraNotes: "Forgot badge",
			IsBeritaAcara:    true,
			IsActive:         true,
		},
		{
			ID:            102,
			UserID:        employee.ID,
			Date:          time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
			StartTime:     "09:00",
			EndTime:       "18:00",
			Status:        "P",
			Activity:      "Regular work without incident",
			IsBeritaAcara: false,
			IsActive:      true,
		},
	}

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")
	initTime := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	if timeSetter, ok := svc.(interface{ SetNowFunc(func() time.Time) }); ok {
		timeSetter.SetNowFunc(func() time.Time { return initTime })
	}

	// Submit Berita Acara
	subResp, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
		Month:     9,
		Year:      2026,
		WitnessID: witness.ID,
	}, service.AuditMeta{IP: "192.168.1.10", UserAgent: "SubmitAgent/1.0"})
	require.NoError(t, err)
	submissionID := subResp.ID

	// 1. Not Found ID -> returns ErrNotFound
	_, err = svc.GetBeritaAcaraReview(ctx, witness.ID, "non-existent-sub-id", service.AuditMeta{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// 2. Strict IDOR / Role Isolation: Applicant employee attempts to review -> ErrForbidden
	_, err = svc.GetBeritaAcaraReview(ctx, employee.ID, submissionID, service.AuditMeta{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden))

	// 3. Strict IDOR / Role Isolation: Third-party or Admin attempts to review -> ErrForbidden
	_, err = svc.GetBeritaAcaraReview(ctx, otherUser.ID, submissionID, service.AuditMeta{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden))

	// 4. Authorized Witness Reviews -> 200 OK
	metaView1 := service.AuditMeta{IP: "192.168.1.50", UserAgent: "WitnessBrowser/1.0"}
	reviewResp, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, metaView1)
	require.NoError(t, err)

	// Validate review payload fields
	assert.Equal(t, submissionID, reviewResp.SubmissionID)
	assert.Equal(t, submissionID, reviewResp.ID)
	assert.Equal(t, 9, reviewResp.Month)
	assert.Equal(t, 2026, reviewResp.Year)
	assert.Equal(t, models.SubmissionStatusPendingWitness, reviewResp.Status)
	assert.NotEmpty(t, reviewResp.VerificationToken)

	// Applicant data
	assert.Equal(t, employee.ID, reviewResp.Applicant.UserID)
	assert.Equal(t, "Budi Santoso", reviewResp.Applicant.Name)
	assert.Equal(t, "12345", reviewResp.Applicant.NPP)
	assert.Equal(t, "12345", reviewResp.Applicant.BNIID)
	assert.Equal(t, "Engineering", reviewResp.Applicant.Department)
	assert.Equal(t, "Core Systems", reviewResp.Applicant.Group)
	assert.Contains(t, reviewResp.Applicant.SignatureURL, "10_sig.png")

	// Witness data
	assert.Equal(t, witness.ID, reviewResp.Witness.UserID)
	assert.Equal(t, "Dewi Sartika", reviewResp.Witness.Name)
	assert.True(t, reviewResp.Witness.HasSignature)
	assert.True(t, reviewResp.Witness.HasPasskey)
	assert.NotNil(t, reviewResp.Witness.ReviewHistory)

	// Incident rows
	require.Len(t, reviewResp.Incidents, 1)
	assert.Equal(t, "2026-09-05", reviewResp.Incidents[0].Date)
	assert.Equal(t, "Saturday", reviewResp.Incidents[0].Day)
	assert.Equal(t, "08:30", reviewResp.Incidents[0].StartTime)
	assert.Equal(t, "17:30", reviewResp.Incidents[0].EndTime)
	assert.Equal(t, "Forgot badge", reviewResp.Incidents[0].Notes)

	// Instant PDF preview URL
	assert.Equal(t, "/api/v1/berita-acara/"+submissionID+"/preview/pdf", reviewResp.PreviewPDFURL)

	// Signing challenge: 32 raw bytes URL-safe base64
	assert.NotEmpty(t, reviewResp.SigningChallenge)
	decodedChallenge, err := base64.RawURLEncoding.DecodeString(reviewResp.SigningChallenge)
	assert.NoError(t, err)
	assert.Len(t, decodedChallenge, 32)

	// Allowed credentials
	require.Len(t, reviewResp.AllowedCredentials, 1)
	assert.Equal(t, "public-key", reviewResp.AllowedCredentials[0].Type)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString([]byte("test_cred_id_32_bytes_long_1234")), reviewResp.AllowedCredentials[0].ID)
	assert.Equal(t, []string{"internal"}, reviewResp.AllowedCredentials[0].Transports)

	// 5. Verify First-View Tracking Idempotency
	subAfterView1, err := subRepo.FindBeritaAcaraByID(ctx, submissionID)
	require.NoError(t, err)

	var metaMap1 map[string]interface{}
	err = json.Unmarshal(subAfterView1.AuditMetadata, &metaMap1)
	require.NoError(t, err)

	firstViewedAt := metaMap1["witness_viewed_at"]
	assert.NotEmpty(t, firstViewedAt)
	assert.Equal(t, "192.168.1.50", metaMap1["witness_view_ip"])
	assert.Equal(t, "WitnessBrowser/1.0", metaMap1["witness_view_user_agent"])

	// Advance clock by 1 hour and review again with different IP & UA
	secondTime := initTime.Add(1 * time.Hour)
	if timeSetter, ok := svc.(interface{ SetNowFunc(func() time.Time) }); ok {
		timeSetter.SetNowFunc(func() time.Time { return secondTime })
	}

	metaView2 := service.AuditMeta{IP: "10.0.0.99", UserAgent: "DifferentDevice/2.0"}
	reviewResp2, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, metaView2)
	require.NoError(t, err)
	assert.NotEmpty(t, reviewResp2.SigningChallenge)

	subAfterView2, err := subRepo.FindBeritaAcaraByID(ctx, submissionID)
	require.NoError(t, err)

	var metaMap2 map[string]interface{}
	err = json.Unmarshal(subAfterView2.AuditMetadata, &metaMap2)
	require.NoError(t, err)

	// The first view values MUST remain 100% identical to the first call
	assert.Equal(t, firstViewedAt, metaMap2["witness_viewed_at"])
	assert.Equal(t, "192.168.1.50", metaMap2["witness_view_ip"])
	assert.Equal(t, "WitnessBrowser/1.0", metaMap2["witness_view_user_agent"])

	// 6. Edge cases and error branches
	t.Run("FindBeritaAcaraByID error branch", func(t *testing.T) {
		subRepo.findBaErr = errors.New("database disk error")
		defer func() { subRepo.findBaErr = nil }()
		_, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, service.AuditMeta{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "find berita acara by id")
	})

	t.Run("UpsertBeritaAcaraSubmission error branch", func(t *testing.T) {
		subRepo.upsertBaErr = errors.New("upsert failure")
		defer func() { subRepo.upsertBaErr = nil }()
		_, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, service.AuditMeta{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update submission audit metadata")
	})

	t.Run("Applicant user missing", func(t *testing.T) {
		delete(userRepo.users, employee.ID)
		defer func() { userRepo.users[employee.ID] = employee }()
		_, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, service.AuditMeta{})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("Witness user missing", func(t *testing.T) {
		delete(userRepo.users, witness.ID)
		defer func() { userRepo.users[witness.ID] = witness }()
		_, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, service.AuditMeta{})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("Activity repo list error", func(t *testing.T) {
		actRepo.listErr = errors.New("activity table locked")
		defer func() { actRepo.listErr = nil }()
		_, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, service.AuditMeta{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "list activities for review")
	})

	t.Run("Witness without passkeys", func(t *testing.T) {
		delete(userRepo.passkeys, witness.ID)
		defer func() {
			userRepo.passkeys[witness.ID] = []models.WebAuthnCredential{{ID: 1, UserID: witness.ID, CredentialID: []byte("test_cred")}}
		}()
		noPkResp, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, service.AuditMeta{})
		require.NoError(t, err)
		assert.False(t, noPkResp.Witness.HasPasskey)
		assert.Empty(t, noPkResp.AllowedCredentials)
	})

	t.Run("Activity fallback notes and prior rejection history", func(t *testing.T) {
		savedActs := actRepo.activities
		actRepo.activities = []models.DailyActivity{{
			ID:            103,
			UserID:        employee.ID,
			Date:          time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC),
			StartTime:     "08:00",
			EndTime:       "17:00",
			Activity:      "Fallback Activity Description",
			IsBeritaAcara: true,
			IsActive:      true,
		}}
		defer func() { actRepo.activities = savedActs }()

		subRecord, err := subRepo.FindBeritaAcaraByID(ctx, submissionID)
		require.NoError(t, err)
		subRecord.WitnessRejectionNotes = "Please fix day 5"
		_ = subRepo.UpsertBeritaAcaraSubmission(ctx, subRecord)

		fallbackResp, err := svc.GetBeritaAcaraReview(ctx, witness.ID, submissionID, service.AuditMeta{})
		require.NoError(t, err)
		require.Len(t, fallbackResp.Incidents, 1)
		assert.Equal(t, "Fallback Activity Description", fallbackResp.Incidents[0].Notes)
		assert.Equal(t, []string{"Please fix day 5"}, fallbackResp.Witness.ReviewHistory)
		assert.Equal(t, "Please fix day 5", fallbackResp.Witness.WitnessRejectionNotes)
	})
}

func TestSignatureService_AuditTrail_WitnessViewed(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()

	employee := &models.User{
		ID:               11,
		Username:         "emp11",
		Name:             "Charlie Emp",
		SignatureFileKey: "signatures/11_sig.png",
		IsActive:         true,
	}
	witness := &models.User{
		ID:               21,
		Username:         "wit21",
		Name:             "Dana Wit",
		SignatureFileKey: "signatures/21_sig.png",
		IsActive:         true,
	}
	_ = userRepo.Create(ctx, employee)
	_ = userRepo.Create(ctx, witness)

	actRepo.activities = []models.DailyActivity{{
		ID:            201,
		UserID:        employee.ID,
		Date:          time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
		Activity:      "Deployment issue",
		IsBeritaAcara: true,
		WitnessID:     &witness.ID,
		IsActive:      true,
	}}

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")
	sub, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
		Month:     10,
		Year:      2026,
		WitnessID: witness.ID,
	}, service.AuditMeta{IP: "192.168.10.11"})
	require.NoError(t, err)

	// Case 1: Before witness review, AuditTrail has 3 steps (no witness viewed step)
	vBefore, err := svc.VerifyDocument(ctx, sub.VerificationToken)
	require.NoError(t, err)
	assert.Len(t, vBefore.AuditTrail, 3)
	assert.Equal(t, 1, vBefore.AuditTrail[0].Step)
	assert.Equal(t, "Document Submitted & Signed", vBefore.AuditTrail[0].Title)
	assert.Equal(t, 2, vBefore.AuditTrail[1].Step)
	assert.Equal(t, "Witness Review Pending", vBefore.AuditTrail[1].Title)

	// Case 2: Witness reviews the document
	_, err = svc.GetBeritaAcaraReview(ctx, witness.ID, sub.ID, service.AuditMeta{
		IP:        "192.168.20.22",
		UserAgent: "DanaBrowser/1.0",
	})
	require.NoError(t, err)

	// Now VerifyDocument should show 4 steps, including "Document Viewed by Witness"
	vAfter, err := svc.VerifyDocument(ctx, sub.VerificationToken)
	require.NoError(t, err)
	require.Len(t, vAfter.AuditTrail, 4)

	// Step 1: Employee submit
	assert.Equal(t, 1, vAfter.AuditTrail[0].Step)
	assert.Equal(t, "Document Submitted & Signed", vAfter.AuditTrail[0].Title)

	// Step 2: Document Viewed by Witness
	assert.Equal(t, 2, vAfter.AuditTrail[1].Step)
	assert.Equal(t, "Document Viewed by Witness", vAfter.AuditTrail[1].Title)
	assert.Equal(t, "Dana Wit", vAfter.AuditTrail[1].Actor)
	assert.Equal(t, "Witness", vAfter.AuditTrail[1].Role)
	assert.Equal(t, "192.168.***.***", vAfter.AuditTrail[1].MaskedIP)
	assert.Equal(t, "completed", vAfter.AuditTrail[1].Status)

	// Step 3: Witness Review Pending
	assert.Equal(t, 3, vAfter.AuditTrail[2].Step)
	assert.Equal(t, "Witness Review Pending", vAfter.AuditTrail[2].Title)

	// Step 4: Cryptographic SHA-256 Seal
	assert.Equal(t, 4, vAfter.AuditTrail[3].Step)
	assert.Equal(t, "Cryptographic SHA-256 Seal", vAfter.AuditTrail[3].Title)

	// Case 3: IPv6 witness view masking
	sub2, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
		Month:     11,
		Year:      2026,
		WitnessID: witness.ID,
	}, service.AuditMeta{IP: "10.0.0.1"})
	require.NoError(t, err)

	_, err = svc.GetBeritaAcaraReview(ctx, witness.ID, sub2.ID, service.AuditMeta{
		IP:        "2001:db8:85a3::8a2e:370:7334",
		UserAgent: "IPv6Browser/1.0",
	})
	require.NoError(t, err)

	vIPv6, err := svc.VerifyDocument(ctx, sub2.VerificationToken)
	require.NoError(t, err)
	require.Len(t, vIPv6.AuditTrail, 4)
	assert.Equal(t, "Document Viewed by Witness", vIPv6.AuditTrail[1].Title)
	assert.Contains(t, vIPv6.AuditTrail[1].MaskedIP, "****")
}

func createTestPasskey(t *testing.T, userID uint, credIDStr, friendlyName string) (*ecdsa.PrivateKey, models.WebAuthnCredential) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)

	credID := []byte(credIDStr)
	aaguid := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	aaguidStr := "01020304-0506-0708-090a-0b0c0d0e0f10"

	cred := models.WebAuthnCredential{
		ID:                  10,
		UserID:              userID,
		CredentialID:        credID,
		PublicKey:           pubBytes,
		AttestationType:     "none",
		AAGUID:              aaguid,
		AuthenticatorAAGUID: &aaguidStr,
		SignCount:           1,
		FriendlyName:        friendlyName,
	}
	return priv, cred
}

func signWebAuthnAssertion(
	t *testing.T,
	priv *ecdsa.PrivateKey,
	credID []byte,
	challenge string,
	origin string,
	counter uint32,
	userPresent bool,
) *request.WebAuthnAssertionPayload {
	t.Helper()
	clientData := fmt.Sprintf(`{"type":"webauthn.get","challenge":"%s","origin":"%s"}`, challenge, origin)
	clientDataHash := sha256.Sum256([]byte(clientData))

	authData := make([]byte, 37)
	rpHash := sha256.Sum256([]byte("localhost"))
	copy(authData[0:32], rpHash[:])
	if userPresent {
		authData[32] = 0x01 // UP bit
	} else {
		authData[32] = 0x00
	}
	authData[33] = byte(counter >> 24)
	authData[34] = byte(counter >> 16)
	authData[35] = byte(counter >> 8)
	authData[36] = byte(counter)

	verificationData := append(authData, clientDataHash[:]...)
	digest := sha256.Sum256(verificationData)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	require.NoError(t, err)

	return &request.WebAuthnAssertionPayload{
		CredentialID:      base64.RawURLEncoding.EncodeToString(credID),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData),
		ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(clientData)),
		Signature:         base64.RawURLEncoding.EncodeToString(sig),
	}
}

func TestSignatureService_WitnessAction_WebAuthn(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	actRepo := newMockActivityRepo()
	subRepo := newMockSubmissionRepo()
	storageSvc := newMockStorageSvc()
	meta := service.AuditMeta{IP: "192.168.1.100", UserAgent: "TestClient/1.0"}

	employee := &models.User{
		ID:               201,
		Username:         "emp201",
		Name:             "Alice Applicant",
		Email:            "alice@example.com",
		SignatureFileKey: "signatures/alice.png",
		IsActive:         true,
	}
	witness := &models.User{
		ID:               202,
		Username:         "wit202",
		Name:             "Bob Witness",
		Email:            "bob@example.com",
		SignatureFileKey: "signatures/bob.png",
		IsActive:         true,
	}
	require.NoError(t, userRepo.Create(ctx, employee))
	require.NoError(t, userRepo.Create(ctx, witness))

	// Register a hardware passkey for witness
	privKey, cred := createTestPasskey(t, witness.ID, "yubikey-cred-202", "YubiKey 5 NFC")
	userRepo.passkeys[witness.ID] = []models.WebAuthnCredential{cred}

	svc := service.NewSignatureService(userRepo, actRepo, subRepo, storageSvc, nil, nil, "https://timesheet.local")

	// Seed Berita Acara activity
	actRepo.activities = []models.DailyActivity{{
		ID:            201,
		UserID:        employee.ID,
		Date:          time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		Activity:      "System maintenance anomaly",
		IsBeritaAcara: true,
		IsActive:      true,
	}}

	// Helper to submit and review a fresh Berita Acara
	setupSubmissionAndReview := func(month, year int) (*response.BeritaAcaraSubmissionResponse, *response.BeritaAcaraReviewResponse) {
		subResp, err := svc.SubmitBeritaAcara(ctx, employee.ID, &request.SubmitBeritaAcaraRequest{
			Month:     month,
			Year:      year,
			WitnessID: witness.ID,
		}, meta)
		require.NoError(t, err)

		reviewResp, err := svc.GetBeritaAcaraReview(ctx, witness.ID, subResp.ID, meta)
		require.NoError(t, err)
		require.NotEmpty(t, reviewResp.SigningChallenge)
		return subResp, reviewResp
	}

	t.Run("tampered challenge rejection", func(t *testing.T) {
		subResp, _ := setupSubmissionAndReview(1, 2026)
		tamperedAssertion := signWebAuthnAssertion(t, privKey, cred.CredentialID, "invalid_or_tampered_challenge", "https://timesheet.local", 2, true)

		_, err := svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{
			Action:    "approve",
			Assertion: tamperedAssertion,
		}, meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Signing challenge")

		// Ensure submission remains pending
		checkSub, err := subRepo.FindBeritaAcaraByID(ctx, subResp.ID)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusPendingWitness, checkSub.Status)
		assert.Nil(t, checkSub.WitnessSignedAt)
	})

	t.Run("unknown credential rejection", func(t *testing.T) {
		subResp, reviewResp := setupSubmissionAndReview(2, 2026)
		unknownAssertion := signWebAuthnAssertion(t, privKey, []byte("unknown-credential-id"), reviewResp.SigningChallenge, "https://timesheet.local", 2, true)

		_, err := svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{
			Action:    "approve",
			Assertion: unknownAssertion,
		}, meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Kredensial passkey tidak ditemukan")

		checkSub, err := subRepo.FindBeritaAcaraByID(ctx, subResp.ID)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusPendingWitness, checkSub.Status)
	})

	t.Run("missing user presence rejection", func(t *testing.T) {
		subResp, reviewResp := setupSubmissionAndReview(3, 2026)
		noPresenceAssertion := signWebAuthnAssertion(t, privKey, cred.CredentialID, reviewResp.SigningChallenge, "https://timesheet.local", 2, false)

		_, err := svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{
			Action:    "approve",
			Assertion: noPresenceAssertion,
		}, meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "User presence")

		checkSub, err := subRepo.FindBeritaAcaraByID(ctx, subResp.ID)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusPendingWitness, checkSub.Status)
	})

	t.Run("valid passkey assertion approval", func(t *testing.T) {
		subResp, reviewResp := setupSubmissionAndReview(4, 2026)
		validAssertion := signWebAuthnAssertion(t, privKey, cred.CredentialID, reviewResp.SigningChallenge, "https://timesheet.local", 15, true)

		actionResp, err := svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{
			Action:    "approve",
			Notes:     "Approved with hardware security key",
			Assertion: validAssertion,
		}, meta)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusSigned, actionResp.Status)
		assert.NotNil(t, actionResp.WitnessSignedAt)
		assert.NotEmpty(t, actionResp.DocumentHash)

		// Verify SignCount update
		require.Len(t, userRepo.passkeys[witness.ID], 1)
		assert.Equal(t, uint32(15), userRepo.passkeys[witness.ID][0].SignCount)

		// Verify Document Verification and Audit Trail Milestone
		vDoc, err := svc.VerifyDocument(ctx, actionResp.VerificationToken)
		require.NoError(t, err)
		assert.True(t, vDoc.IsValid)
		assert.Equal(t, "valid", vDoc.Status)

		// Find the endorsement step
		var witnessStep *response.DocumentAuditTrailItem
		for i := range vDoc.AuditTrail {
			item := &vDoc.AuditTrail[i]
			if item.Role == "Witness" && item.Title != "Document Viewed by Witness" {
				witnessStep = item
				break
			}
		}
		require.NotNil(t, witnessStep, "expected witness endorsement step in audit trail")
		assert.Equal(t, "Endorsed with Hardware Passkey (Non-Repudiation Verified)", witnessStep.Title)
		assert.Contains(t, witnessStep.Description, "YubiKey 5 NFC")
		assert.Equal(t, witness.Name, witnessStep.Actor)
	})

	t.Run("graceful fallback for nil assertion", func(t *testing.T) {
		subResp, _ := setupSubmissionAndReview(5, 2026)

		actionResp, err := svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{
			Action: "approve",
			Notes:  "Standard approval without hardware key",
		}, meta)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusSigned, actionResp.Status)

		// Verify Document Audit Trail shows standard milestone
		vDoc, err := svc.VerifyDocument(ctx, actionResp.VerificationToken)
		require.NoError(t, err)
		assert.True(t, vDoc.IsValid)

		var witnessStep *response.DocumentAuditTrailItem
		for i := range vDoc.AuditTrail {
			item := &vDoc.AuditTrail[i]
			if item.Role == "Witness" && item.Title != "Document Viewed by Witness" {
				witnessStep = item
				break
			}
		}
		require.NotNil(t, witnessStep)
		assert.Equal(t, "Endorsed by Witness Signatory", witnessStep.Title)
		assert.Equal(t, fmt.Sprintf("Approved and signed by %s", witness.Name), witnessStep.Description)
	})

	t.Run("rejection flow", func(t *testing.T) {
		subResp, _ := setupSubmissionAndReview(6, 2026)

		rejectResp, err := svc.WitnessAction(ctx, witness.ID, subResp.ID, &request.WitnessActionRequest{
			Action: "reject",
			Notes:  "Please revise incident hours for day 10",
		}, meta)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusRejected, rejectResp.Status)
		assert.Equal(t, "Please revise incident hours for day 10", rejectResp.WitnessRejectionNotes)

		checkSub, err := subRepo.FindBeritaAcaraByID(ctx, subResp.ID)
		require.NoError(t, err)
		assert.Equal(t, models.SubmissionStatusRejected, checkSub.Status)
		assert.Equal(t, "Please revise incident hours for day 10", checkSub.WitnessRejectionNotes)
	})
}
