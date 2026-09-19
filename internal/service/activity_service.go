package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

const (
	dateFormatYYYYMMDD = "2006-01-02"
)

// ActivityService provides business operations for daily activity entries.
type ActivityService interface {
	UpsertDailyActivity(ctx context.Context, userID uint, req *request.DailyActivityRequest) error
	GetDailyActivity(ctx context.Context, userID uint, id uint) (*response.DailyActivityResponse, error)
	ListActivities(ctx context.Context, userID uint, filter repository.ActivityFilter) ([]response.DailyActivityResponse, response.PaginationMeta, error)
}

type activityService struct {
	repo repository.ActivityRepository
}

// NewActivityService constructs an ActivityService implementation.
func NewActivityService(repo repository.ActivityRepository) ActivityService {
	return &activityService{repo: repo}
}

func jakartaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

func (s *activityService) resolveProject(ctx context.Context, req *request.DailyActivityRequest, activity *models.DailyActivity) error {
	if req.ProjectRefID != nil && *req.ProjectRefID != 0 {
		proj, err := s.repo.FindActiveProjectByRefID(ctx, *req.ProjectRefID)
		if err != nil {
			return domain.NewUserError(domain.ErrInvalidInput, "Invalid project ID: project does not exist or is inactive")
		}
		activity.ProjectRefID = &proj.ID
		activity.ProjectRef = proj
		return nil
	}

	activity.ProjectRefID = nil
	activity.ProjectRef = nil
	return nil
}

func validateWorkingHours(startTime, endTime string) error {
	s := strings.TrimSpace(startTime)
	e := strings.TrimSpace(endTime)
	if s == "" || e == "" {
		return nil
	}
	start, err := time.Parse("15:04", s)
	if err != nil {
		return domain.NewUserError(domain.ErrInvalidInput, fmt.Sprintf("Invalid check-in time format ('%s'), expected HH:MM (e.g. 08:00)", s))
	}
	end, err := time.Parse("15:04", e)
	if err != nil {
		return domain.NewUserError(domain.ErrInvalidInput, fmt.Sprintf("Invalid check-out time format ('%s'), expected HH:MM (e.g. 17:00)", e))
	}
	if !start.Before(end) {
		return domain.NewUserError(domain.ErrInvalidInput, fmt.Sprintf("Check-out time (%s) must be later than check-in time (%s)", e, s))
	}
	return nil
}

func (s *activityService) UpsertDailyActivity(ctx context.Context, userID uint, req *request.DailyActivityRequest) error {
	if err := validateWorkingHours(req.StartTime, req.EndTime); err != nil {
		return err
	}

	date, err := time.ParseInLocation(dateFormatYYYYMMDD, req.Date, jakartaLocation())
	if err != nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Invalid date format, expected YYYY-MM-DD")
	}

	// Default status to 'P' if not provided
	if req.Status == "" {
		req.Status = "P"
	}

	// Validate status against activity_statuses
	valid, err := s.repo.ValidateStatus(ctx, req.Status)
	if err != nil || !valid {
		return domain.NewUserError(domain.ErrInvalidInput, "Invalid status code: must be a valid activity status (e.g. P, BT, S, PM, V, X)")
	}

	activity := models.DailyActivity{
		UserID:       userID,
		Date:         date,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		Status:       req.Status,
		Activity:     req.Activity,
		ProjectRefID: req.ProjectRefID,
	}

	if err := s.resolveProject(ctx, req, &activity); err != nil {
		return domain.NewUserError(domain.ErrInvalidInput, err.Error())
	}

	existing, err := s.repo.FindActiveByUserAndDate(ctx, userID, date)
	if err == nil && existing != nil && existing.ID != 0 {
		// Existing active record found -> UPDATE
		existing.StartTime = req.StartTime
		existing.EndTime = req.EndTime
		existing.Status = req.Status
		existing.Activity = req.Activity
		existing.ProjectRefID = activity.ProjectRefID
		existing.UpdatedAt = time.Now()
		return s.repo.Update(ctx, existing)
	}

	// No active record for this date -> INSERT new record with is_active = true
	activity.IsActive = true
	return s.repo.Create(ctx, &activity)
}

func (s *activityService) GetDailyActivity(ctx context.Context, userID uint, id uint) (*response.DailyActivityResponse, error) {
	activity, err := s.repo.FindActiveByID(ctx, id)
	if err != nil || activity == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Daily activity not found")
	}

	if activity.UserID != userID {
		return nil, domain.NewUserError(domain.ErrForbidden, "Forbidden: you do not have access to this activity")
	}

	return &response.DailyActivityResponse{
		ID:          activity.ID,
		Date:        activity.Date,
		StartTime:   activity.StartTime,
		EndTime:     activity.EndTime,
		Activity:    activity.Activity,
		ProjectName: activity.GetProjectName(),
		ProjectID:   activity.GetProjectCode(),
		Status:      activity.Status,
	}, nil
}

func (s *activityService) ListActivities(ctx context.Context, userID uint, filter repository.ActivityFilter) ([]response.DailyActivityResponse, response.PaginationMeta, error) {
	activities, totalRows, err := s.repo.ListActiveByUser(ctx, userID, filter)
	if err != nil {
		return nil, response.PaginationMeta{}, err
	}

	respList := make([]response.DailyActivityResponse, len(activities))
	for i, a := range activities {
		respList[i] = response.DailyActivityResponse{
			ID:          a.ID,
			Date:        a.Date,
			StartTime:   a.StartTime,
			EndTime:     a.EndTime,
			Activity:    a.Activity,
			ProjectName: a.GetProjectName(),
			ProjectID:   a.GetProjectCode(),
			Status:      a.Status,
		}
	}

	limit := filter.Limit
	if filter.IsAll {
		limit = int(totalRows)
		if limit == 0 {
			limit = 1
		}
	} else if limit <= 0 {
		limit = 10
	} else if limit > 100 {
		limit = 100
	}

	totalPages := 1
	if limit > 0 && totalRows > 0 {
		totalPages = int(math.Ceil(float64(totalRows) / float64(limit)))
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}

	meta := response.PaginationMeta{
		Page:       page,
		Limit:      limit,
		TotalRows:  totalRows,
		TotalPages: totalPages,
	}

	return respList, meta, nil
}
