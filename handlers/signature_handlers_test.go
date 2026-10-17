package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/dto/request"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

type fakeDocusealClient struct {
	enabled      bool
	submitterRes *services.DocusealSubmitterResponse
	submitterErr error
	assetBytes   []byte
	assetErr     error
}

func (f *fakeDocusealClient) IsEnabled() bool {
	return f.enabled
}

func (f *fakeDocusealClient) CreateSpecimenSubmission(ctx context.Context, userID uint, name, email, employeeID, division string) (*services.DocusealSubmitterResponse, error) {
	if f.submitterErr != nil {
		return nil, f.submitterErr
	}
	return f.submitterRes, nil
}

func (f *fakeDocusealClient) GetSubmitter(ctx context.Context, submitterID int64) (*services.DocusealSubmitterResponse, error) {
	if f.submitterErr != nil {
		return nil, f.submitterErr
	}
	return f.submitterRes, nil
}

func (f *fakeDocusealClient) DownloadAsset(ctx context.Context, assetURL string) ([]byte, error) {
	return f.assetBytes, f.assetErr
}

func (f *fakeDocusealClient) RewriteEmbedURL(slug string, rawEmbedSrc string) string {
	return "https://docuseal.fafr.my.id/s/" + slug
}

func TestSignatureHandlers_EndToEndFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	require.NoError(t, tx.Exec(`
		CREATE TABLE IF NOT EXISTS user_signatures (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
			docuseal_submission_id BIGINT NOT NULL DEFAULT 0,
			docuseal_submitter_id BIGINT NOT NULL DEFAULT 0,
			docuseal_slug VARCHAR(128) NOT NULL DEFAULT '',
			signature_type VARCHAR(16) NOT NULL DEFAULT 'drawn',
			signature_text VARCHAR(255),
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			s3_signature_key VARCHAR(512),
			s3_audit_pdf_key VARCHAR(512),
			signer_ip VARCHAR(45),
			signer_user_agent TEXT,
			signed_at TIMESTAMPTZ,
			is_active BOOLEAN NOT NULL DEFAULT FALSE
		)
	`).Error)

	cfg.DocusealEnabled = true
	cfg.DocusealPublicURL = "https://docuseal.fafr.my.id"
	cfg.DocusealInternalURL = "http://192.168.0.2:3003"
	cfg.DocusealAPIKey = "test-api-key"
	cfg.DocusealWebhookSecret = "webhook-secret-key"
	cfg.DocusealSpecimenTemplateID = 1
	cfg.DocusealSignatureFieldName = "Signature"

	srv := newTestServer(t, tx, cfg)
	mockStorage := &mockStorageService{
		existsResult: true,
		downloadURL:  "https://s3.example.com/signatures/preview.png",
	}
	fakeDS := &fakeDocusealClient{
		enabled: true,
		submitterRes: &services.DocusealSubmitterResponse{
			ID:           5001,
			SubmissionID: 3001,
			Slug:         "sigSlug5001",
			Status:       "completed",
			ExternalID:   "user-1",
			EmbedSrc:     "https://docuseal.fafr.my.id/s/sigSlug5001",
			Values: []services.DocusealSubmitterValue{
				{
					Field: "Signature",
					Value: "Faisal Rahman",
				},
			},
		},
	}
	srv.DocusealClient = fakeDS
	srv.SetStorage(mockStorage)

	user := &models.User{
		Username:     "faisal_sig",
		Email:        "faisal.sig@example.com",
		Name:         "Faisal Al Munawar Fathur Rahman",
		Role:         models.RoleUser,
		PasswordHash: "secret",
		IsActive:     true,
	}
	userRepo := repository.NewUserRepository(tx)
	require.NoError(t, userRepo.Create(context.Background(), user))
	fakeDS.submitterRes.ExternalID = fmt.Sprintf("user-%d", user.ID)

	// Update SignatureSvc with fakeDS and mockStorage
	srv.SignatureSvc = service.NewSignatureService(cfg, srv.SignatureRepo, srv.UserRepo, fakeDS, mockStorage)

	// 1. GET /profile/signature initially returns status "none" and abbreviated default_signature_text
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/profile/signature", nil)
	c1.Set(ctxUserID, user.ID)
	srv.GetMySignature(c1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Contains(t, w1.Body.String(), `"default_signature_text":"Faisal Rahman"`)
	assert.Contains(t, w1.Body.String(), `"has_active_signature":false`)

	// 2. POST /profile/signature/session initializes pending session
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/signature/session", nil)
	c2.Set(ctxUserID, user.ID)
	srv.CreateSignatureSession(c2)
	assert.Equal(t, http.StatusCreated, w2.Code)
	assert.Contains(t, w2.Body.String(), `"embed_src":"https://docuseal.fafr.my.id/s/sigSlug5001"`)

	// 3. POST /profile/signature/sync renders cursive PNG, uploads to S3, and activates specimen
	syncBody, _ := json.Marshal(request.SyncSignatureRequest{SubmitterID: 5001})
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/signature/sync", bytes.NewReader(syncBody))
	c3.Request.Header.Set("Content-Type", "application/json")
	c3.Set(ctxUserID, user.ID)
	srv.SyncMySignature(c3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Contains(t, w3.Body.String(), `"has_active_signature":true`)
	assert.Contains(t, w3.Body.String(), `"signature_type":"typed"`)
	assert.Contains(t, w3.Body.String(), `"preview_url":"https://s3.example.com/signatures/preview.png"`)

	// 4. POST /webhooks/docuseal with valid HMAC-SHA256 signature succeeds
	webhookPayload := request.DocusealWebhookPayload{
		EventType: "form.completed",
		Data: request.DocusealWebhookData{
			ID:           5001,
			SubmissionID: 3001,
			Slug:         "sigSlug5001",
			Status:       "completed",
			Values: []request.DocusealWebhookFieldValue{
				{
					Field: "Signature",
					Value: "Faisal Rahman",
				},
			},
		},
	}
	whBytes, _ := json.Marshal(webhookPayload)
	mac := hmac.New(sha256.New, []byte(cfg.DocusealWebhookSecret))
	mac.Write(whBytes)
	sigHex := hex.EncodeToString(mac.Sum(nil))

	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/docuseal", bytes.NewReader(whBytes))
	c4.Request.Header.Set("Content-Type", "application/json")
	c4.Request.Header.Set("X-Docuseal-Signature", sigHex)
	srv.DocusealWebhook(c4)
	assert.Equal(t, http.StatusOK, w4.Code)

	// 5. POST /webhooks/docuseal with invalid signature returns 401 Unauthorized
	w5 := httptest.NewRecorder()
	c5, _ := gin.CreateTestContext(w5)
	c5.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/docuseal", bytes.NewReader(whBytes))
	c5.Request.Header.Set("Content-Type", "application/json")
	c5.Request.Header.Set("X-Docuseal-Signature", "bad-signature")
	srv.DocusealWebhook(c5)
	assert.Equal(t, http.StatusUnauthorized, w5.Code)

	// 6. DELETE /profile/signature revokes active specimen
	w6 := httptest.NewRecorder()
	c6, _ := gin.CreateTestContext(w6)
	c6.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/profile/signature", nil)
	c6.Set(ctxUserID, user.ID)
	srv.DeleteMySignature(c6)
	assert.Equal(t, http.StatusOK, w6.Code)
}
