package database

import (
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/models"
)

func setupDB(t *testing.T) *gorm.DB {
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}
	return db
}

func TestSetup_SeedFunctions(t *testing.T) {
	db := setupDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	t.Run("seedDefaultCompanies", func(t *testing.T) {
		err := seedDefaultCompanies(tx)
		if err != nil {
			t.Fatalf("seedDefaultCompanies failed: %v", err)
		}

		var count int64
		_ = tx.Model(&models.Company{}).Where("code IN ?", []string{"mii", "sdd", "adidata", "ntt"}).Count(&count)
		if count < 4 {
			t.Errorf("expected at least 4 default companies, got %d", count)
		}
	})

	t.Run("SeedActivityStatuses", func(t *testing.T) {
		err := SeedActivityStatuses(tx)
		if err != nil {
			t.Fatalf("SeedActivityStatuses failed: %v", err)
		}

		var count int64
		_ = tx.Model(&models.ActivityStatus{}).Count(&count)
		if count == 0 {
			t.Error("expected activity statuses to be seeded, got 0")
		}
	})

	t.Run("EnsureSystemSettings", func(t *testing.T) {
		err := EnsureSystemSettings(tx)
		if err != nil {
			t.Fatalf("EnsureSystemSettings failed: %v", err)
		}

		var setting models.SystemSetting
		if err := tx.Where("key = ?", "is_new").First(&setting).Error; err != nil {
			t.Errorf("expected is_new setting to exist: %v", err)
		}
	})
}
