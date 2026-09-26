package models

import (
	"testing"
	"time"
)

func TestEntityMappers_RoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	deptID := uint(10)
	divID := uint(20)
	siteID := uint(30)
	compID := uint(40)
	revID := uint(50)

	t.Run("User Mapping", func(t *testing.T) {
		u := &User{
			ID:           1,
			CreatedAt:    now,
			UpdatedAt:    now,
			Username:     "johndoe",
			Email:        "john@example.com",
			PasswordHash: "hashed",
			Role:         RoleUser,
			IsActive:     true,
			Name:         "John Doe",
			BniID:        "12345",
			EmployeeID:   "EMP001",
			Division:     "IT",
			DivisionID:   &divID,
			Department:   "Engineering",
			DepartmentID: &deptID,
			GroupName:    "Core",
			Position:     "Developer",
			Site:         "HQ",
			SiteID:       &siteID,
			Company:      "MII",
			CompanyID:    &compID,
		}

		dom := u.ToDomain()
		if dom.Username != u.Username || dom.Email != u.Email || string(dom.Role) != string(u.Role) {
			t.Fatalf("expected matching fields, got %+v", dom)
		}
		back := UserFromDomain(dom)
		if back.Username != u.Username || back.Email != u.Email {
			t.Fatalf("roundtrip mismatch: %+v vs %+v", u, back)
		}

		// Nil safety
		var nilUser *User
		if nilUser.ToDomain() != nil {
			t.Error("expected nil for nil user")
		}
		if UserFromDomain(nil) != nil {
			t.Error("expected nil for nil entity")
		}
	})

	t.Run("ProfileChangeRequest Mapping", func(t *testing.T) {
		p := &ProfileChangeRequest{
			ID:           2,
			CreatedAt:    now,
			UpdatedAt:    now,
			UserID:       1,
			Status:       ProfilePending,
			Name:         "New Name",
			BniID:        "54321",
			EmployeeID:   "EMP002",
			Division:     "Operations",
			DivisionID:   &divID,
			Department:   "Logistics",
			DepartmentID: &deptID,
			GroupName:    "OpsGroup",
			Position:     "Lead",
			Site:         "Branch",
			SiteID:       &siteID,
			CompanyID:    &compID,
			Email:        "new@example.com",
			Notes:        "Updated title",
			ReviewedBy:   &revID,
			ReviewedAt:   &now,
		}

		dom := p.ToDomain()
		if dom.Name != p.Name || string(dom.Status) != string(p.Status) {
			t.Fatalf("mismatch in domain: %+v", dom)
		}
		back := ProfileChangeRequestFromDomain(dom)
		if back.Name != p.Name || back.Notes != p.Notes {
			t.Fatalf("roundtrip mismatch: %+v vs %+v", p, back)
		}

		var nilReq *ProfileChangeRequest
		if nilReq.ToDomain() != nil || ProfileChangeRequestFromDomain(nil) != nil {
			t.Error("expected nil on nil inputs")
		}
	})

	t.Run("Master Entities Mapping", func(t *testing.T) {
		c := &Company{ID: 1, Code: "mii", Name: "MII", IsActive: true, CreatedAt: now, UpdatedAt: now}
		if c.ToDomain().Code != "mii" || CompanyFromDomain(c.ToDomain()).Name != "MII" {
			t.Error("Company mapping failed")
		}
		var nilC *Company
		if nilC.ToDomain() != nil || CompanyFromDomain(nil) != nil {
			t.Error("Company nil check failed")
		}

		d := &Department{ID: 2, Code: "DEP", Name: "Dept", Division: "Div", DivisionID: &divID, IsActive: true}
		if d.ToDomain().Name != "Dept" || DepartmentFromDomain(d.ToDomain()).Code != "DEP" {
			t.Error("Department mapping failed")
		}
		var nilD *Department
		if nilD.ToDomain() != nil || DepartmentFromDomain(nil) != nil {
			t.Error("Department nil check failed")
		}

		div := &Division{ID: 3, Code: "DIV", Name: "Division", IsActive: true}
		if div.ToDomain().Name != "Division" || DivisionFromDomain(div.ToDomain()).Code != "DIV" {
			t.Error("Division mapping failed")
		}
		var nilDiv *Division
		if nilDiv.ToDomain() != nil || DivisionFromDomain(nil) != nil {
			t.Error("Division nil check failed")
		}

		s := &Site{ID: 4, Code: "ST", Name: "Site", IsActive: true}
		if s.ToDomain().Name != "Site" || SiteFromDomain(s.ToDomain()).Code != "ST" {
			t.Error("Site mapping failed")
		}
		var nilS *Site
		if nilS.ToDomain() != nil || SiteFromDomain(nil) != nil {
			t.Error("Site nil check failed")
		}

		a := &Approver{ID: 5, Name: "Boss", RoleType: ApproverRoleTeamLeader, Title: "TL", IsActive: true}
		if a.ToDomain().Name != "Boss" || ApproverFromDomain(a.ToDomain()).Title != "TL" {
			t.Error("Approver mapping failed")
		}
		var nilA *Approver
		if nilA.ToDomain() != nil || ApproverFromDomain(nil) != nil {
			t.Error("Approver nil check failed")
		}

		prj := &Project{ID: 6, Code: "PRJ", Name: "Project", IsActive: true}
		if prj.ToDomain().Name != "Project" || ProjectFromDomain(prj.ToDomain()).Code != "PRJ" {
			t.Error("Project mapping failed")
		}
		var nilPrj *Project
		if nilPrj.ToDomain() != nil || ProjectFromDomain(nil) != nil {
			t.Error("Project nil check failed")
		}

		st := &ActivityStatus{Code: "P", Name: "Present", Description: "Working", IsWorkingDay: true, SortOrder: 1}
		if st.ToDomain().Code != "P" || ActivityStatusFromDomain(st.ToDomain()).Name != "Present" {
			t.Error("ActivityStatus mapping failed")
		}
		var nilSt *ActivityStatus
		if nilSt.ToDomain() != nil || ActivityStatusFromDomain(nil) != nil {
			t.Error("ActivityStatus nil check failed")
		}
	})

	t.Run("Activity and Overtime Mapping", func(t *testing.T) {
		act := &DailyActivity{
			ID:        1,
			UserID:    2,
			Date:      now,
			StartTime: "08:00",
			EndTime:   "17:00",
			Activity:  "Coding",
			Status:    "P",
		}
		if act.ToDomain().Activity != "Coding" || DailyActivityFromDomain(act.ToDomain()).Status != "P" {
			t.Error("DailyActivity mapping failed")
		}
		var nilAct *DailyActivity
		if nilAct.ToDomain() != nil || DailyActivityFromDomain(nil) != nil {
			t.Error("DailyActivity nil check failed")
		}

		ot := &OvertimeEntry{
			ID:              1,
			UserID:          2,
			Date:            now,
			StartTime:       "18:00",
			EndTime:         "20:00",
			TaskDescription: "Deployment",
		}
		if ot.ToDomain().TaskDescription != "Deployment" || OvertimeEntryFromDomain(ot.ToDomain()).StartTime != "18:00" {
			t.Error("OvertimeEntry mapping failed")
		}
		var nilOt *OvertimeEntry
		if nilOt.ToDomain() != nil || OvertimeEntryFromDomain(nil) != nil {
			t.Error("OvertimeEntry nil check failed")
		}
	})

	t.Run("Other Mappings", func(t *testing.T) {
		h := &Holiday{ID: 1, Date: now, Description: "Holiday", IsCivic: true}
		if h.ToDomain().Description != "Holiday" || HolidayFromDomain(h.ToDomain()).IsCivic != true {
			t.Error("Holiday mapping failed")
		}
		var nilH *Holiday
		if nilH.ToDomain() != nil || HolidayFromDomain(nil) != nil {
			t.Error("Holiday nil check failed")
		}

		ps := &PushSubscription{ID: 1, UserID: 2, Endpoint: "https://push", P256dh: "key", Auth: "secret"}
		if ps.ToDomain().Endpoint != "https://push" || PushSubscriptionFromDomain(ps.ToDomain()).P256dh != "key" {
			t.Error("PushSubscription mapping failed")
		}
		var nilPs *PushSubscription
		if nilPs.ToDomain() != nil || PushSubscriptionFromDomain(nil) != nil {
			t.Error("PushSubscription nil check failed")
		}

		ag := &AuthenticatorAAGUID{AAGUID: "abc", Name: "YubiKey", Icon: "icon"}
		if ag.ToDomain().Name != "YubiKey" || AuthenticatorAAGUIDFromDomain(ag.ToDomain()).AAGUID != "abc" {
			t.Error("AuthenticatorAAGUID mapping failed")
		}
		var nilAg *AuthenticatorAAGUID
		if nilAg.ToDomain() != nil || AuthenticatorAAGUIDFromDomain(nil) != nil {
			t.Error("AuthenticatorAAGUID nil check failed")
		}

		ss := &SystemSetting{Key: "is_new", Value: "N"}
		if ss.ToDomain().Key != "is_new" || SystemSettingFromDomain(ss.ToDomain()).Value != "N" {
			t.Error("SystemSetting mapping failed")
		}
		var nilSs *SystemSetting
		if nilSs.ToDomain() != nil || SystemSettingFromDomain(nil) != nil {
			t.Error("SystemSetting nil check failed")
		}
	})
}
