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

	// 4b. Verify Document via Tabler HTML (Browser mode via Accept header)
	wVerHTML := httptest.NewRecorder()
	cVerHTML, _ := gin.CreateTestContext(wVerHTML)
	cVerHTML.Request = httptest.NewRequest(http.MethodGet, "/verify/doc/"+subResp.Data.VerificationToken, nil)
	cVerHTML.Request.Header.Set("Accept", "text/html,application/xhtml+xml")
	cVerHTML.Params = gin.Params{{Key: "token", Value: subResp.Data.VerificationToken}}
	srv.VerifyDocument(cVerHTML)
	assert.Equal(t, http.StatusOK, wVerHTML.Code)
	assert.Contains(t, wVerHTML.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, wVerHTML.Body.String(), "DOKUMEN ASLI &amp; TERVERIFIKASI")
	assert.Contains(t, wVerHTML.Body.String(), u1.Name)
	assert.Contains(t, wVerHTML.Body.String(), u2.Name)

	// 4c. Verify Document via Tabler HTML (?format=html query param)
	wVerQuery := httptest.NewRecorder()
	cVerQuery, _ := gin.CreateTestContext(wVerQuery)
	cVerQuery.Request = httptest.NewRequest(http.MethodGet, "/verify/doc/"+subResp.Data.VerificationToken+"?format=html", nil)
	cVerQuery.Params = gin.Params{{Key: "token", Value: subResp.Data.VerificationToken}}
	srv.VerifyDocument(cVerQuery)
	assert.Equal(t, http.StatusOK, wVerQuery.Code)
	assert.Contains(t, wVerQuery.Body.String(), "DOKUMEN ASLI &amp; TERVERIFIKASI")

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
		assert.Contains(t, wHTML.Body.String(), "DOKUMEN TIDAK VALID")
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
		assert.Contains(t, w.Body.String(), "DOKUMEN TIDAK VALID")
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

func TestSignatureHandlers_ServiceErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		params     gin.Params
		handler    func(srv *Server, c *gin.Context)
		hasUserErr bool
	}{
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
			mockSvc := &mockErrSigService{errToReturn: errors.New("db connection failure")}
			srv := &Server{SignatureSvc: mockSvc}

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
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		if tc.hasUserErr {
			t.Run(tc.name+"_UserError", func(t *testing.T) {
				mockSvc := &mockErrSigService{errToReturn: domain.NewUserError(domain.ErrNotFound, "Entity not found")}
				srv := &Server{SignatureSvc: mockSvc}

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
				assert.Equal(t, http.StatusNotFound, w.Code)
			})
		}
	}
}
