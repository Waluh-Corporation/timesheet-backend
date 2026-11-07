package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/handlers"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

// testMemoryStorage implements storage.StorageService in-memory for E2E tests.
type testMemoryStorage struct {
	files map[string][]byte
}

func newTestMemoryStorage() *testMemoryStorage {
	return &testMemoryStorage{files: make(map[string][]byte)}
}

func (m *testMemoryStorage) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.files[key] = b
	return nil
}

func (m *testMemoryStorage) Download(ctx context.Context, key string) ([]byte, error) {
	b, ok := m.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return b, nil
}

func (m *testMemoryStorage) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}

func (m *testMemoryStorage) GetPresignedDownloadURLWithFilename(ctx context.Context, key string, filename string, expiry time.Duration) (string, error) {
	return "https://storage.test/" + key + "?att=" + filename, nil
}

func (m *testMemoryStorage) Delete(ctx context.Context, key string) error {
	delete(m.files, key)
	return nil
}

func (m *testMemoryStorage) FileExists(ctx context.Context, key string) (bool, error) {
	_, ok := m.files[key]
	return ok, nil
}

func generateTestBase64PNG(w, h int) string {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 12, G: 34, B: 56, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// setupE2ETestContext provisions an isolated test server, database transaction, and seeded users.
func setupE2ETestContext(t *testing.T) (*handlers.Server, *gin.Engine, *gorm.DB, models.User, models.User, models.User, models.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}
	tx := db.Begin()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	userRepo := repository.NewUserRepository(tx)
	activityRepo := repository.NewActivityRepository(tx)
	subRepo := repository.NewSubmissionRepository(tx)
	memStorage := newTestMemoryStorage()

	sigSvc := service.NewSignatureService(userRepo, activityRepo, subRepo, memStorage, nil, nil, "https://timesheet.local")

	srv := &handlers.Server{
		Cfg:            cfg,
		Auth:           authSvc,
		UserRepo:       userRepo,
		ActivityRepo:   activityRepo,
		SubmissionRepo: subRepo,
		Storage:        memStorage,
		SignatureSvc:   sigSvc,
	}

	// 1. Applicant (Employee)
	applicant := models.User{
		Username: "e2e_emp_" + uuid.New().String()[:8],
		Email:    "e2e_emp_" + uuid.New().String()[:8] + "@example.com",
		Name:     "Budi Santoso",
		Role:     models.RoleUser,
		BniID:    "12345",
		IsActive: true,
	}
	require.NoError(t, tx.Create(&applicant).Error)

	// 2. Designated Witness
	witness := models.User{
		Username: "e2e_wit_" + uuid.New().String()[:8],
		Email:    "e2e_wit_" + uuid.New().String()[:8] + "@example.com",
		Name:     "Dewi Sartika",
		Role:     models.RoleUser,
		BniID:    "67890",
		IsActive: true,
	}
	require.NoError(t, tx.Create(&witness).Error)

	// 3. Unauthorized Third-Party Employee
	thirdParty := models.User{
		Username: "e2e_third_" + uuid.New().String()[:8],
		Email:    "e2e_third_" + uuid.New().String()[:8] + "@example.com",
		Name:     "Joko Anwar",
		Role:     models.RoleUser,
		IsActive: true,
	}
	require.NoError(t, tx.Create(&thirdParty).Error)

	// 4. Admin User
	admin := models.User{
		Username: "e2e_admin_" + uuid.New().String()[:8],
		Email:    "e2e_admin_" + uuid.New().String()[:8] + "@example.com",
		Name:     "Super Admin",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	require.NoError(t, tx.Create(&admin).Error)

	// Seed signatures in memory storage
	sigKey1 := fmt.Sprintf("signatures/%d/sig.png", applicant.ID)
	sigKey2 := fmt.Sprintf("signatures/%d/sig.png", witness.ID)
	rawPNG, _ := base64.StdEncoding.DecodeString(generateTestBase64PNG(100, 40)[len("data:image/png;base64,"):])
	_ = memStorage.Upload(context.Background(), sigKey1, bytes.NewReader(rawPNG), "image/png")
	_ = memStorage.Upload(context.Background(), sigKey2, bytes.NewReader(rawPNG), "image/png")

	applicant.SignatureFileKey = sigKey1
	_ = tx.Save(&applicant)
	witness.SignatureFileKey = sigKey2
	_ = tx.Save(&witness)

	// Build Gin router matching main.go routes
	r := gin.New()
	api := r.Group("/api/v1")
	{
		// Mock auth middleware injecting claims
		authed := api.Group("")
		authed.Use(func(c *gin.Context) {
			tokenStr := c.GetHeader("Authorization")
			if tokenStr == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
				return
			}
			var uid uint
			var role models.Role
			switch tokenStr {
			case "Bearer applicant":
				uid = applicant.ID
				role = models.RoleUser
			case "Bearer witness":
				uid = witness.ID
				role = models.RoleUser
			case "Bearer thirdparty":
				uid = thirdParty.ID
				role = models.RoleUser
			case "Bearer admin":
				uid = admin.ID
				role = models.RoleAdmin
			default:
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid token"})
				return
			}
			c.Set("userID", uid)
			c.Set("userRole", role)
			c.Set("user_id", uid)
			c.Set("role", role)
			c.Next()
		})

		authed.GET("/berita-acara/:id/review", srv.ReviewBeritaAcara)
		authed.GET("/berita-acara/:id/preview/pdf", srv.PreviewBeritaAcaraPDF)
		authed.POST("/berita-acara/:id/witness-action", srv.WitnessAction)
	}

	return srv, r, tx, applicant, witness, thirdParty, admin
}

// createTestBeritaAcaraSubmission seeds a test submission into the database.
func createTestBeritaAcaraSubmission(t *testing.T, tx *gorm.DB, applicantID, witnessID uint, month, year int) string {
	t.Helper()
	subID := fmt.Sprintf("ba-e2e-%d-%02d-%s", year, month, uuid.New().String()[:6])

	metaJSON, _ := json.Marshal(map[string]interface{}{
		"employee_non_repudiation": true,
	})

	sub := models.BeritaAcaraSubmission{
		ID:                subID,
		UserID:            applicantID,
		WitnessID:         witnessID,
		Month:             month,
		Year:              year,
		Status:            models.SubmissionStatusPendingWitness,
		VerificationToken: uuid.New().String(),
		AuditMetadata:     datatypes.JSON(metaJSON),
	}
	require.NoError(t, tx.Create(&sub).Error)

	// Seed 2 attendance activities flagged as Berita Acara
	d1 := time.Date(year, time.Month(month), 5, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(year, time.Month(month), 12, 0, 0, 0, 0, time.UTC)
	act1 := models.DailyActivity{
		UserID:           applicantID,
		Date:             d1,
		StartTime:        "08:30",
		EndTime:          "17:30",
		Status:           "P",
		IsBeritaAcara:    true,
		BeritaAcaraNotes: "Forgot ID badge at front desk",
	}
	act2 := models.DailyActivity{
		UserID:           applicantID,
		Date:             d2,
		StartTime:        "09:00",
		EndTime:          "18:00",
		Status:           "P",
		IsBeritaAcara:    true,
		BeritaAcaraNotes: "Fingerprint scanner failure at East lobby",
	}
	require.NoError(t, tx.Create(&act1).Error)
	require.NoError(t, tx.Create(&act2).Error)

	return subID
}

// =============================================================================
// TIER 1 & TIER 2: FEATURE COVERAGE & BOUNDARY / IDOR ISOLATION TESTS
// =============================================================================

// TestE2E_BeritaAcaraWitnessReview_RoleIsolation verifies Feature 1 (Anti-IDOR)
// and Boundaries BND-01, BND-02, BND-03.
func TestE2E_BeritaAcaraWitnessReview_RoleIsolation(t *testing.T) {
	_, r, tx, applicant, witness, _, _ := setupE2ETestContext(t)
	defer tx.Rollback()

	subID := createTestBeritaAcaraSubmission(t, tx, applicant.ID, witness.ID, 9, 2026)
	url := fmt.Sprintf("/api/v1/berita-acara/%s/review", subID)

	// 1. Unauthenticated Request -> HTTP 401
	t.Run("Unauthenticated_Returns401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 2. Applicant Attempts Review -> HTTP 403 Forbidden (BND-01 Anti-IDOR)
	t.Run("Applicant_Forbidden403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer applicant")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized access to witness review")
	})

	// 3. Admin Attempts Review -> HTTP 403 Forbidden (BND-02 Strict Role Isolation)
	t.Run("Admin_Forbidden403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer admin")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized access to witness review")
	})

	// 4. Third-Party User Attempts Review -> HTTP 403 Forbidden
	t.Run("ThirdParty_Forbidden403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer thirdparty")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized access to witness review")
	})

	// 5. Designated Witness Permitted -> HTTP 200 OK (Feature 1 Happy Path)
	t.Run("DesignatedWitness_Permitted200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer witness")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestE2E_BeritaAcaraWitnessReview_FirstViewTracking_Idempotency verifies
// Feature 3 (First-View Tracking) and Boundary BND-04 (Idempotency).
func TestE2E_BeritaAcaraWitnessReview_FirstViewTracking_Idempotency(t *testing.T) {
	srv, r, tx, applicant, witness, _, _ := setupE2ETestContext(t)
	defer tx.Rollback()

	subID := createTestBeritaAcaraSubmission(t, tx, applicant.ID, witness.ID, 9, 2026)
	url := fmt.Sprintf("/api/v1/berita-acara/%s/review", subID)

	// Call 1: First view by witness from IP 192.0.2.1
	req1 := httptest.NewRequest(http.MethodGet, url, nil)
	req1.Header.Set("Authorization", "Bearer witness")
	req1.Header.Set("X-Forwarded-For", "192.0.2.1")
	req1.Header.Set("User-Agent", "E2E-Witness-Browser-v1")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Verify first-view metadata recorded in DB
	sub1, err := srv.SubmissionRepo.FindBeritaAcaraByID(context.Background(), subID)
	require.NoError(t, err)
	require.NotNil(t, sub1)

	var metaMap1 map[string]interface{}
	require.NoError(t, json.Unmarshal(sub1.AuditMetadata, &metaMap1))

	viewedAt1, ok1 := metaMap1["witness_viewed_at"].(string)
	assert.True(t, ok1, "witness_viewed_at must be present")
	assert.NotEmpty(t, viewedAt1)

	// Verify signing challenge was generated
	challenge1, chkOk1 := metaMap1["signing_challenge"].(string)
	assert.True(t, chkOk1, "signing_challenge must be present")
	assert.Len(t, challenge1, 43, "signing_challenge should be 32 bytes base64url (43 chars)")

	// Call 2: Second view by witness from a DIFFERENT IP and UA after time delay
	time.Sleep(10 * time.Millisecond)
	req2 := httptest.NewRequest(http.MethodGet, url, nil)
	req2.Header.Set("Authorization", "Bearer witness")
	req2.Header.Set("X-Forwarded-For", "198.51.100.99")
	req2.Header.Set("User-Agent", "Different-Agent-v2")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Verify idempotency: witness_viewed_at MUST NOT be overwritten
	sub2, err := srv.SubmissionRepo.FindBeritaAcaraByID(context.Background(), subID)
	require.NoError(t, err)
	var metaMap2 map[string]interface{}
	require.NoError(t, json.Unmarshal(sub2.AuditMetadata, &metaMap2))

	viewedAt2 := metaMap2["witness_viewed_at"].(string)
	assert.Equal(t, viewedAt1, viewedAt2, "witness_viewed_at must remain strictly immutable (idempotent)")
}

// TestE2E_BeritaAcaraReview_ResponseSchema verifies Feature 2 (DTO schema completeness).
func TestE2E_BeritaAcaraReview_ResponseSchema(t *testing.T) {
	_, r, tx, applicant, witness, _, _ := setupE2ETestContext(t)
	defer tx.Rollback()

	subID := createTestBeritaAcaraSubmission(t, tx, applicant.ID, witness.ID, 9, 2026)
	url := fmt.Sprintf("/api/v1/berita-acara/%s/review", subID)

	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer witness")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var envelope struct {
		Code    int                                `json:"code"`
		Message string                             `json:"message"`
		Data    response.BeritaAcaraReviewResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))

	data := envelope.Data
	assert.Equal(t, subID, data.SubmissionID)
	assert.Equal(t, models.SubmissionStatusPendingWitness, data.Status)
	assert.Equal(t, 9, data.Month)
	assert.Equal(t, 2026, data.Year)

	// Applicant data
	assert.Equal(t, applicant.ID, data.Applicant.UserID)
	assert.Equal(t, applicant.Name, data.Applicant.Name)
	assert.Equal(t, "12345", data.Applicant.NPP)
	assert.True(t, data.Applicant.EmployeeNonRepudiation)
	assert.NotEmpty(t, data.Applicant.SignatureURL)

	// Witness data
	assert.Equal(t, witness.ID, data.Witness.UserID)
	assert.Equal(t, witness.Name, data.Witness.Name)
	assert.True(t, data.Witness.HasSignature)

	// Preview PDF and Signing Challenge
	expectedPDFURL := fmt.Sprintf("/api/v1/berita-acara/%s/preview/pdf", subID)
	assert.Equal(t, expectedPDFURL, data.PreviewPDFURL)
	assert.NotEmpty(t, data.SigningChallenge)

	// Incidents
	assert.Len(t, data.Incidents, 2, "Expected 2 attendance incident items")
	assert.Equal(t, "08:30", data.Incidents[0].StartTime)
	assert.Equal(t, "Forgot ID badge at front desk", data.Incidents[0].Notes)
}

// TestE2E_InstantPDFPreviewStreaming verifies Feature 14 & 13 (Pure-Go PDF preview stream).
func TestE2E_InstantPDFPreviewStreaming(t *testing.T) {
	_, r, tx, applicant, witness, _, _ := setupE2ETestContext(t)
	defer tx.Rollback()

	subID := createTestBeritaAcaraSubmission(t, tx, applicant.ID, witness.ID, 9, 2026)
	url := fmt.Sprintf("/api/v1/berita-acara/%s/preview/pdf", subID)

	// 1. Third-party is forbidden from viewing preview -> HTTP 403
	t.Run("ThirdPartyForbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer thirdparty")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	// 2. Designated Witness calls preview -> HTTP 200, application/pdf, begins with %PDF-
	t.Run("WitnessStreamsPDF", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer witness")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "inline; filename=\"Berita_Acara_Preview.pdf\"")

		body := w.Body.Bytes()
		require.Greater(t, len(body), 500, "PDF stream should be at least 500 bytes")
		assert.Equal(t, "%PDF-", string(body[:5]), "Must begin with standard %PDF- magic bytes")
	})

	// 3. Applicant calls preview -> HTTP 200, application/pdf
	t.Run("ApplicantStreamsPDF", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer applicant")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	})
}

// =============================================================================
// TIER 3: CROSS-FEATURE INTEGRATION WORKFLOW
// =============================================================================

// TestE2E_ReviewAndGracefulFallbackApprovalWorkflow verifies the complete
// Tier 3 & Tier 4 interaction:
// Employee submits -> Witness reviews -> First view tracked -> Instant PDF inspected ->
// Witness approves via graceful fallback (no passkey) -> Public audit trail verifies milestones.
func TestE2E_ReviewAndGracefulFallbackApprovalWorkflow(t *testing.T) {
	srv, r, tx, applicant, witness, _, _ := setupE2ETestContext(t)
	defer tx.Rollback()

	// Step 1: Create submission
	subID := createTestBeritaAcaraSubmission(t, tx, applicant.ID, witness.ID, 9, 2026)

	// Step 2: Witness views review endpoint
	reviewURL := fmt.Sprintf("/api/v1/berita-acara/%s/review", subID)
	reqReview := httptest.NewRequest(http.MethodGet, reviewURL, nil)
	reqReview.Header.Set("Authorization", "Bearer witness")
	reqReview.Header.Set("X-Forwarded-For", "203.0.113.50")
	wReview := httptest.NewRecorder()
	r.ServeHTTP(wReview, reqReview)
	require.Equal(t, http.StatusOK, wReview.Code)

	// Step 3: Witness inspects PDF preview
	previewURL := fmt.Sprintf("/api/v1/berita-acara/%s/preview/pdf", subID)
	reqPreview := httptest.NewRequest(http.MethodGet, previewURL, nil)
	reqPreview.Header.Set("Authorization", "Bearer witness")
	wPreview := httptest.NewRecorder()
	r.ServeHTTP(wPreview, reqPreview)
	require.Equal(t, http.StatusOK, wPreview.Code)
	assert.Equal(t, "%PDF-", string(wPreview.Body.Bytes()[:5]))

	// Step 4: Witness approves via graceful fallback (standard approval without passkey assertion)
	actionURL := fmt.Sprintf("/api/v1/berita-acara/%s/witness-action", subID)
	actionBody := request.WitnessActionRequest{
		Action: "approve",
		Notes:  "Reviewed incident details; approved without hardware passkey",
	}
	bodyBytes, _ := json.Marshal(actionBody)

	reqAction := httptest.NewRequest(http.MethodPost, actionURL, bytes.NewReader(bodyBytes))
	reqAction.Header.Set("Authorization", "Bearer witness")
	reqAction.Header.Set("Content-Type", "application/json")
	wAction := httptest.NewRecorder()
	r.ServeHTTP(wAction, reqAction)
	require.Equal(t, http.StatusOK, wAction.Code)

	// Step 5: Verify submission status and audit trail
	subAfter, err := srv.SubmissionRepo.FindBeritaAcaraByID(context.Background(), subID)
	require.NoError(t, err)
	assert.Equal(t, models.SubmissionStatusSigned, subAfter.Status)

	// Step 6: Verify public audit trail contains Document Viewed by Witness milestone
	verifyResp, err := srv.SignatureSvc.VerifyDocument(context.Background(), subAfter.VerificationToken)
	require.NoError(t, err)
	require.NotNil(t, verifyResp)

	foundViewedMilestone := false
	for _, m := range verifyResp.AuditTrail {
		if m.Title == "Document Viewed by Witness" {
			foundViewedMilestone = true
			assert.Equal(t, "completed", m.Status)
			assert.Equal(t, "Witness", m.Role)
			break
		}
	}
	assert.True(t, foundViewedMilestone, "Audit trail must include 'Document Viewed by Witness' milestone")
}
