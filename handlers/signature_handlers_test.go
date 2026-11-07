package handlers

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
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

func generateTestBase64PNG(w, h int) string {
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

func setupSignatureHandlerTest(t *testing.T) (*Server, *gorm.DB, models.User, models.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	userRepo := repository.NewUserRepository(tx)
	activityRepo := repository.NewActivityRepository(tx)
	subRepo := repository.NewSubmissionRepository(tx)
	memStorage := newTestMemoryStorage()

	sigSvc := service.NewSignatureService(userRepo, activityRepo, subRepo, memStorage, nil, nil, "https://timesheet.local")

	srv := &Server{
		Cfg:            cfg,
		Auth:           authSvc,
		UserRepo:       userRepo,
		ActivityRepo:   activityRepo,
		SubmissionRepo: subRepo,
		SignatureSvc:   sigSvc,
	}

	u1 := models.User{
		Username: "sig_handler_user_" + uuid.New().String()[:8],
		Email:    "sig_user_" + uuid.New().String()[:8] + "@example.com",
		Name:     "Signature User 1",
		Role:     models.RoleUser,
		IsActive: true,
	}
	_ = tx.Create(&u1)

	u2 := models.User{
		Username: "sig_handler_witness_" + uuid.New().String()[:8],
		Email:    "sig_witness_" + uuid.New().String()[:8] + "@example.com",
		Name:     "Signature Witness 2",
		Role:     models.RoleUser,
		IsActive: true,
	}
	_ = tx.Create(&u2)

	return srv, tx, u1, u2
}

func TestSignatureHandlers_ProfileFlow(t *testing.T) {
	srv, tx, u1, _ := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	// 1. Get initial signature -> false
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/signature", nil)
	c.Set("userID", u1.ID)
	c.Set("role", models.RoleUser)

	srv.GetSignature(c)
	assert.Equal(t, http.StatusOK, w.Code)

	var getResp struct {
		Code int                            `json:"code"`
		Data response.UserSignatureResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &getResp)
	assert.False(t, getResp.Data.HasSignature)

	// 2. Upload valid signature
	b64 := generateTestBase64PNG(80, 40)
	bodyStr := fmt.Sprintf(`{"signature_base64":"%s"}`, b64)

	wUp := httptest.NewRecorder()
	cUp, _ := gin.CreateTestContext(wUp)
	cUp.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/signature", strings.NewReader(bodyStr))
	cUp.Request.Header.Set("Content-Type", "application/json")
	cUp.Set("userID", u1.ID)
	cUp.Set("role", models.RoleUser)

	srv.UploadSignature(cUp)
	assert.Equal(t, http.StatusOK, wUp.Code)

	// 3. Delete signature
	wDel := httptest.NewRecorder()
	cDel, _ := gin.CreateTestContext(wDel)
	cDel.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/user/signature", nil)
	cDel.Set("userID", u1.ID)
	cDel.Set("role", models.RoleUser)

	srv.DeleteSignature(cDel)
	assert.Equal(t, http.StatusOK, wDel.Code)
}

func TestSignatureHandlers_TimesheetSignAndRevoke(t *testing.T) {
	srv, tx, u1, _ := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	// First upload signature
	b64 := generateTestBase64PNG(80, 40)
	wUp := httptest.NewRecorder()
	cUp, _ := gin.CreateTestContext(wUp)
	cUp.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/signature", strings.NewReader(fmt.Sprintf(`{"signature_base64":"%s"}`, b64)))
	cUp.Request.Header.Set("Content-Type", "application/json")
	cUp.Set("userID", u1.ID)
	cUp.Set("role", models.RoleUser)
	srv.UploadSignature(cUp)

	// Seed activity
	act := models.DailyActivity{
		UserID:    u1.ID,
		Date:      time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
		StartTime: "08:00",
		EndTime:   "17:00",
		Status:    "P",
		Activity:  "Core backend development",
		IsActive:  true,
	}
	if err := tx.Create(&act).Error; err != nil {
		t.Fatalf("failed to seed activity: %v", err)
	}

	// 1. Sign Timesheet
	wSign := httptest.NewRecorder()
	cSign, _ := gin.CreateTestContext(wSign)
	cSign.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheets/sign", strings.NewReader(`{"month":10,"year":2026}`))
	cSign.Request.Header.Set("Content-Type", "application/json")
	cSign.Set("userID", u1.ID)
	cSign.Set("role", models.RoleUser)

	srv.SignTimesheet(cSign)
	assert.Equal(t, http.StatusOK, wSign.Code)

	var signResp struct {
		Data response.TimesheetSubmissionResponse `json:"data"`
	}
	_ = json.Unmarshal(wSign.Body.Bytes(), &signResp)
	assert.Equal(t, models.SubmissionStatusSigned, signResp.Data.Status)
	assert.NotEmpty(t, signResp.Data.VerificationToken)

	// 2. Status check
	wStat := httptest.NewRecorder()
	cStat, _ := gin.CreateTestContext(wStat)
	cStat.Request = httptest.NewRequest(http.MethodGet, "/api/v1/timesheets/status?month=10&year=2026", nil)
	cStat.Set("userID", u1.ID)
	cStat.Set("role", models.RoleUser)

	srv.GetTimesheetSubmissionStatus(cStat)
	assert.Equal(t, http.StatusOK, wStat.Code)

	// 3. Revoke Timesheet
	wRev := httptest.NewRecorder()
	cRev, _ := gin.CreateTestContext(wRev)
	cRev.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheets/revoke", strings.NewReader(`{"month":10,"year":2026}`))
	cRev.Request.Header.Set("Content-Type", "application/json")
	cRev.Set("userID", u1.ID)
	cRev.Set("role", models.RoleUser)

	srv.RevokeTimesheet(cRev)
	assert.Equal(t, http.StatusOK, wRev.Code)
}

func TestSignatureHandlers_BeritaAcaraWorkflow(t *testing.T) {
	srv, tx, u1, u2 := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	// Upload signatures for both u1 and u2
	b64 := generateTestBase64PNG(80, 40)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u1.ID, b64)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u2.ID, b64)

	// Seed BA activity
	act := models.DailyActivity{
		UserID:        u1.ID,
		Date:          time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
		StartTime:     "08:00",
		EndTime:       "17:00",
		Status:        "P",
		Activity:      "Datacenter maintenance",
		IsBeritaAcara: true,
		IsActive:      true,
	}
	if err := tx.Create(&act).Error; err != nil {
		t.Fatalf("failed to seed BA activity: %v", err)
	}

	// 1. Submit BA
	submitBody := fmt.Sprintf(`{"month":10,"year":2026,"witness_id":%d}`, u2.ID)
	wSub := httptest.NewRecorder()
	cSub, _ := gin.CreateTestContext(wSub)
	cSub.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/submit", strings.NewReader(submitBody))
	cSub.Request.Header.Set("Content-Type", "application/json")
	cSub.Set("userID", u1.ID)
	cSub.Set("role", models.RoleUser)

	srv.SubmitBeritaAcara(cSub)
	assert.Equal(t, http.StatusOK, wSub.Code)

	var subResp struct {
		Data response.BeritaAcaraSubmissionResponse `json:"data"`
	}
	_ = json.Unmarshal(wSub.Body.Bytes(), &subResp)
	assert.Equal(t, models.SubmissionStatusPendingWitness, subResp.Data.Status)
	submissionID := subResp.Data.ID

	// 2. Witness lists requests
	wReq := httptest.NewRecorder()
	cReq, _ := gin.CreateTestContext(wReq)
	cReq.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/witness-requests", nil)
	cReq.Set("userID", u2.ID)
	cReq.Set("role", models.RoleUser)

	srv.ListPendingWitnessRequests(cReq)
	assert.Equal(t, http.StatusOK, wReq.Code)

	// 3. Witness approves
	wAct := httptest.NewRecorder()
	cAct, _ := gin.CreateTestContext(wAct)
	cAct.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/"+submissionID+"/witness-action", strings.NewReader(`{"action":"approve"}`))
	cAct.Request.Header.Set("Content-Type", "application/json")
	cAct.Params = gin.Params{{Key: "id", Value: submissionID}}
	cAct.Set("userID", u2.ID)
	cAct.Set("role", models.RoleUser)

	srv.WitnessAction(cAct)
	assert.Equal(t, http.StatusOK, wAct.Code)

	// 4. Verify Document via JSON API
	wVer := httptest.NewRecorder()
	cVer, _ := gin.CreateTestContext(wVer)
	cVer.Request = httptest.NewRequest(http.MethodGet, "/api/v1/verify/doc/"+subResp.Data.VerificationToken, nil)
	cVer.Params = gin.Params{{Key: "token", Value: subResp.Data.VerificationToken}}

	srv.VerifyDocument(cVer)
	assert.Equal(t, http.StatusOK, wVer.Code)

	var verResp struct {
		Code   int                                         `json:"code"`
		Status string                                      `json:"status"`
		Data   response.PublicDocumentVerificationResponse `json:"data"`
	}
	err := json.Unmarshal(wVer.Body.Bytes(), &verResp)
	assert.NoError(t, err)
	assert.True(t, verResp.Data.IsValid)
	assert.Equal(t, "valid", verResp.Data.Status)
	assert.Equal(t, "berita_acara", verResp.Data.DocumentType)
	assert.Equal(t, u1.Name, verResp.Data.EmployeeName)
	assert.Equal(t, u2.Name, verResp.Data.WitnessName)
	assert.NotEmpty(t, verResp.Data.DocumentHash)

	// 4b. Verify Document via Tabler HTML (Browser mode via Accept: text/html)
	srv.Cfg.AppName = "Corporate Timesheet Central"
	wVerHTML := httptest.NewRecorder()
	cVerHTML, _ := gin.CreateTestContext(wVerHTML)
	cVerHTML.Request = httptest.NewRequest(http.MethodGet, "/verify/doc/"+subResp.Data.VerificationToken, nil)
	cVerHTML.Request.Header.Set("Accept", "text/html,application/xhtml+xml")
	cVerHTML.Params = gin.Params{{Key: "token", Value: subResp.Data.VerificationToken}}
	srv.VerifyDocument(cVerHTML)
	assert.Equal(t, http.StatusOK, wVerHTML.Code)
	assert.Contains(t, wVerHTML.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, wVerHTML.Body.String(), "AUTHENTIC &amp; VERIFIED DOCUMENT")
	assert.Contains(t, wVerHTML.Body.String(), "Corporate Timesheet Central")
	assert.Contains(t, wVerHTML.Body.String(), u1.Name)
	assert.Contains(t, wVerHTML.Body.String(), u2.Name)

	// 4c. Verify Document via Tabler HTML (?format=html query param)
	wVerQuery := httptest.NewRecorder()
	cVerQuery, _ := gin.CreateTestContext(wVerQuery)
	cVerQuery.Request = httptest.NewRequest(http.MethodGet, "/verify/doc/"+subResp.Data.VerificationToken+"?format=html", nil)
	cVerQuery.Params = gin.Params{{Key: "token", Value: subResp.Data.VerificationToken}}
	srv.VerifyDocument(cVerQuery)
	assert.Equal(t, http.StatusOK, wVerQuery.Code)
	assert.Contains(t, wVerQuery.Body.String(), "AUTHENTIC &amp; VERIFIED DOCUMENT")
	assert.Contains(t, wVerQuery.Body.String(), "Document Information")

	// 5. Get Berita Acara Status
	wStat := httptest.NewRecorder()
	cStat, _ := gin.CreateTestContext(wStat)
	cStat.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/status?month=10&year=2026", nil)
	cStat.Set("userID", u1.ID)
	cStat.Set("role", models.RoleUser)

	srv.GetBeritaAcaraSubmissionStatus(cStat)
	assert.Equal(t, http.StatusOK, wStat.Code)

	// 6. Revoke Berita Acara
	wRev := httptest.NewRecorder()
	cRev, _ := gin.CreateTestContext(wRev)
	cRev.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/"+submissionID+"/revoke", nil)
	cRev.Params = gin.Params{{Key: "id", Value: submissionID}}
	cRev.Set("userID", u1.ID)
	cRev.Set("role", models.RoleUser)

	srv.RevokeBeritaAcara(cRev)
	assert.Equal(t, http.StatusOK, wRev.Code)
}

func TestSignatureHandlers_ReviewBeritaAcara_RoleIsolation(t *testing.T) {
	srv, tx, u1, u2 := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	// Create admin user
	uAdmin := models.User{
		Username: "sig_handler_admin_" + uuid.New().String()[:8],
		Email:    "sig_admin_" + uuid.New().String()[:8] + "@example.com",
		Name:     "Signature Admin",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	if err := tx.Create(&uAdmin).Error; err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	// Upload signatures for both u1 and u2
	b64 := generateTestBase64PNG(80, 40)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u1.ID, b64)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u2.ID, b64)

	// Seed BA activity
	act := models.DailyActivity{
		UserID:           u1.ID,
		Date:             time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
		StartTime:        "08:30",
		EndTime:          "17:30",
		Status:           "P",
		Activity:         "Incident work",
		BeritaAcaraNotes: "Forgot badge",
		IsBeritaAcara:    true,
		IsActive:         true,
	}
	if err := tx.Create(&act).Error; err != nil {
		t.Fatalf("failed to seed BA activity: %v", err)
	}

	// Submit BA with u1 as applicant and u2 as witness
	submitBody := fmt.Sprintf(`{"month":9,"year":2026,"witness_id":%d}`, u2.ID)
	wSub := httptest.NewRecorder()
	cSub, _ := gin.CreateTestContext(wSub)
	cSub.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/submit", strings.NewReader(submitBody))
	cSub.Request.Header.Set("Content-Type", "application/json")
	cSub.Set("userID", u1.ID)
	cSub.Set("role", models.RoleUser)

	srv.SubmitBeritaAcara(cSub)
	assert.Equal(t, http.StatusOK, wSub.Code)

	var subResp struct {
		Data response.BeritaAcaraSubmissionResponse `json:"data"`
	}
	_ = json.Unmarshal(wSub.Body.Bytes(), &subResp)
	submissionID := subResp.Data.ID
	assert.NotEmpty(t, submissionID)

	// 1. Applicant (u1) calls review -> MUST be rejected with HTTP 403 Forbidden
	wApp := httptest.NewRecorder()
	cApp, _ := gin.CreateTestContext(wApp)
	cApp.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/"+submissionID+"/review", nil)
	cApp.Params = gin.Params{{Key: "id", Value: submissionID}}
	cApp.Set("userID", u1.ID)
	cApp.Set("role", models.RoleUser)

	srv.ReviewBeritaAcara(cApp)
	assert.Equal(t, http.StatusForbidden, wApp.Code)
	assert.Contains(t, wApp.Body.String(), "unauthorized access to witness review")

	// 2. Admin (uAdmin) calls review -> MUST be rejected with HTTP 403 Forbidden
	wAdm := httptest.NewRecorder()
	cAdm, _ := gin.CreateTestContext(wAdm)
	cAdm.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/"+submissionID+"/review", nil)
	cAdm.Params = gin.Params{{Key: "id", Value: submissionID}}
	cAdm.Set("userID", uAdmin.ID)
	cAdm.Set("role", models.RoleAdmin)

	srv.ReviewBeritaAcara(cAdm)
	assert.Equal(t, http.StatusForbidden, wAdm.Code)
	assert.Contains(t, wAdm.Body.String(), "unauthorized access to witness review")

	// 3. Designated Witness (u2) calls review -> MUST succeed with HTTP 200 OK
	wWit := httptest.NewRecorder()
	cWit, _ := gin.CreateTestContext(wWit)
	cWit.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/"+submissionID+"/review", nil)
	cWit.Params = gin.Params{{Key: "id", Value: submissionID}}
	cWit.Set("userID", u2.ID)
	cWit.Set("role", models.RoleUser)

	srv.ReviewBeritaAcara(cWit)
	assert.Equal(t, http.StatusOK, wWit.Code)

	var reviewResp struct {
		Code int                                `json:"code"`
		Data response.BeritaAcaraReviewResponse `json:"data"`
	}
	err := json.Unmarshal(wWit.Body.Bytes(), &reviewResp)
	assert.NoError(t, err)
	assert.Equal(t, submissionID, reviewResp.Data.SubmissionID)
	assert.Equal(t, 9, reviewResp.Data.Month)
	assert.Equal(t, 2026, reviewResp.Data.Year)
	assert.Equal(t, models.SubmissionStatusPendingWitness, reviewResp.Data.Status)
	assert.Equal(t, u1.Name, reviewResp.Data.Applicant.Name)
	assert.Equal(t, u2.Name, reviewResp.Data.Witness.Name)
	assert.NotEmpty(t, reviewResp.Data.SigningChallenge)
	assert.Equal(t, "/api/v1/berita-acara/"+submissionID+"/preview/pdf", reviewResp.Data.PreviewPDFURL)
	assert.Len(t, reviewResp.Data.Incidents, 1)
	assert.Equal(t, "Forgot badge", reviewResp.Data.Incidents[0].Notes)

	// 4. Unauthenticated or non-matching caller (userID = 0) -> MUST be rejected with HTTP 403 Forbidden
	wAnon := httptest.NewRecorder()
	cAnon, _ := gin.CreateTestContext(wAnon)
	cAnon.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/"+submissionID+"/review", nil)
	cAnon.Params = gin.Params{{Key: "id", Value: submissionID}}
	cAnon.Set("userID", uint(0))
	cAnon.Set("role", models.RoleUser)

	srv.ReviewBeritaAcara(cAnon)
	assert.Equal(t, http.StatusForbidden, wAnon.Code)
	assert.Contains(t, wAnon.Body.String(), "unauthorized access to witness review")
}

func TestSignatureHandlers_ReviewBeritaAcara_IdempotentViewTracking(t *testing.T) {
	srv, tx, u1, u2 := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	// Upload signatures for both u1 and u2
	b64 := generateTestBase64PNG(80, 40)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u1.ID, b64)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u2.ID, b64)

	// Seed BA activity
	act := models.DailyActivity{
		UserID:           u1.ID,
		Date:             time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
		StartTime:        "08:30",
		EndTime:          "17:30",
		Status:           "P",
		Activity:         "Server outage",
		BeritaAcaraNotes: "Network crash",
		IsBeritaAcara:    true,
		IsActive:         true,
	}
	if err := tx.Create(&act).Error; err != nil {
		t.Fatalf("failed to seed BA activity: %v", err)
	}

	submitBody := fmt.Sprintf(`{"month":9,"year":2026,"witness_id":%d}`, u2.ID)
	wSub := httptest.NewRecorder()
	cSub, _ := gin.CreateTestContext(wSub)
	cSub.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/submit", strings.NewReader(submitBody))
	cSub.Request.Header.Set("Content-Type", "application/json")
	cSub.Set("userID", u1.ID)
	cSub.Set("role", models.RoleUser)

	srv.SubmitBeritaAcara(cSub)
	assert.Equal(t, http.StatusOK, wSub.Code)

	var subResp struct {
		Data response.BeritaAcaraSubmissionResponse `json:"data"`
	}
	_ = json.Unmarshal(wSub.Body.Bytes(), &subResp)
	submissionID := subResp.Data.ID

	// Call 1: Witness views for first time with IP 192.168.1.50
	wView1 := httptest.NewRecorder()
	cView1, _ := gin.CreateTestContext(wView1)
	cView1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/"+submissionID+"/review", nil)
	cView1.Request.Header.Set("X-Forwarded-For", "192.168.1.50")
	cView1.Request.Header.Set("User-Agent", "WitnessAgent/1.0")
	cView1.Params = gin.Params{{Key: "id", Value: submissionID}}
	cView1.Set("userID", u2.ID)
	cView1.Set("role", models.RoleUser)

	srv.ReviewBeritaAcara(cView1)
	assert.Equal(t, http.StatusOK, wView1.Code)

	// Check AuditMetadata in DB
	var subInDB models.BeritaAcaraSubmission
	err := tx.Where("id = ?", submissionID).First(&subInDB).Error
	assert.NoError(t, err)

	var metaMap1 map[string]interface{}
	err = json.Unmarshal(subInDB.AuditMetadata, &metaMap1)
	assert.NoError(t, err)
	viewedAt1, ok := metaMap1["witness_viewed_at"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, viewedAt1)
	assert.Equal(t, "192.168.1.50", metaMap1["witness_view_ip"])
	assert.Equal(t, "WitnessAgent/1.0", metaMap1["witness_view_user_agent"])

	time.Sleep(20 * time.Millisecond)

	// Call 2: Witness views a second time with different IP (10.0.0.99) and different UA
	wView2 := httptest.NewRecorder()
	cView2, _ := gin.CreateTestContext(wView2)
	cView2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/"+submissionID+"/review", nil)
	cView2.Request.Header.Set("X-Forwarded-For", "10.0.0.99")
	cView2.Request.Header.Set("User-Agent", "DifferentAgent/2.0")
	cView2.Params = gin.Params{{Key: "id", Value: submissionID}}
	cView2.Set("userID", u2.ID)
	cView2.Set("role", models.RoleUser)

	srv.ReviewBeritaAcara(cView2)
	assert.Equal(t, http.StatusOK, wView2.Code)

	// Check AuditMetadata in DB after second call - MUST be strictly identical
	var subInDB2 models.BeritaAcaraSubmission
	err = tx.Where("id = ?", submissionID).First(&subInDB2).Error
	assert.NoError(t, err)

	var metaMap2 map[string]interface{}
	err = json.Unmarshal(subInDB2.AuditMetadata, &metaMap2)
	assert.NoError(t, err)
	assert.Equal(t, viewedAt1, metaMap2["witness_viewed_at"])
	assert.Equal(t, "192.168.1.50", metaMap2["witness_view_ip"])
	assert.Equal(t, "WitnessAgent/1.0", metaMap2["witness_view_user_agent"])
}

func TestSignatureHandlers_ReviewBeritaAcara_ValidationAndErrors(t *testing.T) {
	srv, tx, _, u2 := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	// Missing ID -> 400 Bad Request
	wEmpty := httptest.NewRecorder()
	cEmpty, _ := gin.CreateTestContext(wEmpty)
	cEmpty.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara//review", nil)
	cEmpty.Params = gin.Params{{Key: "id", Value: ""}}
	cEmpty.Set("userID", u2.ID)
	cEmpty.Set("role", models.RoleUser)

	srv.ReviewBeritaAcara(cEmpty)
	assert.Equal(t, http.StatusBadRequest, wEmpty.Code)

	// Non-existent ID -> 404 Not Found
	wNotFound := httptest.NewRecorder()
	cNotFound, _ := gin.CreateTestContext(wNotFound)
	cNotFound.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/non-existent-ba-id/review", nil)
	cNotFound.Params = gin.Params{{Key: "id", Value: "non-existent-ba-id"}}
	cNotFound.Set("userID", u2.ID)
	cNotFound.Set("role", models.RoleUser)

	srv.ReviewBeritaAcara(cNotFound)
	assert.Equal(t, http.StatusNotFound, wNotFound.Code)

	// Mock returning ErrForbidden -> 403 Forbidden
	forbidSrv := &Server{
		SignatureSvc: &mockErrSigService{errToReturn: domain.ErrForbidden},
	}
	wForbid := httptest.NewRecorder()
	cForbid, _ := gin.CreateTestContext(wForbid)
	cForbid.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/ba-1/review", nil)
	cForbid.Params = gin.Params{{Key: "id", Value: "ba-1"}}
	cForbid.Set("userID", u2.ID)
	forbidSrv.ReviewBeritaAcara(cForbid)
	assert.Equal(t, http.StatusForbidden, wForbid.Code)
	assert.Contains(t, wForbid.Body.String(), "unauthorized access to witness review")
}

func TestSignatureHandlers_NilServiceHandling(t *testing.T) {
	srv := &Server{SignatureSvc: nil}

	runNilTest := func(method, path string, handler func(*gin.Context)) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, path, nil)
		handler(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	}

	runNilTest(http.MethodPost, "/api/v1/user/signature", srv.UploadSignature)
	runNilTest(http.MethodGet, "/api/v1/user/signature", srv.GetSignature)
	runNilTest(http.MethodDelete, "/api/v1/user/signature", srv.DeleteSignature)
	runNilTest(http.MethodPost, "/api/v1/timesheets/sign", srv.SignTimesheet)
	runNilTest(http.MethodPost, "/api/v1/timesheets/revoke", srv.RevokeTimesheet)
	runNilTest(http.MethodGet, "/api/v1/timesheets/status", srv.GetTimesheetSubmissionStatus)
	runNilTest(http.MethodPost, "/api/v1/berita-acara/submit", srv.SubmitBeritaAcara)
	runNilTest(http.MethodGet, "/api/v1/berita-acara/1/review", srv.ReviewBeritaAcara)
	runNilTest(http.MethodPost, "/api/v1/berita-acara/1/witness-action", srv.WitnessAction)
	runNilTest(http.MethodPost, "/api/v1/berita-acara/1/revoke", srv.RevokeBeritaAcara)
	runNilTest(http.MethodGet, "/api/v1/berita-acara/status", srv.GetBeritaAcaraSubmissionStatus)
	runNilTest(http.MethodGet, "/api/v1/berita-acara/witness-requests", srv.ListPendingWitnessRequests)
	runNilTest(http.MethodGet, "/api/v1/verify/doc/token", srv.VerifyDocument)
}

func TestSignatureHandlers_ValidationAndErrorBranches(t *testing.T) {
	srv, tx, u1, u2 := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	t.Run("UploadSignature bad requests and user errors", func(t *testing.T) {
		// Invalid JSON
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/signature", strings.NewReader(`{invalid}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("userID", u1.ID)
		srv.UploadSignature(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Empty signature base64
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/signature", strings.NewReader(`{"signature_base64":""}`))
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Set("userID", u1.ID)
		srv.UploadSignature(c2)
		assert.Equal(t, http.StatusBadRequest, w2.Code)
	})

	t.Run("Timesheet handlers bad requests", func(t *testing.T) {
		// SignTimesheet invalid JSON
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheets/sign", strings.NewReader(`{invalid}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("userID", u1.ID)
		srv.SignTimesheet(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// RevokeTimesheet invalid JSON
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheets/revoke", strings.NewReader(`{invalid}`))
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Set("userID", u1.ID)
		srv.RevokeTimesheet(c2)
		assert.Equal(t, http.StatusBadRequest, w2.Code)

		// Status invalid query
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Request = httptest.NewRequest(http.MethodGet, "/api/v1/timesheets/status?month=0&year=2026", nil)
		c3.Set("userID", u1.ID)
		srv.GetTimesheetSubmissionStatus(c3)
		assert.Equal(t, http.StatusBadRequest, w3.Code)

		// SignTimesheet user error (no signature uploaded)
		uNoSig := models.User{
			Username: "no_sig_user",
			Email:    "nosig@example.com",
			Name:     "No Sig User",
			Role:     models.RoleUser,
			IsActive: true,
		}
		_ = tx.Create(&uNoSig)

		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheets/sign", strings.NewReader(`{"month":10,"year":2026}`))
		c4.Request.Header.Set("Content-Type", "application/json")
		c4.Set("userID", uNoSig.ID)
		srv.SignTimesheet(c4)
		assert.Equal(t, http.StatusBadRequest, w4.Code)

		// RevokeTimesheet user error (timesheet not signed)
		w5 := httptest.NewRecorder()
		c5, _ := gin.CreateTestContext(w5)
		c5.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheets/revoke", strings.NewReader(`{"month":10,"year":2026}`))
		c5.Request.Header.Set("Content-Type", "application/json")
		c5.Set("userID", uNoSig.ID)
		srv.RevokeTimesheet(c5)
		assert.Equal(t, http.StatusBadRequest, w5.Code)
	})

	t.Run("BeritaAcara handlers bad requests and rejection", func(t *testing.T) {
		// Submit invalid JSON
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/submit", strings.NewReader(`{invalid}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("userID", u1.ID)
		srv.SubmitBeritaAcara(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Witness action missing id
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara//witness-action", strings.NewReader(`{"action":"reject","notes":"revisi"}`))
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Params = gin.Params{{Key: "id", Value: ""}}
		c2.Set("userID", u2.ID)
		srv.WitnessAction(c2)
		assert.Equal(t, http.StatusBadRequest, w2.Code)

		// Witness action invalid JSON
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/123/witness-action", strings.NewReader(`{invalid}`))
		c3.Request.Header.Set("Content-Type", "application/json")
		c3.Params = gin.Params{{Key: "id", Value: "123"}}
		c3.Set("userID", u2.ID)
		srv.WitnessAction(c3)
		assert.Equal(t, http.StatusBadRequest, w3.Code)

		// Revoke missing id
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara//revoke", nil)
		c4.Params = gin.Params{{Key: "id", Value: ""}}
		c4.Set("userID", u1.ID)
		srv.RevokeBeritaAcara(c4)
		assert.Equal(t, http.StatusBadRequest, w4.Code)

		// Status invalid month/year
		w5 := httptest.NewRecorder()
		c5, _ := gin.CreateTestContext(w5)
		c5.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/status?month=15&year=2026", nil)
		c5.Set("userID", u1.ID)
		srv.GetBeritaAcaraSubmissionStatus(c5)
		assert.Equal(t, http.StatusBadRequest, w5.Code)
	})

	t.Run("VerifyDocument not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/verify/doc/non-existent-token", nil)
		c.Params = gin.Params{{Key: "token", Value: "non-existent-token"}}
		srv.VerifyDocument(c)
		assert.Equal(t, http.StatusNotFound, w.Code)

		// HTML mode not found
		wHTML := httptest.NewRecorder()
		cHTML, _ := gin.CreateTestContext(wHTML)
		cHTML.Request = httptest.NewRequest(http.MethodGet, "/verify/doc/non-existent-token", nil)
		cHTML.Request.Header.Set("Accept", "text/html")
		cHTML.Params = gin.Params{{Key: "token", Value: "non-existent-token"}}
		srv.VerifyDocument(cHTML)
		assert.Equal(t, http.StatusNotFound, wHTML.Code)
		assert.Contains(t, wHTML.Body.String(), "INVALID DOCUMENT")
	})

	t.Run("VerifyDocument nil service HTML mode", func(t *testing.T) {
		nilSrv := &Server{SignatureSvc: nil}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/verify/doc/any-token", nil)
		c.Request.Header.Set("Accept", "text/html")
		c.Params = gin.Params{{Key: "token", Value: "any-token"}}
		nilSrv.VerifyDocument(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID DOCUMENT")
	})
}

type mockErrSigService struct {
	errToReturn error
}

func (m *mockErrSigService) UploadSignature(ctx context.Context, userID uint, base64Str string) (*response.UserSignatureResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) GetSignature(ctx context.Context, userID uint) (*response.UserSignatureResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) DeleteSignature(ctx context.Context, userID uint) error {
	return m.errToReturn
}

func (m *mockErrSigService) SignTimesheet(ctx context.Context, userID uint, req *request.SignTimesheetRequest, meta service.AuditMeta) (*response.TimesheetSubmissionResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) RevokeTimesheet(ctx context.Context, userID uint, req *request.RevokeTimesheetRequest, meta service.AuditMeta) error {
	return m.errToReturn
}

func (m *mockErrSigService) GetTimesheetSubmission(ctx context.Context, userID uint, month, year int) (*response.TimesheetSubmissionResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) SubmitBeritaAcara(ctx context.Context, userID uint, req *request.SubmitBeritaAcaraRequest, meta service.AuditMeta) (*response.BeritaAcaraSubmissionResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) GetBeritaAcaraReview(ctx context.Context, witnessID uint, submissionID string, meta service.AuditMeta) (*response.BeritaAcaraReviewResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) WitnessAction(ctx context.Context, witnessID uint, submissionID string, req *request.WitnessActionRequest, meta service.AuditMeta) (*response.BeritaAcaraSubmissionResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) RevokeBeritaAcara(ctx context.Context, userID uint, submissionID string, meta service.AuditMeta) error {
	return m.errToReturn
}

func (m *mockErrSigService) GetBeritaAcaraSubmission(ctx context.Context, userID uint, month, year int) (*response.BeritaAcaraSubmissionResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) ListPendingWitnessRequests(ctx context.Context, witnessID uint) ([]response.BeritaAcaraSubmissionResponse, error) {
	return nil, m.errToReturn
}

func (m *mockErrSigService) VerifyDocument(ctx context.Context, token string) (*response.PublicDocumentVerificationResponse, error) {
	return nil, m.errToReturn
}

type sigServiceErrorTestCase struct {
	name       string
	method     string
	path       string
	body       string
	params     gin.Params
	handler    func(srv *Server, c *gin.Context)
	hasUserErr bool
}

func testServiceErrorEndpoint(t *testing.T, tc sigServiceErrorTestCase, svcErr error, expectedStatus int) {
	t.Helper()
	srv := &Server{SignatureSvc: &mockErrSigService{errToReturn: svcErr}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var bodyReader io.Reader
	if tc.body != "" {
		bodyReader = strings.NewReader(tc.body)
	}
	c.Request = httptest.NewRequest(tc.method, tc.path, bodyReader)
	if tc.body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	if len(tc.params) > 0 {
		c.Params = tc.params
	}
	c.Set("userID", uint(1))

	tc.handler(srv, c)
	assert.Equal(t, expectedStatus, w.Code)
}

func TestSignatureHandlers_ServiceErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []sigServiceErrorTestCase{
		{
			name:       "UploadSignature",
			method:     http.MethodPost,
			path:       "/api/v1/user/signature",
			body:       `{"signature_base64":"abc"}`,
			handler:    func(s *Server, c *gin.Context) { s.UploadSignature(c) },
			hasUserErr: true,
		},
		{
			name:       "GetSignature",
			method:     http.MethodGet,
			path:       "/api/v1/user/signature",
			handler:    func(s *Server, c *gin.Context) { s.GetSignature(c) },
			hasUserErr: true,
		},
		{
			name:       "DeleteSignature",
			method:     http.MethodDelete,
			path:       "/api/v1/user/signature",
			handler:    func(s *Server, c *gin.Context) { s.DeleteSignature(c) },
			hasUserErr: true,
		},
		{
			name:       "SignTimesheet",
			method:     http.MethodPost,
			path:       "/api/v1/timesheets/sign",
			body:       `{"month":10,"year":2026}`,
			handler:    func(s *Server, c *gin.Context) { s.SignTimesheet(c) },
			hasUserErr: true,
		},
		{
			name:       "RevokeTimesheet",
			method:     http.MethodPost,
			path:       "/api/v1/timesheets/revoke",
			body:       `{"month":10,"year":2026}`,
			handler:    func(s *Server, c *gin.Context) { s.RevokeTimesheet(c) },
			hasUserErr: true,
		},
		{
			name:       "GetTimesheetSubmissionStatus",
			method:     http.MethodGet,
			path:       "/api/v1/timesheets/status?month=10&year=2026",
			handler:    func(s *Server, c *gin.Context) { s.GetTimesheetSubmissionStatus(c) },
			hasUserErr: false,
		},
		{
			name:       "SubmitBeritaAcara",
			method:     http.MethodPost,
			path:       "/api/v1/berita-acara/submit",
			body:       `{"month":10,"year":2026,"witness_id":2}`,
			handler:    func(s *Server, c *gin.Context) { s.SubmitBeritaAcara(c) },
			hasUserErr: true,
		},
		{
			name:       "ReviewBeritaAcara",
			method:     http.MethodGet,
			path:       "/api/v1/berita-acara/ba-1/review",
			params:     gin.Params{{Key: "id", Value: "ba-1"}},
			handler:    func(s *Server, c *gin.Context) { s.ReviewBeritaAcara(c) },
			hasUserErr: true,
		},
		{
			name:       "WitnessAction",
			method:     http.MethodPost,
			path:       "/api/v1/berita-acara/ba-1/witness-action",
			body:       `{"action":"approve"}`,
			params:     gin.Params{{Key: "id", Value: "ba-1"}},
			handler:    func(s *Server, c *gin.Context) { s.WitnessAction(c) },
			hasUserErr: true,
		},
		{
			name:       "RevokeBeritaAcara",
			method:     http.MethodPost,
			path:       "/api/v1/berita-acara/ba-1/revoke",
			params:     gin.Params{{Key: "id", Value: "ba-1"}},
			handler:    func(s *Server, c *gin.Context) { s.RevokeBeritaAcara(c) },
			hasUserErr: true,
		},
		{
			name:       "GetBeritaAcaraSubmissionStatus",
			method:     http.MethodGet,
			path:       "/api/v1/berita-acara/status?month=10&year=2026",
			handler:    func(s *Server, c *gin.Context) { s.GetBeritaAcaraSubmissionStatus(c) },
			hasUserErr: false,
		},
		{
			name:       "ListPendingWitnessRequests",
			method:     http.MethodGet,
			path:       "/api/v1/berita-acara/pending-witness",
			handler:    func(s *Server, c *gin.Context) { s.ListPendingWitnessRequests(c) },
			hasUserErr: false,
		},
		{
			name:       "VerifyDocument",
			method:     http.MethodGet,
			path:       "/api/v1/verify/doc/mytoken",
			params:     gin.Params{{Key: "token", Value: "mytoken"}},
			handler:    func(s *Server, c *gin.Context) { s.VerifyDocument(c) },
			hasUserErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name+"_InternalServerError", func(t *testing.T) {
			testServiceErrorEndpoint(t, tc, errors.New("db connection failure"), http.StatusInternalServerError)
		})

		if tc.hasUserErr {
			t.Run(tc.name+"_UserError", func(t *testing.T) {
				testServiceErrorEndpoint(t, tc, domain.NewUserError(domain.ErrNotFound, "Entity not found"), http.StatusNotFound)
			})
		}
	}
}

func TestSignatureHandlers_WitnessAction_PasskeyAssertion(t *testing.T) {
	srv, tx, u1, u2 := setupSignatureHandlerTest(t)
	defer tx.Rollback()

	// Upload signatures
	b64 := generateTestBase64PNG(80, 40)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u1.ID, b64)
	_, _ = srv.SignatureSvc.UploadSignature(context.Background(), u2.ID, b64)

	// Create and register passkey for witness u2
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pubBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	require.NoError(t, err)

	credID := []byte("hardware-cred-u2")
	aaguid := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	credRecord := models.WebAuthnCredential{
		UserID:              u2.ID,
		CredentialID:        credID,
		PublicKey:           pubBytes,
		AttestationType:     "none",
		AAGUID:              aaguid,
		AuthenticatorAAGUID: nil,
		SignCount:           1,
		FriendlyName:        "YubiKey 5 NFC",
	}
	require.NoError(t, tx.Create(&credRecord).Error)

	buildTestAssertion := func(challenge string, credIDBytes []byte) *request.WebAuthnAssertionPayload {
		clientData := fmt.Sprintf(`{"type":"webauthn.get","challenge":"%s","origin":"https://timesheet.local"}`, challenge)
		clientDataHash := sha256.Sum256([]byte(clientData))

		authData := make([]byte, 37)
		rpHash := sha256.Sum256([]byte("localhost"))
		copy(authData[0:32], rpHash[:])
		authData[32] = 0x01 // UP bit
		authData[36] = 5    // counter = 5

		verificationData := append(authData, clientDataHash[:]...)
		digest := sha256.Sum256(verificationData)
		sig, sErr := ecdsa.SignASN1(rand.Reader, privKey, digest[:])
		require.NoError(t, sErr)

		return &request.WebAuthnAssertionPayload{
			CredentialID:      base64.RawURLEncoding.EncodeToString(credIDBytes),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData),
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(clientData)),
			Signature:         base64.RawURLEncoding.EncodeToString(sig),
		}
	}

	submitAndReview := func(month, year int) (string, string) {
		// Seed BA activity for the requested month/year
		act := models.DailyActivity{
			UserID:        u1.ID,
			Date:          time.Date(year, time.Month(month), 15, 0, 0, 0, 0, time.UTC),
			StartTime:     "08:00",
			EndTime:       "17:00",
			Status:        "P",
			Activity:      fmt.Sprintf("Security patch verification %d/%d", month, year),
			IsBeritaAcara: true,
			IsActive:      true,
		}
		require.NoError(t, tx.Create(&act).Error)

		// Submit
		submitBody := fmt.Sprintf(`{"month":%d,"year":%d,"witness_id":%d}`, month, year, u2.ID)
		wSub := httptest.NewRecorder()
		cSub, _ := gin.CreateTestContext(wSub)
		cSub.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/submit", strings.NewReader(submitBody))
		cSub.Request.Header.Set("Content-Type", "application/json")
		cSub.Set("userID", u1.ID)
		cSub.Set("role", models.RoleUser)
		srv.SubmitBeritaAcara(cSub)
		require.Equal(t, http.StatusOK, wSub.Code)

		var subResp struct {
			Data response.BeritaAcaraSubmissionResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(wSub.Body.Bytes(), &subResp))
		subID := subResp.Data.ID

		// Review
		wRev := httptest.NewRecorder()
		cRev, _ := gin.CreateTestContext(wRev)
		cRev.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/"+subID+"/review", nil)
		cRev.Params = gin.Params{{Key: "id", Value: subID}}
		cRev.Set("userID", u2.ID)
		cRev.Set("role", models.RoleUser)
		srv.ReviewBeritaAcara(cRev)
		require.Equal(t, http.StatusOK, wRev.Code)

		var revResp struct {
			Data response.BeritaAcaraReviewResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(wRev.Body.Bytes(), &revResp))
		return subID, revResp.Data.SigningChallenge
	}

	t.Run("tampered challenge rejection returns HTTP 400", func(t *testing.T) {
		subID, _ := submitAndReview(1, 2026)
		assertion := buildTestAssertion("tampered-challenge-999", credID)

		reqBody, _ := json.Marshal(request.WitnessActionRequest{
			Action:    "approve",
			Assertion: assertion,
		})
		wAct := httptest.NewRecorder()
		cAct, _ := gin.CreateTestContext(wAct)
		cAct.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/"+subID+"/witness-action", bytes.NewReader(reqBody))
		cAct.Request.Header.Set("Content-Type", "application/json")
		cAct.Params = gin.Params{{Key: "id", Value: subID}}
		cAct.Set("userID", u2.ID)
		cAct.Set("role", models.RoleUser)

		srv.WitnessAction(cAct)
		assert.Equal(t, http.StatusBadRequest, wAct.Code)
		assert.Contains(t, wAct.Body.String(), "Signing challenge")
	})

	t.Run("unknown credential rejection returns HTTP 404", func(t *testing.T) {
		subID, challenge := submitAndReview(2, 2026)
		assertion := buildTestAssertion(challenge, []byte("unregistered-cred-id"))

		reqBody, _ := json.Marshal(request.WitnessActionRequest{
			Action:    "approve",
			Assertion: assertion,
		})
		wAct := httptest.NewRecorder()
		cAct, _ := gin.CreateTestContext(wAct)
		cAct.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/"+subID+"/witness-action", bytes.NewReader(reqBody))
		cAct.Request.Header.Set("Content-Type", "application/json")
		cAct.Params = gin.Params{{Key: "id", Value: subID}}
		cAct.Set("userID", u2.ID)
		cAct.Set("role", models.RoleUser)

		srv.WitnessAction(cAct)
		assert.Equal(t, http.StatusNotFound, wAct.Code)
		assert.Contains(t, wAct.Body.String(), "Kredensial passkey tidak ditemukan")
	})

	t.Run("valid passkey assertion approval returns HTTP 200 and signed status", func(t *testing.T) {
		subID, challenge := submitAndReview(3, 2026)
		assertion := buildTestAssertion(challenge, credID)

		reqBody, _ := json.Marshal(request.WitnessActionRequest{
			Action:    "approve",
			Notes:     "Approved with biometric passkey",
			Assertion: assertion,
		})
		wAct := httptest.NewRecorder()
		cAct, _ := gin.CreateTestContext(wAct)
		cAct.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/"+subID+"/witness-action", bytes.NewReader(reqBody))
		cAct.Request.Header.Set("Content-Type", "application/json")
		cAct.Params = gin.Params{{Key: "id", Value: subID}}
		cAct.Set("userID", u2.ID)
		cAct.Set("role", models.RoleUser)

		srv.WitnessAction(cAct)
		assert.Equal(t, http.StatusOK, wAct.Code)

		var actResp struct {
			Data response.BeritaAcaraSubmissionResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(wAct.Body.Bytes(), &actResp))
		assert.Equal(t, models.SubmissionStatusSigned, actResp.Data.Status)
		assert.NotNil(t, actResp.Data.WitnessSignedAt)
	})

	t.Run("graceful fallback with nil assertion returns HTTP 200", func(t *testing.T) {
		subID, _ := submitAndReview(4, 2026)

		reqBody := `{"action":"approve","notes":"Standard approval"}`
		wAct := httptest.NewRecorder()
		cAct, _ := gin.CreateTestContext(wAct)
		cAct.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/"+subID+"/witness-action", strings.NewReader(reqBody))
		cAct.Request.Header.Set("Content-Type", "application/json")
		cAct.Params = gin.Params{{Key: "id", Value: subID}}
		cAct.Set("userID", u2.ID)
		cAct.Set("role", models.RoleUser)

		srv.WitnessAction(cAct)
		assert.Equal(t, http.StatusOK, wAct.Code)

		var actResp struct {
			Data response.BeritaAcaraSubmissionResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(wAct.Body.Bytes(), &actResp))
		assert.Equal(t, models.SubmissionStatusSigned, actResp.Data.Status)
	})

	t.Run("witness rejection returns HTTP 200 and rejected status", func(t *testing.T) {
		subID, _ := submitAndReview(5, 2026)

		reqBody := `{"action":"reject","notes":"Incident notes require elaboration"}`
		wAct := httptest.NewRecorder()
		cAct, _ := gin.CreateTestContext(wAct)
		cAct.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara/"+subID+"/witness-action", strings.NewReader(reqBody))
		cAct.Request.Header.Set("Content-Type", "application/json")
		cAct.Params = gin.Params{{Key: "id", Value: subID}}
		cAct.Set("userID", u2.ID)
		cAct.Set("role", models.RoleUser)

		srv.WitnessAction(cAct)
		assert.Equal(t, http.StatusOK, wAct.Code)

		var actResp struct {
			Data response.BeritaAcaraSubmissionResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(wAct.Body.Bytes(), &actResp))
		assert.Equal(t, models.SubmissionStatusRejected, actResp.Data.Status)
		assert.Equal(t, "Incident notes require elaboration", actResp.Data.WitnessRejectionNotes)
	})
}
