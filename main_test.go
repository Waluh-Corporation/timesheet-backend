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

func TestRegisterRoutes_RateLimitConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("rate limiting disabled", func(t *testing.T) {
		r := gin.New()
		cfg := &config.Config{
			RateLimitEnabled: false,
		}
		srv := &handlers.Server{Cfg: cfg}
		registerRoutes(r, srv)

		// Calling /api/v1/auth/login multiple times should not return 429
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == http.StatusTooManyRequests {
				t.Errorf("expected route not to be rate limited, got 429 on iteration %d", i)
			}
		}
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

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == http.StatusTooManyRequests {
				t.Errorf("expected route not to be rate limited when requests=0, got 429 on iteration %d", i)
			}
		}
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

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == http.StatusTooManyRequests {
				t.Errorf("expected route not to be rate limited when window=0, got 429 on iteration %d", i)
			}
		}
	})

	t.Run("nil server or nil config defaults to rate limiting disabled", func(t *testing.T) {
		r := gin.New()
		registerRoutes(r, nil)

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == http.StatusTooManyRequests {
				t.Errorf("expected route not to be rate limited with nil server, got 429 on iteration %d", i)
			}
		}
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
