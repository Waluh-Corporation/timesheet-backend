package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"timesheet-backend/config"
)

func TestAuthHandlers_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RPOrigins: []string{"http://localhost:3000", "https://timesheet.example.com"},
	}
	srv := &Server{Cfg: cfg}

	t.Run("WebAuthnRelatedOrigins returns configured origins", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/.well-known/webauthn", nil)

		srv.WebAuthnRelatedOrigins(c)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		data, ok := resp["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("missing data envelope in response")
		}
		origins, ok := data["origins"].([]interface{})
		if !ok || len(origins) != 2 {
			t.Errorf("expected 2 origins, got %v", data["origins"])
		}
	})

	t.Run("Login rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty login payload, got %d", w.Code)
		}
	})

	t.Run("ForgotPassword rejects empty payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ForgotPassword(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty forgot-password, got %d", w.Code)
		}
	})

	t.Run("ResetPassword rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ResetPassword(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty reset-password, got %d", w.Code)
		}
	})

	t.Run("BeginPasskeyLogin without webauthn returns 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.BeginPasskeyLogin(c)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when webauthn not configured, got %d", w.Code)
		}
	})

	t.Run("FinishPasskeyLogin rejects empty payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.FinishPasskeyLogin(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty finish login, got %d", w.Code)
		}
	})
}
