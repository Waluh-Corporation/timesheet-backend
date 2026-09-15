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
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp["data"].(map[string]interface{})
		origins, _ := data["origins"].([]interface{})
		if len(origins) != 2 {
			t.Errorf("expected 2 origins, got %v", data["origins"])
		}
	})

	t.Run("Login rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ForgotPassword rejects empty payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ForgotPassword(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ResetPassword rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.ResetPassword(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("BeginPasskeyLogin without webauthn returns 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.BeginPasskeyLogin(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("FinishPasskeyLogin rejects empty payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.FinishPasskeyLogin(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})
}
