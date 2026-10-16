package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestBeritaAcaraHandlers_ListWitnesses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	srv := newTestServer(t, tx, cfg)
	userRepo := repository.NewUserRepository(tx)

	// Seed caller user
	caller := &models.User{
		Username:     "ba_caller",
		Email:        "ba_caller@example.com",
		Name:         "BA Caller",
		Role:         models.RoleUser,
		PasswordHash: "secret",
		IsActive:     true,
	}
	_ = userRepo.Create(context.Background(), caller)

	// Seed eligible witness 1 (user role)
	witness1 := &models.User{
		Username:     "ba_witness1",
		Email:        "ba_witness1@example.com",
		Name:         "Witness One",
		Role:         models.RoleUser,
		PasswordHash: "secret",
		IsActive:     true,
	}
	_ = userRepo.Create(context.Background(), witness1)

	// Seed eligible witness 2 (user role)
	witness2 := &models.User{
		Username:     "ba_witness2",
		Email:        "ba_witness2@example.com",
		Name:         "Witness Two",
		Role:         models.RoleUser,
		PasswordHash: "secret",
		IsActive:     true,
	}
	_ = userRepo.Create(context.Background(), witness2)

	// Seed admin user (must be excluded)
	adminUser := &models.User{
		Username:     "ba_admin",
		Email:        "ba_admin@example.com",
		Name:         "Admin Person",
		Role:         models.RoleAdmin,
		PasswordHash: "secret",
		IsActive:     true,
	}
	_ = userRepo.Create(context.Background(), adminUser)

	// Test 1: List all witnesses (should return witness1 and witness2, exclude caller and admin)
	{
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, caller.ID)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/witnesses", nil)

		srv.ListWitnesses(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp struct {
			Data []response.WitnessResponse `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		foundCaller := false
		foundAdmin := false
		foundW1 := false
		foundW2 := false

		for _, w := range resp.Data {
			if w.ID == caller.ID {
				foundCaller = true
			}
			if w.ID == adminUser.ID {
				foundAdmin = true
			}
			if w.ID == witness1.ID {
				foundW1 = true
			}
			if w.ID == witness2.ID {
				foundW2 = true
			}
		}

		if foundCaller {
			t.Errorf("caller should not be in witnesses list")
		}
		if foundAdmin {
			t.Errorf("admin user should not be in witnesses list")
		}
		if !foundW1 || !foundW2 {
			t.Errorf("expected both witness1 and witness2 in list, got %+v", resp.Data)
		}
	}

	// Test 2: Search by query filter
	{
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, caller.ID)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara/witnesses?q=One", nil)

		srv.ListWitnesses(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp struct {
			Data []response.WitnessResponse `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if len(resp.Data) != 1 || resp.Data[0].ID != witness1.ID {
			t.Errorf("expected only witness1 for query 'One', got %+v", resp.Data)
		}
	}
}
