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
		_ = tx.Exec("DELETE FROM companies WHERE LOWER(code) IN ('mii', 'sdd', 'adidata', 'ntt')").Error
		err := seedDefaultCompanies(tx)
		if err != nil {
			t.Fatalf("seedDefaultCompanies failed: %v", err)
		}

		var count int64
		_ = tx.Model(&models.Company{}).Where("LOWER(code) IN ?", []string{"mii", "sdd", "adidata", "ntt"}).Count(&count)
		if count < 4 {
			t.Errorf("expected at least 4 default companies, got %d", count)
		}

		// Idempotent test (covers cnt > 0 branch)
		if err := seedDefaultCompanies(tx); err != nil {
			t.Fatalf("seedDefaultCompanies idempotent failed: %v", err)
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

		// Idempotent
		if err := SeedActivityStatuses(tx); err != nil {
			t.Fatalf("SeedActivityStatuses idempotent failed: %v", err)
		}
	})

	t.Run("EnsureSystemSettings", func(t *testing.T) {
		_ = tx.Exec("DELETE FROM system_settings WHERE key = 'is_new'").Error
		err := EnsureSystemSettings(tx)
		if err != nil {
			t.Fatalf("EnsureSystemSettings failed: %v", err)
		}

		var setting models.SystemSetting
		if err := tx.Where("key = ?", "is_new").First(&setting).Error; err != nil {
			t.Errorf("expected is_new setting to exist: %v", err)
		}

		// Idempotent
		if err := EnsureSystemSettings(tx); err != nil {
			t.Fatalf("EnsureSystemSettings idempotent failed: %v", err)
		}
	})

	t.Run("seedDefaultProjectsAndNormalize", func(t *testing.T) {
		err := seedDefaultProjectsAndNormalize(tx)
		if err != nil {
			t.Fatalf("seedDefaultProjectsAndNormalize failed: %v", err)
		}
	})

	t.Run("seedDefaultDepartments", func(t *testing.T) {
		_ = tx.Exec("DELETE FROM departments WHERE LOWER(code) = 'wdl'").Error
		err := seedDefaultDepartments(tx)
		if err != nil {
			t.Fatalf("seedDefaultDepartments failed: %v", err)
		}
		if err := seedDefaultDepartments(tx); err != nil {
			t.Fatalf("seedDefaultDepartments idempotent run failed: %v", err)
		}
	})

	t.Run("seedDefaultSitesAndDivisions", func(t *testing.T) {
		_ = tx.Exec("DELETE FROM sites WHERE LOWER(code) IN ('ctcn', 'rdtx', 'pjp')").Error
		_ = tx.Exec("DELETE FROM divisions WHERE LOWER(code) = 'wdd'").Error
		err := seedDefaultSitesAndDivisions(tx)
		if err != nil {
			t.Fatalf("seedDefaultSitesAndDivisions failed: %v", err)
		}
		if err := seedDefaultSitesAndDivisions(tx); err != nil {
			t.Fatalf("seedDefaultSitesAndDivisions idempotent run failed: %v", err)
		}
		var siteCount, divCount int64
		_ = tx.Model(&models.Site{}).Count(&siteCount)
		_ = tx.Model(&models.Division{}).Count(&divCount)
		if siteCount < 3 {
			t.Errorf("expected at least 3 sites, got %d", siteCount)
		}
		if divCount < 1 {
			t.Errorf("expected at least 1 division, got %d", divCount)
		}
	})

	t.Run("seedAdmin skipped when empty password", func(t *testing.T) {
		emptyCfg := &config.Config{}
		err := seedAdmin(tx, emptyCfg)
		if err != nil {
			t.Fatalf("seedAdmin with empty password failed: %v", err)
		}
	})

	t.Run("seedAdmin creates admin when configured", func(t *testing.T) {
		adminCfg := &config.Config{
			AdminUsername: "testadmboot",
			AdminEmail:    "admboot@example.com",
			AdminPassword: "SuperAdminPass2026!",
		}
		_ = tx.Exec("DELETE FROM users WHERE role = 'admin'").Error
		err := seedAdmin(tx, adminCfg)
		if err != nil {
			t.Fatalf("seedAdmin failed: %v", err)
		}
		// Idempotent (count > 0 branch)
		err = seedAdmin(tx, adminCfg)
		if err != nil {
			t.Fatalf("seedAdmin idempotent run failed: %v", err)
		}
	})

	t.Run("AutoMigrate in isolated schema", func(t *testing.T) {
		txMig := db.Begin()
		defer func() {
			_ = txMig.Exec("DROP SCHEMA IF EXISTS test_automigrate CASCADE").Error
			txMig.Rollback()
		}()
		_ = txMig.Exec("CREATE SCHEMA IF NOT EXISTS test_automigrate").Error
		_ = txMig.Exec("SET search_path TO test_automigrate").Error
		if err := AutoMigrate(txMig); err != nil {
			t.Fatalf("AutoMigrate failed: %v", err)
		}
	})

	t.Run("Setup", func(t *testing.T) {
		txSetup := db.Begin()
		defer txSetup.Rollback()
		cfg := config.Load()
		if err := Setup(txSetup, cfg); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	})

	t.Run("SyncAuthenticatorAAGUIDs", func(t *testing.T) {
		txSync := db.Begin()
		defer txSync.Rollback()
		aaguid := "fa264024-4a24-4e2b-a489-3224b1263d91" // gitleaks:allow
		_ = txSync.Create(&models.AuthenticatorAAGUID{
			AAGUID:    aaguid,
			Name:      "Sync Test Authenticator",
			IconLight: "data:image/svg+xml;base64,bGlnaHQ=",
			IconDark:  "data:image/svg+xml;base64,ZGFyaw==",
		}).Error
		if err := SyncAuthenticatorAAGUIDs(txSync); err != nil {
			t.Fatalf("SyncAuthenticatorAAGUIDs failed: %v", err)
		}
	})
}
