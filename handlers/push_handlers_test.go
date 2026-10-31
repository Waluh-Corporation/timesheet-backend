package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/config"
	"timesheet-backend/dto/request"
	"timesheet-backend/models"
	"timesheet-backend/push"
)

func TestPushHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		VAPIDPublicKey:  "test-public-key",
		VAPIDPrivateKey: "test-private-key",
		VAPIDSubject:    "mailto:test@example.com",
	}
	pushSvc := push.New(cfg, nil)

	srv := &Server{
		Push: pushSvc,
	}

	// 1. GetVAPIDKey
	wKey := httptest.NewRecorder()
	cKey, _ := gin.CreateTestContext(wKey)
	srv.GetVAPIDKey(cKey)
	if wKey.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wKey.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(wKey.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok || data["public_key"] != "test-public-key" {
		t.Errorf("expected public_key 'test-public-key', got %v", resp)
	}

	// 2. Subscribe with invalid body returns 400
	wSubErr := httptest.NewRecorder()
	cSubErr, _ := gin.CreateTestContext(wSubErr)
	cSubErr.Request = httptest.NewRequest(http.MethodPost, "/api/v1/push/subscribe", bytes.NewReader([]byte(`invalid json`)))
	cSubErr.Request.Header.Set("Content-Type", "application/json")
	srv.Subscribe(cSubErr)
	if wSubErr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid json, got %d", wSubErr.Code)
	}
}

func TestGetPushSchedule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Default configuration", func(t *testing.T) {
		srv := &Server{
			Cfg: &config.Config{
				Timezone:     "UTC",
				ReminderCron: "0 17 * * *",
			},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srv.GetPushSchedule(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data, ok := resp["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected data map in response, got %v", resp)
		}
		if data["hour"] != float64(17) {
			t.Errorf("expected hour 17, got %v", data["hour"])
		}
		if data["minute"] != float64(0) {
			t.Errorf("expected minute 0, got %v", data["minute"])
		}
		if data["timezone"] != "UTC" {
			t.Errorf("expected timezone UTC, got %v", data["timezone"])
		}
		if data["is_enabled"] != true {
			t.Errorf("expected is_enabled true, got %v", data["is_enabled"])
		}
		if data["next_run"] == nil {
			t.Errorf("expected non-nil next_run")
		}
		if data["human_readable"] == "" {
			t.Errorf("expected non-empty human_readable")
		}
	})

	t.Run("Custom timezone and weekdays schedule", func(t *testing.T) {
		srv := &Server{
			Cfg: &config.Config{
				Timezone:     "Asia/Jakarta",
				ReminderCron: "30 18 * * 1-5",
			},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srv.GetPushSchedule(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data := resp["data"].(map[string]interface{})
		if data["hour"] != float64(18) {
			t.Errorf("expected hour 18, got %v", data["hour"])
		}
		if data["minute"] != float64(30) {
			t.Errorf("expected minute 30, got %v", data["minute"])
		}
		if data["timezone"] != "Asia/Jakarta" {
			t.Errorf("expected timezone Asia/Jakarta, got %v", data["timezone"])
		}
		hr, _ := data["human_readable"].(string)
		if !strings.Contains(hr, "18:30") || !strings.Contains(hr, "Asia/Jakarta") {
			t.Errorf("expected human_readable containing '18:30' and 'Asia/Jakarta', got %s", hr)
		}
	})

	t.Run("Disabled reminder cron", func(t *testing.T) {
		srv := &Server{
			Cfg: &config.Config{
				Timezone:     "Asia/Jakarta",
				ReminderCron: "disabled",
			},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srv.GetPushSchedule(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data := resp["data"].(map[string]interface{})
		if data["is_enabled"] != false {
			t.Errorf("expected is_enabled false, got %v", data["is_enabled"])
		}
	})

	t.Run("Nil server and nil config graceful fallback with dynamic TZ", func(t *testing.T) {
		t.Setenv("TZ", "Asia/Makassar")

		var srvNil *Server
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srvNil.GetPushSchedule(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data := resp["data"].(map[string]interface{})
		if data["timezone"] != "Asia/Makassar" {
			t.Errorf("expected timezone Asia/Makassar from AppLocation fallback, got %v", data["timezone"])
		}

		srvEmpty := &Server{}
		wEmpty := httptest.NewRecorder()
		cEmpty, _ := gin.CreateTestContext(wEmpty)
		srvEmpty.GetPushSchedule(cEmpty)

		if wEmpty.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", wEmpty.Code)
		}
	})

	t.Run("Non-nil config with empty timezone falls back to dynamic AppLocation", func(t *testing.T) {
		t.Setenv("TZ", "Asia/Jayapura")

		srvWithEmptyTZ := &Server{
			Cfg: &config.Config{
				Timezone:     "",
				ReminderCron: "15 9 * * *",
			},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srvWithEmptyTZ.GetPushSchedule(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data := resp["data"].(map[string]interface{})
		if data["timezone"] != "Asia/Jayapura" {
			t.Errorf("expected timezone Asia/Jayapura, got %v", data["timezone"])
		}
		if data["hour"] != float64(9) || data["minute"] != float64(15) {
			t.Errorf("expected 9:15, got %v:%v", data["hour"], data["minute"])
		}
	})

	t.Run("Non-standard cron descriptors extract hour and minute from NextRun", func(t *testing.T) {
		srvDaily := &Server{
			Cfg: &config.Config{
				Timezone:     "UTC",
				ReminderCron: "@daily",
			},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srvDaily.GetPushSchedule(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data := resp["data"].(map[string]interface{})
		if data["hour"] != float64(0) || data["minute"] != float64(0) {
			t.Errorf("expected 0:0 for @daily, got %v:%v", data["hour"], data["minute"])
		}
	})

	t.Run("ReminderCron with surrounding whitespace and out-of-range bounds check", func(t *testing.T) {
		srv := &Server{
			Cfg: &config.Config{
				Timezone:     "UTC",
				ReminderCron: "  45 8 * * *  ",
			},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srv.GetPushSchedule(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data := resp["data"].(map[string]interface{})
		if data["hour"] != float64(8) || data["minute"] != float64(45) {
			t.Errorf("expected 8:45, got %v:%v", data["hour"], data["minute"])
		}
	})

	t.Run("Route mounted publicly without auth requirement", func(t *testing.T) {
		srv := &Server{
			Cfg: &config.Config{
				Timezone:     "UTC",
				ReminderCron: "0 17 * * *",
			},
		}

		r := gin.New()
		api := r.Group("/api/v1")
		api.GET("/push/schedule", srv.GetPushSchedule)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/push/schedule", nil)
		// No Authorization header provided
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected public endpoint 200 OK without token, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestPushHandlers_Extended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)

	testUser := models.User{
		Username: "pushtestuser",
		Email:    "pushtest@example.com",
		Role:     models.RoleUser,
		IsActive: true,
	}
	if err := tx.Create(&testUser).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	uid := testUser.ID

	t.Run("Subscribe valid payload", func(t *testing.T) {
		payload := request.SubscribeRequest{
			Endpoint: "https://fcm.googleapis.com/fcm/send/test-endpoint-1",
			Keys: request.PushKeyPayload{
				P256dh: "test-p256dh-key",
				Auth:   "test-auth-secret",
			},
		}
		body, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/push/subscribe", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, uid)

		srv.Subscribe(c)
		assertResponseCode(t, w, http.StatusCreated)
	})

	t.Run("SendTestPush dispatches", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/push/test", nil)
		c.Set(ctxUserID, uid)

		srv.SendTestPush(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("Unsubscribe specific endpoint", func(t *testing.T) {
		payload := request.UnsubscribeRequest{
			Endpoint: "https://fcm.googleapis.com/fcm/send/test-endpoint-1",
		}
		body, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/push/unsubscribe", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, uid)

		srv.Unsubscribe(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	t.Run("Unsubscribe all for user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/push/unsubscribe", nil)
		c.Set(ctxUserID, uid)

		srv.Unsubscribe(c)
		assertResponseCode(t, w, http.StatusOK)
	})
}

func TestPushHandlers_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srvNil := &Server{} // Push == nil

	t.Run("GetVAPIDKey returns 500 when nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srvNil.GetVAPIDKey(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("Subscribe returns 500 when nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		payload := `{"endpoint":"https://fcm.example.com","keys":{"p256dh":"k","auth":"a"}}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/push/subscribe", strings.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		srvNil.Subscribe(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("Unsubscribe returns 500 when nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srvNil.Unsubscribe(c)
		assertResponseCode(t, w, http.StatusInternalServerError)
	})

	t.Run("SendTestPush returns 200 when nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		srvNil.SendTestPush(c)
		assertResponseCode(t, w, http.StatusOK)
	})
}
