package response

import (
	"testing"
	"time"

	"timesheet-backend/models"
)

func TestResponseDTOs(t *testing.T) {
	apiResp := APIResponse{Code: 200, Status: "success", Message: "ok"}
	if apiResp.Code != 200 {
		t.Errorf("APIResponse mismatch: %+v", apiResp)
	}

	msgResp := MessageResponse{Code: 200, Status: "success", Message: "done"}
	if msgResp.Message != "done" {
		t.Errorf("MessageResponse mismatch: %+v", msgResp)
	}

	errResp := ErrorResponse{Code: 400, Status: "error", Error: "err"}
	if errResp.Error != "err" {
		t.Errorf("ErrorResponse mismatch: %+v", errResp)
	}

	delResp := DeleteResponse{Code: 200, Status: "success", Deleted: true}
	if !delResp.Deleted {
		t.Errorf("DeleteResponse mismatch: %+v", delResp)
	}

	loginResp := LoginResponse{Token: "jwt-token"}
	if loginResp.Token != "jwt-token" {
		t.Errorf("LoginResponse mismatch: %+v", loginResp)
	}

	vapidResp := VAPIDKeyResponse{PublicKey: "vapid-key"}
	if vapidResp.PublicKey != "vapid-key" {
		t.Errorf("VAPIDKeyResponse mismatch: %+v", vapidResp)
	}

	originsResp := OriginsResponse{Origins: []string{"https://example.com"}}
	if len(originsResp.Origins) != 1 {
		t.Errorf("OriginsResponse mismatch: %+v", originsResp)
	}

	sessionResp := PasskeySessionResponse{SessionID: "sess-1"}
	if sessionResp.SessionID != "sess-1" {
		t.Errorf("PasskeySessionResponse mismatch: %+v", sessionResp)
	}

	actDetailResp := DailyActivityDetailResponse{ID: 1, Date: time.Now()}
	if actDetailResp.ID != 1 {
		t.Errorf("DailyActivityDetailResponse mismatch: %+v", actDetailResp)
	}

	actResp := DailyActivityResponse{ID: 1, Activity: "Testing", Status: "WFO"}
	if actResp.Activity != "Testing" || actResp.Status != "WFO" {
		t.Errorf("DailyActivityResponse mismatch: %+v", actResp)
	}

	otResp := OvertimeResponse{ID: 1, TaskDescription: "Fixing bugs", TeamLeaderName: "Leader"}
	if otResp.TaskDescription != "Fixing bugs" || otResp.TeamLeaderName != "Leader" {
		t.Errorf("OvertimeResponse mismatch: %+v", otResp)
	}

	adminUserResp := AdminUserResponse{ID: 1, Username: "admin", Role: models.RoleAdmin}
	if adminUserResp.Username != "admin" || adminUserResp.Role != models.RoleAdmin {
		t.Errorf("AdminUserResponse mismatch: %+v", adminUserResp)
	}

	adminProfResp := AdminProfileChangeResponse{ID: 1, UserName: "user1", Status: models.ProfilePending}
	if adminProfResp.UserName != "user1" || adminProfResp.Status != models.ProfilePending {
		t.Errorf("AdminProfileChangeResponse mismatch: %+v", adminProfResp)
	}

	adminPkResp := AdminPasskeyResponse{ID: 1, FriendlyName: "My Key"}
	if adminPkResp.FriendlyName != "My Key" {
		t.Errorf("AdminPasskeyResponse mismatch: %+v", adminPkResp)
	}

	pagResp := PaginatedResponse{
		Code:       200,
		Status:     "success",
		Pagination: PaginationMeta{Page: 1, Limit: 10, TotalRows: 1, TotalPages: 1},
	}
	if pagResp.Pagination.Page != 1 {
		t.Errorf("PaginatedResponse mismatch: %+v", pagResp)
	}
}

func TestUserResponseDTOs(t *testing.T) {
	u := &models.User{
		Username: "alice",
		Email:    "alice@example.com",
		Role:     models.RoleUser,
		Name:     "Alice Wonderland",
		IsActive: true,
	}
	u.ID = 42

	userResp := ToUserResponse(u)
	if userResp.ID != 42 || userResp.Username != "alice" || userResp.Email != "alice@example.com" {
		t.Fatalf("ToUserResponse mismatch: %+v", userResp)
	}

	createData := CreateUserData{
		Message: "user created",
		User:    userResp,
	}
	createResp := CreateUserResponse{
		Code:   201,
		Status: "success",
		Data:   createData,
	}
	if createResp.Code != 201 || createResp.Data.User.ID != 42 {
		t.Errorf("CreateUserResponse mismatch: %+v", createResp)
	}

	changeResp := ChangePasswordResponse{
		Code:    200,
		Status:  "success",
		Message: "password changed",
	}
	if changeResp.Code != 200 || changeResp.Message != "password changed" {
		t.Errorf("ChangePasswordResponse mismatch: %+v", changeResp)
	}

	updateResp := UpdateUserResponse{
		Code:    200,
		Status:  "success",
		Message: "user updated",
		Data:    &userResp,
	}
	if updateResp.Code != 200 || updateResp.Data == nil || updateResp.Data.Username != "alice" {
		t.Errorf("UpdateUserResponse mismatch: %+v", updateResp)
	}

	submitResp := SubmitProfileChangeResponse{
		Code:    201,
		Status:  "success",
		Message: "profile change request submitted",
		Data:    &ProfileChangeResponse{ID: 10, UserID: 42, Status: models.ProfilePending},
	}
	if submitResp.Code != 201 || submitResp.Data == nil || submitResp.Data.ID != 10 {
		t.Errorf("SubmitProfileChangeResponse mismatch: %+v", submitResp)
	}

	detailResp := UserDetailResponse{
		Code:   200,
		Status: "success",
		Data:   userResp,
	}
	if detailResp.Code != 200 || detailResp.Data.ID != 42 {
		t.Errorf("UserDetailResponse mismatch: %+v", detailResp)
	}

	listResp := UserListResponse{
		Code:   200,
		Status: "success",
		Data:   []UserResponse{userResp},
	}
	if listResp.Code != 200 || len(listResp.Data) != 1 {
		t.Errorf("UserListResponse mismatch: %+v", listResp)
	}
}
