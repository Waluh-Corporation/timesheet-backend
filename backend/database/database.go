package database

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/models"
)

// Connect opens the PostgreSQL connection, runs migrations, and seeds the
// bootstrap admin account.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pooling for reliability and sub-second performance
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)
	sqlDB.SetConnMaxIdleTime(15 * time.Minute)

	// Run versioned SQL migrations first
	if err := RunMigrations(db); err != nil {
		log.Printf("[database] warning: migration runner: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		return nil, err
	}

	if err := seedAdmin(db, cfg); err != nil {
		return nil, err
	}
	if err := seedDefaultTemplate(db); err != nil {
		// Non-fatal: the portal still runs, admins can upload a template.
		log.Printf("[database] could not seed default template: %v", err)
	}
	if err := seedDefaultProjectsAndNormalize(db); err != nil {
		log.Printf("[database] normalization/project seeding error: %v", err)
	}
	return db, nil
}

// seedDefaultTemplate installs the bundled company templates on first boot
// (when no template exists yet): MII, SDD, Adidata, and NTT.
func seedDefaultTemplate(db *gorm.DB) error {
	// Seed companies first
	companies := []models.Company{
		{Code: "mii", Name: "PT Mitra Integrasi Informatika"},
		{Code: "sdd", Name: "PT Swadharma Duta Data"},
		{Code: "adidata", Name: "PT Adidata Informatics"},
		{Code: "ntt", Name: "PT NTT Data Indonesia"},
	}
	for _, c := range companies {
		var cnt int64
		_ = db.Model(&models.Company{}).Where("code = ?", c.Code).Count(&cnt).Error
		if cnt == 0 {
			_ = db.Create(&c).Error
		}
	}

	var count int64
	if err := db.Model(&models.Template{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Helper to find company ID
	findCompanyID := func(code string) *uint {
		var comp models.Company
		if err := db.Where("code = ?", code).Limit(1).Find(&comp).Error; err == nil && comp.ID != 0 {
			return &comp.ID
		}
		return nil
	}

	// 1. MII timesheet — the default template.
	mii := models.Template{
		Name:        "MII Timesheet",
		Description: "Built-in MII timesheet template (pure programmatic builder with K1 header logo).",
		SheetName:   "Sheet1",
		Company:     "MII",
		CompanyID:   findCompanyID("mii"),
		IsDefault:   true,
		Builtin:     "mii",
	}
	_ = db.Create(&mii)
	log.Printf("[database] seeded default template '%s' (builtin mii)", mii.Name)

	// 2. SDD timesheet — pure programmatic builder.
	sdd := models.Template{
		Name:        "SDD Timesheet",
		Description: "Built-in SDD timesheet template (pure programmatic builder with A2 header logo).",
		SheetName:   "Juni",
		Company:     "SDD",
		CompanyID:   findCompanyID("sdd"),
		IsDefault:   false,
		Builtin:     "sdd",
	}
	_ = db.Create(&sdd)
	log.Printf("[database] seeded template '%s' (builtin sdd)", sdd.Name)

	// 3. Adidata timesheet — pure programmatic builder with SPL.
	adidata := models.Template{
		Name:        "Adidata Timesheet",
		Description: "Built-in Adidata timesheet template (pure programmatic builder with N2 header logo & SPL support).",
		SheetName:   "TIMESHEET",
		Company:     "Adidata",
		CompanyID:   findCompanyID("adidata"),
		IsDefault:   false,
		Builtin:     "adidata",
	}
	_ = db.Create(&adidata)
	log.Printf("[database] seeded template '%s' (builtin adidata)", adidata.Name)

	// 4. NTT timesheet — pure programmatic builder.
	ntt := models.Template{
		Name:        "NTT Timesheet",
		Description: "Built-in NTT timesheet template (pure programmatic builder with N2 header logo).",
		SheetName:   "Timesheet",
		Company:     "NTT",
		CompanyID:   findCompanyID("ntt"),
		IsDefault:   false,
		Builtin:     "ntt",
	}
	_ = db.Create(&ntt)
	log.Printf("[database] seeded template '%s' (builtin ntt)", ntt.Name)

	return nil
}

// AutoMigrate runs GORM migrations for every entity.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Company{},
		&models.Department{},
		&models.ActivityStatus{},
		&models.Holiday{},
		&models.Project{},
		&models.User{},
		&models.WebAuthnCredential{},
		&models.Template{},
		&models.CellMapping{},
		&models.DailyActivity{},
		&models.OvertimeEntry{},
		&models.PushSubscription{},
		&models.ProfileChangeRequest{},
		&models.PasswordResetToken{},
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

	// 2. Backfill missing company_id on templates
	_ = db.Exec(`
		UPDATE templates 
		SET company_id = (
			SELECT id FROM companies 
			WHERE LOWER(companies.code) = LOWER(templates.company) 
			   OR LOWER(companies.name) LIKE '%' || LOWER(templates.company) || '%' 
			LIMIT 1
		) 
		WHERE (company_id IS NULL OR company_id = 0) 
		  AND company IS NOT NULL 
		  AND company != ''
	`).Error

	// 3. Seed default projects
	defaultProjects := []models.Project{
		{Code: "P24015", Name: "BNI Direct", AppImpacted: "BNI Direct Cash", CompanyID: findCompanyID("mii"), IsActive: true},
		{Code: "P24016", Name: "BNI Direct Overseas", AppImpacted: "BNI Direct Overseas", CompanyID: findCompanyID("mii"), IsActive: true},
		{Code: "P24017", Name: "BNI Direct Bisnis", AppImpacted: "BNI Direct Bisnis", CompanyID: findCompanyID("mii"), IsActive: true},
		{Code: "P24015", Name: "BNI Direct", AppImpacted: "BNI Direct", CompanyID: findCompanyID("ntt"), IsActive: true},
		{Code: "SDD-01", Name: "Core Banking Development", AppImpacted: "Core Banking", CompanyID: findCompanyID("sdd"), IsActive: true},
		{Code: "ADI-01", Name: "BNI Direct Maintenance", AppImpacted: "BNI Direct", CompanyID: findCompanyID("adidata"), IsActive: true},
	}
	for _, p := range defaultProjects {
		var cnt int64
		_ = db.Model(&models.Project{}).Where("code = ? AND (company_id = ? OR (company_id IS NULL AND ? IS NULL))", p.Code, p.CompanyID, p.CompanyID).Count(&cnt).Error
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
			_ = db.Create(&s).Error
		}
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
		_ = db.Model(&models.Department{}).Where("code = ? AND (company_id = ? OR (company_id IS NULL AND ? IS NULL))", d.Code, d.CompanyID, d.CompanyID).Count(&cnt).Error
		if cnt == 0 {
			_ = db.Create(&d).Error
		}
	}

	// 7. Backfill users.department_id
	_ = db.Exec(`
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
	`).Error

	// 8. Backfill templates.created_by to admin user if null
	_ = db.Exec(`
		UPDATE templates 
		SET created_by = (SELECT id FROM users WHERE role = 'admin' ORDER BY id ASC LIMIT 1) 
		WHERE (created_by IS NULL OR created_by = 0)
		  AND EXISTS (SELECT 1 FROM users WHERE role = 'admin')
	`).Error

	return nil
}

// seedAdmin creates the bootstrap admin the first time the portal boots with an
// empty users table. Public sign-up is disabled, so this is the only way an
// initial administrator can come into existence.
func seedAdmin(db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Never seed a known/guessable admin password. When BOOTSTRAP_ADMIN_PASSWORD
	// is not explicitly provided, generate a strong, NIST SP 800-63B-compliant
	// random one and print it once so an operator can capture it from the logs
	// and rotate it. When it IS provided, validate it against the same policy so
	// a weak default can never enter the system through the seeder.
	adminPassword := cfg.AdminPassword
	generated := false
	if adminPassword == "" {
		pw, err := auth.GeneratePassword(auth.GeneratedPasswordLength)
		if err != nil {
			return err
		}
		adminPassword = pw
		generated = true
	} else if err := auth.ValidatePassword(adminPassword, cfg.AdminUsername, cfg.AdminEmail); err != nil {
		log.Fatalf("[database] BOOTSTRAP_ADMIN_PASSWORD rejected by password policy: %v", err)
	}

	hash, err := auth.HashPassword(adminPassword)
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
	if generated {
		log.Printf("[database] seeded bootstrap admin '%s' (%s) with a GENERATED password: %s", cfg.AdminUsername, cfg.AdminEmail, adminPassword)
		log.Printf("[database] ^ capture this now and change it after first login; it will not be shown again")
	} else {
		log.Printf("[database] seeded bootstrap admin '%s' (%s) using BOOTSTRAP_ADMIN_PASSWORD", cfg.AdminUsername, cfg.AdminEmail)
	}
	return nil
}
