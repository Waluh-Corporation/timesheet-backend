package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "timesheet-backend/docs"
	"timesheet-backend/handlers"
)

func setupSwaggerRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	return r
}

func TestSwaggerRedirect(t *testing.T) {
	r := setupSwaggerRouter()
	req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected redirect 301, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/swagger/index.html" {
		t.Fatalf("expected redirect to /swagger/index.html, got %s", loc)
	}
}

func TestSwaggerIndexUI(t *testing.T) {
	r := setupSwaggerRouter()
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if body := w.Body.String(); len(body) == 0 {
		t.Fatal("expected swagger HTML content, got empty body")
	}
}

func TestSwaggerDocJSON(t *testing.T) {
	r := setupSwaggerRouter()
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse swagger doc.json: %v", err)
	}

	info, ok := doc["info"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'info' object in swagger doc.json")
	}

	if title, _ := info["title"].(string); title != "Timesheet Automation Portal API" {
		t.Fatalf("expected title 'Timesheet Automation Portal API', got %s", title)
	}

	// Verify all API paths in Swagger are versioned under /api/v1 and old /api/ routes are gone
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'paths' in swagger doc.json")
	}

	foundV1 := false
	for path := range paths {
		if path == "/.well-known/webauthn" {
			continue
		}
		if len(path) >= 5 && path[:5] == "/api/" && (len(path) < 8 || path[:8] != "/api/v1/") {
			t.Errorf("unversioned API route still exists in swagger doc: %s", path)
		}
		if len(path) >= 8 && path[:8] == "/api/v1/" {
			foundV1 = true
		}
	}

	if !foundV1 {
		t.Fatal("expected /api/v1/* routes in swagger doc.json, but none found")
	}
}

func TestAPIRouteVersioningTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := &handlers.Server{}
	registerRoutes(r, srv)

	routes := r.Routes()
	if len(routes) == 0 {
		t.Fatal("expected registered routes, got 0")
	}

	foundV1 := false
	for _, route := range routes {
		if route.Path == "/.well-known/webauthn" || strings.HasPrefix(route.Path, "/swagger") {
			continue
		}
		// If route starts with /api/, it MUST have /api/v1/ prefix
		if strings.HasPrefix(route.Path, "/api/") && !strings.HasPrefix(route.Path, "/api/v1/") {
			t.Errorf("found unversioned API route in gin engine: %s %s", route.Method, route.Path)
		}
		if strings.HasPrefix(route.Path, "/api/v1/") {
			foundV1 = true
		}
		// Verify templates routes are removed
		if strings.Contains(route.Path, "templates") {
			t.Errorf("templates route should be removed, but found: %s %s", route.Method, route.Path)
		}
	}

	if !foundV1 {
		t.Fatal("expected /api/v1/* routes in gin engine, but none found")
	}

	// Verify that calling old unversioned route returns 404
	req := httptest.NewRequest(http.MethodGet, "/api/push/vapid-public-key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for unversioned route /api/push/vapid-public-key, got %d", w.Code)
	}

	// Verify that calling deleted template route returns 404
	reqTmpl := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	wTmpl := httptest.NewRecorder()
	r.ServeHTTP(wTmpl, reqTmpl)

	if wTmpl.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for deleted route /api/v1/templates, got %d", wTmpl.Code)
	}
}

func TestSPAHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	indexFile := tempDir + "/index.html"
	if err := os.WriteFile(indexFile, []byte("<html>Root Index</html>"), 0644); err != nil {
		t.Fatalf("failed to write root index.html: %v", err)
	}

	loginFile := tempDir + "/login.html"
	if err := os.WriteFile(loginFile, []byte("<html>Login Page</html>"), 0644); err != nil {
		t.Fatalf("failed to write login.html: %v", err)
	}

	r := gin.New()
	r.NoRoute(spaHandler(tempDir))

	// 1. Unmatched /api/* returns JSON 404
	reqAPI := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	wAPI := httptest.NewRecorder()
	r.ServeHTTP(wAPI, reqAPI)
	if wAPI.Code != http.StatusNotFound {
		t.Errorf("expected 404 for /api/ route, got %d", wAPI.Code)
	}

	// 2. Unmatched /swagger/* returns JSON 404
	reqSwag := httptest.NewRequest(http.MethodGet, "/swagger/notfound", nil)
	wSwag := httptest.NewRecorder()
	r.ServeHTTP(wSwag, reqSwag)
	if wSwag.Code != http.StatusNotFound {
		t.Errorf("expected 404 for /swagger/ route, got %d", wSwag.Code)
	}

	// 3. Exact root / match
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	wRoot := httptest.NewRecorder()
	r.ServeHTTP(wRoot, reqRoot)
	if wRoot.Code != http.StatusOK {
		t.Errorf("expected 200 for /, got %d", wRoot.Code)
	}

	// 4. File extension match (/login -> login.html)
	reqLogin := httptest.NewRequest(http.MethodGet, "/login", nil)
	wLogin := httptest.NewRecorder()
	r.ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Errorf("expected 200 for /login, got %d", wLogin.Code)
	}

	// 5. Fallback route (/client/subpath)
	reqFallback := httptest.NewRequest(http.MethodGet, "/client/subpath", nil)
	wFallback := httptest.NewRecorder()
	r.ServeHTTP(wFallback, reqFallback)
	if wFallback.Code != http.StatusOK {
		t.Errorf("expected 200 for fallback, got %d", wFallback.Code)
	}

	// 6. Directory traversal attempt
	reqTraversal := httptest.NewRequest(http.MethodGet, "/../../../../etc/passwd", nil)
	wTraversal := httptest.NewRecorder()
	r.ServeHTTP(wTraversal, reqTraversal)
	if wTraversal.Code != http.StatusOK && wTraversal.Code != http.StatusNotFound {
		t.Errorf("unexpected status code for traversal: %d", wTraversal.Code)
	}

	// 7. Empty static directory returns 404
	emptyDir := t.TempDir()
	rEmpty := gin.New()
	rEmpty.NoRoute(spaHandler(emptyDir))
	reqEmpty := httptest.NewRequest(http.MethodGet, "/something", nil)
	wEmpty := httptest.NewRecorder()
	rEmpty.ServeHTTP(wEmpty, reqEmpty)
	if wEmpty.Code != http.StatusNotFound {
		t.Errorf("expected 404 when no index.html exists, got %d", wEmpty.Code)
	}
}
