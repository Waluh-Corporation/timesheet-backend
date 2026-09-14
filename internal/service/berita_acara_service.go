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

var indonesianDayNames = map[time.Weekday]string{
	time.Sunday:    "Minggu",
	time.Monday:    "Senin",
	time.Tuesday:   "Selasa",
	time.Wednesday: "Rabu",
	time.Thursday:  "Kamis",
	time.Friday:    "Jumat",
	time.Saturday:  "Sabtu",
}

// IndonesianDayName returns the Indonesian day name for a given date.
func IndonesianDayName(t time.Time) string {
	return indonesianDayNames[t.Weekday()]
}

// BeritaAcaraService provides business logic for Berita Acara entries.
type BeritaAcaraService interface {
	UpsertBeritaAcara(ctx context.Context, userID uint, req *request.BeritaAcaraRequest) error
	GetBeritaAcara(ctx context.Context, userID uint, id uint) (*response.BeritaAcaraDetailResponse, error)
	ListBeritaAcara(ctx context.Context, userID uint, filter repository.BeritaAcaraFilter) ([]response.BeritaAcaraResponse, response.PaginationMeta, error)
	FindActiveForMonth(ctx context.Context, userID uint, year, month int) ([]models.BeritaAcara, error)
}

type beritaAcaraService struct {
	repo repository.BeritaAcaraRepository
}

// NewBeritaAcaraService instantiates a new BeritaAcaraService.
func NewBeritaAcaraService(repo repository.BeritaAcaraRepository) BeritaAcaraService {
	return &beritaAcaraService{repo: repo}
}

func (s *beritaAcaraService) UpsertBeritaAcara(ctx context.Context, userID uint, req *request.BeritaAcaraRequest) error {
	date, err := time.ParseInLocation(dateFormatYYYYMMDD, req.Date, jakartaLocation())
	if err != nil {
		return fmt.Errorf("%w: invalid date format, expected YYYY-MM-DD", domain.ErrInvalidInput)
	}

	if strings.TrimSpace(req.Keterangan) == "" {
		return fmt.Errorf("%w: keterangan cannot be empty", domain.ErrInvalidInput)
	}

	day := strings.TrimSpace(req.Day)
	if day == "" {
		day = IndonesianDayName(date)
	}

	existing, err := s.repo.FindActiveByUserAndDate(ctx, userID, date)
	if err == nil && existing != nil && existing.ID != 0 {
		// Update existing record
		existing.Day = day
		existing.StartTime = req.StartTime
		existing.EndTime = req.EndTime
		existing.Keterangan = req.Keterangan
		existing.DailyActivityID = req.DailyActivityID
		existing.TeamLeaderID = req.TeamLeaderID
		existing.DepartmentHeadID = req.DepartmentHeadID
		existing.UpdatedAt = time.Now()
		return s.repo.Update(ctx, existing)
	}

	item := models.BeritaAcara{
		UserID:           userID,
		DailyActivityID:  req.DailyActivityID,
		Date:             date,
		Day:              day,
		StartTime:        req.StartTime,
		EndTime:          req.EndTime,
		Keterangan:       req.Keterangan,
		TeamLeaderID:     req.TeamLeaderID,
		DepartmentHeadID: req.DepartmentHeadID,
		IsActive:         true,
	}

	return s.repo.Create(ctx, &item)
}

func (s *beritaAcaraService) GetBeritaAcara(ctx context.Context, userID uint, id uint) (*response.BeritaAcaraDetailResponse, error) {
	item, err := s.repo.FindActiveByID(ctx, id)
	if err != nil || item == nil {
		return nil, domain.ErrNotFound
	}

	if item.UserID != userID {
		return nil, domain.ErrForbidden
	}

	res := &response.BeritaAcaraDetailResponse{
		ID:               item.ID,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		UserID:           item.UserID,
		Date:             item.Date,
		Day:              item.Day,
		StartTime:        item.StartTime,
		EndTime:          item.EndTime,
		Keterangan:       item.Keterangan,
		DailyActivityID:  item.DailyActivityID,
		TeamLeaderID:     item.TeamLeaderID,
		DepartmentHeadID: item.DepartmentHeadID,
	}

	if item.DailyActivity != nil {
		res.DailyActivity = &response.DailyActivityResponse{
			ID:          item.DailyActivity.ID,
			Date:        item.DailyActivity.Date,
			StartTime:   item.DailyActivity.StartTime,
			EndTime:     item.DailyActivity.EndTime,
			Activity:    item.DailyActivity.Activity,
			ProjectName: item.DailyActivity.ProjectName,
			ProjectID:   item.DailyActivity.ProjectID,
			Status:      item.DailyActivity.Status,
		}
	}

	if item.TeamLeader != nil {
		res.TeamLeader = item.TeamLeader
	}

	if item.DepartmentHead != nil {
		res.DepartmentHead = item.DepartmentHead
	}

	return res, nil
}

func (s *beritaAcaraService) ListBeritaAcara(ctx context.Context, userID uint, filter repository.BeritaAcaraFilter) ([]response.BeritaAcaraResponse, response.PaginationMeta, error) {
	items, totalRows, err := s.repo.ListActiveByUser(ctx, userID, filter)
	if err != nil {
		return nil, response.PaginationMeta{}, err
	}

	list := make([]response.BeritaAcaraResponse, 0, len(items))
	for _, it := range items {
		var tlName, dhName string
		if it.TeamLeader != nil {
			tlName = it.TeamLeader.Name
		}
		if it.DepartmentHead != nil {
			dhName = it.DepartmentHead.Name
		}

		list = append(list, response.BeritaAcaraResponse{
			ID:                 it.ID,
			Date:               it.Date,
			Day:                it.Day,
			StartTime:          it.StartTime,
			EndTime:            it.EndTime,
			Keterangan:         it.Keterangan,
			DailyActivityID:    it.DailyActivityID,
			TeamLeaderID:       it.TeamLeaderID,
			TeamLeaderName:     tlName,
			DepartmentHeadID:   it.DepartmentHeadID,
			DepartmentHeadName: dhName,
		})
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	totalPages := 1
	if limit > 0 && totalRows > 0 {
		totalPages = int(math.Ceil(float64(totalRows) / float64(limit)))
	}
	if filter.IsAll {
		page = 1
		limit = int(totalRows)
		totalPages = 1
	}

	meta := response.PaginationMeta{
		Page:       page,
		Limit:      limit,
		TotalRows:  totalRows,
		TotalPages: totalPages,
	}

	return list, meta, nil
}

func (s *beritaAcaraService) FindActiveForMonth(ctx context.Context, userID uint, year, month int) ([]models.BeritaAcara, error) {
	return s.repo.FindActiveForMonth(ctx, userID, year, month)
}
