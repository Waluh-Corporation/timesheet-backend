package service_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
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
	updateSigErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[uint]*models.User),
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
