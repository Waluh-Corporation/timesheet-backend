package service

import (
	"context"
	"errors"
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
	return res, int64(len(res)), nil
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
		if (projectID != "" && p.Code == projectID) || (projectName != "" && strings.EqualFold(p.Name, projectName)) {
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
	if act.ProjectID != "PRJ-01" || act.ProjectName != "Project Alpha" {
		t.Errorf("project fields mismatch: code=%s, name=%s", act.ProjectID, act.ProjectName)
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
	repo.activities[1] = &models.DailyActivity{
		ID:        1,
		UserID:    100,
		Date:      parsedDate,
		StartTime: "08:30",
		EndTime:   "17:30",
		Status:    "P",
		Activity:  "Test task",
		IsActive:  true,
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
}

func TestActivityService_ListActivities(t *testing.T) {
	repo := newMockActivityRepo()
	svc := NewActivityService(repo)
	ctx := context.Background()

	parsedDate, _ := time.Parse("2006-01-02", "2026-09-20")
	repo.activities[1] = &models.DailyActivity{
		ID:        1,
		UserID:    100,
		Date:      parsedDate,
		StartTime: "08:30",
		EndTime:   "17:30",
		Status:    "P",
		Activity:  "Task 1",
		IsActive:  true,
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
}
