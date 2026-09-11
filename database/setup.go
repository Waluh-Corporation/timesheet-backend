package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/models"
)

// Setup runs database migrations and initial seeding.
func Setup(db *gorm.DB, cfg *config.Config) error {
	log.Println("[database] running database migrations...")
	if err := RunMigrations(db); err != nil {
		return fmt.Errorf("migration runner: %w", err)
	}

	if err := SeedActivityStatuses(db); err != nil {
		log.Printf("[database] could not seed activity statuses: %v", err)
	}
	if err := EnsureSystemSettings(db); err != nil {
		log.Printf("[database] could not ensure system settings: %v", err)
	}
	if err := seedAdmin(db, cfg); err != nil {
		return err
	}
	if err := seedDefaultCompanies(db); err != nil {
		log.Printf("[database] could not seed default companies: %v", err)
	}
	if err := seedDefaultProjectsAndNormalize(db); err != nil {
		log.Printf("[database] normalization/project seeding error: %v", err)
	}

	return nil
}

// seedDefaultCompanies seeds the primary companies on first boot: MII, SDD, Adidata, and NTT.
func seedDefaultCompanies(db *gorm.DB) error {
	companies := []models.Company{
		{Code: "mii", Name: "PT Mitra Integrasi Informatika"},
		{Code: "sdd", Name: "PT Swadharma Duta Data"},
		{Code: "adidata", Name: "PT Adidata Informatics"},
		{Code: "ntt", Name: "PT NTT Data Indonesia"},
	}
	for _, c := range companies {
		var cnt int64
		if err := db.Model(&models.Company{}).Where("code = ?", c.Code).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt == 0 {
			if err := db.Create(&c).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// AutoMigrate runs GORM migrations for every entity.
func AutoMigrate(db *gorm.DB) error {
	// Clean up any dangling foreign key references that would prevent constraint creation
	_ = db.Exec(`DELETE FROM daily_activities WHERE user_id NOT IN (SELECT id FROM users)`).Error
	_ = db.Exec(`UPDATE profile_change_requests SET reviewed_by = NULL WHERE reviewed_by IS NOT NULL AND reviewed_by NOT IN (SELECT id FROM users)`).Error
	_ = db.Exec(`UPDATE daily_activities SET project_ref_id = NULL WHERE project_ref_id IS NOT NULL AND project_ref_id NOT IN (SELECT id FROM projects)`).Error
	_ = db.Exec(`UPDATE overtime_entries SET daily_activity_id = NULL WHERE daily_activity_id IS NOT NULL AND daily_activity_id NOT IN (SELECT id FROM daily_activities)`).Error
	_ = db.Exec(`UPDATE overtime_entries SET team_leader_id = NULL WHERE team_leader_id IS NOT NULL AND team_leader_id NOT IN (SELECT id FROM approvers)`).Error
	_ = db.Exec(`UPDATE overtime_entries SET department_head_id = NULL WHERE department_head_id IS NOT NULL AND department_head_id NOT IN (SELECT id FROM approvers)`).Error
	// Drop legacy duplicate foreign key constraint on daily_activities if it exists
	_ = db.Exec(`ALTER TABLE daily_activities DROP CONSTRAINT IF EXISTS daily_activities_user_id_fkey`).Error
	// Drop holidays company_id foreign key constraint, index, and column if they exist
	_ = db.Exec(`ALTER TABLE holidays DROP CONSTRAINT IF EXISTS holidays_company_id_fkey`).Error
	_ = db.Exec(`DROP INDEX IF EXISTS idx_holidays_company_id`).Error
	_ = db.Exec(`ALTER TABLE holidays DROP COLUMN IF EXISTS company_id`).Error
	// Drop projects company_id foreign key constraint, index, and column if they exist
	_ = db.Exec(`ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_company_id_fkey`).Error
	_ = db.Exec(`DROP INDEX IF EXISTS idx_projects_company_id`).Error
	_ = db.Exec(`ALTER TABLE projects DROP COLUMN IF EXISTS company_id`).Error
	// Rename mii_id to bni_id and set comment on users and profile_change_requests if needed
	_ = db.Exec(`DO $$ BEGIN IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'mii_id') THEN ALTER TABLE users RENAME COLUMN mii_id TO bni_id; END IF; END $$;`).Error
	_ = db.Exec(`COMMENT ON COLUMN users.bni_id IS 'NPP BNI'`).Error
	_ = db.Exec(`DO $$ BEGIN IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profile_change_requests' AND column_name = 'mii_id') THEN ALTER TABLE profile_change_requests RENAME COLUMN mii_id TO bni_id; END IF; END $$;`).Error
	_ = db.Exec(`COMMENT ON COLUMN profile_change_requests.bni_id IS 'NPP BNI'`).Error

	return db.AutoMigrate(
		&models.Company{},
		&models.Department{},
		&models.ActivityStatus{},
		&models.Holiday{},
		&models.Project{},
		&models.User{},
		&models.Approver{},
		&models.WebAuthnCredential{},
		&models.DailyActivity{},
		&models.OvertimeEntry{},
		&models.PushSubscription{},
		&models.ProfileChangeRequest{},
		&models.PasswordResetToken{},
		&models.SystemSetting{},
	)
}

// seedDefaultProjectsAndNormalize backfills missing associations,
// seeds master statuses/projects/departments, and links existing data.
func seedDefaultProjectsAndNormalize(db *gorm.DB) error {
	findCompanyID := func(code string) *uint {
		var comp models.Company
		if err := db.Where("code = ?", code).Limit(1).Find(&comp).Error; err == nil && comp.ID != 0 {
			return &comp.ID
		}
		return nil
	}

	// 1. Backfill missing company_id on users
	_ = db.Exec(`
		UPDATE users 
		SET company_id = (
			SELECT id FROM companies 
			WHERE LOWER(companies.code) = LOWER(users.company) 
			   OR LOWER(companies.name) LIKE '%' || LOWER(users.company) || '%' 
			LIMIT 1
		) 
		WHERE (company_id IS NULL OR company_id = 0) 
		  AND company IS NOT NULL 
		  AND company != ''
	`).Error

	// 3. Seed default projects
	defaultProjects := []models.Project{
		{Code: "P24015", Name: "BNI Direct Cash", AppImpacted: "BNI Direct Cash", IsActive: true},
		{Code: "P24015", Name: "BNI Direct Overseas", AppImpacted: "BNI Direct Overseas", IsActive: true},
		{Code: "P24015", Name: "BNI Direct Bisnis", AppImpacted: "BNI Direct Bisnis", IsActive: true},
	}
	for _, p := range defaultProjects {
		var cnt int64
		_ = db.Model(&models.Project{}).Where("code = ? AND name = ?", p.Code, p.Name).Count(&cnt).Error
		if cnt == 0 {
			_ = db.Create(&p).Error
		}
	}

	// 4. Backfill daily_activities.project_ref_id
	_ = db.Exec(`
		UPDATE daily_activities 
		SET project_ref_id = (
			SELECT id FROM projects 
			WHERE projects.code = daily_activities.project_id 
			LIMIT 1
		) 
		WHERE project_ref_id IS NULL 
		  AND project_id IS NOT NULL 
		  AND project_id != ''
	`).Error

	_ = db.Exec(`
		UPDATE daily_activities 
		SET project_ref_id = (
			SELECT id FROM projects 
			WHERE LOWER(projects.name) = LOWER(daily_activities.project_name) 
			LIMIT 1
		) 
		WHERE project_ref_id IS NULL 
		  AND project_name IS NOT NULL 
		  AND project_name != ''
	`).Error

	// 5. Seed default activity statuses
	if err := SeedActivityStatuses(db); err != nil {
		log.Printf("[database] could not seed activity statuses: %v", err)
	}

	// Standardize daily_activities.status to uppercase and map unmapped to 'P'
	_ = db.Exec(`
		UPDATE daily_activities 
		SET status = UPPER(TRIM(status)) 
		WHERE status IS NOT NULL AND status != ''
	`).Error
	_ = db.Exec(`
		UPDATE daily_activities 
		SET status = 'P' 
		WHERE status IS NULL OR status = '' OR status NOT IN (SELECT code FROM activity_statuses)
	`).Error

	// 6. Seed default departments
	defaultDepartments := []models.Department{
		{Code: "WCSD", Name: "Wholesale Channel and Service Delivery", Division: "Wholesale Digital Delivery", CompanyID: findCompanyID("mii"), IsActive: true},
		{Code: "SDD-DEV1", Name: "Kelompok Pengembangan 1", Division: "Application Development Division", CompanyID: findCompanyID("sdd"), IsActive: true},
		{Code: "IT-BANK", Name: "IT Banking Application", Division: "IT Banking", CompanyID: findCompanyID("ntt"), IsActive: true},
		{Code: "ADI-TS", Name: "Technical Support & Dev", Division: "Application Development", CompanyID: findCompanyID("adidata"), IsActive: true},
	}
	for _, d := range defaultDepartments {
		var cnt int64
		q := db.Model(&models.Department{}).Where("code = ?", d.Code)
		if d.CompanyID == nil {
			q = q.Where("company_id IS NULL")
		} else {
			q = q.Where("company_id = ?", *d.CompanyID)
		}
		_ = q.Count(&cnt).Error
		if cnt == 0 {
			_ = db.Create(&d).Error
		}
	}

	// 7. Backfill users.department_id
	if err := db.Exec(`
		UPDATE users 
		SET department_id = (
			SELECT id FROM departments 
			WHERE LOWER(departments.name) = LOWER(users.department) 
			   OR LOWER(departments.code) = LOWER(users.department) 
			   OR (users.department IS NULL AND LOWER(departments.division) = LOWER(users.division))
			LIMIT 1
		) 
		WHERE (department_id IS NULL OR department_id = 0) 
		  AND ((department IS NOT NULL AND department != '') OR (division IS NOT NULL AND division != ''))
	`).Error; err != nil {
		return err
	}

	return nil
}

// SeedActivityStatuses seeds default activity statuses (P, BT, S, PM, V, X) idempotently.
func SeedActivityStatuses(db *gorm.DB) error {
	defaultStatuses := []models.ActivityStatus{
		{Code: "P", Name: "Present", Description: "Hadir bekerja normal", IsWorkingDay: true, SortOrder: 1},
		{Code: "BT", Name: "Business Trip", Description: "Perjalanan dinas", IsWorkingDay: true, SortOrder: 2},
		{Code: "S", Name: "Sick", Description: "Sakit", IsWorkingDay: false, SortOrder: 3},
		{Code: "PM", Name: "Permission", Description: "Izin", IsWorkingDay: false, SortOrder: 4},
		{Code: "V", Name: "Leave", Description: "Cuti / Vacation", IsWorkingDay: false, SortOrder: 5},
		{Code: "X", Name: "Off", Description: "Libur / Off", IsWorkingDay: false, SortOrder: 6},
	}
	for _, s := range defaultStatuses {
		var cnt int64
		_ = db.Model(&models.ActivityStatus{}).Where("code = ?", s.Code).Count(&cnt).Error
		if cnt == 0 {
			if err := db.Create(&s).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// seedAdmin creates the bootstrap admin when BOOTSTRAP_ADMIN_PASSWORD is set in config
// and the users table has no admin account. If no password is provided in config,
// seeding is skipped so the initial setup / onboarding wizard can be used.
func seedAdmin(db *gorm.DB, cfg *config.Config) error {
	if cfg.AdminPassword == "" {
		return nil
	}

	var count int64
	if err := db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	if err := auth.ValidatePassword(cfg.AdminPassword, cfg.AdminUsername, cfg.AdminEmail); err != nil {
		log.Fatalf("[database] BOOTSTRAP_ADMIN_PASSWORD rejected by password policy: %v", err)
	}

	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	admin := models.User{
		Username:     cfg.AdminUsername,
		Email:        cfg.AdminEmail,
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
		IsActive:     true,
		Name:         "Portal Administrator",
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	_ = db.Save(&models.SystemSetting{
		Key:       "is_new",
		Value:     "N",
		UpdatedAt: time.Now(),
	}).Error
	log.Printf("[database] seeded bootstrap admin '%s' (%s) using BOOTSTRAP_ADMIN_PASSWORD", cfg.AdminUsername, cfg.AdminEmail)
	return nil
}

// EnsureSystemSettings ensures that essential system configuration flags exist.
func EnsureSystemSettings(db *gorm.DB) error {
	var cnt int64
	_ = db.Model(&models.SystemSetting{}).Where("key = ?", "is_new").Count(&cnt).Error
	if cnt == 0 {
		var adminCount int64
		_ = db.Model(&models.User{}).Where("role = ? AND deleted_at IS NULL", models.RoleAdmin).Count(&adminCount).Error
		val := "Y"
		if adminCount > 0 {
			val = "N"
		}
		_ = db.Create(&models.SystemSetting{
			Key:       "is_new",
			Value:     val,
			UpdatedAt: time.Now(),
		}).Error
	}
	return nil
}
