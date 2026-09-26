package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

type mockActivityRepo struct {
	activities  map[uint]*models.DailyActivity
	projects    map[uint]*models.Project
	validStatus map[string]bool
	createErr   error
	updateErr   error
	listErr     error
}

func newMockActivityRepo() *mockActivityRepo {
	return &mockActivityRepo{
		activities: make(map[uint]*models.DailyActivity),
		projects:   make(map[uint]*models.Project),
		validStatus: map[string]bool{
			"P":  true,
			"BT": true,
			"S":  true,
			"PM": true,
			"V":  true,
			"X":  true,
		},
	}
}

func (m *mockActivityRepo) FindActiveByID(ctx context.Context, id uint) (*models.DailyActivity, error) {
	act, ok := m.activities[id]
	if !ok || !act.IsActive {
		return nil, domain.ErrNotFound
	}
	return act, nil
}

func (m *mockActivityRepo) FindActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.DailyActivity, error) {
	for _, act := range m.activities {
		if act.UserID == userID && act.Date.Equal(date) && act.IsActive {
			return act, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockActivityRepo) Create(ctx context.Context, activity *models.DailyActivity) error {
	if m.createErr != nil {
		return m.createErr
	}
	activity.ID = uint(len(m.activities) + 1)
	m.activities[activity.ID] = activity
	return nil
}

func (m *mockActivityRepo) Update(ctx context.Context, activity *models.DailyActivity) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.activities[activity.ID] = activity
	return nil
}

func (m *mockActivityRepo) ListActiveByUser(ctx context.Context, userID uint, filter repository.ActivityFilter) ([]models.DailyActivity, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var res []models.DailyActivity
	for _, act := range m.activities {
		if act.UserID == userID && act.IsActive {
			res = append(res, *act)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})
	return res, int64(len(res)), nil
}

func (m *mockActivityRepo) GetLatestActivityUpdateTime(ctx context.Context, userID uint, month, year int) (time.Time, error) {
	var latest time.Time
	for _, act := range m.activities {
		if act.UserID == userID && int(act.Date.Month()) == month && act.Date.Year() == year && act.IsActive {
			if act.UpdatedAt.After(latest) {
				latest = act.UpdatedAt
			}
		}
	}
	return latest, nil
}

func (m *mockActivityRepo) ValidateStatus(ctx context.Context, status string) (bool, error) {
	return m.validStatus[status], nil
}

func (m *mockActivityRepo) FindActiveProjectByRefID(ctx context.Context, refID uint) (*models.Project, error) {
	proj, ok := m.projects[refID]
	if !ok || !proj.IsActive {
		return nil, errors.New("project not found")
	}
	return proj, nil
}

func (m *mockActivityRepo) FindActiveProjectByCodeOrName(ctx context.Context, projectID, projectName string) (*models.Project, error) {
	for _, p := range m.projects {
		if !p.IsActive {
			continue
		}
		if projectID != "" && projectName != "" {
			if p.Code == projectID && strings.EqualFold(p.Name, projectName) {
				return p, nil
			}
		} else if projectID != "" && p.Code == projectID {
			return p, nil
		} else if projectName != "" && strings.EqualFold(p.Name, projectName) {
			return p, nil
		}
	}
	return nil, errors.New("project not found")
}

func TestActivityService_UpsertDailyActivity(t *testing.T) {
	repo := newMockActivityRepo()
	svc := NewActivityService(repo)
	ctx := context.Background()

	// 1. Invalid date format
	badDateReq := &request.DailyActivityRequest{
		Date:     "invalid-date",
		Activity: "Coding",
	}
	if err := svc.UpsertDailyActivity(ctx, 1, badDateReq); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput on bad date, got: %v", err)
	}

	// 2. Invalid status code
	badStatusReq := &request.DailyActivityRequest{
		Date:     "2026-09-20",
		Status:   "INVALID_STATUS",
		Activity: "Coding",
	}
	if err := svc.UpsertDailyActivity(ctx, 1, badStatusReq); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput on bad status, got: %v", err)
	}

	// 3. Project resolution by Ref ID failing
	badProjRef := uint(999)
	badProjReq := &request.DailyActivityRequest{
		Date:         "2026-09-20",
		Status:       "P",
		ProjectRefID: &badProjRef,
		Activity:     "Coding",
	}
	if err := svc.UpsertDailyActivity(ctx, 1, badProjReq); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput on invalid project ref, got: %v", err)
	}

	// 4. Success Insert (creates new active entry)
	validProjRef := uint(10)
	repo.projects[validProjRef] = &models.Project{
		ID:       validProjRef,
		Code:     "PRJ-01",
		Name:     "Project Alpha",
		IsActive: true,
	}

	validReq := &request.DailyActivityRequest{
		Date:         "2026-09-20",
		StartTime:    "08:30",
		EndTime:      "17:30",
		ProjectRefID: &validProjRef,
		Activity:     "First implementation",
	}
	if err := svc.UpsertDailyActivity(ctx, 1, validReq); err != nil {
		t.Fatalf("unexpected error on upsert: %v", err)
	}

	if len(repo.activities) != 1 {
		t.Fatalf("expected 1 activity created, got %d", len(repo.activities))
	}
	act := repo.activities[1]
	if act.Status != "P" {
		t.Errorf("expected default status P, got %s", act.Status)
	}
	if act.GetProjectCode() != "PRJ-01" || act.GetProjectName() != "Project Alpha" {
		t.Errorf("project fields mismatch: code=%s, name=%s", act.GetProjectCode(), act.GetProjectName())
	}

	// 5. Success Update (same date, same user -> update existing)
	updateReq := &request.DailyActivityRequest{
		Date:         "2026-09-20",
		StartTime:    "09:00",
		EndTime:      "18:00",
		Status:       "BT",
		ProjectRefID: &validProjRef,
		Activity:     "Updated implementation",
	}
	if err := svc.UpsertDailyActivity(ctx, 1, updateReq); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	if len(repo.activities) != 1 {
		t.Fatalf("expected still 1 activity after update, got %d", len(repo.activities))
	}
	updated := repo.activities[1]
	if updated.Status != "BT" || updated.StartTime != "09:00" || updated.Activity != "Updated implementation" {
		t.Errorf("activity not properly updated: %+v", updated)
	}
}

func TestActivityService_GetDailyActivity(t *testing.T) {
	repo := newMockActivityRepo()
	svc := NewActivityService(repo)
	ctx := context.Background()

	parsedDate, _ := time.Parse("2006-01-02", "2026-09-20")
	proj55 := &models.Project{ID: 55, Code: "PRJ-55", Name: "Project Beta", AppImpacted: "Beta App", IsActive: true}
	repo.projects[55] = proj55
	projRefID := uint(55)
	repo.activities[1] = &models.DailyActivity{
		ID:           1,
		UserID:       100,
		Date:         parsedDate,
		StartTime:    "08:30",
		EndTime:      "17:30",
		Status:       "P",
		Activity:     "Test task",
		ProjectRefID: &projRefID,
		ProjectRef:   proj55,
		IsActive:     true,
	}

	// 1. Not found
	_, err := svc.GetDailyActivity(ctx, 100, 999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}

	// 2. Forbidden (other user)
	_, err = svc.GetDailyActivity(ctx, 999, 1)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}

	// 3. Success
	resp, err := svc.GetDailyActivity(ctx, 100, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 1 || resp.Activity != "Test task" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.AppImpacted != "Beta App" {
		t.Errorf("expected AppImpacted 'Beta App', got %q", resp.AppImpacted)
	}
	if resp.ProjectRefID == nil || *resp.ProjectRefID != 55 {
		t.Errorf("expected ProjectRefID 55, got %v", resp.ProjectRefID)
	}
	if resp.ProjectRef == nil || resp.ProjectRef.Name != "Project Beta" {
		t.Errorf("expected ProjectRef to be populated, got %+v", resp.ProjectRef)
	}
}

func TestActivityService_ListActivities(t *testing.T) {
	repo := newMockActivityRepo()
	svc := NewActivityService(repo)
	ctx := context.Background()

	parsedDate, _ := time.Parse("2006-01-02", "2026-09-20")
	proj55 := &models.Project{ID: 55, Code: "PRJ-55", Name: "Project Beta", AppImpacted: "Beta App", IsActive: true}
	projRefID := uint(55)
	repo.activities[1] = &models.DailyActivity{
		ID:           1,
		UserID:       100,
		Date:         parsedDate,
		StartTime:    "08:30",
		EndTime:      "17:30",
		Status:       "P",
		Activity:     "Task 1",
		ProjectRefID: &projRefID,
		ProjectRef:   proj55,
		IsActive:     true,
	}
	repo.activities[2] = &models.DailyActivity{
		ID:        2,
		UserID:    100,
		Date:      parsedDate.AddDate(0, 0, 1),
		StartTime: "08:30",
		EndTime:   "17:30",
		Status:    "P",
		Activity:  "Task 2",
		IsActive:  true,
	}

	filter := repository.ActivityFilter{
		Page:  1,
		Limit: 10,
	}

	list, meta, err := svc.ListActivities(ctx, 100, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(list))
	}
	if meta.TotalRows != 2 || meta.TotalPages != 1 || meta.Page != 1 {
		t.Errorf("pagination meta mismatch: %+v", meta)
	}
	if list[0].AppImpacted != "Beta App" {
		t.Errorf("expected list[0].AppImpacted 'Beta App', got %q", list[0].AppImpacted)
	}
	if list[0].ProjectRefID == nil || *list[0].ProjectRefID != 55 {
		t.Errorf("expected list[0].ProjectRefID 55, got %v", list[0].ProjectRefID)
	}

	// 2. IsAll = true with totalRows > 0
	listAll, metaAll, err := svc.ListActivities(ctx, 100, repository.ActivityFilter{IsAll: true})
	if err != nil || len(listAll) != 2 || metaAll.Limit != 2 {
		t.Fatalf("expected IsAll to set limit to 2, got: %+v", metaAll)
	}

	// 3. IsAll = true with 0 rows
	listEmpty, metaEmpty, err := svc.ListActivities(ctx, 9999, repository.ActivityFilter{IsAll: true})
	if err != nil || len(listEmpty) != 0 || metaEmpty.Limit != 1 {
		t.Fatalf("expected IsAll with 0 rows to set limit to 1, got: %+v", metaEmpty)
	}

	// 4. Limit <= 0 and Page < 1
	_, metaDefault, err := svc.ListActivities(ctx, 100, repository.ActivityFilter{Page: 0, Limit: -1})
	if err != nil || metaDefault.Page != 1 || metaDefault.Limit != 10 {
		t.Fatalf("expected default page=1 and limit=10, got: %+v", metaDefault)
	}

	// 5. Limit > 100
	_, metaCap, err := svc.ListActivities(ctx, 100, repository.ActivityFilter{Page: 1, Limit: 200})
	if err != nil || metaCap.Limit != 100 {
		t.Fatalf("expected capped limit=100, got: %+v", metaCap)
	}

	// 6. Repo list error
	repo.listErr = errors.New("db query failed")
	_, _, err = svc.ListActivities(ctx, 100, filter)
	if err == nil {
		t.Fatal("expected error on repo.ListActiveByUser failure, got nil")
	}
}

func TestActivityService_UpsertProjectResolution(t *testing.T) {
	repo := newMockActivityRepo()
	svc := NewActivityService(repo)
	ctx := context.Background()

	proj := &models.Project{
		ID:          55,
		Code:        "PRJ-55",
		Name:        "Project Beta",
		AppImpacted: "Beta App",
		IsActive:    true,
	}
	repo.projects[proj.ID] = proj

	// Resolve by ProjectRefID
	projID := uint(55)
	req := &request.DailyActivityRequest{
		Date:         "2026-09-21",
		StartTime:    "08:00",
		EndTime:      "17:00",
		ProjectRefID: &projID,
		Activity:     "Beta development",
	}
	if err := svc.UpsertDailyActivity(ctx, 10, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved := repo.activities[1]
	if saved.ProjectRefID == nil || *saved.ProjectRefID != 55 {
		t.Errorf("expected ProjectRefID 55, got: %v", saved.ProjectRefID)
	}
	if saved.GetProjectCode() != "PRJ-55" || saved.GetProjectName() != "Project Beta" {
		t.Errorf("expected project code/name from ref, got %s / %s", saved.GetProjectCode(), saved.GetProjectName())
	}

	// Empty project
	reqEmptyProj := &request.DailyActivityRequest{
		Date:      "2026-09-22",
		StartTime: "08:00",
		EndTime:   "17:00",
		Activity:  "No project activity",
	}
	if err := svc.UpsertDailyActivity(ctx, 10, reqEmptyProj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	savedEmpty := repo.activities[2]
	if savedEmpty.ProjectRefID != nil {
		t.Errorf("expected nil ProjectRefID, got: %v", savedEmpty.ProjectRefID)
	}
	if savedEmpty.GetProjectCode() != "" || savedEmpty.GetProjectName() != "" {
		t.Errorf("expected empty project code/name, got %s / %s", savedEmpty.GetProjectCode(), savedEmpty.GetProjectName())
	}

	// Project resolved by Code and Name fallback when ProjectRefID is nil
	reqCodeName := &request.DailyActivityRequest{
		Date:        "2026-09-23",
		StartTime:   "08:00",
		EndTime:     "17:00",
		ProjectID:   "PRJ-55",
		ProjectName: "Project Beta",
		Activity:    "Fallback resolution",
	}
	if err := svc.UpsertDailyActivity(ctx, 10, reqCodeName); err != nil {
		t.Fatalf("unexpected error on fallback resolution: %v", err)
	}
	savedFallback := repo.activities[3]
	if savedFallback.ProjectRefID == nil || *savedFallback.ProjectRefID != 55 {
		t.Errorf("expected ProjectRefID 55 from fallback, got: %v", savedFallback.ProjectRefID)
	}
	if savedFallback.GetAppImpacted() != "Beta App" {
		t.Errorf("expected AppImpacted 'Beta App' from fallback, got %q", savedFallback.GetAppImpacted())
	}
}

func TestActivityService_WorkingHoursValidation(t *testing.T) {
	repo := newMockActivityRepo()
	svc := NewActivityService(repo)
	ctx := context.Background()

	tests := []struct {
		name        string
		startTime   string
		endTime     string
		expectError bool
		errContains string
	}{
		{
			name:        "valid normal hours",
			startTime:   "08:00",
			endTime:     "17:00",
			expectError: false,
		},
		{
			name:        "empty hours allowed",
			startTime:   "",
			endTime:     "",
			expectError: false,
		},
		{
			name:        "check-out earlier than check-in (06:00 < 07:00)",
			startTime:   "07:00",
			endTime:     "06:00",
			expectError: true,
			errContains: "Check-out time (06:00) must be later than check-in time (07:00)",
		},
		{
			name:        "check-in equals check-out (08:00 == 08:00)",
			startTime:   "08:00",
			endTime:     "08:00",
			expectError: true,
			errContains: "Check-out time (08:00) must be later than check-in time (08:00)",
		},
		{
			name:        "invalid check-in format",
			startTime:   "25:00",
			endTime:     "17:00",
			expectError: true,
			errContains: "Invalid check-in time format",
		},
		{
			name:        "invalid check-out format",
			startTime:   "08:00",
			endTime:     "99:99",
			expectError: true,
			errContains: "Invalid check-out time format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &request.DailyActivityRequest{
				Date:      "2026-09-20",
				StartTime: tc.startTime,
				EndTime:   tc.endTime,
				Status:    "P",
				Activity:  "Test task",
			}
			err := svc.UpsertDailyActivity(ctx, 1, req)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, domain.ErrInvalidInput) {
					t.Errorf("expected ErrInvalidInput, got %v", err)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("expected error message to contain %q, got: %s", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
