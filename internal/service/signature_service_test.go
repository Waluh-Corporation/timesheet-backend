package service_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
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
	tsSubmissions map[string]*models.TimesheetSubmission
	baSubmissions map[string]*models.BeritaAcaraSubmission
	tsLocked      map[string]bool
	baLocked      map[string]bool
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
	for _, s := range m.tsSubmissions {
		if s.UserID == userID && s.Month == month && s.Year == year {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) FindTimesheetByToken(ctx context.Context, token string) (*models.TimesheetSubmission, error) {
	for _, s := range m.tsSubmissions {
		if s.VerificationToken == token {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) UpsertTimesheetSubmission(ctx context.Context, sub *models.TimesheetSubmission) error {
	m.tsSubmissions[sub.ID] = sub
	if sub.Status == models.SubmissionStatusSigned || sub.Status == models.SubmissionStatusSubmitted {
		m.tsLocked[sub.ID] = true
	} else {
		m.tsLocked[sub.ID] = false
	}
	return nil
}

func (m *mockSubmissionRepo) FindBeritaAcaraSubmission(ctx context.Context, userID uint, month, year int) (*models.BeritaAcaraSubmission, error) {
	for _, s := range m.baSubmissions {
		if s.UserID == userID && s.Month == month && s.Year == year {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) FindBeritaAcaraByID(ctx context.Context, id string) (*models.BeritaAcaraSubmission, error) {
	s, ok := m.baSubmissions[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockSubmissionRepo) FindBeritaAcaraByToken(ctx context.Context, token string) (*models.BeritaAcaraSubmission, error) {
	for _, s := range m.baSubmissions {
		if s.VerificationToken == token {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSubmissionRepo) UpsertBeritaAcaraSubmission(ctx context.Context, sub *models.BeritaAcaraSubmission) error {
	m.baSubmissions[sub.ID] = sub
	if sub.Status == models.SubmissionStatusPendingWitness || sub.Status == models.SubmissionStatusSigned {
		m.baLocked[sub.ID] = true
	} else {
		m.baLocked[sub.ID] = false
	}
	return nil
}

func (m *mockSubmissionRepo) ListPendingWitnessRequests(ctx context.Context, witnessID uint) ([]models.BeritaAcaraSubmission, error) {
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
	files map[string][]byte
}

func newMockStorageSvc() *mockStorageSvc {
	return &mockStorageSvc{files: make(map[string][]byte)}
}

func (m *mockStorageSvc) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
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
	repository.UserRepository
	users map[uint]*models.User
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
}

func newMockActivityRepo() *mockActivityRepo {
	return &mockActivityRepo{}
}

func (m *mockActivityRepo) ListActiveByUser(ctx context.Context, userID uint, filter repository.ActivityFilter) ([]models.DailyActivity, int64, error) {
	return m.activities, int64(len(m.activities)), nil
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
