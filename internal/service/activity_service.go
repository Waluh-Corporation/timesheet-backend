package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/config"
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
	GetPreviousDailyActivity(ctx context.Context, userID uint, targetDate string) (*response.DailyActivityResponse, error)
	ListActivities(ctx context.Context, userID uint, filter repository.ActivityFilter) ([]response.DailyActivityResponse, response.PaginationMeta, error)
	GetEligibleWitnesses(ctx context.Context, currentUserID uint) ([]response.WitnessResponse, error)
	GetMonthlyWitness(ctx context.Context, userID uint, month, year int) (*response.WitnessResponse, error)
	UpdateMonthlyWitness(ctx context.Context, userID uint, month, year int, newWitnessID uint) (int64, *models.User, error)
}

type activityService struct {
	repo    repository.ActivityRepository
	subRepo repository.SubmissionRepository
	nowFunc func() time.Time
}

// NewActivityService constructs an ActivityService implementation.
func NewActivityService(repo repository.ActivityRepository, subRepo ...repository.SubmissionRepository) ActivityService {
	var sRepo repository.SubmissionRepository
	if len(subRepo) > 0 {
		sRepo = subRepo[0]
	}
	return &activityService{
		repo:    repo,
		subRepo: sRepo,
		nowFunc: time.Now,
	}
}

// SetNowFunc overrides the time provider for testing purposes.
func (s *activityService) SetNowFunc(fn func() time.Time) {
	s.nowFunc = fn
}

func appLocation() *time.Location {
	return config.AppLocation()
}

func jakartaLocation() *time.Location {
	return appLocation()
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

	if req.ProjectID != "" || req.ProjectName != "" {
		proj, err := s.repo.FindActiveProjectByCodeOrName(ctx, req.ProjectID, req.ProjectName)
		if err == nil && proj != nil && proj.ID != 0 {
			activity.ProjectRefID = &proj.ID
			activity.ProjectRef = proj
			return nil
		}
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

func (s *activityService) parseAndValidateDate(ctx context.Context, dateStr string) (time.Time, error) {
	date, err := time.ParseInLocation(dateFormatYYYYMMDD, dateStr, jakartaLocation())
	if err != nil {
		return time.Time{}, domain.NewUserError(domain.ErrInvalidInput, "Invalid date format, expected YYYY-MM-DD")
	}

	now := time.Now()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}

	if !domain.IsAdminFromContext(ctx) {
		if err := domain.ValidateTimesheetDate(date, now, jakartaLocation()); err != nil {
			return time.Time{}, err
		}
	}
	return date, nil
}

func (s *activityService) resolveBeritaAcaraWitness(ctx context.Context, userID uint, date time.Time, req *request.DailyActivityRequest) (*uint, string, error) {
	if !req.IsBeritaAcara {
		return nil, "", nil
	}

	notes := strings.TrimSpace(req.BeritaAcaraNotes)
	if notes == "" {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "Berita acara notes are required")
	}

	month := int(date.Month())
	year := date.Year()

	lockedWitness, err := s.repo.GetMonthlyWitness(ctx, userID, month, year)
	if err != nil {
		return nil, "", err
	}

	if lockedWitness != nil {
		if req.WitnessID != nil && *req.WitnessID != 0 && *req.WitnessID != lockedWitness.ID {
			return nil, "", domain.NewUserError(domain.ErrInvalidInput, fmt.Sprintf("The witness for this month has already been designated as %s. Please use the same witness.", lockedWitness.Name))
		}
		return &lockedWitness.ID, notes, nil
	}

	if req.WitnessID == nil || *req.WitnessID == 0 {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "Berita acara witness is required")
	}
	if *req.WitnessID == userID {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "Cannot select yourself as a berita acara witness")
	}
	witness, err := s.repo.FindWitnessByID(ctx, *req.WitnessID)
	if err != nil || witness == nil {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "Witness not found or inactive")
	}
	if witness.Role != models.RoleUser {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "Witness must be a user with 'user' role")
	}

	return &witness.ID, notes, nil
}

func (s *activityService) UpsertDailyActivity(ctx context.Context, userID uint, req *request.DailyActivityRequest) error {
	if err := validateWorkingHours(req.StartTime, req.EndTime); err != nil {
		return err
	}

	date, err := s.parseAndValidateDate(ctx, req.Date)
	if err != nil {
		return err
	}

	if s.subRepo != nil {
		m := int(date.Month())
		y := date.Year()
		if tsLocked, _ := s.subRepo.IsTimesheetLocked(ctx, userID, m, y); tsLocked {
			return domain.NewUserError(domain.ErrLockedPeriod, "Periode timesheet ini telah ditandatangani. Batalkan tanda tangan terlebih dahulu untuk mengubah aktivitas.")
		}
		if req.IsBeritaAcara {
			if baLocked, _ := s.subRepo.IsBeritaAcaraLocked(ctx, userID, m, y); baLocked {
				return domain.NewUserError(domain.ErrLockedPeriod, "Periode Berita Acara ini telah diajukan/ditandatangani. Batalkan pengajuan terlebih dahulu untuk mengubah aktivitas.")
			}
		}
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

	targetWitnessID, notes, err := s.resolveBeritaAcaraWitness(ctx, userID, date, req)
	if err != nil {
		return err
	}

	activity := models.DailyActivity{
		UserID:           userID,
		Date:             date,
		StartTime:        req.StartTime,
		EndTime:          req.EndTime,
		Status:           req.Status,
		Activity:         req.Activity,
		ProjectRefID:     req.ProjectRefID,
		IsBeritaAcara:    req.IsBeritaAcara,
		WitnessID:        targetWitnessID,
		BeritaAcaraNotes: notes,
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
		existing.ProjectRef = activity.ProjectRef
		existing.IsBeritaAcara = activity.IsBeritaAcara
		existing.WitnessID = activity.WitnessID
		existing.BeritaAcaraNotes = activity.BeritaAcaraNotes
		existing.UpdatedAt = time.Now()
		return s.repo.Update(ctx, existing)
	}

	// No active record for this date -> INSERT new record with is_active = true
	activity.IsActive = true
	return s.repo.Create(ctx, &activity)
}

func toWitnessResponse(w *models.User) *response.WitnessResponse {
	if w == nil {
		return nil
	}
	return &response.WitnessResponse{
		ID:   w.ID,
		Name: w.Name,
	}
}

func toDailyActivityResponse(a *models.DailyActivity) *response.DailyActivityResponse {
	if a == nil {
		return nil
	}
	return &response.DailyActivityResponse{
		ID:               a.ID,
		Date:             a.Date,
		StartTime:        a.StartTime,
		EndTime:          a.EndTime,
		Activity:         a.Activity,
		ProjectName:      a.GetProjectName(),
		ProjectID:        a.GetProjectCode(),
		Status:           a.Status,
		AppImpacted:      a.GetAppImpacted(),
		ProjectRefID:     a.ProjectRefID,
		ProjectRef:       a.ProjectRef,
		StatusRef:        a.StatusRef,
		IsBeritaAcara:    a.IsBeritaAcara,
		WitnessID:        a.WitnessID,
		Witness:          toWitnessResponse(a.Witness),
		BeritaAcaraNotes: a.BeritaAcaraNotes,
	}
}

func (s *activityService) GetDailyActivity(ctx context.Context, userID uint, id uint) (*response.DailyActivityResponse, error) {
	activity, err := s.repo.FindActiveByID(ctx, id)
	if err != nil || activity == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "Daily activity not found")
	}

	if activity.UserID != userID {
		return nil, domain.NewUserError(domain.ErrForbidden, "Forbidden: you do not have access to this activity")
	}

	return toDailyActivityResponse(activity), nil
}

func (s *activityService) ListActivities(ctx context.Context, userID uint, filter repository.ActivityFilter) ([]response.DailyActivityResponse, response.PaginationMeta, error) {
	activities, totalRows, err := s.repo.ListActiveByUser(ctx, userID, filter)
	if err != nil {
		return nil, response.PaginationMeta{}, err
	}

	respList := make([]response.DailyActivityResponse, len(activities))
	for i := range activities {
		respList[i] = *toDailyActivityResponse(&activities[i])
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

func (s *activityService) GetPreviousDailyActivity(ctx context.Context, userID uint, targetDate string) (*response.DailyActivityResponse, error) {
	loc := jakartaLocation()
	var refDate time.Time

	trimmed := strings.TrimSpace(targetDate)
	if trimmed == "" {
		if s.nowFunc != nil {
			refDate = s.nowFunc().In(loc)
		} else {
			refDate = time.Now().In(loc)
		}
		refDate = time.Date(refDate.Year(), refDate.Month(), refDate.Day(), 0, 0, 0, 0, loc)
	} else {
		parsed, err := time.ParseInLocation(dateFormatYYYYMMDD, trimmed, loc)
		if err != nil {
			return nil, domain.NewUserError(domain.ErrInvalidInput, "Invalid date format, expected YYYY-MM-DD")
		}
		refDate = parsed
	}

	activity, err := s.repo.FindPreviousActiveByUserAndDate(ctx, userID, refDate)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewUserError(domain.ErrNotFound, "No previous activity found")
		}
		return nil, err
	}
	if activity == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "No previous activity found")
	}

	res := toDailyActivityResponse(activity)
	res.StartTime = ""
	res.EndTime = ""
	res.IsBeritaAcara = false
	res.WitnessID = nil
	res.Witness = nil
	res.BeritaAcaraNotes = ""
	return res, nil
}

func (s *activityService) GetEligibleWitnesses(ctx context.Context, currentUserID uint) ([]response.WitnessResponse, error) {
	users, err := s.repo.GetEligibleWitnesses(ctx, currentUserID)
	if err != nil {
		return nil, err
	}
	res := make([]response.WitnessResponse, len(users))
	for i, u := range users {
		res[i] = *toWitnessResponse(&u)
	}
	return res, nil
}

func (s *activityService) GetMonthlyWitness(ctx context.Context, userID uint, month, year int) (*response.WitnessResponse, error) {
	w, err := s.repo.GetMonthlyWitness(ctx, userID, month, year)
	if err != nil {
		return nil, err
	}
	return toWitnessResponse(w), nil
}

func (s *activityService) UpdateMonthlyWitness(ctx context.Context, userID uint, month, year int, newWitnessID uint) (int64, *models.User, error) {
	if month < 1 || month > 12 {
		return 0, nil, domain.NewUserError(domain.ErrInvalidInput, "Month must be between 1 and 12")
	}
	if year < 2000 {
		return 0, nil, domain.NewUserError(domain.ErrInvalidInput, "Invalid year")
	}
	if newWitnessID == userID {
		return 0, nil, domain.NewUserError(domain.ErrInvalidInput, "Cannot select yourself as a berita acara witness")
	}
	witness, err := s.repo.FindWitnessByID(ctx, newWitnessID)
	if err != nil || witness == nil {
		return 0, nil, domain.NewUserError(domain.ErrInvalidInput, "Witness not found or inactive")
	}
	if witness.Role != models.RoleUser {
		return 0, nil, domain.NewUserError(domain.ErrInvalidInput, "Witness must be a user with 'user' role")
	}
	if s.subRepo != nil {
		if baLocked, _ := s.subRepo.IsBeritaAcaraLocked(ctx, userID, month, year); baLocked {
			return 0, nil, domain.NewUserError(domain.ErrLockedPeriod, "Berita Acara bulan ini telah diajukan/ditandatangani. Batalkan pengajuan terlebih dahulu untuk mengubah saksi.")
		}
	}
	rows, err := s.repo.UpdateMonthlyWitness(ctx, userID, month, year, newWitnessID)
	if err != nil {
		return 0, nil, err
	}
	return rows, witness, nil
}
