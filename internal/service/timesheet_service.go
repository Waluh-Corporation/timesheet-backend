package service

import (
	"context"
	"fmt"
	"strings"
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
	GetHistoricalSummary(ctx context.Context, userID uint, year int, month *int) (*response.TimesheetSummaryResponse, error)
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
	if len(activities) == 0 {
		return nil, "", fmt.Errorf("%w: timesheet belum dapat dibuat karena belum ada aktivitas yang tercatat pada periode ini. Silakan isi aktivitas harian Anda terlebih dahulu sebelum mengunduh timesheet", domain.ErrInvalidInput)
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
	if err := validateWorkingHours(req.StartTime, req.EndTime); err != nil {
		return err
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

func parseDurationHours(startStr, endStr string) float64 {
	start, err1 := time.Parse("15:04", strings.TrimSpace(startStr))
	end, err2 := time.Parse("15:04", strings.TrimSpace(endStr))
	if err1 != nil || err2 != nil {
		return 0
	}
	diff := end.Sub(start).Hours()
	if diff < 0 {
		diff += 24
	}
	return diff
}

func (s *timesheetService) GetHistoricalSummary(ctx context.Context, userID uint, year int, month *int) (*response.TimesheetSummaryResponse, error) {
	if year < 2000 || year > 2100 {
		return nil, fmt.Errorf("%w: invalid year, must be between 2000 and 2100", domain.ErrInvalidInput)
	}
	if month != nil && (*month < 1 || *month > 12) {
		return nil, fmt.Errorf("%w: invalid month, must be between 1 and 12", domain.ErrInvalidInput)
	}

	if _, err := s.userRepo.FindByID(ctx, userID); err != nil {
		return nil, fmt.Errorf("%w: user not found", domain.ErrNotFound)
	}

	loc := jakartaLocation()
	var start, end time.Time
	var monthsToProcess []int
	if month != nil {
		monthsToProcess = []int{*month}
		start = time.Date(year, time.Month(*month), 1, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 1, 0)
	} else {
		monthsToProcess = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
		start = time.Date(year, 1, 1, 0, 0, 0, 0, loc)
		end = time.Date(year+1, 1, 1, 0, 0, 0, 0, loc)
	}

	var allActivities []models.DailyActivity
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Scopes(models.ActiveOnly).
		Order("date asc").
		Find(&allActivities).Error; err != nil {
		return nil, err
	}

	var allOvertimes []models.OvertimeEntry
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Scopes(models.ActiveOnly).
		Order("date asc").
		Find(&allOvertimes).Error; err != nil {
		return nil, err
	}

	activitiesByMonth := make(map[int][]models.DailyActivity)
	for _, act := range allActivities {
		m := int(act.Date.Month())
		activitiesByMonth[m] = append(activitiesByMonth[m], act)
	}

	overtimesByMonth := make(map[int][]models.OvertimeEntry)
	for _, ot := range allOvertimes {
		m := int(ot.Date.Month())
		overtimesByMonth[m] = append(overtimesByMonth[m], ot)
	}

	monthlySummaries := make([]response.MonthlySummaryDTO, 0, len(monthsToProcess))
	totalWorkingDays := 0
	totalDaysFilled := 0
	totalWorkingHours := 0.0
	totalOvertimeHours := 0.0
	yearlyAttendance := make(map[string]int)

	for _, m := range monthsToProcess {
		holidayMap := make(map[int]string)
		if hs, herr := services.FetchHolidays(year, m); herr == nil {
			for _, h := range hs {
				var hy, hm, hd int
				if _, e := fmt.Sscanf(h.Date, "%d-%d-%d", &hy, &hm, &hd); e == nil {
					holidayMap[hd] = h.Description
				}
			}
		}

		daysInMonth := services.GetDaysInMonth(year, m)
		workingDays := 0
		for day := 1; day <= daysInMonth; day++ {
			d := time.Date(year, time.Month(m), day, 0, 0, 0, 0, time.UTC)
			if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday && holidayMap[day] == "" {
				workingDays++
			}
		}

		mActs := activitiesByMonth[m]
		mOts := overtimesByMonth[m]

		filledDates := make(map[int]bool)
		mWorkingHours := 0.0
		mBreakdown := map[string]int{
			"P": 0, "S": 0, "V": 0, "PM": 0, "BT": 0, "X": 0,
		}

		for _, act := range mActs {
			filledDates[act.Date.Day()] = true
			st := strings.ToUpper(strings.TrimSpace(act.Status))
			if st != "" {
				mBreakdown[st]++
				yearlyAttendance[st]++
			}
			if act.StartTime != "" && act.EndTime != "" {
				mWorkingHours += parseDurationHours(act.StartTime, act.EndTime)
			} else if st == "P" {
				mWorkingHours += 8.0
			}
		}

		mOvertimeHours := 0.0
		for _, ot := range mOts {
			if ot.StartTime != "" && ot.EndTime != "" {
				mOvertimeHours += parseDurationHours(ot.StartTime, ot.EndTime)
			}
		}

		daysFilled := len(filledDates)
		isComplete := daysFilled >= workingDays && workingDays > 0

		totalWorkingDays += workingDays
		totalDaysFilled += daysFilled
		totalWorkingHours += mWorkingHours
		totalOvertimeHours += mOvertimeHours

		monthlySummaries = append(monthlySummaries, response.MonthlySummaryDTO{
			Month:               m,
			MonthName:           services.MonthNameIndonesian(m),
			WorkingDays:         workingDays,
			DaysFilled:          daysFilled,
			IsComplete:          isComplete,
			WorkingHours:        mWorkingHours,
			OvertimeHours:       mOvertimeHours,
			AttendanceBreakdown: mBreakdown,
		})
	}

	return &response.TimesheetSummaryResponse{
		Year:                      year,
		Month:                     month,
		TotalWorkingDays:          totalWorkingDays,
		TotalDaysFilled:           totalDaysFilled,
		TotalWorkingHours:         totalWorkingHours,
		TotalOvertimeHours:        totalOvertimeHours,
		YearlyAttendanceBreakdown: yearlyAttendance,
		Months:                    monthlySummaries,
	}, nil
}
