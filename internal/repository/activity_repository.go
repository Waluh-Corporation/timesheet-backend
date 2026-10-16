package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/models"
)

// ActivityFilter encapsulates criteria for querying daily activities.
type ActivityFilter struct {
	StartDate             *time.Time
	EndDate               *time.Time
	Month                 *int
	Year                  *int
	Status                string
	SortOrder             string
	Page                  int
	Limit                 int
	IsAll                 bool
	IsCurrentMonthDefault bool
}

// ActivityRepository defines the database persistence contract for DailyActivity and related lookups.
type ActivityRepository interface {
	FindActiveByID(ctx context.Context, id uint) (*models.DailyActivity, error)
	FindActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.DailyActivity, error)
	FindPreviousActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.DailyActivity, error)
	Create(ctx context.Context, activity *models.DailyActivity) error
	Update(ctx context.Context, activity *models.DailyActivity) error
	ListActiveByUser(ctx context.Context, userID uint, filter ActivityFilter) ([]models.DailyActivity, int64, error)
	ValidateStatus(ctx context.Context, status string) (bool, error)
	FindActiveProjectByRefID(ctx context.Context, refID uint) (*models.Project, error)
	FindActiveProjectByCodeOrName(ctx context.Context, projectID, projectName string) (*models.Project, error)
	GetLatestActivityUpdateTime(ctx context.Context, userID uint, month, year int) (time.Time, error)
	GetMonthlyWitness(ctx context.Context, userID uint, month, year int) (*models.User, error)
	UpdateMonthlyWitness(ctx context.Context, userID uint, month, year int, newWitnessID uint) (int64, error)
	GetEligibleWitnesses(ctx context.Context, excludeUserID uint) ([]models.User, error)
	FindWitnessByID(ctx context.Context, id uint) (*models.User, error)
}

type activityRepository struct {
	db *gorm.DB
}

// NewActivityRepository constructs an instance of ActivityRepository.
func NewActivityRepository(db *gorm.DB) ActivityRepository {
	return &activityRepository{db: db}
}

func (r *activityRepository) FindActiveByID(ctx context.Context, id uint) (*models.DailyActivity, error) {
	var activity models.DailyActivity
	if err := r.db.WithContext(ctx).Preload("ProjectRef", models.ActiveOnly).Preload("StatusRef").Preload("Witness").Where("id = ? AND is_active = true", id).First(&activity).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *activityRepository) FindActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.DailyActivity, error) {
	var activity models.DailyActivity
	if err := r.db.WithContext(ctx).Preload("ProjectRef", models.ActiveOnly).Preload("StatusRef").Preload("Witness").Where("user_id = ? AND date = ? AND is_active = true", userID, date).First(&activity).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *activityRepository) FindPreviousActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.DailyActivity, error) {
	var activity models.DailyActivity
	if err := r.db.WithContext(ctx).Preload("ProjectRef", models.ActiveOnly).Preload("StatusRef").Preload("Witness").
		Where("user_id = ? AND date < ? AND is_active = true", userID, date).
		Order("date desc").
		First(&activity).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *activityRepository) Create(ctx context.Context, activity *models.DailyActivity) error {
	return r.db.WithContext(ctx).Omit(clause.Associations).Create(activity).Error
}

func (r *activityRepository) Update(ctx context.Context, activity *models.DailyActivity) error {
	return r.db.WithContext(ctx).Omit(clause.Associations).Save(activity).Error
}

func jakartaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

const (
	queryDateBetween = "date >= ? AND date < ?"
)

func resolvePeriodRange(filter ActivityFilter, loc *time.Location) (time.Time, time.Time, bool) {
	if filter.Month != nil || filter.Year != nil {
		year := time.Now().In(loc).Year()
		if filter.Year != nil {
			year = *filter.Year
		}
		if filter.Month != nil {
			start := time.Date(year, time.Month(*filter.Month), 1, 0, 0, 0, 0, loc)
			return start, start.AddDate(0, 1, 0), true
		}
		start := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
		return start, start.AddDate(1, 0, 0), true
	}
	if filter.IsCurrentMonthDefault {
		now := time.Now().In(loc)
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		return start, start.AddDate(0, 1, 0), true
	}
	return time.Time{}, time.Time{}, false
}

func (r *activityRepository) ListActiveByUser(ctx context.Context, userID uint, filter ActivityFilter) ([]models.DailyActivity, int64, error) {
	loc := jakartaLocation()
	query := r.db.WithContext(ctx).Model(&models.DailyActivity{}).Where("user_id = ? AND is_active = true", userID)

	if filter.StartDate != nil {
		query = query.Where("date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("date <= ?", *filter.EndDate)
	}
	if start, end, ok := resolvePeriodRange(filter, loc); ok {
		query = query.Where(queryDateBetween, start, end)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var totalRows int64
	if err := query.Count(&totalRows).Error; err != nil {
		return nil, 0, err
	}

	sortOrder := filter.SortOrder
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
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

	activities := make([]models.DailyActivity, 0)
	if err := dataQuery.Preload("ProjectRef", models.ActiveOnly).Preload("StatusRef").Preload("Witness").Find(&activities).Error; err != nil {
		return nil, 0, err
	}

	return activities, totalRows, nil
}

func (r *activityRepository) ValidateStatus(ctx context.Context, status string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.ActivityStatus{}).Where("code = ?", status).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *activityRepository) FindActiveProjectByRefID(ctx context.Context, refID uint) (*models.Project, error) {
	var proj models.Project
	if err := r.db.WithContext(ctx).Scopes(models.ActiveOnly).Where("id = ?", refID).First(&proj).Error; err != nil {
		return nil, err
	}
	if proj.ID == 0 {
		return nil, errors.New("project not found or inactive")
	}
	return &proj, nil
}

func (r *activityRepository) FindActiveProjectByCodeOrName(ctx context.Context, projectID, projectName string) (*models.Project, error) {
	query := r.db.WithContext(ctx).Model(&models.Project{}).Scopes(models.ActiveOnly)
	if projectID != "" && projectName != "" {
		if idNum, err := strconv.Atoi(projectID); err == nil && idNum > 0 {
			query = query.Where("(id = ? OR code = ?) AND LOWER(name) = LOWER(?)", idNum, projectID, projectName)
		} else {
			query = query.Where("code = ? AND LOWER(name) = LOWER(?)", projectID, projectName)
		}
	} else if projectID != "" {
		if idNum, err := strconv.Atoi(projectID); err == nil && idNum > 0 {
			query = query.Where("id = ? OR code = ?", idNum, projectID)
		} else {
			query = query.Where("code = ?", projectID)
		}
	} else if projectName != "" {
		query = query.Where("LOWER(name) = LOWER(?)", projectName)
	}

	var proj models.Project
	if err := query.Limit(1).Find(&proj).Error; err != nil {
		return nil, err
	}
	if proj.ID == 0 {
		return nil, errors.New("project not found")
	}
	return &proj, nil
}

func (r *activityRepository) GetLatestActivityUpdateTime(ctx context.Context, userID uint, month, year int) (time.Time, error) {
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)
	var latest *time.Time
	err := r.db.WithContext(ctx).Model(&models.DailyActivity{}).
		Where("user_id = ? AND date >= ? AND date < ? AND is_active = true", userID, startDate, endDate).
		Select("MAX(updated_at)").
		Scan(&latest).Error
	if err != nil {
		return time.Time{}, err
	}
	if latest == nil {
		return time.Time{}, nil
	}
	return *latest, nil
}

func (r *activityRepository) GetMonthlyWitness(ctx context.Context, userID uint, month, year int) (*models.User, error) {
	loc := jakartaLocation()
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	var activity models.DailyActivity
	err := r.db.WithContext(ctx).
		Preload("Witness.DepartmentRel").
		Where("user_id = ? AND is_active = true AND is_berita_acara = true AND witness_id IS NOT NULL", userID).
		Where(queryDateBetween, startOfMonth, startOfNextMonth).
		Order("date asc").
		First(&activity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return activity.Witness, nil
}

func (r *activityRepository) UpdateMonthlyWitness(ctx context.Context, userID uint, month, year int, newWitnessID uint) (int64, error) {
	loc := jakartaLocation()
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	res := r.db.WithContext(ctx).
		Model(&models.DailyActivity{}).
		Where("user_id = ? AND is_active = true AND is_berita_acara = true", userID).
		Where(queryDateBetween, startOfMonth, startOfNextMonth).
		Update("witness_id", newWitnessID)
	return res.RowsAffected, res.Error
}

func (r *activityRepository) GetEligibleWitnesses(ctx context.Context, excludeUserID uint) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).
		Preload("DepartmentRel").
		Where("id != ? AND is_active = true AND role = ?", excludeUserID, models.RoleUser).
		Order("name asc").
		Find(&users).Error
	return users, err
}

func (r *activityRepository) FindWitnessByID(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Preload("DepartmentRel").Where("id = ? AND is_active = true", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
