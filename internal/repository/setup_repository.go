package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/database"
	"timesheet-backend/internal/domain"
	"timesheet-backend/models"
)

// SetupRepository is re-exported from domain.SetupRepository.
type SetupRepository = domain.SetupRepository

type setupRepository struct {
	db *gorm.DB
}

// NewSetupRepository constructs a SetupRepository.
func NewSetupRepository(db *gorm.DB) SetupRepository {
	return &setupRepository{db: db}
}

func (r *setupRepository) GetAdminCount(ctx context.Context) (int64, error) {
	var adminCount int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Where("role = ? AND deleted_at IS NULL", models.RoleAdmin).Count(&adminCount).Error
	return adminCount, err
}

func (r *setupRepository) GetSystemSetting(ctx context.Context, key string) (string, error) {
	var setting models.SystemSetting
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *setupRepository) ExecuteSetup(ctx context.Context, adminUser *models.User, companies []models.Company, depts []models.Department, approvers []models.Approver) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Seed companies
		var existingComps []models.Company
		if err := tx.Find(&existingComps).Error; err != nil {
			return err
		}
		compMap := make(map[string]struct{}, len(existingComps))
		for _, comp := range existingComps {
			compMap[strings.ToLower(comp.Code)] = struct{}{}
		}
		for _, c := range companies {
			code := strings.ToLower(strings.TrimSpace(c.Code))
			if code == "" {
				continue
			}
			if _, exists := compMap[code]; !exists {
				newComp := models.Company{
					Code:     code,
					Name:     strings.TrimSpace(c.Name),
					IsActive: true,
				}
				if err := tx.Create(&newComp).Error; err != nil {
					return err
				}
				compMap[code] = struct{}{}
			}
		}

		// Seed departments
		for _, d := range depts {
			code := strings.TrimSpace(d.Code)
			if code == "" {
				continue
			}
			var count int64
			_ = tx.Model(&models.Department{}).Where("code = ?", code).Count(&count).Error
			if count == 0 {
				newDept := models.Department{
					Code:     code,
					Name:     strings.TrimSpace(d.Name),
					Division: strings.TrimSpace(d.Division),
					IsActive: true,
				}
				if err := tx.Create(&newDept).Error; err != nil {
					return err
				}
			}
		}

		// Seed approvers
		for _, a := range approvers {
			name := strings.TrimSpace(a.Name)
			if name == "" {
				continue
			}
			newAppr := models.Approver{
				Name:     name,
				RoleType: a.RoleType,
				Title:    strings.TrimSpace(a.Title),
				IsActive: true,
			}
			if err := tx.Create(&newAppr).Error; err != nil {
				return err
			}
		}

		// Create admin user
		if err := tx.Create(adminUser).Error; err != nil {
			return err
		}

		// Seed activity statuses
		if err := database.SeedActivityStatuses(tx); err != nil {
			return err
		}

		// Set is_new = N
		return tx.Save(&models.SystemSetting{
			Key:       "is_new",
			Value:     "N",
			UpdatedAt: time.Now(),
		}).Error
	})
}
