package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
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
		srv.FinishPasskeyLogin(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("RefreshToken rejects invalid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.RefreshToken(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("Auth endpoints with nil repositories return 500", func(t *testing.T) {
		// Login nil repo
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(`{"identifier":"user","password":"pass"}`)))
		c1.Request.Header.Set("Content-Type", "application/json")
		srv.Login(c1)
		assertResponseCode(t, w1, http.StatusInternalServerError)

		// RefreshToken nil repo
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{"refresh_token":"abc"}`)))
		c2.Request.Header.Set("Content-Type", "application/json")
		srv.RefreshToken(c2)
		assertResponseCode(t, w2, http.StatusInternalServerError)

		// Me nil repo
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		srv.Me(c3)
		assertResponseCode(t, w3, http.StatusInternalServerError)

		// ResetPassword nil repo
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader([]byte(`{"token":"abc","password":"Password123!"}`)))
		c4.Request.Header.Set("Content-Type", "application/json")
		srv.ResetPassword(c4)
		assertResponseCode(t, w4, http.StatusInternalServerError)

		// BeginPasskeyRegistration nil repo
		w5 := httptest.NewRecorder()
		c5, _ := gin.CreateTestContext(w5)
		c5.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/begin", nil)
		srv.BeginPasskeyRegistration(c5)
		assertResponseCode(t, w5, http.StatusInternalServerError)

		// ListPasskeys nil repo
		w6 := httptest.NewRecorder()
		c6, _ := gin.CreateTestContext(w6)
		c6.Request = httptest.NewRequest(http.MethodGet, "/api/v1/passkeys", nil)
		srv.ListPasskeys(c6)
		assertResponseCode(t, w6, http.StatusInternalServerError)

		// DeletePasskey nil repo
		w7 := httptest.NewRecorder()
		c7, _ := gin.CreateTestContext(w7)
		c7.Params = gin.Params{{Key: "id", Value: "1"}}
		c7.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/passkeys/1", nil)
		srv.DeletePasskey(c7)
		assertResponseCode(t, w7, http.StatusInternalServerError)

		// AdminListPasskeys nil repo
		w8 := httptest.NewRecorder()
		c8, _ := gin.CreateTestContext(w8)
		c8.Params = gin.Params{{Key: "id", Value: "1"}}
		c8.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1/passkeys", nil)
		srv.AdminListPasskeys(c8)
		assertResponseCode(t, w8, http.StatusInternalServerError)

		// AdminDeletePasskey nil repo
		w9 := httptest.NewRecorder()
		c9, _ := gin.CreateTestContext(w9)
		c9.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "pid", Value: "1"}}
		c9.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1/passkeys/1", nil)
		srv.AdminDeletePasskey(c9)
		assertResponseCode(t, w9, http.StatusInternalServerError)
	})

	t.Run("Passkey endpoints reject invalid id parameters", func(t *testing.T) {
		// DeletePasskey invalid id
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Params = gin.Params{{Key: "id", Value: "not-an-id"}}
		c1.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/passkeys/not-an-id", nil)
		srv.DeletePasskey(c1)
		assertResponseCode(t, w1, http.StatusBadRequest)

		// AdminListPasskeys invalid user id
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Params = gin.Params{{Key: "id", Value: "not-an-id"}}
		c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/not-an-id/passkeys", nil)
		srv.AdminListPasskeys(c2)
		assertResponseCode(t, w2, http.StatusBadRequest)

		// AdminDeletePasskey invalid pid
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "pid", Value: "not-an-id"}}
		c3.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1/passkeys/not-an-id", nil)
		srv.AdminDeletePasskey(c3)
		assertResponseCode(t, w3, http.StatusBadRequest)

		// FinishPasskeyRegistration unknown session
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=unknown-sid", nil)
		srv.FinishPasskeyRegistration(c4)
		assertResponseCode(t, w4, http.StatusBadRequest)

		// FinishPasskeyLogin unknown session
		w5 := httptest.NewRecorder()
		c5, _ := gin.CreateTestContext(w5)
		c5.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish?session_id=unknown-sid", nil)
		srv.FinishPasskeyLogin(c5)
		assertResponseCode(t, w5, http.StatusBadRequest)

		// Put dummy session and test nil repo for finish endpoints
		srv.putSession("test-dummy-sid", &webauthn.SessionData{})
		w6 := httptest.NewRecorder()
		c6, _ := gin.CreateTestContext(w6)
		c6.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/finish?session_id=test-dummy-sid", nil)
		srv.FinishPasskeyRegistration(c6)
		assertResponseCode(t, w6, http.StatusInternalServerError)

		srv.putSession("test-dummy-sid-2", &webauthn.SessionData{})
		w7 := httptest.NewRecorder()
		c7, _ := gin.CreateTestContext(w7)
		c7.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish?session_id=test-dummy-sid-2", nil)
		srv.FinishPasskeyLogin(c7)
		assertResponseCode(t, w7, http.StatusInternalServerError)

		// Me nil repo
		wMe := httptest.NewRecorder()
		cMe, _ := gin.CreateTestContext(wMe)
		cMe.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		srv.Me(cMe)
		assertResponseCode(t, wMe, http.StatusInternalServerError)

		// BeginPasskeyRegistration nil repo
		wBPR := httptest.NewRecorder()
		cBPR, _ := gin.CreateTestContext(wBPR)
		cBPR.Request = httptest.NewRequest(http.MethodPost, "/api/v1/passkey/register/begin", nil)
		srv.BeginPasskeyRegistration(cBPR)
		assertResponseCode(t, wBPR, http.StatusInternalServerError)

		// BeginPasskeyLogin nil WebAuthn
		wBPL := httptest.NewRecorder()
		cBPL, _ := gin.CreateTestContext(wBPL)
		cBPL.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", nil)
		srv.BeginPasskeyLogin(cBPL)
		assertResponseCode(t, wBPL, http.StatusInternalServerError)

		// BeginPasskeyLogin with identifier and nil repo
		wa, _ := webauthn.New(&webauthn.Config{RPDisplayName: "Test", RPID: "localhost"})
		srvWithWA := &Server{WebAuthn: wa}
		wBPL2 := httptest.NewRecorder()
		cBPL2, _ := gin.CreateTestContext(wBPL2)
		cBPL2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", strings.NewReader(`{"identifier":"user123"}`))
		cBPL2.Request.Header.Set("Content-Type", "application/json")
		srvWithWA.BeginPasskeyLogin(cBPL2)
		assertResponseCode(t, wBPL2, http.StatusInternalServerError)

		// Logout with empty token
		wLogout := httptest.NewRecorder()
		cLogout, _ := gin.CreateTestContext(wLogout)
		cLogout.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{}`))
		cLogout.Request.Header.Set("Content-Type", "application/json")
		srv.Logout(cLogout)
		assertResponseCode(t, wLogout, http.StatusOK)
	})
}
