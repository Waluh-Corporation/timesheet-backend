package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/config"
)

func TestDocusealClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/submissions", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-Auth-Token"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode([]DocusealSubmitterResponse{
			{
				ID:           101,
				SubmissionID: 55,
				Slug:         "slugABC123",
				Email:        "faisal@example.com",
				Name:         "Faisal Rahman",
				Status:       "pending",
				ExternalID:   "user-7",
				EmbedSrc:     "http://192.168.0.2:3003/s/slugABC123",
			},
		})
	})

	mux.HandleFunc("/api/submitters/101", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-Auth-Token"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DocusealSubmitterResponse{
			ID:           101,
			SubmissionID: 55,
			Slug:         "slugABC123",
			Status:       "completed",
			ExternalID:   "user-7",
			EmbedSrc:     "http://192.168.0.2:3003/s/slugABC123",
			Values: []DocusealSubmitterValue{
				{
					Field: "Signature",
					Value: "Faisal Rahman",
				},
			},
		})
	})

	mux.HandleFunc("/blobs/sig.png", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock-png-bytes"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		DocusealEnabled:            true,
		DocusealInternalURL:        server.URL,
		DocusealPublicURL:          "https://docuseal.fafr.my.id",
		DocusealAPIKey:             "test-api-key",
		DocusealSpecimenTemplateID: 1,
		DocusealSignatureFieldName: "Signature",
	}
	client := NewDocusealClient(cfg)
	ctx := context.Background()

	t.Run("CreateSpecimenSubmission rewrites embed_src to public URL", func(t *testing.T) {
		sub, err := client.CreateSpecimenSubmission(ctx, 7, "Faisal Al Munawar Fathur Rahman", "faisal@example.com", "NPP123", "IT")
		require.NoError(t, err)
		require.NotNil(t, sub)
		assert.Equal(t, int64(101), sub.ID)
		assert.Equal(t, int64(55), sub.SubmissionID)
		assert.Equal(t, "slugABC123", sub.Slug)
		assert.Equal(t, "https://docuseal.fafr.my.id/s/slugABC123", sub.EmbedSrc)
	})

	t.Run("GetSubmitter retrieves completed submitter and extracts signature value", func(t *testing.T) {
		sub, err := client.GetSubmitter(ctx, 101)
		require.NoError(t, err)
		require.NotNil(t, sub)
		assert.Equal(t, "completed", sub.Status)

		val, isImg, isDataURI := ExtractSignatureValue(sub.Values, "Signature")
		assert.Equal(t, "Faisal Rahman", val)
		assert.False(t, isImg)
		assert.False(t, isDataURI)
	})

	t.Run("DownloadAsset rewrites public URL to internal URL", func(t *testing.T) {
		data, err := client.DownloadAsset(ctx, "https://docuseal.fafr.my.id/blobs/sig.png")
		require.NoError(t, err)
		assert.Equal(t, []byte("mock-png-bytes"), data)
	})

	t.Run("VerifyDocusealWebhookSignature validates HMAC-SHA256", func(t *testing.T) {
		body := []byte(`{"event_type":"form.completed"}`)
		secret := "my-webhook-secret"
		now := time.Now()

		// Direct hex and sha256= prefixed
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		sigHex := hex.EncodeToString(mac.Sum(nil))
		assert.True(t, VerifyDocusealWebhookSignature(sigHex, body, secret, now))
		assert.True(t, VerifyDocusealWebhookSignature("sha256="+sigHex, body, secret, now))

		// Timestamped [ts].[sig]
		tsStr := fmt.Sprintf("%d", now.Unix())
		tsMac := hmac.New(sha256.New, []byte(secret))
		tsMac.Write([]byte(tsStr + "."))
		tsMac.Write(body)
		tsSig := tsStr + "." + hex.EncodeToString(tsMac.Sum(nil))
		assert.True(t, VerifyDocusealWebhookSignature(tsSig, body, secret, now))

		assert.False(t, VerifyDocusealWebhookSignature("invalid-sig", body, secret, now))
	})
}
