package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestResponseHelpers(t *testing.T) {
	t.Run("RespondSuccess envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		type TestData struct {
			Name string `json:"name"`
		}
		RespondSuccess(c, http.StatusOK, TestData{Name: "Antigravity"})

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Code   int                    `json:"code"`
			Status string                 `json:"status"`
			Data   map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp.Code != 200 {
			t.Errorf("expected code 200, got %d", resp.Code)
		}
		if resp.Status != "success" {
			t.Errorf("expected status 'success', got %s", resp.Status)
		}
		if resp.Data["name"] != "Antigravity" {
			t.Errorf("expected data.name 'Antigravity', got %v", resp.Data["name"])
		}
	})

	t.Run("RespondMessage envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		RespondMessage(c, http.StatusOK, "action completed")

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Code    int    `json:"code"`
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp.Code != 200 || resp.Status != "success" || resp.Message != "action completed" {
			t.Errorf("unexpected message response: %+v", resp)
		}
	})

	t.Run("RespondError envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		RespondError(c, http.StatusBadRequest, "invalid payload")

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}

		var resp struct {
			Code    int    `json:"code"`
			Status  string `json:"status"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp.Code != 400 || resp.Status != "error" || resp.Error != "invalid payload" || resp.Message != "invalid payload" {
			t.Errorf("unexpected error response: %+v", resp)
		}
	})

	t.Run("RespondDelete envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		RespondDelete(c, http.StatusOK)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Code    int    `json:"code"`
			Status  string `json:"status"`
			Deleted bool   `json:"deleted"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp.Code != 200 || resp.Status != "success" || !resp.Deleted {
			t.Errorf("unexpected delete response: %+v", resp)
		}
	})
}
