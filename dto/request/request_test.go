package request

import (
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
}
