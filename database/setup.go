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

const (
	queryCode          = "code = ?"
	defaultDivisionWDD = "Wholesale Digital Delivery"
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
	if err := SyncAuthenticatorAAGUIDs(db); err != nil {
		log.Printf("[database] could not sync authenticator aaguids: %v", err)
	}
	if err := seedAdmin(db, cfg); err != nil {
		return err
	}
	if err := seedDefaultCompanies(db); err != nil {
		log.Printf("[database] could not seed default companies: %v", err)
	}
	if err := seedDefaultDepartments(db); err != nil {
		log.Printf("[database] could not seed default departments: %v", err)
	}
	if err := seedDefaultSitesAndDivisions(db); err != nil {
		log.Printf("[database] could not seed default sites and divisions: %v", err)
	}
	if err := seedDefaultProjectsAndNormalize(db); err != nil {
		log.Printf("[database] normalization/project seeding error: %v", err)
	}

	return nil
}

// seedDefaultCompanies seeds the primary companies on first boot: MII, SDD, Adidata, and NTT.
func seedDefaultCompanies(db *gorm.DB) error {
	companies := []models.Company{
		{Code: "MII", Name: "PT Mitra Integrasi Informatika", IsActive: true},
		{Code: "SDD", Name: "PT Swadharma Duta Data", IsActive: true},
		{Code: "Adidata", Name: "PT Adidata Informatika", IsActive: true},
		{Code: "NTT", Name: "PT NTT Data Indonesia", IsActive: true},
	}
	for _, c := range companies {
		var cnt int64
		if err := db.Model(&models.Company{}).Where(queryCode, c.Code).Count(&cnt).Error; err != nil {
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

// seedDefaultDepartments seeds the default department on first boot.
func seedDefaultDepartments(db *gorm.DB) error {
	departments := []models.Department{
		{Code: "WDL", Name: "Wholesale Channel and Service Delivery", Division: defaultDivisionWDD, IsActive: true},
	}
	for _, d := range departments {
		var cnt int64
		if err := db.Model(&models.Department{}).Where(queryCode, d.Code).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt == 0 {
			if err := db.Create(&d).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// seedDefaultSitesAndDivisions seeds default sites and divisions on first boot.
func seedDefaultSitesAndDivisions(db *gorm.DB) error {
	sites := []models.Site{
		{Code: "ctcn", Name: "Citicon", IsActive: true},
		{Code: "rdtx", Name: "RDTX", IsActive: true},
		{Code: "pjp", Name: "Pejompongan", IsActive: true},
	}
	for _, s := range sites {
		var cnt int64
		if err := db.Model(&models.Site{}).Where(queryCode, s.Code).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt == 0 {
			if err := db.Create(&s).Error; err != nil {
				return err
			}
		}
	}

	divisions := []models.Division{
		{Code: "wdd", Name: defaultDivisionWDD, IsActive: true},
	}
	for _, d := range divisions {
		var cnt int64
		if err := db.Model(&models.Division{}).Where(queryCode, d.Code).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt == 0 {
			if err := db.Create(&d).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// AutoMigrate runs GORM migrations for every entity.
// Schema modifications and DDL statements are managed exclusively via versioned
// SQL migrations in database/migrations/*.sql as the Single Source of Truth.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Site{},
		&models.Division{},
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
		&models.RefreshToken{},
		&models.SystemSetting{},
		&models.AuthenticatorAAGUID{},
	)
}

// seedDefaultProjectsAndNormalize backfills missing associations,
// seeds master statuses/projects/departments, and links existing data.
func seedDefaultProjectsAndNormalize(db *gorm.DB) error {
	// 1. Backfill missing company_id on non-admin users
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
		  AND role != 'admin'
	`).Error

	// 2. Clear any company association for admin accounts
	_ = db.Exec(`
		UPDATE users 
		SET company_id = NULL, company = '' 
		WHERE role = 'admin' AND (company_id IS NOT NULL OR (company IS NOT NULL AND company != ''))
	`).Error

	// 3. Seed default projects
	defaultProjects := []models.Project{
		{Code: "P24015", Name: "BNI Direct Cash", AppImpacted: "Cash", IsActive: true},
		{Code: "P24015", Name: "BNI Direct Overseas", AppImpacted: "Overseas", IsActive: true},
		{Code: "P24015", Name: "BNI Direct Bisnis", AppImpacted: "Bisnis", IsActive: true},
	}
	for _, p := range defaultProjects {
		var cnt int64
		_ = db.Model(&models.Project{}).Where("code = ? AND name = ?", p.Code, p.Name).Count(&cnt).Error
		if cnt == 0 {
			_ = db.Create(&p).Error
		}
	}

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
		{Code: "WDL", Name: "Wholesale Channel and Service Delivery", Division: defaultDivisionWDD, IsActive: true},
	}
	for _, d := range defaultDepartments {
		var cnt int64
		_ = db.Model(&models.Department{}).Where(queryCode, d.Code).Count(&cnt).Error
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

	// 8. Synchronize users.company and users.department with master records
	_ = db.Exec(`
		UPDATE users 
		SET company = companies.name 
		FROM companies 
		WHERE users.company_id = companies.id
		  AND (users.company IS NULL OR users.company != companies.name)
	`).Error
	_ = db.Exec(`
		UPDATE users 
		SET department = departments.name 
		FROM departments 
		WHERE users.department_id = departments.id
		  AND (users.department IS NULL OR users.department != departments.name)
	`).Error

	// 9. Backfill users.site_id and users.division_id
	_ = db.Exec(`
		UPDATE users 
		SET site_id = (
			SELECT id FROM sites 
			WHERE LOWER(sites.name) = LOWER(users.site) 
			   OR LOWER(sites.code) = LOWER(users.site) 
			LIMIT 1
		) 
		WHERE (site_id IS NULL OR site_id = 0) 
		  AND site IS NOT NULL AND site != ''
	`).Error

	_ = db.Exec(`
		UPDATE users 
		SET division_id = (
			SELECT id FROM divisions 
			WHERE LOWER(divisions.name) = LOWER(users.division) 
			   OR LOWER(divisions.code) = LOWER(users.division) 
			LIMIT 1
		) 
		WHERE (division_id IS NULL OR division_id = 0) 
		  AND division IS NOT NULL AND division != ''
	`).Error

	// 10. Backfill departments.division_id
	_ = db.Exec(`
		UPDATE departments 
		SET division_id = (
			SELECT id FROM divisions 
			WHERE LOWER(divisions.name) = LOWER(departments.division) 
			   OR LOWER(divisions.code) = LOWER(departments.division) 
			LIMIT 1
		) 
		WHERE (division_id IS NULL OR division_id = 0) 
		  AND division IS NOT NULL AND division != ''
	`).Error

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
		_ = db.Model(&models.ActivityStatus{}).Where(queryCode, s.Code).Count(&cnt).Error
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

// SyncAuthenticatorAAGUIDs loads all AAGUID authenticator mappings from the database
// into the thread-safe in-memory models registry.
func SyncAuthenticatorAAGUIDs(db *gorm.DB) error {
	var rows []models.AuthenticatorAAGUID
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		models.RegisterAuthenticator(r.AAGUID, r.Name, r.Icon)
	}
	log.Printf("[database] synced %d authenticator aaguids from database into in-memory registry", len(rows))
	return nil
}
