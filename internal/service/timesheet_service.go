package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

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
	userRepo     repository.UserRepository
	activityRepo repository.ActivityRepository
	overtimeRepo repository.OvertimeRepository
	masterRepo   repository.MasterRepository
	mailer       *mailer.Mailer
	nowFunc      func() time.Time
}

// NewTimesheetService constructs a TimesheetService implementation.
func NewTimesheetService(
	userRepo repository.UserRepository,
	activityRepo repository.ActivityRepository,
	overtimeRepo repository.OvertimeRepository,
	masterRepo repository.MasterRepository,
	m *mailer.Mailer,
) TimesheetService {
	return &timesheetService{
		userRepo:     userRepo,
		activityRepo: activityRepo,
		overtimeRepo: overtimeRepo,
		masterRepo:   masterRepo,
		mailer:       m,
		nowFunc:      time.Now,
	}
}

// SetNowFunc overrides the time provider for testing purposes.
func (s *timesheetService) SetNowFunc(fn func() time.Time) {
	s.nowFunc = fn
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
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "Invalid month or year")
	}
	loc := jakartaLocation()
	now := time.Now()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}

	if !domain.IsAdminFromContext(ctx) {
		if err := domain.ValidateTimesheetPeriod(month, year, now, loc); err != nil {
			return nil, "", err
		}
	}

	user, err := s.userRepo.FindByIDWithDetails(ctx, userID)
	if err != nil || user == nil {
		return nil, "", domain.NewUserError(domain.ErrNotFound, "User not found")
	}

	companyCode := ""
	if user.CompanyRel != nil {
		companyCode = user.CompanyRel.Code
	} else if user.CompanyID != nil && *user.CompanyID != 0 && s.masterRepo != nil {
		if comp, cerr := s.masterRepo.FindCompanyByID(ctx, *user.CompanyID); cerr == nil && comp != nil {
			companyCode = comp.Code
		}
	}
	if companyCode == "" && user.Company != "" {
		companyCode = user.Company
	}
	if companyCode == "" {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "User has no company assigned")
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	filter := repository.ActivityFilter{
		Month:     &month,
		Year:      &year,
		SortOrder: "asc",
		IsAll:     true,
	}
	activities, _, err := s.activityRepo.ListActiveByUser(ctx, user.ID, filter)
	if err != nil {
		return nil, "", err
	}
	if len(activities) == 0 {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "No activities recorded for this period")
	}

	overtimes, err := s.overtimeRepo.FindByUserAndMonth(ctx, user.ID, start, end)
	if err != nil {
		return nil, "", err
	}

	holidays := map[int]string{}
	if s.masterRepo != nil {
		if hs, herr := s.masterRepo.ListHolidaysByMonth(ctx, year, month); herr == nil {
			for _, h := range hs {
				holidays[h.Date.Day()] = h.Description
			}
		}
	}

	var approvers []models.Approver
	if s.masterRepo != nil {
		activeStatus := true
		approvers, _ = s.masterRepo.ListApprovers(ctx, "", &activeStatus)
	}

	excelBytes, err := services.GenerateFromTemplate(services.GenerationInput{
		CompanyCode: companyCode,
		User:        user,
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

	// Detect if there are any Berita Acara entries in the month
	baActivities := make([]models.DailyActivity, 0)
	var witnessID *uint
	for _, a := range activities {
		if a.IsBeritaAcara || strings.EqualFold(a.Status, "BA") {
			baActivities = append(baActivities, a)
			if a.WitnessID == nil || *a.WitnessID == 0 {
				return nil, "", domain.NewUserError(domain.ErrInvalidInput, "A witness is required for all Berita Acara entries")
			}
			if witnessID == nil {
				witnessID = a.WitnessID
			} else if *witnessID != *a.WitnessID {
				return nil, "", domain.NewUserError(domain.ErrInvalidInput, "All Berita Acara entries in the month must have the same witness")
			}
		}
	}

	excelFilename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", sanitizeFilename(user.Username), month, year)

	// If no Berita Acara entries, return Excel directly
	if len(baActivities) == 0 {
		return excelBytes, excelFilename, nil
	}

	// Resolve witness for Berita Acara
	var witness *models.User
	if witnessID != nil && s.userRepo != nil {
		witness, _ = s.userRepo.FindByIDWithDetails(ctx, *witnessID)
	}
	if witness == nil || witness.Role != models.RoleUser {
		return nil, "", domain.NewUserError(domain.ErrInvalidInput, "Selected witness for Berita Acara is invalid or not an active user")
	}

	docxBytes, err := services.GenerateBeritaAcara(services.BeritaAcaraInput{
		User:       user,
		Witness:    witness,
		Month:      month,
		Year:       year,
		Activities: baActivities,
		Approvers:  approvers,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate berita acara: %w", err)
	}

	docxFilename := fmt.Sprintf("Berita_Acara_%s_%02d_%04d.docx", sanitizeFilename(user.Username), month, year)

	// Bundle Excel and Word into a ZIP archive
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	ew, err := zw.Create(excelFilename)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create zip entry for timesheet: %w", err)
	}
	if _, err := ew.Write(excelBytes); err != nil {
		return nil, "", fmt.Errorf("failed to write timesheet to zip: %w", err)
	}

	dw, err := zw.Create(docxFilename)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create zip entry for berita acara: %w", err)
	}
	if _, err := dw.Write(docxBytes); err != nil {
		return nil, "", fmt.Errorf("failed to write berita acara to zip: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize zip archive: %w", err)
	}

	zipFilename := fmt.Sprintf("Timesheet_and_Berita_Acara_%s_%02d_%04d.zip", sanitizeFilename(user.Username), month, year)
	return zipBuf.Bytes(), zipFilename, nil
}

func (s *timesheetService) UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error {
	if req == nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Request payload is required")
	}
	if err := validateWorkingHours(req.StartTime, req.EndTime); err != nil {
		return err
	}
	loc := jakartaLocation()
	date, err := time.ParseInLocation(dateFormatYYYYMMDD, req.Date, loc)
	if err != nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Invalid date format, expected YYYY-MM-DD")
	}

	now := time.Now()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}

	if !domain.IsAdminFromContext(ctx) {
		if err := domain.ValidateTimesheetDate(date, now, loc); err != nil {
			return err
		}
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
		return domain.NewUserError(domain.ErrNotFound, "Overtime entry not found")
	}

	now := time.Now()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}

	if !domain.IsAdminFromContext(ctx) {
		if err := domain.ValidateTimesheetDate(entry.Date, now, jakartaLocation()); err != nil {
			return err
		}
	}
	return s.overtimeRepo.SoftDelete(ctx, id, userID)
}
