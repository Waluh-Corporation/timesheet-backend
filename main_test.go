package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"timesheet-backend/config"
	"timesheet-backend/handlers"

	"github.com/gin-gonic/gin"
)

func assertNotRateLimited(t *testing.T, r *gin.Engine, attempts int, msg string) {
	t.Helper()
	for i := 0; i < attempts; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Errorf("%s, got 429 on iteration %d", msg, i)
		}
	}
}

func TestRegisterRoutes_RateLimitConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("rate limiting disabled", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			RateLimitEnabled: false,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		assertNotRateLimited(t, r, 5, "expected route not to be rate limited")
	})

	t.Run("custom rate limit", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			RateLimitEnabled:  true,
			RateLimitRequests: 1,
			RateLimitWindow:   1 * time.Minute,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		// First request passes through rate limiter (may return 400 or other status, but not 429)
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		if w1.Code == http.StatusTooManyRequests {
			t.Errorf("first request should not be rate limited")
		}

		// Second request should be rate limited (429)
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusTooManyRequests {
			t.Errorf("expected second request to be 429 Too Many Requests, got %d", w2.Code)
		}
	})

	t.Run("zero requests bypasses rate limiting", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			RateLimitEnabled:  true,
			RateLimitRequests: 0,
			RateLimitWindow:   1 * time.Minute,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		assertNotRateLimited(t, r, 5, "expected route not to be rate limited when requests=0")
	})

	t.Run("zero window bypasses rate limiting", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			RateLimitEnabled:  true,
			RateLimitRequests: 1,
			RateLimitWindow:   0,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		assertNotRateLimited(t, r, 5, "expected route not to be rate limited when window=0")
	})

	t.Run("nil server or nil config defaults to rate limiting disabled", func(t *testing.T) {
		r := gin.New()
		registerRoutes(r, nil)

		assertNotRateLimited(t, r, 5, "expected route not to be rate limited with nil server")
	})
}

func TestRegisterRoutes_ActivityPrevious(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := &config.Config{}
	srv := &handlers.Server{Cfg: cfg}
	registerRoutes(r, srv)

	// An unauthenticated request to /api/v1/activities/previous should hit AuthMiddleware and return 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/activities/previous", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request to /api/v1/activities/previous, got %d", w.Code)
	}
}

func TestRegisterRoutes_SwaggerGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("swagger disabled returns 404", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			SwaggerEnabled: false,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 when swagger is disabled, got %d", w.Code)
		}
	})

	t.Run("swagger enabled mounts handler", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			SwaggerEnabled: true,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 when swagger is enabled, got %d", w.Code)
		}
	})
}

func TestRegisterRoutes_DownloadsRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("rate limit applies to timesheet downloads", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			RateLimitEnabled:  true,
			RateLimitRequests: 1,
			RateLimitWindow:   1 * time.Minute,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		// First request passes through rate limiter (returns 400 since token is invalid or job repo is nil)
		req1 := httptest.NewRequest(http.MethodGet, "/api/v1/timesheet/downloads/token123", nil)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		if w1.Code == http.StatusTooManyRequests {
			t.Errorf("first download request should not be rate limited")
		}

		// Second request should be blocked by rate limiter (429)
		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/timesheet/downloads/token123", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusTooManyRequests {
			t.Errorf("expected second download request to be 429 Too Many Requests, got %d", w2.Code)
		}
	})
}
