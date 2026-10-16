package models

import (
	"timesheet-backend/internal/domain/entity"
)

// ToDomain maps the GORM User persistence model to its domain entity representation.
func (u *User) ToDomain() *entity.User {
	if u == nil {
		return nil
	}
	return &entity.User{
		ID:           u.ID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		Username:     u.Username,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         entity.Role(u.Role),
		IsActive:     u.IsActive,
		Name:         u.Name,
		BniID:        u.BniID,
		EmployeeID:   u.EmployeeID,
		Division:     u.Division,
		DivisionID:   u.DivisionID,
		Department:   u.Department,
		DepartmentID: u.DepartmentID,
		GroupName:    u.GroupName,
		Position:     u.Position,
		Site:         u.Site,
		SiteID:       u.SiteID,
		Company:      u.Company,
		CompanyID:    u.CompanyID,
	}
}

// UserFromDomain creates a GORM User model from a domain User entity.
func UserFromDomain(e *entity.User) *User {
	if e == nil {
		return nil
	}
	return &User{
		ID:           e.ID,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
		Username:     e.Username,
		Email:        e.Email,
		PasswordHash: e.PasswordHash,
		Role:         Role(e.Role),
		IsActive:     e.IsActive,
		Name:         e.Name,
		BniID:        e.BniID,
		EmployeeID:   e.EmployeeID,
		Division:     e.Division,
		DivisionID:   e.DivisionID,
		Department:   e.Department,
		DepartmentID: e.DepartmentID,
		GroupName:    e.GroupName,
		Position:     e.Position,
		Site:         e.Site,
		SiteID:       e.SiteID,
		Company:      e.Company,
		CompanyID:    e.CompanyID,
	}
}

// ToDomain maps the GORM ProfileChangeRequest model to its domain entity.
func (p *ProfileChangeRequest) ToDomain() *entity.ProfileChangeRequest {
	if p == nil {
		return nil
	}
	return &entity.ProfileChangeRequest{
		ID:           p.ID,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
		UserID:       p.UserID,
		Status:       entity.ProfileStatus(p.Status),
		Name:         p.Name,
		BniID:        p.BniID,
		EmployeeID:   p.EmployeeID,
		Division:     p.Division,
		DivisionID:   p.DivisionID,
		Department:   p.Department,
		DepartmentID: p.DepartmentID,
		GroupName:    p.GroupName,
		Position:     p.Position,
		Site:         p.Site,
		SiteID:       p.SiteID,
		CompanyID:    p.CompanyID,
		Email:        p.Email,
		Notes:        p.Notes,
		ReviewedBy:   p.ReviewedBy,
		ReviewedAt:   p.ReviewedAt,
	}
}

// ProfileChangeRequestFromDomain creates a GORM ProfileChangeRequest from a domain entity.
func ProfileChangeRequestFromDomain(e *entity.ProfileChangeRequest) *ProfileChangeRequest {
	if e == nil {
		return nil
	}
	return &ProfileChangeRequest{
		ID:           e.ID,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
		UserID:       e.UserID,
		Status:       ProfileStatus(e.Status),
		Name:         e.Name,
		BniID:        e.BniID,
		EmployeeID:   e.EmployeeID,
		Division:     e.Division,
		DivisionID:   e.DivisionID,
		Department:   e.Department,
		DepartmentID: e.DepartmentID,
		GroupName:    e.GroupName,
		Position:     e.Position,
		Site:         e.Site,
		SiteID:       e.SiteID,
		CompanyID:    e.CompanyID,
		Email:        e.Email,
		Notes:        e.Notes,
		ReviewedBy:   e.ReviewedBy,
		ReviewedAt:   e.ReviewedAt,
	}
}

// ToDomain maps Company to domain entity.
func (c *Company) ToDomain() *entity.Company {
	if c == nil {
		return nil
	}
	return &entity.Company{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Code:      c.Code,
		Name:      c.Name,
		IsActive:  c.IsActive,
	}
}

// CompanyFromDomain creates Company model from domain entity.
func CompanyFromDomain(e *entity.Company) *Company {
	if e == nil {
		return nil
	}
	return &Company{
		ID:        e.ID,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		Code:      e.Code,
		Name:      e.Name,
		IsActive:  e.IsActive,
	}
}

// ToDomain maps Department to domain entity.
func (d *Department) ToDomain() *entity.Department {
	if d == nil {
		return nil
	}
	return &entity.Department{
		ID:         d.ID,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
		Code:       d.Code,
		Name:       d.Name,
		Division:   d.Division,
		DivisionID: d.DivisionID,
		IsActive:   d.IsActive,
	}
}

// DepartmentFromDomain creates Department model from domain entity.
func DepartmentFromDomain(e *entity.Department) *Department {
	if e == nil {
		return nil
	}
	return &Department{
		ID:         e.ID,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
		Code:       e.Code,
		Name:       e.Name,
		Division:   e.Division,
		DivisionID: e.DivisionID,
		IsActive:   e.IsActive,
	}
}

// ToDomain maps Division to domain entity.
func (d *Division) ToDomain() *entity.Division {
	if d == nil {
		return nil
	}
	return &entity.Division{
		ID:        d.ID,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
		Code:      d.Code,
		Name:      d.Name,
		IsActive:  d.IsActive,
	}
}

// DivisionFromDomain creates Division model from domain entity.
func DivisionFromDomain(e *entity.Division) *Division {
	if e == nil {
		return nil
	}
	return &Division{
		ID:        e.ID,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		Code:      e.Code,
		Name:      e.Name,
		IsActive:  e.IsActive,
	}
}

// ToDomain maps Site to domain entity.
func (s *Site) ToDomain() *entity.Site {
	if s == nil {
		return nil
	}
	return &entity.Site{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Code:      s.Code,
		Name:      s.Name,
		IsActive:  s.IsActive,
	}
}

// SiteFromDomain creates Site model from domain entity.
func SiteFromDomain(e *entity.Site) *Site {
	if e == nil {
		return nil
	}
	return &Site{
		ID:        e.ID,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		Code:      e.Code,
		Name:      e.Name,
		IsActive:  e.IsActive,
	}
}

// ToDomain maps Approver to domain entity.
func (a *Approver) ToDomain() *entity.Approver {
	if a == nil {
		return nil
	}
	return &entity.Approver{
		ID:        a.ID,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
		Name:      a.Name,
		RoleType:  entity.ApproverRoleType(a.RoleType),
		Title:     a.Title,
		IsActive:  a.IsActive,
	}
}

// ApproverFromDomain creates Approver model from domain entity.
func ApproverFromDomain(e *entity.Approver) *Approver {
	if e == nil {
		return nil
	}
	return &Approver{
		ID:        e.ID,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		Name:      e.Name,
		RoleType:  ApproverRoleType(e.RoleType),
		Title:     e.Title,
		IsActive:  e.IsActive,
	}
}

// ToDomain maps Project to domain entity.
func (p *Project) ToDomain() *entity.Project {
	if p == nil {
		return nil
	}
	return &entity.Project{
		ID:          p.ID,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		Code:        p.Code,
		Name:        p.Name,
		AppImpacted: p.AppImpacted,
		IsActive:    p.IsActive,
	}
}

// ProjectFromDomain creates Project model from domain entity.
func ProjectFromDomain(e *entity.Project) *Project {
	if e == nil {
		return nil
	}
	return &Project{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		Code:        e.Code,
		Name:        e.Name,
		AppImpacted: e.AppImpacted,
		IsActive:    e.IsActive,
	}
}

// ToDomain maps ActivityStatus to domain entity.
func (s *ActivityStatus) ToDomain() *entity.ActivityStatus {
	if s == nil {
		return nil
	}
	return &entity.ActivityStatus{
		Code:         s.Code,
		Name:         s.Name,
		Description:  s.Description,
		IsWorkingDay: s.IsWorkingDay,
		SortOrder:    s.SortOrder,
	}
}

// ActivityStatusFromDomain creates ActivityStatus model from domain entity.
func ActivityStatusFromDomain(e *entity.ActivityStatus) *ActivityStatus {
	if e == nil {
		return nil
	}
	return &ActivityStatus{
		Code:         e.Code,
		Name:         e.Name,
		Description:  e.Description,
		IsWorkingDay: e.IsWorkingDay,
		SortOrder:    e.SortOrder,
	}
}

// ToDomain maps DailyActivity to domain entity.
func (a *DailyActivity) ToDomain() *entity.DailyActivity {
	if a == nil {
		return nil
	}
	return &entity.DailyActivity{
		ID:           a.ID,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
		UserID:       a.UserID,
		Date:         a.Date,
		StartTime:    a.StartTime,
		EndTime:      a.EndTime,
		Status:       a.Status,
		Activity:     a.Activity,
		IsActive:     a.IsActive,
		ProjectRefID: a.ProjectRefID,
	}
}

// DailyActivityFromDomain creates DailyActivity model from domain entity.
func DailyActivityFromDomain(e *entity.DailyActivity) *DailyActivity {
	if e == nil {
		return nil
	}
	return &DailyActivity{
		ID:           e.ID,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
		UserID:       e.UserID,
		Date:         e.Date,
		StartTime:    e.StartTime,
		EndTime:      e.EndTime,
		Status:       e.Status,
		Activity:     e.Activity,
		IsActive:     e.IsActive,
		ProjectRefID: e.ProjectRefID,
	}
}

// ToDomain maps OvertimeEntry to domain entity.
func (o *OvertimeEntry) ToDomain() *entity.OvertimeEntry {
	if o == nil {
		return nil
	}
	return &entity.OvertimeEntry{
		ID:               o.ID,
		CreatedAt:        o.CreatedAt,
		UpdatedAt:        o.UpdatedAt,
		UserID:           o.UserID,
		DailyActivityID:  o.DailyActivityID,
		Date:             o.Date,
		StartTime:        o.StartTime,
		EndTime:          o.EndTime,
		TaskDescription:  o.TaskDescription,
		TeamLeaderID:     o.TeamLeaderID,
		DepartmentHeadID: o.DepartmentHeadID,
		IsActive:         o.IsActive,
	}
}

// OvertimeEntryFromDomain creates OvertimeEntry model from domain entity.
func OvertimeEntryFromDomain(e *entity.OvertimeEntry) *OvertimeEntry {
	if e == nil {
		return nil
	}
	return &OvertimeEntry{
		ID:               e.ID,
		CreatedAt:        e.CreatedAt,
		UpdatedAt:        e.UpdatedAt,
		UserID:           e.UserID,
		DailyActivityID:  e.DailyActivityID,
		Date:             e.Date,
		StartTime:        e.StartTime,
		EndTime:          e.EndTime,
		TaskDescription:  e.TaskDescription,
		TeamLeaderID:     e.TeamLeaderID,
		DepartmentHeadID: e.DepartmentHeadID,
		IsActive:         e.IsActive,
	}
}

// ToDomain maps Holiday to domain entity.
func (h *Holiday) ToDomain() *entity.Holiday {
	if h == nil {
		return nil
	}
	return &entity.Holiday{
		ID:           h.ID,
		CreatedAt:    h.CreatedAt,
		UpdatedAt:    h.UpdatedAt,
		Date:         h.Date,
		Description:  h.Description,
		IsJointLeave: h.IsJointLeave,
		IsCivic:      h.IsCivic,
		IsReligious:  h.IsReligious,
	}
}

// HolidayFromDomain creates Holiday model from domain entity.
func HolidayFromDomain(e *entity.Holiday) *Holiday {
	if e == nil {
		return nil
	}
	return &Holiday{
		ID:           e.ID,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
		Date:         e.Date,
		Description:  e.Description,
		IsJointLeave: e.IsJointLeave,
		IsCivic:      e.IsCivic,
		IsReligious:  e.IsReligious,
	}
}

// ToDomain maps PushSubscription to domain entity.
func (s *PushSubscription) ToDomain() *entity.PushSubscription {
	if s == nil {
		return nil
	}
	return &entity.PushSubscription{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UserID:    s.UserID,
		Endpoint:  s.Endpoint,
		P256dh:    s.P256dh,
		Auth:      s.Auth,
	}
}

// PushSubscriptionFromDomain creates PushSubscription model from domain entity.
func PushSubscriptionFromDomain(e *entity.PushSubscription) *PushSubscription {
	if e == nil {
		return nil
	}
	return &PushSubscription{
		ID:        e.ID,
		CreatedAt: e.CreatedAt,
		UserID:    e.UserID,
		Endpoint:  e.Endpoint,
		P256dh:    e.P256dh,
		Auth:      e.Auth,
	}
}

// ToDomain maps AuthenticatorAAGUID to domain entity.
func (a *AuthenticatorAAGUID) ToDomain() *entity.AuthenticatorAAGUID {
	if a == nil {
		return nil
	}
	return &entity.AuthenticatorAAGUID{
		AAGUID:    a.AAGUID,
		Name:      a.Name,
		Icon:      a.Icon,
		UpdatedAt: a.UpdatedAt,
	}
}

// AuthenticatorAAGUIDFromDomain creates AuthenticatorAAGUID model from domain entity.
func AuthenticatorAAGUIDFromDomain(e *entity.AuthenticatorAAGUID) *AuthenticatorAAGUID {
	if e == nil {
		return nil
	}
	return &AuthenticatorAAGUID{
		AAGUID:    e.AAGUID,
		Name:      e.Name,
		Icon:      e.Icon,
		UpdatedAt: e.UpdatedAt,
	}
}

// ToDomain maps SystemSetting to domain entity.
func (s *SystemSetting) ToDomain() *entity.SystemSetting {
	if s == nil {
		return nil
	}
	return &entity.SystemSetting{
		Key:       s.Key,
		Value:     s.Value,
		UpdatedAt: s.UpdatedAt,
	}
}

// SystemSettingFromDomain creates SystemSetting model from domain entity.
func SystemSettingFromDomain(e *entity.SystemSetting) *SystemSetting {
	if e == nil {
		return nil
	}
	return &SystemSetting{
		Key:       e.Key,
		Value:     e.Value,
		UpdatedAt: e.UpdatedAt,
	}
}
