package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestPushHandlers_Extended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	pushSvc := push.New(cfg, tx)
	srv := &Server{
		DB:   tx,
		Cfg:  cfg,
		Push: pushSvc,
	}

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

	t.Run("GetPushSchedule enabled and disabled", func(t *testing.T) {
		srv.Cfg.ReminderCron = "0 17 * * *"
		srv.Cfg.Timezone = "Asia/Jakarta"

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/push/schedule", nil)
		c.Set(ctxUserID, uid)

		srv.GetPushSchedule(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		if data["cron_expression"] != "0 17 * * *" || data["timezone"] != "Asia/Jakarta" || data["is_enabled"] != true {
			t.Errorf("unexpected schedule data: %v", data)
		}

		// Test disabled
		srv.Cfg.ReminderCron = "disabled"
		wDis := httptest.NewRecorder()
		cDis, _ := gin.CreateTestContext(wDis)
		cDis.Request = httptest.NewRequest(http.MethodGet, "/api/v1/push/schedule", nil)
		srv.GetPushSchedule(cDis)
		assertResponseCode(t, wDis, http.StatusOK)
	})

	t.Run("AdminSendTestPush validation and dispatch", func(t *testing.T) {
		// Missing user_id -> 400
		wMissing := httptest.NewRecorder()
		cMissing, _ := gin.CreateTestContext(wMissing)
		cMissing.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/push/test", bytes.NewReader([]byte(`{}`)))
		cMissing.Request.Header.Set("Content-Type", "application/json")
		srv.AdminSendTestPush(cMissing)
		assertResponseCode(t, wMissing, http.StatusBadRequest)

		// Non-existent user -> 404
		wNotFound := httptest.NewRecorder()
		cNotFound, _ := gin.CreateTestContext(wNotFound)
		cNotFound.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/push/test", bytes.NewReader([]byte(`{"user_id": 99999}`)))
		cNotFound.Request.Header.Set("Content-Type", "application/json")
		srv.AdminSendTestPush(cNotFound)
		assertResponseCode(t, wNotFound, http.StatusNotFound)

		// Valid user via JSON body with custom title and body
		reqBody := request.AdminTestPushRequest{
			UserID: uid,
			Title:  "Custom Admin Test",
			Body:   "Custom notification body",
			URL:    "/custom-url",
		}
		raw, _ := json.Marshal(reqBody)
		wOK := httptest.NewRecorder()
		cOK, _ := gin.CreateTestContext(wOK)
		cOK.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/push/test", bytes.NewReader(raw))
		cOK.Request.Header.Set("Content-Type", "application/json")
		srv.AdminSendTestPush(cOK)
		assertResponseCode(t, wOK, http.StatusOK)

		// Valid user via path param :id
		wPath := httptest.NewRecorder()
		cPath, _ := gin.CreateTestContext(wPath)
		cPath.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/1/push/test", nil)
		cPath.Params = gin.Params{{Key: "id", Value: "1"}}
		srv.AdminSendTestPush(cPath)
		// User 1 exists or may not, check it handles resolution
		if wPath.Code != http.StatusOK && wPath.Code != http.StatusNotFound {
			t.Errorf("expected 200 or 404 for path param test, got %d", wPath.Code)
		}

		// Valid user via query param ?user_id=...
		wQuery := httptest.NewRecorder()
		cQuery, _ := gin.CreateTestContext(wQuery)
		cQuery.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/push/test?user_id=1", nil)
		srv.AdminSendTestPush(cQuery)
		if wQuery.Code != http.StatusOK && wQuery.Code != http.StatusNotFound {
			t.Errorf("expected 200 or 404 for query param test, got %d", wQuery.Code)
		}
	})
}
