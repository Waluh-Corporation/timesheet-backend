package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

// TimesheetService defines business operations for generating timesheets and managing overtime records.
type TimesheetService interface {
	GenerateWorkbook(ctx context.Context, userID uint, month int, year int) ([]byte, string, error)
	UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error
	ListMonthlyOvertimes(ctx context.Context, userID uint, month int, year int) ([]response.OvertimeResponse, error)
	DeleteOvertime(ctx context.Context, id uint, userID uint) error
}

type timesheetService struct {
	db           *gorm.DB
	userRepo     repository.UserRepository
	activityRepo repository.ActivityRepository
	overtimeRepo repository.OvertimeRepository
	mailer       *mailer.Mailer
}

// NewTimesheetService constructs a TimesheetService implementation.
func NewTimesheetService(
	db *gorm.DB,
	userRepo repository.UserRepository,
	activityRepo repository.ActivityRepository,
	overtimeRepo repository.OvertimeRepository,
	m *mailer.Mailer,
) TimesheetService {
	return &timesheetService{
		db:           db,
		userRepo:     userRepo,
		activityRepo: activityRepo,
		overtimeRepo: overtimeRepo,
		mailer:       m,
	}
}

func sanitizeFilename(s string) string {
	var out []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		}
	}
	return string(out)
}

func (s *timesheetService) GenerateWorkbook(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	if month < 1 || month > 12 || year < 2000 || year > 2100 {
		return nil, "", fmt.Errorf("%w: invalid month or year", domain.ErrInvalidInput)
	}
	loc := jakartaLocation()
	var user models.User
	if err := s.db.WithContext(ctx).Scopes(models.ActiveOnly).
		Preload("CompanyRel").
		Preload("SiteRel").
		Preload("DepartmentRel").
		Preload("DivisionRel").
		Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, "", fmt.Errorf("%w: user not found", domain.ErrNotFound)
	}

	companyCode := ""
	companyName := ""
	if user.CompanyRel != nil {
		companyCode = user.CompanyRel.Code
		companyName = user.CompanyRel.Name
	} else if user.CompanyID != nil && *user.CompanyID != 0 {
		var comp models.Company
		if err := s.db.WithContext(ctx).Where("id = ?", *user.CompanyID).First(&comp).Error; err == nil {
			companyCode = comp.Code
			companyName = comp.Name
		}
	}
	if companyCode == "" && user.Company != "" {
		companyCode = user.Company
		companyName = user.Company
	}
	if companyCode == "" {
		return nil, "", fmt.Errorf("%w: user has no company assigned", domain.ErrInvalidInput)
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	var activities []models.DailyActivity
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND date >= ? AND date < ?", user.ID, start, end).
		Scopes(models.ActiveOnly).
		Preload("ProjectRef", models.ActiveOnly).
		Preload("StatusRef").
		Order("date asc").
		Find(&activities).Error; err != nil {
		return nil, "", err
	}

	overtimes, err := s.overtimeRepo.FindByUserAndMonth(ctx, user.ID, start, end)
	if err != nil {
		return nil, "", err
	}

	holidays := map[int]string{}
	if hs, herr := services.FetchHolidays(year, month); herr == nil {
		for _, h := range hs {
			var y, m, d int
			if _, e := fmt.Sscanf(h.Date, "%d-%d-%d", &y, &m, &d); e == nil {
				holidays[d] = h.Description
			}
		}
	}

	var approvers []models.Approver
	_ = s.db.WithContext(ctx).Scopes(models.ActiveOnly).Order("id asc").Find(&approvers).Error

	out, err := services.GenerateFromTemplate(services.GenerationInput{
		CompanyCode: companyCode,
		User:        &user,
		Month:       month,
		Year:        year,
		Activities:  activities,
		Overtimes:   overtimes,
		Approvers:   approvers,
		Holidays:    holidays,
	})
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", sanitizeFilename(user.Username), month, year)
	period := mailer.FormatMonthYearIndonesian(month, year)

	// Asynchronous notification email if mailer is configured
	if s.mailer != nil {
		go func(to, uname, comp, per, fn string, data []byte) {
			_ = s.mailer.SendTimesheetEmailWithDetails(to, uname, comp, per, fn, data)
		}(user.Email, user.Username, companyName, period, filename, out)
	}

	return out, filename, nil
}

func (s *timesheetService) UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error {
	if req == nil {
		return fmt.Errorf("%w: request is required", domain.ErrInvalidInput)
	}
	loc := jakartaLocation()
	date, err := time.ParseInLocation(dateFormatYYYYMMDD, req.Date, loc)
	if err != nil {
		return fmt.Errorf("%w: invalid date format, expected YYYY-MM-DD", domain.ErrInvalidInput)
	}

	var entry *models.OvertimeEntry
	if req.ID != 0 {
		entry, _ = s.overtimeRepo.FindActiveByID(ctx, req.ID, userID)
	}
	if entry == nil {
		entry, _ = s.overtimeRepo.FindByUserAndDate(ctx, userID, date)
	}

	if entry != nil && entry.ID != 0 {
		entry.Date = date
		entry.StartTime = req.StartTime
		entry.EndTime = req.EndTime
		entry.TaskDescription = req.TaskDescription
		entry.TeamLeaderID = req.TeamLeaderID
		entry.DepartmentHeadID = req.DepartmentHeadID
		entry.UpdatedAt = time.Now()
		return s.overtimeRepo.Update(ctx, entry)
	}

	newEntry := &models.OvertimeEntry{
		UserID:           userID,
		Date:             date,
		StartTime:        req.StartTime,
		EndTime:          req.EndTime,
		TaskDescription:  req.TaskDescription,
		TeamLeaderID:     req.TeamLeaderID,
		DepartmentHeadID: req.DepartmentHeadID,
		IsActive:         true,
	}
	return s.overtimeRepo.Create(ctx, newEntry)
}

func (s *timesheetService) ListMonthlyOvertimes(ctx context.Context, userID uint, month int, year int) ([]response.OvertimeResponse, error) {
	loc := jakartaLocation()
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	overtimes, err := s.overtimeRepo.FindByUserAndMonth(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	resp := make([]response.OvertimeResponse, len(overtimes))
	for i, ot := range overtimes {
		var tlName, dhName string
		if ot.TeamLeader != nil {
			tlName = ot.TeamLeader.Name
		}
		if ot.DepartmentHead != nil {
			dhName = ot.DepartmentHead.Name
		}
		resp[i] = response.OvertimeResponse{
			ID:                 ot.ID,
			Date:               ot.Date,
			StartTime:          ot.StartTime,
			EndTime:            ot.EndTime,
			TaskDescription:    ot.TaskDescription,
			TeamLeaderID:       ot.TeamLeaderID,
			TeamLeaderName:     tlName,
			DepartmentHeadID:   ot.DepartmentHeadID,
			DepartmentHeadName: dhName,
		}
	}
	return resp, nil
}

func (s *timesheetService) DeleteOvertime(ctx context.Context, id uint, userID uint) error {
	entry, err := s.overtimeRepo.FindActiveByID(ctx, id, userID)
	if err != nil || entry == nil {
		return domain.ErrNotFound
	}
	return s.overtimeRepo.SoftDelete(ctx, id, userID)
}
