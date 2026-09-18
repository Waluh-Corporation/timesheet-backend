package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/dto/response"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
)

func TestRegistration_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{}

	t.Run("Register rejects empty body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Register(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("Register rejects weak password", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		payload := `{"username":"newuser","email":"newuser@example.com","password":"123","name":"New User"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(payload)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Register(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("Register rejects password containing username", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		payload := `{"username":"mysecretuser","email":"user@example.com","password":"mysecretuser123","name":"New User"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(payload)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Register(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("ReviewRegistration rejects invalid ID", func(t *testing.T) {
		for _, invID := range []string{"0", "abc", "-1"} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: invID}}
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/registrations/"+invID+"/review?action=approve", nil)

			srv.ReviewRegistration(c)
			assertResponseCode(t, w, http.StatusBadRequest)
		}
	})

	t.Run("ReviewRegistration rejects invalid action", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "10"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/registrations/10/review?action=invalid_action", nil)

		srv.ReviewRegistration(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})

	t.Run("GetRegistration rejects invalid ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/registrations/invalid", nil)

		srv.GetRegistration(c)
		assertResponseCode(t, w, http.StatusBadRequest)
	})
}

func TestRegistration_DatabaseWorkflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", cfg.JWTExpiry)
	srv := &Server{
		DB:     tx,
		Cfg:    cfg,
		Auth:   authSvc,
		Mailer: mailer.New(cfg),
	}

	adminUser := models.User{
		Username: "admin_reg_flow",
		Email:    "admin_reg_flow@example.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}
	if err := tx.Create(&adminUser).Error; err != nil {
		t.Fatalf("failed to create test admin: %v", err)
	}

	comp := models.Company{Code: "COMP_REG", Name: "PT Company Reg", IsActive: true}
	_ = tx.Create(&comp)

	dept := models.Department{Name: "Core Tech", Division: "Tech Delivery", IsActive: true}
	_ = tx.Create(&dept)

	var regID uint
	var registeredUserID uint

	// 1. Self-register successfully
	t.Run("Self registration creates inactive user and pending registration", func(t *testing.T) {
		payload := `{
			"username": "self_register_user",
			"email": "self_reg@example.com",
			"password": "SecurePassword123!",
			"name": "Self Register User",
			"bni_id": "88889999",
			"employee_id": "EMP-9999",
			"company": "COMP_REG",
			"department": "Core Tech",
			"position": "Software Engineer",
			"group_name": "SDD"
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(payload)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Register(c)
		assertFatalCode(t, w, http.StatusCreated)

		var res response.RegisterResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if res.Data.RegistrationID == 0 || res.Data.UserID == 0 {
			t.Fatalf("expected valid registration ID and user ID, got reg=%d, user=%d", res.Data.RegistrationID, res.Data.UserID)
		}
		regID = res.Data.RegistrationID
		registeredUserID = res.Data.UserID

		// Verify database state: user must be inactive
		var u models.User
		if err := tx.First(&u, registeredUserID).Error; err != nil {
			t.Fatalf("failed to find registered user: %v", err)
		}
		if u.IsActive {
			t.Errorf("expected registered user to be inactive pending approval, got active")
		}
		if u.Role != models.RoleUser {
			t.Errorf("expected role user, got %s", u.Role)
		}
		if u.BniID != "88889999" || u.EmployeeID != "EMP-9999" {
			t.Errorf("profile fields not persisted correctly: %+v", u)
		}

		// Verify registration record
		var reg models.UserRegistration
		if err := tx.First(&reg, regID).Error; err != nil {
			t.Fatalf("failed to find registration record: %v", err)
		}
		if reg.Status != models.RegistrationPending {
			t.Errorf("expected status pending, got %s", reg.Status)
		}
		if reg.UserID != u.ID {
			t.Errorf("expected registration user_id %d, got %d", u.ID, reg.UserID)
		}
	})

	// 2. Reject duplicate username/email
	t.Run("Self registration rejects duplicate username and email", func(t *testing.T) {
		// Duplicate username
		payloadDupUser := `{
			"username": "self_register_user",
			"email": "different_email@example.com",
			"password": "SecurePassword123!",
			"name": "Another User"
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(payloadDupUser)))
		c.Request.Header.Set("Content-Type", "application/json")
		srv.Register(c)
		assertResponseCode(t, w, http.StatusConflict)

		// Duplicate email
		payloadDupEmail := `{
			"username": "different_username",
			"email": "self_reg@example.com",
			"password": "SecurePassword123!",
			"name": "Another User"
		}`
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(payloadDupEmail)))
		c2.Request.Header.Set("Content-Type", "application/json")
		srv.Register(c2)
		assertResponseCode(t, w2, http.StatusConflict)
	})

	// 3. User attempts login before approval -> 403 Forbidden with specific message
	t.Run("Login returns forbidden with pending approval message", func(t *testing.T) {
		loginPayload := `{"identifier":"self_register_user","password":"SecurePassword123!"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(loginPayload)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusForbidden)

		var errResp response.ErrorResponse
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		expectedSubstr := "pending administrator approval"
		if !bytes.Contains(w.Body.Bytes(), []byte(expectedSubstr)) {
			t.Errorf("expected login error to contain %q, got: %s", expectedSubstr, w.Body.String())
		}
	})

	// 4. Admin lists registrations
	t.Run("Admin ListRegistrations returns pending registration", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/registrations?status=pending", nil)
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.ListRegistrations(c)
		assertResponseCode(t, w, http.StatusOK)

		var listResp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &listResp)
		data, ok := listResp["data"].([]interface{})
		if !ok || len(data) == 0 {
			t.Fatalf("expected list of registrations, got: %v", listResp)
		}
	})

	// 5. Admin gets registration details
	t.Run("Admin GetRegistration returns details", func(t *testing.T) {
		regIDStr := strconv.FormatUint(uint64(regID), 10)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: regIDStr}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/registrations/"+regIDStr, nil)
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.GetRegistration(c)
		assertResponseCode(t, w, http.StatusOK)
	})

	// 6. Admin approves registration -> activates user
	t.Run("Admin approves registration and activates user", func(t *testing.T) {
		regIDStr := strconv.FormatUint(uint64(regID), 10)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: regIDStr}}
		body := `{"action":"approve","admin_notes":"All data verified by HR"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/registrations/"+regIDStr+"/review", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.ReviewRegistration(c)
		assertResponseCode(t, w, http.StatusOK)

		// Verify database state: user is now active!
		var u models.User
		if err := tx.First(&u, registeredUserID).Error; err != nil {
			t.Fatalf("failed to query user: %v", err)
		}
		if !u.IsActive {
			t.Errorf("expected user to be active after approval, got is_active=false")
		}

		// Registration status is approved
		var reg models.UserRegistration
		if err := tx.First(&reg, regID).Error; err != nil {
			t.Fatalf("failed to query registration: %v", err)
		}
		if reg.Status != models.RegistrationApproved {
			t.Errorf("expected status approved, got %s", reg.Status)
		}
		if reg.ReviewedBy == nil || *reg.ReviewedBy != adminUser.ID {
			t.Errorf("expected reviewer ID %d, got %v", adminUser.ID, reg.ReviewedBy)
		}
	})

	// 7. User logs in successfully now that they are approved
	t.Run("Approved user logs in successfully", func(t *testing.T) {
		loginPayload := `{"identifier":"self_register_user","password":"SecurePassword123!"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(loginPayload)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Login(c)
		assertResponseCode(t, w, http.StatusOK)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp["data"].(map[string]interface{})
		token, _ := data["token"].(string)
		if token == "" {
			t.Errorf("expected JWT token, got empty")
		}
	})

	// 8. Re-reviewing already reviewed registration -> 409 Conflict
	t.Run("Re-reviewing already reviewed registration yields 409", func(t *testing.T) {
		regIDStr := strconv.FormatUint(uint64(regID), 10)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: regIDStr}}
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/registrations/"+regIDStr+"/review?action=approve", nil)
		c.Set(ctxUserID, adminUser.ID)
		c.Set(ctxRole, models.RoleAdmin)

		srv.ReviewRegistration(c)
		assertResponseCode(t, w, http.StatusConflict)
	})

	// 9. Register another user and test rejection flow
	t.Run("Reject registration keeps user inactive and reflects in login", func(t *testing.T) {
		payload := `{
			"username": "rejected_user",
			"email": "rejected@example.com",
			"password": "SecurePassword123!",
			"name": "Rejected User"
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(payload)))
		c.Request.Header.Set("Content-Type", "application/json")

		srv.Register(c)
		assertFatalCode(t, w, http.StatusCreated)

		var res response.RegisterResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		rejRegID := res.Data.RegistrationID

		// Admin rejects
		wRej := httptest.NewRecorder()
		cRej, _ := gin.CreateTestContext(wRej)
		idStr := strconv.FormatUint(uint64(rejRegID), 10)
		cRej.Params = gin.Params{{Key: "id", Value: idStr}}
		body := `{"action":"reject","admin_notes":"Identity verification failed"}`
		cRej.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/registrations/"+idStr+"/review", bytes.NewReader([]byte(body)))
		cRej.Request.Header.Set("Content-Type", "application/json")
		cRej.Set(ctxUserID, adminUser.ID)
		cRej.Set(ctxRole, models.RoleAdmin)

		srv.ReviewRegistration(cRej)
		assertResponseCode(t, wRej, http.StatusOK)

		// Login attempt should show rejected
		loginPayload := `{"identifier":"rejected_user","password":"SecurePassword123!"}`
		wLog := httptest.NewRecorder()
		cLog, _ := gin.CreateTestContext(wLog)
		cLog.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(loginPayload)))
		cLog.Request.Header.Set("Content-Type", "application/json")

		srv.Login(cLog)
		assertResponseCode(t, wLog, http.StatusForbidden)
		if !bytes.Contains(wLog.Body.Bytes(), []byte("rejected")) {
			t.Errorf("expected login error to contain 'rejected', got: %s", wLog.Body.String())
		}
	})
}
