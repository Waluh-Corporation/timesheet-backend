package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"timesheet-backend/config"
	"timesheet-backend/handlers"

	"github.com/gin-gonic/gin"
)

func TestMainHelpers_PathAndFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "main_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(testFile, []byte("<h1>Timesheet</h1>"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	subDir := filepath.Join(tmpDir, "static")
	_ = os.Mkdir(subDir, 0755)

	t.Run("isPathWithinRoot", func(t *testing.T) {
		if !isPathWithinRoot(tmpDir, tmpDir) {
			t.Errorf("expected root itself to be within root")
		}
		if !isPathWithinRoot(tmpDir, testFile) {
			t.Errorf("expected testFile to be within root")
		}
		if isPathWithinRoot(tmpDir, filepath.Join(tmpDir, "..", "escaped")) {
			t.Errorf("expected escaped path not to be within root")
		}
	})

	t.Run("tryFiles", func(t *testing.T) {
		file, ok := tryFiles(filepath.Join(tmpDir, "nonexistent.html"), testFile)
		if !ok || file != testFile {
			t.Errorf("expected to find testFile, got %s, ok=%v", file, ok)
		}

		// candidate that is a directory should be skipped
		_, okDir := tryFiles(subDir)
		if okDir {
			t.Errorf("expected directory not to be returned by tryFiles")
		}

		// non-existent candidates
		_, okNone := tryFiles(filepath.Join(tmpDir, "none1"), filepath.Join(tmpDir, "none2"))
		if okNone {
			t.Errorf("expected false for non-existent files")
		}
	})
}

func TestSPAHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "spa_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexFile := filepath.Join(tmpDir, "index.html")
	_ = os.WriteFile(indexFile, []byte("<html>SPA Root</html>"), 0644)

	loginFile := filepath.Join(tmpDir, "login.html")
	_ = os.WriteFile(loginFile, []byte("<html>Login Page</html>"), 0644)

	r := gin.New()
	r.NoRoute(spaHandler(tmpDir))

	t.Run("returns 404 JSON for unmatched API route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for /api/ route, got %d", w.Code)
		}
	})

	t.Run("returns 404 JSON for unmatched Swagger route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger/notfound", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for /swagger/ route, got %d", w.Code)
		}
	})

	t.Run("serves exact route with .html extension", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for /login, got %d", w.Code)
		}
	})

	t.Run("falls back to root index.html for unknown SPA route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/custom-view", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 with index fallback, got %d", w.Code)
		}
	})
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
