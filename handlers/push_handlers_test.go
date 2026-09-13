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
}
