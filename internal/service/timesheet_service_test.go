package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
	"timesheet-backend/storage"
)

func setupTestDBForService(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}
	return db
}

func TestTimesheetService_OvertimeAndWorkbook(t *testing.T) {
	db := setupTestDBForService(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	actRepo := repository.NewActivityRepository(tx)
	otRepo := repository.NewOvertimeRepository(tx)
	masterRepo := repository.NewMasterRepository(tx)
	svc := service.NewTimesheetService(userRepo, actRepo, otRepo, masterRepo, nil)
	if tsImpl, ok := svc.(interface{ SetNowFunc(func() time.Time) }); ok {
		tsImpl.SetNowFunc(func() time.Time {
			return time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
		})
	}
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Username:   "ts_svc_user",
		Email:      "ts_svc@example.com",
		Role:       models.RoleUser,
		Name:       "Timesheet User",
		Company:    "sdd",
		IsActive:   true,
		EmployeeID: "EMP-TS-1",
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	t.Run("Overtime_Validation", func(t *testing.T) {
		testOvertimeValidation(t, svc, ctx, user.ID)
	})
	t.Run("Overtime_Lifecycle", func(t *testing.T) {
		testOvertimeLifecycle(t, svc, ctx, user.ID)
	})
	t.Run("Workbook_Validation", func(t *testing.T) {
		testWorkbookValidation(t, svc, ctx, user.ID)
	})
	t.Run("Workbook_Generation", func(t *testing.T) {
		testWorkbookGenerationSuccess(t, svc, tx, ctx, user.ID)
	})
	t.Run("BeritaAcara_Generation", func(t *testing.T) {
		testBeritaAcaraGeneration(t, svc, tx, ctx, user.ID)
	})
}

func testOvertimeValidation(t *testing.T, svc service.TimesheetService, ctx context.Context, userID uint) {
	t.Helper()
	if err := svc.UpsertOvertime(ctx, userID, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil req, got %v", err)
	}
	invalidDateReq := &request.OvertimeRequest{Date: "invalid-date"}
	if err := svc.UpsertOvertime(ctx, userID, invalidDateReq); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid date, got %v", err)
	}
	otInvalidTime := &request.OvertimeRequest{
		Date:            "2026-06-16",
		StartTime:       "20:00",
		EndTime:         "18:00",
		TaskDescription: "Invalid overtime hours",
	}
	if err := svc.UpsertOvertime(ctx, userID, otInvalidTime); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for start > end, got %v", err)
	}
}

func testOvertimeLifecycle(t *testing.T, svc service.TimesheetService, ctx context.Context, userID uint) {
	t.Helper()
	validReq := &request.OvertimeRequest{
		Date:            "2026-06-15",
		StartTime:       "17:00",
		EndTime:         "21:00",
		TaskDescription: "Overtime Task 1",
	}
	if err := svc.UpsertOvertime(ctx, userID, validReq); err != nil {
		t.Fatalf("UpsertOvertime create failed: %v", err)
	}

	validReq.TaskDescription = "Overtime Task 1 Updated"
	if err := svc.UpsertOvertime(ctx, userID, validReq); err != nil {
		t.Fatalf("UpsertOvertime update failed: %v", err)
	}

	ots, err := svc.ListMonthlyOvertimes(ctx, userID, 6, 2026)
	if err != nil {
		t.Fatalf("ListMonthlyOvertimes failed: %v", err)
	}
	if len(ots) != 1 || ots[0].TaskDescription != "Overtime Task 1 Updated" {
		t.Fatalf("unexpected overtime list: %+v", ots)
	}

	if _, err := svc.ListMonthlyOvertimes(ctx, userID, 0, 0); err != nil {
		t.Fatalf("ListMonthlyOvertimes with defaults failed: %v", err)
	}

	otID := ots[0].ID
	if err := svc.DeleteOvertime(ctx, otID, userID); err != nil {
		t.Fatalf("DeleteOvertime failed: %v", err)
	}
	if err := svc.DeleteOvertime(ctx, 999999, userID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func testWorkbookValidation(t *testing.T, svc service.TimesheetService, ctx context.Context, userID uint) {
	t.Helper()
	if _, _, err := svc.GenerateWorkbook(ctx, 999999, 6, 2026); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent user, got %v", err)
	}
	if _, _, err := svc.GenerateWorkbook(ctx, userID, 13, 2026); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for month 13, got %v", err)
	}
	if _, _, err := svc.GenerateWorkbook(ctx, userID, 6, 1999); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for year 1999, got %v", err)
	}
	if _, _, err := svc.GenerateWorkbook(ctx, userID, 6, 2026); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when activities are empty, got %v", err)
	}
}

func testWorkbookGenerationSuccess(t *testing.T, svc service.TimesheetService, tx *gorm.DB, ctx context.Context, userID uint) {
	t.Helper()
	act := &models.DailyActivity{
		UserID:    userID,
		Date:      time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "Feature development",
		IsActive:  true,
	}
	if err := tx.Create(act).Error; err != nil {
		t.Fatalf("failed to create activity: %v", err)
	}

	wb, filename, err := svc.GenerateWorkbook(ctx, userID, 6, 2026)
	if err != nil {
		t.Fatalf("GenerateWorkbook failed: %v", err)
	}
	if len(wb) == 0 || filename == "" {
		t.Fatalf("expected non-empty workbook and filename, got len %d, filename %s", len(wb), filename)
	}
}

func testBeritaAcaraGeneration(t *testing.T, svc service.TimesheetService, tx *gorm.DB, ctx context.Context, userID uint) {
	t.Helper()

	// 1. Invalid input validation
	if _, _, err := svc.GenerateBeritaAcara(ctx, 999999, 6, 2026); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent user, got %v", err)
	}
	if _, _, err := svc.GenerateBeritaAcara(ctx, userID, 13, 2026); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for month 13, got %v", err)
	}

	// 2. User with no Berita Acara activities returns nil, "", nil
	doc, fname, err := svc.GenerateBeritaAcara(ctx, userID, 6, 2026)
	if err != nil {
		t.Fatalf("unexpected error when no BA activities: %v", err)
	}
	if doc != nil || fname != "" {
		t.Fatalf("expected nil doc and empty fname when no BA activities, got len %d, fname %s", len(doc), fname)
	}

	// 3. User with a Berita Acara activity generates docx successfully
	witness := &models.User{
		Username: "ba_witness_user",
		Email:    "witness@example.com",
		Role:     models.RoleUser,
		Name:     "Saksi Berita Acara",
		Company:  "sdd",
		IsActive: true,
	}
	if err := tx.Create(witness).Error; err != nil {
		t.Fatalf("failed to create witness: %v", err)
	}

	baAct := &models.DailyActivity{
		UserID:           userID,
		Date:             time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
		StartTime:        "08:00",
		EndTime:          "17:00",
		Status:           "P",
		Activity:         "Critical hotfix deployment",
		IsActive:         true,
		IsBeritaAcara:    true,
		WitnessID:        &witness.ID,
		BeritaAcaraNotes: "Deployment went smoothly without downtime",
	}
	if err := tx.Create(baAct).Error; err != nil {
		t.Fatalf("failed to create BA activity: %v", err)
	}

	doc, fname, err = svc.GenerateBeritaAcara(ctx, userID, 6, 2026)
	if err != nil {
		t.Fatalf("GenerateBeritaAcara failed: %v", err)
	}
	if len(doc) == 0 || fname == "" {
		t.Fatalf("expected non-empty doc and filename, got len %d, filename %s", len(doc), fname)
	}
}

func TestTimesheetService_SignedDocuments(t *testing.T) {
	db := setupTestDBForService(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	actRepo := repository.NewActivityRepository(tx)
	otRepo := repository.NewOvertimeRepository(tx)
	masterRepo := repository.NewMasterRepository(tx)
	subRepo := repository.NewSubmissionRepository(tx)
	memStorage := newMockStorageSvc()

	memStorage.files["signatures/u1.png"] = []byte("png-bytes-u1")
	memStorage.files["signatures/w1.png"] = []byte("png-bytes-w1")

	svc := service.NewTimesheetService(userRepo, actRepo, otRepo, masterRepo, nil)
	if tsImpl, ok := svc.(interface{ SetNowFunc(func() time.Time) }); ok {
		tsImpl.SetNowFunc(func() time.Time {
			return time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
		})
	}
	if depSetter, ok := svc.(interface {
		SetSignatureDependencies(repository.SubmissionRepository, storage.StorageService, string)
	}); ok {
		depSetter.SetSignatureDependencies(subRepo, memStorage, "https://timesheet.local")
	}

	ctx := context.Background()

	u := &models.User{
		Username:         "ts_signed_user",
		Email:            "ts_signed@example.com",
		Role:             models.RoleUser,
		Name:             "Signed User",
		Company:          "sdd",
		IsActive:         true,
		EmployeeID:       "EMP-SIGNED-1",
		SignatureFileKey: "signatures/u1.png",
	}
	require.NoError(t, tx.Create(u).Error)

	w := &models.User{
		Username:         "ts_signed_wit",
		Email:            "ts_signed_wit@example.com",
		Role:             models.RoleUser,
		Name:             "Witness User",
		Company:          "sdd",
		IsActive:         true,
		EmployeeID:       "EMP-SIGNED-WIT",
		SignatureFileKey: "signatures/w1.png",
	}
	require.NoError(t, tx.Create(w).Error)

	act := &models.DailyActivity{
		UserID:           u.ID,
		Date:             time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		StartTime:        "08:00",
		EndTime:          "17:00",
		Status:           "P",
		Activity:         "Signed feature work",
		IsActive:         true,
		IsBeritaAcara:    true,
		WitnessID:        &w.ID,
		BeritaAcaraNotes: "BA incident resolved",
	}
	require.NoError(t, tx.Create(act).Error)

	now := time.Now()
	// Create signed timesheet submission
	tsSub := &models.TimesheetSubmission{
		ID:                "ts-sub-1",
		UserID:            u.ID,
		Month:             6,
		Year:              2026,
		Status:            models.SubmissionStatusSigned,
		SignedAt:          &now,
		VerificationToken: "verif-token-ts-1",
		DocumentHash:      "hash-ts-1",
	}
	require.NoError(t, subRepo.UpsertTimesheetSubmission(ctx, tsSub))

	// GenerateWorkbook should include signature and QR
	wb, fname, err := svc.GenerateWorkbook(ctx, u.ID, 6, 2026)
	require.NoError(t, err)
	assert.NotEmpty(t, wb)
	assert.NotEmpty(t, fname)

	// Create signed Berita Acara submission
	baSub := &models.BeritaAcaraSubmission{
		ID:                "ba-sub-1",
		UserID:            u.ID,
		WitnessID:         w.ID,
		Month:             6,
		Year:              2026,
		Status:            models.SubmissionStatusSigned,
		EmployeeSignedAt:  &now,
		WitnessSignedAt:   &now,
		VerificationToken: "verif-token-ba-1",
		DocumentHash:      "hash-ba-1",
	}
	require.NoError(t, subRepo.UpsertBeritaAcaraSubmission(ctx, baSub))

	// GenerateBeritaAcara should include employee signature, witness signature and QR
	doc, bname, err := svc.GenerateBeritaAcara(ctx, u.ID, 6, 2026)
	require.NoError(t, err)
	assert.NotEmpty(t, doc)
	assert.NotEmpty(t, bname)
}

type mockBundlingTimesheetService struct {
	wbBytes  []byte
	wbName   string
	wbErr    error
	baBytes  []byte
	baName   string
	baErr    error
	pdfBytes []byte
	pdfName  string
	pdfErr   error
	baPdf    []byte
	baPdfNm  string
	baPdfErr error
}

func (m *mockBundlingTimesheetService) GenerateWorkbook(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	if m.wbErr != nil {
		return nil, "", m.wbErr
	}
	name := m.wbName
	if name == "" {
		name = "Timesheet_mock_06_2026.xlsx"
	}
	return m.wbBytes, name, nil
}

func (m *mockBundlingTimesheetService) GenerateBeritaAcara(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	if m.baErr != nil {
		return nil, "", m.baErr
	}
	return m.baBytes, m.baName, nil
}

func (m *mockBundlingTimesheetService) UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error {
	return nil
}

func (m *mockBundlingTimesheetService) ListMonthlyOvertimes(ctx context.Context, userID uint, month int, year int) ([]response.OvertimeResponse, error) {
	return nil, nil
}

func (m *mockBundlingTimesheetService) DeleteOvertime(ctx context.Context, id uint, userID uint) error {
	return nil
}

type mockBundlingWithPDFService struct {
	mockBundlingTimesheetService
}

func (m *mockBundlingWithPDFService) GenerateTimesheetPDF(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	if m.pdfErr != nil {
		return nil, "", m.pdfErr
	}
	out := m.pdfBytes
	if len(out) == 0 {
		out = []byte("%PDF-1.4 mock timesheet pdf")
	}
	name := m.pdfName
	if name == "" {
		name = "Timesheet_mock_06_2026.pdf"
	}
	return out, name, nil
}

func (m *mockBundlingWithPDFService) GenerateBeritaAcaraPDF(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	if m.baPdfErr != nil {
		return nil, "", m.baPdfErr
	}
	out := m.baPdf
	if len(out) == 0 {
		out = []byte("%PDF-1.4 mock ba pdf")
	}
	name := m.baPdfNm
	if name == "" {
		name = "Berita_Acara_mock_06_2026.pdf"
	}
	return out, name, nil
}

func TestTimesheetService_PackageTimesheetBundle_Fallback(t *testing.T) {
	ctx := context.Background()

	t.Run("Fallback without PDF and without BA", func(t *testing.T) {
		svc := &mockBundlingTimesheetService{
			wbBytes: []byte("mock-xlsx"),
			wbName:  "Timesheet_test_06_2026.xlsx",
		}
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, 1, 6, 2026, false)
		require.NoError(t, err)
		assert.Equal(t, "Timesheet_test_06_2026.xlsx", fname)
		assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ctype)
		assert.Equal(t, []byte("mock-xlsx"), data)
	})

	t.Run("Fallback without PDF and with BA", func(t *testing.T) {
		svc := &mockBundlingTimesheetService{
			wbBytes: []byte("mock-xlsx"),
			wbName:  "Timesheet_test_06_2026.xlsx",
			baBytes: []byte("mock-ba-docx"),
			baName:  "Berita_Acara_test_06_2026.docx",
		}
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, 1, 6, 2026, false)
		require.NoError(t, err)
		assert.Equal(t, "Timesheet_test_06_2026_and_Berita_Acara.zip", fname)
		assert.Equal(t, "application/zip", ctype)

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		assert.Len(t, zr.File, 2)
	})

	t.Run("Fallback with PDF when service does not implement PDFGenerator returns error", func(t *testing.T) {
		svc := &mockBundlingTimesheetService{
			wbBytes: []byte("mock-xlsx"),
		}
		_, _, _, err := service.PackageTimesheetBundle(ctx, svc, 1, 6, 2026, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not support PDF generation")
	})

	t.Run("Fallback with PDF and without BA", func(t *testing.T) {
		svc := &mockBundlingWithPDFService{
			mockBundlingTimesheetService: mockBundlingTimesheetService{
				wbBytes: []byte("mock-xlsx"),
				wbName:  "Timesheet_test_06_2026.xlsx",
			},
		}
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, 1, 6, 2026, true)
		require.NoError(t, err)
		assert.Equal(t, "Timesheet_test_06_2026_with_PDF.zip", fname)
		assert.Equal(t, "application/zip", ctype)

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		assert.Len(t, zr.File, 2)
	})

	t.Run("Fallback with PDF and with BA", func(t *testing.T) {
		svc := &mockBundlingWithPDFService{
			mockBundlingTimesheetService: mockBundlingTimesheetService{
				wbBytes: []byte("mock-xlsx"),
				wbName:  "Timesheet_test_06_2026.xlsx",
				baBytes: []byte("mock-ba-docx"),
				baName:  "Berita_Acara_test_06_2026.docx",
			},
		}
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, 1, 6, 2026, true)
		require.NoError(t, err)
		assert.Equal(t, "Timesheet_test_06_2026_with_PDF.zip", fname)
		assert.Equal(t, "application/zip", ctype)

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		assert.Len(t, zr.File, 4)
	})
}

func TestTimesheetService_PackageTimesheetBundle_Integration(t *testing.T) {
	db := setupTestDBForService(t)
	tx := db.Begin()
	defer tx.Rollback()

	userRepo := repository.NewUserRepository(tx)
	actRepo := repository.NewActivityRepository(tx)
	otRepo := repository.NewOvertimeRepository(tx)
	masterRepo := repository.NewMasterRepository(tx)

	svc := service.NewTimesheetService(userRepo, actRepo, otRepo, masterRepo, nil)
	if tsImpl, ok := svc.(interface{ SetNowFunc(func() time.Time) }); ok {
		tsImpl.SetNowFunc(func() time.Time {
			return time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
		})
	}
	ctx := context.Background()

	// Seed user with company
	user := &models.User{
		Username:   "bundle_user",
		Email:      "bundle@example.com",
		Role:       models.RoleUser,
		Name:       "Bundle User",
		Company:    "sdd",
		IsActive:   true,
		EmployeeID: "EMP-BND-1",
	}
	require.NoError(t, tx.Create(user).Error)

	// Seed standard activity
	act := &models.DailyActivity{
		UserID:    user.ID,
		Date:      time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "Standard development activity",
		IsActive:  true,
	}
	require.NoError(t, tx.Create(act).Error)

	t.Run("Bundle without PDF and without BA", func(t *testing.T) {
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, user.ID, 6, 2026, false)
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(fname, ".xlsx"))
		assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ctype)
		assert.NotEmpty(t, data)
	})

	t.Run("Bundle with PDF and without BA", func(t *testing.T) {
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, user.ID, 6, 2026, true)
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(fname, "_with_PDF.zip"))
		assert.Equal(t, "application/zip", ctype)

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		require.Len(t, zr.File, 2)

		var foundXlsx, foundPDF bool
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, ".xlsx") {
				foundXlsx = true
			}
			if strings.HasSuffix(f.Name, ".pdf") {
				foundPDF = true
				rc, rerr := f.Open()
				require.NoError(t, rerr)
				buf, _ := io.ReadAll(rc)
				_ = rc.Close()
				assert.True(t, bytes.HasPrefix(buf, []byte("%PDF-")))
			}
		}
		assert.True(t, foundXlsx)
		assert.True(t, foundPDF)
	})

	// Seed witness and BA activity
	witness := &models.User{
		Username: "bundle_witness",
		Email:    "witness_bundle@example.com",
		Role:     models.RoleUser,
		Name:     "Bundle Witness",
		Company:  "sdd",
		IsActive: true,
	}
	require.NoError(t, tx.Create(witness).Error)

	baAct := &models.DailyActivity{
		UserID:           user.ID,
		Date:             time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
		StartTime:        "08:00",
		EndTime:          "17:00",
		Status:           "P",
		Activity:         "Critical hotfix deployment",
		IsActive:         true,
		IsBeritaAcara:    true,
		WitnessID:        &witness.ID,
		BeritaAcaraNotes: "Deployment went smoothly without downtime",
	}
	require.NoError(t, tx.Create(baAct).Error)

	t.Run("Bundle without PDF and with BA", func(t *testing.T) {
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, user.ID, 6, 2026, false)
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(fname, "_and_Berita_Acara.zip"))
		assert.Equal(t, "application/zip", ctype)

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		assert.Len(t, zr.File, 2)
	})

	t.Run("Bundle with PDF and with BA", func(t *testing.T) {
		data, fname, ctype, err := service.PackageTimesheetBundle(ctx, svc, user.ID, 6, 2026, true)
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(fname, "_with_PDF.zip"))
		assert.Equal(t, "application/zip", ctype)

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		require.Len(t, zr.File, 4)

		var foundXlsx, foundTimesheetPDF, foundDocx, foundBAPDF bool
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, ".xlsx") {
				foundXlsx = true
			}
			if strings.HasPrefix(f.Name, "Timesheet_") && strings.HasSuffix(f.Name, ".pdf") {
				foundTimesheetPDF = true
				rc, rerr := f.Open()
				require.NoError(t, rerr)
				buf, _ := io.ReadAll(rc)
				_ = rc.Close()
				assert.True(t, bytes.HasPrefix(buf, []byte("%PDF-")))
			}
			if strings.HasSuffix(f.Name, ".docx") {
				foundDocx = true
			}
			if strings.HasPrefix(f.Name, "Berita_Acara_") && strings.HasSuffix(f.Name, ".pdf") {
				foundBAPDF = true
				rc, rerr := f.Open()
				require.NoError(t, rerr)
				buf, _ := io.ReadAll(rc)
				_ = rc.Close()
				assert.True(t, bytes.HasPrefix(buf, []byte("%PDF-")))
			}
		}
		assert.True(t, foundXlsx)
		assert.True(t, foundTimesheetPDF)
		assert.True(t, foundDocx)
		assert.True(t, foundBAPDF)
	})
}
