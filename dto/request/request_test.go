package request

import (
	"encoding/json"
	"testing"

	"timesheet-backend/models"
)

func TestRequestDTOs(t *testing.T) {
	loginReq := LoginRequest{Identifier: "admin", Password: "pwd"}
	if loginReq.Identifier != "admin" {
		t.Errorf("LoginRequest mismatch: %+v", loginReq)
	}

	forgotReq := ForgotRequest{Email: "admin@example.com"}
	if forgotReq.Email != "admin@example.com" {
		t.Errorf("ForgotRequest mismatch: %+v", forgotReq)
	}

	resetReq := ResetRequest{Token: "tok", Password: "pwd"}
	if resetReq.Token != "tok" {
		t.Errorf("ResetRequest mismatch: %+v", resetReq)
	}

	passkeyReq := BeginPasskeyLoginRequest{Identifier: "user1"}
	if passkeyReq.Identifier != "user1" {
		t.Errorf("BeginPasskeyLoginRequest mismatch: %+v", passkeyReq)
	}

	userReq := CreateUserRequest{Username: "newuser", Email: "new@example.com", Role: models.RoleUser}
	if userReq.Username != "newuser" {
		t.Errorf("CreateUserRequest mismatch: %+v", userReq)
	}

	updateReq := UpdateUserRequest{}
	if updateReq.Role != nil {
		t.Errorf("UpdateUserRequest mismatch: %+v", updateReq)
	}

	profileReq := ProfileChangeRequestDTO{Name: "Name", EmployeeID: "EMP01"}
	if profileReq.EmployeeID != "EMP01" {
		t.Errorf("ProfileChangeRequestDTO mismatch: %+v", profileReq)
	}

	actReq := DailyActivityRequest{Date: "2026-09-01", Status: "P"}
	if actReq.Date != "2026-09-01" {
		t.Errorf("DailyActivityRequest mismatch: %+v", actReq)
	}

	genReq := GenerateRequest{Month: 9, Year: 2026}
	if genReq.Month != 9 {
		t.Errorf("GenerateRequest mismatch: %+v", genReq)
	}

	pushKey := PushKeyPayload{P256dh: "key", Auth: "auth"}
	if pushKey.P256dh != "key" {
		t.Errorf("PushKeyPayload mismatch: %+v", pushKey)
	}

	subReq := SubscribeRequest{Endpoint: "ep"}
	if subReq.Endpoint != "ep" {
		t.Errorf("SubscribeRequest mismatch: %+v", subReq)
	}

	unsubReq := UnsubscribeRequest{Endpoint: "ep"}
	if unsubReq.Endpoint != "ep" {
		t.Errorf("UnsubscribeRequest mismatch: %+v", unsubReq)
	}

	updatePkReq := UpdatePasskeyRequest{Name: "My Passkey"}
	if updatePkReq.Name != "My Passkey" {
		t.Errorf("UpdatePasskeyRequest mismatch: %+v", updatePkReq)
	}

	verifyTokenReq := VerifyResetTokenRequest{Token: "test-token"}
	if verifyTokenReq.Token != "test-token" {
		t.Errorf("VerifyResetTokenRequest mismatch: %+v", verifyTokenReq)
	}
}

func TestEmployeeID(t *testing.T) {
	t.Run("CreateUserRequest with employee_id", func(t *testing.T) {
		var req CreateUserRequest
		payload := `{"username":"test","email":"test@example.com","role":"user","employee_id":"EMP-123"}`
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if req.EmployeeID != "EMP-123" {
			t.Errorf("expected EMP-123, got %s", req.EmployeeID)
		}
	})

	t.Run("UpdateUserRequest with employee_id", func(t *testing.T) {
		var req UpdateUserRequest
		payload := `{"employee_id":"EMP-999"}`
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if req.EmployeeID == nil || *req.EmployeeID != "EMP-999" {
			t.Errorf("expected EMP-999, got %v", req.EmployeeID)
		}
	})

	t.Run("ProfileChangeRequestDTO with employee_id", func(t *testing.T) {
		var req ProfileChangeRequestDTO
		payload := `{"name":"John","employee_id":"EMP-888"}`
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if req.EmployeeID != "EMP-888" {
			t.Errorf("expected EMP-888, got %s", req.EmployeeID)
		}
	})
}
