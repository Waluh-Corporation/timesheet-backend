package handlers

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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/dto/response"
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
}
