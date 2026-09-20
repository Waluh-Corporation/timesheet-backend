package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

func TestAdminAuthenticators_Endpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)

	srv, err := NewServer(db, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Seed a test authenticator
	testAAGUID := "fa264024-4a24-4e2b-a489-3224b1263d90"
	_ = db.Where("aaguid = ?", testAAGUID).Delete(&models.AuthenticatorAAGUID{})
	testAuth := models.AuthenticatorAAGUID{
		AAGUID: testAAGUID,
		Name:   "Test Authenticator Pro",
		Icon:   "data:image/svg+xml;base64,bGlnaHQ=",
	}
	if err := db.Create(&testAuth).Error; err != nil {
		t.Fatalf("failed to create test authenticator: %v", err)
	}
	defer func() {
		_ = db.Where("aaguid = ?", testAAGUID).Delete(&models.AuthenticatorAAGUID{})
	}()

	t.Run("AdminListAuthenticators returns list and search works", func(t *testing.T) {
		// 1. General list
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/authenticators?limit=10", nil)
		srv.AdminListAuthenticators(c)

		assertResponseCode(t, w, http.StatusOK)
		var envelope struct {
			Data response.AuthenticatorListResponse `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if envelope.Data.Total < 1 {
			t.Errorf("expected at least 1 authenticator, got %d", envelope.Data.Total)
		}

		// 2. Search query match
		wSearch := httptest.NewRecorder()
		cSearch, _ := gin.CreateTestContext(wSearch)
		cSearch.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/authenticators?search=Test%20Authenticator%20Pro", nil)
		srv.AdminListAuthenticators(cSearch)

		assertResponseCode(t, wSearch, http.StatusOK)
		var searchEnv struct {
			Data response.AuthenticatorListResponse `json:"data"`
		}
		_ = json.Unmarshal(wSearch.Body.Bytes(), &searchEnv)
		if len(searchEnv.Data.Authenticators) != 1 {
			t.Errorf("expected exactly 1 result for specific search, got %d", len(searchEnv.Data.Authenticators))
		}
		if searchEnv.Data.Authenticators[0].AAGUID != testAAGUID {
			t.Errorf("expected AAGUID %s, got %s", testAAGUID, searchEnv.Data.Authenticators[0].AAGUID)
		}

		// 3. Search query no match
		wNoMatch := httptest.NewRecorder()
		cNoMatch, _ := gin.CreateTestContext(wNoMatch)
		cNoMatch.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/authenticators?search=NonExistentDevice999xyz", nil)
		srv.AdminListAuthenticators(cNoMatch)

		assertResponseCode(t, wNoMatch, http.StatusOK)
		var noMatchEnv struct {
			Data response.AuthenticatorListResponse `json:"data"`
		}
		_ = json.Unmarshal(wNoMatch.Body.Bytes(), &noMatchEnv)
		if len(noMatchEnv.Data.Authenticators) != 0 {
			t.Errorf("expected 0 results, got %d", len(noMatchEnv.Data.Authenticators))
		}

		// 4. Edge cases for page and limit bounds
		wBounds := httptest.NewRecorder()
		cBounds, _ := gin.CreateTestContext(wBounds)
		cBounds.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/authenticators?page=-5&limit=-10", nil)
		srv.AdminListAuthenticators(cBounds)
		assertResponseCode(t, wBounds, http.StatusOK)

		wLimitMax := httptest.NewRecorder()
		cLimitMax, _ := gin.CreateTestContext(wLimitMax)
		cLimitMax.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/authenticators?limit=999", nil)
		srv.AdminListAuthenticators(cLimitMax)
		assertResponseCode(t, wLimitMax, http.StatusOK)
	})

	t.Run("SyncCommunityAuthenticators success with mock server", func(t *testing.T) {
		mockAAGUID := "89d70fb5-1b03-4c9f-8ec0-7f9999999999"
		mockData := map[string]services.CommunityAAGUIDEntry{
			mockAAGUID: {
				Name: "Mock Community Passkey",
				Icon: "data:image/svg+xml;base64,bW9jaw==",
			},
		}

		mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockData)
		}))
		defer mockSrv.Close()

		origURL := services.CommunityAAGUIDURL
		services.CommunityAAGUIDURL = mockSrv.URL
		defer func() {
			services.CommunityAAGUIDURL = origURL
			_ = db.Where("aaguid = ?", mockAAGUID).Delete(&models.AuthenticatorAAGUID{})
		}()

		wSync := httptest.NewRecorder()
		cSync, _ := gin.CreateTestContext(wSync)
		cSync.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/authenticators/sync", nil)
		srv.SyncCommunityAuthenticators(cSync)

		assertResponseCode(t, wSync, http.StatusOK)
		var syncEnv struct {
			Data response.AuthenticatorSyncResponse `json:"data"`
		}
		if err := json.Unmarshal(wSync.Body.Bytes(), &syncEnv); err != nil {
			t.Fatalf("failed to decode sync response: %v", err)
		}
		if syncEnv.Data.TotalSynced != 1 {
			t.Errorf("expected 1 synced item, got %d", syncEnv.Data.TotalSynced)
		}

		// Check DB has the mock item
		var found models.AuthenticatorAAGUID
		if err := db.Where("aaguid = ?", mockAAGUID).First(&found).Error; err != nil {
			t.Fatalf("expected item %s in database: %v", mockAAGUID, err)
		}
		if found.Name != "Mock Community Passkey" {
			t.Errorf("expected name 'Mock Community Passkey', got '%s'", found.Name)
		}
	})

	t.Run("SyncCommunityAuthenticators returns 502 on external failure", func(t *testing.T) {
		mockErrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer mockErrSrv.Close()

		origURL := services.CommunityAAGUIDURL
		services.CommunityAAGUIDURL = mockErrSrv.URL
		defer func() { services.CommunityAAGUIDURL = origURL }()

		wErr := httptest.NewRecorder()
		cErr, _ := gin.CreateTestContext(wErr)
		cErr.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/authenticators/sync", nil)
		srv.SyncCommunityAuthenticators(cErr)

		assertResponseCode(t, wErr, http.StatusBadGateway)
	})

	t.Run("Nil DB returns 500", func(t *testing.T) {
		nilSrv := &Server{}

		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/authenticators", nil)
		nilSrv.AdminListAuthenticators(c1)
		assertResponseCode(t, w1, http.StatusInternalServerError)

		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/authenticators/sync", nil)
		nilSrv.SyncCommunityAuthenticators(c2)
		assertResponseCode(t, w2, http.StatusInternalServerError)
	})
}
