package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

func TestProfileChange_NotesAndEmailFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := setupTestDB(t)

	srv := &Server{DB: db}

	// Clean up any test records
	db.Exec("DELETE FROM profile_change_requests WHERE notes LIKE '%TestNotesEmail%'")
	db.Exec("DELETE FROM users WHERE username IN ('test_user_notes_1', 'test_user_notes_2', 'test_admin_notes')")

	adminUser := models.User{
		Username:     "test_admin_notes",
		Email:        "admin_notes@example.com",
		Role:         models.RoleAdmin,
		PasswordHash: "hashedpass",
		Name:         "Admin Notes",
		IsActive:     true,
	}
	if err := db.Create(&adminUser).Error; err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	user1 := models.User{
		Username:     "test_user_notes_1",
		Email:        "user1_initial@example.com",
		Role:         models.RoleUser,
		PasswordHash: "hashedpass",
		Name:         "User 1",
		IsActive:     true,
	}
	if err := db.Create(&user1).Error; err != nil {
		t.Fatalf("failed to create user1: %v", err)
	}

	user2 := models.User{
		Username:     "test_user_notes_2",
		Email:        "user2_initial@example.com",
		Role:         models.RoleUser,
		PasswordHash: "hashedpass",
		Name:         "User 2",
		IsActive:     true,
	}
	if err := db.Create(&user2).Error; err != nil {
		t.Fatalf("failed to create user2: %v", err)
	}

	t.Cleanup(func() {
		db.Exec("DELETE FROM profile_change_requests WHERE user_id IN (?, ?)", user1.ID, user2.ID)
		db.Exec("DELETE FROM users WHERE id IN (?, ?, ?)", user1.ID, user2.ID, adminUser.ID)
	})

	t.Run("SubmitProfileChange with Email and Notes", func(t *testing.T) {
		body := `{"name": "User 1 Updated", "email": "user1_new@example.com", "notes": "TestNotesEmail: Requesting new email and name update"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user1.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/change", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.SubmitProfileChange(c)
		assertFatalCode(t, w, http.StatusCreated)

		var req models.ProfileChangeRequest
		if err := db.Where("user_id = ? AND notes LIKE '%TestNotesEmail%'", user1.ID).Order("id desc").First(&req).Error; err != nil {
			t.Fatalf("expected to find profile change request: %v", err)
		}
		if req.Email != "user1_new@example.com" {
			t.Errorf("expected Email 'user1_new@example.com', got %s", req.Email)
		}
		if req.Notes != "TestNotesEmail: Requesting new email and name update" {
			t.Errorf("expected Notes to match, got %s", req.Notes)
		}

		// Verify MyProfileChanges exposes email and notes
		wGet := httptest.NewRecorder()
		cGet, _ := gin.CreateTestContext(wGet)
		cGet.Set(ctxUserID, user1.ID)
		cGet.Request = httptest.NewRequest(http.MethodGet, "/api/v1/profile/changes", nil)

		srv.MyProfileChanges(cGet)
		assertFatalCode(t, wGet, http.StatusOK)

		var listResp struct {
			Code   int                              `json:"code"`
			Status string                           `json:"status"`
			Data   []response.ProfileChangeResponse `json:"data"`
		}
		if err := json.Unmarshal(wGet.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("failed to decode profile changes: %v", err)
		}
		if len(listResp.Data) == 0 {
			t.Fatalf("expected profile changes in response, got 0")
		}
		if listResp.Data[0].Email != "user1_new@example.com" {
			t.Errorf("expected Email in response 'user1_new@example.com', got %s", listResp.Data[0].Email)
		}
		if listResp.Data[0].Notes != "TestNotesEmail: Requesting new email and name update" {
			t.Errorf("expected Notes in response, got %s", listResp.Data[0].Notes)
		}

		// Verify ListProfileChanges (Admin) exposes email and notes
		wAdminList := httptest.NewRecorder()
		cAdminList, _ := gin.CreateTestContext(wAdminList)
		cAdminList.Set(ctxUserID, adminUser.ID)
		cAdminList.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/profile-changes", nil)

		srv.ListProfileChanges(cAdminList)
		assertFatalCode(t, wAdminList, http.StatusOK)

		var adminListResp struct {
			Code   int                                   `json:"code"`
			Status string                                `json:"status"`
			Data   []response.AdminProfileChangeResponse `json:"data"`
		}
		if err := json.Unmarshal(wAdminList.Body.Bytes(), &adminListResp); err != nil {
			t.Fatalf("failed to decode admin profile changes: %v", err)
		}
		var found bool
		for _, item := range adminListResp.Data {
			if item.ID == req.ID {
				found = true
				if item.Email != "user1_new@example.com" {
					t.Errorf("expected Admin list email 'user1_new@example.com', got %s", item.Email)
				}
				if item.Notes != "TestNotesEmail: Requesting new email and name update" {
					t.Errorf("expected Admin list notes to match, got %s", item.Notes)
				}
			}
		}
		if !found {
			t.Errorf("expected to find request %d in admin list", req.ID)
		}

		// Approve request and verify live email is updated
		reqIDStr := fmt.Sprintf("%d", req.ID)
		wApprove := httptest.NewRecorder()
		cApprove, _ := gin.CreateTestContext(wApprove)
		cApprove.Set(ctxUserID, adminUser.ID)
		cApprove.Params = gin.Params{{Key: "id", Value: reqIDStr}}
		cApprove.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/profile-changes/"+reqIDStr+"/review?action=approve", nil)

		srv.ReviewProfileChange(cApprove)
		assertFatalCode(t, wApprove, http.StatusOK)

		var updatedUser models.User
		if err := db.First(&updatedUser, user1.ID).Error; err != nil {
			t.Fatalf("failed to fetch updated user: %v", err)
		}
		if updatedUser.Email != "user1_new@example.com" {
			t.Errorf("expected updated live email 'user1_new@example.com', got %s", updatedUser.Email)
		}
	})

	t.Run("SubmitProfileChange rejects duplicate email", func(t *testing.T) {
		// user1 tries to request email already used by user2
		body := `{"email": "user2_initial@example.com", "notes": "TestNotesEmail: trying duplicate email"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, user1.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/profile/change", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.SubmitProfileChange(c)
		assertFatalCode(t, w, http.StatusBadRequest)
	})

	t.Run("Admin UpdateUser updates email directly", func(t *testing.T) {
		body := `{"email": "user2_admin_updated@example.com"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", user2.ID)}}
		c.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", user2.ID), bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusOK)

		var updated models.User
		if err := db.First(&updated, user2.ID).Error; err != nil {
			t.Fatalf("failed to fetch user2: %v", err)
		}
		if updated.Email != "user2_admin_updated@example.com" {
			t.Errorf("expected email 'user2_admin_updated@example.com', got %s", updated.Email)
		}
	})

	t.Run("Admin UpdateUser rejects duplicate email", func(t *testing.T) {
		// Admin tries to set user2's email to user1's current email
		body := `{"email": "user1_new@example.com"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ctxUserID, adminUser.ID)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", user2.ID)}}
		c.Request = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", user2.ID), bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.UpdateUser(c)
		assertFatalCode(t, w, http.StatusBadRequest)
	})
}
