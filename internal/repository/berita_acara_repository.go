package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/models"
)

// BeritaAcaraFilter encapsulates search and pagination criteria for Berita Acara.
type BeritaAcaraFilter struct {
	StartDate             *time.Time
	EndDate               *time.Time
	Month                 *int
	Year                  *int
	SortOrder             string
	Page                  int
	Limit                 int
	IsAll                 bool
	IsCurrentMonthDefault bool
}

// BeritaAcaraRepository defines database operations for Berita Acara.
type BeritaAcaraRepository interface {
	FindActiveByID(ctx context.Context, id uint) (*models.BeritaAcara, error)
	FindActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.BeritaAcara, error)
	Create(ctx context.Context, item *models.BeritaAcara) error
	Update(ctx context.Context, item *models.BeritaAcara) error
	ListActiveByUser(ctx context.Context, userID uint, filter BeritaAcaraFilter) ([]models.BeritaAcara, int64, error)
	FindActiveForMonth(ctx context.Context, userID uint, year, month int) ([]models.BeritaAcara, error)
}

type beritaAcaraRepository struct {
	db *gorm.DB
}

// NewBeritaAcaraRepository instantiates a new BeritaAcaraRepository.
func NewBeritaAcaraRepository(db *gorm.DB) BeritaAcaraRepository {
	return &beritaAcaraRepository{db: db}
}

func (r *beritaAcaraRepository) FindActiveByID(ctx context.Context, id uint) (*models.BeritaAcara, error) {
	var item models.BeritaAcara
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("DailyActivity").
		Preload("TeamLeader").
		Preload("DepartmentHead").
		Where("id = ? AND is_active = true", id).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *beritaAcaraRepository) FindActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.BeritaAcara, error) {
	var item models.BeritaAcara
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND date = ? AND is_active = true", userID, date).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *beritaAcaraRepository) Create(ctx context.Context, item *models.BeritaAcara) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *beritaAcaraRepository) Update(ctx context.Context, item *models.BeritaAcara) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *beritaAcaraRepository) ListActiveByUser(ctx context.Context, userID uint, filter BeritaAcaraFilter) ([]models.BeritaAcara, int64, error) {
	loc := jakartaLocation()
	query := r.db.WithContext(ctx).Model(&models.BeritaAcara{}).
		Preload("TeamLeader").
		Preload("DepartmentHead").
		Where("user_id = ? AND is_active = true", userID)

	if filter.StartDate != nil {
		query = query.Where("date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("date <= ?", *filter.EndDate)
	}

	actFilter := ActivityFilter{
		Month:                 filter.Month,
		Year:                  filter.Year,
		IsCurrentMonthDefault: filter.IsCurrentMonthDefault,
	}
	if start, end, ok := resolvePeriodRange(actFilter, loc); ok {
		query = query.Where(queryDateBetween, start, end)
	}

	var totalRows int64
	if err := query.Count(&totalRows).Error; err != nil {
		return nil, 0, err
	}

	sortOrder := filter.SortOrder
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}
	dataQuery := query.Order("date " + sortOrder)

	if !filter.IsAll && filter.Limit > 0 {
		limit := filter.Limit
		if limit > 100 {
			limit = 100
		}
		offset := (filter.Page - 1) * limit
		dataQuery = dataQuery.Limit(limit).Offset(offset)
	}

	var items []models.BeritaAcara
	if err := dataQuery.Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, totalRows, nil
}

func (r *beritaAcaraRepository) FindActiveForMonth(ctx context.Context, userID uint, year, month int) ([]models.BeritaAcara, error) {
	loc := jakartaLocation()
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	var items []models.BeritaAcara
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND date >= ? AND date < ? AND is_active = true", userID, start, end).
		Preload("TeamLeader").
		Preload("DepartmentHead").
		Order("date asc").
		Find(&items).Error
	return items, err
}
