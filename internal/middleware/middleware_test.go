package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestID(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Case 1: Header generated automatically
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if reqID := w.Header().Get(middleware.HeaderXRequestID); reqID == "" {
		t.Errorf("expected X-Request-ID header to be set")
	}

	// Case 2: Inbound header preserved
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set(middleware.HeaderXRequestID, "custom-id-123")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if reqID := w2.Header().Get(middleware.HeaderXRequestID); reqID != "custom-id-123" {
		t.Errorf("expected preserved X-Request-ID, got %s", reqID)
	}
}

func TestSecurityHeaders(t *testing.T) {
	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options: DENY")
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Errorf("expected Content-Security-Policy to be set")
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := middleware.NewIPRateLimiter(2, 50*time.Millisecond)
	r := gin.New()
	r.Use(middleware.RateLimitMiddleware(limiter))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d expected 200, got %d", i, w.Code)
		}
	}

	// 3rd request should be blocked
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests, got %d", w.Code)
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("after window reset expected 200, got %d", w2.Code)
	}
}

func TestStructuredRecovery(t *testing.T) {
	r := gin.New()
	r.Use(middleware.StructuredRecovery())
	r.GET("/panic", func(c *gin.Context) {
		panic("simulated test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
