package response

import (
	"testing"
	"time"
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

	pagResp := PaginatedResponse{
		Code:       200,
		Status:     "success",
		Pagination: PaginationMeta{Page: 1, Limit: 10, TotalRows: 1, TotalPages: 1},
	}
	if pagResp.Pagination.Page != 1 {
		t.Errorf("PaginatedResponse mismatch: %+v", pagResp)
	}
}
